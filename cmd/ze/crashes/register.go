// Design: plan/spec-diag-crash-capture.md -- offline crash file CLI

package crashes

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("crashes", cmdregistry.Meta{
		Description: "Show crash reports (works offline, no daemon required)",
		Mode:        "offline",
		Section:     cmdregistry.SectionSystem,
		Subs:        "show [latest]",
	})
	cmdregistry.MustRegisterLocal("crashes show", RunShow)
	cmdregistry.MustRegisterLocal("crashes", RunHint)
}
