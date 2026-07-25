package service

import (
	"errors"
	"testing"

	"mikvoc/internal/core"
	"mikvoc/internal/routeros"
)

func TestGetByName_FromCache(t *testing.T) {
	p := NewPool()
	p.users.set(1, []routeros.HotspotUser{
		{ID: "*1", Name: "alice", Profile: "1hari", Comment: "vc-a"},
		{ID: "*2", Name: "bob", Profile: "2hari"},
	})
	svc := NewUser(p)
	u, err := svc.GetByName(1, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != "*2" || u.Name != "bob" {
		t.Fatalf("got %+v", u)
	}
	if _, err := svc.GetByName(1, "missing"); !errors.Is(err, core.ErrNotFound) && err == nil {
		// no client → RequireClient fails with ErrNotConnected after cache miss
	}
	_, err = svc.GetByName(1, "missing")
	if err == nil {
		t.Fatal("expected error for missing user")
	}
	if !errors.Is(err, core.ErrNotConnected) && !errors.Is(err, core.ErrNotFound) {
		// cache miss then no client → ErrNotConnected
		if !errors.Is(err, core.ErrNotConnected) {
			t.Fatalf("err=%v", err)
		}
	}
	_, err = svc.GetByName(1, "  ")
	if !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("empty name err=%v", err)
	}
}

func TestRemoveByComment_MultiProfileAndNotFound(t *testing.T) {
	p := NewPool()
	p.users.set(3, []routeros.HotspotUser{
		{ID: "*1", Name: "a", Profile: "default", Comment: "batch-a", Uptime: "0s", BytesIn: "0", BytesOut: "0"},
		{ID: "*2", Name: "b", Profile: "premium", Comment: "batch-a", Uptime: "0s", BytesIn: "0", BytesOut: "0"},
		{ID: "*3", Name: "c", Profile: "premium", Comment: "batch-a", Uptime: "1h", BytesIn: "1", BytesOut: "1"},
		{ID: "*4", Name: "d", Profile: "vip", Comment: "other", Uptime: "0s", BytesIn: "0", BytesOut: "0"},
	})
	svc := NewUser(p)

	n, err := svc.RemoveByComment(3, "batch-a", "")
	if n != 0 {
		t.Fatalf("removed=%d", n)
	}
	var multi *CommentMultiProfileError
	if !errors.As(err, &multi) {
		t.Fatalf("want multi profile err, got %v", err)
	}
	if len(multi.Profiles) != 2 {
		t.Fatalf("profiles=%v", multi.Profiles)
	}
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("unwrap conflict: %v", err)
	}

	n, err = svc.RemoveByComment(3, "nope", "")
	if n != 0 || !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("n=%d err=%v", n, err)
	}

	n, err = svc.RemoveByComment(3, "", "")
	if n != 0 || !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func TestCommentRemovalCandidates_Service(t *testing.T) {
	users := []routeros.HotspotUser{
		{ID: "*1", Profile: "default", Comment: "batch-a", Uptime: ""},
		{ID: "*2", Profile: "premium", Comment: "batch-a", Uptime: ""},
		{ID: "*3", Profile: "premium", Comment: "batch-a", Uptime: "2m"},
	}
	got := commentRemovalCandidates(users, "batch-a", "premium")
	if len(got) != 1 || got[0].ID != "*2" {
		t.Fatalf("got=%v", got)
	}
}
