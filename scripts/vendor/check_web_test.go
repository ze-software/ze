// Related: check_web.go -- the drift gate these tests drive from its entry point
// Related: sync_web_test.go -- the sync side, which reuses the fixture built here
//
// Both programs in this directory carry //go:build ignore, so this package
// holds no non-test file and cannot call driftCheck directly. That is the right
// shape anyway: ai/rules/evidence.md requires a guard to be driven from its
// ENTRY POINT, and the entry point of a gate is its exit status.

package main

import (
	"context"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const vendorTimeout = 60 * time.Second

// vendorRepoRoot returns the repository root, two directories above this test.
func vendorRepoRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve the repository root: %v", err)
	}

	return root
}

// vendorFixtureFile writes one file into the fixture tree.
func vendorFixtureFile(t *testing.T, root, rel, content string) {
	t.Helper()

	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create the fixture directory for %s: %v", rel, err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write the fixture file %s: %v", rel, err)
	}
}

// vendorFixture builds a tree shaped like this repository's vendoring: one
// source copy under third_party/web/htmx and one copy per consumer. Each entry
// in drifted replaces that consumer copy's content.
//
// The tree carries its own go.mod, because repoRoot() in both programs walks up
// for one. The live checkout is therefore never the tree either program reads,
// whatever a caller passes as --root.
//
// MANIFEST.md names both files and carries no version. checkVersion returns
// before it queries registry.npmjs.org when the manifest holds no version, so
// this fixture reaches driftCheck with no network call.
func vendorFixture(t *testing.T, drifted map[string]string) string {
	t.Helper()

	root := t.TempDir()

	vendorFixtureFile(t, root, "go.mod", "module example.test/vendorweb\n\ngo 1.26\n")
	vendorFixtureFile(t, root, "third_party/web/MANIFEST.md",
		"| File | Package |\n|------|---------|\n| htmx.min.js | htmx.org |\n| sse.js | htmx-ext-sse |\n")

	sources := map[string]string{
		"htmx.min.js": "// htmx source copy\n",
		"sse.js":      "// sse source copy\n",
	}

	consumers := []string{
		"internal/chaos/web/assets",
		"internal/component/lg/assets",
		"internal/component/web/assets",
	}

	for name, content := range sources {
		vendorFixtureFile(t, root, filepath.Join("third_party", "web", "htmx", name), content)

		for _, consumer := range consumers {
			rel := filepath.Join(consumer, name)

			body, ok := drifted[rel]
			if !ok {
				body = content
			}

			vendorFixtureFile(t, root, rel, body)
		}
	}

	return root
}

// runVendorCommand runs one command in dir and returns its combined output with
// the process error.
func runVendorCommand(t *testing.T, dir, name string, args ...string) (string, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), vendorTimeout)
	defer cancel()

	cmd := osexec.CommandContext(ctx, name, args...)
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()

	return string(out), err
}

// runVendorProgram runs one of this directory's programs against the fixture
// root and returns its combined output with the process error.
//
// cmd.Dir is the fixture, not the checkout, and --root names the same tree.
// Either one alone keeps the run off the live checkout; both are passed so the
// call reads the way an operator would type it.
func runVendorProgram(t *testing.T, program, root string, args ...string) (string, error) {
	t.Helper()

	return runVendorProgramEnv(t, program, root, nil, args...)
}

// runVendorProgramEnv is runVendorProgram with extra environment entries
// appended to the parent environment.
func runVendorProgramEnv(t *testing.T, program, root string, env []string, args ...string) (string, error) {
	t.Helper()

	path := filepath.Join(vendorRepoRoot(t), "scripts", "vendor", program)
	argv := append([]string{"run", path, "--root", root}, args...)

	ctx, cancel := context.WithTimeout(context.Background(), vendorTimeout)
	defer cancel()

	cmd := osexec.CommandContext(ctx, "go", argv...)
	cmd.Dir = root
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	out, err := cmd.CombinedOutput()

	return string(out), err
}

// unreachableNetwork points every HTTP client in the child process at a closed
// port on the loopback interface. Go's default transport reads these, so a
// registry query made under it fails at once instead of reaching npm.
//
// Both spellings are set: net/http/httpproxy reads the upper-case name first,
// and the lower-case one would otherwise survive from the parent environment.
var unreachableNetwork = []string{
	"HTTP_PROXY=http://127.0.0.1:1",
	"HTTPS_PROXY=http://127.0.0.1:1",
	"http_proxy=http://127.0.0.1:1",
	"https_proxy=http://127.0.0.1:1",
	"NO_PROXY=",
	"no_proxy=",
}

// vendorFixtureVersions is a MANIFEST.md that CARRIES a version for each
// vendored file.
//
// The version is what makes the network tests discriminate. checkVersion
// returns before it queries registry.npmjs.org when the manifest holds no
// version, so a versionless fixture would report "no network was used" whatever
// the code did.
const vendorFixtureVersions = "| Asset | Version | Source |\n" +
	"|-------|---------|--------|\n" +
	"| htmx.min.js | 2.0.4 | https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js |\n" +
	"| sse.js | 2.0.4 (htmx ext) | https://unpkg.com/htmx-ext-sse@2.0.4/sse.js |\n"

// vendorVersionedFixture is vendorFixture with a MANIFEST.md that carries a
// version for every file.
func vendorVersionedFixture(t *testing.T, drifted map[string]string) string {
	t.Helper()

	root := vendorFixture(t, drifted)
	vendorFixtureFile(t, root, "third_party/web/MANIFEST.md", vendorFixtureVersions)

	return root
}

// verifyStages returns the stage list `make ze-verify` runs.
//
// The list lives in stagesForMode (scripts/status/verify_run.go), not in the
// Makefile, so it is read through the runner's own --list entry point.
func verifyStages(t *testing.T) string {
	t.Helper()

	out, err := runVendorCommand(t, vendorRepoRoot(t), "go", "run", "./scripts/status/verify_run.go", "--list", "ze-verify")
	if err != nil {
		t.Fatalf("list the ze-verify stages: %v\n%s", err, out)
	}

	return out
}

// VALIDATES: scripts/vendor/check_web.go exits non-zero when a consumer copy
// differs from its third_party/web/ source, and names the file it found.
// PREVENTS: a vendoring guard that prints DRIFT and returns success. ze then
// serves an asset that is in no consumer's source of truth, and every gate
// reading this program's exit status stays green while it happens.
func TestDriftCheckExitsNonZeroOnMismatch(t *testing.T) {
	const drifted = "internal/component/web/assets/htmx.min.js"

	root := vendorFixture(t, map[string]string{drifted: "// an edited consumer copy\n"})

	out, err := runVendorProgram(t, "check_web.go", root)
	if err == nil {
		t.Fatalf("check_web.go exited 0 over a drifted consumer copy; a drift gate must fail closed\n%s", out)
	}

	if !strings.Contains(out, drifted) {
		t.Errorf("check_web.go did not name %s in its output:\n%s", drifted, out)
	}
}

// VALIDATES: scripts/vendor/check_web.go completes and reports its verdict with
// the network unreachable, because the drift comparison queries no registry
// (AC-3). The second subtest proves the first one discriminates: the same run
// with --updates DOES reach for the registry and says so.
// PREVENTS: a commit gate whose verdict depends on npm being up and reachable.
// A gate that cannot run offline turns every airgapped checkout, every CI
// sandbox with no egress, and every train journey into a red `make ze-verify`
// that no edit in the tree explains.
func TestDriftCheckNeedsNoNetwork(t *testing.T) {
	// Lines the registry query prints. checkVersion emits one per package,
	// whether the fetch works or fails.
	registryOutput := []string{
		"npm registry",
		"could not fetch latest version",
		"up to date",
		"available",
	}

	t.Run("the-gate-reports-with-the-network-unreachable", func(t *testing.T) {
		root := vendorVersionedFixture(t, nil)

		out, err := runVendorProgramEnv(t, "check_web.go", root, unreachableNetwork)
		if err != nil {
			t.Fatalf("check_web.go failed with the network unreachable; the drift gate must query no registry\n%s", out)
		}

		if !strings.Contains(out, "consumer copies") {
			t.Errorf("check_web.go reported no verdict on the consumer copies:\n%s", out)
		}

		for _, line := range registryOutput {
			if strings.Contains(out, line) {
				t.Errorf("check_web.go reported %q without --updates, so it queried the registry:\n%s", line, out)
			}
		}
	})

	t.Run("the-registry-query-is-what-reaches-the-network", func(t *testing.T) {
		root := vendorVersionedFixture(t, nil)

		out, err := runVendorProgramEnv(t, "check_web.go", root, unreachableNetwork, "--updates")
		if err != nil {
			t.Fatalf("check_web.go --updates: %v\n%s", err, out)
		}

		if !strings.Contains(out, "could not fetch latest version") {
			t.Fatalf("check_web.go --updates reached no registry with the network unreachable, so the subtest above proves nothing:\n%s", out)
		}
	})
}

// VALIDATES: the drift gate is a stage of `make ze-verify`, so a consumer copy
// that stops matching its source is caught before a commit.
// PREVENTS: a gate that fails closed but that nothing runs. Drift then reaches
// a release, because the only command that would have reported it is one an
// operator has to remember to type.
func TestZeVerifyRunsDriftGate(t *testing.T) {
	const gate = "ze-check-vendor-web"

	stages := verifyStages(t)

	for line := range strings.SplitSeq(strings.TrimSpace(stages), "\n") {
		if strings.TrimSpace(line) == gate {
			return
		}
	}

	t.Fatalf("`make ze-verify` runs no %s stage; its stages are:\n%s", gate, stages)
}
