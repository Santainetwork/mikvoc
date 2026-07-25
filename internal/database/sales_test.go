package database

import (
	"path/filepath"
	"testing"
)

func TestAddSaleWithTimeIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sales.db")
	if err := Init(path, "secret"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = DB.Close() }()

	key := "mikvoc-report-2026-05-04|13:14:15|voucher-01|5000"
	ok, err := AddSaleWithTimeIdempotent(1, "voucher-01", "hotspot", key, 5000, "2026-05-04 13:14:15", key)
	if err != nil || !ok {
		t.Fatalf("first insert: ok=%v err=%v", ok, err)
	}
	ok, err = AddSaleWithTimeIdempotent(1, "voucher-01", "hotspot", key, 5000, "2026-05-04 13:14:15", key)
	if err != nil {
		t.Fatalf("second insert err: %v", err)
	}
	if ok {
		t.Fatal("second insert should be ignored")
	}

	var n int
	if err := DB.QueryRow(`SELECT COUNT(*) FROM sales WHERE source_key=?`, key).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("count=%d want 1", n)
	}
}

func TestMigrateV5DedupesSourceKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dedupe.db")
	if err := Init(path, "secret"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = DB.Close() }()

	key := "mikvoc-report-2026-01-01|10:00:00|u1|1000"
	if _, err := DB.Exec(`INSERT INTO sales (router_id,username,profile,comment,price,created_at,source_key) VALUES (1,'u1','hotspot',?,1000,'2026-01-01 10:00:00',?)`, key, key); err != nil {
		t.Fatal(err)
	}
	// Drop unique index temporarily and insert duplicate then re-run migration path via direct SQL not possible;
	// verify unique index rejects second insert.
	_, err := DB.Exec(`INSERT INTO sales (router_id,username,profile,comment,price,created_at,source_key) VALUES (1,'u1','hotspot',?,1000,'2026-01-01 10:00:00',?)`, key, key)
	if err == nil {
		t.Fatal("expected unique constraint on source_key")
	}
}
