package fleetui

import (
	"io/fs"
	"os"
	"testing"

	"github.com/ze-software/ze/internal/test/markupcheck"
	"github.com/ze-software/ze/internal/test/templcheck"
)

// The markup guards of the fleet package. Each guard walks the directory it
// is given, and the web package's guards name the web package, so the fleet
// package runs its own (docs/architecture/web-components.md, "Markup guards").

// The number of templ components each family of templates declares. The
// typed-data guard counts every function of every *_templ.go that answers a
// templ.Component, and the count is exact.
//
// WHEN YOU ADD OR REMOVE A COMPONENT, CHANGE THE CONSTANT OF YOUR OWN FAMILY,
// and add a constant of your own, summed below, for a new family. A count
// that is off by one fails TestFleetViewDataIsTyped with the number the
// guard read.
const (
	// shellComponentCount is shell.templ (15), tree.templ (6) and
	// search.templ (1).
	shellComponentCount = 22
	// sharedComponentCount is icons.templ and every component_*.templ.
	sharedComponentCount = 83
	// pageComponentCount is every page_*.templ: page_devices.templ (1),
	// page_dashboard.templ (13), page_device.templ (5), page_activity.templ
	// (5), page_config.templ (6), page_rollouts.templ (4) and
	// page_admin.templ (2).
	pageComponentCount = 36

	fleetComponentCount = shellComponentCount + sharedComponentCount + pageComponentCount
)

// fleetMarkupExempt lists the Go files allowed to build markup. None is.
var fleetMarkupExempt = map[string]string{}

// fleetMarkupFloors is what makes a vacuous walk visible: the walk must read
// at least this many string literals, and the exemption table holds exactly
// this many entries.
var fleetMarkupFloors = markupcheck.Floors{Literals: 20, Exempt: 0}

// fleetTemplFileFloor is the fewest .templ files the inline walk must read:
// the three of the shell. The package holds more.
const fleetTemplFileFloor = 3

// fleetAssetRefFloor is the fewest asset references the asset walk must
// resolve: the eight stylesheets and the seven scripts of the shell's head.
const fleetAssetRefFloor = 15

// webAssetsPath is the prefix the web package serves its embedded assets at,
// and webAssetsDir is the directory it embeds them from, seen from this
// package. The shell links the vendored htmx and uPlot from there.
const (
	webAssetsPath = "/assets/"
	webAssetsDir  = "../assets"
)

// TestFleetViewDataIsTyped inspects every generated component's parameters.
//
// VALIDATES: each fleet component takes named, typed view data.
// PREVENTS: a map or an empty interface as view data, whose misspelled key
// renders a blank region and reports success.
func TestFleetViewDataIsTyped(t *testing.T) {
	templcheck.AssertTyped(t, ".", fleetComponentCount)
}

// TestFleetNoGoFileBuildsMarkup scans every Go string literal of the package.
//
// VALIDATES: no fleet Go file builds markup; markup lives in .templ files.
// PREVENTS: markup in a Go string, which nothing type-checks.
func TestFleetNoGoFileBuildsMarkup(t *testing.T) {
	markupcheck.AssertNoMarkup(t, ".", fleetMarkupExempt, fleetMarkupFloors)
}

// TestFleetTemplatesAvoidInlineScriptAndStyle scans every fleet .templ file.
//
// VALIDATES: no inline script block, style attribute, event handler attribute
// or hx-on attribute.
// PREVENTS: markup the content security policy refuses in the browser while
// the server answers 200.
func TestFleetTemplatesAvoidInlineScriptAndStyle(t *testing.T) {
	markupcheck.AssertNoInlineScriptOrStyle(t, ".", fleetTemplFileFloor)
}

// TestFleetTemplAssetsResolve resolves every asset the fleet templates name
// against the filesystem that serves it: the package's own embedded assets
// for a path under /fleet/assets/, and the directory the web package embeds
// for a path under /assets/.
//
// VALIDATES: each src and href under either prefix names a file that is
// served there, and a path under neither is refused.
// PREVENTS: a renamed stylesheet or script, in this package or in the web
// package's vendored set, which only a browser would see.
func TestFleetTemplAssetsResolve(t *testing.T) {
	served, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		t.Fatalf("sub-FS for the served fleet assets: %v", err)
	}

	if _, statErr := os.Stat(webAssetsDir); statErr != nil {
		t.Fatalf("the web package's assets directory, which it embeds and serves at %s: %v", webAssetsPath, statErr)
	}

	markupcheck.AssertAssetsResolveIn(t, ".", []markupcheck.ServedTree{
		{Prefix: fleetAssetsPath, Assets: served},
		{Prefix: webAssetsPath, Assets: os.DirFS(webAssetsDir)},
	}, fleetAssetRefFloor)
}
