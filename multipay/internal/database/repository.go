package database

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"multipay/internal/model"
)

// GenerateRandomID creates a prefixed random ID
func GenerateRandomID(prefix string) string {
	bytes := make([]byte, 8)
	_, _ = rand.Read(bytes)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(bytes))
}

// ==========================================
// SYSTEM CONFIG REPOSITORY
// ==========================================

func (d *DB) GetConfig(key string) (string, error) {
	var val string
	err := d.SqlDB.QueryRow("SELECT value FROM system_config WHERE key = ?", key).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return val, err
}

func (d *DB) SetConfig(key, value string) error {
	_, err := d.SqlDB.Exec(`
		INSERT INTO system_config (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
	`, key, value)
	return err
}

func (d *DB) GetAllConfig() (map[string]string, error) {
	rows, err := d.SqlDB.Query("SELECT key, value FROM system_config")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			res[k] = v
		}
	}
	return res, nil
}

// ==========================================
// APPS REPOSITORY
// ==========================================

func (d *DB) GetAppByAPIKey(apiKey string) (*model.App, error) {
	row := d.SqlDB.QueryRow("SELECT id, name, api_key, webhook_url, webhook_secret, is_active, created_at FROM apps WHERE api_key = ? AND is_active = 1", apiKey)
	var a model.App
	err := row.Scan(&a.ID, &a.Name, &a.APIKey, &a.WebhookURL, &a.WebhookSecret, &a.IsActive, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (d *DB) GetAppByID(id string) (*model.App, error) {
	row := d.SqlDB.QueryRow("SELECT id, name, api_key, webhook_url, webhook_secret, is_active, created_at FROM apps WHERE id = ?", id)
	var a model.App
	err := row.Scan(&a.ID, &a.Name, &a.APIKey, &a.WebhookURL, &a.WebhookSecret, &a.IsActive, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (d *DB) ListApps() ([]model.App, error) {
	rows, err := d.SqlDB.Query("SELECT id, name, api_key, webhook_url, webhook_secret, is_active, created_at FROM apps ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apps []model.App
	for rows.Next() {
		var a model.App
		if err := rows.Scan(&a.ID, &a.Name, &a.APIKey, &a.WebhookURL, &a.WebhookSecret, &a.IsActive, &a.CreatedAt); err == nil {
			apps = append(apps, a)
		}
	}
	return apps, nil
}

func (d *DB) CreateApp(name, webhookURL, webhookSecret string) (*model.App, error) {
	appID := GenerateRandomID("app")
	prefix := "ak_live"
	if d.IsSandbox {
		prefix = "ak_test"
	}
	apiKey := GenerateRandomID(prefix)
	if webhookSecret == "" {
		webhookSecret = GenerateRandomID("whsec")
	}

	_, err := d.SqlDB.Exec(
		"INSERT INTO apps (id, name, api_key, webhook_url, webhook_secret) VALUES (?, ?, ?, ?, ?)",
		appID, name, apiKey, webhookURL, webhookSecret,
	)
	if err != nil {
		return nil, err
	}
	return d.GetAppByID(appID)
}

func (d *DB) CreateAppWithExplicit(id, name, apiKey, webhookURL, webhookSecret string) (*model.App, error) {
	_, err := d.SqlDB.Exec(
		"INSERT INTO apps (id, name, api_key, webhook_url, webhook_secret) VALUES (?, ?, ?, ?, ?)",
		id, name, apiKey, webhookURL, webhookSecret,
	)
	if err != nil {
		return nil, err
	}
	return d.GetAppByID(id)
}

func (d *DB) DeleteApp(id string) error {
	_, err := d.SqlDB.Exec("DELETE FROM apps WHERE id = ?", id)
	return err
}

// ==========================================
// ORDERS REPOSITORY
// ==========================================

func scanOrder(row *sql.Row) (*model.Order, error) {
	var o model.Order
	var txID, txHash, fromInfo, meta sql.NullString
	var blockNum, paidAt, nextRetry sql.NullInt64

	err := row.Scan(
		&o.ID, &o.AppID, &o.Channel, &o.ReferenceID, &o.Currency, &o.Network, &o.WalletAddress,
		&o.OriginalAmount, &o.OriginalAmountMilli, &o.UniqueCode, &o.TotalAmount, &o.TotalAmountStr, &o.TotalAmountMilli,
		&o.Status, &txID, &txHash, &fromInfo, &blockNum, &o.QRContent, &o.QRPath,
		&o.CreatedAt, &o.ExpiresAt, &o.CooldownUntil, &paidAt,
		&o.WebhookDelivered, &o.WebhookAttempts, &nextRetry, &meta,
	)
	if err != nil {
		return nil, err
	}

	o.TxID = txID.String
	o.TxHash = txHash.String
	o.FromInfo = fromInfo.String
	o.BlockNumber = blockNum.Int64
	o.PaidAt = paidAt.Int64
	o.NextWebhookRetryAt = nextRetry.Int64
	o.Metadata = meta.String
	return &o, nil
}

func scanOrderFromRows(rows *sql.Rows) (*model.Order, error) {
	var o model.Order
	var txID, txHash, fromInfo, meta sql.NullString
	var blockNum, paidAt, nextRetry sql.NullInt64

	err := rows.Scan(
		&o.ID, &o.AppID, &o.Channel, &o.ReferenceID, &o.Currency, &o.Network, &o.WalletAddress,
		&o.OriginalAmount, &o.OriginalAmountMilli, &o.UniqueCode, &o.TotalAmount, &o.TotalAmountStr, &o.TotalAmountMilli,
		&o.Status, &txID, &txHash, &fromInfo, &blockNum, &o.QRContent, &o.QRPath,
		&o.CreatedAt, &o.ExpiresAt, &o.CooldownUntil, &paidAt,
		&o.WebhookDelivered, &o.WebhookAttempts, &nextRetry, &meta,
	)
	if err != nil {
		return nil, err
	}

	o.TxID = txID.String
	o.TxHash = txHash.String
	o.FromInfo = fromInfo.String
	o.BlockNumber = blockNum.Int64
	o.PaidAt = paidAt.Int64
	o.NextWebhookRetryAt = nextRetry.Int64
	o.Metadata = meta.String
	return &o, nil
}

func (d *DB) CreateOrder(o *model.Order) error {
	query := `
		INSERT INTO orders (
			id, app_id, channel, reference_id, currency, network, wallet_address,
			original_amount, original_amount_milli, unique_code, total_amount, total_amount_str, total_amount_milli,
			status, qr_content, qr_path, created_at, expires_at, cooldown_until, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := d.SqlDB.Exec(
		query,
		o.ID, o.AppID, o.Channel, o.ReferenceID, o.Currency, o.Network, o.WalletAddress,
		o.OriginalAmount, o.OriginalAmountMilli, o.UniqueCode, o.TotalAmount, o.TotalAmountStr, o.TotalAmountMilli,
		o.Status, o.QRContent, o.QRPath, o.CreatedAt, o.ExpiresAt, o.CooldownUntil, o.Metadata,
	)
	if err == nil {
		d.codeMu.Lock()
		if d.reservedCodes != nil {
			delete(d.reservedCodes, fmt.Sprintf("%s:%s:%d", o.Channel, o.Currency, o.TotalAmountMilli))
		}
		d.codeMu.Unlock()
	}
	return err
}

func (d *DB) GetOrder(id string) (*model.Order, error) {
	row := d.SqlDB.QueryRow(`
		SELECT id, app_id, channel, reference_id, currency, network, wallet_address,
		       original_amount, original_amount_milli, unique_code, total_amount, total_amount_str, total_amount_milli,
		       status, tx_id, tx_hash, from_info, block_number, qr_content, qr_path,
		       created_at, expires_at, cooldown_until, paid_at, webhook_delivered, webhook_attempts, next_webhook_retry_at, metadata
		FROM orders WHERE id = ?
	`, id)
	return scanOrder(row)
}

func (d *DB) GetOrderByAppAndRef(appID, refID string) (*model.Order, error) {
	row := d.SqlDB.QueryRow(`
		SELECT id, app_id, channel, reference_id, currency, network, wallet_address,
		       original_amount, original_amount_milli, unique_code, total_amount, total_amount_str, total_amount_milli,
		       status, tx_id, tx_hash, from_info, block_number, qr_content, qr_path,
		       created_at, expires_at, cooldown_until, paid_at, webhook_delivered, webhook_attempts, next_webhook_retry_at, metadata
		FROM orders WHERE app_id = ? AND reference_id = ? AND status = 'PENDING'
	`, appID, refID)
	return scanOrder(row)
}

func (d *DB) GetPendingOrders(channel string) ([]model.Order, error) {
	nowMs := time.Now().UnixMilli()
	query := `
		SELECT id, app_id, channel, reference_id, currency, network, wallet_address,
		       original_amount, original_amount_milli, unique_code, total_amount, total_amount_str, total_amount_milli,
		       status, tx_id, tx_hash, from_info, block_number, qr_content, qr_path,
		       created_at, expires_at, cooldown_until, paid_at, webhook_delivered, webhook_attempts, next_webhook_retry_at, metadata
		FROM orders WHERE status = 'PENDING' AND expires_at > ?
	`
	var args []interface{}
	args = append(args, nowMs)
	if channel != "" {
		query += " AND channel = ?"
		args = append(args, channel)
	}

	rows, err := d.SqlDB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []model.Order
	for rows.Next() {
		o, err := scanOrderFromRows(rows)
		if err == nil {
			orders = append(orders, *o)
		}
	}
	return orders, nil
}

func (d *DB) FindMatchingOrder(channel, currency string, amountMilli int64, allowRecentExpired bool) (*model.Order, error) {
	nowMs := time.Now().UnixMilli()
	var query string
	if allowRecentExpired {
		query = `
			SELECT id, app_id, channel, reference_id, currency, network, wallet_address,
			       original_amount, original_amount_milli, unique_code, total_amount, total_amount_str, total_amount_milli,
			       status, tx_id, tx_hash, from_info, block_number, qr_content, qr_path,
			       created_at, expires_at, cooldown_until, paid_at, webhook_delivered, webhook_attempts, next_webhook_retry_at, metadata
			FROM orders
			WHERE channel = ? AND currency = ? AND total_amount_milli = ?
			  AND (status = 'PENDING' OR (status = 'EXPIRED' AND cooldown_until > ?))
			ORDER BY created_at DESC LIMIT 1
		`
		row := d.SqlDB.QueryRow(query, channel, currency, amountMilli, nowMs)
		return scanOrder(row)
	}

	query = `
		SELECT id, app_id, channel, reference_id, currency, network, wallet_address,
		       original_amount, original_amount_milli, unique_code, total_amount, total_amount_str, total_amount_milli,
		       status, tx_id, tx_hash, from_info, block_number, qr_content, qr_path,
		       created_at, expires_at, cooldown_until, paid_at, webhook_delivered, webhook_attempts, next_webhook_retry_at, metadata
		FROM orders
		WHERE channel = ? AND currency = ? AND total_amount_milli = ? AND status = 'PENDING' AND expires_at > ?
		ORDER BY created_at DESC LIMIT 1
	`
	row := d.SqlDB.QueryRow(query, channel, currency, amountMilli, nowMs)
	return scanOrder(row)
}

func (d *DB) MarkOrderPaid(orderID, txID, txHash, fromInfo string, blockNum int64) error {
	nowMs := time.Now().UnixMilli()
	query := `
		UPDATE orders SET
			status = 'PAID',
			tx_id = ?,
			tx_hash = ?,
			from_info = ?,
			block_number = ?,
			paid_at = ?
		WHERE id = ? AND status != 'PAID'
	`
	_, err := d.SqlDB.Exec(query, txID, txHash, fromInfo, blockNum, nowMs, orderID)
	return err
}

func (d *DB) CancelOrder(orderID string) error {
	_, err := d.SqlDB.Exec("UPDATE orders SET status = 'CANCELLED' WHERE id = ? AND status = 'PENDING'", orderID)
	return err
}

func (d *DB) SweepExpiredOrders() (int64, error) {
	nowMs := time.Now().UnixMilli()
	res, err := d.SqlDB.Exec("UPDATE orders SET status = 'EXPIRED' WHERE status = 'PENDING' AND expires_at <= ?", nowMs)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (d *DB) GetOrdersForWebhookRetry(limit int) ([]model.Order, error) {
	nowMs := time.Now().UnixMilli()
	query := `
		SELECT id, app_id, channel, reference_id, currency, network, wallet_address,
		       original_amount, original_amount_milli, unique_code, total_amount, total_amount_str, total_amount_milli,
		       status, tx_id, tx_hash, from_info, block_number, qr_content, qr_path,
		       created_at, expires_at, cooldown_until, paid_at, webhook_delivered, webhook_attempts, next_webhook_retry_at, metadata
		FROM orders
		WHERE status = 'PAID' AND webhook_delivered = 0 AND (next_webhook_retry_at <= ? OR next_webhook_retry_at IS NULL) AND webhook_attempts < 5
		LIMIT ?
	`
	rows, err := d.SqlDB.Query(query, nowMs, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []model.Order
	for rows.Next() {
		o, err := scanOrderFromRows(rows)
		if err == nil {
			orders = append(orders, *o)
		}
	}
	return orders, nil
}

func (d *DB) UpdateWebhookStatus(orderID string, delivered bool, attempts int, nextRetryAt int64) error {
	val := 0
	if delivered {
		val = 1
	}
	_, err := d.SqlDB.Exec(
		"UPDATE orders SET webhook_delivered = ?, webhook_attempts = ?, next_webhook_retry_at = ? WHERE id = ?",
		val, attempts, nextRetryAt, orderID,
	)
	return err
}

// ==========================================
// UNIQUE CODE ALLOCATOR
// ==========================================

func (d *DB) AllocateUniqueCode(channel, currency string, baseAmountMilli int64) (int, int64, error) {
	d.codeMu.Lock()
	defer d.codeMu.Unlock()

	nowMs := time.Now().UnixMilli()

	if d.reservedCodes == nil {
		d.reservedCodes = make(map[string]int64)
	}
	// Evict expired in-flight entries (hold for 30s)
	for k, exp := range d.reservedCodes {
		if exp <= nowMs {
			delete(d.reservedCodes, k)
		}
	}

	if channel == model.ChannelQRIS {
		// QRIS: 1..999 IDR
		for i := 0; i < 50; i++ {
			n, _ := rand.Int(rand.Reader, big.NewInt(999))
			code := int(n.Int64()) + 1
			totalMilli := baseAmountMilli + int64(code) // for IDR, milli equals integer amount

			flightKey := fmt.Sprintf("%s:%s:%d", channel, currency, totalMilli)
			if exp, ok := d.reservedCodes[flightKey]; ok && exp > nowMs {
				continue
			}

			var exists int
			err := d.SqlDB.QueryRow(
				"SELECT COUNT(*) FROM orders WHERE channel = 'qris' AND total_amount_milli = ? AND status = 'PENDING' AND expires_at > ?",
				totalMilli, nowMs,
			).Scan(&exists)
			if err == nil && exists == 0 {
				d.reservedCodes[flightKey] = nowMs + 30000 // 30s in-flight hold
				return code, totalMilli, nil
			}
		}
		return 0, 0, errors.New("no unique code available for QRIS")
	}

	// Crypto (Binance & USDT): Adaptive micro-code in milli-units (0.001 - 0.099 USDT)
	maxCode := 25
	baseUSDT := float64(baseAmountMilli) / 1000.0
	if baseUSDT >= 50.0 {
		maxCode = 99
	} else if baseUSDT >= 5.0 {
		maxCode = 50
	}

	// Attempt within tier
	for i := 0; i < 60; i++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(maxCode)))
		code := int(n.Int64()) + 1
		totalMilli := baseAmountMilli + int64(code)

		flightKey := fmt.Sprintf("%s:%s:%d", channel, currency, totalMilli)
		if exp, ok := d.reservedCodes[flightKey]; ok && exp > nowMs {
			continue
		}

		var exists int
		err := d.SqlDB.QueryRow(
			"SELECT COUNT(*) FROM orders WHERE channel = ? AND currency = ? AND total_amount_milli = ? AND (status = 'PENDING' OR cooldown_until > ?)",
			channel, currency, totalMilli, nowMs,
		).Scan(&exists)
		if err == nil && exists == 0 {
			d.reservedCodes[flightKey] = nowMs + 30000 // 30s in-flight hold
			return code, totalMilli, nil
		}
	}

	// Emergency fallback expansion to 250 slots
	for i := 0; i < 100; i++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(250))
		code := int(n.Int64()) + 1
		totalMilli := baseAmountMilli + int64(code)

		flightKey := fmt.Sprintf("%s:%s:%d", channel, currency, totalMilli)
		if exp, ok := d.reservedCodes[flightKey]; ok && exp > nowMs {
			continue
		}

		var exists int
		err := d.SqlDB.QueryRow(
			"SELECT COUNT(*) FROM orders WHERE channel = ? AND currency = ? AND total_amount_milli = ? AND (status = 'PENDING' OR cooldown_until > ?)",
			channel, currency, totalMilli, nowMs,
		).Scan(&exists)
		if err == nil && exists == 0 {
			d.reservedCodes[flightKey] = nowMs + 30000 // 30s in-flight hold
			return code, totalMilli, nil
		}
	}

	return 0, 0, errors.New("unique code pool exhausted for crypto channel")
}

// ==========================================
// UNMATCHED TRANSACTIONS REPOSITORY
// ==========================================

func (d *DB) SaveUnmatchedTx(u *model.UnmatchedTx) error {
	query := `
		INSERT OR IGNORE INTO unmatched_transactions (
			id, channel, tx_hash, from_info, currency, amount_str, amount_milli, raw_payload, tx_time, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`
	_, err := d.SqlDB.Exec(
		query,
		u.ID, u.Channel, u.TxHash, u.FromInfo, u.Currency, u.AmountStr, u.AmountMilli, u.RawPayload, u.TxTime,
	)
	return err
}

func (d *DB) ListUnmatched(channel string, limit int) ([]model.UnmatchedTx, error) {
	query := "SELECT id, channel, tx_hash, from_info, currency, amount_str, amount_milli, raw_payload, tx_time, resolved_order_id, recorded_at FROM unmatched_transactions"
	var args []interface{}
	if channel != "" {
		query += " WHERE channel = ?"
		args = append(args, channel)
	}
	query += " ORDER BY recorded_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := d.SqlDB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.UnmatchedTx
	for rows.Next() {
		var u model.UnmatchedTx
		var txHash, fromInfo, rawPayload, resID sql.NullString
		if err := rows.Scan(&u.ID, &u.Channel, &txHash, &fromInfo, &u.Currency, &u.AmountStr, &u.AmountMilli, &rawPayload, &u.TxTime, &resID, &u.RecordedAt); err == nil {
			u.TxHash = txHash.String
			u.FromInfo = fromInfo.String
			u.RawPayload = rawPayload.String
			u.ResolvedOrderID = resID.String
			list = append(list, u)
		}
	}
	return list, nil
}

func (d *DB) ResolveUnmatched(unmatchedID, orderID string) error {
	tx, err := d.SqlDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var amountMilli int64
	var channel, txHash, fromInfo string
	err = tx.QueryRow(
		"SELECT channel, tx_hash, from_info, amount_milli FROM unmatched_transactions WHERE id = ?",
		unmatchedID,
	).Scan(&channel, &txHash, &fromInfo, &amountMilli)
	if err != nil {
		return fmt.Errorf("unmatched tx not found: %w", err)
	}

	var currentStatus string
	err = tx.QueryRow("SELECT status FROM orders WHERE id = ?", orderID).Scan(&currentStatus)
	if err != nil {
		return fmt.Errorf("order not found: %w", err)
	}
	if currentStatus == "PAID" {
		return errors.New("order is already paid")
	}

	nowMs := time.Now().UnixMilli()
	_, err = tx.Exec(`
		UPDATE orders SET status = 'PAID', tx_id = ?, tx_hash = ?, from_info = ?, paid_at = ? WHERE id = ?
	`, unmatchedID, txHash, fromInfo, nowMs, orderID)
	if err != nil {
		return err
	}

	_, err = tx.Exec("UPDATE unmatched_transactions SET resolved_order_id = ? WHERE id = ?", orderID, unmatchedID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// ==========================================
// ADMIN DASHBOARD STATS
// ==========================================

type ChannelStats struct {
	TotalVolume float64 `json:"total_volume"`
	PaidCount   int     `json:"paid_count"`
	PendingCount int    `json:"pending_count"`
	ExpiredCount int    `json:"expired_count"`
}

type GlobalStats struct {
	QRIS    ChannelStats `json:"qris"`
	Binance ChannelStats `json:"binance"`
	USDT    ChannelStats `json:"usdt"`
}

func (d *DB) GetStats() (*GlobalStats, error) {
	stats := &GlobalStats{}

	channels := []string{model.ChannelQRIS, model.ChannelBinance, model.ChannelUSDT}
	for _, ch := range channels {
		var cs ChannelStats
		// Sum volume of PAID orders
		_ = d.SqlDB.QueryRow("SELECT COALESCE(SUM(total_amount), 0) FROM orders WHERE channel = ? AND status = 'PAID'", ch).Scan(&cs.TotalVolume)
		_ = d.SqlDB.QueryRow("SELECT COUNT(*) FROM orders WHERE channel = ? AND status = 'PAID'", ch).Scan(&cs.PaidCount)
		_ = d.SqlDB.QueryRow("SELECT COUNT(*) FROM orders WHERE channel = ? AND status = 'PENDING'", ch).Scan(&cs.PendingCount)
		_ = d.SqlDB.QueryRow("SELECT COUNT(*) FROM orders WHERE channel = ? AND status = 'EXPIRED'", ch).Scan(&cs.ExpiredCount)

		switch ch {
		case model.ChannelQRIS:
			stats.QRIS = cs
		case model.ChannelBinance:
			stats.Binance = cs
		case model.ChannelUSDT:
			stats.USDT = cs
		}
	}
	return stats, nil
}

func (d *DB) ListOrders(channel, status string, limit, offset int) ([]model.Order, error) {
	query := `
		SELECT id, app_id, channel, reference_id, currency, network, wallet_address,
		       original_amount, original_amount_milli, unique_code, total_amount, total_amount_str, total_amount_milli,
		       status, tx_id, tx_hash, from_info, block_number, qr_content, qr_path,
		       created_at, expires_at, cooldown_until, paid_at, webhook_delivered, webhook_attempts, next_webhook_retry_at, metadata
		FROM orders
	`
	var conditions []string
	var args []interface{}

	if channel != "" {
		conditions = append(conditions, "channel = ?")
		args = append(args, channel)
	}
	if status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := d.SqlDB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.Order
	for rows.Next() {
		o, err := scanOrderFromRows(rows)
		if err == nil {
			list = append(list, *o)
		}
	}
	return list, nil
}
