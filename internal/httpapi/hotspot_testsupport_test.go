package httpapi

import (
	"path/filepath"
	"testing"

	"mikvoc/internal/database"
)

func withTestDB(t *testing.T) {
	t.Helper()
	old := database.DB
	path := filepath.Join(t.TempDir(), "mikvoc.db")
	if err := database.Init(path, "test-secret"); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() {
		if database.DB != nil {
			_ = database.DB.Close()
		}
		database.DB = old
	})
}

func setTestSetting(key, value string) error {
	return database.SetSetting(key, value)
}
