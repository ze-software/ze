//go:build ze_distro

package system

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/zefs"
)

const (
	testKeyVer    = "version"
	testKeySHA    = "sha256"
	testKeyPaused = "paused"
	testKeyMinVer = "minimum-version"
	testKeySize   = "size"
)

func computeSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func newTestBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	target := filepath.Join(dir, "ze-test")
	if err := os.WriteFile(target, []byte("old binary content for testing"), 0o755); err != nil {
		t.Fatal(err)
	}
	return target
}

func newTestServer(t *testing.T, manifest map[string]any, binaryContent []byte) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "version.json") {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(manifest); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		if _, err := w.Write(binaryContent); err != nil {
			return
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

func newTestUpdater(t *testing.T, serverURL, target string, cfg SelfUpdateConfig) *SelfUpdater {
	t.Helper()
	su := newSelfUpdater(serverURL+"/version.json", 86400, cfg, nil)
	su.running = "26.01.01"
	su.targetPath = target
	su.restartFunc = func(string) error { return nil }
	su.nowFunc = func() time.Time { return time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC) }
	su.identityFunc = func() string { return "test-device-1" }
	su.identity = "test-device-1"
	return su
}

func TestSelfUpdateFullCycle(t *testing.T) {
	target := newTestBinary(t)
	newBinary := []byte("new binary content v2")
	newHash := computeSHA256(newBinary)

	manifest := map[string]any{
		testKeyVer:  "26.05.20",
		testKeySHA:  newHash,
		testKeySize: len(newBinary),
	}
	ts := newTestServer(t, manifest, newBinary)

	su := newTestUpdater(t, ts.URL, target, SelfUpdateConfig{AutoApply: true, Spread: 0})
	su.check(context.Background())

	st := su.extendedStatus()
	if st.DownloadStatus != "staged" {
		t.Errorf("expected status staged, got %q", st.DownloadStatus)
	}
	if st.StagedVersion != "26.05.20" {
		t.Errorf("expected staged version 26.05.20, got %q", st.StagedVersion)
	}

	prevPath := target + ".prev"
	if !fileExists(prevPath) {
		t.Error(".prev backup not created")
	}

	staged, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(staged, newBinary) {
		t.Error("staged binary content mismatch")
	}

	events := su.History()
	if len(events) != 1 || events[0].Result != "success" {
		t.Errorf("expected 1 success event, got %v", events)
	}
}

func TestSelfUpdateChecksumMismatch(t *testing.T) {
	target := newTestBinary(t)
	newBinary := []byte("new binary content")

	manifest := map[string]any{
		testKeyVer:  "26.05.20",
		testKeySHA:  "0000000000000000000000000000000000000000000000000000000000000000",
		testKeySize: len(newBinary),
	}
	ts := newTestServer(t, manifest, newBinary)

	su := newTestUpdater(t, ts.URL, target, SelfUpdateConfig{AutoApply: true, Spread: 0})
	su.check(context.Background())

	st := su.extendedStatus()
	if !strings.Contains(st.DownloadStatus, "checksum mismatch") {
		t.Errorf("expected checksum mismatch error, got %q", st.DownloadStatus)
	}

	events := su.History()
	if len(events) != 1 || events[0].Result != "failed-checksum" {
		t.Errorf("expected failed-checksum event, got %v", events)
	}

	// Verify no temp files left
	dir := filepath.Dir(target)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".update.") {
			t.Errorf("temp file not cleaned up: %s", e.Name())
		}
	}
}

func TestSelfUpdateServerPaused(t *testing.T) {
	target := newTestBinary(t)
	manifest := map[string]any{
		testKeyVer:    "26.05.20",
		testKeySHA:    "abc123",
		testKeyPaused: true,
	}
	ts := newTestServer(t, manifest, nil)

	su := newTestUpdater(t, ts.URL, target, SelfUpdateConfig{AutoApply: true, Spread: 0})
	su.check(context.Background())

	st := su.extendedStatus()
	if st.DownloadStatus != "paused by server" {
		t.Errorf("expected paused by server, got %q", st.DownloadStatus)
	}
	if !st.ServerPaused {
		t.Error("expected ServerPaused=true")
	}
}

func TestSelfUpdateSpreadDeterministic(t *testing.T) {
	su1 := newSelfUpdater("http://localhost/version.json", 86400, SelfUpdateConfig{Spread: 3600}, nil)
	su1.identityFunc = func() string { return "device-A" }

	su2 := newSelfUpdater("http://localhost/version.json", 86400, SelfUpdateConfig{Spread: 3600}, nil)
	su2.identityFunc = func() string { return "device-A" }

	su3 := newSelfUpdater("http://localhost/version.json", 86400, SelfUpdateConfig{Spread: 3600}, nil)
	su3.identityFunc = func() string { return "device-B" }

	d1 := su1.computeSpreadDelay("26.05.20")
	d2 := su2.computeSpreadDelay("26.05.20")
	d3 := su3.computeSpreadDelay("26.05.20")

	if d1 != d2 {
		t.Errorf("same device+version should produce same delay: %v vs %v", d1, d2)
	}
	if d1 == d3 {
		t.Log("different devices happened to get same delay (unlikely but possible)")
	}
	if d1 >= 3600*time.Second {
		t.Errorf("delay should be < spread: %v", d1)
	}
}

func TestSelfUpdateMaintenanceWindow(t *testing.T) {
	target := newTestBinary(t)
	newBinary := []byte("new binary v2")
	newHash := computeSHA256(newBinary)

	manifest := map[string]any{
		testKeyVer: "26.05.20",
		testKeySHA: newHash,
	}
	ts := newTestServer(t, manifest, newBinary)

	// Outside window: 12:00, window is 02:00-06:00
	su := newTestUpdater(t, ts.URL, target, SelfUpdateConfig{
		AutoApply:        true,
		Spread:           0,
		MaintenanceStart: "02:00",
		MaintenanceEnd:   "06:00",
	})
	su.nowFunc = func() time.Time { return time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC) }

	su.check(context.Background())

	st := su.extendedStatus()
	if st.DownloadStatus != "waiting for maintenance window" {
		t.Errorf("expected waiting for maintenance window, got %q", st.DownloadStatus)
	}

	// Inside window: 03:00
	target2 := newTestBinary(t)
	su2 := newTestUpdater(t, ts.URL, target2, SelfUpdateConfig{
		AutoApply:        true,
		Spread:           0,
		MaintenanceStart: "02:00",
		MaintenanceEnd:   "06:00",
	})
	su2.nowFunc = func() time.Time { return time.Date(2026, 5, 21, 3, 0, 0, 0, time.UTC) }

	su2.check(context.Background())

	st2 := su2.extendedStatus()
	if st2.DownloadStatus != "staged" {
		t.Errorf("expected staged (inside window), got %q", st2.DownloadStatus)
	}
}

func TestSelfUpdateMaintenanceWindowMidnight(t *testing.T) {
	su := newSelfUpdater("http://localhost/version.json", 86400, SelfUpdateConfig{
		MaintenanceStart: "22:00",
		MaintenanceEnd:   "06:00",
	}, nil)

	// 23:00 should be in window
	su.nowFunc = func() time.Time { return time.Date(2026, 5, 21, 23, 0, 0, 0, time.UTC) }
	if !su.inMaintenanceWindow() {
		t.Error("23:00 should be inside 22:00-06:00 window")
	}

	// 03:00 should be in window
	su.nowFunc = func() time.Time { return time.Date(2026, 5, 22, 3, 0, 0, 0, time.UTC) }
	if !su.inMaintenanceWindow() {
		t.Error("03:00 should be inside 22:00-06:00 window")
	}

	// 12:00 should be outside window
	su.nowFunc = func() time.Time { return time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC) }
	if su.inMaintenanceWindow() {
		t.Error("12:00 should be outside 22:00-06:00 window")
	}

	// 21:59 should be outside window
	su.nowFunc = func() time.Time { return time.Date(2026, 5, 21, 21, 59, 0, 0, time.UTC) }
	if su.inMaintenanceWindow() {
		t.Error("21:59 should be outside 22:00-06:00 window")
	}

	// 06:00 should be outside window (end is exclusive)
	su.nowFunc = func() time.Time { return time.Date(2026, 5, 22, 6, 0, 0, 0, time.UTC) }
	if su.inMaintenanceWindow() {
		t.Error("06:00 should be outside 22:00-06:00 window (end exclusive)")
	}
}

func TestSelfUpdateMinimumVersion(t *testing.T) {
	target := newTestBinary(t)
	manifest := map[string]any{
		testKeyVer:    "26.05.20",
		testKeySHA:    "abc123",
		testKeyMinVer: "26.03.01",
	}
	ts := newTestServer(t, manifest, nil)

	su := newTestUpdater(t, ts.URL, target, SelfUpdateConfig{AutoApply: true, Spread: 0})
	su.check(context.Background())

	st := su.extendedStatus()
	if !strings.Contains(st.DownloadStatus, "upgrade requires intermediate") {
		t.Errorf("expected minimum version error, got %q", st.DownloadStatus)
	}

	events := su.History()
	if len(events) != 1 || events[0].Result != "blocked-minimum-version" {
		t.Errorf("expected blocked-minimum-version event, got %v", events)
	}
}

func TestSelfUpdateDiskSpaceCheck(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "ze-test")
	if err := os.WriteFile(target, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}

	su := newSelfUpdater("http://localhost/version.json", 86400, SelfUpdateConfig{}, nil)
	su.targetPath = target

	// Should not error for reasonable size (disk is not full)
	err := su.checkDiskSpace(1024)
	if err != nil {
		t.Errorf("unexpected error for small size: %v", err)
	}
}

func TestSelfUpdateAtomicRename(t *testing.T) {
	target := newTestBinary(t)

	su := newSelfUpdater("http://localhost/version.json", 86400, SelfUpdateConfig{}, nil)
	su.targetPath = target

	// Create a temp file to stage
	dir := filepath.Dir(target)
	newContent := []byte("new binary for atomic test")
	tmpPath := filepath.Join(dir, "ze-test.update.test123")
	if err := os.WriteFile(tmpPath, newContent, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := su.stageBinary(tmpPath); err != nil {
		t.Fatalf("stageBinary failed: %v", err)
	}

	// Verify target has new content
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, newContent) {
		t.Error("target content mismatch after stage")
	}

	// Verify .prev exists with old content
	prevData, err := os.ReadFile(target + ".prev")
	if err != nil {
		t.Fatal(".prev not created")
	}
	if string(prevData) != "old binary content for testing" {
		t.Error(".prev content mismatch")
	}
}

func TestSelfUpdateRollback(t *testing.T) {
	target := newTestBinary(t)

	// Create .prev
	prevContent := []byte("previous version binary")
	if err := os.WriteFile(target+".prev", prevContent, 0o755); err != nil {
		t.Fatal(err)
	}

	restarted := false
	su := newSelfUpdater("http://localhost/version.json", 86400, SelfUpdateConfig{}, nil)
	su.targetPath = target
	su.restartFunc = func(string) error { restarted = true; return nil }

	if err := su.Rollback(); err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	if !restarted {
		t.Error("restart not called after rollback")
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, prevContent) {
		t.Error("target should have .prev content after rollback")
	}

	if fileExists(target + ".prev") {
		t.Error(".prev should not exist after rollback (renamed to target)")
	}
}

func TestSelfUpdateRollbackNoPrev(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "ze-test")
	if err := os.WriteFile(target, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}

	su := newSelfUpdater("http://localhost/version.json", 86400, SelfUpdateConfig{}, nil)
	su.targetPath = target
	su.restartFunc = func(string) error { return nil }

	err := su.Rollback()
	if err == nil {
		t.Fatal("expected error when no .prev")
	}
	if !strings.Contains(err.Error(), "no previous version") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSelfUpdateHistory(t *testing.T) {
	dir := t.TempDir()
	su := newSelfUpdater("http://localhost/version.json", 86400, SelfUpdateConfig{}, nil)
	su.targetPath = filepath.Join(dir, "ze-test")
	su.nowFunc = func() time.Time { return time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC) }

	// Record more than 20 events
	for i := range 25 {
		su.recordEvent("26.01.01", "26.05."+string(rune('A'+i)), "success")
	}

	events := su.History()
	if len(events) != historyMaxEvents {
		t.Errorf("expected %d events, got %d", historyMaxEvents, len(events))
	}
}

func TestSelfUpdateNoSha256AutoApply(t *testing.T) {
	target := newTestBinary(t)
	manifest := map[string]any{
		testKeyVer: "26.05.20",
		// no sha256
	}
	ts := newTestServer(t, manifest, nil)

	su := newTestUpdater(t, ts.URL, target, SelfUpdateConfig{AutoApply: true, Spread: 0})
	su.check(context.Background())

	st := su.extendedStatus()
	if !strings.Contains(st.DownloadStatus, "auto-apply requires server to provide sha256") {
		t.Errorf("expected sha256 required error, got %q", st.DownloadStatus)
	}
}

func TestSelfUpdateManualNoSha256(t *testing.T) {
	target := newTestBinary(t)
	newBinary := []byte("new binary no hash")
	manifest := map[string]any{
		testKeyVer: "26.05.20",
	}
	ts := newTestServer(t, manifest, newBinary)

	su := newTestUpdater(t, ts.URL, target, SelfUpdateConfig{})

	ver, err := su.manualDownload(context.Background())
	if err != nil {
		t.Fatalf("ManualDownload without sha256 failed: %v", err)
	}
	if ver != "26.05.20" {
		t.Errorf("expected version 26.05.20, got %q", ver)
	}
	st := su.extendedStatus()
	if st.DownloadStatus != "complete" {
		t.Errorf("expected download complete, got %q", st.DownloadStatus)
	}
}

func TestSelfUpdateManualBypassesPause(t *testing.T) {
	target := newTestBinary(t)
	newBinary := []byte("new binary paused server")
	newHash := computeSHA256(newBinary)
	manifest := map[string]any{
		testKeyVer:    "26.05.20",
		testKeySHA:    newHash,
		testKeyPaused: true,
	}
	ts := newTestServer(t, manifest, newBinary)

	su := newTestUpdater(t, ts.URL, target, SelfUpdateConfig{})

	ver, err := su.manualDownload(context.Background())
	if err != nil {
		t.Fatalf("ManualDownload with pause failed: %v", err)
	}
	if ver != "26.05.20" {
		t.Errorf("expected version 26.05.20, got %q", ver)
	}
}

func TestSelfUpdateDownloadURLValidation(t *testing.T) {
	su := newSelfUpdater("https://update.example.com/version.json", 86400, SelfUpdateConfig{}, nil)

	// HTTPS download URL should be accepted
	_, err := su.resolveDownloadURL(extendedManifest{DownloadURL: "https://cdn.example.com/ze"})
	if err != nil {
		t.Errorf("HTTPS should be accepted: %v", err)
	}

	// HTTP non-localhost should be rejected
	_, err = su.resolveDownloadURL(extendedManifest{DownloadURL: "http://evil.com/ze"})
	if err == nil {
		t.Error("HTTP non-localhost should be rejected")
	}

	// HTTP localhost should be accepted
	_, err = su.resolveDownloadURL(extendedManifest{DownloadURL: "http://127.0.0.1:8080/ze"})
	if err != nil {
		t.Errorf("HTTP localhost should be accepted: %v", err)
	}

	// A host that merely BEGINS with a loopback spelling is not loopback. The
	// rule compared it by prefix until 2026-09-03, so an update host serving a
	// manifest could name either of these and have the binary fetched over
	// plain HTTP from a host it chose.
	for _, lookalike := range []string{
		"http://127.0.0.1.example.com/ze",
		"http://localhost.example.com/ze",
	} {
		if _, err := su.resolveDownloadURL(extendedManifest{DownloadURL: lookalike}); err == nil {
			t.Errorf("%q was accepted as loopback", lookalike)
		}
	}

	// Empty URL should derive from config
	url, err := su.resolveDownloadURL(extendedManifest{})
	if err != nil {
		t.Errorf("empty URL derivation failed: %v", err)
	}
	if !strings.HasPrefix(url, "https://update.example.com/") {
		t.Errorf("derived URL unexpected: %q", url)
	}
}

func TestSelfUpdateHeldTempSkipsRedownload(t *testing.T) {
	target := newTestBinary(t)
	newBinary := []byte("new binary held")
	newHash := computeSHA256(newBinary)

	downloadCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "version.json") {
			m := map[string]any{testKeyVer: "26.05.20", testKeySHA: newHash}
			if err := json.NewEncoder(w).Encode(m); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		downloadCount++
		if _, err := w.Write(newBinary); err != nil {
			return
		}
	}))
	t.Cleanup(ts.Close)

	su := newTestUpdater(t, ts.URL, target, SelfUpdateConfig{
		AutoApply:        true,
		Spread:           0,
		MaintenanceStart: "02:00",
		MaintenanceEnd:   "06:00",
	})
	// Time outside window: download but don't stage
	su.nowFunc = func() time.Time { return time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC) }

	su.check(context.Background())
	if downloadCount != 1 {
		t.Fatalf("expected 1 download, got %d", downloadCount)
	}

	// Second check with same version: should not re-download
	su.check(context.Background())
	if downloadCount != 1 {
		t.Errorf("expected no re-download, got %d downloads", downloadCount)
	}

	st := su.extendedStatus()
	if st.DownloadStatus != "waiting for maintenance window" {
		t.Errorf("expected waiting for maintenance window, got %q", st.DownloadStatus)
	}
}

func TestSelfUpdateHeldTempDiscardedOnNewVersion(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "ze-test")
	if err := os.WriteFile(target, []byte("original"), 0o755); err != nil {
		t.Fatal(err)
	}

	currentVer := "26.05.20"
	newBinary := []byte("binary v20")
	newHash := computeSHA256(newBinary)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "version.json") {
			m := map[string]any{testKeyVer: currentVer, testKeySHA: newHash}
			if err := json.NewEncoder(w).Encode(m); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		if _, err := w.Write(newBinary); err != nil {
			return
		}
	}))
	t.Cleanup(ts.Close)

	su := newTestUpdater(t, ts.URL, target, SelfUpdateConfig{
		AutoApply:        true,
		Spread:           0,
		MaintenanceStart: "02:00",
		MaintenanceEnd:   "06:00",
	})
	su.nowFunc = func() time.Time { return time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC) }

	// First check: downloads v20, held outside window
	su.check(context.Background())

	su.mu.RLock()
	heldTemp := su.verifiedTempPath
	su.mu.RUnlock()
	if heldTemp == "" {
		t.Fatal("expected held temp path")
	}
	if !fileExists(heldTemp) {
		t.Fatal("held temp file should exist")
	}

	// Server publishes v21
	newBinary2 := []byte("binary v21")
	newHash2 := computeSHA256(newBinary2)
	currentVer = "26.05.21"
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "version.json") {
			m := map[string]any{testKeyVer: currentVer, testKeySHA: newHash2}
			if err := json.NewEncoder(w).Encode(m); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		if _, err := w.Write(newBinary2); err != nil {
			return
		}
	})

	su.check(context.Background())

	// Old temp should be gone
	if fileExists(heldTemp) {
		t.Error("old temp file should have been deleted")
	}
}

func TestSelfUpdateStaleCleanup(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "ze-test")
	if err := os.WriteFile(target, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create stale temp files
	stale1 := filepath.Join(dir, "ze-test.update.abc123")
	stale2 := filepath.Join(dir, "ze-test.update.def456")
	if err := os.WriteFile(stale1, []byte("stale1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale2, []byte("stale2"), 0o644); err != nil {
		t.Fatal(err)
	}

	su := newSelfUpdater("http://localhost/version.json", 86400, SelfUpdateConfig{}, nil)
	su.targetPath = target
	su.cleanStaleTempFiles()

	if fileExists(stale1) {
		t.Error("stale1 not cleaned")
	}
	if fileExists(stale2) {
		t.Error("stale2 not cleaned")
	}
}

// newHistoryStore registers a fresh temp database.zefs as the process-wide
// statestore backend, so save/loadHistory (which go through statestore) hit the
// real zefs store, not a loose file. Resets the global store on cleanup.
func newHistoryStore(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "database.zefs")
	bs, err := zefs.Create(path)
	if err != nil {
		t.Fatalf("zefs.Create: %v", err)
	}
	statestore.SetStore(bs)
	t.Cleanup(func() {
		statestore.SetStore(nil)
		if cerr := bs.Close(); cerr != nil {
			t.Errorf("close store: %v", cerr)
		}
	})
}

// VALIDATES: update history round-trips through the shared zefs store (save via
// recordEvent, load into a fresh updater), not a loose JSON file.
func TestSelfUpdateHistoryPersist(t *testing.T) {
	newHistoryStore(t)
	target := filepath.Join(t.TempDir(), "ze-test")

	su := newSelfUpdater("http://localhost/version.json", 86400, SelfUpdateConfig{}, nil)
	su.targetPath = target
	su.nowFunc = func() time.Time { return time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC) }

	su.recordEvent("26.01.01", "26.05.20", "success")
	su.recordEvent("26.05.20", "26.05.21", "failed-checksum")

	// New updater loads from the persisted zefs store.
	su2 := newSelfUpdater("http://localhost/version.json", 86400, SelfUpdateConfig{}, nil)
	su2.targetPath = target
	su2.loadHistory()

	events := su2.History()
	if len(events) != 2 {
		t.Fatalf("expected 2 persisted events, got %d", len(events))
	}
	if events[0].Result != "success" {
		t.Errorf("first event result: %q", events[0].Result)
	}
	if events[1].Result != "failed-checksum" {
		t.Errorf("second event result: %q", events[1].Result)
	}
}

// VALIDATES: a corrupt update-history blob in the zefs store yields empty history
// without error (best-effort restore, no crash).
func TestSelfUpdateHistoryPersistCorrupt(t *testing.T) {
	newHistoryStore(t)
	target := filepath.Join(t.TempDir(), "ze-test")

	// Corrupt blob under the update-history key in the registered store.
	if _, err := statestore.Put(zefs.KeyConfigUpdateHistory.Pattern, []byte("not json{{{")); err != nil {
		t.Fatal(err)
	}

	su := newSelfUpdater("http://localhost/version.json", 86400, SelfUpdateConfig{}, nil)
	su.targetPath = target
	su.loadHistory()

	events := su.History()
	if len(events) != 0 {
		t.Errorf("corrupt blob should yield empty history, got %d events", len(events))
	}
}

func TestSelfUpdateConfigValidation(t *testing.T) {
	// immediate + time = error
	err := validateSelfUpdateConfig(SelfUpdateConfig{
		RestartImmediate: true,
		RestartTime:      "03:00",
	})
	if err == nil {
		t.Error("immediate+time should be rejected")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("unexpected error: %v", err)
	}

	// Malformed time
	err = validateSelfUpdateConfig(SelfUpdateConfig{RestartTime: "25:00"})
	if err == nil {
		t.Error("25:00 should be rejected")
	}

	err = validateSelfUpdateConfig(SelfUpdateConfig{RestartTime: "abc"})
	if err == nil {
		t.Error("abc should be rejected")
	}

	// Valid configs
	err = validateSelfUpdateConfig(SelfUpdateConfig{RestartTime: "03:00"})
	if err != nil {
		t.Errorf("03:00 should be valid: %v", err)
	}

	err = validateSelfUpdateConfig(SelfUpdateConfig{RestartImmediate: true})
	if err != nil {
		t.Errorf("immediate alone should be valid: %v", err)
	}

	err = validateSelfUpdateConfig(SelfUpdateConfig{})
	if err != nil {
		t.Errorf("empty config should be valid: %v", err)
	}

	// Malformed maintenance window
	err = validateSelfUpdateConfig(SelfUpdateConfig{MaintenanceStart: "99:99"})
	if err == nil {
		t.Error("99:99 should be rejected")
	}
}

func TestParseHHMM(t *testing.T) {
	tests := []struct {
		input string
		valid bool
		h, m  int
	}{
		{"00:00", true, 0, 0},
		{"23:59", true, 23, 59},
		{"12:30", true, 12, 30},
		{"24:00", false, 0, 0},
		{"12:60", false, 0, 0},
		{"abc", false, 0, 0},
		{"1:00", false, 0, 0},
		{"", false, 0, 0},
	}

	for _, tt := range tests {
		result, err := parseHHMM(tt.input)
		if tt.valid {
			if err != nil {
				t.Errorf("parseHHMM(%q) unexpected error: %v", tt.input, err)
			}
			if result.hour != tt.h || result.minute != tt.m {
				t.Errorf("parseHHMM(%q) = %d:%d, want %d:%d", tt.input, result.hour, result.minute, tt.h, tt.m)
			}
		} else if err == nil {
			t.Errorf("parseHHMM(%q) expected error", tt.input)
		}
	}
}
