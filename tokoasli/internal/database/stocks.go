package database

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"tokoasli/internal/model"
)

// AddStocks inserts multiple stock lines into product_stocks table
func (d *DB) AddStocks(productID string, lines []string) (int, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339)
	stmt, err := tx.Prepare("INSERT INTO product_stocks (product_id, content, is_used, created_at) VALUES (?, ?, 0, ?)")
	if err != nil {
		return 0, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	count := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if _, err := stmt.Exec(productID, trimmed, now); err != nil {
			return 0, fmt.Errorf("failed to insert stock line: %w", err)
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit stock transaction: %w", err)
	}
	return count, nil
}

// CountAvailableStock returns available (unreserved) stock count for a product
func (d *DB) CountAvailableStock(productID string) (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM product_stocks WHERE product_id = ? AND is_used = 0", productID).Scan(&count)
	return count, err
}

// HoldStock reserves 1 stock unit for a pending order for specified minutes
func (d *DB) HoldStock(productID string, orderID string, holdDurationMinutes int) (*model.ProductStock, error) {
	heldUntil := time.Now().UTC().Add(time.Duration(holdDurationMinutes) * time.Minute).Format(time.RFC3339)

	for attempt := 0; attempt < 3; attempt++ {
		tx, err := d.db.Begin()
		if err != nil {
			return nil, fmt.Errorf("failed to begin hold transaction: %w", err)
		}

		var stockID int64
		var content string
		row := tx.QueryRow("SELECT id, content FROM product_stocks WHERE product_id = ? AND is_used = 0 LIMIT 1", productID)
		if err := row.Scan(&stockID, &content); err != nil {
			_ = tx.Rollback()
			if errors.Is(err, sql.ErrNoRows) {
				return nil, errors.New("stok produk habis")
			}
			return nil, fmt.Errorf("failed to scan available stock: %w", err)
		}

		res, err := tx.Exec("UPDATE product_stocks SET is_used = 2, order_id = ?, held_until = ? WHERE id = ? AND is_used = 0", orderID, heldUntil, stockID)
		if err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("failed to hold stock: %w", err)
		}
		rows, _ := res.RowsAffected()
		if rows == 0 {
			_ = tx.Rollback()
			continue
		}

		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("failed to commit hold transaction: %w", err)
		}

		return &model.ProductStock{
			ID:        stockID,
			ProductID: productID,
			Content:   content,
			IsUsed:    model.StockHeld,
			OrderID:   &orderID,
			HeldUntil: &heldUntil,
		}, nil
	}

	return nil, errors.New("stok produk baru saja diambil pembeli lain")
}

// CommitStock marks the held stock as permanently sold (is_used = 1) and returns delivery content
func (d *DB) CommitStock(orderID string, productID string, deliveryType string, fixedContent string) (string, error) {
	if deliveryType == model.DeliveryTypeFixed {
		return fixedContent, nil
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// 0. Idempotency guard: If this order already has a committed stock row, return it immediately
	var alreadySoldContent string
	if err := d.db.QueryRow("SELECT content FROM product_stocks WHERE order_id = ? AND is_used = 1 LIMIT 1", orderID).Scan(&alreadySoldContent); err == nil && alreadySoldContent != "" {
		return alreadySoldContent, nil
	}

	// 1. Check if this order has a held stock row
	var stockID int64
	var content string
	err := d.db.QueryRow("SELECT id, content FROM product_stocks WHERE order_id = ? AND is_used = 2 LIMIT 1", orderID).Scan(&stockID, &content)
	if err == nil {
		// Mark as sold
		_, updateErr := d.db.Exec("UPDATE product_stocks SET is_used = 1, used_at = ? WHERE id = ?", now, stockID)
		if updateErr != nil {
			return "", fmt.Errorf("failed to mark stock as sold: %w", updateErr)
		}
		return content, nil
	}

	// 2. Fallback edge case: if held stock expired just before webhook arrived, try to grab any available stock
	err = d.db.QueryRow("SELECT id, content FROM product_stocks WHERE product_id = ? AND is_used = 0 LIMIT 1", productID).Scan(&stockID, &content)
	if err == nil {
		_, updateErr := d.db.Exec("UPDATE product_stocks SET is_used = 1, order_id = ?, used_at = ? WHERE id = ?", orderID, now, stockID)
		if updateErr == nil {
			return content, nil
		}
	}

	return "STOK KOSONG (Silakan hubungi admin dengan nomor invoice Anda)", nil
}

// ReleaseExpiredStocks releases all stocks that were held past their expiration time
func (d *DB) ReleaseExpiredStocks() (int, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := d.db.Exec("UPDATE product_stocks SET is_used = 0, order_id = NULL, held_until = NULL WHERE is_used = 2 AND held_until < ?", now)
	if err != nil {
		return 0, fmt.Errorf("failed to release expired stocks: %w", err)
	}
	rows, _ := res.RowsAffected()
	return int(rows), nil
}

// ReleaseHeldStockForOrder immediately releases the held stock for a specific order (e.g. if checkout failed or cancelled)
func (d *DB) ReleaseHeldStockForOrder(orderID string) error {
	_, err := d.db.Exec("UPDATE product_stocks SET is_used = 0, order_id = NULL, held_until = NULL WHERE order_id = ? AND is_used = 2", orderID)
	return err
}

// GetProductStockSummary gets count breakdown and items for a product
func (d *DB) GetProductStockSummary(productID string) (*model.ProductStockSummary, error) {
	summary := &model.ProductStockSummary{
		ProductID: productID,
		Items:     []model.StockItemDetail{},
	}

	var name string
	_ = d.db.QueryRow("SELECT name FROM products WHERE id = ?", productID).Scan(&name)
	summary.ProductName = name

	_ = d.db.QueryRow("SELECT COUNT(*) FROM product_stocks WHERE product_id = ? AND is_used = 0", productID).Scan(&summary.AvailableCount)
	_ = d.db.QueryRow("SELECT COUNT(*) FROM product_stocks WHERE product_id = ? AND is_used = 2", productID).Scan(&summary.HeldCount)
	_ = d.db.QueryRow("SELECT COUNT(*) FROM product_stocks WHERE product_id = ? AND is_used = 1", productID).Scan(&summary.SoldCount)
	summary.TotalCount = summary.AvailableCount + summary.HeldCount + summary.SoldCount

	// Get latest 100 items for detail inspection
	rows, err := d.db.Query(`SELECT id, product_id, content, is_used, order_id, held_until, created_at, used_at
		FROM product_stocks WHERE product_id = ? ORDER BY id DESC LIMIT 100`, productID)
	if err != nil {
		return summary, nil
	}
	defer rows.Close()

	for rows.Next() {
		var item model.StockItemDetail
		var orderID, heldUntil, usedAt sql.NullString
		if err := rows.Scan(&item.ID, &item.ProductID, &item.Content, &item.IsUsed, &orderID, &heldUntil, &item.CreatedAt, &usedAt); err == nil {
			if orderID.Valid {
				item.OrderID = &orderID.String
			}
			if heldUntil.Valid {
				item.HeldUntil = &heldUntil.String
			}
			if usedAt.Valid {
				item.UsedAt = &usedAt.String
			}
			switch item.IsUsed {
			case model.StockAvailable:
				item.Status = "AVAILABLE"
			case model.StockHeld:
				item.Status = "HELD"
			case model.StockSold:
				item.Status = "SOLD"
			default:
				item.Status = "UNKNOWN"
			}
			summary.Items = append(summary.Items, item)
		}
	}

	return summary, nil
}

// DeleteStockItem deletes an unused stock row (is_used = 0 only)
func (d *DB) DeleteStockItem(stockID int64) error {
	res, err := d.db.Exec("DELETE FROM product_stocks WHERE id = ? AND is_used = 0", stockID)
	if err != nil {
		return fmt.Errorf("failed to delete stock item: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New("stok tidak dapat dihapus (mungkin sedang di-hold atau sudah terjual)")
	}
	return nil
}
