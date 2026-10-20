package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/skip2/go-qrcode"

	"multipay/internal/database"
	"multipay/internal/model"
)

type RPCLogItem struct {
	Address          string   `json:"address"`
	Topics           []string `json:"topics"`
	Data             string   `json:"data"`
	BlockNumber      string   `json:"blockNumber"`
	TransactionHash  string   `json:"transactionHash"`
	TransactionIndex string   `json:"transactionIndex"`
	BlockHash        string   `json:"blockHash"`
	LogIndex         string   `json:"logIndex"`
	Removed          bool     `json:"removed"`
}

type jsonRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type USDTEngine struct {
	mu         sync.RWMutex
	db         *database.DB
	webhook    *WebhookDispatcher
	httpClient *http.Client
	activeIdx  int
}

const (
	USDTContractAddress = "0x55d398326f99059ff775485246999027b3197955"
	TransferTopic       = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
)

func NewUSDTEngine(db *database.DB, webhook *WebhookDispatcher) *USDTEngine {
	return &USDTEngine{
		db:         db,
		webhook:    webhook,
		httpClient: &http.Client{Timeout: 6 * time.Second},
	}
}

func (u *USDTEngine) CallRPC(ctx context.Context, method string, params []interface{}, resultDest interface{}) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	rpcURLsStr, _ := u.db.GetConfig("bsc_rpc_urls")
	endpoints := strings.Split(rpcURLsStr, ",")
	if len(endpoints) == 0 || endpoints[0] == "" {
		endpoints = []string{"https://bsc-rpc.publicnode.com", "https://1rpc.io/bnb", "https://bsc.meowrpc.com"}
	}

	totalEndpoints := len(endpoints)
	reqBody := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	for attempt := 0; attempt < totalEndpoints; attempt++ {
		url := strings.TrimSpace(endpoints[u.activeIdx])
		if url == "" {
			u.activeIdx = (u.activeIdx + 1) % totalEndpoints
			continue
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
		if err != nil {
			u.activeIdx = (u.activeIdx + 1) % totalEndpoints
			continue
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := u.httpClient.Do(httpReq)
		if err != nil {
			u.activeIdx = (u.activeIdx + 1) % totalEndpoints
			continue
		}

		respBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK {
			u.activeIdx = (u.activeIdx + 1) % totalEndpoints
			continue
		}

		var rpcResp jsonRPCResponse
		if err := json.Unmarshal(respBytes, &rpcResp); err != nil || rpcResp.Error != nil {
			u.activeIdx = (u.activeIdx + 1) % totalEndpoints
			continue
		}

		if err := json.Unmarshal(rpcResp.Result, resultDest); err != nil {
			u.activeIdx = (u.activeIdx + 1) % totalEndpoints
			continue
		}

		return nil
	}

	return fmt.Errorf("all %d RPC endpoints failed for method %s", totalEndpoints, method)
}

func (u *USDTEngine) GetActiveRPC() string {
	u.mu.RLock()
	defer u.mu.RUnlock()
	rpcURLsStr, _ := u.db.GetConfig("bsc_rpc_urls")
	endpoints := strings.Split(rpcURLsStr, ",")
	if len(endpoints) == 0 || endpoints[0] == "" {
		return "https://bsc-rpc.publicnode.com"
	}
	if u.activeIdx >= len(endpoints) {
		return strings.TrimSpace(endpoints[0])
	}
	return strings.TrimSpace(endpoints[u.activeIdx])
}

func (u *USDTEngine) GetLatestBlockNumber(ctx context.Context) (int64, string, error) {
	var hexStr string
	if err := u.CallRPC(ctx, "eth_blockNumber", []interface{}{}, &hexStr); err != nil {
		return 0, u.GetActiveRPC(), err
	}
	clean := strings.TrimPrefix(hexStr, "0x")
	block, err := strconv.ParseInt(clean, 16, 64)
	return block, u.GetActiveRPC(), err
}

func (u *USDTEngine) FormatReceiverTopic(wallet string) string {
	clean := strings.ToLower(strings.TrimPrefix(wallet, "0x"))
	return fmt.Sprintf("0x000000000000000000000000%s", clean)
}

func (u *USDTEngine) ScanOnChainTransfers(ctx context.Context) (int, error) {
	receiverWallet, _ := u.db.GetConfig("receiver_wallet_address")
	if receiverWallet == "" {
		return 0, errors.New("receiver wallet address not configured")
	}

	latestBlock, _, err := u.GetLatestBlockNumber(ctx)
	if err != nil {
		return 0, err
	}

	confsStr, _ := u.db.GetConfig("required_confirmations")
	confs, _ := strconv.Atoi(confsStr)
	if confs <= 0 {
		confs = 2
	}
	safeBlock := latestBlock - int64(confs)

	lastScannedStr, _ := u.db.GetConfig("last_scanned_block")
	lastScanned, _ := strconv.ParseInt(lastScannedStr, 10, 64)
	if lastScanned == 0 || lastScanned > safeBlock {
		lastScanned = safeBlock - 10 // first run: look back 10 blocks
	}

	fromBlock := lastScanned + 1
	if safeBlock-fromBlock > 400 {
		fromBlock = safeBlock - 400 // prevent block chunk overflow
	}
	if fromBlock > safeBlock {
		return 0, nil
	}

	toBlock := fromBlock + 300
	if toBlock > safeBlock {
		toBlock = safeBlock
	}

	filter := map[string]interface{}{
		"fromBlock": fmt.Sprintf("0x%x", fromBlock),
		"toBlock":   fmt.Sprintf("0x%x", toBlock),
		"address":   USDTContractAddress,
		"topics": []interface{}{
			TransferTopic,
			nil, // any sender
			u.FormatReceiverTopic(receiverWallet),
		},
	}

	var logs []RPCLogItem
	if err := u.CallRPC(ctx, "eth_getLogs", []interface{}{filter}, &logs); err != nil {
		return 0, err
	}

	matchedCount := 0
	divisor := big.NewInt(1000000000000000) // 10^15 (USDT 18 decimals -> milli 10^3)

	for _, item := range logs {
		if len(item.Topics) < 3 {
			continue
		}

		rawHex := strings.TrimPrefix(item.Data, "0x")
		if rawHex == "" {
			continue
		}
		rawWei := new(big.Int)
		rawWei.SetString(rawHex, 16)
		if rawWei.Sign() <= 0 {
			continue // ignore 0-value transfers (anti-poisoning)
		}

		quotient := new(big.Int)
		remainder := new(big.Int)
		quotient.DivMod(rawWei, divisor, remainder)

		milli := quotient.Int64()
		fromAddr := "0x" + strings.TrimPrefix(item.Topics[1], "0x000000000000000000000000")
		txHash := item.TransactionHash

		blockNum, _ := strconv.ParseInt(strings.TrimPrefix(item.BlockNumber, "0x"), 16, 64)
		logIdx, _ := strconv.ParseInt(strings.TrimPrefix(item.LogIndex, "0x"), 16, 64)
		txID := fmt.Sprintf("%s_%d", txHash, logIdx)

		// Check if matches order
		if remainder.Sign() == 0 && milli > 0 {
			order, err := u.db.FindMatchingOrder(model.ChannelUSDT, "USDT", milli, true)
			if err == nil && order != nil {
				err = u.db.MarkOrderPaid(order.ID, txID, txHash, fromAddr, blockNum)
				if err == nil {
					matchedCount++
					log.Printf("[USDTEngine] Match found! Order %s marked PAID via tx %s (block %d)", order.ID, txHash, blockNum)
					order.Status = model.StatusPaid
					order.TxID = txID
					order.TxHash = txHash
					order.FromInfo = fromAddr
					order.PaidAt = time.Now().UnixMilli()
					if u.webhook != nil {
						go u.webhook.DispatchOrder(u.db, order)
					}
					continue
				}
			}
		}

		// Otherwise save as unmatched
		uRecord := &model.UnmatchedTx{
			ID:          txID,
			Channel:     model.ChannelUSDT,
			TxHash:      txHash,
			FromInfo:    fromAddr,
			Currency:    "USDT",
			AmountStr:   fmt.Sprintf("%.3f", float64(milli)/1000.0),
			AmountMilli: milli,
			RawPayload:  rawWei.String(),
			TxTime:      time.Now().UnixMilli(),
		}
		_ = u.db.SaveUnmatchedTx(uRecord)
	}

	_ = u.db.SetConfig("last_scanned_block", strconv.FormatInt(toBlock, 10))
	return matchedCount, nil
}

func (u *USDTEngine) GenerateQRCodeFile(walletAddress, outputPath string) error {
	_ = os.MkdirAll(filepath.Dir(outputPath), 0755)
	return qrcode.WriteFile(walletAddress, qrcode.Medium, 320, outputPath)
}
