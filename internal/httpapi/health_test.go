package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/mux"

	"mikvoc/internal/database"
	"mikvoc/internal/repository"
	"mikvoc/internal/service"
)

func TestHandleHealthzAndMetrics(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	if err := database.Init(dbPath, "test-secret-key-32bytes-long!!"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if database.DB != nil {
			_ = database.DB.Close()
			database.DB = nil
		}
	})

	store := repository.NewStore()
	pool := service.NewPool()
	app := NewApp(store, pool, service.NewAuth(store), service.NewRouter(store, pool), service.NewUser(pool), service.NewProfile(pool), service.NewGenerate(pool, store))
	app.SetDBPath(dbPath)
	app.SetSecret("test-secret-key-32bytes-long!!")

	r := mux.NewRouter()
	app.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz status %d body %s", rec.Code, rec.Body.String())
	}
	var hr healthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &hr); err != nil {
		t.Fatal(err)
	}
	if hr.Status != "ok" || hr.DB != "ok" {
		t.Fatalf("healthz payload: %+v", hr)
	}
	if hr.UptimeSec < 0 {
		t.Fatalf("uptime: %d", hr.UptimeSec)
	}

	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics status %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Fatalf("content-type: %s", ct)
	}
	body := rec.Body.String()
	for _, want := range []string{"mikvoc_up 1", "mikvoc_routers_connected", "mikvoc_routers_total", "mikvoc_uptime_seconds"} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics missing %q in:\n%s", want, body)
		}
	}
}

func TestHandleBackupStreamsFile(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	if err := database.Init(dbPath, "test-secret-key-32bytes-long!!"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if database.DB != nil {
			_ = database.DB.Close()
			database.DB = nil
		}
	})

	store := repository.NewStore()
	pool := service.NewPool()
	app := NewApp(store, pool, service.NewAuth(store), service.NewRouter(store, pool), service.NewUser(pool), service.NewProfile(pool), service.NewGenerate(pool, store))
	app.SetDBPath(dbPath)

	req := httptest.NewRequest(http.MethodGet, "/settings/backup", nil)
	rec := httptest.NewRecorder()
	app.HandleBackup(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	cd := rec.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "mikvoc-backup-") || !strings.Contains(cd, ".db") {
		t.Fatalf("disposition: %s", cd)
	}
	if rec.Body.Len() < 100 {
		t.Fatalf("backup too small: %d", rec.Body.Len())
	}
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if int64(rec.Body.Len()) != info.Size() {
		t.Fatalf("size body=%d file=%d", rec.Body.Len(), info.Size())
	}
}
