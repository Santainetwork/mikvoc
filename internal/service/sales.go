package service

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"mikvoc/internal/core"
	"mikvoc/internal/repository"
	"mikvoc/internal/routeros"
)

type RouterScriptSale struct {
	Username  string
	Profile   string
	Comment   string
	Price     int
	CreatedAt string
	SourceKey string
}

type SalesService struct {
	pool  *Pool
	sales repository.SaleRepo
}

func NewSales(pool *Pool, sales repository.SaleRepo) *SalesService {
	return &SalesService{pool: pool, sales: sales}
}

func (s *SalesService) List(routerID int, from, to string) ([]core.Sale, error) {
	return s.sales.GetSales(routerID, from, to)
}

func (s *SalesService) SyncFromRouter(routerID int) (inserted int, err error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return 0, err
	}
	return syncSalesFromClient(cl, routerID, s.sales, false)
}

func (s *SalesService) PurgeSyncedScripts(routerID int) (removed int, err error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return 0, err
	}
	rows, err := cl.Run("/system/script/print", nil)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	for _, r := range rows {
		sale, ok := SaleFromRouterScript(r, now)
		if !ok {
			continue
		}
		if _, err := cl.Run("/system/script/remove", map[string]string{"=.id": r[".id"]}); err != nil {
			return removed, fmt.Errorf("remove script %s: %w", sale.SourceKey, err)
		}
		removed++
	}
	return removed, nil
}

func SyncSalesFromClient(cl *routeros.Client, routerID int, sales repository.SaleRepo, purge bool) (inserted int, err error) {
	return syncSalesFromClient(cl, routerID, sales, purge)
}

func syncSalesFromClient(cl *routeros.Client, routerID int, sales repository.SaleRepo, purge bool) (inserted int, err error) {
	if cl == nil {
		return 0, nil
	}
	rows, err := cl.Run("/system/script/print", nil)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	for _, r := range rows {
		sale, ok := SaleFromRouterScript(r, now)
		if !ok {
			continue
		}
		if sales != nil {
			okIns, addErr := sales.AddSaleWithTimeIdempotent(
				routerID, sale.Username, sale.Profile, sale.Comment, sale.Price, sale.CreatedAt, sale.SourceKey,
			)
			if addErr != nil {
				return inserted, addErr
			}
			if okIns {
				inserted++
			}
		}
		if purge {
			_, _ = cl.Run("/system/script/remove", map[string]string{"=.id": r[".id"]})
		}
	}
	return inserted, nil
}

func SaleFromRouterScript(row map[string]string, now time.Time) (RouterScriptSale, bool) {
	name := row["name"]
	if strings.HasPrefix(name, "mikvoc-report-") {
		parts := strings.Split(strings.TrimPrefix(name, "mikvoc-report-"), "|")
		if len(parts) < 4 {
			return RouterScriptSale{}, false
		}
		price, _ := strconv.Atoi(strings.TrimSpace(parts[3]))
		return RouterScriptSale{
			Username:  parts[2],
			Profile:   "hotspot",
			Comment:   name,
			Price:     price,
			CreatedAt: routerScriptCreatedAt(parts[0], parts[1], now),
			SourceKey: name,
		}, true
	}

	if row["comment"] == "mikhmon" && strings.Contains(name, "-|-") {
		parts := strings.SplitN(name, "-|-", 9)
		if len(parts) < 4 {
			return RouterScriptSale{}, false
		}
		price, _ := strconv.Atoi(strings.TrimSpace(parts[3]))
		profile := "hotspot"
		if len(parts) > 7 && strings.TrimSpace(parts[7]) != "" {
			profile = strings.TrimSpace(parts[7])
		}
		comment := name
		if len(parts) > 8 && strings.TrimSpace(parts[8]) != "" {
			comment = parts[8]
		}
		return RouterScriptSale{
			Username:  parts[2],
			Profile:   profile,
			Comment:   comment,
			Price:     price,
			CreatedAt: routerScriptCreatedAt(parts[0], parts[1], now),
			SourceKey: name,
		}, true
	}

	return RouterScriptSale{}, false
}

func routerScriptCreatedAt(dateStr, timeStr string, now time.Time) string {
	datePart := routerScriptDate(dateStr)
	timePart := strings.TrimSpace(timeStr)
	if datePart == "" || timePart == "" {
		return now.Format("2006-01-02 15:04:05")
	}
	return fmt.Sprintf("%s %s", datePart, timePart)
}

func routerScriptDate(dateStr string) string {
	dateStr = strings.TrimSpace(dateStr)
	if dateStr == "" {
		return ""
	}
	if strings.Contains(dateStr, "/") {
		parts := strings.Split(dateStr, "/")
		if len(parts) != 3 {
			return ""
		}
		month, ok := routerOSMonth(parts[0])
		if !ok {
			return ""
		}
		day, err := strconv.Atoi(parts[1])
		if err != nil {
			return ""
		}
		year, err := strconv.Atoi(parts[2])
		if err != nil {
			return ""
		}
		date := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
		if date.Year() != year || date.Month() != month || date.Day() != day {
			return ""
		}
		return date.Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", dateStr); err != nil {
		return ""
	}
	return dateStr
}

func routerOSMonth(month string) (time.Month, bool) {
	switch strings.ToLower(strings.TrimSpace(month)) {
	case "jan":
		return time.January, true
	case "feb":
		return time.February, true
	case "mar":
		return time.March, true
	case "apr":
		return time.April, true
	case "may":
		return time.May, true
	case "jun":
		return time.June, true
	case "jul":
		return time.July, true
	case "aug":
		return time.August, true
	case "sep":
		return time.September, true
	case "oct":
		return time.October, true
	case "nov":
		return time.November, true
	case "dec":
		return time.December, true
	default:
		return 0, false
	}
}
