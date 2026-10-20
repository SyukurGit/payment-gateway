package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
	"multipay/internal/config"
)

type DB struct {
	SqlDB         *sql.DB
	DataDir       string
	IsSandbox     bool
	codeMu        sync.Mutex
	reservedCodes map[string]int64 // in-flight code reservation cache
}

func InitDB(dbFilePath string, isSandbox bool, cfg *config.Config) (*DB, error) {
	dataDir := filepath.Dir(dbFilePath)
	return New(dataDir, isSandbox, cfg)
}

func New(dataDir string, isSandbox bool, cfg *config.Config) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	qrDir := filepath.Join(dataDir, "qr")
	if err := os.MkdirAll(qrDir, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(dataDir, "payment.db")
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", dbPath)

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	// SQLite WAL mode with 1 open connection is optimal for embedded concurrency
	sqlDB.SetMaxOpenConns(1)

	d := &DB{
		SqlDB:         sqlDB,
		DataDir:       dataDir,
		IsSandbox:     isSandbox,
		reservedCodes: make(map[string]int64),
	}

	if err := d.migrate(); err != nil {
		return nil, err
	}

	if err := d.seedDefaults(cfg); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *DB) Close() error {
	if d.SqlDB != nil {
		return d.SqlDB.Close()
	}
	return nil
}

func (d *DB) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS apps (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			api_key TEXT NOT NULL UNIQUE,
			webhook_url TEXT NOT NULL,
			webhook_secret TEXT DEFAULT '',
			is_active INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS orders (
			id TEXT PRIMARY KEY,
			app_id TEXT NOT NULL,
			channel TEXT NOT NULL,
			reference_id TEXT NOT NULL,
			currency TEXT NOT NULL,
			network TEXT DEFAULT '',
			wallet_address TEXT DEFAULT '',
			original_amount REAL NOT NULL,
			original_amount_milli INTEGER NOT NULL,
			unique_code INTEGER NOT NULL,
			total_amount REAL NOT NULL,
			total_amount_str TEXT NOT NULL,
			total_amount_milli INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'PENDING',
			tx_id TEXT UNIQUE,
			tx_hash TEXT DEFAULT '',
			from_info TEXT DEFAULT '',
			block_number INTEGER DEFAULT 0,
			qr_content TEXT NOT NULL,
			qr_path TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL,
			cooldown_until INTEGER NOT NULL DEFAULT 0,
			paid_at INTEGER DEFAULT 0,
			webhook_delivered INTEGER DEFAULT 0,
			webhook_attempts INTEGER DEFAULT 0,
			next_webhook_retry_at INTEGER DEFAULT 0,
			metadata TEXT DEFAULT '',
			FOREIGN KEY (app_id) REFERENCES apps(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_orders_matching ON orders(channel, currency, total_amount_milli, status, cooldown_until);`,
		`CREATE INDEX IF NOT EXISTS idx_orders_status_expires ON orders(status, expires_at);`,
		`CREATE INDEX IF NOT EXISTS idx_orders_app_ref ON orders(app_id, reference_id, status);`,
		`CREATE TABLE IF NOT EXISTS unmatched_transactions (
			id TEXT PRIMARY KEY,
			channel TEXT NOT NULL,
			tx_hash TEXT DEFAULT '',
			from_info TEXT DEFAULT '',
			currency TEXT NOT NULL DEFAULT 'USDT',
			amount_str TEXT NOT NULL,
			amount_milli INTEGER NOT NULL,
			raw_payload TEXT DEFAULT '',
			tx_time INTEGER NOT NULL,
			resolved_order_id TEXT DEFAULT '',
			recorded_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	for _, q := range queries {
		if _, err := d.SqlDB.Exec(q); err != nil {
			return fmt.Errorf("migration failed query [%s]: %w", q, err)
		}
	}

	return nil
}

func (d *DB) seedDefaults(cfg *config.Config) error {
	// Seed default app if no apps exist
	var count int
	if err := d.SqlDB.QueryRow("SELECT COUNT(*) FROM apps").Scan(&count); err == nil && count == 0 {
		appID := "app_default_store"
		apiKey := "ak_live_multipay_store_2026"
		if d.IsSandbox {
			apiKey = "ak_test_multipay_store_2026"
		}
		_, _ = d.SqlDB.Exec(
			"INSERT INTO apps (id, name, api_key, webhook_url, webhook_secret) VALUES (?, ?, ?, ?, ?)",
			appID, "Default Demo Store", apiKey, "https://webhook.site/multipay-demo", "sec_multipay_default_2026",
		)
	}

	// Seed system_config from .env if table is empty
	defaults := map[string]string{
		"admin_pin":                cfg.AdminPIN,
		"admin_key":                cfg.AdminKey,
		"shopee_token":             cfg.ShopeeToken,
		"qris_string":              cfg.QRISString,
		"binance_pay_id":           cfg.BinancePayID,
		"binance_account_name":     cfg.BinanceAccountName,
		"binance_uni_qr_url":       cfg.BinanceUniQRURL,
		"binance_api_key":          cfg.BinanceAPIKey,
		"binance_secret_key":       cfg.BinanceSecretKey,
		"receiver_wallet_address":  cfg.ReceiverWalletAddress,
		"bsc_rpc_urls":             strings.Join(cfg.BSCRPCURLs, ","),
		"required_confirmations":   fmt.Sprintf("%d", cfg.RequiredConfirmations),
	}

	for k, v := range defaults {
		if v != "" {
			_, _ = d.SqlDB.Exec("INSERT OR IGNORE INTO system_config (key, value) VALUES (?, ?)", k, v)
		}
	}

	return nil
}
