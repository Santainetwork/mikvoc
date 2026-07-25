package httpapi

import (
	"fmt"
	"html/template"
	"strings"

	"mikvoc/internal/database"
)

type hotspotView struct {
	AppName   string
	Subtitle  string
	BtnLabel  string
	Primary   string
	Bg        string
	LogoHTML  string
	InfoHTML  string
	FormHTML  string
	ClientRow string
	Redirect  string
}

func currentHotspotSettings(routerID int) map[string]string {
	return database.GetRouterSettings(routerID)
}

func normalizeHotspotColor(value, fallback string) string {
	if len(value) != 7 || value[0] != '#' {
		return fallback
	}
	for _, char := range value[1:] {
		if !('0' <= char && char <= '9') && !('a' <= char && char <= 'f') && !('A' <= char && char <= 'F') {
			return fallback
		}
	}
	return value
}

func hotspotViewFrom(settings map[string]string) hotspotView {
	get := func(key, def string) string {
		if value := strings.TrimSpace(settings[key]); value != "" {
			return value
		}
		return def
	}
	escape := func(value string) string { return template.HTMLEscapeString(value) }

	logo := escape(get("tpl_logo_text", "NET"))
	if url := strings.TrimSpace(settings["tpl_logo_url"]); url != "" {
		logo = fmt.Sprintf(`<img src="%s" alt="logo">`, escape(url))
	}
	clientRow := ""
	if get("tpl_show_info", "true") == "true" {
		clientRow = `<div class="client">IP: $(ip) &bull; MAC: $(mac)</div>`
	}

	return hotspotView{
		AppName:   escape(get("tpl_app_name", "Hotspot Login")),
		Subtitle:  escape(get("tpl_subtitle", "Masukkan username dan password untuk akses internet")),
		BtnLabel:  escape(get("tpl_btn_label", "Connect")),
		Primary:   normalizeHotspotColor(get("tpl_primary_color", "#4f46e5"), "#4f46e5"),
		Bg:        normalizeHotspotColor(get("tpl_bg_color", "#f1f5f9"), "#f1f5f9"),
		LogoHTML:  logo,
		InfoHTML:  settings["tpl_info_html"],
		FormHTML:  hotspotFormFields(get("tpl_login_mode", "both")),
		ClientRow: clientRow,
		Redirect:  escape(get("tpl_redirect_url", "$(link-orig)")),
	}
}

func hotspotFormFields(mode string) string {
	switch mode {
	case "voucher":
		return `<label>Kode Voucher<input id="username" name="username" type="text" autocomplete="off" autofocus></label><input id="password" name="password" type="hidden">`
	case "member":
		return `<label>Username<input id="username" name="username" type="text" autocomplete="username" autofocus></label><label>Password<input id="password" name="password" type="password" autocomplete="current-password"></label>`
	default:
		return `<label>Username / Voucher<input id="username" name="username" type="text" autocomplete="username" autofocus></label><label>Password<input id="password" name="password" type="password" autocomplete="current-password"></label>`
	}
}

const hotspotLoginScript = `$(if chap-id)<script src="/md5.js"></script>$(endif)<script>function doLogin(form){var u=document.getElementById('username'),p=document.getElementById('password');if(p&&!p.value)p.value=u.value;var id='$(chap-id)',challenge='$(chap-challenge)';if(id&&typeof hexMD5==='function')form.password.value=hexMD5(id+p.value+challenge);return true}</script>`

func renderVariantLogin(variant string, settings map[string]string) string {
	v := hotspotViewFrom(settings)
	switch normalizeHotspotVariant(variant) {
	case "informative":
		return renderInformativeLogin(v)
	case "minimal":
		return renderMinimalLogin(v)
	case "cafe":
		return renderCafeLogin(v)
	default:
		return renderModernLogin(v)
	}
}

func loginForm(v hotspotView) string {
	return fmt.Sprintf(`<div class="error">$(if error)$(error)$(endif)</div><form action="$(link-login-only)" method="post" onsubmit="return doLogin(this)"><input type="hidden" name="dst" value="%s">%s<button type="submit">%s</button></form>%s`, v.Redirect, v.FormHTML, v.BtnLabel, v.ClientRow)
}

func renderModernLogin(v hotspotView) string {
	return fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s</title><style>:root{--p:%s;--b:%s}*{box-sizing:border-box}body{margin:0;background:var(--b);font:16px system-ui;display:grid;place-items:center;min-height:100vh}.card{background:#fff;width:min(390px,92%%);padding:32px;border-radius:20px;box-shadow:0 18px 45px #0002;text-align:center}.logo{width:64px;height:64px;margin:auto;display:grid;place-items:center;background:var(--p);color:#fff;border-radius:50%%;font-weight:800;overflow:hidden}.logo img{width:100%%;height:100%%;object-fit:contain}h1{margin:18px 0 4px}p,.client{color:#64748b;font-size:13px}label{display:block;text-align:left;margin:14px 0;font-size:13px}input{display:block;width:100%%;padding:12px;margin-top:5px;border:1px solid #cbd5e1;border-radius:8px}button{width:100%%;padding:13px;border:0;border-radius:8px;background:var(--p);color:#fff;font-weight:700}.error{color:#dc2626}</style></head><body><main class="card"><div class="logo">%s</div><h1>%s</h1><p>%s</p>%s</main>%s</body></html>`, v.AppName, v.Primary, v.Bg, v.LogoHTML, v.AppName, v.Subtitle, loginForm(v), hotspotLoginScript)
}

func renderInformativeLogin(v hotspotView) string {
	return fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s</title><style>:root{--p:%s;--b:%s}*{box-sizing:border-box}body{margin:0;background:var(--b);font:16px system-ui;display:grid;place-items:center;min-height:100vh}.shell{display:grid;grid-template-columns:1fr 1fr;width:min(820px,94%%);background:#fff;border-radius:18px;overflow:hidden;box-shadow:0 20px 50px #0002}.pkg,.login{padding:34px}.pkg{background:var(--p);color:#fff}.logo img{max-width:64px;max-height:64px}label{display:block;margin:13px 0;font-size:13px}input{display:block;width:100%%;padding:11px;margin-top:4px}button{width:100%%;padding:12px;background:var(--p);color:#fff;border:0}.client,.error{font-size:12px}.error{color:#c00}@media(max-width:650px){.shell{grid-template-columns:1fr}.pkg{padding:22px}}</style></head><body><main class="shell"><section class="pkg"><div class="logo">%s</div><h2>%s</h2>%s</section><section class="login"><h1>%s</h1><p>%s</p>%s</section></main>%s</body></html>`, v.AppName, v.Primary, v.Bg, v.LogoHTML, v.AppName, v.InfoHTML, v.AppName, v.Subtitle, loginForm(v), hotspotLoginScript)
}

func renderMinimalLogin(v hotspotView) string {
	return fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s</title><style>body{max-width:340px;margin:48px auto;padding:16px;background:%s;color:#111;font:16px Arial,sans-serif}.logo img{max-width:52px;max-height:52px}label{display:block;margin:12px 0}input,button{box-sizing:border-box;width:100%%;padding:11px}button{background:%s;color:#fff;border:0}.client{font-size:12px}.error{color:#b00}</style></head><body><div class="logo">%s</div><h1>%s</h1><p>%s</p>%s%s</body></html>`, v.AppName, v.Bg, v.Primary, v.LogoHTML, v.AppName, v.Subtitle, loginForm(v), hotspotLoginScript)
}

func renderCafeLogin(v hotspotView) string {
	return fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s</title><style>:root{--neon:%s}*{box-sizing:border-box}body{margin:0;background:#080b12;color:#e5e7eb;font:16px system-ui;display:grid;place-items:center;min-height:100vh}.arena{width:min(420px,92%%);padding:34px;background:#111827;border:1px solid var(--neon);box-shadow:0 0 28px var(--neon)}.logo{color:var(--neon);font-size:24px;font-weight:900}.logo img{max-width:70px;max-height:70px}label{display:block;margin:14px 0;color:#9ca3af}input{display:block;width:100%%;padding:12px;margin-top:5px;background:#030712;color:#fff;border:1px solid #374151}button{width:100%%;padding:13px;background:var(--neon);border:0;font-weight:900}.client{font-size:12px;color:#9ca3af}.error{color:#fb7185}</style></head><body><main class="arena"><div class="logo">%s</div><h1>%s</h1><p>%s</p>%s</main>%s</body></html>`, v.AppName, v.Primary, v.LogoHTML, v.AppName, v.Subtitle, loginForm(v), hotspotLoginScript)
}

func renderHotspotStatus(v hotspotView) string {
	return fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s - Online</title>$(if refresh-timeout)<meta http-equiv="refresh" content="$(refresh-timeout-secs)">$(endif)<style>body{background:%s;font:16px system-ui;display:grid;place-items:center;min-height:100vh}.card{background:#fff;padding:30px;border-radius:16px}button{background:%s;color:#fff;border:0;padding:12px;width:100%%}</style></head><body><main class="card"><h1>$(username)</h1><p>IP: $(ip)</p><p>MAC: $(mac)</p><p>Upload / Download: $(bytes-in-nice) / $(bytes-out-nice)</p><p>Online: $(uptime)</p>$(if session-time-left)<p>Sisa waktu: $(session-time-left)</p>$(endif)$(if login-by == 'mac')<p>Login via MAC</p>$(else)<form action="$(link-logout)"><button>Logout</button></form>$(endif)</main></body></html>`, v.AppName, v.Bg, v.Primary)
}

func renderHotspotLogout(v hotspotView) string {
	return fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Logged Out</title><style>body{background:%s;font:16px system-ui;display:grid;place-items:center;min-height:100vh}.card{background:#fff;padding:30px;border-radius:16px}a{display:block;background:%s;color:#fff;padding:12px;text-align:center;text-decoration:none}</style></head><body><main class="card"><h1>Sesi Berakhir</h1><p>Terima kasih, <strong>$(username)</strong></p><p>Total online: $(uptime)</p><p>Data: $(bytes-in-nice) / $(bytes-out-nice)</p><a href="$(link-login)">Login Kembali</a></main></body></html>`, v.Bg, v.Primary)
}

func customHotspotHTMLFor(settings map[string]string) (string, string, string) {
	set, err := assembleTemplateFiles(settings)
	if err == nil {
		return string(set.Get("login.html")), string(set.Get("status.html")), string(set.Get("logout.html"))
	}
	v := hotspotViewFrom(settings)
	return invalidCustomLoginDocument(err), renderHotspotStatus(v), renderHotspotLogout(v)
}
