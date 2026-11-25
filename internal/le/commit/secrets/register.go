// Design: docs/architecture/core-design.md -- le's composition, one import per tool
// Related: secrets.go -- the change-set check registered here
// Related: actions.go -- the command verbs registered here
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.
package commitsecrets

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/commit/changecheck"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupGate, Answer, registry.Meta{
		Description: "Refuse secrets, private keys, credential files, live secret-named environment values, and local home-directory paths added by a commit or uncommitted working-tree change.",
		ShortHelp:   "refuse secrets and local home paths in commits",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterActions(area, Actions)
	leroot.RegisterShape(area, command.ShapeDoc)

	// `./le commit create` judges newly added secrets, keys, and home paths.
	changecheck.Register(changecheck.Check{Name: area + " check", Judge: judgeChange})
}
