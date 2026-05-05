package archive_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/archive"
	"codeberg.org/thomas-mangin/ze/internal/component/config/system"
)

// --- FormatFilename tests ---

// TestFormatFilename verifies token substitution in filename format.
//
// VALIDATES: Token substitution produces correct filename with .conf extension.
// PREVENTS: Malformed archive filenames breaking retrieval.
func TestFormatFilename(t *testing.T) {
	sys := system.SystemConfig{Host: "router1", Domain: "dc1.example.com"}
	ts := time.Date(2026, 3, 9, 14, 30, 45, 0, time.UTC)

	name := archive.FormatFilename("{name}-{host}-{date}-{time}", "myconfig.conf", sys, "backup", ts)
	assert.Equal(t, "myconfig-router1-20260309-143045.conf", name)
}

// TestFormatFilename_Default verifies default format when none specified.
//
// VALIDATES: Empty format string uses default "{name}-{host}-{date}-{time}".
// PREVENTS: Empty filename when format is omitted from config.
func TestFormatFilename_Default(t *testing.T) {
	sys := system.SystemConfig{Host: "host1"}
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	name := archive.FormatFilename("", "config.conf", sys, "test", ts)
	assert.Equal(t, "config-host1-20260101-000000.conf", name)
}

// TestFormatFilename_AllTokens verifies all 6 tokens are substituted.
//
// VALIDATES: All tokens ({name}, {host}, {domain}, {date}, {time}, {archive}) work.
// PREVENTS: Token left unsubstituted in filename.
func TestFormatFilename_AllTokens(t *testing.T) {
	sys := system.SystemConfig{Host: "r1", Domain: "lab.net"}
	ts := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

	name := archive.FormatFilename(
		"{name}_{host}_{domain}_{date}_{time}_{archive}",
		"test.conf", sys, "offsite", ts,
	)
	assert.Equal(t, "test_r1_lab.net_20260310_120000_offsite.conf", name)
}

// TestFormatFilename_NoExtension verifies basename extraction without extension.
//
// VALIDATES: Basename extraction works with no file extension.
// PREVENTS: Extension logic breaking on extensionless filenames.
func TestFormatFilename_NoExtension(t *testing.T) {
	sys := system.SystemConfig{Host: "host"}
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	name := archive.FormatFilename("{name}-{host}", "config", sys, "test", ts)
	assert.Equal(t, "config-host.conf", name)
}

// --- ValidateTrigger tests ---

// TestValidateTrigger verifies valid trigger keywords are accepted.
//
// VALIDATES: All four trigger keywords (commit, manual, daily, hourly) are valid.
// PREVENTS: Valid trigger keywords being rejected.
func TestValidateTrigger(t *testing.T) {
	for _, trigger := range []string{"commit", "manual", "daily", "hourly"} {
		t.Run(trigger, func(t *testing.T) {
			assert.NoError(t, archive.ValidateTrigger(trigger))
		})
	}
}

// TestValidateTrigger_Invalid verifies invalid trigger keywords are rejected.
//
// VALIDATES: Invalid and empty trigger values produce errors.
// PREVENTS: Silent acceptance of typos like "weekly" or "comit".
func TestValidateTrigger_Invalid(t *testing.T) {
	for _, trigger := range []string{"", "weekly", "comit", "always"} {
		t.Run(trigger, func(t *testing.T) {
			assert.Error(t, archive.ValidateTrigger(trigger))
		})
	}
}

// --- ValidateLocation tests ---

// TestValidateLocation_Valid verifies URL parsing for file and HTTP schemes.
//
// VALIDATES: file://, http://, and https:// schemes are accepted.
// PREVENTS: Valid archive URLs being rejected.
func TestValidateLocation_Valid(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"file URL", "file:///backups/configs"},
		{"http URL", "http://config-server.example.com/archive"},
		{"https URL", "https://config-server.example.com/archive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := archive.ValidateLocation(tt.url)
			assert.NoError(t, err)
		})
	}
}

// TestValidateLocation_Invalid verifies rejection of unsupported schemes.
//
// VALIDATES: Unsupported schemes (scp, sftp, ftp, empty) are rejected.
// PREVENTS: Silent acceptance of unsupported protocols.
func TestValidateLocation_Invalid(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"scp scheme", "scp://user@host:/path"},
		{"sftp scheme", "sftp://user@host/path"},
		{"ftp scheme", "ftp://server/path"},
		{"empty string", ""},
		{"no scheme", "just-a-path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := archive.ValidateLocation(tt.url)
			assert.Error(t, err)
		})
	}
}

// --- ToFile tests ---

// TestToFile verifies file:// upload creates archive copy.
//
// VALIDATES: file:// scheme creates a copy with correct name in target directory.
// PREVENTS: File archive silently failing or writing to wrong location.
func TestToFile(t *testing.T) {
	content := []byte("bgp { local-as 65000; }")
	destDir := t.TempDir()

	err := archive.ToFile(content, destDir, "test-router1-20260309-143045.conf")
	require.NoError(t, err)

	expectedPath := filepath.Join(destDir, "test-router1-20260309-143045.conf")
	data, readErr := os.ReadFile(expectedPath)
	require.NoError(t, readErr)
	assert.Equal(t, content, data)
}

// TestToFile_CreatesSubdir verifies MkdirAll creates missing subdirectories.
//
// VALIDATES: ToFile creates missing parent directories.
// PREVENTS: Failure when archive directory doesn't pre-exist.
func TestToFile_CreatesSubdir(t *testing.T) {
	base := t.TempDir()
	subDir := filepath.Join(base, "deep", "nested", "dir")

	err := archive.ToFile([]byte("data"), subDir, "test.conf")
	require.NoError(t, err)

	data, readErr := os.ReadFile(filepath.Join(subDir, "test.conf"))
	require.NoError(t, readErr)
	assert.Equal(t, []byte("data"), data)
}

// TestToFile_PermissionError verifies error on unwritable path.
//
// VALIDATES: Unwritable destination directory produces clear error.
// PREVENTS: Silent failure or panic on permission errors.
func TestToFile_PermissionError(t *testing.T) {
	destDir := t.TempDir()
	err := archive.ToFile([]byte("data"), destDir, ".")
	assert.Error(t, err)
}

// --- ToHTTP tests ---

// TestToHTTP verifies HTTP POST upload.
//
// VALIDATES: HTTP archive sends config as POST with text/plain Content-Type.
// PREVENTS: Wrong HTTP method, content type, or body.
func TestToHTTP(t *testing.T) {
	var receivedBody string
	var receivedContentType string
	var receivedMethod string
	var receivedFilename string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedContentType = r.Header.Get("Content-Type")
		receivedFilename = r.Header.Get("X-Archive-Filename")
		data, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Fatalf("failed to read request body: %v", readErr)
		}
		receivedBody = string(data)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	content := []byte("bgp { local-as 65000; }")
	err := archive.ToHTTP(content, server.URL, "test-router1-20260309-143045.conf", 5*time.Second)
	require.NoError(t, err)

	assert.Equal(t, "POST", receivedMethod)
	assert.Equal(t, "text/plain", receivedContentType)
	assert.Equal(t, string(content), receivedBody)
	assert.Equal(t, "test-router1-20260309-143045.conf", receivedFilename)
}

// TestToHTTP_ServerError verifies error handling on non-2xx response.
//
// VALIDATES: Non-2xx HTTP responses produce an error.
// PREVENTS: Silent acceptance of failed uploads.
func TestToHTTP_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	err := archive.ToHTTP([]byte("data"), server.URL, "test.conf", 5*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

// TestToHTTP_Unreachable verifies error on connection failure.
//
// VALIDATES: Unreachable HTTP server produces timeout/connection error.
// PREVENTS: Hanging on unreachable servers.
func TestToHTTP_Unreachable(t *testing.T) {
	err := archive.ToHTTP([]byte("data"), "http://192.0.2.1:9999/archive", "test.conf", 100*time.Millisecond)
	assert.Error(t, err)
}

// --- NewNotifier tests ---

// TestNewNotifier verifies the constructor creates a working notifier.
//
// VALIDATES: NewNotifier creates a notifier that writes to file:// locations.
// PREVENTS: Constructor returning a non-functional notifier.
func TestNewNotifier(t *testing.T) {
	destDir := t.TempDir()
	sys := system.SystemConfig{Host: "myhost"}
	configs := []archive.ArchiveConfig{
		{
			Name:     "test-backup",
			Location: "file://" + destDir,
			Filename: "{name}-{host}-{date}-{time}",
			Timeout:  30 * time.Second,
			Trigger:  archive.TriggerCommit,
		},
	}

	notifier := archive.NewNotifier("test.conf", configs, sys)
	errs := notifier([]byte("config content"))
	assert.Empty(t, errs)

	entries, readErr := os.ReadDir(destDir)
	require.NoError(t, readErr)
	assert.Len(t, entries, 1)
	assert.Contains(t, entries[0].Name(), "test-myhost-")
}

// --- ExtractConfigs tests ---

// TestExtractConfigs verifies named block extraction from config tree.
//
// VALIDATES: Named archive blocks are extracted with all fields from system.archive.
// PREVENTS: Config-driven archive locations being inaccessible.
func TestExtractConfigs(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	entry := config.NewTree()
	entry.Set("location", "file:///backups")
	entry.Set("trigger", "commit")
	entry.Set("filename", "{name}-{host}")
	entry.Set("timeout", "10s")
	entry.Set("on-change", "true")
	sys.AddListEntry("archive", "local-backup", entry)

	configs := archive.ExtractConfigs(tree)
	require.Len(t, configs, 1)

	ac := configs[0]
	assert.Equal(t, "local-backup", ac.Name)
	assert.Equal(t, "file:///backups", ac.Location)
	assert.Equal(t, "{name}-{host}", ac.Filename)
	assert.Equal(t, 10*time.Second, ac.Timeout)
	assert.Equal(t, "commit", ac.Trigger)
	assert.True(t, ac.OnChange)
}

// TestExtractConfigs_Defaults verifies default values for optional fields.
//
// VALIDATES: Missing optional fields get correct defaults.
// PREVENTS: Zero-value defaults causing unexpected behavior.
func TestExtractConfigs_Defaults(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	entry := config.NewTree()
	entry.Set("location", "file:///backups")
	sys.AddListEntry("archive", "minimal", entry)

	configs := archive.ExtractConfigs(tree)
	require.Len(t, configs, 1)

	ac := configs[0]
	assert.Equal(t, "minimal", ac.Name)
	assert.Equal(t, "file:///backups", ac.Location)
	assert.Equal(t, archive.DefaultFilenameFormat, ac.Filename)
	assert.Equal(t, 30*time.Second, ac.Timeout)
	assert.Equal(t, "manual", ac.Trigger)
	assert.False(t, ac.OnChange)
}

// TestExtractConfigs_MultipleBlocks verifies multiple named blocks extraction.
//
// VALIDATES: Multiple named archive blocks are all extracted correctly.
// PREVENTS: Only first or last block being returned.
func TestExtractConfigs_MultipleBlocks(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")

	e1 := config.NewTree()
	e1.Set("location", "file:///local")
	e1.Set("trigger", "commit")
	sys.AddListEntry("archive", "local", e1)

	e2 := config.NewTree()
	e2.Set("location", "https://server/archive")
	e2.Set("trigger", "daily")
	e2.Set("on-change", "true")
	sys.AddListEntry("archive", "offsite", e2)

	configs := archive.ExtractConfigs(tree)
	require.Len(t, configs, 2)

	assert.Equal(t, "local", configs[0].Name)
	assert.Equal(t, "file:///local", configs[0].Location)
	assert.Equal(t, "commit", configs[0].Trigger)

	assert.Equal(t, "offsite", configs[1].Name)
	assert.Equal(t, "https://server/archive", configs[1].Location)
	assert.Equal(t, "daily", configs[1].Trigger)
	assert.True(t, configs[1].OnChange)
}

// TestExtractConfigs_NoSystem verifies nil return when no system block exists.
//
// VALIDATES: Missing system block returns nil.
// PREVENTS: Panic on configs without system block.
func TestExtractConfigs_NoSystem(t *testing.T) {
	tree := config.NewTree()
	configs := archive.ExtractConfigs(tree)
	assert.Nil(t, configs)
}

// --- FilterByTrigger tests ---

// TestCommitTriggerFilter verifies only commit-triggered blocks are selected.
//
// VALIDATES: FilterByTrigger("commit") returns only commit blocks.
// PREVENTS: Manual/daily/hourly blocks firing on editor commit.
func TestCommitTriggerFilter(t *testing.T) {
	configs := []archive.ArchiveConfig{
		{Name: "a", Trigger: "commit"},
		{Name: "b", Trigger: "manual"},
		{Name: "c", Trigger: "daily"},
		{Name: "d", Trigger: "commit"},
	}

	filtered := archive.FilterByTrigger(configs, "commit")
	require.Len(t, filtered, 2)
	assert.Equal(t, "a", filtered[0].Name)
	assert.Equal(t, "d", filtered[1].Name)
}

// --- ChangeTracker tests ---

// TestChangeTracker verifies hash-based change detection for unchanged content.
//
// VALIDATES: Same content reports no change on second call.
// PREVENTS: Unnecessary archives when config hasn't changed.
func TestChangeTracker(t *testing.T) {
	ct := archive.NewChangeTracker()
	content := []byte("bgp { local-as 65000; }")

	// First call: always changed (boot)
	assert.True(t, ct.HasChanged("test", content))
	// Second call: same content, not changed
	assert.False(t, ct.HasChanged("test", content))
}

// TestChangeTracker_Changed verifies different content is detected.
//
// VALIDATES: Different content is detected as changed.
// PREVENTS: Stale hash causing missed archives after config edit.
func TestChangeTracker_Changed(t *testing.T) {
	ct := archive.NewChangeTracker()

	ct.HasChanged("test", []byte("version 1"))
	assert.True(t, ct.HasChanged("test", []byte("version 2")))
}

// TestChangeTracker_Boot verifies first check always reports changed.
//
// VALIDATES: First HasChanged call for a name always returns true.
// PREVENTS: Boot archive being skipped due to empty baseline.
func TestChangeTracker_Boot(t *testing.T) {
	ct := archive.NewChangeTracker()
	assert.True(t, ct.HasChanged("new-archive", []byte("any content")))
}

// TestChangeTracker_IndependentNames verifies per-name tracking.
//
// VALIDATES: Different archive names have independent change tracking.
// PREVENTS: One archive's hash affecting another's change detection.
func TestChangeTracker_IndependentNames(t *testing.T) {
	ct := archive.NewChangeTracker()
	content := []byte("same content")

	ct.HasChanged("a", content)
	ct.HasChanged("b", content)

	// Both should report not changed for same content
	assert.False(t, ct.HasChanged("a", content))
	assert.False(t, ct.HasChanged("b", content))

	// New name should report changed
	assert.True(t, ct.HasChanged("c", content))
}
