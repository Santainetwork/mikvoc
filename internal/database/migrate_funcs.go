package database

import (
	"database/sql"
	"fmt"
)

func migrateV1Init(tx *sql.Tx) error {
	_, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS admins (
	id       INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL UNIQUE,
	password TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS routers (
	id       INTEGER PRIMARY KEY AUTOINCREMENT,
	name     TEXT NOT NULL DEFAULT '',
	ip       TEXT NOT NULL,
	port     TEXT NOT NULL DEFAULT '8728',
	username TEXT NOT NULL DEFAULT 'admin',
	password TEXT NOT NULL DEFAULT '',
	sort_order INTEGER NOT NULL DEFAULT 0,
	voucher_template TEXT NOT NULL DEFAULT 'classic'
);

CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS router_settings (
	router_id INTEGER NOT NULL REFERENCES routers(id) ON DELETE CASCADE,
	key       TEXT NOT NULL,
	value     TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (router_id, key)
);

CREATE TABLE IF NOT EXISTS sales (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	router_id  INTEGER REFERENCES routers(id) ON DELETE CASCADE,
	username   TEXT NOT NULL,
	profile    TEXT NOT NULL,
	comment    TEXT NOT NULL DEFAULT '',
	price      INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT (datetime('now','localtime'))
);

CREATE TABLE IF NOT EXISTS voucher_templates (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	name       TEXT NOT NULL UNIQUE,
	html       TEXT NOT NULL,
	is_default INTEGER NOT NULL DEFAULT 0
);

INSERT OR IGNORE INTO admins (username, password) VALUES ('admin', 'admin');

INSERT OR IGNORE INTO settings (key, value) VALUES
	('theme',    'dark'),
	('currency', 'Rp'),
	('timezone', 'Asia/Jakarta'),
	('language', 'id');
`)
	if err != nil {
		return fmt.Errorf("init schema: %w", err)
	}
	return nil
}

func migrateV2VoucherTemplate(tx *sql.Tx) error {
	ok, err := hasColumn(tx, "routers", "voucher_template")
	if err != nil {
		return fmt.Errorf("pragma routers: %w", err)
	}
	if ok {
		return nil
	}
	if _, err := tx.Exec(`ALTER TABLE routers ADD COLUMN voucher_template TEXT NOT NULL DEFAULT 'classic'`); err != nil {
		return fmt.Errorf("alter routers: %w", err)
	}
	return nil
}

func migrateV3AdminRole(tx *sql.Tx) error {
	ok, err := hasColumn(tx, "admins", "role")
	if err != nil {
		return fmt.Errorf("pragma admins: %w", err)
	}
	if !ok {
		if _, err := tx.Exec(`ALTER TABLE admins ADD COLUMN role TEXT NOT NULL DEFAULT 'owner'`); err != nil {
			return fmt.Errorf("alter admins role: %w", err)
		}
	}
	if _, err := tx.Exec(`UPDATE admins SET role='owner' WHERE role IS NULL OR role=''`); err != nil {
		return fmt.Errorf("backfill admin role: %w", err)
	}
	return nil
}

func migrateV4AuditLog(tx *sql.Tx) error {
	if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS audit_log (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	admin_id INTEGER NOT NULL DEFAULT 0,
	admin_name TEXT NOT NULL DEFAULT '',
	action TEXT NOT NULL,
	target TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT (datetime('now','localtime'))
);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_log(created_at DESC);
`); err != nil {
		return fmt.Errorf("create audit_log: %w", err)
	}
	return nil
}

func migrateV5SalesIdempotent(tx *sql.Tx) error {
	ok, err := hasColumn(tx, "sales", "source_key")
	if err != nil {
		return fmt.Errorf("pragma sales: %w", err)
	}
	if !ok {
		if _, err := tx.Exec(`ALTER TABLE sales ADD COLUMN source_key TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("alter sales source_key: %w", err)
		}
	}

	if _, err := tx.Exec(`
UPDATE sales
SET source_key = comment
WHERE (source_key IS NULL OR source_key = '')
  AND comment LIKE 'mikvoc-report-%'
`); err != nil {
		return fmt.Errorf("backfill sales source_key: %w", err)
	}

	if _, err := tx.Exec(`
DELETE FROM sales
WHERE id NOT IN (
  SELECT MIN(id) FROM sales
  WHERE source_key != ''
  GROUP BY router_id, source_key
) AND source_key != ''
`); err != nil {
		return fmt.Errorf("dedupe sales source_key: %w", err)
	}

	if _, err := tx.Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS idx_sales_router_source_key
ON sales(router_id, source_key)
WHERE source_key != ''
`); err != nil {
		return fmt.Errorf("unique sales source_key: %w", err)
	}
	return nil
}
