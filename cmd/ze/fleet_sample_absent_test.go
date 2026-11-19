// VALIDATES: AC-8 -- no shipped build links the fleet interface's sample data
// source, and the tag that links it does link it.
// PREVENTS: sample data reaching a shipped binary, where the fleet pages would
// show it as a real estate.
package main

import (
	"slices"
	"testing"
)

const (
	fleetUIImport     = "github.com/ze-software/ze/internal/component/web/fleetui"
	fleetSampleImport = fleetUIImport + "/sample"
	// fleetSampleRegister is the registration of the sample, the package the
	// hub's tagged import names. It links the sample package itself.
	fleetSampleRegister = fleetSampleImport + "/register"
	fleetSampleTag      = "ze_fleet_ui_sample"
)

// fleetSamplePackages are the two packages no shipped build may link: the
// sample estate and the registration that makes it the build's source.
var fleetSamplePackages = []string{fleetSampleImport, fleetSampleRegister}

// TestShippedZeLinksNoFleetSample lists the dependencies of every shipped
// build flavor and refuses the sample package and its registration in each. The flavors are the
// ones TestNormalZeLinksNoInternalLe reads, so a new flavor joins the check
// without an edit here.
//
// At least one flavor must link the fleet package itself. Without that, the
// listing never reached the hub's fleet import, and the absence of the sample
// package beside it would prove nothing.
func TestShippedZeLinksNoFleetSample(t *testing.T) {
	root := personalityRepoRoot(t)
	featureTags := personalityFeatureTags(t, root)
	if slices.Contains(featureTags, fleetSampleTag) {
		t.Fatalf("feature-gates.txt declares %s, so every shipped build would set it", fleetSampleTag)
	}

	fleetLinked := false
	for name, tags := range shippedFlavors(t, root) {
		deps := personalityDeps(t, root, slices.Concat(tags, featureTags))
		for _, sample := range fleetSamplePackages {
			if slices.Contains(deps, sample) {
				t.Errorf("the %s flavor (%v) links the sample package %s", name, tags, sample)
			}
		}
		if slices.Contains(deps, fleetUIImport) {
			fleetLinked = true
		}
	}
	if !fleetLinked {
		t.Fatalf("no shipped flavor links %s, so the sample check read nothing", fleetUIImport)
	}
}

// TestFleetSampleTagLinksTheSample lists the dependencies of the distro
// flavor with the sample tag added, which is the review build.
//
// It is the other half of the test above: the tag that no shipped build sets
// is the one that links both sample packages, through
// cmd/ze/hub/register_fleet_ui_sample.go, so the listing above could see them.
func TestFleetSampleTagLinksTheSample(t *testing.T) {
	root := personalityRepoRoot(t)
	tags := slices.Concat([]string{"ze_core", "ze_distro", fleetSampleTag}, personalityFeatureTags(t, root))

	deps := personalityDeps(t, root, tags)
	for _, sample := range fleetSamplePackages {
		if !slices.Contains(deps, sample) {
			t.Errorf("a build with %s does not link %s", fleetSampleTag, sample)
		}
	}
}
