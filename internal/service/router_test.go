package service

import (
	"testing"
	"time"

	"mikvoc/internal/core"
)

type routerRepoStub struct {
	saved *core.Router
}

func (r *routerRepoStub) ListRouters() ([]core.Router, error) { return nil, nil }
func (r *routerRepoStub) GetRouter(int) (*core.Router, error) { return nil, nil }
func (r *routerRepoStub) DeleteRouter(int) error              { return nil }
func (r *routerRepoStub) SaveRouter(router *core.Router) error {
	r.saved = router
	return nil
}

func TestRouterSaveDoesNotConnect(t *testing.T) {
	repo := &routerRepoStub{}
	svc := NewRouter(repo, NewPool())
	router := &core.Router{ID: 1, IP: "192.0.2.1", Port: "8728", Username: "admin", Password: "secret"}

	started := time.Now()
	if err := svc.Save(router); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("Save() took %v; persistence must not dial RouterOS", elapsed)
	}
	if repo.saved != router {
		t.Fatal("router was not persisted")
	}
	if svc.pool.Client(router.ID) != nil {
		t.Fatal("Save() unexpectedly opened a connection")
	}
}
