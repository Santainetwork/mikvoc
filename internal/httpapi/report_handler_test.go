package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReportHandlersRequireSalesService(t *testing.T) {
	app := &App{}
	for _, handler := range []http.HandlerFunc{app.HandleReport, app.HandleSalesPurge} {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, "/report", nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
		}
	}
}
