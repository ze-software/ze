// Design: plan/spec-diag-crash-capture.md -- offline crash file CLI

package crashes

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("crashes", cmdregistry.Meta{
		Description: "View saved crash reports from panics (offline)",
		Mode:        "offline",
		Section:     cmdregistry.SectionSystem,
		Subs:        "show [latest]",
	})
	cmdregistry.MustRegisterLocalMeta("crashes show", RunShow, cmdregistry.Meta{
		Description: "Show a crash report. Use 'latest' for the most recent, or pass a filename. Contains the goroutine stack trace at the time of panic.",
	})
	cmdregistry.MustRegisterLocalMeta("crashes", RunHint, cmdregistry.Meta{
		Description: "View saved crash reports from panics (offline)",
	})
}
