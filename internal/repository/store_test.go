package repository

import (
	"errors"
	"testing"

	"mikvoc/internal/core"
	"mikvoc/internal/database"
)

func setupTestDB(t *testing.T) *Store {
	t.Helper()
	if err := database.Init(":memory:", "test-secret-key-32bytes-long!!"); err != nil {
		t.Fatalf("init: %v", err)
	}
	t.Cleanup(func() {
		if database.DB != nil {
			_ = database.DB.Close()
		}
	})
	return NewStore()
}

func TestListRoutersEmpty(t *testing.T) {
	s := setupTestDB(t)
	list, err := s.ListRouters()
	if err != nil {
		t.Fatalf("ListRouters: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("want empty, got %d", len(list))
	}
}

func TestSaveGetDeleteRouter(t *testing.T) {
	s := setupTestDB(t)

	r := &core.Router{
		Name:            "r1",
		IP:              "10.0.0.1",
		Port:            "8728",
		Username:        "admin",
		Password:        "secret",
		SortOrder:       1,
		VoucherTemplate: "classic",
	}
	if err := s.SaveRouter(r); err != nil {
		t.Fatalf("SaveRouter: %v", err)
	}
	if r.ID == 0 {
		t.Fatal("ID not set after save")
	}

	got, err := s.GetRouter(r.ID)
	if err != nil {
		t.Fatalf("GetRouter: %v", err)
	}
	if got.Name != r.Name || got.IP != r.IP || got.Port != r.Port ||
		got.Username != r.Username || got.Password != r.Password ||
		got.SortOrder != r.SortOrder || got.VoucherTemplate != r.VoucherTemplate {
		t.Fatalf("mismatch: got %+v want %+v", got, r)
	}

	list, err := s.ListRouters()
	if err != nil {
		t.Fatalf("ListRouters: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 router, got %d", len(list))
	}
	if list[0].Password != "secret" {
		t.Fatalf("list password = %q, want secret", list[0].Password)
	}

	if err := s.DeleteRouter(r.ID); err != nil {
		t.Fatalf("DeleteRouter: %v", err)
	}
	_, err = s.GetRouter(r.ID)
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("after delete: err=%v, want ErrNotFound", err)
	}
}

func TestGetRouterNotFound(t *testing.T) {
	s := setupTestDB(t)
	_, err := s.GetRouter(999)
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("err=%v, want ErrNotFound", err)
	}
}

func TestAdminRoleAndAudit(t *testing.T) {
	s := setupTestDB(t)

	admins, err := s.ListAdmins()
	if err != nil {
		t.Fatalf("ListAdmins: %v", err)
	}
	if len(admins) < 1 {
		t.Fatal("expected default admin")
	}
	if admins[0].Role != core.RoleOwner {
		t.Fatalf("default role=%q want owner", admins[0].Role)
	}

	if err := s.CreateAdmin("ops", "hash1", core.RoleOperator); err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}
	got, err := s.GetAdminByUsername("ops")
	if err != nil {
		t.Fatalf("GetAdminByUsername: %v", err)
	}
	if got.Role != core.RoleOperator || got.Username != "ops" {
		t.Fatalf("got %+v", got)
	}
	byID, err := s.GetAdminByID(got.ID)
	if err != nil || byID.Username != "ops" {
		t.Fatalf("GetAdminByID: %v %+v", err, byID)
	}

	if err := s.UpdateAdmin(got.ID, "ops2", "", core.RoleViewer); err != nil {
		t.Fatalf("UpdateAdmin: %v", err)
	}
	got, err = s.GetAdminByID(got.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "ops2" || got.Role != core.RoleViewer || got.PasswordHash != "hash1" {
		t.Fatalf("update mismatch: %+v", got)
	}

	if err := s.AddAudit(got.ID, "ops2", "login", "ops2"); err != nil {
		t.Fatalf("AddAudit: %v", err)
	}
	if err := s.AddAudit(1, "admin", "user_remove", "u1"); err != nil {
		t.Fatalf("AddAudit2: %v", err)
	}
	entries, err := s.ListAudit(10)
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	if len(entries) < 2 {
		t.Fatalf("want >=2 audit, got %d", len(entries))
	}
	if entries[0].Action != "user_remove" {
		t.Fatalf("newest first: got %q", entries[0].Action)
	}

	if err := s.DeleteAdmin(got.ID); err != nil {
		t.Fatalf("DeleteAdmin: %v", err)
	}
	_, err = s.GetAdminByID(got.ID)
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
}

func TestMigrateHasRoleColumn(t *testing.T) {
	s := setupTestDB(t)
	var name string
	err := s.DB().QueryRow(
		`SELECT name FROM pragma_table_info('admins') WHERE name='role'`,
	).Scan(&name)
	if err != nil || name != "role" {
		t.Fatalf("role column missing: %v %q", err, name)
	}
	var tbl string
	err = s.DB().QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='audit_log'`,
	).Scan(&tbl)
	if err != nil || tbl != "audit_log" {
		t.Fatalf("audit_log missing: %v %q", err, tbl)
	}
}
