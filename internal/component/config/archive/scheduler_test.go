package archive_test

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/archive"
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

// TestReadConfigFromPath_FileNotFound verifies error on missing file.
//
// VALIDATES: ReadConfigFromPath returns error for missing config.
// PREVENTS: Panic on missing config file.
func TestReadConfigFromPath_FileNotFound(t *testing.T) {
	readFn := archive.ReadConfigFromPath(filepath.Join(t.TempDir(), "missing.conf"))
	_, _, err := readFn()
	assert.Error(t, err)
}
