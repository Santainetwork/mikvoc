package service

import (
	"strings"
	"testing"

	"mikvoc/internal/core"
	"mikvoc/internal/routeros"
)

func TestFilterPPPSecrets(t *testing.T) {
	list := []routeros.PPPSecret{
		{ID: "*1", Name: "user001", Profile: "10M", Comment: "batch-a"},
		{ID: "*2", Name: "user002", Profile: "20M", Comment: "batch-b", Disabled: true},
		{ID: "*3", Name: "demo", Profile: "10M", Comment: "batch-a", RemoteAddress: "10.0.0.5"},
	}
	out := filterPPPSecrets(list, core.PPPFilters{Profile: "10M"})
	if len(out) != 2 {
		t.Fatalf("profile filter: %d", len(out))
	}
	out = filterPPPSecrets(list, core.PPPFilters{Search: "demo"})
	if len(out) != 1 || out[0].Name != "demo" {
		t.Fatalf("search: %+v", out)
	}
	out = filterPPPSecrets(list, core.PPPFilters{Search: "10.0.0.5"})
	if len(out) != 1 {
		t.Fatalf("search remote: %d", len(out))
	}
	dis := true
	out = filterPPPSecrets(list, core.PPPFilters{Disabled: &dis})
	if len(out) != 1 || out[0].Name != "user002" {
		t.Fatalf("disabled: %+v", out)
	}
}

func TestPPPGeneratePreview(t *testing.T) {
	p := PPPGeneratePreview("user", 1, 3, 3)
	if strings.Join(p, ",") != "user001,user002,user003" {
		t.Fatalf("%v", p)
	}
}

func TestWritePPPSecretsCSV(t *testing.T) {
	var b strings.Builder
	err := WritePPPSecretsCSV(&b, []core.PPPSecret{
		{Name: "u1", Password: "u1", Service: "pppoe", Profile: "default"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "u1,u1,pppoe,default") {
		t.Fatalf("%s", b.String())
	}
}
