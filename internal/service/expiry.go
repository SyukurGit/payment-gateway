package service

import (
	"context"
	"log"
	"time"

	"paymentg/internal/database"
)

type ExpiryService struct {
	db *database.DB
}

func NewExpiryService(db *database.DB) *ExpiryService {
	return &ExpiryService{db: db}
}

func (e *ExpiryService) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count, err := e.db.ExpireOrders()
			if err != nil {
				log.Printf("Expiry check error: %v", err)
			} else if count > 0 {
				log.Printf("Info: expired %d orders", count)
			}
		}
	}
}
