package service

import (
	"testing"
	"time"

	"mikvoc/internal/core"
)

type memSaleRepo struct {
	keys map[string]bool
	n    int
}

func (m *memSaleRepo) AddSale(routerID int, username, profile, comment string, price int) error {
	return nil
}

func (m *memSaleRepo) AddSaleWithTime(routerID int, username, profile, comment string, price int, timestamp string) error {
	return nil
}

func (m *memSaleRepo) AddSaleWithTimeIdempotent(routerID int, username, profile, comment string, price int, timestamp, sourceKey string) (bool, error) {
	if m.keys == nil {
		m.keys = map[string]bool{}
	}
	k := sourceKey
	if k == "" {
		k = comment
	}
	if m.keys[k] {
		return false, nil
	}
	m.keys[k] = true
	m.n++
	return true, nil
}

func (m *memSaleRepo) GetSales(routerID int, from, to string) ([]core.Sale, error) {
	return nil, nil
}

func (m *memSaleRepo) GetSalesTotalByDay(routerID int, from, to string) ([]map[string]interface{}, error) {
	return nil, nil
}

func TestSaleFromRouterScriptSourceKey(t *testing.T) {
	now := time.Date(2026, 5, 4, 15, 30, 0, 0, time.Local)
	name := "mikvoc-report-2026-05-04|13:14:15|voucher-01|5000"
	sale, ok := SaleFromRouterScript(map[string]string{"name": name}, now)
	if !ok || sale.SourceKey != name {
		t.Fatalf("source key: ok=%v sale=%#v", ok, sale)
	}

	mname := "may/04/2026-|-09:08:07-|-user-01-|-10000-|-10.5.50.1-|-AA:BB:CC:DD:EE:FF-|-1d-|-profile-1d-|-original comment"
	sale, ok = SaleFromRouterScript(map[string]string{"name": mname, "comment": "mikhmon"}, now)
	if !ok || sale.SourceKey != mname {
		t.Fatalf("mikhmon source key: ok=%v sale=%#v", ok, sale)
	}
}

func TestIdempotentInsertViaRepo(t *testing.T) {
	repo := &memSaleRepo{}
	key := "mikvoc-report-2026-05-04|13:14:15|voucher-01|5000"
	ok, err := repo.AddSaleWithTimeIdempotent(1, "voucher-01", "hotspot", key, 5000, "2026-05-04 13:14:15", key)
	if err != nil || !ok {
		t.Fatalf("first: ok=%v err=%v", ok, err)
	}
	ok, err = repo.AddSaleWithTimeIdempotent(1, "voucher-01", "hotspot", key, 5000, "2026-05-04 13:14:15", key)
	if err != nil {
		t.Fatal(err)
	}
	if ok || repo.n != 1 {
		t.Fatalf("second should ignore: ok=%v n=%d", ok, repo.n)
	}
}
