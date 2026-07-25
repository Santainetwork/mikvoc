package database

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestInitWithSecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mikvoc.db")
	if err := Init(path, "secret-test"); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = DB.Close() }()
	if RouterCipher() == nil {
		t.Fatal("cipher should be configured")
	}
}

func TestInitWithoutSecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mikvoc.db")
	if err := Init(path); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = DB.Close() }()
	if RouterCipher() != nil {
		t.Fatal("cipher should be nil when no secret")
	}
}

func TestRouterPasswordEncryptDecrypt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mikvoc.db")
	if err := Init(path, "secret-test"); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = DB.Close() }()

	rt := &Router{Name: "test", IP: "10.0.0.1", Port: "8728", Username: "admin", Password: "rahasia"}
	if err := SaveRouter(rt); err != nil {
		t.Fatalf("save: %v", err)
	}
	if rt.ID == 0 {
		t.Fatal("ID not set after save")
	}

	saved := RouterCipher()
	RouterCipherPtr := saved
	_ = RouterCipherPtr

	var stored string
	if err := DB.QueryRow(`SELECT password FROM routers WHERE id=?`, rt.ID).Scan(&stored); err != nil {
		t.Fatalf("read raw: %v", err)
	}
	if stored == "rahasia" {
		t.Fatal("password stored as plaintext")
	}

	got, err := GetRouter(rt.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Password != "rahasia" {
		t.Fatalf("decrypted password = %q, want %q", got.Password, "rahasia")
	}

	list, err := GetRouters()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Password != "rahasia" {
		t.Fatalf("list returned wrong password: %+v", list)
	}
}

func TestRouterPasswordMigratePlaintext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mikvoc.db")
	if err := Init(path); err != nil {
		t.Fatalf("init without secret: %v", err)
	}
	if _, err := DB.Exec(`INSERT INTO routers (name, ip, port, username, password, sort_order) VALUES ('legacy', '10.0.0.2', '8728', 'admin', 'plain-pass', 0)`); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}
	_ = DB.Close()

	if err := Init(path, "new-secret"); err != nil {
		t.Fatalf("re-init with secret: %v", err)
	}
	defer func() { _ = DB.Close() }()

	got, err := GetRouters()
	if err != nil {
		t.Fatalf("get routers after migration: %v", err)
	}
	if len(got) != 1 || got[0].Password != "plain-pass" {
		t.Fatalf("expected plaintext after migration, got %+v", got)
	}

	var stored string
	if err := DB.QueryRow(`SELECT password FROM routers`).Scan(&stored); err != nil {
		t.Fatalf("read raw: %v", err)
	}
	if stored == "plain-pass" {
		t.Fatal("plaintext not migrated to encrypted form")
	}
}

func TestAdminSetPassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mikvoc.db")
	if err := Init(path, "secret"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = DB.Close() }()

	if err := SetAdminPassword("admin", "$2a$12$abc"); err != nil {
		t.Fatalf("set: %v", err)
	}
	user, hash := GetAdmin()
	if user != "admin" || hash != "$2a$12$abc" {
		t.Fatalf("got %q %q", user, hash)
	}

	if err := SetAdmin("newadmin", ""); err != nil {
		t.Fatalf("set username only: %v", err)
	}
	user, hash = GetAdmin()
	if user != "newadmin" || hash != "$2a$12$abc" {
		t.Fatalf("expected keep hash, got %q %q", user, hash)
	}
}

func TestSetTemplateSettingsRollsBackOnWriteFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "template-settings.db")
	if err := Init(path, "secret"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = DB.Close() }()
	if err := SetSetting("tpl_app_name", "old"); err != nil {
		t.Fatal(err)
	}
	if _, err := DB.Exec(`CREATE TRIGGER reject_template_setting BEFORE INSERT ON settings WHEN NEW.key = 'tpl_variant' AND (SELECT value FROM settings WHERE key = 'tpl_app_name') = 'new' BEGIN SELECT RAISE(FAIL, 'forced failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := SetTemplateSettings(0, map[string]string{"tpl_app_name": "new", "tpl_variant": "modern"}); err == nil {
		t.Fatal("SetTemplateSettings() succeeded despite forced write failure")
	}
	if got := GetSetting("tpl_app_name"); got != "old" {
		t.Fatalf("tpl_app_name = %q after rollback, want old", got)
	}
}

func TestSetTemplateSettingsRouterFallbackAndExplicitAssetClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "template-fallback.db")
	if err := Init(path, "secret"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = DB.Close() }()
	router := &Router{Name: "router", IP: "127.0.0.1", Port: "8728", Username: "admin"}
	if err := SaveRouter(router); err != nil {
		t.Fatal(err)
	}
	if err := SetTemplateSettings(0, map[string]string{"tpl_app_name": "A", "tpl_subtitle": "global", "tpl_custom_assets_zip": "global-zip"}); err != nil {
		t.Fatal(err)
	}
	if err := SetTemplateSettings(router.ID, map[string]string{"tpl_app_name": "A", "tpl_subtitle": "local", "tpl_custom_assets_zip": ""}); err != nil {
		t.Fatal(err)
	}
	var appOverrides, assetOverrides int
	if err := DB.QueryRow(`SELECT COUNT(*) FROM router_settings WHERE router_id=? AND key='tpl_app_name'`, router.ID).Scan(&appOverrides); err != nil {
		t.Fatal(err)
	}
	if err := DB.QueryRow(`SELECT COUNT(*) FROM router_settings WHERE router_id=? AND key='tpl_custom_assets_zip' AND value=''`, router.ID).Scan(&assetOverrides); err != nil {
		t.Fatal(err)
	}
	if appOverrides != 0 || assetOverrides != 1 {
		t.Fatalf("override counts app=%d assets=%d, want 0 and 1", appOverrides, assetOverrides)
	}
	if err := SetTemplateSettings(0, map[string]string{"tpl_app_name": "B", "tpl_subtitle": "changed", "tpl_custom_assets_zip": "new-global-zip"}); err != nil {
		t.Fatal(err)
	}
	got := GetRouterSettings(router.ID)
	if got["tpl_app_name"] != "B" || got["tpl_subtitle"] != "local" || got["tpl_custom_assets_zip"] != "" {
		t.Fatalf("merged settings = %#v", got)
	}
}

func TestMigrateAddsRoleAndAuditLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mikvoc.db")
	if err := Init(path, "secret"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = DB.Close() }()

	var role string
	if err := DB.QueryRow(`SELECT role FROM admins WHERE username='admin'`).Scan(&role); err != nil {
		t.Fatalf("role col: %v", err)
	}
	if role != "owner" {
		t.Fatalf("default role=%q", role)
	}

	var n int
	if err := DB.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='audit_log'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("audit_log table: n=%d err=%v", n, err)
	}
}

func TestVersionedMigrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mig.db")
	if err := Init(path, "secret"); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = DB.Close() }()

	var versions []int
	rows, err := DB.Query(`SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatalf("schema_migrations: %v", err)
	}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		versions = append(versions, v)
	}
	rows.Close()
	want := []int{1, 2, 3, 4, 5}
	if len(versions) != len(want) {
		t.Fatalf("versions=%v want %v", versions, want)
	}
	for i := range want {
		if versions[i] != want[i] {
			t.Fatalf("versions=%v want %v", versions, want)
		}
	}

	if _, err := GetRouters(); err != nil {
		t.Fatalf("GetRouters: %v", err)
	}

	var role string
	if err := DB.QueryRow(`SELECT role FROM admins WHERE username='admin'`).Scan(&role); err != nil {
		t.Fatalf("role: %v", err)
	}
	if role != "owner" {
		t.Fatalf("role=%q", role)
	}

	if err := runMigrations(DB); err != nil {
		t.Fatalf("second run: %v", err)
	}
	var n int
	if err := DB.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil || n != 5 {
		t.Fatalf("after re-run count=%d err=%v", n, err)
	}
	if err := DB.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('sales') WHERE name='source_key'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("source_key missing: n=%d err=%v", n, err)
	}
}

func TestLegacyDBRecordsMigrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
CREATE TABLE admins (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT NOT NULL UNIQUE, password TEXT NOT NULL);
CREATE TABLE routers (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL DEFAULT '', ip TEXT NOT NULL, port TEXT NOT NULL DEFAULT '8728', username TEXT NOT NULL DEFAULT 'admin', password TEXT NOT NULL DEFAULT '', sort_order INTEGER NOT NULL DEFAULT 0);
INSERT INTO admins (username, password) VALUES ('admin', 'admin');
`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()

	if err := Init(path, "secret"); err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	defer func() { _ = DB.Close() }()

	var n int
	if err := DB.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil || n != 5 {
		t.Fatalf("versions recorded: n=%d err=%v", n, err)
	}
	var role string
	if err := DB.QueryRow(`SELECT role FROM admins`).Scan(&role); err != nil || role != "owner" {
		t.Fatalf("role=%q err=%v", role, err)
	}
	if err := DB.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('routers') WHERE name='voucher_template'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("voucher_template missing: n=%d err=%v", n, err)
	}
	if err := DB.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='audit_log'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("audit_log: n=%d err=%v", n, err)
	}
}
