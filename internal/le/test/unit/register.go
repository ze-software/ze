// Design: docs/architecture/core-design.md -- le composition, one import per tool
// Overview: actions.go -- the action table this registration exposes
package testunit

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/hookruntime"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(Area, leroot.GroupSuite, Answer, registry.Meta{
		Description: "The five race-instrumented component-group Go test suites, the installer initrd behind its own tag, `pkg`: named packages tested or vetted under the feature tags and admitted, and `all`: the whole checkout under the race detector.",
		ShortHelp:   "run the race-instrumented unit test groups",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterActions(Area, Actions)

	// The raw-job refusal advises `pkg path <package>` only for a package
	// this action runs, judged by the action's own refusal (PackageRuns).
	hookruntime.RegisterPackageRuns(PackageRuns)

	// A listing and a sweep have different row shapes. Document rendering keeps
	// both structured answers available without claiming one common row set.
	leroot.RegisterShape(Area, command.ShapeDoc)

	// Every claim comes from the table that dispatches the action.

}
