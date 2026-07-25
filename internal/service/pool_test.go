package service

import (
	"testing"

	"mikvoc/internal/core"
)

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
