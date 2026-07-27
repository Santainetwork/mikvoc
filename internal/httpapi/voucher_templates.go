package httpapi

import (
	"bytes"
	"fmt"
	"html/template"
	"net/url"
	"strings"

	"mikvoc/internal/routeros"
)

type VoucherTemplate struct {
	ID          string
	Name        string
	Description string
}

var BuiltinVoucherTemplates = []VoucherTemplate{
	{ID: "classic", Name: "Mikhmon Classic", Description: "Voucher hitam-putih 220px, padat, siap potong"},
	{ID: "thermal", Name: "Mikhmon Thermal", Description: "Template thermal 180px seperti Mikhmon"},
	{ID: "grid", Name: "Grid Sheet", Description: "Kartu voucher rapi untuk kertas A4 landscape"},
	{ID: "compact", Name: "Mikhmon Small", Description: "Template kecil 160px hemat kertas"},
}

type voucherContext struct {
	BrandName string
	LogoURL   string
	LogoText  string
	DNSName   string
}

type voucherProfileMeta struct {
	Price    string
	Validity string
}

type voucherItem struct {
	Num      int
	User     routeros.HotspotUser
	Meta     voucherProfileMeta
	Mode     string
	QRID     string
	LoginURL string
	InfoLine string
	StackUP  bool
	QRSize   int
}

type voucherRenderData struct {
	Ctx    voucherContext
	Items  []voucherItem
	WithQR bool
	CSS    template.CSS
	Count  int
	Tmpl   string
}

func newVoucherContext(settings map[string]string, routerName string) voucherContext {
	brandName := settings["tpl_app_name"]
	if brandName == "" {
		brandName = routerName
	}
	if brandName == "" {
		brandName = "MikVoc Hotspot"
	}

	logoText := settings["tpl_logo_text"]
	if logoText == "" {
		logoText = "NET"
	}

	dnsName := settings["tpl_dns_name"]
	if dnsName == "" {
		dnsName = "hotspot.local"
	}

	return voucherContext{
		BrandName: brandName,
		LogoURL:   settings["tpl_logo_url"],
		LogoText:  logoText,
		DNSName:   dnsName,
	}
}

func generateQuickPrintHTML(tmpl string, settings map[string]string, routerName, username, password, profile, limitUptime, limitBytesTotal, comment, price, validity string, withQR bool) string {
	ctx := newVoucherContext(settings, routerName)
	user := routeros.HotspotUser{
		ID:              "quick",
		Name:            username,
		Password:        password,
		Profile:         profile,
		Comment:         comment,
		LimitUptime:     limitUptime,
		LimitBytesTotal: limitBytesTotal,
	}
	meta := map[string]voucherProfileMeta{
		profile: {Price: price, Validity: validity},
	}
	return generateVoucherDocument(tmpl, ctx, []routeros.HotspotUser{user}, meta, withQR)
}

func generateMultiPrintHTML(tmpl string, settings map[string]string, routerName string, users []routeros.HotspotUser, metas map[string]voucherProfileMeta, withQR bool) string {
	return generateVoucherDocument(tmpl, newVoucherContext(settings, routerName), users, metas, withQR)
}

func buildVoucherItems(ctx voucherContext, users []routeros.HotspotUser, metas map[string]voucherProfileMeta, withQR bool, stackUP bool, qrSize int) []voucherItem {
	items := make([]voucherItem, 0, len(users))
	for i, u := range users {
		num := i + 1
		meta := metas[u.Profile]
		items = append(items, voucherItem{
			Num:      num,
			User:     u,
			Meta:     meta,
			Mode:     voucherMode(u),
			QRID:     qrID(u, num),
			LoginURL: loginURLForVoucher(ctx.DNSName, u.Name, u.Password),
			InfoLine: voucherInfoLine(u, meta),
			StackUP:  stackUP,
			QRSize:   qrSize,
		})
	}
	return items
}

func generateVoucherDocument(tmpl string, ctx voucherContext, users []routeros.HotspotUser, metas map[string]voucherProfileMeta, withQR bool) string {
	if tmpl == "" {
		tmpl = "classic"
	}

	var css string
	var stackUP bool
	var qrSize int
	switch tmpl {
	case "thermal":
		css = thermalVoucherCSS
		stackUP = false
		qrSize = 256
	case "grid":
		css = gridVoucherCSS
		stackUP = false
		qrSize = 92
	case "compact":
		css = compactVoucherCSS
		stackUP = false
		qrSize = 0
	default:
		tmpl = "classic"
		css = classicVoucherCSS
		stackUP = withQR
		qrSize = 256
	}

	data := voucherRenderData{
		Ctx:    ctx,
		Items:  buildVoucherItems(ctx, users, metas, withQR, stackUP, qrSize),
		WithQR: withQR,
		CSS:    template.CSS(css + baseVoucherCSS),
		Count:  len(users),
		Tmpl:   tmpl,
	}

	var buf bytes.Buffer
	if err := voucherRootTmpl.ExecuteTemplate(&buf, "document", data); err != nil {
		return fmt.Sprintf("<!DOCTYPE html><html><body>template error: %s</body></html>", template.HTMLEscapeString(err.Error()))
	}
	return buf.String()
}

type logoView struct {
	URL  template.URL
	Text string
	Size int
}

func logoArgs(ctx voucherContext, size int) logoView {
	return logoView{
		URL:  template.URL(ctx.LogoURL),
		Text: ctx.LogoText,
		Size: size,
	}
}

var voucherFuncMap = template.FuncMap{
	"formatBytes": formatBytesGo,
	"formatPrice": formatPriceGo,
	"loginURL": func(dns, user, pass string) string {
		return loginURLForVoucher(dns, user, pass)
	},
	"voucherMode": func(u routeros.HotspotUser) string {
		return voucherMode(u)
	},
	"infoLine": voucherInfoLine,
	"logoArgs": logoArgs,
}

const baseVoucherCSS = `
*{box-sizing:border-box}
.no-print{display:flex;gap:8px;justify-content:center;align-items:center;padding:10px;background:#f1f5f9;border-bottom:1px solid #dbe3ef;margin-bottom:10px;flex-wrap:wrap}
.btn{border:0;border-radius:6px;padding:7px 16px;font-size:13px;font-weight:700;cursor:pointer}
.btn-print{background:#111827;color:#fff}
.btn-close{background:#e5e7eb;color:#111827}
.hint{font-size:12px;color:#64748b;margin-left:8px}
.empty{padding:24px;font-size:13px;color:#555}
@media print{.no-print{display:none!important}}`

const classicVoucherCSS = `
	.qrcode{
		height:80px;
		width:80px;
	}
body {
  color: #000000;
  background-color: #FFFFFF;
  font-size: 14px;
  font-family: 'Helvetica', arial, sans-serif;
  margin: 0px;
  line-height: 1.25;
  -webkit-print-color-adjust: exact;
}
table.voucher {
  display: inline-block;
  border: 1px solid black;
  margin: 2px;
  vertical-align: top;
  page-break-inside: avoid;
  break-inside: avoid;
}
@page
{
  size: auto;
  margin-left: 7mm;
  margin-right: 3mm;
  margin-top: 9mm;
  margin-bottom: 3mm;
}
@media print
{
  .no-print { display:none!important }
  table { page-break-after:auto }
  tr    { page-break-inside:avoid; page-break-after:auto }
  td    { page-break-inside:avoid; page-break-after:auto }
  thead { display:table-header-group }
  tfoot { display:table-footer-group }
  table.voucher { page-break-inside:avoid; break-inside:avoid }
}
#num {
  float:right;
  display:inline-block;
}
.logo-text{display:inline-block;border:1px solid black;padding:2px 4px;font-size:10px;font-weight:bold;line-height:1}`

const thermalVoucherCSS = `
	.qrcode{
		height:100px;
		width:100px;
	}
body {
  color: #000000;
  background-color: #FFFFFF;
  font-size: 14px;
  font-family: 'Helvetica', arial, sans-serif;
  margin: 0px;
  padding: 2mm;
  -webkit-print-color-adjust: exact;
}
table.voucher {
  display: inline-block;
  border: 1px solid black;
  margin: 1mm 0;
  padding: 1mm;
  width: 58mm;
  max-width: 58mm;
  page-break-inside: avoid;
  break-inside: avoid;
  vertical-align: top;
}
@page
{
  size: auto;
  margin-left: 2mm;
  margin-right: 2mm;
  margin-top: 3mm;
  margin-bottom: 3mm;
}
@media print
{
  .no-print { display:none!important }
  table { page-break-after:auto }
  tr    { page-break-inside:avoid; page-break-after:auto }
  td    { page-break-inside:avoid; page-break-after:auto }
  thead { display:table-header-group }
  tfoot { display:table-footer-group }
  table.voucher { page-break-inside:avoid; break-inside:avoid }
}
.logo-text{display:inline-block;border:1px solid black;padding:2px 4px;font-size:10px;font-weight:bold;line-height:1}`

const gridVoucherCSS = `
@page{size:A4 landscape;margin:8mm}
body{background:#fff;color:#111;font-family:Arial,Helvetica,sans-serif;margin:0;padding:0}
.sheet{display:grid;grid-template-columns:repeat(3,1fr);gap:5mm}
.card{border:1.5px solid #111;min-height:54mm;display:grid;grid-template-columns:minmax(0,1fr) 32mm;page-break-inside:avoid;break-inside:avoid}
.left{padding:4mm}
.right{border-left:1px dashed #111;display:flex;align-items:center;justify-content:center;text-align:center;padding:3mm}
.brand{display:flex;align-items:center;gap:2mm;font-size:14px;font-weight:800;border-bottom:1px solid #111;padding-bottom:2mm;margin-bottom:3mm}
.code{font-family:'Courier New',monospace;font-size:22px;font-weight:900;letter-spacing:.6px;border:1px solid #111;padding:2mm;text-align:center;word-break:break-all}
.pass{font-family:'Courier New',monospace;font-weight:800;text-align:center;margin-top:1.5mm}
.meta{display:grid;grid-template-columns:1fr 1fr;gap:2mm;margin-top:3mm;font-size:10px}
.lbl{font-size:8px;text-transform:uppercase;color:#555;font-weight:700}
.val{font-weight:700}
.qrcode{width:92px;height:92px}
.small{font-size:10px;font-weight:700}
.empty{padding:8mm;color:#555}
@media print{.no-print{display:none!important}.card{page-break-inside:avoid;break-inside:avoid}}`

const compactVoucherCSS = `
body {
  color: #000000;
  background-color: #FFFFFF;
  font-size: 14px;
  font-family: 'Helvetica', arial, sans-serif;
  margin: 0px;
  line-height: 1.15;
  -webkit-print-color-adjust: exact;
}
table.voucher {
  display: inline-block;
  border: 1px solid black;
  margin: 1px;
  vertical-align: top;
  page-break-inside: avoid;
  break-inside: avoid;
}
@page
{
  size: auto;
  margin-left: 7mm;
  margin-right: 3mm;
  margin-top: 9mm;
  margin-bottom: 3mm;
}
@media print
{
  .no-print { display:none!important }
  table { page-break-after:auto }
  tr    { page-break-inside:avoid; page-break-after:auto }
  td    { page-break-inside:avoid; page-break-after:auto }
  thead { display:table-header-group }
  tfoot { display:table-footer-group }
  table.voucher { page-break-inside:avoid; break-inside:avoid }
}
#num {
  float:right;
  display:inline-block;
}`

const voucherTemplatesHTML = `
{{define "document"}}<!DOCTYPE html><html lang="id"><head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta http-equiv="pragma" content="no-cache">
<title>Voucher-{{.Ctx.BrandName}}</title>{{if .WithQR}}<script src="/static/js/qrious.min.js"></script>{{end}}
<style>{{.CSS}}</style>
</head><body onload="window.print()">
<div class="no-print">
  <button class="btn btn-print" onclick="window.print()">Print</button>
  <button class="btn btn-close" onclick="window.close()">Tutup</button>
  <span class="hint">{{.Count}} voucher · template Mikhmon</span>
</div>
{{if eq .Tmpl "thermal"}}{{template "thermal" .}}
{{else if eq .Tmpl "grid"}}{{template "grid" .}}
{{else if eq .Tmpl "compact"}}{{template "compact" .}}
{{else}}{{template "classic" .}}
{{end}}
</body></html>{{end}}

{{define "logo"}}{{if .URL}}<img src="{{.URL}}" alt="logo" style="height:{{.Size}}px;border:0;">{{else}}<span class="logo-text">{{.Text}}</span>{{end}}{{end}}

{{define "round-logo"}}{{if .URL}}<img src="{{.URL}}" alt="logo" style="width:{{.Size}}px;height:{{.Size}}px;object-fit:contain;border:0">{{else}}<span>{{.Text}}</span>{{end}}{{end}}

{{define "qr"}}{{if .WithQR}}
	<canvas class="qrcode" id="{{.Item.QRID}}"></canvas>
    <script>
      (function() {
        new QRious({
          element: document.getElementById({{.Item.QRID}}),
          value: {{.Item.LoginURL}},
          size:{{.Item.QRSize}}
        });

      })();
    </script>{{end}}{{end}}

{{define "credential-rows"}}{{if eq .Mode "vc"}}        <tr>
          <td font-size: 12px;>Kode Voucher</td>
        </tr>
        <tr>
          <td style="width:100%; border: 1px solid black; font-weight:bold; font-size:16px;">{{.User.Name}}</td>
        </tr>
{{else if .StackUP}}        <tr>
          <td>Username</td>
        </tr>
        <tr>
          <td style="border: 1px solid black; font-weight:bold;">{{.User.Name}}</td>
        </tr>
        <tr>
          <td>Password</td>
        </tr>
        <tr>
          <td style="border: 1px solid black; font-weight:bold;">{{.User.Password}}</td>
        </tr>
{{else}}        <tr>
          <td style="width: 50%">Username</td>
          <td >Password</td>
        </tr>
        <tr style="font-size: 14px;">
          <td style="border: 1px solid black; font-weight:bold;">{{.User.Name}}</td>
          <td style="border: 1px solid black; font-weight:bold;">{{.User.Password}}</td>
        </tr>
{{end}}{{end}}

{{define "credential-rows-small"}}{{if eq .Mode "vc"}}        <tr>
          <td >Kode Voucher</td>
        </tr>
        <tr style="color: black; font-size: 14px;">
          <td style="width:100%; border: 1px solid black; font-weight:bold;">{{.User.Name}}</td>
        </tr>
{{else}}          <tr>
          <td style="width: 50%">Username</td>
          <td>Password</td>
        </tr>
        <tr style="color: black; font-size: 14px;">
          <td style="border: 1px solid black; font-weight:bold;">{{.User.Name}}</td>
          <td style="border: 1px solid black; font-weight:bold;">{{.User.Password}}</td>
        </tr>
{{end}}{{end}}

{{define "classic"}}{{if eq (len .Items) 0}}<div class="empty">Tidak ada voucher untuk dicetak.</div>{{end}}{{range .Items}}<table class="voucher" style=" width: 220px;">
  <tbody>
    <tr>
      <td style="text-align: left; font-size: 14px; font-weight:bold; border-bottom: 1px black solid;">{{template "logo" logoArgs $.Ctx 30}}  {{$.Ctx.BrandName}}  <span id="num"> [{{.Num}}]</span></td>
    </tr>
    <tr>
      <td>
    <table style=" text-align: center; width: 210px; font-size: 12px;">
  <tbody>
    <tr>
      <td>
        <table style="width:100%;">
{{template "credential-rows" .}}        </table>
      </td>
{{if $.WithQR}}      <td>
{{template "qr" dict "WithQR" true "Item" .}}
      </td>{{end}}    <tr>
      <td colspan="2" style="border-top: 1px solid black;font-weight:bold; font-size:16px">{{.InfoLine}}</td>
    </tr>
    <tr>
      <td colspan="2" style="font-weight:bold; font-size:12px">Login: http://{{$.Ctx.DNSName}}</td>
    </tr>
  </tbody>
    </table>
      </td>
    </tr>
  </tbody>
</table>{{end}}{{end}}

{{define "thermal"}}{{if eq (len .Items) 0}}<div class="empty">Tidak ada voucher untuk dicetak.</div>{{end}}{{range .Items}}<table class="voucher" style=" width: 180px;">
  <tbody>
    <tr>
      <td style="text-align: center; font-size: 14px; font-weight:bold;">{{$.Ctx.BrandName}}</td>
    </tr>
    <tr>
      <td style="text-align: center; font-size: 14px; font-weight:bold; border-bottom: 1px black solid;">{{template "logo" logoArgs $.Ctx 30}}</td>
    </tr>
    <tr>
      <td>
    <table style=" text-align: center; width: 170px; font-size: 12px;">
  <tbody>
    <tr>
      <td>
        <table style="width:100%;">
{{template "credential-rows" .}}        </table>
      </td>
    </tr>
{{if $.WithQR}}    <tr>
      <td colspan="2">
{{template "qr" dict "WithQR" true "Item" .}}
      </td>
      </tr>
{{end}}    <tr>
      <td colspan="2" style="border-top: 1px solid black;font-weight:bold; font-size:16px">{{.InfoLine}}</td>
    </tr>
    <tr>
      <td colspan="2" style="font-weight:bold; font-size:12px">Login: http://{{$.Ctx.DNSName}}</td>
    </tr>
  </tbody>
    </table>
      </td>
    </tr>
  </tbody>
</table>{{end}}{{end}}

{{define "grid"}}<div class="sheet">{{if eq (len .Items) 0}}<div class="empty">Tidak ada voucher untuk dicetak.</div>{{end}}{{range .Items}}<div class="card"><div class="left"><div class="brand">{{template "round-logo" logoArgs $.Ctx 24}} <span>{{$.Ctx.BrandName}}</span></div><div class="code">{{.User.Name}}</div>{{if eq .Mode "up"}}<div class="pass">PW: {{.User.Password}}</div>{{end}}<div class="meta"><div><div class="lbl">Profil</div><div class="val">{{.User.Profile}}</div></div><div><div class="lbl">Paket</div><div class="val">{{.InfoLine}}</div></div><div><div class="lbl">No</div><div class="val">{{.Num}}</div></div></div></div><div class="right">{{if $.WithQR}}{{template "qr" dict "WithQR" true "Item" .}}{{else}}<div><div class="small">Login</div><div class="small">http://{{$.Ctx.DNSName}}</div></div>{{end}}</div></div>{{end}}</div>{{end}}

{{define "compact"}}{{if eq (len .Items) 0}}<div class="empty">Tidak ada voucher untuk dicetak.</div>{{end}}{{range .Items}}<table class="voucher" style=" width: 160px;">
  <tbody>
    <tr>
      <td style="text-align: left; font-size: 14px; font-weight:bold; border-bottom: 1px black solid;">{{$.Ctx.BrandName}}<span id="num"> [{{.Num}}]</span></td>
    </tr>
    <tr>
      <td>
    <table style=" text-align: center; width: 150px;">
  <tbody>
    <tr style="color: black; font-size: 11px;">
      <td>
        <table style="width:100%;">
{{template "credential-rows-small" .}}          <tr>
          <td colspan="2" style="border: 1px solid black; font-weight:bold;">{{.InfoLine}}</td>
        </tr>
        </table>
      </td>
    </tr>
  </tbody>
    </table>
      </td>
    </tr>
  </tbody>
</table>{{end}}{{end}}
`

var voucherRootTmpl = template.Must(
	template.New("voucher").Funcs(voucherFuncMap).Funcs(template.FuncMap{
		"dict": func(values ...any) (map[string]any, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("dict requires even args")
			}
			m := make(map[string]any, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				m[key] = values[i+1]
			}
			return m, nil
		},
	}).Parse(voucherTemplatesHTML),
)

func voucherInfoLine(u routeros.HotspotUser, meta voucherProfileMeta) string {
	parts := make([]string, 0, 4)
	if meta.Validity != "" {
		parts = append(parts, meta.Validity)
	}
	if u.LimitUptime != "" && u.LimitUptime != "0s" {
		parts = append(parts, u.LimitUptime)
	}
	if u.LimitBytesTotal != "" && u.LimitBytesTotal != "0" {
		parts = append(parts, formatBytesGo(u.LimitBytesTotal))
	}
	if meta.Price != "" && meta.Price != "0" {
		parts = append(parts, "Rp "+formatPriceGo(meta.Price))
	}
	if len(parts) == 0 {
		return u.Profile
	}
	return strings.Join(parts, " ")
}

func voucherMode(u routeros.HotspotUser) string {
	if u.Password == "" || u.Name == u.Password {
		return "vc"
	}
	return "up"
}

func qrID(u routeros.HotspotUser, fallback int) string {
	id := strings.TrimPrefix(u.ID, "*")
	if id == "" {
		id = fmt.Sprintf("%d", fallback)
	}
	return "qr" + sanitizeHTMLID(id)
}

func sanitizeHTMLID(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "x"
	}
	return b.String()
}

func formatBytesGo(s string) string {
	n := int64(0)
	fmt.Sscanf(s, "%d", &n)
	if n <= 0 {
		return ""
	}
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1048576 {
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	}
	if n < 1073741824 {
		return fmt.Sprintf("%.2f MB", float64(n)/1048576)
	}
	return fmt.Sprintf("%.2f GB", float64(n)/1073741824)
}

func formatPriceGo(s string) string {
	n := int64(0)
	fmt.Sscanf(s, "%d", &n)
	if n <= 0 {
		return s
	}
	str := fmt.Sprintf("%d", n)
	result := ""
	for i, c := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result += "."
		}
		result += string(c)
	}
	return result
}

func loginURLForVoucher(dnsName, username, password string) string {
	if dnsName == "" {
		dnsName = "hotspot.local"
	}
	return "http://" + dnsName + "/login?username=" + url.QueryEscape(username) + "&password=" + url.QueryEscape(password)
}
