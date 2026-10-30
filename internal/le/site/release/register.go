// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package siterelease

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupGenerate, Answer, registry.Meta{
		Description: "The releases the website publishes: read them from the forge into website/data/github-releases.json, which the site build reads instead of reaching the network.",
		ShortHelp:   "fetch the published release data into the committed file",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command,
		// as internal/le/site/facts does.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, actions).
		SubsFunc: Subs,
	})
	leroot.RegisterActions(area, Actions)

	// The update answers one map, so the row operators act on it rather than
	// being refused, as for le site facts.
	leroot.RegisterShape(area, command.ShapeMap)
}
