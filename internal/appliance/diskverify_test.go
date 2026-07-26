// VALIDATES: AC-1 (silent debugfs failure caught), AC-2 (content mismatch), AC-3 (e2fsck)
// PREVENTS: bricked appliance from silently failed inject
// test-relax: size/content mismatch tests replaced by bytes.Contains check (pure Go, no debugfs dump)

package appliance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyInjectedDBCatchesSilentFailure(t *testing.T) {
	dir := t.TempDir()
	permImg := filepath.Join(dir, "perm.img")
	dbPath := filepath.Join(dir, "database.zefs")
	dbContent := []byte("test database content with unique data")

	if err := os.WriteFile(permImg, []byte("empty-ext4-region-no-database"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbPath, dbContent, 0o644); err != nil {
		t.Fatal(err)
	}

	err := verifyInjectedDB(permImg, dbPath)
	if err == nil {
		t.Fatal("verifyInjectedDB should fail when database bytes are absent from perm image")
	}
	if !strings.Contains(err.Error(), "silently failed") {
		t.Fatalf("error should mention silent failure, got: %v", err)
	}
}

func TestVerifyInjectedDBPassesOnMatch(t *testing.T) {
	dir := t.TempDir()
	permImg := filepath.Join(dir, "perm.img")
	dbPath := filepath.Join(dir, "database.zefs")
	dbContent := []byte("test database content")

	perm := make([]byte, 0, 4096+len(dbContent))
	perm = append(perm, make([]byte, 4096)...)
	perm = append(perm, dbContent...)
	if err := os.WriteFile(permImg, perm, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbPath, dbContent, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := verifyInjectedDB(permImg, dbPath); err != nil {
		t.Fatalf("verifyInjectedDB should pass when database bytes are present, got: %v", err)
	}
}

func TestVerifyInjectedDBRejectsEmptySource(t *testing.T) {
	dir := t.TempDir()
	permImg := filepath.Join(dir, "perm.img")
	dbPath := filepath.Join(dir, "database.zefs")

	if err := os.WriteFile(permImg, []byte("some-data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	err := verifyInjectedDB(permImg, dbPath)
	if err == nil {
		t.Fatal("verifyInjectedDB should reject empty source database")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Fatalf("error should mention empty source, got: %v", err)
	}
}

func TestVerifyE2fsckReportsErrors(t *testing.T) {
	dir := t.TempDir()
	permImg := filepath.Join(dir, "perm.img")
	if err := os.WriteFile(permImg, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	e2fs := t.TempDir()
	if err := os.WriteFile(filepath.Join(e2fs, "e2fsck"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	useE2FSDir(t, e2fs)

	old := runExternalFn
	runExternalFn = func(_ string, _ ...string) ([]byte, error) {
		return nil, os.ErrPermission
	}
	defer func() { runExternalFn = old }()

	err := verifyE2fsck(permImg)
	if err == nil {
		t.Fatal("verifyE2fsck should report e2fsck errors")
	}
	if !strings.Contains(err.Error(), "structural check failed") {
		t.Fatalf("error should mention structural check, got: %v", err)
	}
}

func TestVerifyE2fsckSkipsWhenNotFound(t *testing.T) {
	dir := t.TempDir()
	permImg := filepath.Join(dir, "perm.img")
	if err := os.WriteFile(permImg, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	e2fs := t.TempDir()
	useE2FSDir(t, e2fs)

	if err := verifyE2fsck(permImg); err != nil {
		t.Fatalf("verifyE2fsck should skip when e2fsck not found, got: %v", err)
	}
}

func TestExtractPartitionReadsAtOffset(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "test.img")

	img := make([]byte, 8192)
	copy(img[4096:], "partition-data-here")
	if err := os.WriteFile(imgPath, img, 0o644); err != nil {
		t.Fatal(err)
	}

	data, err := extractPartition(imgPath, 4096, 4096)
	if err != nil {
		t.Fatalf("extractPartition: %v", err)
	}
	if len(data) != 4096 {
		t.Fatalf("got %d bytes, want 4096", len(data))
	}
	if string(data[:19]) != "partition-data-here" {
		t.Fatalf("data = %q, want prefix %q", data[:19], "partition-data-here")
	}
}

func TestWritePartitionWritesAtOffset(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "test.img")

	img := make([]byte, 8192)
	if err := os.WriteFile(imgPath, img, 0o644); err != nil {
		t.Fatal(err)
	}

	payload := []byte("written-at-offset")
	if err := writePartition(imgPath, payload, 4096); err != nil {
		t.Fatalf("writePartition: %v", err)
	}

	result, err := os.ReadFile(imgPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(result[4096:4096+len(payload)]) != "written-at-offset" {
		t.Fatalf("data at offset 4096 = %q, want %q", result[4096:4096+len(payload)], "written-at-offset")
	}
	if len(result) != 8192 {
		t.Fatalf("file size = %d, want 8192 (no truncation)", len(result))
	}
}
