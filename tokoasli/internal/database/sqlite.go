package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type DB struct {
	db *sql.DB
}

// New opens a connection to SQLite database and runs migrations
func New(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "store.db")
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", filepath.ToSlash(dbPath))

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// SQLite WAL handles multiple readers, but keep connection settings clean
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	storeDB := &DB{db: db}
	if err := storeDB.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return storeDB, nil
}

// Close closes the underlying database
func (d *DB) Close() error {
	return d.db.Close()
}

// migrate creates tables and indexes
func (d *DB) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS products (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			category TEXT NOT NULL DEFAULT 'Umum',
			price INTEGER NOT NULL,
			description TEXT,
			image_url TEXT,
			delivery_type TEXT NOT NULL DEFAULT 'pool',
			fixed_content TEXT,
			is_active INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS product_stocks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			product_id TEXT NOT NULL,
			content TEXT NOT NULL,
			is_used INTEGER NOT NULL DEFAULT 0,
			order_id TEXT,
			held_until TEXT,
			created_at TEXT NOT NULL,
			used_at TEXT,
			FOREIGN KEY(product_id) REFERENCES products(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS store_orders (
			id TEXT PRIMARY KEY,
			secret_token TEXT NOT NULL UNIQUE,
			product_id TEXT NOT NULL,
			product_name TEXT NOT NULL,
			customer_email TEXT NOT NULL,
			customer_phone TEXT,
			original_price INTEGER NOT NULL,
			unique_code INTEGER NOT NULL,
			total_amount INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'PENDING',
			delivery_content TEXT,
			paymentg_order_id TEXT,
			qr_string TEXT,
			qr_url TEXT,
			client_ip TEXT NOT NULL,
			created_at TEXT NOT NULL,
			paid_at TEXT,
			expires_at TEXT NOT NULL,
			FOREIGN KEY(product_id) REFERENCES products(id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_products_active ON products(is_active);`,
		`CREATE INDEX IF NOT EXISTS idx_stocks_lookup ON product_stocks(product_id, is_used);`,
		`CREATE INDEX IF NOT EXISTS idx_stocks_held ON product_stocks(is_used, held_until);`,
		`CREATE INDEX IF NOT EXISTS idx_orders_status ON store_orders(status);`,
		`CREATE INDEX IF NOT EXISTS idx_orders_secret ON store_orders(secret_token);`,
		`CREATE INDEX IF NOT EXISTS idx_orders_ip ON store_orders(client_ip, status);`,
	}

	for _, q := range queries {
		if _, err := d.db.Exec(q); err != nil {
			return fmt.Errorf("migration query error (%s): %w", q, err)
		}
	}

	// Add customer_phone to store_orders if missing (for backwards compatibility)
	d.addColumnIfNotExists("store_orders", "customer_phone", "TEXT")

	return nil
}

func (d *DB) addColumnIfNotExists(table, column, colType string) {
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = '%s'", table, column)
	if err := d.db.QueryRow(query).Scan(&count); err == nil && count == 0 {
		alter := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, colType)
		_, _ = d.db.Exec(alter)
	}
}
