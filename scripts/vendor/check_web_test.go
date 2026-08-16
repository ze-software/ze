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
		"| File | Package |\n|------|---------|\n| htmx.min.js | htmx.org |\n| hx-sse.min.js | htmx.org |\n")

	sources := map[string]string{
		"htmx.min.js":   "// htmx source copy\n",
		"hx-sse.min.js": "// hx-sse source copy\n",
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
	"| htmx.min.js | 4.0.0-beta6 | https://unpkg.com/htmx.org@4.0.0-beta6/dist/htmx.min.js |\n" +
	"| hx-sse.min.js | 4.0.0-beta6 (htmx ext) | https://unpkg.com/htmx.org@4.0.0-beta6/dist/ext/hx-sse.min.js |\n"

// vendorVersionedFixture is vendorFixture with a MANIFEST.md that carries a
// version for every file.
func vendorVersionedFixture(t *testing.T, drifted map[string]string) string {
	t.Helper()

	root := vendorFixture(t, drifted)
	vendorFixtureFile(t, root, "third_party/web/MANIFEST.md", vendorFixtureVersions)

	return root
}

// verifyStages returns the stage list `make ze-precommit-verify` runs.
//
// The list lives in stagesForMode (scripts/status/verify_run.go), not in the
// Makefile, so it is read through the runner's own --list entry point.
func verifyStages(t *testing.T) string {
	t.Helper()

	out, err := runVendorCommand(t, vendorRepoRoot(t), "go", "run", "./scripts/status/verify_run.go", "--list", "ze-precommit-verify")
	if err != nil {
		t.Fatalf("list the ze-precommit-verify stages: %v\n%s", err, out)
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
// sandbox with no egress, and every train journey into a red `make ze-precommit-verify`
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

// VALIDATES: the drift gate is a stage of `make ze-precommit-verify`, so a consumer copy
// that stops matching its source is caught before a commit.
// PREVENTS: a gate that fails closed but that nothing runs. Drift then reaches
// a release, because the only command that would have reported it is one an
// operator has to remember to type.
func TestZeVerifyRunsDriftGate(t *testing.T) {
	const gate = "ze-vendor-web-check"

	stages := verifyStages(t)

	for line := range strings.SplitSeq(strings.TrimSpace(stages), "\n") {
		if strings.TrimSpace(line) == gate {
			return
		}
	}

	t.Fatalf("`make ze-precommit-verify` runs no %s stage; its stages are:\n%s", gate, stages)
}

// htmx2CoreVersion is the version literal htmx 2 writes into its minified core.
// htmx 4 writes a bare "4.0.0-beta6" and no version key at all, so this finds
// htmx 2 whatever file name it wears. A name check would pass over htmx 2
// wearing htmx 4's name, which is the one file name the cutover keeps.
const htmx2CoreVersion = `version:"2.`

// htmx2SSEExtension is the file name htmx published its htmx 2 SSE extension
// under. htmx 4 ships its extensions inside the core npm package and names this
// one hx-sse.min.js.
const htmx2SSEExtension = "sse.js"

// htmx4CoreVersion is the version literal the served core carries. It is the
// control: a tree that held no library at all would satisfy every other
// assertion here in silence.
const htmx4CoreVersion = `"4.0.0-beta`

// htmxCoreName is the file name every consumer serves the core under. It did
// not change at the cutover, so a page's script tag did not either.
const htmxCoreName = "htmx.min.js"

// htmxCoreFloor is the least number of core copies the tree holds: the
// third_party/web source and one per consumer that embeds it. Four on
// 2026-08-15.
const htmxCoreFloor = 4

// htmxAssetDirs returns every directory that can hold a vendored web asset: the
// source of truth and every consumer's assets directory. The consumers are
// walked rather than listed, so a fourth one is read the day it appears.
func htmxAssetDirs(t *testing.T, root string) []string {
	t.Helper()

	dirs := []string{filepath.Join(root, "third_party", "web", "htmx")}

	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() && d.Name() == "assets" {
			dirs = append(dirs, path)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walk internal/ for consumer asset directories: %v", err)
	}

	return dirs
}

// VALIDATES: AC-2 -- htmx 2's core and its SSE extension are absent from
// third_party/web/ and from every consumer, and htmx 4 stands in their place.
// PREVENTS: the two versions coexisting (ai/rules/no-layering.md). That is the
// state where a page silently loads the wrong one: both files answer, both
// pages render, and only the browser knows which library ran.
func TestHtmx2IsGone(t *testing.T) {
	root := vendorRepoRoot(t)

	cores := 0

	for _, dir := range htmxAssetDirs(t, root) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			path := filepath.Join(dir, entry.Name())

			if entry.Name() == htmx2SSEExtension {
				t.Errorf("%s is htmx 2's SSE extension, and htmx 4 publishes hx-sse.min.js", path)
			}

			if !strings.HasSuffix(entry.Name(), ".js") {
				continue
			}

			body, err := os.ReadFile(path) //nolint:gosec // the path comes from a walk of this repository
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}

			if strings.Contains(string(body), htmx2CoreVersion) {
				t.Errorf("%s carries htmx 2's version literal, whatever its file name says", path)
			}

			if entry.Name() != htmxCoreName {
				continue
			}

			cores++

			if !strings.Contains(string(body), htmx4CoreVersion) {
				t.Errorf("%s is the served core and carries no htmx 4 version literal", path)
			}
		}
	}

	if cores < htmxCoreFloor {
		t.Errorf("the walk read %d copies of %s, want at least %d; it has stopped reading the tree",
			cores, htmxCoreName, htmxCoreFloor)
	}
}
