package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"multipay/internal/config"
	"multipay/internal/database"
	"multipay/internal/model"
)

type WebhookDispatcher struct {
	cfg        *config.Config
	httpClient *http.Client
}

func NewWebhookDispatcher(cfg *config.Config) *WebhookDispatcher {
	return &WebhookDispatcher{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (w *WebhookDispatcher) sign(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func (w *WebhookDispatcher) DispatchOrder(db *database.DB, order *model.Order) {
	app, err := db.GetAppByID(order.AppID)
	if err != nil || app == nil || app.WebhookURL == "" {
		log.Printf("[Webhook] No webhook URL configured for app %s (order %s)", order.AppID, order.ID)
		return
	}

	payload := model.WebhookPayload{
		Event:          "payment.success",
		IsSandbox:      db.IsSandbox,
		Channel:        order.Channel,
		OrderID:        order.ID,
		ReferenceID:    order.ReferenceID,
		Currency:       order.Currency,
		OriginalAmount: order.OriginalAmount,
		UniqueCode:     order.UniqueCode,
		TotalAmount:    order.TotalAmount,
		TotalAmountStr: order.TotalAmountStr,
		PaidAt:         order.PaidAt,
		TxID:           order.TxID,
		TxHash:         order.TxHash,
		Timestamp:      time.Now().Unix(),
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return
	}

	secret := app.WebhookSecret
	if secret == "" {
		secret = app.APIKey
	}
	signature := w.sign(bodyBytes, secret)

	req, err := http.NewRequest("POST", app.WebhookURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "PaymentG-Multipay/2.0")
	req.Header.Set("X-Webhook-Event", "payment.success")
	req.Header.Set("X-Webhook-Signature", signature)
	req.Header.Set("X-PaymentG-Signature", signature)

	resp, err := w.httpClient.Do(req)
	delivered := err == nil && resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 300
	if resp != nil {
		resp.Body.Close()
	}

	attempts := order.WebhookAttempts + 1
	var nextRetry int64
	if !delivered && attempts < 5 {
		// Backoff delays: 10s, 30s, 60s, 300s, 900s
		delays := []time.Duration{10 * time.Second, 30 * time.Second, 60 * time.Second, 300 * time.Second, 900 * time.Second}
		delay := delays[attempts-1]
		nextRetry = time.Now().Add(delay).UnixMilli()
		log.Printf("[Webhook] Delivery failed for order %s to %s (attempt %d). Next retry in %v", order.ID, app.WebhookURL, attempts, delay)
	} else if delivered {
		log.Printf("[Webhook] Successfully delivered webhook for order %s to %s", order.ID, app.WebhookURL)
	}

	_ = db.UpdateWebhookStatus(order.ID, delivered, attempts, nextRetry)
}

func (w *WebhookDispatcher) RunRetrySweep(db *database.DB) {
	orders, err := db.GetOrdersForWebhookRetry(10)
	if err != nil || len(orders) == 0 {
		return
	}

	for _, o := range orders {
		orderCopy := o
		go w.DispatchOrder(db, &orderCopy)
	}
}
