package httpapi

import (
	"encoding/base64"
	"fmt"
	"html/template"
	"strings"
	"unicode"
)

func validateCustomLogin(html string) error {
	if strings.TrimSpace(html) == "" {
		return fmt.Errorf("login.html custom masih kosong; isi editor atau unggah ZIP yang memuat login.html")
	}
	if hasCustomLoginContract(html) {
		return nil
	}
	return fmt.Errorf(`login.html custom wajib memuat form action="$(link-login-only)" dengan input name="username" di dalamnya`)
}

type scannedHTMLTag struct {
	name        string
	attrs       map[string]string
	end         bool
	selfClosing bool
}

func hasCustomLoginContract(html string) bool {
	var forms []bool
	raw := ""
	for pos := 0; pos < len(html); {
		rel := strings.IndexByte(html[pos:], '<')
		if rel < 0 {
			break
		}
		start := pos + rel
		tag, next, ok := scanHTMLTag(html, start)
		if !ok {
			pos = next
			continue
		}
		pos = next
		if raw != "" {
			if tag.end && tag.name == raw {
				raw = ""
			}
			continue
		}
		if !tag.end && (tag.name == "script" || tag.name == "style" || tag.name == "textarea") {
			if !tag.selfClosing {
				raw = tag.name
			}
			continue
		}
		if tag.name == "form" {
			if tag.end {
				if len(forms) > 0 {
					forms = forms[:len(forms)-1]
				}
				continue
			}
			if len(forms) > 0 {
				return false
			}
			forms = append(forms, strings.TrimSpace(tag.attrs["action"]) == "$(link-login-only)")
			if tag.selfClosing {
				forms = forms[:len(forms)-1]
			}
			continue
		}
		if tag.end || tag.name != "input" || len(forms) == 0 || !forms[len(forms)-1] {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(tag.attrs["name"]), "username") {
			return true
		}
	}
	return false
}

func scanHTMLTag(html string, start int) (scannedHTMLTag, int, bool) {
	if strings.HasPrefix(html[start:], "<!--") {
		if end := strings.Index(html[start+4:], "-->"); end >= 0 {
			return scannedHTMLTag{}, start + 4 + end + 3, false
		}
		return scannedHTMLTag{}, len(html), false
	}
	quote := byte(0)
	end := start + 1
	for ; end < len(html); end++ {
		char := html[end]
		if quote != 0 {
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
		} else if char == '>' {
			break
		}
	}
	if end == len(html) {
		return scannedHTMLTag{}, len(html), false
	}
	body := strings.TrimSpace(html[start+1 : end])
	if body == "" || body[0] == '!' || body[0] == '?' {
		return scannedHTMLTag{}, end + 1, false
	}
	tag := scannedHTMLTag{attrs: map[string]string{}}
	if body[0] == '/' {
		tag.end = true
		body = strings.TrimSpace(body[1:])
	}
	nameEnd := 0
	for nameEnd < len(body) && isHTMLNameByte(body[nameEnd]) {
		nameEnd++
	}
	if nameEnd == 0 {
		return scannedHTMLTag{}, end + 1, false
	}
	tag.name = strings.ToLower(body[:nameEnd])
	if tag.end {
		return tag, end + 1, true
	}
	tag.attrs, tag.selfClosing = parseHTMLAttributes(body[nameEnd:])
	return tag, end + 1, true
}

func parseHTMLAttributes(text string) (map[string]string, bool) {
	attrs := map[string]string{}
	for pos := 0; pos < len(text); {
		for pos < len(text) && unicode.IsSpace(rune(text[pos])) {
			pos++
		}
		if pos == len(text) {
			return attrs, false
		}
		if text[pos] == '/' && strings.TrimSpace(text[pos+1:]) == "" {
			return attrs, true
		}
		start := pos
		for pos < len(text) && isHTMLNameByte(text[pos]) {
			pos++
		}
		if start == pos {
			pos++
			continue
		}
		name := strings.ToLower(text[start:pos])
		for pos < len(text) && unicode.IsSpace(rune(text[pos])) {
			pos++
		}
		value := ""
		if pos < len(text) && text[pos] == '=' {
			pos++
			for pos < len(text) && unicode.IsSpace(rune(text[pos])) {
				pos++
			}
			if pos < len(text) && (text[pos] == '\'' || text[pos] == '"') {
				quote := text[pos]
				pos++
				start = pos
				for pos < len(text) && text[pos] != quote {
					pos++
				}
				value = text[start:pos]
				if pos < len(text) {
					pos++
				}
			} else {
				start = pos
				for pos < len(text) && !unicode.IsSpace(rune(text[pos])) {
					pos++
				}
				value = text[start:pos]
			}
		}
		if _, exists := attrs[name]; !exists {
			attrs[name] = value
		}
	}
	return attrs, false
}

func isHTMLNameByte(char byte) bool {
	return char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == ':' || char == '_'
}

func storedAssetPackage(settings map[string]string) (templateFileSet, error) {
	encoded := settings["tpl_custom_assets_zip"]
	if strings.TrimSpace(encoded) == "" {
		return templateFileSet{}, nil
	}
	if strings.IndexFunc(encoded, unicode.IsSpace) >= 0 {
		return templateFileSet{}, fmt.Errorf("paket aset tersimpan memuat spasi atau baris baru; unggah ulang ZIP")
	}
	if len(encoded) > base64.StdEncoding.EncodedLen(maxAssetCompressedBytes) {
		return templateFileSet{}, fmt.Errorf("paket aset tersimpan terlalu besar; batas ZIP %d byte", maxAssetCompressedBytes)
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return templateFileSet{}, fmt.Errorf("paket aset tersimpan rusak; unggah ulang ZIP: %w", err)
	}
	set, err := validateAssetZip(raw)
	if err != nil {
		return templateFileSet{}, fmt.Errorf("paket aset tersimpan bukan ZIP valid; unggah ulang: %w", err)
	}
	return set, nil
}

func assembleTemplateFiles(settings map[string]string) (templateFileSet, error) {
	return assembleTemplateFilesWithPackage(settings, nil)
}

func assembleTemplateFilesWithPackage(settings map[string]string, packageOverride *templateFileSet) (templateFileSet, error) {
	variant := normalizeHotspotVariant(settings["tpl_variant"])
	view := hotspotViewFrom(settings)
	if variant != "custom" {
		return templateFileSet{Files: []templateFile{
			{Name: "login.html", Content: []byte(renderVariantLogin(variant, settings))},
			{Name: "status.html", Content: []byte(renderHotspotStatus(view))},
			{Name: "logout.html", Content: []byte(renderHotspotLogout(view))},
		}}, nil
	}

	var pkg templateFileSet
	if packageOverride != nil {
		pkg = *packageOverride
	} else {
		var err error
		pkg, err = storedAssetPackage(settings)
		if err != nil {
			return templateFileSet{}, err
		}
	}
	resolve := func(setting, name string) string {
		if editor := settings[setting]; strings.TrimSpace(editor) != "" {
			return editor
		}
		return string(getTemplateFileFold(pkg, name))
	}
	login := resolve("tpl_custom_login_html", "login.html")
	if err := validateCustomLogin(login); err != nil {
		return templateFileSet{}, err
	}
	status := resolve("tpl_custom_status_html", "status.html")
	if strings.TrimSpace(status) == "" {
		status = renderHotspotStatus(view)
	}
	logout := resolve("tpl_custom_logout_html", "logout.html")
	if strings.TrimSpace(logout) == "" {
		logout = renderHotspotLogout(view)
	}

	out := templateFileSet{Files: []templateFile{
		{Name: "login.html", Content: []byte(login)},
		{Name: "status.html", Content: []byte(status)},
		{Name: "logout.html", Content: []byte(logout)},
	}}
	for _, file := range pkg.Files {
		if isStandardHotspotFile(file.Name) {
			continue
		}
		out.Files = append(out.Files, file)
	}
	return out, nil
}

func pushableHotspotHTML(settings map[string]string) (string, string, string, error) {
	set, err := assembleTemplateFiles(settings)
	if err != nil {
		return "", "", "", err
	}
	if err := set.PushCheck(); err != nil {
		return "", "", "", err
	}
	return string(set.Get("login.html")), string(set.Get("status.html")), string(set.Get("logout.html")), nil
}

func getTemplateFileFold(set templateFileSet, name string) []byte {
	for _, file := range set.Files {
		if strings.EqualFold(file.Name, name) {
			return file.Content
		}
	}
	return nil
}

func isStandardHotspotFile(name string) bool {
	return strings.EqualFold(name, "login.html") || strings.EqualFold(name, "status.html") || strings.EqualFold(name, "logout.html")
}

func invalidCustomLoginDocument(err error) string {
	return fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8"><title>Template custom tidak valid</title></head><body><main><h1>Template custom tidak valid</h1><p>%s</p></main></body></html>`, template.HTMLEscapeString(err.Error()))
}
