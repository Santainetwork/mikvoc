package httpapi

import "testing"

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
