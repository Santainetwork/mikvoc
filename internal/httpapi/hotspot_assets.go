package httpapi

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"unicode/utf8"
)

const (
	maxAssetCompressedBytes   = 2 << 20
	maxAssetUncompressedBytes = 8 << 20
	maxAssetEntries           = 100
)

var assetExtensions = map[string]bool{
	".html": true, ".htm": true, ".css": true, ".js": true,
	".json": true, ".txt": true, ".svg": true, ".png": true,
	".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
	".ico": true, ".woff": true, ".woff2": true, ".ttf": true,
	".otf": true,
}

func normalizeAssetName(raw string) (string, error) {
	if !utf8.ValidString(raw) || strings.IndexFunc(raw, func(r rune) bool { return r <= 0x1f || r == 0x7f }) >= 0 {
		return "", fmt.Errorf("invalid asset path %q", raw)
	}
	name := strings.ReplaceAll(raw, `\`, "/")
	if name == "" || path.IsAbs(name) || hasWindowsDrive(name) {
		return "", fmt.Errorf("invalid asset path %q", raw)
	}
	for _, segment := range strings.Split(name, "/") {
		if segment == ".." {
			return "", fmt.Errorf("invalid asset path %q", raw)
		}
	}
	name = path.Clean(name)
	if name == "." || name == "" || !assetExtensions[strings.ToLower(path.Ext(name))] {
		return "", fmt.Errorf("unsupported asset path %q", raw)
	}
	return name, nil
}

func hasWindowsDrive(name string) bool {
	return len(name) >= 2 && ((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z')) && name[1] == ':'
}

func sharedTopDir(names []string) string {
	if len(names) == 0 {
		return ""
	}
	first, _, ok := strings.Cut(names[0], "/")
	if !ok || first == "" {
		return ""
	}
	for _, name := range names[1:] {
		top, _, nested := strings.Cut(name, "/")
		if !nested || top != first {
			return ""
		}
	}
	return first
}

func validateAssetZip(raw []byte) (templateFileSet, error) {
	if len(raw) == 0 || len(raw) > maxAssetCompressedBytes {
		return templateFileSet{}, fmt.Errorf("ZIP size must be between 1 and %d bytes", maxAssetCompressedBytes)
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return templateFileSet{}, fmt.Errorf("invalid ZIP: %w", err)
	}
	if len(zr.File) > maxAssetEntries {
		return templateFileSet{}, fmt.Errorf("ZIP has more than %d entries", maxAssetEntries)
	}

	type archiveFile struct {
		name    string
		content []byte
	}
	files := make([]archiveFile, 0, len(zr.File))
	names := make([]string, 0, len(zr.File))
	total := int64(0)
	for _, file := range zr.File {
		mode := file.Mode()
		if mode&os.ModeType == os.ModeDir {
			continue
		}
		if !mode.IsRegular() {
			return templateFileSet{}, fmt.Errorf("non-regular file %q is not allowed", file.Name)
		}
		name, err := normalizeAssetName(file.Name)
		if err != nil {
			return templateFileSet{}, err
		}
		r, err := file.Open()
		if err != nil {
			return templateFileSet{}, fmt.Errorf("open %q: %w", file.Name, err)
		}
		content, readErr := io.ReadAll(io.LimitReader(r, int64(maxAssetUncompressedBytes)-total+1))
		closeErr := r.Close()
		if readErr != nil {
			return templateFileSet{}, fmt.Errorf("read %q: %w", file.Name, readErr)
		}
		if closeErr != nil {
			return templateFileSet{}, fmt.Errorf("close %q: %w", file.Name, closeErr)
		}
		total += int64(len(content))
		if total > maxAssetUncompressedBytes {
			return templateFileSet{}, fmt.Errorf("ZIP expands beyond %d bytes", maxAssetUncompressedBytes)
		}
		files = append(files, archiveFile{name: name, content: content})
		names = append(names, name)
	}

	if top := sharedTopDir(names); top != "" {
		prefix := top + "/"
		for i := range files {
			files[i].name = strings.TrimPrefix(files[i].name, prefix)
		}
	}
	seen := make(map[string]bool, len(files))
	result := templateFileSet{Files: make([]templateFile, 0, len(files))}
	for _, file := range files {
		key := strings.ToLower(file.name)
		if key == "hotspot" || strings.HasPrefix(key, "hotspot/") {
			return templateFileSet{}, fmt.Errorf("asset path %q must not include the hotspot directory", file.name)
		}
		if seen[key] {
			return templateFileSet{}, fmt.Errorf("duplicate asset path %q", file.name)
		}
		seen[key] = true
		result.Files = append(result.Files, templateFile{Name: file.name, Content: file.content, Asset: !isStandardHotspotFile(file.name)})
	}
	if !seen["login.html"] {
		return templateFileSet{}, fmt.Errorf("ZIP must contain login.html")
	}
	return result, nil
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
