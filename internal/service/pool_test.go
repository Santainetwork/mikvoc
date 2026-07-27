package service

import (
	"testing"

	"mikvoc/internal/core"
	"mikvoc/internal/routeros"
)

func TestPoolConnectedCount(t *testing.T) {
	p := NewPool()
	p.clients[1] = routeros.NewClient("127.0.0.1", "8728")

	if got := p.ConnectedCount(); got != 0 {
		t.Fatalf("ConnectedCount() = %d, want 0", got)
	}
}

func TestPoolRejectsStaleConnectionAfterClear(t *testing.T) {
	p := NewPool()
	p.mu.Lock()
	p.revisions[1] = 1
	epoch := p.epoch
	p.mu.Unlock()

	p.Clear()
	stale := routeros.NewClient("127.0.0.1", "8728")
	if p.installClient(1, 1, epoch, stale) {
		t.Fatal("stale connection installed after Clear")
	}
	if p.Client(1) != nil {
		t.Fatal("stale client retained")
	}
}

func TestPool_RequireClient_NotConnected(t *testing.T) {
	p := NewPool()
	cl, err := p.RequireClient(0)
	if cl != nil || err != core.ErrNotConnected {
		t.Fatalf("id=0: cl=%v err=%v", cl, err)
	}
	cl, err = p.RequireClient(99)
	if cl != nil || err != core.ErrNotConnected {
		t.Fatalf("missing: cl=%v err=%v", cl, err)
	}
}

func TestPool_IsConnected_Empty(t *testing.T) {
	p := NewPool()
	if p.IsConnected(1) {
		t.Fatal("expected false")
	}
	if p.Client(1) != nil {
		t.Fatal("expected nil client")
	}
}

func TestPool_Disconnect_Noop(t *testing.T) {
	p := NewPool()
	p.Disconnect(0)
	p.Disconnect(42)
}
