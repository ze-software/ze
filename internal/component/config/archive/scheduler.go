// Design: docs/guide/config-archive.md -- config archive: the daily and hourly triggers and the boot archive
// Related: archive.go -- archiveToLocation and pruneAfterWrite, the write and the prune each run performs

package archive

import (
	"context"
	"log/slog"
	"os"
	"time"

	iconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

const (
	tickDaily  = 24 * time.Hour
	tickHourly = time.Hour
)

// runBoot names the run a failure happened in when it is the archive written
// once at daemon boot. A daily or hourly tick names its trigger keyword.
const runBoot = "boot"

// reportSourceArchive is the report-bus source of a scheduled archive's
// failure, so `ze show errors` lists it under the component that wrote it.
const reportSourceArchive = "archive"

// subjectConfig is the report-bus subject of a run that could not read the
// config: no archive block was reached, so none can be named.
const subjectConfig = "config"

// The reasons a scheduled failure gives the operator, one per step that can
// fail. Each run tries every step again, so none of them is permanent.
const (
	whyReadFailed  = "the daily and hourly archives copy the config file, so a file that cannot be read or parsed leaves nothing to copy; the next run tries again"
	whyWriteFailed = "this run left no copy of the config at the archive location; the next run tries again"
	whyPruneFailed = "the new copy is stored, but the copies past commit-revisions stay at the archive location, which keeps growing until the directory or file the error names can be read and removed"
)

// Scheduler runs time-based archive triggers (daily/hourly).
// It fires all matching archives on boot, then on interval.
// The owner MUST cancel its context and join Run before releasing readConfig's
// borrowed resources; cancellation does not interrupt an in-flight read.
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

// scheduledFailure is one step of a scheduled run that failed: what the
// operator reads, the archive block or config it concerns, why it matters and
// the command that fixes it.
type scheduledFailure struct {
	problem string // what failed, in the operator's terms: the log message
	subject string // the archive block's name, or subjectConfig
	run     string // runBoot, TriggerDaily or TriggerHourly
	why     string // why it matters, one of the why* constants
	fix     string // the exact command that fixes it
}

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
// then runs on interval. Cancellation prevents further archives; Run returns
// after any in-flight read or archive finishes. The owner MUST join that return
// before closing the resources readConfig borrows.
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
		reportScheduled(s.readFailure(runBoot), err)
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
		if archErr := archiveToLocation(content, ac.Name, ac.Location, filename, ac.Timeout); archErr != nil {
			reportScheduled(scheduledFailure{
				problem: "archive scheduler: the boot archive could not be written",
				subject: ac.Name, run: runBoot, why: whyWriteFailed, fix: archiveFix(ac.Name),
			}, archErr)
			continue
		}

		if s.eventFn != nil {
			s.eventFn(ac.Name, filename, content)
		}
		if pruneErr := pruneAfterWrite(ac, s.configPath, &sys); pruneErr != nil {
			reportScheduled(scheduledFailure{
				problem: "archive scheduler: the boot archive was written, but older copies could not be removed",
				subject: ac.Name, run: runBoot, why: whyPruneFailed, fix: archiveFix(ac.Name),
			}, pruneErr)
		}
	}
}

func (s *Scheduler) fireByTrigger(ctx context.Context, trigger string) {
	content, tree, err := s.readConfig()
	if err != nil {
		reportScheduled(s.readFailure(trigger), err)
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
		if archErr := archiveToLocation(content, ac.Name, ac.Location, filename, ac.Timeout); archErr != nil {
			reportScheduled(scheduledFailure{
				problem: "archive scheduler: the scheduled archive could not be written",
				subject: ac.Name, run: trigger, why: whyWriteFailed, fix: archiveFix(ac.Name),
			}, archErr)
			continue
		}

		if s.eventFn != nil {
			s.eventFn(ac.Name, filename, content)
		}
		if pruneErr := pruneAfterWrite(ac, s.configPath, &sys); pruneErr != nil {
			reportScheduled(scheduledFailure{
				problem: "archive scheduler: the scheduled archive was written, but older copies could not be removed",
				subject: ac.Name, run: trigger, why: whyPruneFailed, fix: archiveFix(ac.Name),
			}, pruneErr)
		}
	}
}

// readFailure is the failure of a run whose config could not be read. Its fix
// validates the file the scheduler reads, which names what is wrong with it.
func (s *Scheduler) readFailure(run string) scheduledFailure {
	return scheduledFailure{
		problem: "archive scheduler: the config could not be read, so no scheduled archive was written",
		subject: subjectConfig,
		run:     run,
		why:     whyReadFailed,
		fix:     "ze config validate " + s.configPath,
	}
}

// reportScheduled logs f at Warn and raises it as an error event on the
// report bus, both under CodeDoctorArchiveScheduledFailed. A scheduled run
// has no caller to answer, so the log and the bus are the only places the
// operator meets the failure; the bus keeps it for `ze show errors` and the
// recent-events row of `show doctor` once the log has rotated. register.go
// declares the code as an event code. Safe for concurrent use.
func reportScheduled(f scheduledFailure, err error) {
	slog.Warn(f.problem, "subject", f.subject, "run", f.run, "error", err,
		"why", f.why, "fix", f.fix, "code", diagnostic.CodeDoctorArchiveScheduledFailed)
	report.RaiseErrorFix(reportSourceArchive, diagnostic.CodeDoctorArchiveScheduledFailed, f.subject,
		report.FitMessage(f.problem+": "+err.Error()), f.fix, map[string]any{"run": f.run})
}

// archiveFix is the command that writes archive block name at once, outside
// the schedule: it runs the same write and prune, and prints the error while
// its cause is not yet corrected.
func archiveFix(name string) string {
	return "ze config archive " + name
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
