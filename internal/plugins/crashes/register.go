// Design: plan/learned/726-diag-crash-capture.md -- offline crash file CLI

// codegen:skip -- CLI command wired via cmd/ze/main.go, not a runtime plugin.

package crashes

import (
	"codeberg.org/thomas-mangin/ze/internal/component/command/registry"
)

func init() {
	registry.RegisterRoot("crashes", registry.Meta{
		Description: "View saved crash reports from panics (offline)",
		Mode:        "offline",
		Section:     registry.SectionSystem,
		Subs:        "show [latest]",
	})
	registry.MustRegisterLocalMeta("crashes show", RunShow, registry.Meta{
		Description: "Show a crash report. Use 'latest' for the most recent, or pass a filename. Contains the goroutine stack trace at the time of panic.",
	})
	registry.MustRegisterLocalMeta("crashes", RunHint, registry.Meta{
		Description: "View saved crash reports from panics (offline)",
	})
}
