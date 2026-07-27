package httpapi

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"mikvoc/internal/database"
	"mikvoc/internal/middleware"
	"mikvoc/internal/repository"
	"mikvoc/internal/service"
)

// TestRenderPages ensures every page template compiles and renders without error
// using the new shadcn-inspired layout. Catches template syntax issues, missing
// partials, and broken template functions.
func TestRenderPages(t *testing.T) {
	// Init test database
	oldDB := database.DB
	path := filepath.Join(t.TempDir(), "mikvoc.db")
	if err := database.Init(path, "test-secret"); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() {
		if database.DB != nil {
			_ = database.DB.Close()
		}
		database.DB = oldDB
	})

	// Init session store with test secret
	middleware.InitSession("test-secret-for-render-test")

	store := repository.NewStore()
	pool := service.NewPool()
	app := NewApp(store, pool, service.NewAuth(store), service.NewRouter(store, pool), service.NewUser(pool), service.NewProfile(pool), service.NewGenerate(pool, store))
	app.Stats = service.NewStats(pool)
	app.Sales = service.NewSales(pool, store)
	if err := app.LoadTemplates(); err != nil {
		t.Fatalf("load templates: %v", err)
	}

	pages := []struct {
		path  string
		title string
	}{
		{"/login", "Login"},
		{"/routers", "Sesi Router"},
		{"/dashboard", "Dashboard"},
		{"/hotspot/users", "User List"},
		{"/hotspot/generate", "Generate"},
		{"/hotspot/active", "Hotspot Aktif"},
		{"/hotspot/profiles", "Profiles"},
		{"/report", "Laporan"},
		{"/settings", "Pengaturan"},
		{"/template", "Template Editor"},
	}

	for _, p := range pages {
		t.Run(p.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, p.path, nil)
			rr := httptest.NewRecorder()

			// Create session with authenticated + router_id = 0 (no router connected)
			sess, _ := middleware.Store.Get(req, middleware.SessionName)
			sess.Values["authenticated"] = true
			sess.Values["router_id"] = 0
			_ = sess.Save(req, rr)

			req = req.WithContext(req.Context())
			// Re-read cookie set by sess.Save
			req.Header.Set("Cookie", rr.Header().Get("Set-Cookie"))
			rr = httptest.NewRecorder()

			switch p.path {
			case "/login":
				app.HandleLogin(rr, req)
			case "/routers":
				app.HandleRouters(rr, req)
			case "/dashboard":
				app.HandleDashboard(rr, req)
			case "/hotspot/users":
				app.HandleUsers(rr, req)
			case "/hotspot/generate":
				app.HandleGenerate(rr, req)
			case "/hotspot/active":
				app.HandleActiveUsers(rr, req)
			case "/hotspot/profiles":
				app.HandleProfiles(rr, req)
			case "/report":
				app.HandleReport(rr, req)
			case "/settings":
				app.HandleSettings(rr, req)
			case "/template":
				app.HandleTemplateEditor(rr, req)
			}

			body := rr.Body.String()
			if rr.Code >= 500 {
				t.Fatalf("%s returned %d: %s", p.path, rr.Code, body)
			}
			// Redirects (3xx) are fine for some pages, but for pages we expect HTML
			if rr.Code >= 300 && rr.Code < 400 {
				return
			}
			if !strings.Contains(body, p.title) {
				t.Fatalf("%s: title %q not found in body (HTTP %d)", p.path, p.title, rr.Code)
			}
			if !strings.Contains(body, `app.css`) {
				t.Fatalf("%s: app.css not referenced", p.path)
			}
			if !strings.Contains(body, `id="sidebar"`) && p.path != "/login" {
				t.Fatalf("%s: sidebar missing", p.path)
			}
			if !strings.Contains(body, `id="toast-container"`) {
				t.Fatalf("%s: toast container missing", p.path)
			}
		})
	}
}
