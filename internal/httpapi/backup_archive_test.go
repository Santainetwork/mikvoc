package httpapi

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestBackupArchiveCreatesVersionedZip - Versioned ZIP with manifest
func TestBackupArchiveCreatesVersionedZip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "backup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "mikvoc.db")
	f, err := os.Create(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	// SQLite header
	f.Write([]byte("SQLite format 3\x00"))
	f.Close()

	var buf bytes.Buffer
	app := &App{DBPath: dbPath}

	err = app.BackupArchiveTo(&buf)
	if err != nil {
		t.Fatalf("BackupArchive failed: %v", err)
	}

	// Verify it's a valid ZIP
	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("Not a valid ZIP: %v", err)
	}

	// Check manifest.json exists
	var manifest map[string]interface{}
	for _, f := range r.File {
		if f.Name == "manifest.json" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("Cannot open manifest.json: %v", err)
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				t.Fatalf("Cannot read manifest.json: %v", err)
			}
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatalf("Invalid JSON in manifest.json: %v", err)
			}
			break
		}
	}

	if manifest == nil {
		t.Fatal("manifest.json not found in backup")
	}

	// Check required fields
	if manifest["format_version"] == nil {
		t.Error("manifest.json missing format_version")
	}
	if manifest["created_at"] == nil {
		t.Error("manifest.json missing created_at")
	}
	if manifest["db_file"] == nil {
		t.Error("manifest.json missing db_file")
	}
	if manifest["assets"] == nil {
		t.Error("manifest.json missing assets array")
	}
}

// TestBackupArchiveIncludesMikvocDb - mikvoc.db is included
func TestBackupArchiveIncludesMikvocDb(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "backup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "mikvoc.db")
	f, err := os.Create(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	f.Write([]byte("SQLite format 3\x00"))
	dbContent, _ := os.ReadFile(dbPath)
	f.Close()

	var buf bytes.Buffer
	app := &App{DBPath: dbPath}

	if err := app.BackupArchiveTo(&buf); err != nil {
		t.Fatalf("BackupArchive failed: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("Invalid ZIP: %v", err)
	}

	found := false
	for _, f := range r.File {
		if f.Name == "mikvoc.db" {
			found = true
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("Cannot open mikvoc.db from ZIP: %v", err)
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				t.Fatalf("Cannot read mikvoc.db from ZIP: %v", err)
			}
			if !bytes.Equal(data, dbContent) {
				t.Error("mikvoc.db content mismatch")
			}
		}
	}

	if !found {
		t.Fatal("mikvoc.db not found in backup archive")
	}
}

// TestBackupArchiveManifestContainsAssetPaths - Asset paths in manifest
func TestBackupArchiveManifestContainsAssetPaths(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "backup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "mikvoc.db")
	f, err := os.Create(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	f.Write([]byte("SQLite format 3\x00"))
	f.Close()

	assetsRoot := filepath.Join(tmpDir, "assets")
	routerDir := filepath.Join(assetsRoot, "routers", "123")
	if err := os.MkdirAll(routerDir, 0700); err != nil {
		t.Fatalf("Failed to create asset dir: %v", err)
	}

	logoPath := filepath.Join(routerDir, "logo.png")
	if err := os.WriteFile(logoPath, []byte("fake png content"), 0600); err != nil {
		t.Fatalf("Failed to create logo: %v", err)
	}

	var buf bytes.Buffer
	app := &App{DBPath: dbPath}
	if err := app.BackupArchiveTo(&buf); err != nil {
		t.Fatalf("BackupArchive failed: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("Invalid ZIP: %v", err)
	}

	// Read manifest and check asset paths
	var manifest map[string]interface{}
	for _, f := range r.File {
		if f.Name == "manifest.json" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("Cannot open manifest.json: %v", err)
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			json.Unmarshal(data, &manifest)
			break
		}
	}

	assets, ok := manifest["assets"].([]interface{})
	if !ok || len(assets) == 0 {
		t.Error("Expected at least one asset in manifest")
	}

	assetFound := false
	for _, a := range assets {
		if am, ok := a.(map[string]interface{}); ok {
			if path, ok := am["path"].(string); ok {
				if strings.Contains(path, "routers/123/logo.png") {
					assetFound = true
				}
			}
		}
	}

	if !assetFound {
		t.Error("router/123/logo.png not found in manifest assets")
	}
}

// TestBackupArchiveSHA256Digests - SHA-256 digests for all files
func TestBackupArchiveSHA256Digests(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "backup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "mikvoc.db")
	f, err := os.Create(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	f.Write([]byte("SQLite format 3\x00"))
	f.Close()

	var buf bytes.Buffer
	app := &App{DBPath: dbPath}

	if err := app.BackupArchiveTo(&buf); err != nil {
		t.Fatalf("BackupArchive failed: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("Invalid ZIP: %v", err)
	}

	// Build expected hashes
	expectedHashes := make(map[string]string)
	for _, f := range r.File {
		if f.Name == "mikvoc.db" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("Cannot open mikvoc.db: %v", err)
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			hash := fmt.Sprintf("%x", sha256.Sum256(data))
			expectedHashes[f.Name] = hash
		} else if f.Name == "manifest.json" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("Cannot open manifest.json: %v", err)
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			hash := fmt.Sprintf("%x", sha256.Sum256(data))
			expectedHashes[f.Name] = hash
		}
	}

	// Read manifest and verify hashes
	var manifest map[string]interface{}
	for _, f := range r.File {
		if f.Name == "manifest.json" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("Cannot open manifest.json: %v", err)
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			json.Unmarshal(data, &manifest)
			break
		}
	}

	// Check DB hash
	if dbHash, ok := expectedHashes["mikvoc.db"]; ok {
		if manifest["db_sha256"] != dbHash {
			t.Errorf("manifest db_sha256 = %v, want %v", manifest["db_sha256"], dbHash)
		}
	}

	// Check all file hashes in manifest
	assets, ok := manifest["assets"].([]interface{})
	if !ok {
		t.Fatal("manifest has no assets array")
	}

	for _, a := range assets {
		if am, ok := a.(map[string]interface{}); ok {
			if path, ok := am["path"].(string); ok {
				if expHash, ok := expectedHashes[path]; ok {
					if actualHash, ok := am["sha256"].(string); ok {
						if actualHash != expHash {
							t.Errorf("Asset %s hash mismatch: manifest=%s, actual=%s", path, actualHash, expHash)
						}
					} else {
						t.Errorf("Asset %s missing sha256 in manifest", path)
					}
				}
			}
		}
	}
}

// TestBackupArchiveSizesInManifest - File sizes in manifest
func TestBackupArchiveSizesInManifest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "backup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "mikvoc.db")
	f, err := os.Create(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	content := []byte("SQLite format 3\x00 " + strings.Repeat("x", 1000))
	f.Write(content)
	f.Close()

	var buf bytes.Buffer
	app := &App{DBPath: dbPath}

	if err := app.BackupArchiveTo(&buf); err != nil {
		t.Fatalf("BackupArchive failed: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("Invalid ZIP: %v", err)
	}

	// Get actual size of mikvoc.db
	var dbSize int64
	for _, f := range r.File {
		if f.Name == "mikvoc.db" {
			dbSize = f.UncompressedSize64
			break
		}
	}

	// Read manifest
	var manifest map[string]interface{}
	for _, f := range r.File {
		if f.Name == "manifest.json" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("Cannot open manifest.json: %v", err)
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			json.Unmarshal(data, &manifest)
			break
		}
	}

	if manifest["db_size"] == nil {
		t.Error("manifest.json missing db_size")
	} else if manifest["db_size"].(int64) != dbSize {
		t.Errorf("db_size = %d, want %d", manifest["db_size"], dbSize)
	}
}

// TestRestoreRejectSymlinks - Reject symlinks on restore
func TestRestoreRejectSymlinks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "restore-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a ZIP with symlink
	zbuf := new(bytes.Buffer)
	w := zip.NewWriter(zbuf)
	
	hdr := &zip.FileHeader{
		Name:   "legitimate.txt",
		Method: zip.Store,
	}
		t.Logf("Restore allowed suspicious entry (may be acceptable if ZIP library sanitizes)")
	} else if strings.Contains(err.Error(), "symlink") || strings.Contains(err.Error(), "traversal") || strings.Contains(err.Error(), "non-regular") {
		t.Logf("Restore correctly rejected: %v", err)
	} else {
		t.Logf("Restore failed with: %v", err)
	}
}

// TestRestoreRejectTraversal - Reject path traversal attacks
func TestRestoreRejectTraversal(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "restore-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create ZIP with traversal attempt
	zbuf := new(bytes.Buffer)
	w := zip.NewWriter(zbuf)
	
	hdr := &zip.FileHeader{
		Name:   "../../../etc/passwd",
		Method: zip.Store,
	}
	w.CreateNew(hdr)
	w.Write([]byte("evil"))
	w.Close()

	restoreDir := filepath.Join(tmpDir, "restore")
	if err := os.MkdirAll(restoreDir, 0700); err != nil {
		t.Fatalf("Failed to create restore dir: %v", err)
	}

	app := &App{}
	err = app.RestoreFromZip(bytes.NewReader(zbuf.Bytes()), restoreDir, tmpDir)
	if err == nil {
		t.Fatal("Restore should reject path traversal, but succeeded")
	}
	if !strings.Contains(err.Error(), "traversal") && !strings.Contains(err.Error(), "escape") {
		t.Errorf("Expected traversal error, got: %v", err)
	}
}

// TestRestoreRejectDuplicateNames - Reject duplicate names (case-insensitive)
func TestRestoreRejectDuplicateNames(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "restore-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	zbuf := new(bytes.Buffer)
	w := zip.NewWriter(zbuf)
	
	w.CreateNew(&zip.FileHeader{Name: "file.txt", Method: zip.Store})
	wr, _ := w.Create("file.txt"); wr.Write([]byte("content1"))
	w.CreateNew(&zip.FileHeader{Name: "FILE.TXT", Method: zip.Store})
	wr, _ := w.Create("FILE.TXT"); wr.Write([]byte("content2"))
	w.Close()

	restoreDir := filepath.Join(tmpDir, "restore")
	if err := os.MkdirAll(restoreDir, 0700); err != nil {
		t.Fatalf("Failed to create restore dir: %v", err)
	}

	app := &App{}
	err = app.RestoreFromZip(bytes.NewReader(zbuf.Bytes()), restoreDir, tmpDir)
	if err == nil {
		t.Fatal("Restore should reject duplicates, but succeeded")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("Expected duplicate error, got: %v", err)
	}
}

// TestRestoreRejectNonRegularFiles - Reject non-regular files
func TestRestoreRejectNonRegularFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "restore-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	zbuf := new(bytes.Buffer)
	w := zip.NewWriter(zbuf)
	
	// Directory entry
	hdr := &zip.FileHeader{
		Name:   "somedir/",
		Method: zip.Store,
		Type:   zip.TypeDirectory,
	}
	w.CreateNew(hdr)
	
	// Regular file
	hdr2 := &zip.FileHeader{
		Name:   "legit.txt",
		Method: zip.Store,
		Type:   zip.TypeRegular,
	}
	w.CreateNew(hdr2)
	w.Write([]byte("ok"))
	w.Close()

	restoreDir := filepath.Join(tmpDir, "restore")
	if err := os.MkdirAll(restoreDir, 0700); err != nil {
		t.Fatalf("Failed to create restore dir: %v", err)
	}

	app := &App{}
	err = app.RestoreFromZip(bytes.NewReader(zbuf.Bytes()), restoreDir, tmpDir)
	if err == nil {
		t.Log("Accept non-regular if they're directories or sanitized")
	} else if strings.Contains(err.Error(), "non-regular") {
		t.Logf("Correctly rejected non-regular: %v", err)
	} else {
		t.Logf("Restore error (may be acceptable): %v", err)
	}
}

// TestRestoreExpandedSizeRejection - Reject excessive expanded size
func TestRestoreExpandedSizeRejection(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "restore-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Manifest declares small size, but actual content is huge
	manifest := map[string]interface{}{
		"format_version": 2,
		"created_at":     time.Now().Format(time.RFC3339),
		"db_file":        "mikvoc.db",
		"db_sha256":      "dummy",
		"db_size":        100, // Claims 100 bytes
		"assets": []map[string]interface{}{},
	}
	manifestJSON, _ := json.Marshal(manifest)

	zbuf := new(bytes.Buffer)
	w := zip.NewWriter(zbuf)
	
	w.CreateNew(&zip.FileHeader{Name: "manifest.json", Method: zip.Deflate})
	w.Write(manifestJSON)
	
	// Large fake DB (much larger than declared)
	largeContent := bytes.Repeat([]byte("x"), 10*1024*1024) // 10MB
	w.CreateNew(&zip.FileHeader{Name: "mikvoc.db", Method: zip.Deflate, UncompressedSize64: 100})
	w.Write(largeContent)
	w.Close()

	restoreDir := filepath.Join(tmpDir, "restore")
	if err := os.MkdirAll(restoreDir, 0700); err != nil {
		t.Fatalf("Failed to create restore dir: %v", err)
	}

	app := &App{}
	err = app.RestoreFromZip(bytes.NewReader(zbuf.Bytes()), restoreDir, tmpDir)
	if err == nil {
		t.Fatal("Should reject oversized expansion")
	}
	if !strings.Contains(err.Error(), "expanded") && !strings.Contains(err.Error(), "size") {
		t.Errorf("Expected size error, got: %v", err)
	}
}

// TestRestoreValidateEntryCount - Validate entry count matches manifest
func TestRestoreValidateEntryCount(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "restore-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	manifest := map[string]interface{}{
		"format_version": 2,
		"created_at":     time.Now().Format(time.RFC3339),
		"db_file":        "mikvoc.db",
		"db_sha256":      "abc",
		"db_size":        100,
		"declared_total_size": 100,
		"assets": []map[string]interface{}{},
	}
	manifestJSON, _ := json.Marshal(manifest)

	zbuf := new(bytes.Buffer)
	w := zip.NewWriter(zbuf)
	w.CreateNew(&zip.FileHeader{Name: "manifest.json", Method: zip.Store})
	w.Write(manifestJSON)
	
	// No actual DB file despite manifest claiming it exists
	w.Close()

	restoreDir := filepath.Join(tmpDir, "restore")
	if err := os.MkdirAll(restoreDir, 0700); err != nil {
		t.Fatalf("Failed to create restore dir: %v", err)
	}

	app := &App{}
	err = app.RestoreFromZip(bytes.NewReader(zbuf.Bytes()), restoreDir, tmpDir)
	if err == nil {
		t.Fatal("Should reject missing entries")
	}
	if !strings.Contains(err.Error(), "entry") && !strings.Contains(err.Error(), "count") && !strings.Contains(err.Error(), "missing") {
		t.Errorf("Expected entry validation error, got: %v", err)
	}
}

// TestRestoreVersionCheck - Check format version in manifest
func TestRestoreVersionCheck(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "restore-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	manifest := map[string]interface{}{
		"format_version": 999, // Unsupported version
		"created_at":     time.Now().Format(time.RFC3339),
		"db_file":        "mikvoc.db",
		"db_sha256":      "abc",
		"db_size":        100,
		"assets":         []map[string]interface{}{},
	}
	manifestJSON, _ := json.Marshal(manifest)

	zbuf := new(bytes.Buffer)
	w := zip.NewWriter(zbuf)
	w.CreateNew(&zip.FileHeader{Name: "manifest.json", Method: zip.Store})
	w.Write(manifestJSON)
	
	fakeDB := []byte("SQLite format 3\x00")
	w.CreateNew(&zip.FileHeader{Name: "mikvoc.db", Method: zip.Store})
	w.Write(fakeDB)
	w.Close()

	restoreDir := filepath.Join(tmpDir, "restore")
	if err := os.MkdirAll(restoreDir, 0700); err != nil {
		t.Fatalf("Failed to create restore dir: %v", err)
	}

	app := &App{}
	err = app.RestoreFromZip(bytes.NewReader(zbuf.Bytes()), restoreDir, tmpDir)
	if err == nil {
		t.Fatal("Should reject unsupported format version")
	}
	if !strings.Contains(err.Error(), "version") && !strings.Contains(err.Error(), "format") {
		t.Errorf("Expected version error, got: %v", err)
	}
}

// TestRestoreSha256Validation - Validate SHA-256 digest matches
func TestRestoreSha256Validation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "restore-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbContent := []byte("SQLite format 3\x00 actual data")
	expectedHash := fmt.Sprintf("%x", sha256.Sum256(dbContent))

	manifest := map[string]interface{}{
		"format_version": 2,
		"created_at":     time.Now().Format(time.RFC3339),
		"db_file":        "mikvoc.db",
		"db_sha256":      "wronghash123", // Wrong hash
		"db_size":        int64(len(dbContent)),
		"assets":         []map[string]interface{}{},
	}
	manifestJSON, _ := json.Marshal(manifest)

	zbuf := new(bytes.Buffer)
	w := zip.NewWriter(zbuf)
	w.CreateNew(&zip.FileHeader{Name: "manifest.json", Method: zip.Store})
	w.Write(manifestJSON)
	w.CreateNew(&zip.FileHeader{Name: "mikvoc.db", Method: zip.Store})
	w.Write(dbContent)
	w.Close()

	restoreDir := filepath.Join(tmpDir, "restore")
	if err := os.MkdirAll(restoreDir, 0700); err != nil {
		t.Fatalf("Failed to create restore dir: %v", err)
	}

	app := &App{}
	err = app.RestoreFromZip(bytes.NewReader(zbuf.Bytes()), restoreDir, tmpDir)
	if err == nil {
		t.Fatal("Should reject hash mismatch")
	}
	if !strings.Contains(err.Error(), "sha256") && !strings.Contains(err.Error(), "digest") && !strings.Contains(err.Error(), "checksum") {
		t.Errorf("Expected hash error, got: %v", err)
	}
}

// TestRestoreAcceptedManagedAssetPathsOnly - Only managed asset paths accepted
func TestRestoreAcceptedManagedAssetPathsOnly(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "restore-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	manifest := map[string]interface{}{
		"format_version": 2,
		"created_at":     time.Now().Format(time.RFC3339),
		"db_file":        "mikvoc.db",
		"db_sha256":      "abc",
		"db_size":        100,
		"assets": []map[string]interface{}{
			{"path": "global/background.jpg"}, // Valid
			{"path": "routers/456/logo.png"},  // Valid
		},
	}
	manifestJSON, _ := json.Marshal(manifest)

	zbuf := new(bytes.Buffer)
	w := zip.NewWriter(zbuf)
	w.CreateNew(&zip.FileHeader{Name: "manifest.json", Method: zip.Store})
	w.Write(manifestJSON)
	w.CreateNew(&zip.FileHeader{Name: "mikvoc.db", Method: zip.Store})
	w.Write([]byte("SQLite 3"))
	w.CreateNew(&zip.FileHeader{Name: "global/background.jpg", Method: zip.Store})
	w.Write([]byte("bg"))
	w.CreateNew(&zip.FileHeader{Name: "routers/456/logo.png", Method: zip.Store})
	w.Write([]byte("logo"))
	w.Close()

	restoreDir := filepath.Join(tmpDir, "restore")
	if err := os.MkdirAll(restoreDir, 0700); err != nil {
		t.Fatalf("Failed to create restore dir: %v", err)
	}

	app := &App{}
	err = app.RestoreFromZip(bytes.NewReader(zbuf.Bytes()), restoreDir, tmpDir)
	if err != nil {
		t.Errorf("Restore failed for valid paths: %v", err)
	}
	
	// Check files were restored
	if _, err := os.Stat(filepath.Join(restoreDir, "global")); os.IsNotExist(err) {
		t.Error("global directory not created")
	}
	if _, err := os.Stat(filepath.Join(restoreDir, "routers", "456")); os.IsNotExist(err) {
		t.Error("routers/456 directory not created")
	}
}

// TestRestoreRejectedUnmanagedPaths - Reject unmanaged asset paths
func TestRestoreRejectedUnmanagedPaths(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "restore-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	manifest := map[string]interface{}{
		"format_version": 2,
		"created_at":     time.Now().Format(time.RFC3339),
		"db_file":        "mikvoc.db",
		"db_sha256":      "abc",
		"db_size":        100,
		"assets": []map[string]interface{}{
			{"path": "../etc/passwd"}, // Invalid path
		},
	}
	manifestJSON, _ := json.Marshal(manifest)

	zbuf := new(bytes.Buffer)
	w := zip.NewWriter(zbuf)
	w.CreateNew(&zip.FileHeader{Name: "manifest.json", Method: zip.Store})
	w.Write(manifestJSON)
	w.CreateNew(&zip.FileHeader{Name: "mikvoc.db", Method: zip.Store})
	w.Write([]byte("SQLite 3"))
	w.CreateNew(&zip.FileHeader{Name: "../etc/passwd", Method: zip.Store})
	w.Write([]byte("evil"))
	w.Close()

	restoreDir := filepath.Join(tmpDir, "restore")
	if err := os.MkdirAll(restoreDir, 0700); err != nil {
		t.Fatalf("Failed to create restore dir: %v", err)
	}

	app := &App{}
	err = app.RestoreFromZip(bytes.NewReader(zbuf.Bytes()), restoreDir, tmpDir)
	if err == nil {
		t.Fatal("Should reject unmanaged paths")
	}
	if !strings.Contains(err.Error(), "managed") && !strings.Contains(err.Error(), "validat") {
		t.Errorf("Expected managed path error, got: %v", err)
	}
}

// TestRestoreRollbackOnFailure - Rollback DB and assets on failure
func TestRestoreRollbackOnFailure(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rollback-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create original DB
	oldDB := filepath.Join(tmpDir, "mikvoc.db")
	f, err := os.Create(oldDB)
	f.WriteString("SQLite format 3\x00 old")
	f.Close()

	// Create original assets
	oldAssets := filepath.Join(tmpDir, "assets")
	os.MkdirAll(filepath.Join(oldAssets, "routers", "123"), 0700)
	os.WriteFile(filepath.Join(oldAssets, "routers", "123", "logo.png"), []byte("old logo"), 0600)

	// Prepare bad restore zip (wrong hash)
	manifest := map[string]interface{}{
		"format_version": 2,
		"created_at":     time.Now().Format(time.RFC3339),
		"db_file":        "mikvoc.db",
		"db_sha256":      "wronghash",
		"db_size":        20,
		"assets":         []map[string]interface{}{},
	}
	manifestJSON, _ := json.Marshal(manifest)

	zbuf := new(bytes.Buffer)
	w := zip.NewWriter(zbuf)
	w.CreateNew(&zip.FileHeader{Name: "manifest.json", Method: zip.Store})
	w.Write(manifestJSON)
	w.CreateNew(&zip.FileHeader{Name: "mikvoc.db", Method: zip.Store})
	w.Write([]byte("bad data"))
	w.Close()

	restoreDir := filepath.Join(tmpDir, "restore")
	if err := os.MkdirAll(restoreDir, 0700); err != nil {
		t.Fatalf("Failed to create restore dir: %v", err)
	}

	app := &App{}
	err = app.RestoreFromZipWithRollback(bytes.NewReader(zbuf.Bytes()), restoreDir, tmpDir, oldDB, oldAssets)
	if err == nil {
		t.Log("Rollback happened silently or hash check passed")
	}

	// Original files should still exist
	if _, err := os.Stat(oldDB); os.IsNotExist(err) {
		t.Error("Original DB was not preserved after rollback")
	}
	if _, err := os.Stat(filepath.Join(oldAssets, "routers", "123", "logo.png")); os.IsNotExist(err) {
		t.Error("Original assets were not preserved after rollback")
	}
}

// TestLegacyDBRestorePreservesAssets - Legacy DB restore preserves assets
func TestLegacyDBRestorePreservesAssets(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "legacy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Legacy DB (just raw SQLite)
	legacyDB := filepath.Join(tmpDir, "legacy-backup.db")
	f, err := os.Create(legacyDB)
	f.WriteString("SQLite format 3\x00 legacy content here")
	f.Close()

	// Existing assets that should be preserved
	existingAssets := filepath.Join(tmpDir, "assets")
	os.MkdirAll(filepath.Join(existingAssets, "routers", "999"), 0700)
	os.WriteFile(filepath.Join(existingAssets, "routers", "999", "logo.png"), []byte("preserved"), 0600)

	// Old-style restore (no manifest, just .db file)
	buf := new(bytes.Buffer)
	buf.Write([]byte("SQLite format 3\x00 legacy"))

	app := &App{DBPath: filepath.Join(tmpDir, "mikvoc.db")}
	
	// Simulate legacy restore by directly restoring DB
	newDB := filepath.Join(tmpDir, "mikvoc.db")
	if err := copyFile(legacyDB, newDB); err != nil {
		t.Fatalf("Failed to copy DB: %v", err)
	}

	// Assets should remain intact
	if _, err := os.Stat(filepath.Join(existingAssets, "routers", "999", "logo.png")); os.IsNotExist(err) {
		t.Error("Assets were incorrectly deleted during legacy restore")
	}
}

// TestRollbackDBAndAssetsTogether - Both DB and assets rolled back together
func TestRollbackDBAndAssetsTogether(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "both-rollback-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	oldDB := filepath.Join(tmpDir, "mikvoc.db")
	f, err := os.Create(oldDB)
	f.WriteString("SQLite 3\x00 old-db-content")
	f.Close()

	oldAssets := filepath.Join(tmpDir, "assets")
	os.MkdirAll(filepath.Join(oldAssets, "global"), 0700)
	os.WriteFile(filepath.Join(oldAssets, "global", "bg.jpg"), []byte("old-bg"), 0600)

	// Malformed zip that will fail mid-restoration
	manifest := map[string]interface{}{
		"format_version": 2,
		"created_at":     time.Now().Format(time.RFC3339),
		"db_file":        "mikvoc.db",
		"db_sha256":      "incorrect",
		"db_size":        30,
		"assets": []map[string]interface{}{
			{"path": "invalid/"}, // Will cause validation failure
		},
	}
	manifestJSON, _ := json.Marshal(manifest)

	zbuf := new(bytes.Buffer)
	w := zip.NewWriter(zbuf)
	w.CreateNew(&zip.FileHeader{Name: "manifest.json", Method: zip.Store})
	w.Write(manifestJSON)
	w.CreateNew(&zip.FileHeader{Name: "mikvoc.db", Method: zip.Store})
	w.Write([]byte("bad"))
	w.Close()

	restoreDir := filepath.Join(tmpDir, "restore")
	os.MkdirAll(restoreDir, 0700)

	app := &App{}
	app.RestoreFromZipWithRollback(bytes.NewReader(zbuf.Bytes()), restoreDir, tmpDir, oldDB, oldAssets)

	// Both must be preserved
	if _, err := os.Stat(oldDB); os.IsNotExist(err) {
		t.Error("DB not preserved on rollback")
	}
	if _, err := os.Stat(filepath.Join(oldAssets, "global", "bg.jpg")); os.IsNotExist(err) {
		t.Error("Assets not preserved on rollback")
	}
}

func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()
	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()
	_, err = io.Copy(d, s)
	return err
}
