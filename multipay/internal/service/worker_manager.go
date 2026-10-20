package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"multipay/internal/config"
	"multipay/internal/database"
	"multipay/internal/model"
)

type WorkerManager struct {
	cfg        *config.Config
	liveDB     *database.DB
	sandboxDB  *database.DB
	qrisEngine *QRISEngine
	binanceEng *BinanceEngine
	usdtEngine *USDTEngine
	webhook    *WebhookDispatcher
	httpClient *http.Client

	// Watchdog state
	alertSent    bool
	criticalSent bool
	mu           sync.Mutex
}

func NewWorkerManager(
	cfg *config.Config,
	liveDB *database.DB,
	sandboxDB *database.DB,
	qrisEngine *QRISEngine,
	binanceEng *BinanceEngine,
	usdtEngine *USDTEngine,
	webhook *WebhookDispatcher,
) *WorkerManager {
	return &WorkerManager{
		cfg:        cfg,
		liveDB:     liveDB,
		sandboxDB:  sandboxDB,
		qrisEngine: qrisEngine,
		binanceEng: binanceEng,
		usdtEngine: usdtEngine,
		webhook:    webhook,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (w *WorkerManager) Start(ctx context.Context) {
	log.Println("[WorkerManager] Starting unified background workers...")

	// 1. Expiry Sweeper (5s)
	go w.runExpirySweeper(ctx)

	// 2. Webhook Retry Worker (10s)
	go w.runWebhookRetryWorker(ctx)

	// 3. ShopeePay QRIS Poller (5s)
	go w.runShopeePoller(ctx)

	// 4. Binance Pay Poller (20s)
	go w.runBinancePoller(ctx)

	// 5. USDT BEP-20 RPC Scanner (6s)
	go w.runUSDTScanner(ctx)

	// 6. Token Watchdog (15m)
	go w.runWatchdog(ctx)
}

func (w *WorkerManager) runExpirySweeper(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = w.liveDB.SweepExpiredOrders()
			if w.sandboxDB != nil {
				_, _ = w.sandboxDB.SweepExpiredOrders()
			}
		}
	}
}

func (w *WorkerManager) runWebhookRetryWorker(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.webhook.RunRetrySweep(w.liveDB)
			if w.sandboxDB != nil {
				w.webhook.RunRetrySweep(w.sandboxDB)
			}
		}
	}
}

func (w *WorkerManager) runShopeePoller(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check if any pending orders exist for QRIS
			pending, err := w.liveDB.GetPendingOrders(model.ChannelQRIS)
			if err != nil || len(pending) == 0 {
				continue
			}

			token, _ := w.liveDB.GetConfig("shopee_token")
			if strings.TrimSpace(token) == "" {
				continue
			}

			txs, err := w.qrisEngine.FetchShopeeTransactions(token)
			if err != nil {
				continue
			}

			for _, tx := range txs {
				if tx.Status != 1 && tx.Status != 3 {
					continue
				}
				amt, err := ParseIDRAmount(tx.Amount)
				if err != nil {
					continue
				}

				order, err := w.liveDB.FindMatchingOrder(model.ChannelQRIS, "IDR", amt, false)
				if err == nil && order != nil {
					if err := w.liveDB.MarkOrderPaid(order.ID, tx.TransactionID, "", "", 0); err == nil {
						log.Printf("[ShopeePoller] MATCH! Order %s marked PAID with TX %s", order.ID, tx.TransactionID)
						order.Status = model.StatusPaid
						order.TxID = tx.TransactionID
						order.PaidAt = time.Now().UnixMilli()
						go w.webhook.DispatchOrder(w.liveDB, order)
					}
				}
			}
		}
	}
}

func (w *WorkerManager) runBinancePoller(ctx context.Context) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pending, err := w.liveDB.GetPendingOrders(model.ChannelBinance)
			if err != nil || len(pending) == 0 {
				continue
			}

			// Hold-fire rule: wait until oldest pending order is > 30s old
			nowMs := time.Now().UnixMilli()
			hasMatureOrder := false
			for _, o := range pending {
				if nowMs-o.CreatedAt >= 30*1000 {
					hasMatureOrder = true
					break
				}
			}
			if !hasMatureOrder {
				continue
			}

			_, _ = w.binanceEng.ProcessIncomingTransactions()
		}
	}
}

func (w *WorkerManager) runUSDTScanner(ctx context.Context) {
	ticker := time.NewTicker(6 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pending, err := w.liveDB.GetPendingOrders(model.ChannelUSDT)
			if err != nil || len(pending) == 0 {
				continue
			}

			_, _ = w.usdtEngine.ScanOnChainTransfers(ctx)
		}
	}
}

func (w *WorkerManager) runWatchdog(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	w.checkShopeeTokenAge()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.checkShopeeTokenAge()
		}
	}
}

func (w *WorkerManager) checkShopeeTokenAge() {
	updatedAtStr, _ := w.liveDB.GetConfig("shopee_token_updated_at")
	if updatedAtStr == "" {
		return
	}
	t, err := time.Parse(time.RFC3339, updatedAtStr)
	if err != nil {
		return
	}

	age := time.Since(t)
	w.mu.Lock()
	defer w.mu.Unlock()

	botToken, _ := w.liveDB.GetConfig("telegram_bot_token")
	if botToken == "" {
		botToken = w.cfg.TelegramBotToken
	}
	chatID, _ := w.liveDB.GetConfig("telegram_chat_id")
	if chatID == "" {
		chatID = w.cfg.TelegramChatID
	}

	if botToken == "" || chatID == "" {
		return
	}

	if age >= 23*time.Hour && !w.criticalSent {
		w.criticalSent = true
		w.sendTelegram(botToken, chatID, fmt.Sprintf("🚨 *[CRITICAL] ShopeePay Token Expiring!* Usia token sudah %d jam. Segera perbarui di dashboard!", int(age.Hours())))
	} else if age >= 20*time.Hour && !w.alertSent {
		w.alertSent = true
		w.sendTelegram(botToken, chatID, fmt.Sprintf("⚠️ *[WARNING] ShopeePay Token Expiring Soon.* Usia token sudah %d jam. Mohon perbarui.", int(age.Hours())))
	} else if age < 2*time.Hour && (w.alertSent || w.criticalSent) {
		w.alertSent = false
		w.criticalSent = false
		w.sendTelegram(botToken, chatID, "✅ *[RECOVERY] ShopeePay Token Updated Successfully.*")
	}
}

func (w *WorkerManager) sendTelegram(botToken, chatID, message string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	body, _ := json.Marshal(map[string]string{
		"chat_id":    chatID,
		"text":       message,
		"parse_mode": "Markdown",
	})
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.httpClient.Do(req)
	if err == nil && resp != nil {
		resp.Body.Close()
	}
}
