package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDashboardHandlersRequireStatsService(t *testing.T) {
	app := &App{}
	for _, handler := range []http.HandlerFunc{app.HandleDashboard, app.HandleDashboardAPI, app.HandleTrafficAPI} {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
		}
	}
}

func TestDashboardResponseEscapesRouterValues(t *testing.T) {
	payload := dashboardResponse{Connected: true, Board: `RB5009"test`, HealthDetail: "line\nbreak"}
	var body strings.Builder
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		t.Fatal(err)
	}
	var decoded dashboardResponse
	if err := json.Unmarshal([]byte(body.String()), &decoded); err != nil {
		t.Fatalf("invalid JSON %q: %v", body.String(), err)
	}
	if decoded.Board != payload.Board || decoded.HealthDetail != payload.HealthDetail {
		t.Fatalf("decoded = %#v, want %#v", decoded, payload)
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(body.String()), &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"active", "users", "mem_pct"} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("zero-value field %q missing from %s", key, body.String())
		}
	}
}

func TestDashboardHealthSummaryClassifiesRouterState(t *testing.T) {
	tests := []struct {
		name      string
		connected bool
		cpu       int
		mem       int
		wantLabel string
		wantTone  string
	}{
		{name: "disconnected", connected: false, wantLabel: "Tidak terhubung", wantTone: "amber"},
		{name: "healthy", connected: true, cpu: 24, mem: 42, wantLabel: "Sehat", wantTone: "emerald"},
		{name: "warning cpu", connected: true, cpu: 72, mem: 40, wantLabel: "Perlu dicek", wantTone: "amber"},
		{name: "critical memory", connected: true, cpu: 30, mem: 91, wantLabel: "Kritis", wantTone: "red"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dashboardHealthSummary(tt.connected, tt.cpu, tt.mem)
			if got.Label != tt.wantLabel || got.Tone != tt.wantTone {
				t.Fatalf("expected %s/%s, got %s/%s", tt.wantLabel, tt.wantTone, got.Label, got.Tone)
			}
			if got.Detail == "" || got.Icon == "" {
				t.Fatalf("expected detail and icon to be populated, got %#v", got)
			}
		})
	}
}
