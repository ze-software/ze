// Design: docs/architecture/config/syntax.md -- config archive scheduler

package archive

import (
	"context"
	"log/slog"
	"os"
	"time"

	iconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/system"
)

const (
	tickDaily  = 24 * time.Hour
	tickHourly = time.Hour
)

// Scheduler runs time-based archive triggers (daily/hourly).
// It fires all matching archives on boot, then on interval.
// Stopped by canceling the context.
type Scheduler struct {
	configs    []ArchiveConfig
	tracker    *ChangeTracker
	configPath string
	readConfig func() ([]byte, *iconfig.Tree, error)
	eventFn    EventEmitter
}

// EventEmitter is called after each archive to emit a config/archive event.
// Nil means no event emission.
type EventEmitter func(archiveName, filename string, content []byte)

// NewScheduler creates a scheduler for time-based archive configs.
// readConfig returns the current config content and parsed tree.
// eventFn is called after each successful archive (may be nil).
func NewScheduler(configs []ArchiveConfig, configPath string, readConfig func() ([]byte, *iconfig.Tree, error), eventFn EventEmitter) *Scheduler {
	timeBased := make([]ArchiveConfig, 0, len(configs))
	for _, ac := range configs {
		if ac.Trigger == TriggerDaily || ac.Trigger == TriggerHourly {
			timeBased = append(timeBased, ac)
		}
	}

	return &Scheduler{
		configs:    timeBased,
		tracker:    NewChangeTracker(),
		configPath: configPath,
		readConfig: readConfig,
		eventFn:    eventFn,
	}
}

// Run starts the scheduler. It fires all time-based archives immediately (boot),
// then runs on interval. Blocks until ctx is canceled.
func (s *Scheduler) Run(ctx context.Context) {
	if len(s.configs) == 0 {
		return
	}

	s.fireAll(ctx)

	dailyTicker := time.NewTicker(tickDaily)
	hourlyTicker := time.NewTicker(tickHourly)
	defer dailyTicker.Stop()
	defer hourlyTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-dailyTicker.C:
			s.fireByTrigger(ctx, TriggerDaily)
		case <-hourlyTicker.C:
			s.fireByTrigger(ctx, TriggerHourly)
		}
	}
}

func (s *Scheduler) fireAll(ctx context.Context) {
	content, tree, err := s.readConfig()
	if err != nil {
		slog.Warn("archive scheduler: read config", "error", err)
		return
	}

	sys := system.ExtractSystemConfig(tree)
	ts := time.Now()

	for _, ac := range s.configs {
		if ctx.Err() != nil {
			return
		}
		s.tracker.HasChanged(ac.Name, content)

		filename := FormatFilename(ac.Filename, s.configPath, &sys, ac.Name, ts)
		if archErr := archiveToLocation(content, ac.Location, filename, ac.Timeout); archErr != nil {
			slog.Warn("archive scheduler: boot archive failed", "name", ac.Name, "error", archErr)
			continue
		}

		if s.eventFn != nil {
			s.eventFn(ac.Name, filename, content)
		}
		if sys.CommitRevisions > 0 {
			prefix := ArchivePrefix(ac.Filename, s.configPath, &sys, ac.Name)
			PruneFileArchives(ac.Location, sys.CommitRevisions, prefix)
		}
	}
}

func (s *Scheduler) fireByTrigger(ctx context.Context, trigger string) {
	content, tree, err := s.readConfig()
	if err != nil {
		slog.Warn("archive scheduler: read config", "error", err)
		return
	}

	sys := system.ExtractSystemConfig(tree)
	ts := time.Now()

	for _, ac := range s.configs {
		if ctx.Err() != nil {
			return
		}
		if ac.Trigger != trigger {
			continue
		}
		if ac.OnChange && !s.tracker.HasChanged(ac.Name, content) {
			continue
		}
		if !ac.OnChange {
			s.tracker.HasChanged(ac.Name, content)
		}

		filename := FormatFilename(ac.Filename, s.configPath, &sys, ac.Name, ts)
		if archErr := archiveToLocation(content, ac.Location, filename, ac.Timeout); archErr != nil {
			slog.Warn("archive scheduler: archive failed", "name", ac.Name, "trigger", trigger, "error", archErr)
			continue
		}

		if s.eventFn != nil {
			s.eventFn(ac.Name, filename, content)
		}
		if sys.CommitRevisions > 0 {
			prefix := ArchivePrefix(ac.Filename, s.configPath, &sys, ac.Name)
			PruneFileArchives(ac.Location, sys.CommitRevisions, prefix)
		}
	}
}

// ReadConfigFromPath returns a function that reads and parses config from a file path.
// Suitable as the readConfig parameter for NewScheduler.
func ReadConfigFromPath(configPath string) func() ([]byte, *iconfig.Tree, error) {
	return func() ([]byte, *iconfig.Tree, error) {
		data, err := os.ReadFile(configPath) //nolint:gosec // Config path from daemon startup
		if err != nil {
			return nil, nil, err
		}

		schema, schErr := iconfig.YANGSchema()
		if schErr != nil {
			return nil, nil, schErr
		}

		parser := iconfig.NewParser(schema)
		tree, parseErr := parser.Parse(string(data))
		if parseErr != nil {
			return nil, nil, parseErr
		}

		return data, tree, nil
	}
}
