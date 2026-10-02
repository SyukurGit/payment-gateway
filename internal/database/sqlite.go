package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
	"paymentg/internal/model"
)

type DB struct {
	db *sql.DB
}

func New(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(dataDir, "payment.db")
	
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}

	d := &DB{db: db}
	if err := d.migrate(); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *DB) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS apps (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			api_key TEXT UNIQUE NOT NULL,
			webhook_url TEXT NOT NULL,
			webhook_secret TEXT,
			is_active BOOLEAN DEFAULT 1,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS orders (
			id TEXT PRIMARY KEY,
			app_id TEXT NOT NULL,
			reference_id TEXT,
			original_amount INTEGER NOT NULL,
			unique_code INTEGER NOT NULL,
			total_amount INTEGER NOT NULL,
			status TEXT NOT NULL,
			qr_string TEXT,
			expires_at TEXT NOT NULL,
			paid_at TEXT,
			shopee_tx_id TEXT,
			webhook_sent BOOLEAN DEFAULT 0,
			webhook_attempts INTEGER DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			metadata TEXT,
			FOREIGN KEY(app_id) REFERENCES apps(id)
		);`,
		`CREATE TABLE IF NOT EXISTS processed_transactions (
			shopee_tx_id TEXT PRIMARY KEY,
			amount INTEGER NOT NULL,
			matched_order_id TEXT,
			processed_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);`,
		`CREATE INDEX IF NOT EXISTS idx_orders_total_amount ON orders(total_amount);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_orders_active_amount ON orders(total_amount) WHERE status = 'PENDING';`,
		`INSERT OR IGNORE INTO config (key, value, updated_at) VALUES ('dashboard_pin', '2207', datetime('now'));`,
	}

	for _, query := range queries {
		if _, err := d.db.Exec(query); err != nil {
			return err
		}
	}
	return nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

// App operations
func (d *DB) CreateApp(app *model.App) error {
	_, err := d.db.Exec(`INSERT INTO apps (id, name, api_key, webhook_url, webhook_secret, is_active, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		app.ID, app.Name, app.APIKey, app.WebhookURL, app.WebhookSecret, app.IsActive, app.CreatedAt)
	return err
}

func (d *DB) GetAppByAPIKey(apiKey string) (*model.App, error) {
	row := d.db.QueryRow(`SELECT id, name, api_key, webhook_url, webhook_secret, is_active, created_at FROM apps WHERE api_key = ?`, apiKey)
	var app model.App
	var secret sql.NullString
	err := row.Scan(&app.ID, &app.Name, &app.APIKey, &app.WebhookURL, &secret, &app.IsActive, &app.CreatedAt)
	if err != nil {
		return nil, err
	}
	app.WebhookSecret = secret.String
	return &app, nil
}

func (d *DB) GetAppByID(id string) (*model.App, error) {
	row := d.db.QueryRow(`SELECT id, name, api_key, webhook_url, webhook_secret, is_active, created_at FROM apps WHERE id = ?`, id)
	var app model.App
	var secret sql.NullString
	err := row.Scan(&app.ID, &app.Name, &app.APIKey, &app.WebhookURL, &secret, &app.IsActive, &app.CreatedAt)
	if err != nil {
		return nil, err
	}
	app.WebhookSecret = secret.String
	return &app, nil
}

func (d *DB) ListApps() ([]model.App, error) {
	rows, err := d.db.Query(`SELECT id, name, api_key, webhook_url, webhook_secret, is_active, created_at FROM apps`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apps []model.App
	for rows.Next() {
		var app model.App
		var secret sql.NullString
		if err := rows.Scan(&app.ID, &app.Name, &app.APIKey, &app.WebhookURL, &secret, &app.IsActive, &app.CreatedAt); err != nil {
			return nil, err
		}
		app.WebhookSecret = secret.String
		apps = append(apps, app)
	}
	return apps, nil
}

func (d *DB) DeleteApp(id string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM orders WHERE app_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM apps WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// Order operations
func (d *DB) CreateOrder(order *model.Order) error {
	_, err := d.db.Exec(`INSERT INTO orders (id, app_id, reference_id, original_amount, unique_code, total_amount, status, qr_string, expires_at, paid_at, shopee_tx_id, webhook_sent, webhook_attempts, created_at, updated_at, metadata) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		order.ID, order.AppID, order.ReferenceID, order.OriginalAmount, order.UniqueCode, order.TotalAmount, order.Status, order.QRString, order.ExpiresAt, order.PaidAt, order.ShopeeTxID, order.WebhookSent, order.WebhookAttempts, order.CreatedAt, order.UpdatedAt, order.Metadata)
	return err
}

func scanOrder(row *sql.Row) (*model.Order, error) {
	var order model.Order
	var refID, qrStr, shopeeTxID, metadata sql.NullString
	err := row.Scan(&order.ID, &order.AppID, &refID, &order.OriginalAmount, &order.UniqueCode, &order.TotalAmount, &order.Status, &qrStr, &order.ExpiresAt, &order.PaidAt, &shopeeTxID, &order.WebhookSent, &order.WebhookAttempts, &order.CreatedAt, &order.UpdatedAt, &metadata)
	if err != nil {
		return nil, err
	}
	order.ReferenceID = refID.String
	order.QRString = qrStr.String
	order.ShopeeTxID = shopeeTxID.String
	order.Metadata = metadata.String
	return &order, nil
}

func scanOrderFromRows(rows *sql.Rows) (*model.Order, error) {
	var order model.Order
	var refID, qrStr, shopeeTxID, metadata sql.NullString
	err := rows.Scan(&order.ID, &order.AppID, &refID, &order.OriginalAmount, &order.UniqueCode, &order.TotalAmount, &order.Status, &qrStr, &order.ExpiresAt, &order.PaidAt, &shopeeTxID, &order.WebhookSent, &order.WebhookAttempts, &order.CreatedAt, &order.UpdatedAt, &metadata)
	if err != nil {
		return nil, err
	}
	order.ReferenceID = refID.String
	order.QRString = qrStr.String
	order.ShopeeTxID = shopeeTxID.String
	order.Metadata = metadata.String
	return &order, nil
}


func (d *DB) GetOrder(id string) (*model.Order, error) {
	row := d.db.QueryRow(`SELECT id, app_id, reference_id, original_amount, unique_code, total_amount, status, qr_string, expires_at, paid_at, shopee_tx_id, webhook_sent, webhook_attempts, created_at, updated_at, metadata FROM orders WHERE id = ?`, id)
	return scanOrder(row)
}

func (d *DB) GetOrderByAppAndRef(appID, refID string) (*model.Order, error) {
	row := d.db.QueryRow(`SELECT id, app_id, reference_id, original_amount, unique_code, total_amount, status, qr_string, expires_at, paid_at, shopee_tx_id, webhook_sent, webhook_attempts, created_at, updated_at, metadata FROM orders WHERE app_id = ? AND reference_id = ?`, appID, refID)
	return scanOrder(row)
}

func (d *DB) GetPendingOrders() ([]model.Order, error) {
	rows, err := d.db.Query(`SELECT id, app_id, reference_id, original_amount, unique_code, total_amount, status, qr_string, expires_at, paid_at, shopee_tx_id, webhook_sent, webhook_attempts, created_at, updated_at, metadata FROM orders WHERE status = 'PENDING' AND expires_at >= ?`, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []model.Order
	for rows.Next() {
		order, err := scanOrderFromRows(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, *order)
	}
	return orders, nil
}

func (d *DB) GetPendingOrderByAmount(totalAmount int64) (*model.Order, error) {
	row := d.db.QueryRow(`SELECT id, app_id, reference_id, original_amount, unique_code, total_amount, status, qr_string, expires_at, paid_at, shopee_tx_id, webhook_sent, webhook_attempts, created_at, updated_at, metadata FROM orders WHERE status = 'PENDING' AND total_amount = ? AND expires_at >= ?`, totalAmount, time.Now().UTC().Format(time.RFC3339))
	return scanOrder(row)
}

func (d *DB) UpdateOrderStatus(id, status string) error {
	_, err := d.db.Exec(`UPDATE orders SET status = ?, updated_at = ? WHERE id = ?`, status, time.Now().UTC().Format(time.RFC3339), id)
	return err
}

func (d *DB) MarkOrderPaid(id, shopeeTxID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := d.db.Exec(`UPDATE orders SET status = ?, paid_at = ?, shopee_tx_id = ?, updated_at = ? WHERE id = ?`, model.StatusPaid, now, shopeeTxID, now, id)
	return err
}

func (d *DB) MarkOrderWebhookSent(id string) error {
	_, err := d.db.Exec(`UPDATE orders SET webhook_sent = 1, updated_at = ? WHERE id = ?`, time.Now().UTC().Format(time.RFC3339), id)
	return err
}

func (d *DB) IncrementWebhookAttempts(id string) error {
	_, err := d.db.Exec(`UPDATE orders SET webhook_attempts = webhook_attempts + 1, updated_at = ? WHERE id = ?`, time.Now().UTC().Format(time.RFC3339), id)
	return err
}

func (d *DB) ListAllOrders(limit, offset int) ([]model.Order, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := d.db.Query(`SELECT id, app_id, reference_id, original_amount, unique_code, total_amount, status, qr_string, expires_at, paid_at, shopee_tx_id, webhook_sent, webhook_attempts, created_at, updated_at, metadata FROM orders ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []model.Order
	for rows.Next() {
		order, err := scanOrderFromRows(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, *order)
	}
	return orders, nil
}

func (d *DB) GetStats() (map[string]interface{}, error) {
	var totalOrders, paidOrders, pendingOrders, expiredOrders int
	var totalRevenue sql.NullInt64

	_ = d.db.QueryRow(`SELECT COUNT(*) FROM orders`).Scan(&totalOrders)
	_ = d.db.QueryRow(`SELECT COUNT(*) FROM orders WHERE status = 'PAID'`).Scan(&paidOrders)
	_ = d.db.QueryRow(`SELECT COUNT(*) FROM orders WHERE status = 'PENDING'`).Scan(&pendingOrders)
	_ = d.db.QueryRow(`SELECT COUNT(*) FROM orders WHERE status = 'EXPIRED'`).Scan(&expiredOrders)
	_ = d.db.QueryRow(`SELECT SUM(total_amount) FROM orders WHERE status = 'PAID'`).Scan(&totalRevenue)

	rev := int64(0)
	if totalRevenue.Valid {
		rev = totalRevenue.Int64
	}

	return map[string]interface{}{
		"total_orders":   totalOrders,
		"paid_orders":    paidOrders,
		"pending_orders": pendingOrders,
		"expired_orders": expiredOrders,
		"total_revenue":  rev,
	}, nil
}

func (d *DB) ExpireOrders() (int, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := d.db.Exec(`UPDATE orders SET status = ?, updated_at = ? WHERE status = ? AND expires_at < ?`, model.StatusExpired, now, model.StatusPending, now)
	if err != nil {
		return 0, err
	}
	affected, err := res.RowsAffected()
	return int(affected), err
}

func (d *DB) IsAmountInUse(totalAmount int64) (bool, error) {
	var count int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM orders WHERE status = 'PENDING' AND total_amount = ? AND expires_at >= ?`, totalAmount, time.Now().UTC().Format(time.RFC3339)).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (d *DB) CountPendingOrders() (int, error) {
	var count int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM orders WHERE status = 'PENDING' AND expires_at >= ?`, time.Now().UTC().Format(time.RFC3339)).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// Processed transactions
func (d *DB) IsTransactionProcessed(txID string) (bool, error) {
	var count int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM processed_transactions WHERE shopee_tx_id = ?`, txID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (d *DB) SaveProcessedTransaction(txID string, amount int64, orderID string) error {
	_, err := d.db.Exec(`INSERT INTO processed_transactions (shopee_tx_id, amount, matched_order_id, processed_at) VALUES (?, ?, ?, ?)`, txID, amount, orderID, time.Now().UTC().Format(time.RFC3339))
	return err
}

// Config
func (d *DB) GetConfig(key string) (string, error) {
	var value string
	err := d.db.QueryRow(`SELECT value FROM config WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (d *DB) SetConfig(key, value string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := d.db.Exec(`INSERT INTO config (key, value, updated_at) VALUES (?, ?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`, key, value, now)
	return err
}

func (d *DB) VerifyPIN(pin string) bool {
	var storedPin string
	err := d.db.QueryRow(`SELECT value FROM config WHERE key = 'dashboard_pin'`).Scan(&storedPin)
	if err != nil || storedPin == "" {
		return pin == "2207"
	}
	return pin == storedPin
}
