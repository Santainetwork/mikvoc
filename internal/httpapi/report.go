package httpapi

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"mikvoc/internal/database"
	"mikvoc/internal/routeros"
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
	Sales             []database.SaleRecord
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

func buildReportViewData(sales []database.SaleRecord, filters reportFilters) reportViewData {
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

	filtered := make([]database.SaleRecord, 0, len(sales))
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
	rt := a.routerFor(r)
	routerID := 0
	if rt != nil {
		routerID = rt.ID
	}

	if a.Sales != nil && routerID > 0 {
		_, _ = a.Sales.SyncFromRouter(routerID)
	} else {
		cl := a.clientFor(r)
		if cl != nil && cl.IsConnected() {
			syncSalesFromRouter(cl, routerID)
		}
	}

	filters := resolveReportFilters(time.Now(), r.URL.Query())
	sales, _ := database.GetSales(routerID, filters.From, filters.To)
	data := buildReportViewData(sales, filters)

	a.render(w, r, "report.html", TemplateData{
		Title:      "Laporan Penjualan — MikVoc",
		ActiveMenu: "report",
		Data:       data,
	})
}

func (a *App) HandleSalesPurge(w http.ResponseWriter, r *http.Request) {
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

	if a.Sales != nil {
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
		return
	}

	cl := a.clientFor(r)
	if cl == nil || !cl.IsConnected() {
		a.setFlash(w, r, "Error: Tidak terhubung ke router.")
		http.Redirect(w, r, "/report", http.StatusSeeOther)
		return
	}
	syncSalesFromRouter(cl, routerID)
	n := purgeSalesScriptsFromRouter(cl)
	a.setFlash(w, r, fmt.Sprintf("Success: %d script penjualan di router dibersihkan.", n))
	http.Redirect(w, r, "/report", http.StatusSeeOther)
}

// syncSalesFromRouter fetches sales scripts and inserts idempotently. Does NOT delete scripts.
func syncSalesFromRouter(cl *routeros.Client, routerID int) {
	rows, err := cl.Run("/system/script/print", nil)
	if err != nil {
		return
	}
	now := time.Now()
	for _, r := range rows {
		sale, ok := saleFromRouterScript(r, now)
		if !ok {
			continue
		}
		_, _ = database.AddSaleWithTimeIdempotent(
			routerID, sale.Username, sale.Profile, sale.Comment, sale.Price, sale.CreatedAt, sale.SourceKey,
		)
	}
}

func purgeSalesScriptsFromRouter(cl *routeros.Client) int {
	rows, err := cl.Run("/system/script/print", nil)
	if err != nil {
		return 0
	}
	removed := 0
	now := time.Now()
	for _, r := range rows {
		if _, ok := saleFromRouterScript(r, now); !ok {
			continue
		}
		if _, err := cl.Run("/system/script/remove", map[string]string{"=.id": r[".id"]}); err == nil {
			removed++
		}
	}
	return removed
}

func addSaleRecord(routerID int, username, profile, comment string, price int) {
	if price > 0 {
		_ = database.AddSale(routerID, username, profile, comment, price)
	}
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
