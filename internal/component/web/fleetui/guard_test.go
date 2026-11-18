package fleetui

import (
	"io/fs"
	"testing"

	"github.com/ze-software/ze/internal/test/markupcheck"
	"github.com/ze-software/ze/internal/test/templcheck"
)

// The markup guards of the fleet package. Each guard walks the directory it
// is given, and the web package's guards name the web package, so the fleet
// package runs its own (docs/architecture/web-components.md, "Markup guards").

// fleetComponentCount is the number of templ components the package declares.
// It is exact: update it when the component set changes.
const fleetComponentCount = 4

// fleetMarkupExempt lists the Go files allowed to build markup. None is.
var fleetMarkupExempt = map[string]string{}

// fleetMarkupFloors is what makes a vacuous walk visible: the walk must read
// at least this many string literals, and the exemption table holds exactly
// this many entries.
var fleetMarkupFloors = markupcheck.Floors{Literals: 20, Exempt: 0}

// fleetTemplFileFloor is the fewest .templ files the inline walk must read.
const fleetTemplFileFloor = 1

// fleetAssetRefFloor is the fewest asset references the asset walk must
// resolve: the shell's stylesheet and its script.
const fleetAssetRefFloor = 2

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
// against the filesystem the assets route serves.
//
// VALIDATES: each src and href under /fleet/assets/ names an embedded file.
// PREVENTS: a renamed stylesheet or script, which only a browser would see.
func TestFleetTemplAssetsResolve(t *testing.T) {
	served, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		t.Fatalf("sub-FS for the served fleet assets: %v", err)
	}

	markupcheck.AssertAssetsResolve(t, ".", fleetAssetsPath, served, fleetAssetRefFloor)
}
