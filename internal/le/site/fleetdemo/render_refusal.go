// Design: docs/contributing/gh-pages.md -- the fleet interface demonstration: a le without the fleet interface
// Related: render_absent.go -- render in a le built without the fleet interface, which answers this refusal

package sitefleetdemo

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

// fleetGate is one row of feature-gates.txt a le that records the
// demonstration is built with: the tag, and the package the tag gates.
type fleetGate struct {
	tag         string
	packagePath string
}

// fleetGates are the two rows the fleet pages need. They are a copy of rows
// of feature-gates.txt, kept so a refusal can name the line to restore when
// the manifest lost one; TestFleetDemoGatesAreRowsOfTheManifest holds them
// to the checkout's manifest.
var fleetGates = []fleetGate{
	{tag: "ze_web", packagePath: "internal/component/web"},
	{tag: "ze_fleet_ui", packagePath: "internal/component/web/fleetui"},
}

// absentProblem is the first line of every refusal of a le built without the
// fleet interface.
const absentProblem = "This le was built without the fleet interface, so it has no fleet pages to record"

// absentRefusal answers why a le of the checkout at repository was built
// without the fleet interface, and the command that gives it one. The
// launcher (./le) builds le with every tag feature-gates.txt declares, so the
// answer turns on the manifest: when it lacks a fleet row, the fix restores
// the row; when it declares both, this le was built by hand, and the fix is
// to run the action through the launcher.
func absentRefusal(repository string) *opererr.Error {
	manifest := filepath.Join(repository, "feature-gates.txt")
	declared, err := repofeaturetags.DaemonTags(repository)
	if err != nil {
		return opererr.Wrap(err, absentProblem,
			"The launcher builds le with every tag "+manifest+" declares, and that manifest cannot be read.",
			"git -C "+shellQuote(repository)+" status -- feature-gates.txt")
	}
	for _, gate := range fleetGates {
		if !slices.Contains(declared, gate.tag) {
			return opererr.New(absentProblem,
				"The launcher builds le with every tag "+manifest+" declares, and it declares no "+gate.tag+" row, which links "+gate.packagePath+". Restore the row, then run ./le site fleet-demo render again.",
				"printf '%s\\n' '"+gate.tag+"  "+gate.packagePath+"' >> "+shellQuote(manifest))
		}
	}
	return opererr.New(absentProblem,
		"The demonstration records the fleet package's own handlers, which the ze_web and ze_fleet_ui tags link. "+manifest+
			" declares both, and the launcher builds le with every tag it declares, so this le was built by hand without them.",
		"cd "+shellQuote(repository)+" && ./le site fleet-demo render")
}

// shellQuote answers value as one shell word, so a fix command stays one
// command when the checkout's path holds a space or a quote: the value in
// single quotes, each single quote of it closed, escaped and reopened.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
