package httpapi

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mikvoc/internal/core"
	"mikvoc/internal/database"
	"mikvoc/internal/middleware"
)

const (
	BackupFormatVersion = 2
	MaxExpandedSize     = 500 << 20
)

type BackupManifest struct {
	FormatVersion       int           `json:"format_version"`
	CreatedAt           string        `json:"created_at"`
	DBFile              string        `json:"db_file"`
	DBSHA256            string        `json:"db_sha256"`
	DBSize              int64         `json:"db_size"`
	DeclaredTotalSize   int64         `json:"declared_total_size"`
	Assets              []ManifestAsset `json:"assets"`
}

type ManifestAsset struct {
	Path string `json:"path"`
	SHA256 string `json:"sha256"`
	Size int64 `json:"size"`
}

func (a *App) BackupArchiveTo(w io.Writer) error {
	if a.DBPath == "" {
		return fmt.Errorf("DB path not configured")
	}
	if database.DB != nil {
		_, err := database.DB.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
		if err != nil {
			return fmt.Errorf("checkpoint failed: %w", err)
		}
	}
	dbContent, err := os.ReadFile(a.DBPath)
	if err != nil {
		return fmt.Errorf("read DB: %w", err)
	}
	dbHash := fmt.Sprintf("%x", sha256.Sum256(dbContent))
	dbSize := int64(len(dbContent))
	var assets []ManifestAsset
	var totalSize int64 = dbSize
	assetsRoot := "assets"
	err = filepath.WalkDir(assetsRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink rejected: %s", path)
		}
		if d.IsDir() {
			rel, _ := filepath.Rel(assetsRoot, path)
			parts := strings.Split(filepath.ToSlash(rel), "/")
			if len(parts) == 1 {
				if parts[0] != "global" && parts[0] != "routers" {
					return fmt.Errorf("unexpected asset directory: %s", path)
				}
				return nil
			} else if len(parts) == 2 && parts[0] == "routers" {
				return nil
			} else if len(parts) > 2 && parts[0] == "routers" {
				if !d.Type().IsRegular() {
					return fmt.Errorf("non-regular file rejected: %s", path)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, _ = filepath.Rel(assetsRoot, path)
				assets = append(assets, ManifestAsset{Path: rel, SHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Size: int64(len(data))})
				totalSize += int64(len(data))
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("non-regular file rejected: %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(assetsRoot, path)
		assets = append(assets, ManifestAsset{Path: rel, SHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Size: int64(len(data))})
		totalSize += int64(len(data))
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk assets: %w", err)
	}
	zw := zip.NewWriter(w)
	hdr := &zip.FileHeader{Name: "mikvoc.db", Method: zip.Deflate}
	wr, err := zw.CreateHeader(hdr)
	if err != nil {
		return fmt.Errorf("create DB entry: %w", err)
	}
	if _, err := wr.Write(dbContent); err != nil {
		zw.Close()
		return fmt.Errorf("write DB: %w", err)
	}
	for _, asset := range assets {
		data, err := os.ReadFile(filepath.Join(assetsRoot, asset.Path))
		if err != nil {
			zw.Close()
			return fmt.Errorf("read asset %s: %w", asset.Path, err)
		}
		hdr := &zip.FileHeader{Name: asset.Path, Method: zip.Deflate}
		wr, err := zw.CreateHeader(hdr)
		if err != nil {
			zw.Close()
			return fmt.Errorf("create asset entry %s: %w", asset.Path, err)
		}
		if _, err := wr.Write(data); err != nil {
			zw.Close()
			return fmt.Errorf("write asset %s: %w", asset.Path, err)
		}
	}
	manifest := BackupManifest{FormatVersion: BackupFormatVersion, CreatedAt: time.Now().Format(time.RFC3339), DBFile: "mikvoc.db", DBSHA256: dbHash, DBSize: dbSize, DeclaredTotalSize: totalSize, Assets: assets}
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		zw.Close()
		return fmt.Errorf("marshal manifest: %w", err)
	}
	manifestHdr := &zip.FileHeader{Name: "manifest.json", Method: zip.Store}
	manifestWr, err := zw.CreateHeader(manifestHdr)
	if err != nil {
		zw.Close()
		return fmt.Errorf("create manifest entry: %w", err)
	}
	if _, err := manifestWr.Write(manifestJSON); err != nil {
		zw.Close()
		return fmt.Errorf("write manifest: %w", err)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("close ZIP: %w", err)
	}
	return nil
}

func (a *App) HandleBackupNew(w http.ResponseWriter, r *http.Request) {
	if middleware.RoleLevel(middleware.RoleFromRequest(r)) < middleware.RoleLevel(core.RoleOwner) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+fmt.Sprintf("mikvoc-backup-%s.zip", time.Now().Format("20060102-150405"))+`"`)
	if err := a.BackupArchiveTo(w); err != nil {
		log.Printf("[backup] archive error: %v", err)
		http.Error(w, "Backup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	a.audit(r, "backup_new", "versioned-zip")
}

func (a *App) RestoreFromZip(r io.Reader, restoreDir, targetDir string) error {
	buf, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read ZIP: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		return fmt.Errorf("open ZIP: %w", err)
	}
	var manifestFile *zip.File
	var dbFile *zip.File
	entryCount := 0
	for _, f := range zr.File {
		entryCount++
		name := filepath.Clean(f.Name)
		if strings.Contains(name, "..") || strings.HasPrefix(name, "/") {
			return fmt.Errorf("path traversal rejected: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			continue
		}
		if f.Name == "manifest.json" {
			manifestFile = f
		} else if f.Name == "mikvoc.db" {
			dbFile = f
		}
	}
	if manifestFile == nil {
		return fmt.Errorf("manifest.json not found")
	}
	if dbFile == nil {
		return fmt.Errorf("mikvoc.db not found")
	}
	if entryCount < 2 {
		return fmt.Errorf("invalid entry count: %d (expected at least 2)", entryCount)
	}
	rc, err := manifestFile.Open()
	if err != nil {
		return fmt.Errorf("open manifest: %w", err)
	}
	manifestBytes, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	var manifest BackupManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}
	if manifest.FormatVersion != BackupFormatVersion {
		return fmt.Errorf("unsupported format version %d (expected %d)", manifest.FormatVersion, BackupFormatVersion)
	}
	expandedSize := int64(0)
	for _, f := range zr.File {
		expandedSize += int64(f.UncompressedSize64)
	}
	if expandedSize > MaxExpandedSize {
		return fmt.Errorf("expanded size %d exceeds limit %d", expandedSize, MaxExpandedSize)
	}
	if manifest.DeclaredTotalSize > 0 && expandedSize > manifest.DeclaredTotalSize*2 {
		return fmt.Errorf("size mismatch: declared=%d, actual=%d", manifest.DeclaredTotalSize, expandedSize)
	}
	seenPaths := make(map[string]bool)
	for _, f := range zr.File {
		normalized := strings.ToLower(filepath.Clean(f.Name))
		if seenPaths[normalized] {
			return fmt.Errorf("duplicate entry detected: %s", f.Name)
		}
		seenPaths[normalized] = true
		if f.Name != "manifest.json" && f.Name != "mikvoc.db" {
			if !isValidManagedPath(f.Name) {
				return fmt.Errorf("unmanaged asset path rejected: %s", f.Name)
			}
		}
	}
	if err := a.validateSHA256Entries(zr, manifest); err != nil {
		return err
	}
	stagingDir := filepath.Join(targetDir, ".restore-staging", time.Now().Format("20060102-150405"))
	if err := os.MkdirAll(stagingDir, 0700); err != nil {
		return fmt.Errorf("create staging dir: %w", err)
	}
	for _, f := range zr.File {
		cleanName := filepath.Clean(f.Name)
		targetPath := filepath.Join(stagingDir, cleanName)
		if !strings.HasPrefix(targetPath, stagingDir) {
			return fmt.Errorf("extraction escapes staging: %s -> %s", f.Name, targetPath)
		}
		parentDir := filepath.Dir(targetPath)
		if err := os.MkdirAll(parentDir, 0700); err != nil {
			return fmt.Errorf("create dir %s: %w", parentDir, err)
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open %s: %w", f.Name, err)
		}
		outFile, err := os.Create(targetPath)
		if err != nil {
			rc.Close()
			return fmt.Errorf("create %s: %w", targetPath, err)
		}
		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return fmt.Errorf("write %s: %w", targetPath, err)
		}
	}
	return nil
}

func (a *App) RestoreFromZipWithRollback(r io.Reader, restoreDir, targetDir, oldDBPath, oldAssetsPath string) error {
	stagingDir := filepath.Join(targetDir, ".restore-staging-new", time.Now().Format("20060102-150405"))
	if err := os.MkdirAll(stagingDir, 0700); err != nil {
		return fmt.Errorf("create staging: %w", err)
	}
	if err := a.RestoreFromZip(r, stagingDir, targetDir); err != nil {
		os.RemoveAll(stagingDir)
		return fmt.Errorf("staged restore failed: %w", err)
	}
	if a.Pool != nil {
		a.Pool.Clear()
	}
	if database.DB != nil {
		_ = database.DB.Close()
		database.DB = nil
	}
	oldAssetsBase := filepath.Base(oldAssetsPath)
	stagedAssets := filepath.Join(stagingDir, oldAssetsBase)
	stagedDB := filepath.Join(stagingDir, "mikvoc.db")
	timestamp := time.Now().Format("20060102-150405")
	oldDBBackup := oldDBPath + ".bak." + timestamp
	oldAssetsBackup := oldAssetsPath + ".bak." + timestamp
	if _, err := os.Stat(oldDBPath); err == nil {
		if err := os.Rename(oldDBPath, oldDBBackup); err != nil {
			os.RemoveAll(stagingDir)
			return fmt.Errorf("backup old DB: %w", err)
		}
	}
	if _, err := os.Stat(oldAssetsPath); err == nil {
		if err := os.Rename(oldAssetsPath, oldAssetsBackup); err != nil {
			os.Rename(oldDBBackup, oldDBPath)
			os.RemoveAll(stagingDir)
			return fmt.Errorf("backup old assets: %w", err)
		}
	}
	if err := os.Rename(stagedDB, oldDBPath); err != nil {
		rollbackOld(oldDBBackup, oldDBPath, oldAssetsBackup, oldAssetsPath)
		os.RemoveAll(stagingDir)
		return fmt.Errorf("move DB: %w", err)
	}
	if _, err := os.Stat(stagedAssets); err == nil {
		if err := os.Rename(stagedAssets, oldAssetsPath); err != nil {
			rollbackOld(oldDBPath, oldDBBackup, oldAssetsPath, oldAssetsBackup)
			os.RemoveAll(stagingDir)
			return fmt.Errorf("move assets: %w", err)
		}
	}
	if err := database.Init(oldDBPath, a.Secret); err != nil {
		rollbackOld(oldDBPath, oldDBBackup, oldAssetsPath, oldAssetsBackup)
		os.RemoveAll(stagingDir)
		return fmt.Errorf("reinit DB: %w", err)
	}
	os.RemoveAll(stagingDir)
	a.InvalidateSettingsCache()
	a.InvalidateRoutersCache()
	a.InvalidateTemplateCache()
	if a.Routers != nil {
		go func() {
			_ = a.Routers.ConnectAll()
		}()
	}
	return nil
}

func rollbackOld(newDB, oldDBBak, newAssets, oldAssetsBak string) {
	if _, err := os.Stat(oldDBBak); err == nil {
		os.Rename(oldDBBak, newDB)
	}
	if _, err := os.Stat(oldAssetsBak); err == nil {
		os.Rename(oldAssetsBak, newAssets)
	}
}

func (a *App) RestoreLegacyDB(r io.Reader) error {
	content, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read legacy DB: %w", err)
	}
	if len(content) < 16 || string(content[:16]) != "SQLite format 3\x00" {
		return fmt.Errorf("not a valid SQLite database")
	}
	tmpDB := a.DBPath + ".incoming"
	if err := os.WriteFile(tmpDB, content, 0600); err != nil {
		return fmt.Errorf("write temp DB: %w", err)
	}
	if a.Pool != nil {
		a.Pool.Clear()
	}
	if database.DB != nil {
		_ = database.DB.Close()
		database.DB = nil
	}
	timestamp := time.Now().Format("20060102-150405")
	oldBackup := a.DBPath + ".bak." + timestamp
	if err := os.Rename(a.DBPath, oldBackup); err != nil && !os.IsNotExist(err) {
		os.Remove(tmpDB)
		return fmt.Errorf("backup old DB: %w", err)
	}
	if err := os.Rename(tmpDB, a.DBPath); err != nil {
		os.Rename(oldBackup, a.DBPath)
		return fmt.Errorf("install new DB: %w", err)
	}
	if err := database.Init(a.DBPath, a.Secret); err != nil {
		os.Remove(a.DBPath)
		os.Rename(oldBackup, a.DBPath)
		return fmt.Errorf("reinit DB: %w", err)
	}
	a.InvalidateSettingsCache()
	a.InvalidateRoutersCache()
	if a.Routers != nil {
		go func() {
			_ = a.Routers.ConnectAll()
		}()
	}
	return nil
}

func isValidManagedPath(path string) bool {
	clean := filepath.Clean(path)
	parts := strings.Split(filepath.ToSlash(clean), "/")
	if len(parts) == 0 || parts[0] == "" {
		return false
	}
	switch parts[0] {
	case "global":
		if len(parts) < 2 {
			return false
		}
		return true
	case "routers":
		if len(parts) < 3 {
			return false
		}
		id := parts[1]
		valid := true
		for _, c := range id {
			if c < '0' || c > '9' {
				valid = false
				break
			}
		}
		return valid && id != "0" && !strings.HasPrefix(id, "0")
	default:
		return false
	}
}

func (a *App) validateSHA256Entries(zr *zip.Reader, manifest BackupManifest) error {
	expectedHashes := make(map[string]string)
	expectedHashes[manifest.DBFile] = manifest.DBSHA256
	for _, asset := range manifest.Assets {
		expectedHashes[asset.Path] = asset.SHA256
	}
	for _, f := range zr.File {
		expected, ok := expectedHashes[f.Name]
		if !ok {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open %s for hash check: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return fmt.Errorf("read %s for hash check: %w", f.Name, err)
		}
		actualHash := fmt.Sprintf("%x", sha256.Sum256(data))
		if actualHash != expected {
			return fmt.Errorf("hash mismatch for %s: expected %s, got %s", f.Name, expected, actualHash)
		}
	}
	return nil
}
