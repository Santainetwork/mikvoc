package httpapi

import (
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"mikvoc/internal/core"
)

func TestResolveReportFiltersSupportsMikhmonPresets(t *testing.T) {
	now := time.Date(2026, 5, 4, 15, 30, 0, 0, time.Local)

	tests := []struct {
		name        string
		values      url.Values
		wantFrom    string
		wantTo      string
		wantPreset  string
		wantProfile string
	}{
		{name: "today", values: url.Values{"preset": {"today"}}, wantFrom: "2026-05-04", wantTo: "2026-05-04", wantPreset: "today"},
		{name: "yesterday", values: url.Values{"preset": {"yesterday"}}, wantFrom: "2026-05-03", wantTo: "2026-05-03", wantPreset: "yesterday"},
		{name: "last seven days", values: url.Values{"preset": {"last_7"}}, wantFrom: "2026-04-28", wantTo: "2026-05-04", wantPreset: "last_7"},
		{name: "this month", values: url.Values{"preset": {"this_month"}}, wantFrom: "2026-05-01", wantTo: "2026-05-04", wantPreset: "this_month"},
		{name: "last month", values: url.Values{"preset": {"last_month"}}, wantFrom: "2026-04-01", wantTo: "2026-04-30", wantPreset: "last_month"},
		{name: "default current month", values: url.Values{}, wantFrom: "2026-05-01", wantTo: "2026-05-04", wantPreset: "this_month"},
		{name: "custom range with profile", values: url.Values{"from": {"2026-04-10"}, "to": {"2026-04-12"}, "profile": {"1d"}}, wantFrom: "2026-04-10", wantTo: "2026-04-12", wantPreset: "custom", wantProfile: "1d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveReportFilters(now, tt.values)
			if got.From != tt.wantFrom || got.To != tt.wantTo || got.Preset != tt.wantPreset || got.Profile != tt.wantProfile {
				t.Fatalf("resolveReportFilters() = from %q to %q preset %q profile %q", got.From, got.To, got.Preset, got.Profile)
			}
		})
	}
}

func TestBuildReportViewDataFiltersByProfileAndBuildsSummary(t *testing.T) {
	sales := []core.Sale{
		{Username: "A1", Profile: "1d", Comment: "batch-a", Price: 2000, CreatedAt: "2026-05-04 09:00:00"},
		{Username: "A2", Profile: "1d", Comment: "batch-a", Price: 2000, CreatedAt: "2026-05-03 10:00:00"},
		{Username: "B1", Profile: "7d", Comment: "batch-b", Price: 10000, CreatedAt: "2026-05-04 11:00:00"},
	}

	data := buildReportViewData(sales, reportFilters{From: "2026-05-01", To: "2026-05-04", Preset: "this_month", Profile: "1d"})

	if data.TotalCount != 2 || data.TotalRevenue != 4000 || data.TotalRevenueLabel != "Rp 4.000" {
		t.Fatalf("unexpected selected summary: count=%d revenue=%d label=%q", data.TotalCount, data.TotalRevenue, data.TotalRevenueLabel)
	}
	if len(data.Sales) != 2 || data.Sales[0].Username != "A1" || data.Sales[1].Username != "A2" {
		t.Fatalf("expected only 1d sales to remain, got %#v", data.Sales)
	}
	if len(data.Daily) != 2 || data.Daily[0].Day != "2026-05-04" || data.Daily[0].Count != 1 || data.Daily[0].TotalLabel != "Rp 2.000" {
		t.Fatalf("unexpected daily summary: %#v", data.Daily)
	}
	if got := strings.Join(data.ProfileOptions, ","); got != "1d,7d" {
		t.Fatalf("expected profile options from existing sales data, got %q", got)
	}

	oneDay := findReportProfileSummary(data.ProfileSummaries, "1d")
	if oneDay == nil || oneDay.Count != 2 || oneDay.Revenue != 4000 || oneDay.RevenueLabel != "Rp 4.000" {
		t.Fatalf("unexpected 1d profile summary: %#v", oneDay)
	}
	sevenDay := findReportProfileSummary(data.ProfileSummaries, "7d")
	if sevenDay == nil || sevenDay.Count != 1 || sevenDay.Revenue != 10000 || sevenDay.RevenueLabel != "Rp 10.000" {
		t.Fatalf("unexpected 7d profile summary: %#v", sevenDay)
	}
}

func TestReportTemplateContainsDatePresetsAndProfileControls(t *testing.T) {
	b, err := os.ReadFile("../../web/templates/pages/report.html")
	if err != nil {
		t.Fatalf("read report template: %v", err)
	}
	html := string(b)

	for _, want := range []string{
		`preset=today`,
		`preset=yesterday`,
		`preset=last_7`,
		`preset=this_month`,
		`preset=last_month`,
		`&profile=`,
		`name="profile"`,
		`{{range .ProfileOptions}}`,
		`{{.TotalRevenueLabel}}`,
		`{{.AverageDailyLabel}}`,
		`Ringkasan Profil`,
		`{{range .ProfileSummaries}}`,
		`/report/purge-scripts`,
		`Bersihkan script di router`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected report template to contain %q", want)
		}
	}
}

func TestSaleFromRouterScriptParsesMikvocReportV7Format(t *testing.T) {
	now := time.Date(2026, 5, 4, 15, 30, 0, 0, time.Local)
	name := "mikvoc-report-2026-05-04|13:14:15|voucher-01|5000"

	sale, ok := saleFromRouterScript(map[string]string{"name": name}, now)
	if !ok {
		t.Fatalf("expected mikvoc report script to parse")
	}

	if sale.Username != "voucher-01" || sale.Profile != "hotspot" || sale.Comment != name || sale.Price != 5000 || sale.CreatedAt != "2026-05-04 13:14:15" {
		t.Fatalf("unexpected sale: %#v", sale)
	}
}

func TestSaleFromRouterScriptParsesMikhmonNameWithOldCommentAndProfile(t *testing.T) {
	now := time.Date(2026, 5, 4, 15, 30, 0, 0, time.Local)
	name := "may/04/2026-|-09:08:07-|-user-01-|-10000-|-10.5.50.1-|-AA:BB:CC:DD:EE:FF-|-1d-|-profile-1d-|-original comment"

	sale, ok := saleFromRouterScript(map[string]string{"name": name, "comment": "mikhmon"}, now)
	if !ok {
		t.Fatalf("expected mikhmon report script to parse")
	}

	if sale.Username != "user-01" || sale.Profile != "profile-1d" || sale.Comment != "original comment" || sale.Price != 10000 || sale.CreatedAt != "2026-05-04 09:08:07" {
		t.Fatalf("unexpected sale: %#v", sale)
	}
}

func TestSaleFromRouterScriptPreservesMikhmonOldCommentContainingSeparator(t *testing.T) {
	now := time.Date(2026, 5, 4, 15, 30, 0, 0, time.Local)
	name := "2026-05-04-|-10:11:12-|-user-02-|-7500-|-10.5.50.2-|-11:22:33:44:55:66-|-1d-|-profile-1d-|-note -|- with -|- separators"

	sale, ok := saleFromRouterScript(map[string]string{"name": name, "comment": "mikhmon"}, now)
	if !ok {
		t.Fatalf("expected mikhmon report script to parse")
	}

	if sale.Comment != "note -|- with -|- separators" {
		t.Fatalf("expected old comment to preserve separators, got %q", sale.Comment)
	}
}

func TestSaleFromRouterScriptRejectsUnrelatedScripts(t *testing.T) {
	now := time.Date(2026, 5, 4, 15, 30, 0, 0, time.Local)

	if sale, ok := saleFromRouterScript(map[string]string{"name": "router-maintenance", "comment": "keep"}, now); ok {
		t.Fatalf("expected unrelated script to be rejected, got %#v", sale)
	}
}

func findReportProfileSummary(summaries []reportProfileSummary, profile string) *reportProfileSummary {
	for i := range summaries {
		if summaries[i].Profile == profile {
			return &summaries[i]
		}
	}
	return nil
}
