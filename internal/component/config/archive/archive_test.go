package archive_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/archive"
	"github.com/ze-software/ze/internal/component/config/system"
)

// --- FormatFilename tests ---

// TestFormatFilename verifies token substitution in filename format.
//
// VALIDATES: Token substitution produces correct filename with .conf extension.
// PREVENTS: Malformed archive filenames breaking retrieval.
func TestFormatFilename(t *testing.T) {
	sys := system.SystemConfig{Host: "router1", Domain: "dc1.example.com"}
	ts := time.Date(2026, 3, 9, 14, 30, 45, 0, time.UTC)

	name := archive.FormatFilename("{name}-{host}-{date}-{time}", "myconfig.conf", &sys, "backup", ts)
	assert.Equal(t, "myconfig-router1-20260309-143045.conf", name)
}

// TestFormatFilename_Default verifies default format when none specified.
//
// VALIDATES: Empty format string uses default "{name}-{host}-{date}-{time}".
// PREVENTS: Empty filename when format is omitted from config.
func TestFormatFilename_Default(t *testing.T) {
	sys := system.SystemConfig{Host: "host1"}
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	name := archive.FormatFilename("", "config.conf", &sys, "test", ts)
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
		"test.conf", &sys, "offsite", ts,
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

	name := archive.FormatFilename("{name}-{host}", "config", &sys, "test", ts)
	assert.Equal(t, "config-host.conf", name)
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

	notifier := archive.NewNotifier("test.conf", configs, &sys, nil)
	errs := notifier([]byte("config content"))
	assert.Empty(t, errs)

	entries, readErr := os.ReadDir(destDir)
	require.NoError(t, readErr)
	assert.Len(t, entries, 1)
	assert.Contains(t, entries[0].Name(), "test-myhost-")
}

// TestNewNotifier_EventEmission verifies event emitter is called on successful archive.
//
// VALIDATES: AC-12 -- archive event emitted for plugin subscribers.
// PREVENTS: Event emission being silently skipped.
func TestNewNotifier_EventEmission(t *testing.T) {
	destDir := t.TempDir()
	sys := system.SystemConfig{Host: "evhost"}
	configs := []archive.ArchiveConfig{
		{
			Name:     "ev-test",
			Location: "file://" + destDir,
			Filename: "{archive}",
			Timeout:  5 * time.Second,
			Trigger:  archive.TriggerCommit,
		},
	}

	var emittedName, emittedFilename string
	var emittedContent []byte
	eventFn := func(name, filename string, content []byte) {
		emittedName = name
		emittedFilename = filename
		emittedContent = content
	}

	notifier := archive.NewNotifier("test.conf", configs, &sys, eventFn)
	errs := notifier([]byte("config data"))
	assert.Empty(t, errs)
	assert.Equal(t, "ev-test", emittedName)
	assert.Equal(t, "ev-test.conf", emittedFilename)
	assert.Equal(t, []byte("config data"), emittedContent)
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

// --- ArchiveMatcher tests ---

// TestArchiveMatcher checks which names the matcher accepts for one block.
//
// VALIDATES: a name matches only when it is the whole format with {date} as 8
// digits, {time} as 6 digits and every other token as its literal value.
// PREVENTS: pruning a file that only shares a prefix, a suffix or a shape with
// this block's archive copies.
func TestArchiveMatcher(t *testing.T) {
	sys := system.SystemConfig{Host: "router1", Domain: "example.net"}
	tests := []struct {
		format string
		file   string
		want   bool
	}{
		{"{name}-{host}-{date}-{time}", "ze-router1-20261007-120001.conf", true},
		{"{name}-{host}-{date}-{time}", "ze-router1-2026107-120001.conf", false},
		{"{name}-{host}-{date}-{time}", "ze-router1-20261007-1200011.conf", false},
		{"{name}-{host}-{date}-{time}", "ze-router1-2026100a-120001.conf", false},
		{"{name}-{host}-{date}-{time}", "ze-router1-20261007-120001.conf.bak", false},
		{"{name}-{host}-{date}-{time}", "old-ze-router1-20261007-120001.conf", false},
		{"{name}-{host}-{date}-{time}", "ze-router2-20261007-120001.conf", false},
		{"{date}-{name}", "20261007-ze.conf", true},
		{"{date}-{name}", "foo.conf", false},
		{"{date}-{name}", "20261007-other.conf", false},
		{"{host}.{domain}-{date}", "router1.example.net-20261007.conf", true},
		{"{host}.{domain}-{date}", "router1Xexample.net-20261007.conf", false},
		{"{time}-{archive}", "120001-backup.conf", true},
		{"{time}-{archive}", "120001-offsite.conf", false},
		{"", "ze-router1-20261007-120001.conf", true},
	}
	for _, tt := range tests {
		matcher := matcherFor(t, tt.format, &sys, "backup")
		assert.Equal(t, tt.want, matcher.MatchString(tt.file), "format %q file %q", tt.format, tt.file)
	}
}

// TestArchiveMatcher_NoTimeTokens checks a format that names one fixed file.
//
// VALIDATES: the matcher accepts exactly that file, so pruning never removes
// it and never touches anything else in the directory.
// PREVENTS: a format without {date} or {time} widening to other files.
func TestArchiveMatcher_NoTimeTokens(t *testing.T) {
	sys := system.SystemConfig{Host: "r1"}
	matcher := matcherFor(t, "{name}-{host}", &sys, "backup")
	assert.True(t, matcher.MatchString("ze-r1.conf"))
	assert.False(t, matcher.MatchString("ze-r1.conf.conf"))
	assert.False(t, matcher.MatchString("ze-r1-20261007.conf"))

	dir := t.TempDir()
	for _, name := range []string{"ze-r1.conf", "foo.conf"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600))
	}
	require.NoError(t, archive.PruneFileArchives("file://"+dir, 1, matcher))
	assert.ElementsMatch(t, []string{"ze-r1.conf", "foo.conf"}, dirNames(t, dir))
}

// TestArchiveMatcher_PrunesRealFilenames prunes files FormatFilename names at
// real timestamps, beside foreign .conf files that are older than all of them.
//
// VALIDATES: commit-revisions removes the oldest archive copy for the default
// format and for {date} or {time} at the start, middle and end of the format.
// PREVENTS: a format starting with a time token matching every .conf file in
// the directory, and a matcher that matches no real archive name.
func TestArchiveMatcher_PrunesRealFilenames(t *testing.T) {
	sys := system.SystemConfig{Host: "r1", Domain: "example.net"}
	tests := []struct {
		name   string
		format string
	}{
		{"default format", ""},
		{"default format spelled out", archive.DefaultFilenameFormat},
		{"date then time", "{archive}-{date}-{time}"},
		{"time before date", "{name}-{time}-{date}"},
		{"date only, last", "{host}.{domain}-{date}"},
		{"time only, last", "{name}{time}"},
		{"date first", "{date}-{time}-{name}"},
		{"time first", "{time}-{archive}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			foreign := writeForeignConfs(t, dir)
			written := writeArchiveCopies(t, dir, tt.format, &sys, "backup")

			require.NoError(t, archive.PruneFileArchives("file://"+dir, 2, matcherFor(t, tt.format, &sys, "backup")))

			want := append([]string{written[1], written[2]}, foreign...)
			assert.ElementsMatch(t, want, dirNames(t, dir))
		})
	}
}

// TestArchiveMatcher_DateFirstSparesOtherFiles prunes a format that starts
// with {date} in a directory holding an operator file and another block's copies.
//
// VALIDATES: only this block's copies are counted and removed.
// PREVENTS: an empty filename prefix that counts and deletes every .conf file.
func TestArchiveMatcher_DateFirstSparesOtherFiles(t *testing.T) {
	sys := system.SystemConfig{Host: "r1"}
	dir := t.TempDir()
	foreign := writeForeignConfs(t, dir)
	other := writeArchiveCopies(t, dir, "{date}-{archive}", &sys, "offsite")
	own := writeArchiveCopies(t, dir, "{date}-{name}", &sys, "backup")

	require.NoError(t, archive.PruneFileArchives("file://"+dir, 1, matcherFor(t, "{date}-{name}", &sys, "backup")))

	want := append(append([]string{own[2]}, other...), foreign...)
	assert.ElementsMatch(t, want, dirNames(t, dir))
}

// TestArchiveMatcher_SharedDirectory prunes two blocks that write to one
// directory with one format that holds {archive}.
//
// VALIDATES: each block counts and prunes only its own copies.
// PREVENTS: one block's pruning removing the other block's archives.
func TestArchiveMatcher_SharedDirectory(t *testing.T) {
	sys := system.SystemConfig{Host: "r1"}
	format := "{date}-{time}-{archive}"
	dir := t.TempDir()
	local := writeArchiveCopies(t, dir, format, &sys, "local")
	offsite := writeArchiveCopies(t, dir, format, &sys, "offsite")

	require.NoError(t, archive.PruneFileArchives("file://"+dir, 2, matcherFor(t, format, &sys, "local")))
	assert.ElementsMatch(t, append([]string{local[1], local[2]}, offsite...), dirNames(t, dir))

	require.NoError(t, archive.PruneFileArchives("file://"+dir, 2, matcherFor(t, format, &sys, "offsite")))
	assert.ElementsMatch(t, []string{local[1], local[2], offsite[1], offsite[2]}, dirNames(t, dir))
}

// archiveStamps are three real write times, oldest first, crossing midnight.
var archiveStamps = []time.Time{
	time.Date(2026, 10, 5, 23, 59, 58, 0, time.UTC),
	time.Date(2026, 10, 6, 9, 15, 0, 0, time.UTC),
	time.Date(2026, 10, 7, 12, 0, 1, 0, time.UTC),
}

// writeArchiveCopies writes one archive copy per archiveStamps entry, named by
// FormatFilename and dated by its stamp, and returns the names oldest first.
func writeArchiveCopies(t *testing.T, dir, format string, sys *system.SystemConfig, archiveName string) []string {
	t.Helper()
	var written []string
	for _, ts := range archiveStamps {
		name := archive.FormatFilename(format, "ze.conf", sys, archiveName, ts)
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte(name), 0o600))
		require.NoError(t, os.Chtimes(path, ts, ts))
		written = append(written, name)
	}
	return written
}

// writeForeignConfs writes .conf files no archive block wrote, older than every
// archive copy, so a matcher that admits them would prune them first.
func writeForeignConfs(t *testing.T, dir string) []string {
	t.Helper()
	names := []string{"foo.conf", "other-backup.conf", "20260101.conf"}
	old := archiveStamps[0].Add(-time.Hour)
	for _, name := range names {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))
		require.NoError(t, os.Chtimes(path, old, old))
	}
	return names
}

// dirNames lists the entry names in dir.
func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// matcherFor builds the matcher of one archive block for ze.conf.
func matcherFor(t *testing.T, format string, sys *system.SystemConfig, archiveName string) *regexp.Regexp {
	t.Helper()
	matcher, err := archive.ArchiveMatcher(format, "ze.conf", sys, archiveName)
	require.NoError(t, err)
	return matcher
}

// TestArchiveMatcher_InvalidUTF8 builds a matcher from a host name that is not
// valid UTF-8, directly and through a notifier commit with commit-revisions set.
//
// VALIDATES: the matcher reports an error, and the notifier returns it as the
// block's error after writing the copy, without pruning.
// PREVENTS: a panic in the daemon's archive path from regexp.MustCompile, which
// refuses a pattern that is not valid UTF-8.
func TestArchiveMatcher_InvalidUTF8(t *testing.T) {
	sys := system.SystemConfig{Host: "r\xff1", CommitRevisions: 1}
	_, err := archive.ArchiveMatcher(archive.DefaultFilenameFormat, "ze.conf", &sys, "backup")
	require.Error(t, err)

	dir := t.TempDir()
	foreign := writeForeignConfs(t, dir)
	configs := []archive.ArchiveConfig{{Name: "backup", Location: "file://" + dir, Filename: "{date}-{host}", Trigger: archive.TriggerCommit}}
	errs := archive.NewNotifier("ze.conf", configs, &sys, nil)([]byte("config"))
	require.Len(t, errs, 1)
	assert.ErrorContains(t, errs[0], "archive backup: archive filename matcher")
	assert.Len(t, dirNames(t, dir), len(foreign)+1)
}

// TestNewNotifier_Prunes commits once through a notifier with commit-revisions
// 2 and a format that starts with {date}, beside two older copies.
//
// VALIDATES: the editor-commit path prunes to the cap and leaves the .conf
// files no archive block wrote.
// PREVENTS: pruning wired on the scheduler paths but not on the notifier.
func TestNewNotifier_Prunes(t *testing.T) {
	sys := system.SystemConfig{Host: "r1", CommitRevisions: 2}
	dir := t.TempDir()
	foreign := writeForeignConfs(t, dir)
	copies := writeArchiveCopies(t, dir, "{date}-{time}-{host}", &sys, "backup")
	configs := []archive.ArchiveConfig{{Name: "backup", Location: "file://" + dir, Filename: "{date}-{time}-{host}", Trigger: archive.TriggerCommit}}

	errs := archive.NewNotifier("ze.conf", configs, &sys, nil)([]byte("config"))
	require.Empty(t, errs)

	names := dirNames(t, dir)
	assert.Len(t, names, len(foreign)+2)
	assert.Subset(t, names, append([]string{copies[2]}, foreign...))
	assert.NotContains(t, names, copies[0])
	assert.NotContains(t, names, copies[1])
}

// --- PruneFileArchives tests ---

// TestPruneFileArchives_EqualTimesByName prunes copies restored in batches of
// five, as a copy without preserved times leaves them: each batch shares one
// modification time, and the batches holding the newest names were restored
// first.
//
// VALIDATES: within one modification time the copies go by name, so with
// {date} before {time} the chronologically older copy goes first.
// PREVENTS: the order sort.Slice leaves equal elements in deciding which copy
// is deleted; on this input it puts the newest name of the oldest batch first.
func TestPruneFileArchives_EqualTimesByName(t *testing.T) {
	sys := system.SystemConfig{Host: "r1"}
	dir := t.TempDir()
	base := time.Date(2026, 10, 7, 13, 0, 0, 0, time.UTC)
	var names []string
	for i := range 40 { // past the 12 entries below which sort.Slice is stable
		name := archive.FormatFilename(archive.DefaultFilenameFormat, "ze.conf", &sys, "backup", base.Add(time.Duration(i)*time.Hour))
		restored := base.Add(time.Duration((39-i)/5) * time.Minute)
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte(name), 0o600))
		require.NoError(t, os.Chtimes(path, restored, restored))
		names = append(names, name)
	}

	require.NoError(t, archive.PruneFileArchives("file://"+dir, 38, r1Matcher(t)))
	assert.ElementsMatch(t, append(append([]string{}, names[:35]...), names[37:]...), dirNames(t, dir))
}

// TestPruneFileArchives_ReportsFailures prunes a directory that does not exist
// and, when the test does not run as root (root removes entries from a
// read-only directory), a directory whose entries cannot be removed.
//
// VALIDATES: both failures come back as an error, and nothing is removed.
// PREVENTS: commit-revisions silently keeping more files than the cap.
func TestPruneFileArchives_ReportsFailures(t *testing.T) {
	require.Error(t, archive.PruneFileArchives("file://"+filepath.Join(t.TempDir(), "absent"), 1, r1Matcher(t)))

	if os.Geteuid() != 0 {
		sys := system.SystemConfig{Host: "r1"}
		dir := t.TempDir()
		copies := writeArchiveCopies(t, dir, archive.DefaultFilenameFormat, &sys, "backup")
		require.NoError(t, os.Chmod(dir, 0o500))
		t.Cleanup(func() { // so TempDir can remove it
			if err := os.Chmod(dir, 0o700); err != nil {
				t.Log(err)
			}
		})

		require.Error(t, archive.PruneFileArchives("file://"+dir, 1, r1Matcher(t)))
		assert.ElementsMatch(t, copies, dirNames(t, dir))
	}
}

// r1Matcher matches the default-format archive copies of ze.conf on host r1.
func r1Matcher(t *testing.T) *regexp.Regexp {
	t.Helper()
	return matcherFor(t, archive.DefaultFilenameFormat, &system.SystemConfig{Host: "r1"}, "backup")
}

func TestPruneFileArchives_KeepsNewest(t *testing.T) {
	dir := t.TempDir()
	location := "file://" + dir

	for i, name := range []string{
		"ze-r1-20260101-000000.conf",
		"ze-r1-20260102-000000.conf",
		"ze-r1-20260103-000000.conf",
		"ze-r1-20260104-000000.conf",
	} {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte("v"+string(rune('1'+i))), 0o600))
		ts := time.Date(2026, 1, 1+i, 0, 0, 0, 0, time.UTC)
		require.NoError(t, os.Chtimes(path, ts, ts))
	}

	require.NoError(t, archive.PruneFileArchives(location, 2, r1Matcher(t)))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 2)
	assert.Equal(t, "ze-r1-20260103-000000.conf", entries[0].Name())
	assert.Equal(t, "ze-r1-20260104-000000.conf", entries[1].Name())
}

func TestPruneFileArchives_IgnoresNonMatchingFiles(t *testing.T) {
	dir := t.TempDir()
	location := "file://" + dir

	require.NoError(t, os.WriteFile(filepath.Join(dir, "ze-r1-20260101-000000.conf"), []byte("a"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "other-backup.conf"), []byte("b"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ze-r1-20260102-000000.conf"), []byte("c"), 0o600))

	ts1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ts2 := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	require.NoError(t, os.Chtimes(filepath.Join(dir, "ze-r1-20260101-000000.conf"), ts1, ts1))
	require.NoError(t, os.Chtimes(filepath.Join(dir, "ze-r1-20260102-000000.conf"), ts2, ts2))

	require.NoError(t, archive.PruneFileArchives(location, 1, r1Matcher(t)))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 2)
	names := []string{entries[0].Name(), entries[1].Name()}
	assert.Contains(t, names, "other-backup.conf")
	assert.Contains(t, names, "ze-r1-20260102-000000.conf")
}

func TestPruneFileArchives_UnderLimit(t *testing.T) {
	dir := t.TempDir()
	location := "file://" + dir

	require.NoError(t, os.WriteFile(filepath.Join(dir, "ze-r1-20260101-000000.conf"), []byte("a"), 0o600))

	require.NoError(t, archive.PruneFileArchives(location, 5, r1Matcher(t)))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestPruneFileArchives_NonFileScheme(t *testing.T) {
	require.NoError(t, archive.PruneFileArchives("https://example.com/archive", 1, r1Matcher(t)))
}

func TestPruneFileArchives_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, archive.PruneFileArchives("file://"+dir, 1, r1Matcher(t)))
}

// --- RedactURL tests ---

// TestRedactURL_WithCredentials verifies password is replaced with xxxxx.
//
// VALIDATES: AC-1 -- URLs with embedded credentials are sanitized.
// PREVENTS: Credential leak in error/log output.
func TestRedactURL_WithCredentials(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"https with userinfo", "https://admin:s3cret@host.example.com/archive", "https://admin:xxxxx@host.example.com/archive"},
		{"http with userinfo", "http://user:pass@10.0.0.1:8080/path", "http://user:xxxxx@10.0.0.1:8080/path"},
		{"user only no password", "https://user@host.example.com/path", "https://user@host.example.com/path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := archive.RedactURL(tt.url)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestRedactURL_WithoutCredentials verifies URLs without credentials pass through unchanged.
//
// VALIDATES: AC-2 -- URLs without credentials are not modified.
// PREVENTS: Unnecessary URL mutation breaking error message clarity.
func TestRedactURL_WithoutCredentials(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"https no auth", "https://host.example.com/archive"},
		{"file scheme", "file:///backups/configs"},
		{"http with port", "http://10.0.0.1:8080/path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := archive.RedactURL(tt.url)
			assert.Equal(t, tt.url, got)
		})
	}
}

// TestRedactURL_InvalidURL verifies fallback to raw string on parse failure.
//
// VALIDATES: RedactURL does not panic on unparseable input.
// PREVENTS: Panic or empty string from malformed URL.
func TestRedactURL_InvalidURL(t *testing.T) {
	raw := "://bad"
	got := archive.RedactURL(raw)
	assert.Equal(t, raw, got)
}

// TestValidateLocation_RedactsCredentials verifies error messages do not leak passwords.
//
// VALIDATES: AC-3 -- ValidateLocation error output sanitizes credentials.
// PREVENTS: Password appearing in validation error message.
func TestValidateLocation_RedactsCredentials(t *testing.T) {
	err := archive.ValidateLocation("ftp://admin:s3cret@host.example.com/path")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "s3cret")
}

// TestValidateLocation_NoSchemeRedactsCredentials verifies missing-scheme error sanitizes credentials.
//
// VALIDATES: AC-3 -- missing-scheme error path also sanitizes.
// PREVENTS: Password leak in "missing URL scheme" error message.
func TestValidateLocation_NoSchemeRedactsCredentials(t *testing.T) {
	err := archive.ValidateLocation("//admin:s3cret@host/path")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "s3cret")
}

// TestToHTTP_RedactsCredentials verifies HTTP error messages do not leak passwords.
//
// VALIDATES: AC-1 -- HTTP upload error output sanitizes credentials.
// PREVENTS: Password appearing in HTTP upload error message.
func TestToHTTP_RedactsCredentials(t *testing.T) {
	err := archive.ToHTTP([]byte("data"), "https://admin:s3cret@192.0.2.1:9999/archive", "test.conf", 100*time.Millisecond)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "s3cret")
	assert.Contains(t, err.Error(), "xxxxx")
}
