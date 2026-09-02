// Related: alltests.go -- integrationPackages, optionalPackages, excludedIntegrationPackages

package qemu

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/population"
)

// treeRoot walks up from the working directory to the checkout holding go.mod.
func treeRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("read the working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod above %s", dir)
		}
		dir = parent
	}
}

// integrationTestPackages answers every package directory, relative to root and
// slash-separated, holding a _test.go file whose build constraint names the
// integration tag.
//
// The constraint is read from the file's leading comment block: a //go:build
// line must appear before the package clause, so a bounded read of the head is
// the whole of it and the file never needs parsing.
func integrationTestPackages(t *testing.T, root string) []string {
	t.Helper()

	seen := map[string]bool{}
	for _, area := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, area), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if name := d.Name(); name == "testdata" || name == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(d.Name(), "_test.go") {
				return nil
			}
			data, readErr := os.ReadFile(path) //nolint:gosec // a path this walk produced under the checkout
			if readErr != nil {
				return readErr
			}
			head, _, _ := strings.Cut(string(data), "\npackage ")
			for line := range strings.SplitSeq(head, "\n") {
				if !strings.HasPrefix(line, "//go:build ") {
					continue
				}
				if !constraintNamesIntegration(line) {
					continue
				}
				rel, relErr := filepath.Rel(root, filepath.Dir(path))
				if relErr != nil {
					return relErr
				}
				seen[filepath.ToSlash(rel)] = true
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", area, err)
		}
	}

	out := make([]string, 0, len(seen))
	for dir := range seen {
		out = append(out, dir)
	}
	return out
}

// constraintNamesIntegration reports whether a //go:build line names the
// integration tag as a term. The tag is matched as a whole word so that a
// hypothetical `integration_slow` never counts as this one.
func constraintNamesIntegration(line string) bool {
	return slices.Contains(strings.FieldsFunc(line, func(r rune) bool {
		return r == ' ' || r == '(' || r == ')' || r == '&' || r == '|' || r == '!' || r == ','
	}), "integration")
}

// covered reports whether a package directory is reached by one of the go test
// patterns the run passes, honoring the `/...` suffix.
func covered(dir string, patterns []string) bool {
	for _, pattern := range patterns {
		p := strings.TrimPrefix(pattern, "./")
		if prefix, ok := strings.CutSuffix(p, "/..."); ok {
			if dir == prefix || strings.HasPrefix(dir, prefix+"/") {
				return true
			}
			continue
		}
		if dir == p {
			return true
		}
	}
	return false
}

// VALIDATES: every package holding an integration-tagged test is either named
// by the run or excluded with a written reason.
// PREVENTS: an integration test that compiles and executes nowhere.
//
// This is verifySuiteCoverage's guarantee for the other list in this file. That
// one derives its population from functional.Suites, which is compiled in; this
// population lives in the tree, so the derivation is a walk and the guard is a
// test rather than a run-time refusal.
//
// It found 19 such packages on 2026-08-29, of 36 holding integration tests:
// cmd/ze/hub, chaos/peer, ike/dataplane, l2tp, l2tp/pppoeclient,
// telemetry/collector, core/dnsserver, core/smart, exabgp/bridge,
// flowexport/conntrack, flowexport/sampling, iface/dhcp, iface/netlink,
// iface/ra, ldp, ntp, ospf, static and trafficusage. Every one compiled under
// GOOS=linux -tags integration, so nothing announced their absence: the lint
// matrix type-checks that tag and no runner executed them.
func TestEveryIntegrationPackageIsNamed(t *testing.T) {
	root := treeRoot(t)

	found := integrationTestPackages(t, root)
	if len(found) == 0 {
		t.Fatal("no integration-tagged test package was found, so this test asserted nothing")
	}

	patterns := append(append([]string{}, integrationPackages...), optionalPackages...)
	holding := make(map[string]bool, len(found))
	named := make(map[string]bool, len(found))
	for _, dir := range found {
		holding[dir] = true
		if covered(dir, patterns) {
			named[dir] = true
		}
	}

	claim := population.Claim{
		Subject:         "integration-tagged test package",
		Population:      holding,
		Walked:          named,
		Excused:         excludedIntegrationPackages,
		UnexcusedReason: "NAMED BY NO RUNNER",
	}
	coverage, err := claim.Assess()
	if err != nil {
		t.Fatalf("integration package coverage: %v", err)
	}
	for _, dir := range coverage.Unexcused {
		t.Errorf("%s holds integration-tagged tests and no runner names it, so they execute nowhere; "+
			"add it to integrationPackages or give excludedIntegrationPackages a reason", dir)
	}
	// A stale exclusion is the half this test was blind to until 2026-09-02. An
	// entry whose package a runner now names, or whose package no longer holds
	// an integration-tagged test, is a statement nobody rechecked.
	for _, dir := range coverage.Healed {
		t.Errorf("excludedIntegrationPackages still names %s, which a runner now covers or which"+
			" holds no integration-tagged test; delete the entry", dir)
	}
}

// VALIDATES: every package the run names is a directory that exists.
// PREVENTS: a `go test` pattern that resolves to nothing, which is how
// "./cmd/ze/doctor" made every run print `FAIL ./cmd/ze/doctor [setup failed]`
// while the check it was supposed to run never executed.
//
// It asserts existence, NOT that the package holds an integration-tagged test.
// Naming a package here means "run this package inside the VM under -tags
// integration", and the two VPP backends, ./internal/plugins/firewall/vpp/...
// and ./internal/plugins/traffic/vpp/..., are the standing examples of
// legitimate entries whose tests carry only //go:build linux.
//
// That pair is also why TestEveryIntegrationPackageIsNamed cannot find them:
// it derives its population from //go:build integration, which neither carries,
// so a linux-only backend enters this list by hand or not at all. The traffic
// one did not, and its reply-timeout tests ran in no VM until 2026-08-29.
//
// optionalPackages is exempt: it is defined as added when the directory is
// there, so an absent one is its stated behavior rather than a stale entry.
func TestEveryNamedIntegrationPackageExists(t *testing.T) {
	root := treeRoot(t)

	for _, pattern := range integrationPackages {
		rel := strings.TrimSuffix(strings.TrimPrefix(pattern, "./"), "/...")
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("%s names %s, which the tree does not hold: %v", pattern, rel, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s names %s, which is not a directory", pattern, rel)
		}
	}
}
