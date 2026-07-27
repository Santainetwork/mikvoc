package httpapi

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"mikvoc/internal/core"
	"mikvoc/internal/service"
)

type reportFilters struct {
	From    string
	To      string
	Preset  string
	Profile string
}

type reportDailySummary struct {
	Day        string
	Count      int
	Total      int
	TotalLabel string
}

type reportProfileSummary struct {
	Profile      string
	Count        int
	Revenue      int
	RevenueLabel string
}

type reportViewData struct {
	Sales             []core.Sale
	Daily             []reportDailySummary
	ProfileSummaries  []reportProfileSummary
	ProfileOptions    []string
	TotalRevenue      int
	TotalRevenueLabel string
	AverageDailyLabel string
	TotalCount        int
	From              string
	To                string
	Preset            string
	Profile           string
}

type routerScriptSale = service.RouterScriptSale

func saleFromRouterScript(row map[string]string, now time.Time) (routerScriptSale, bool) {
	return service.SaleFromRouterScript(row, now)
}

func resolveReportFilters(now time.Time, values url.Values) reportFilters {
	profile := strings.TrimSpace(values.Get("profile"))
	preset := strings.TrimSpace(values.Get("preset"))
	from := strings.TrimSpace(values.Get("from"))
	to := strings.TrimSpace(values.Get("to"))

	if from != "" || to != "" {
		if from == "" {
			from = to
		}
		if to == "" {
			to = from
		}
		return reportFilters{From: from, To: to, Preset: "custom", Profile: profile}
	}
	if preset == "" {
		preset = "this_month"
	}

	date := func(t time.Time) string { return t.Format("2006-01-02") }
	switch preset {
	case "today":
		from = date(now)
		to = from
	case "yesterday":
		yesterday := now.AddDate(0, 0, -1)
		from = date(yesterday)
		to = from
	case "last_7":
		from = date(now.AddDate(0, 0, -6))
		to = date(now)
	case "last_month":
		firstThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		lastMonthEnd := firstThisMonth.AddDate(0, 0, -1)
		lastMonthStart := time.Date(lastMonthEnd.Year(), lastMonthEnd.Month(), 1, 0, 0, 0, 0, now.Location())
		from = date(lastMonthStart)
		to = date(lastMonthEnd)
	default:
		preset = "this_month"
		from = fmt.Sprintf("%d-%02d-01", now.Year(), now.Month())
		to = date(now)
	}
	return reportFilters{From: from, To: to, Preset: preset, Profile: profile}
}

func buildReportViewData(sales []core.Sale, filters reportFilters) reportViewData {
	profileSet := map[string]bool{}
	profileSummaries := map[string]*reportProfileSummary{}
	for _, sale := range sales {
		profile := strings.TrimSpace(sale.Profile)
		if profile == "" {
			profile = "hotspot"
		}
		profileSet[profile] = true
		summary := profileSummaries[profile]
		if summary == nil {
			summary = &reportProfileSummary{Profile: profile}
			profileSummaries[profile] = summary
		}
		summary.Count++
		summary.Revenue += sale.Price
	}

	profileOptions := make([]string, 0, len(profileSet))
	for profile := range profileSet {
		profileOptions = append(profileOptions, profile)
	}
	sort.Strings(profileOptions)

	summaries := make([]reportProfileSummary, 0, len(profileSummaries))
	for _, profile := range profileOptions {
		summary := *profileSummaries[profile]
		summary.RevenueLabel = formatRupiah(summary.Revenue)
		summaries = append(summaries, summary)
	}

	filtered := make([]core.Sale, 0, len(sales))
	dailyMap := map[string]*reportDailySummary{}
	totalRevenue := 0
	for _, sale := range sales {
		if filters.Profile != "" && sale.Profile != filters.Profile {
			continue
		}
		filtered = append(filtered, sale)
		totalRevenue += sale.Price
		day := sale.CreatedAt
		if len(day) >= 10 {
			day = day[:10]
		}
		if day == "" {
			day = filters.To
		}
		daily := dailyMap[day]
		if daily == nil {
			daily = &reportDailySummary{Day: day}
			dailyMap[day] = daily
		}
		daily.Count++
		daily.Total += sale.Price
	}

	days := make([]string, 0, len(dailyMap))
	for day := range dailyMap {
		days = append(days, day)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(days)))
	daily := make([]reportDailySummary, 0, len(days))
	for _, day := range days {
		entry := *dailyMap[day]
		entry.TotalLabel = formatRupiah(entry.Total)
		daily = append(daily, entry)
	}

	average := 0
	if len(daily) > 0 {
		average = totalRevenue / len(daily)
	}

	return reportViewData{
		Sales:             filtered,
		Daily:             daily,
		ProfileSummaries:  summaries,
		ProfileOptions:    profileOptions,
		TotalRevenue:      totalRevenue,
		TotalRevenueLabel: formatRupiah(totalRevenue),
		AverageDailyLabel: formatRupiah(average),
		TotalCount:        len(filtered),
		From:              filters.From,
		To:                filters.To,
		Preset:            filters.Preset,
		Profile:           filters.Profile,
	}
}

// HandleReport renders the sales report page.
func (a *App) HandleReport(w http.ResponseWriter, r *http.Request) {
	if a.Sales == nil {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	rt := a.routerFor(r)
	routerID := 0
	if rt != nil {
		routerID = rt.ID
	}

	if routerID > 0 {
		_, _ = a.Sales.SyncFromRouter(routerID)
	}

	filters := resolveReportFilters(time.Now(), r.URL.Query())
	sales, err := a.Sales.List(routerID, filters.From, filters.To)
	if err != nil {
		http.Error(w, "failed to load sales", http.StatusInternalServerError)
		return
	}
	data := buildReportViewData(sales, filters)

	a.render(w, r, "report.html", TemplateData{
		Title:      "Laporan Penjualan — MikVoc",
		ActiveMenu: "report",
		Data:       data,
	})
}

func (a *App) HandleSalesPurge(w http.ResponseWriter, r *http.Request) {
	if a.Sales == nil {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	rt := a.routerFor(r)
	routerID := 0
	if rt != nil {
		routerID = rt.ID
	}
	if routerID == 0 {
		a.setFlash(w, r, "Error: Router tidak dipilih.")
		http.Redirect(w, r, "/report", http.StatusSeeOther)
		return
	}

	if _, err := a.Sales.SyncFromRouter(routerID); err != nil {
		a.setFlash(w, r, "Error: Gagal sync sebelum purge: "+err.Error())
		http.Redirect(w, r, "/report", http.StatusSeeOther)
		return
	}
	n, err := a.Sales.PurgeSyncedScripts(routerID)
	if err != nil {
		a.setFlash(w, r, "Error: Gagal bersihkan script: "+err.Error())
	} else {
		a.setFlash(w, r, fmt.Sprintf("Success: %d script penjualan di router dibersihkan.", n))
	}
	http.Redirect(w, r, "/report", http.StatusSeeOther)
}

// formatRupiah formats an integer to "Rp 1.000"
func formatRupiah(n int) string {
	s := strconv.Itoa(n)
	result := ""
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result += "."
		}
		result += string(c)
	}
	return "Rp " + result
}
