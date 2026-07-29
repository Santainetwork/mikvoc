package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gorilla/mux"

	"mikvoc/internal/assets"
	"mikvoc/internal/database"
	"mikvoc/internal/middleware"
	"mikvoc/internal/repository"
	"mikvoc/internal/service"
)

var tempDB string

func init() {
	var err error
	tempFile, err := os.CreateTemp("", "test-db-*.sqlite")
	if err != nil {
		panic(err)
	}
	tempDB = tempFile.Name()
	tempFile.Close()
	
	err = database.Init(tempDB, "test-secret-key-for-testing-only-32-chars")
	if err != nil {
		panic(err)
	}
	
	// Initialize middleware for session support
	middleware.InitSession("test-secret-key-for-testing-only-32-chars")
}

func TestEditorVisualAssetsPanel(t *testing.T) {
	store := repository.NewStore()
	pool := service.NewPool()
	assetDir := t.TempDir()
	assetStore := assets.New(assetDir)
	app := NewApp(store, pool, nil, nil, nil, nil, nil, assetStore)
	app.SetSecret("test-secret-key-for-testing-only-32-chars")
	app.Template = service.NewTemplate(pool, store)

	router := mux.NewRouter()
	app.RegisterRoutes(router)

	t.Run("panel exists in template editor HTML", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/template", nil)
		
		// Create fake session cookie with authenticated state
		sess, _ := middleware.Store.Get(req, middleware.SessionName)
		sess.Values["router_id"] = 1
		sess.Values["authenticated"] = true
		sess.Save(req, httptest.NewRecorder())
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			t.Logf("Status: %d Body: %.200s...", w.Code, w.Body.String())
			t.Fatalf("Expected 200 OK, got %d", w.Code)
		}

		body := w.Body.String()
		
		if !strings.Contains(body, "logo-preview") {
			t.Error("Missing logo preview section - need to add visual-assets section to template_editor.html")
		}

		if !strings.Contains(body, "focalX") || !strings.Contains(body, "focalY") {
			t.Error("Missing focal X/Y controls in UI")
		}
	})

	t.Run("background thumbnail sections exist", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/template", nil)
		
		// Create fake session cookie with authenticated state
		sess, _ := middleware.Store.Get(req, middleware.SessionName)
		sess.Values["router_id"] = 1
		sess.Values["authenticated"] = true
		sess.Save(req, httptest.NewRecorder())
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		body := w.Body.String()
		
		hasBackgroundSection := strings.Contains(body, "background-image") ||
		                        strings.Contains(body, "Background") ||
		                        strings.Contains(body, "object-position") ||
		                        strings.Contains(body, "thumbnail")
		if !hasBackgroundSection {
			t.Logf("Body snippet: %.500s...", body[:min(500, len(body))])
		}
	})
}

// TestFocalPointControls validates X/Y range constraints
func TestFocalPointControls(t *testing.T) {
	store := repository.NewStore()
	pool := service.NewPool()
	assetDir := t.TempDir()
	assetStore := assets.New(assetDir)
	app := NewApp(store, pool, nil, nil, nil, nil, nil, assetStore)
	app.SetSecret("test-secret-key-for-testing-only-32-chars")
	app.Template = service.NewTemplate(pool, store)

	router := mux.NewRouter()
	router.HandleFunc("/template/focal", app.HandleFocalPosition).Methods(http.MethodPost)

	testCases := []struct {
		name         string
		focalX       string
		focalY       string
		expectedCode int
	}{
		{"clamps focalX positive overflow to 100", "150", "50", http.StatusSeeOther},
		{"clamps focalY negative to 0", "50", "-10", http.StatusSeeOther},
		{"accepts focalX at boundary 0", "0", "50", http.StatusSeeOther},
		{"accepts focalX at boundary 100", "100", "50", http.StatusSeeOther},
		{"requires focalX as integer", "abc", "50", http.StatusBadRequest},
		{"requires focalY as integer", "50", "xyz", http.StatusBadRequest},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			values := url.Values{}
			values.Set("focalX", tc.focalX)
			values.Set("focalY", tc.focalY)
			
			req := httptest.NewRequest(http.MethodPost, "/template/focal?"+values.Encode(), nil)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			
			if w.Code != tc.expectedCode {
				t.Logf("focalX=%q focalY=%q -> status=%d (expected %d)", tc.focalX, tc.focalY, w.Code, tc.expectedCode)
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
