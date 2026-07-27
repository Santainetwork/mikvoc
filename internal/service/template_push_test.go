package service

import (
	"errors"
	"reflect"
	"testing"

	"mikvoc/internal/core"
	"mikvoc/internal/repository"
)

type templateSettingRepo struct {
	repository.SettingRepo
	settingsRouterID int
	settings         map[string]string
	voucherRouterID  int
	voucher          string
	savedRouterID    int
	savedUpdates     map[string]string
	saveErr          error
	voucherWriteID   int
	voucherID        string
	voucherErr       error
}

func (r *templateSettingRepo) GetRouterSettings(routerID int) map[string]string {
	r.settingsRouterID = routerID
	return r.settings
}

func (r *templateSettingRepo) SetTemplateSettings(routerID int, updates map[string]string) error {
	r.savedRouterID = routerID
	r.savedUpdates = updates
	return r.saveErr
}

func (r *templateSettingRepo) GetRouterVoucherTemplate(routerID int) string {
	r.voucherRouterID = routerID
	return r.voucher
}

func (r *templateSettingRepo) SetRouterVoucherTemplate(routerID int, templateID string) error {
	r.voucherWriteID = routerID
	r.voucherID = templateID
	return r.voucherErr
}

func TestTemplateSettings(t *testing.T) {
	want := map[string]string{"tpl_app_name": "MikVoc"}
	repo := &templateSettingRepo{settings: want}
	svc := NewTemplate(NewPool(), repo)

	if got := svc.Settings(7); !reflect.DeepEqual(got, want) {
		t.Fatalf("Settings() = %#v, want %#v", got, want)
	}
	if repo.settingsRouterID != 7 {
		t.Fatalf("GetRouterSettings() routerID = %d, want 7", repo.settingsRouterID)
	}
}

func TestTemplateSaveUsesAtomicRepositoryOperation(t *testing.T) {
	wantErr := errors.New("save failed")
	updates := map[string]string{"tpl_app_name": "MikVoc", "tpl_variant": "modern"}
	repo := &templateSettingRepo{saveErr: wantErr}
	svc := NewTemplate(NewPool(), repo)

	if err := svc.Save(7, updates); !errors.Is(err, wantErr) {
		t.Fatalf("Save() error = %v, want %v", err, wantErr)
	}
	if repo.savedRouterID != 7 || !reflect.DeepEqual(repo.savedUpdates, updates) {
		t.Fatalf("SetTemplateSettings() = (%d, %#v), want (7, %#v)", repo.savedRouterID, repo.savedUpdates, updates)
	}
}

func TestTemplateVoucherTemplate(t *testing.T) {
	repo := &templateSettingRepo{voucher: "compact"}
	svc := NewTemplate(NewPool(), repo)

	if got := svc.VoucherTemplate(7); got != "compact" {
		t.Fatalf("VoucherTemplate() = %q, want compact", got)
	}
	if repo.voucherRouterID != 7 {
		t.Fatalf("GetRouterVoucherTemplate() routerID = %d, want 7", repo.voucherRouterID)
	}
}

func TestTemplateSetVoucherTemplate(t *testing.T) {
	wantErr := errors.New("save failed")
	repo := &templateSettingRepo{voucherErr: wantErr}
	svc := NewTemplate(NewPool(), repo)

	if err := svc.SetVoucherTemplate(7, "compact"); !errors.Is(err, wantErr) {
		t.Fatalf("SetVoucherTemplate() error = %v, want %v", err, wantErr)
	}
	if repo.voucherWriteID != 7 || repo.voucherID != "compact" {
		t.Fatalf("SetRouterVoucherTemplate() = (%d, %q), want (7, compact)", repo.voucherWriteID, repo.voucherID)
	}
}

func TestTemplatePushDisconnected(t *testing.T) {
	svc := NewTemplate(NewPool(), &templateSettingRepo{})

	dir, err := svc.Push(7, "login", "status", "logout")
	if dir != "" || !errors.Is(err, core.ErrNotConnected) {
		t.Fatalf("Push() = (%q, %v), want (\"\", %v)", dir, err, core.ErrNotConnected)
	}
}
