package archive_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/archive"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

func makeTestTree() *config.Tree {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	sys.Set("host", "testhost")
	return tree
}

func makeReadConfig(content []byte) func() ([]byte, *config.Tree, error) {
	return func() ([]byte, *config.Tree, error) {
		return content, makeTestTree(), nil
	}
}

// TestScheduler_BootFire verifies all time-based archives fire on boot.
//
// VALIDATES: AC-9 -- daemon boot with trigger daily fires immediately.
// PREVENTS: Boot archive being skipped.
func TestScheduler_BootFire(t *testing.T) {
	destDir := t.TempDir()
	configs := []archive.ArchiveConfig{
		{
			Name:     "daily-backup",
			Location: "file://" + destDir,
			Filename: "{archive}-boot",
			Timeout:  5 * time.Second,
			Trigger:  archive.TriggerDaily,
		},
	}

	content := []byte("bgp { local-as 65000; }")
	s := archive.NewScheduler(configs, "test.conf", makeReadConfig(content), nil)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	s.Run(ctx)

	entries, err := os.ReadDir(destDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Contains(t, entries[0].Name(), "daily-backup-boot")
}

// TestScheduler_FiltersNonTimeBased verifies manual and commit triggers are ignored.
//
// VALIDATES: Scheduler only handles daily/hourly triggers.
// PREVENTS: Manual/commit blocks firing on schedule.
func TestScheduler_FiltersNonTimeBased(t *testing.T) {
	destDir := t.TempDir()
	configs := []archive.ArchiveConfig{
		{Name: "manual", Location: "file://" + destDir, Trigger: archive.TriggerManual, Timeout: 5 * time.Second},
		{Name: "commit", Location: "file://" + destDir, Trigger: archive.TriggerCommit, Timeout: 5 * time.Second},
	}

	content := []byte("config data")
	s := archive.NewScheduler(configs, "test.conf", makeReadConfig(content), nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	s.Run(ctx)

	entries, err := os.ReadDir(destDir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// TestScheduler_OnChangeSkipsUnchanged verifies on-change skips when config unchanged.
//
// VALIDATES: AC-7 -- on-change set, config unchanged, archive skipped.
// PREVENTS: Unnecessary archives when config is stable.
func TestScheduler_OnChangeSkipsUnchanged(t *testing.T) {
	destDir := t.TempDir()
	configs := []archive.ArchiveConfig{
		{
			Name:     "hourly-check",
			Location: "file://" + destDir,
			Filename: "{archive}-{time}",
			Timeout:  5 * time.Second,
			Trigger:  archive.TriggerHourly,
			OnChange: true,
		},
	}

	content := []byte("static config")
	readConfig := makeReadConfig(content)

	s := archive.NewScheduler(configs, "test.conf", readConfig, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	s.Run(ctx)

	entries, err := os.ReadDir(destDir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "boot fire creates one file, interval skips because unchanged")
}

// TestScheduler_EventEmission verifies event emitter is called on archive.
//
// VALIDATES: AC-12 -- archive triggers emit config/archive event.
// PREVENTS: Plugin subscribers missing archive notifications.
func TestScheduler_EventEmission(t *testing.T) {
	destDir := t.TempDir()
	configs := []archive.ArchiveConfig{
		{
			Name:     "with-event",
			Location: "file://" + destDir,
			Filename: "{archive}",
			Timeout:  5 * time.Second,
			Trigger:  archive.TriggerDaily,
		},
	}

	var eventCount atomic.Int32
	eventFn := func(name, filename string, content []byte) {
		eventCount.Add(1)
	}

	content := []byte("config data")
	s := archive.NewScheduler(configs, "test.conf", makeReadConfig(content), eventFn)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	s.Run(ctx)

	assert.Equal(t, int32(1), eventCount.Load(), "event emitted once on boot")
}

// TestScheduler_EmptyConfigs verifies scheduler returns immediately with no configs.
//
// VALIDATES: No-op when no time-based archives are configured.
// PREVENTS: Goroutine leak or block when no configs match.
func TestScheduler_EmptyConfigs(t *testing.T) {
	s := archive.NewScheduler(nil, "test.conf", nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not return with empty configs")
	}
}

// warnCapture records every Warn and Error record logged through the default
// logger, each as its message under "msg" and its attributes by key. Safe for
// concurrent use.
type warnCapture struct {
	mu      sync.Mutex
	records []map[string]string
}

func (c *warnCapture) Enabled(context.Context, slog.Level) bool { return true }

func (c *warnCapture) Handle(_ context.Context, r slog.Record) error {
	if r.Level < slog.LevelWarn {
		return nil
	}
	attrs := map[string]string{"msg": r.Message}
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.String()
		return true
	})
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records = append(c.records, attrs)
	return nil
}

func (c *warnCapture) WithAttrs([]slog.Attr) slog.Handler { return c }

func (c *warnCapture) WithGroup(string) slog.Handler { return c }

func (c *warnCapture) logged() []map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]map[string]string(nil), c.records...)
}

// captureWarns makes the default logger a warnCapture and clears the report
// bus for the test, restoring both when it ends. The test MUST NOT run in
// parallel: the default logger and the bus are process-wide.
func captureWarns(t *testing.T) *warnCapture {
	t.Helper()
	capture := &warnCapture{}
	previous := slog.Default()
	slog.SetDefault(slog.New(capture))
	report.ResetForTest()
	t.Cleanup(func() {
		slog.SetDefault(previous)
		report.ResetForTest()
	})
	return capture
}

// runBootOnce runs a scheduler over configs until its boot archive is done:
// the timeout is far below the hourly tick, so only the boot run fires.
func runBootOnce(configs []archive.ArchiveConfig, readConfig func() ([]byte, *config.Tree, error)) {
	s := archive.NewScheduler(configs, "test.conf", readConfig, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	s.Run(ctx)
}

// requireScheduledFailure checks that the run logged one Warn and raised one
// error event, both naming the scheduled-failure doctor code, the subject and
// the fix, the Warn with a why, and that both report the problem the failed
// step owes.
func requireScheduledFailure(t *testing.T, capture *warnCapture, problem, subject, fix string) {
	t.Helper()
	records := capture.logged()
	require.Len(t, records, 1, "one failed step logs one Warn")
	record := records[0]
	assert.Equal(t, problem, record["msg"])
	assert.Equal(t, diagnostic.CodeDoctorArchiveScheduledFailed, record["code"],
		"the Warn names the doctor code ze show diagnostic describes")
	assert.NotEmpty(t, record["why"], "the Warn says why the failure matters")
	assert.Equal(t, fix, record["fix"], "the Warn names the command that fixes it")
	assert.Equal(t, subject, record["subject"])
	assert.NotEmpty(t, record["error"], "the Warn carries the error as evidence")

	events := report.Errors(0)
	require.Len(t, events, 1, "a scheduled failure is an event on the report bus too")
	assert.Equal(t, "archive", events[0].Source)
	assert.Equal(t, diagnostic.CodeDoctorArchiveScheduledFailed, events[0].Code)
	assert.Equal(t, subject, events[0].Subject)
	assert.Equal(t, fix, events[0].Fix)
	assert.Contains(t, events[0].Message, problem)
}

// TestSchedulerFailuresNameTheirDoctorCode proves every step of a scheduled
// archive that can fail is reported under a doctor code, with why it matters
// and the command that fixes it, in the log and on the report bus.
//
// VALIDATES: ai/rules/user-facing-errors.md -- a Warn names a doctor code a
// doctor check declares (the recent-events check, through the event code
// register.go registers), and a transient problem is raised on the bus.
// METHOD: a boot run of the scheduler over a config reader that fails, over a
// location whose scheme no uploader takes, and over a filename the prune
// matcher refuses (a host name that is not valid UTF-8, as
// TestArchiveMatcher_InvalidUTF8 builds), with the default logger and the
// report bus captured.
// PREVENTS: a scheduled archive failing with a log line no doctor code
// explains, which `ze show errors` and `show doctor` never list.
func TestSchedulerFailuresNameTheirDoctorCode(t *testing.T) {
	assert.Contains(t, diagnostic.EventCodes(), diagnostic.CodeDoctorArchiveScheduledFailed,
		"register.go declares the code for the recent-events check")

	t.Run("read", func(t *testing.T) {
		capture := captureWarns(t)
		configs := []archive.ArchiveConfig{{Name: "nightly", Location: "file://" + t.TempDir(), Timeout: 5 * time.Second, Trigger: archive.TriggerDaily}}
		runBootOnce(configs, func() ([]byte, *config.Tree, error) {
			return nil, nil, errors.New("injected read failure")
		})
		requireScheduledFailure(t, capture,
			"archive scheduler: the config could not be read, so no scheduled archive was written",
			"config", "ze config validate test.conf")
	})

	t.Run("write", func(t *testing.T) {
		capture := captureWarns(t)
		configs := []archive.ArchiveConfig{{Name: "offsite", Location: "ftp://archive.example/ze", Timeout: 5 * time.Second, Trigger: archive.TriggerHourly}}
		runBootOnce(configs, makeReadConfig([]byte("config data")))
		requireScheduledFailure(t, capture,
			"archive scheduler: the boot archive could not be written",
			"offsite", "ze config archive offsite")
	})

	t.Run("prune", func(t *testing.T) {
		capture := captureWarns(t)
		host := "r\xff1"
		configs := []archive.ArchiveConfig{{Name: "local", Location: "file://" + t.TempDir(), Filename: "{host}", Timeout: 5 * time.Second, Trigger: archive.TriggerDaily}}
		runBootOnce(configs, func() ([]byte, *config.Tree, error) {
			tree := config.NewTree()
			sys := tree.GetOrCreateContainer("system")
			sys.Set("host", host)
			sys.Set("commit-revisions", "1")
			return []byte("config data"), tree, nil
		})
		// A filesystem that refuses a name that is not valid UTF-8 (APFS) fails
		// the write before the prune runs, and the write's failure is then the
		// one owed; a probe file decides which outcome the host owes.
		problem := "archive scheduler: the boot archive was written, but older copies could not be removed"
		if os.WriteFile(filepath.Join(t.TempDir(), host), nil, 0o600) != nil {
			problem = "archive scheduler: the boot archive could not be written"
		}
		requireScheduledFailure(t, capture, problem, "local", "ze config archive local")
	})
}

// TestReadConfigFromPath_FileNotFound verifies error on missing file.
//
// VALIDATES: ReadConfigFromPath returns error for missing config.
// PREVENTS: Panic on missing config file.
func TestReadConfigFromPath_FileNotFound(t *testing.T) {
	readFn := archive.ReadConfigFromPath(filepath.Join(t.TempDir(), "missing.conf"))
	_, _, err := readFn()
	assert.Error(t, err)
}
