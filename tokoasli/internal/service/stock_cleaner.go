package service

import (
	"context"
	"log"
	"time"

	"tokoasli/internal/database"
)

type StockCleaner struct {
	db       *database.DB
	sseHub   *SSEHub
	interval time.Duration
}

func NewStockCleaner(db *database.DB, sseHub *SSEHub) *StockCleaner {
	return &StockCleaner{
		db:       db,
		sseHub:   sseHub,
		interval: 30 * time.Second,
	}
}

// Start begins the background loop releasing held stocks and marking expired orders
func (s *StockCleaner) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 1. Release expired held stocks back to pool
			releasedStocks, err := s.db.ReleaseExpiredStocks()
			if err != nil {
				log.Printf("[StockCleaner] Error releasing expired stocks: %v", err)
			}

			// 2. Broadcast EXPIRED event to SSE subscribers before marking them in DB
			if s.sseHub != nil {
				if expiredIDs, err := s.db.GetExpiredPendingOrderIDs(); err == nil && len(expiredIDs) > 0 {
					for _, id := range expiredIDs {
						s.sseHub.Broadcast(id, map[string]interface{}{
							"order_id": id,
							"status":   "EXPIRED",
						})
					}
				}
			}

			// 3. Mark pending orders as EXPIRED
			expiredOrders, err := s.db.ExpirePendingOrders()
			if err != nil {
				log.Printf("[StockCleaner] Error expiring pending orders: %v", err)
			}

			if releasedStocks > 0 || expiredOrders > 0 {
				log.Printf("[StockCleaner] Berhasil melepas %d stok kedaluwarsa & menandai %d pesanan expired", releasedStocks, expiredOrders)
			}
		}
	}
}
