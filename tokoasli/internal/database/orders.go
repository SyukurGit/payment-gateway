package database

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"tokoasli/internal/model"
)

// CreateOrder inserts a new customer order
func (d *DB) CreateOrder(o *model.StoreOrder) error {
	now := time.Now().UTC().Format(time.RFC3339)
	o.CreatedAt = now

	query := `INSERT INTO store_orders (
		id, secret_token, product_id, product_name, customer_email, customer_phone,
		original_price, unique_code, total_amount, status,
		delivery_content, paymentg_order_id, qr_string, qr_url,
		client_ip, created_at, paid_at, expires_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := d.db.Exec(query,
		o.ID, o.SecretToken, o.ProductID, o.ProductName, o.CustomerEmail, o.CustomerPhone,
		o.OriginalPrice, o.UniqueCode, o.TotalAmount, o.Status,
		o.DeliveryContent, o.PaymentGOrderID, o.QRString, o.QRURL,
		o.ClientIP, o.CreatedAt, o.PaidAt, o.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert order: %w", err)
	}
	return nil
}

// GetOrder fetches an order by its ID
func (d *DB) GetOrder(id string) (*model.StoreOrder, error) {
	query := `SELECT id, secret_token, product_id, product_name, customer_email, customer_phone,
					 original_price, unique_code, total_amount, status,
					 delivery_content, paymentg_order_id, qr_string, qr_url,
					 client_ip, created_at, paid_at, expires_at
			  FROM store_orders WHERE id = ?`

	row := d.db.QueryRow(query, id)
	return scanOrder(row)
}

// GetOrderBySecret fetches an order by ID and validates its secret token
func (d *DB) GetOrderBySecret(id, secretToken string) (*model.StoreOrder, error) {
	query := `SELECT id, secret_token, product_id, product_name, customer_email, customer_phone,
					 original_price, unique_code, total_amount, status,
					 delivery_content, paymentg_order_id, qr_string, qr_url,
					 client_ip, created_at, paid_at, expires_at
			  FROM store_orders WHERE id = ? AND secret_token = ?`

	row := d.db.QueryRow(query, id, secretToken)
	return scanOrder(row)
}

// TrackOrder finds an order by ID and case-insensitive customer email
func (d *DB) TrackOrder(orderID, customerEmail string) (*model.StoreOrder, error) {
	query := `SELECT id, secret_token, product_id, product_name, customer_email, customer_phone,
					 original_price, unique_code, total_amount, status,
					 delivery_content, paymentg_order_id, qr_string, qr_url,
					 client_ip, created_at, paid_at, expires_at
			  FROM store_orders
			  WHERE id = ? AND LOWER(customer_email) = LOWER(?)`

	row := d.db.QueryRow(query, strings.TrimSpace(orderID), strings.TrimSpace(customerEmail))
	return scanOrder(row)
}

// MarkOrderPaid updates status to PAID, stores delivery content, and sets paid_at timestamp
func (d *DB) MarkOrderPaid(id string, deliveryContent string, paymentgOrderID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	query := `UPDATE store_orders
			  SET status = ?, delivery_content = ?, paymentg_order_id = ?, paid_at = ?
			  WHERE id = ?`

	res, err := d.db.Exec(query, model.OrderStatusPaid, deliveryContent, paymentgOrderID, now, id)
	if err != nil {
		return fmt.Errorf("failed to mark order paid: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New("order not found")
	}
	return nil
}

// CancelOrder marks a PENDING order as CANCELLED
func (d *DB) CancelOrder(id string) error {
	query := `UPDATE store_orders SET status = ? WHERE id = ? AND status = ?`
	res, err := d.db.Exec(query, model.OrderStatusCancelled, id, model.OrderStatusPending)
	if err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New("order cannot be cancelled or is not in PENDING status")
	}
	return nil
}

// CountPendingOrdersByIP counts how many active PENDING orders exist for this IP using UTC comparison
func (d *DB) CountPendingOrdersByIP(clientIP string) (int, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	var count int
	query := `SELECT COUNT(*) FROM store_orders WHERE client_ip = ? AND status = ? AND expires_at > ?`
	err := d.db.QueryRow(query, clientIP, model.OrderStatusPending, now).Scan(&count)
	return count, err
}

// CountOrders returns total count of orders matching status filter
func (d *DB) CountOrders(statusFilter string) (int, error) {
	var count int
	var err error
	if statusFilter != "" && statusFilter != "ALL" {
		query := `SELECT COUNT(*) FROM store_orders WHERE status = ?`
		err = d.db.QueryRow(query, statusFilter).Scan(&count)
	} else {
		query := `SELECT COUNT(*) FROM store_orders`
		err = d.db.QueryRow(query).Scan(&count)
	}
	return count, err
}

// ListOrders returns orders with optional status filter for admin pagination
func (d *DB) ListOrders(statusFilter string, limit, offset int) ([]model.StoreOrder, error) {
	var rows *sql.Rows
	var err error

	if statusFilter != "" && statusFilter != "ALL" {
		query := `SELECT id, secret_token, product_id, product_name, customer_email, customer_phone,
						 original_price, unique_code, total_amount, status,
						 delivery_content, paymentg_order_id, qr_string, qr_url,
						 client_ip, created_at, paid_at, expires_at
				  FROM store_orders WHERE status = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`
		rows, err = d.db.Query(query, statusFilter, limit, offset)
	} else {
		query := `SELECT id, secret_token, product_id, product_name, customer_email, customer_phone,
						 original_price, unique_code, total_amount, status,
						 delivery_content, paymentg_order_id, qr_string, qr_url,
						 client_ip, created_at, paid_at, expires_at
				  FROM store_orders ORDER BY created_at DESC LIMIT ? OFFSET ?`
		rows, err = d.db.Query(query, limit, offset)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list orders: %w", err)
	}
	defer rows.Close()

	var orders []model.StoreOrder
	for rows.Next() {
		o, scanErr := scanOrderRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		orders = append(orders, *o)
	}
	return orders, nil
}

// ExportOrders fetches all orders matching status filter without pagination limits (for CSV export)
func (d *DB) ExportOrders(statusFilter string) ([]model.StoreOrder, error) {
	var rows *sql.Rows
	var err error

	if statusFilter != "" && statusFilter != "ALL" {
		query := `SELECT id, secret_token, product_id, product_name, customer_email, customer_phone,
						 original_price, unique_code, total_amount, status,
						 delivery_content, paymentg_order_id, qr_string, qr_url,
						 client_ip, created_at, paid_at, expires_at
				  FROM store_orders WHERE status = ? ORDER BY created_at DESC`
		rows, err = d.db.Query(query, statusFilter)
	} else {
		query := `SELECT id, secret_token, product_id, product_name, customer_email, customer_phone,
						 original_price, unique_code, total_amount, status,
						 delivery_content, paymentg_order_id, qr_string, qr_url,
						 client_ip, created_at, paid_at, expires_at
				  FROM store_orders ORDER BY created_at DESC`
		rows, err = d.db.Query(query)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to export orders: %w", err)
	}
	defer rows.Close()

	var orders []model.StoreOrder
	for rows.Next() {
		o, scanErr := scanOrderRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		orders = append(orders, *o)
	}
	return orders, nil
}

// GetAdminStats calculates total revenue, paid orders, pending orders, and total products
func (d *DB) GetAdminStats() (*model.AdminStatsResponse, error) {
	stats := &model.AdminStatsResponse{}
	now := time.Now().UTC().Format(time.RFC3339)

	// Total revenue
	var rev sql.NullInt64
	_ = d.db.QueryRow(`SELECT SUM(total_amount) FROM store_orders WHERE status = 'PAID'`).Scan(&rev)
	if rev.Valid {
		stats.TotalRevenue = rev.Int64
	}

	// Total paid orders
	_ = d.db.QueryRow(`SELECT COUNT(*) FROM store_orders WHERE status = 'PAID'`).Scan(&stats.TotalPaidOrders)

	// Total pending orders (active, not expired)
	_ = d.db.QueryRow(`SELECT COUNT(*) FROM store_orders WHERE status = 'PENDING' AND expires_at > ?`, now).Scan(&stats.TotalPendingOrders)

	// Total products
	_ = d.db.QueryRow(`SELECT COUNT(*) FROM products`).Scan(&stats.TotalProducts)

	return stats, nil
}

// GetExpiredPendingOrderIDs returns IDs of pending orders whose expires_at is before now
func (d *DB) GetExpiredPendingOrderIDs() ([]string, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	rows, err := d.db.Query("SELECT id FROM store_orders WHERE status = ? AND expires_at < ?", model.OrderStatusPending, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// ExpirePendingOrders marks expired orders as EXPIRED
func (d *DB) ExpirePendingOrders() (int, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := d.db.Exec("UPDATE store_orders SET status = ? WHERE status = ? AND expires_at < ?", model.OrderStatusExpired, model.OrderStatusPending, now)
	if err != nil {
		return 0, fmt.Errorf("failed to expire orders: %w", err)
	}
	rows, _ := res.RowsAffected()
	return int(rows), nil
}

func scanOrder(row *sql.Row) (*model.StoreOrder, error) {
	var o model.StoreOrder
	var customerPhone, deliveryContent, paymentgOrderID, qrString, qrURL, paidAt sql.NullString

	err := row.Scan(
		&o.ID, &o.SecretToken, &o.ProductID, &o.ProductName, &o.CustomerEmail, &customerPhone,
		&o.OriginalPrice, &o.UniqueCode, &o.TotalAmount, &o.Status,
		&deliveryContent, &paymentgOrderID, &qrString, &qrURL,
		&o.ClientIP, &o.CreatedAt, &paidAt, &o.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("order not found")
		}
		return nil, fmt.Errorf("failed to scan order: %w", err)
	}

	if customerPhone.Valid {
		o.CustomerPhone = customerPhone.String
	}
	if deliveryContent.Valid {
		o.DeliveryContent = deliveryContent.String
	}
	if paymentgOrderID.Valid {
		o.PaymentGOrderID = paymentgOrderID.String
	}
	if qrString.Valid {
		o.QRString = qrString.String
	}
	if qrURL.Valid {
		o.QRURL = qrURL.String
	}
	if paidAt.Valid {
		p := paidAt.String
		o.PaidAt = &p
	}

	return &o, nil
}

func scanOrderRow(rows *sql.Rows) (*model.StoreOrder, error) {
	var o model.StoreOrder
	var customerPhone, deliveryContent, paymentgOrderID, qrString, qrURL, paidAt sql.NullString

	err := rows.Scan(
		&o.ID, &o.SecretToken, &o.ProductID, &o.ProductName, &o.CustomerEmail, &customerPhone,
		&o.OriginalPrice, &o.UniqueCode, &o.TotalAmount, &o.Status,
		&deliveryContent, &paymentgOrderID, &qrString, &qrURL,
		&o.ClientIP, &o.CreatedAt, &paidAt, &o.ExpiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan order row: %w", err)
	}

	if customerPhone.Valid {
		o.CustomerPhone = customerPhone.String
	}
	if deliveryContent.Valid {
		o.DeliveryContent = deliveryContent.String
	}
	if paymentgOrderID.Valid {
		o.PaymentGOrderID = paymentgOrderID.String
	}
	if qrString.Valid {
		o.QRString = qrString.String
	}
	if qrURL.Valid {
		o.QRURL = qrURL.String
	}
	if paidAt.Valid {
		p := paidAt.String
		o.PaidAt = &p
	}

	return &o, nil
}
