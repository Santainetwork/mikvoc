package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVoucherPrintHandlersRequireTemplateService(t *testing.T) {
	for _, tt := range []struct {
		name   string
		path   string
		handle func(*App, http.ResponseWriter, *http.Request)
	}{
		{"batch", "/hotspot/users/print", func(a *App, w http.ResponseWriter, r *http.Request) { a.HandlePrint(w, r) }},
		{"quick", "/hotspot/users/quickprint?username=x", func(a *App, w http.ResponseWriter, r *http.Request) { a.HandleQuickPrint(w, r) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tt.handle(&App{}, rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503", rec.Code)
			}
		})
	}
}

func TestVoucherTemplateSelectionPrecedence(t *testing.T) {
	tests := []struct {
		name, query, small, saved, want string
		batch                           bool
	}{
		{"query", "grid", "yes", "thermal", "grid", true},
		{"batch legacy small", "", "yes", "thermal", "compact", true},
		{"quick ignores small", "", "yes", "thermal", "thermal", false},
		{"saved", "", "", "thermal", "thermal", true},
		{"default", "", "", "", "classic", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := voucherTemplateID(tt.query, tt.small, tt.saved, tt.batch); got != tt.want {
				t.Fatalf("voucherTemplateID() = %q, want %q", got, tt.want)
			}
		})
	}
}
