// Design: docs/architecture/hub-architecture.md -- hub CLI entry point
// Related: main.go -- run, whose first statement is this gate
//
// startup_gate.go answers one question the daemon asks before it does anything
// irreversible: did a plugin record a setup failure the daemon cannot run
// without?
//
// The answer is a replay of what each plugin's init() recorded, so it costs a
// map read and no probe. It is reached from run alone, which is why a CLI verb
// is never refused by it.

package hub

import (
	"errors"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// hardSetupFailure returns the error that names every plugin which recorded a
// hard setup failure, or nil when none did.
//
// It computes; it does not print and it does not exit. The caller applies the
// refusal in the idiom every other startup stage uses, so one reader sees one
// shape.
//
// Every failing plugin is named, not only the first. An operator who repairs
// one fault and restarts to meet the next one pays a whole boot for each fault
// after the first.
func hardSetupFailure() error {
	failures := registry.HardSetupFailures()
	if len(failures) == 0 {
		return nil
	}

	var text textbuf.Buffer
	for index, failure := range failures {
		if index > 0 {
			text.Str("; ")
		}
		text.Str(failure.Plugin)
		if failure.Reason != "" {
			text.Str(": ").Str(failure.Reason)
		}
	}
	return errors.New(text.String())
}
