package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"paymentg/internal/database"
	"paymentg/internal/model"
)

type WebhookService struct {
	db         *database.DB
	httpClient *http.Client
}

func NewWebhookService(db *database.DB) *WebhookService {
	return &WebhookService{
		db:         db,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (w *WebhookService) Send(app *model.App, order *model.Order) error {
	paidAt := ""
	if order.PaidAt != nil {
		paidAt = *order.PaidAt
	}

	payload := model.WebhookPayload{
		Event:          "payment.success",
		OrderID:        order.ID,
		ReferenceID:    order.ReferenceID,
		OriginalAmount: order.OriginalAmount,
		UniqueCode:     order.UniqueCode,
		TotalAmount:    order.TotalAmount,
		PaidAt:         paidAt,
		ShopeeTxID:     order.ShopeeTxID,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", app.WebhookURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Event", "payment.success")

	resp, err := w.httpClient.Do(req)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		w.db.IncrementWebhookAttempts(order.ID)
		if err != nil {
			log.Printf("Webhook send error for order %s: %v", order.ID, err)
			return err
		}
		log.Printf("Webhook non-2xx status %d for order %s", resp.StatusCode, order.ID)
		return fmt.Errorf("non-2xx status: %d", resp.StatusCode)
	}

	w.db.MarkOrderWebhookSent(order.ID)
	log.Printf("Webhook sent successfully for order %s", order.ID)
	return nil
}
