package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/skip2/go-qrcode"

	"multipay/internal/database"
	"multipay/internal/model"
)

type BinanceTransaction struct {
	OrderType       string `json:"orderType"` // e.g. "C2C"
	TransactionID   string `json:"transactionId"`
	TransactionTime int64  `json:"transactionTime"` // ms
	Amount          string `json:"amount"`
	Currency        string `json:"currency"` // "USDT"
	PayerInfo       struct {
		Name      string `json:"name"`
		BinanceID string `json:"binanceId"`
	} `json:"payerInfo"`
}

type BinanceTransactionsResponse struct {
	Code    string               `json:"code"`
	Message string               `json:"message"`
	Data    []BinanceTransaction `json:"data"`
	Success bool                 `json:"success"`
}

type BinanceEngine struct {
	mu           sync.RWMutex
	db           *database.DB
	webhook      *WebhookDispatcher
	httpClient   *http.Client
	timeOffsetMs int64
	backoffUntil time.Time
	lastPollTime time.Time
}

func NewBinanceEngine(db *database.DB, webhook *WebhookDispatcher) *BinanceEngine {
	return &BinanceEngine{
		db:         db,
		webhook:    webhook,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (b *BinanceEngine) sign(queryString, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(queryString))
	return hex.EncodeToString(mac.Sum(nil))
}

func (b *BinanceEngine) SyncServerTime(baseURL string) (int64, error) {
	if baseURL == "" {
		baseURL = "https://api.binance.com"
	}
	resp, err := b.httpClient.Get(baseURL + "/api/v3/time")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var res struct {
		ServerTime int64 `json:"serverTime"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return 0, err
	}

	localTime := time.Now().UnixMilli()
	offset := res.ServerTime - localTime
	b.mu.Lock()
	b.timeOffsetMs = offset
	b.mu.Unlock()
	return offset, nil
}

func (b *BinanceEngine) FetchTransactions() ([]BinanceTransaction, error) {
	b.mu.RLock()
	if time.Now().Before(b.backoffUntil) {
		b.mu.RUnlock()
		return nil, errors.New("binance api rate limited; backing off")
	}
	offset := b.timeOffsetMs
	b.mu.RUnlock()

	apiKey, _ := b.db.GetConfig("binance_api_key")
	secretKey, _ := b.db.GetConfig("binance_secret_key")
	baseURL, _ := b.db.GetConfig("binance_base_url")
	if baseURL == "" {
		baseURL = "https://api.binance.com"
	}

	if apiKey == "" || secretKey == "" {
		return nil, errors.New("binance API credentials not configured in system_config")
	}

	nowMs := time.Now().UnixMilli() + offset
	startMs := nowMs - (24 * 60 * 60 * 1000) // look back 24h

	params := url.Values{}
	params.Set("startTime", strconv.FormatInt(startMs, 10))
	params.Set("limit", "100")
	params.Set("recvWindow", "60000")
	params.Set("timestamp", strconv.FormatInt(nowMs, 10))

	rawQuery := params.Encode()
	signature := b.sign(rawQuery, secretKey)
	fullURL := fmt.Sprintf("%s/sapi/v1/pay/transactions?%s&signature=%s", baseURL, rawQuery, signature)

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-MBX-APIKEY", apiKey)

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 || resp.StatusCode == 418 {
		b.mu.Lock()
		b.backoffUntil = time.Now().Add(60 * time.Second)
		b.mu.Unlock()
		return nil, fmt.Errorf("binance rate limit hit (HTTP %d)", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res BinanceTransactionsResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}

	// Handle clock drift error code "-1021"
	if res.Code == "-1021" {
		log.Println("[BinanceEngine] Timestamp out of sync, resyncing server time...")
		_, _ = b.SyncServerTime(baseURL)
		return nil, errors.New("timestamp out of sync, resynced with binance")
	}

	if res.Code != "000000" && res.Code != "0" && res.Code != "" {
		return nil, fmt.Errorf("binance api error code %s: %s", res.Code, res.Message)
	}

	b.mu.Lock()
	b.lastPollTime = time.Now()
	b.mu.Unlock()

	return res.Data, nil
}

func (b *BinanceEngine) ProcessIncomingTransactions() (int, error) {
	txs, err := b.FetchTransactions()
	if err != nil {
		return 0, err
	}

	matchedCount := 0
	for _, tx := range txs {
		// Only incoming USDT payments
		amtFloat, err := strconv.ParseFloat(tx.Amount, 64)
		if err != nil || amtFloat <= 0 {
			continue
		}
		if tx.Currency != "USDT" {
			continue
		}

		milli := int64(math.Round(amtFloat * 1000.0))
		payerName := tx.PayerInfo.Name
		if payerName == "" {
			payerName = tx.PayerInfo.BinanceID
		}

		// Try to match pending or recently expired order
		order, err := b.db.FindMatchingOrder(model.ChannelBinance, "USDT", milli, true)
		if err == nil && order != nil {
			err = b.db.MarkOrderPaid(order.ID, tx.TransactionID, "", payerName, 0)
			if err == nil {
				matchedCount++
				log.Printf("[BinanceEngine] Match found! Order %s marked PAID with TX %s", order.ID, tx.TransactionID)
				order.Status = model.StatusPaid
				order.TxID = tx.TransactionID
				order.FromInfo = payerName
				order.PaidAt = time.Now().UnixMilli()
				if b.webhook != nil {
					go b.webhook.DispatchOrder(b.db, order)
				}
				continue
			}
		}

		// If no matching order, record as unmatched transaction
		u := &model.UnmatchedTx{
			ID:          tx.TransactionID,
			Channel:     model.ChannelBinance,
			Currency:    "USDT",
			AmountStr:   fmt.Sprintf("%.3f", amtFloat),
			AmountMilli: milli,
			FromInfo:    payerName,
			TxTime:      tx.TransactionTime,
		}
		_ = b.db.SaveUnmatchedTx(u)
	}

	return matchedCount, nil
}

func (b *BinanceEngine) GenerateQRCodeFile(content, outputPath string) error {
	_ = os.MkdirAll(filepath.Dir(outputPath), 0755)
	return qrcode.WriteFile(content, qrcode.Medium, 320, outputPath)
}
