// Design: docs/architecture/chaos-web-dashboard.md -- run observations

package orchestrator

import (
	"errors"
	"os"

	"github.com/ze-software/ze/internal/chaos/inprocess"
	"github.com/ze-software/ze/internal/chaos/report"
	"github.com/ze-software/ze/internal/chaos/validation"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func observeInProcess(result *inprocess.RunResult, ep *validation.EventProcessor,
	properties *validation.PropertyEngine, path string, config report.JSONLogConfig,
) int {
	var log *report.JSONLog
	closeLog := func() error { return nil }
	if path != "" {
		file, err := cliio.Create(path)
		if err != nil {
			var tb textbuf.Buffer
			tb.Str("error: opening event log ").Str(path).Str(": ").Err(err).Byte('\n').StdErr() //nolint:errcheck // CLI diagnostic
			return 1
		}
		log = report.NewJSONLog(file, config)
		closeLog = func() error {
			return errors.Join(log.Close(), file.Close())
		}
	}
	for i := range result.Events {
		event := result.Events[i]
		if event.PlannedShutdown {
			continue
		}
		if !event.Time.Before(result.ObservationEnd) {
			continue
		}
		ep.Process(event)
		if properties != nil {
			properties.ProcessEvent(event)
		}
		if log != nil {
			log.ProcessEvent(event)
		}
	}
	if log != nil {
		log.EndObservation(result.ObservationEnd)
	}
	if err := closeLog(); err != nil {
		var tb textbuf.Buffer
		tb.Str("error: closing event log ").Str(path).Str(": ").Err(err).Byte('\n').StdErr() //nolint:errcheck // CLI diagnostic
		return 1
	}
	summary := report.Summary{
		Seed: config.Seed, PeerCount: config.Peers, Duration: result.ObservationEnd.Sub(config.Start),
	}
	summary.Observe(ep, properties, result.ObservationEnd)
	return summary.Write(os.Stderr)
}
