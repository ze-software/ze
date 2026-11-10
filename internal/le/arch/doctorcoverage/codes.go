// Design: docs/architecture/core-design.md -- le links the product to introspect it
//
// Overview: doctorcoverage.go -- the site kinds and statuses, and the scan
//
// codes.go builds the covered set: every code a registered doctor check
// declares, with the checks that declare it. It is read from the two doctor
// registries in this process, behind the blank import of the product
// composition root, so nothing here transcribes a code (ai/rules/principles.md).

package archdoctorcoverage

import (
	"errors"
	"slices"

	// The blank import runs every product init(), so the registries below hold
	// what the daemon holds. This direction is the allowed one: le may link
	// ze, ze never links le (ai/doors/le-tooling.md).
	_ "github.com/ze-software/ze/internal/component/plugin/all"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Covered maps each code a registered doctor check declares to the names of
// the checks that declare it, sorted. A plugin's check is named
// `<plugin>/<check>`.
type Covered map[string][]string

// CoveredCodes reads the covered set from the in-tree doctor check registry
// (internal/core/diagnostic) and the plugin doctor check declarations
// (internal/component/plugin/registry), every phase of both.
func CoveredCodes() Covered {
	covered := Covered{}
	for _, check := range diagnostic.DoctorChecks() {
		covered.add(check.Name, check.Codes)
	}
	for _, check := range registry.PluginDoctorChecks() {
		covered.add(pluginCheckName(check.PluginName, check.Name), check.Codes)
	}
	for code := range covered {
		slices.Sort(covered[code])
	}
	return covered
}

// add records that check declares codes.
func (c Covered) add(check string, codes []string) {
	for _, code := range codes {
		if slices.Contains(c[code], check) {
			continue
		}
		c[code] = append(c[code], check)
	}
}

// pluginCheckName names a plugin's doctor check in the covered set.
func pluginCheckName(plugin, check string) string { return plugin + "/" + check }

// ErrThinCoveredSet names a covered set smaller than the registries are known
// to hold.
var ErrThinCoveredSet = errors.New("the doctor-coverage gate read too few doctor codes from the registries to judge the tree")

// checkCodesFloor refuses a covered set below floor codes. A registry the gate
// no longer reaches (a moved package, a composition root that stopped linking
// the doctor checks) would otherwise answer every site as uncovered, and a
// refusal of every new line is a broken gate, not a finding.
func checkCodesFloor(covered Covered, floor int) error {
	if len(covered) >= floor {
		return nil
	}
	var tb textbuf.Buffer
	tb.Str(": read ").Int(int64(len(covered))).Str(", at least ").Int(int64(floor)).
		Str(" expected. Check that internal/le/arch/doctorcoverage/codes.go still links internal/component/plugin/all and reads diagnostic.DoctorChecks and registry.PluginDoctorChecks")
	return errors.New(ErrThinCoveredSet.Error() + tb.String())
}
