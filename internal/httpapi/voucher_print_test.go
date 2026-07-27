package httpapi

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mikvoc/internal/core"
	"mikvoc/internal/database"
	"mikvoc/internal/middleware"
	"mikvoc/internal/repository"
	"mikvoc/internal/service"
)

func TestVoucherPrintHandlersRequireTemplateService(t *testing.T) {
	for _, tt := range []struct {
		name   string
		path   string
		handle func(*App, http.ResponseWriter, *http.Request)
	}{
		{"batch", "/hotspot/users/print", func(a *App, w http.ResponseWriter, r *http.Request) { a.HandlePrint(w, r) }},
		{"quick", "/hotspot/users/quickprint?username=x", func(a *App, w http.ResponseWriter, r *http.Request) { a.HandleQuickPrint(w, r) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tt.handle(&App{}, rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503", rec.Code)
			}
		})
	}
}

func TestVoucherTemplateSelectionPrecedence(t *testing.T) {
	tests := []struct {
		name, query, small, saved, want string
		batch                           bool
	}{
		{"query", "grid", "yes", "thermal", "grid", true},
		{"batch legacy small", "", "yes", "thermal", "compact", true},
		{"quick ignores small", "", "yes", "thermal", "thermal", false},
		{"saved", "", "", "thermal", "thermal", true},
		{"default", "", "", "", "classic", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := voucherTemplateID(tt.query, tt.small, tt.saved, tt.batch); got != tt.want {
				t.Fatalf("voucherTemplateID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHandlePrintUsesActiveRouterTemplateAndMergedBranding(t *testing.T) {
	withTestDB(t)
	middleware.InitSession("voucher-print-router")
	if err := database.SetTemplateSettings(0, map[string]string{
		"tpl_app_name":  "Global Cafe",
		"tpl_logo_text": "GLOBAL",
		"tpl_dns_name":  "global.example",
	}); err != nil {
		t.Fatal(err)
	}
	router := &database.Router{Name: "Router Seven", IP: "127.0.0.1", Username: "admin", VoucherTemplate: "thermal"}
	if err := database.SaveRouter(router); err != nil {
		t.Fatal(err)
	}
	if err := database.SetTemplateSettings(router.ID, map[string]string{"tpl_logo_text": "R7"}); err != nil {
		t.Fatal(err)
	}

	address := startVoucherRouterOS(t)
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatal(err)
	}
	pool := service.NewPool()
	if err := pool.Connect(&core.Router{ID: router.ID, IP: host, Port: port, Username: "admin"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Clear)
	store := repository.NewStore()
	app := NewApp(store, pool, nil, nil, service.NewUser(pool), service.NewProfile(pool), nil)
	app.Template = service.NewTemplate(pool, store)

	tests := []struct {
		name, path, marker string
	}{
		{"saved router template", "/hotspot/users/print", "height:100px"},
		{"query overrides saved template", "/hotspot/users/print?template=grid", `class="sheet"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := voucherRequestForRouter(t, app, tt.path, router.ID)
			rec := httptest.NewRecorder()
			app.HandlePrint(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			for _, want := range []string{tt.marker, "Global Cafe", "R7", "global.example"} {
				if !strings.Contains(body, want) {
					t.Fatalf("rendered voucher missing %q", want)
				}
			}
		})
	}
}

func TestHandleQuickPrintUsesActiveRouterTemplateAndMergedBranding(t *testing.T) {
	withTestDB(t)
	middleware.InitSession("voucher-quick-print-router")
	if err := database.SetTemplateSettings(0, map[string]string{
		"tpl_app_name": "Global Quick Cafe",
		"tpl_dns_name": "quick.example",
	}); err != nil {
		t.Fatal(err)
	}
	router := &database.Router{Name: "Quick Router", IP: "127.0.0.1", Username: "admin", VoucherTemplate: "thermal"}
	if err := database.SaveRouter(router); err != nil {
		t.Fatal(err)
	}
	if err := database.SetTemplateSettings(router.ID, map[string]string{"tpl_logo_text": "QR7"}); err != nil {
		t.Fatal(err)
	}

	address := startVoucherRouterOS(t)
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatal(err)
	}
	pool := service.NewPool()
	if err := pool.Connect(&core.Router{ID: router.ID, IP: host, Port: port, Username: "admin"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Clear)
	store := repository.NewStore()
	app := NewApp(store, pool, nil, nil, service.NewUser(pool), service.NewProfile(pool), nil)
	app.Template = service.NewTemplate(pool, store)

	req := voucherRequestForRouter(t, app, "/hotspot/users/quickprint?username=voucher-1", router.ID)
	rec := httptest.NewRecorder()
	app.HandleQuickPrint(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{"height:100px", "Global Quick Cafe", "QR7", "quick.example"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("rendered quick voucher missing %q", want)
		}
	}
}

func voucherRequestForRouter(t *testing.T, app *App, path string, routerID int) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	app.SetSessionRouterID(rec, req, routerID)
	req.Header.Set("Cookie", rec.Header().Get("Set-Cookie"))
	return req
}

func startVoucherRouterOS(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		for {
			words, err := readRouterOSSentence(reader)
			if err != nil {
				return
			}
			var rows [][]string
			switch words[0] {
			case "/ip/hotspot/user/print":
				rows = [][]string{{"!re", "=.id=*1", "=name=voucher-1", "=password=secret", "=profile=day"}}
			case "/ip/hotspot/user/profile/print":
				rows = [][]string{{"!re", "=.id=*2", "=name=day", "=price=2000", "=validity=1d"}}
			}
			rows = append(rows, []string{"!done"})
			if err := writeRouterOSSentences(conn, rows); err != nil {
				return
			}
		}
	}()
	return listener.Addr().String()
}

func readRouterOSSentence(r *bufio.Reader) ([]string, error) {
	var words []string
	for {
		length, err := readRouterOSLength(r)
		if err != nil {
			return nil, err
		}
		if length == 0 {
			return words, nil
		}
		word := make([]byte, length)
		if _, err := io.ReadFull(r, word); err != nil {
			return nil, err
		}
		words = append(words, string(word))
	}
}

func readRouterOSLength(r *bufio.Reader) (int, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	if b&0x80 == 0 {
		return int(b), nil
	}
	if b&0xC0 == 0x80 {
		next, err := r.ReadByte()
		return int(b&0x3F)<<8 | int(next), err
	}
	return 0, fmt.Errorf("unsupported RouterOS test word length prefix %#x", b)
}

func writeRouterOSSentences(w io.Writer, sentences [][]string) error {
	for _, sentence := range sentences {
		for _, word := range sentence {
			if len(word) >= 128 {
				return fmt.Errorf("RouterOS test response word too long")
			}
			if _, err := w.Write(append([]byte{byte(len(word))}, word...)); err != nil {
				return err
			}
		}
		if _, err := w.Write([]byte{0}); err != nil {
			return err
		}
	}
	return nil
}
