package service

import (
	"log"
	"time"

	"paymentg/internal/database"
	"paymentg/internal/model"
)

type Matcher struct {
	db      *database.DB
	webhook *WebhookService
}

func NewMatcher(db *database.DB, webhook *WebhookService) *Matcher {
	return &Matcher{
		db:      db,
		webhook: webhook,
	}
}

func (m *Matcher) MatchTransactions(transactions []model.ShopeeTransaction) (int, error) {
	matches := 0
	for _, tx := range transactions {
		// In ShopeePay Partner, status 1 and 3 are valid completed payments
		if tx.Status != 1 && tx.Status != 3 {
			continue
		}
		processed, err := m.db.IsTransactionProcessed(tx.TransactionID)
		if err != nil || processed {
			continue
		}

		amount, err := ParseAmount(tx.Amount)
		if err != nil {
			log.Printf("[Matcher] Error parsing amount %s: %v", tx.Amount, err)
			continue
		}

		log.Printf("[Matcher] Checking incoming ShopeePay TX %s: Amount=Rp %d, Status=%d", tx.TransactionID, amount, tx.Status)

		order, err := m.db.GetPendingOrderByAmount(amount)
		if err == nil && order != nil {
			// Timing check: transaction create time should not be older than order creation (allow 60s clock skew)
			if tx.CreateTime > 0 {
				orderTime, parseErr := time.Parse(time.RFC3339, order.CreatedAt)
				if parseErr == nil && tx.CreateTime < (orderTime.Unix()-60) {
					log.Printf("[Matcher] Skipping TX %s (created %d): older than order %s created time %d", tx.TransactionID, tx.CreateTime, order.ID, orderTime.Unix())
					m.db.SaveProcessedTransaction(tx.TransactionID, amount, "")
					continue
				}
			}

			log.Printf("[Matcher] MATCH FOUND! Order %s matched with TX %s (Amount: Rp %d)", order.ID, tx.TransactionID, amount)
			err = m.db.MarkOrderPaid(order.ID, tx.TransactionID)
			if err != nil {
				log.Printf("[Matcher] Error marking order %s paid: %v", order.ID, err)
				continue
			}
			m.db.SaveProcessedTransaction(tx.TransactionID, amount, order.ID)

			app, err := m.db.GetAppByID(order.AppID)
			if err == nil && app != nil {
				updatedOrder, _ := m.db.GetOrder(order.ID)
				if updatedOrder != nil {
					go m.webhook.Send(app, updatedOrder)
				}
			}
			matches++
		} else {
			// No matching pending order for this transaction. Save as processed to prevent
			// phantom matching against future orders created with the same amount.
			m.db.SaveProcessedTransaction(tx.TransactionID, amount, "")
		}
	}
	return matches, nil
}
