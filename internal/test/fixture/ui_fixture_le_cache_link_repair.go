// Design: docs/contributing/running-commands.md -- when the disk is full: the cache link
// Related: register_le_cache_link_repair.go -- the registration
// Related: ui_fixture_le_store_trim_answers.go -- the built-le fixture whose build and run helpers this one uses

package fixture

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// leCacheLinkXDGKey names the per-user cache root that the le under test reads
// (scratch.cacheTarget), so it is pointed into the fixture's own directory.
const leCacheLinkXDGKey = "XDG_CACHE_HOME"

// leCacheLinkRepair proves, through the built le binary, that every le run
// makes the checkout's cache/ the link to the per-user cache and repairs a
// real cache/ directory (spec-le-one-go-cache-per-host AC-1, AC-2, AC-3).
//
// It builds le from this checkout, then runs a cheap le command three times
// over a throwaway checkout: with no cache/, the run leaves the link; with a
// real cache/ holding a file, the run renames it to cache.stale-*, names both
// paths and the size on stderr, and leaves the link; with the link in place,
// the run prints nothing on stderr and the same stdout as the repairing run.
//
// The real checkout and the real per-user cache are never named to the le it
// runs: ZE_REPO_ROOT, XDG_CACHE_HOME and CLAUDE_CONFIG_DIR all point into the
// fixture's own directory, and the automatic store trim is off.
func leCacheLinkRepair(ctx context.Context) error {
	source := os.Getenv("ZE_REPO_ROOT")
	if source == "" {
		return leStoreTrimFailf("ZE_REPO_ROOT is not set")
	}
	source, err := filepath.Abs(source)
	if err != nil {
		return leStoreTrimFailf("resolve ZE_REPO_ROOT: %v", err)
	}

	work, err := os.MkdirTemp("", "le-cache-link-")
	if err != nil {
		return leStoreTrimFailf("create fixture directory: %v", err)
	}
	defer os.RemoveAll(work) //nolint:errcheck // fixture cleanup

	// le resolves its checkout through its symlinks, so the fixture compares
	// against the same spelling.
	work, err = filepath.EvalSymlinks(work)
	if err != nil {
		return leStoreTrimFailf("resolve fixture directory: %v", err)
	}
	lePath, err := leStoreTrimBuild(ctx, source, work)
	if err != nil {
		return err
	}

	checkout := filepath.Join(work, "checkout")
	if err := os.Mkdir(checkout, 0o750); err != nil {
		return leStoreTrimFailf("create the throwaway checkout: %v", err)
	}
	// le links cache/ only in a Ze checkout (lepath.IsCheckout), so the
	// throwaway tree carries the two markers that make it one.
	for name, body := range map[string]string{fileGoMod: "module example.com/cachelink\n", fileFeatureGates: ""} {
		if err := os.WriteFile(filepath.Join(checkout, name), []byte(body), 0o600); err != nil {
			return leStoreTrimFailf("write the throwaway checkout's %s: %v", name, err)
		}
	}
	perUser := filepath.Join(work, "xdg")
	target := filepath.Join(perUser, "ze")
	link := filepath.Join(checkout, "cache")
	environment := childEnvironment(envRootedAt(checkout), map[string]string{
		leStoreTrimSwitchKey: "off",
		leCacheLinkXDGKey:    perUser,
		"CLAUDE_CONFIG_DIR":  filepath.Join(work, "claude"),
	})
	le := func() (leStoreTrimResult, error) {
		return leStoreTrimRun(ctx, lePath, checkout, environment, "scratch")
	}

	// Step 1 (AC-1): no cache/ at all.
	created, err := le()
	if err != nil {
		return err
	}
	if created.exitCode != 0 {
		return leStoreTrimFailf("step 1: `le scratch` exited %d: %s", created.exitCode, created.stderr)
	}
	if err := leCacheLinkIs(link, target); err != nil {
		return leStoreTrimFailf("step 1, no cache/: %v", err)
	}

	// Step 2 (AC-2): cache/ is a real directory holding a built entry.
	if err := os.Remove(link); err != nil {
		return leStoreTrimFailf("remove the link: %v", err)
	}
	entry := filepath.Join("go-cache", "00", "entry-a")
	if err := os.MkdirAll(filepath.Join(link, "go-cache", "00"), 0o750); err != nil {
		return leStoreTrimFailf("create the real cache/: %v", err)
	}
	if err := os.WriteFile(filepath.Join(link, entry), []byte("built"), 0o600); err != nil {
		return leStoreTrimFailf("seed the real cache/: %v", err)
	}
	repaired, err := le()
	if err != nil {
		return err
	}
	if repaired.exitCode != 0 {
		return leStoreTrimFailf("step 2: `le scratch` exited %d: %s", repaired.exitCode, repaired.stderr)
	}
	if err := leCacheLinkIs(link, target); err != nil {
		return leStoreTrimFailf("step 2, real cache/: %v", err)
	}
	stale, err := filepath.Glob(link + ".stale-*")
	if err != nil {
		return leStoreTrimFailf("list the stale copies: %v", err)
	}
	if len(stale) != 1 {
		return leStoreTrimFailf("step 2: stale copies %v, want exactly one", stale)
	}
	body, err := os.ReadFile(filepath.Join(stale[0], entry)) //nolint:gosec // the fixture's own throwaway checkout
	if err != nil {
		return leStoreTrimFailf("step 2: the seeded entry is not in %s: %v", stale[0], err)
	}
	if string(body) != "built" {
		return leStoreTrimFailf("step 2: the moved entry holds %q, want built", body)
	}
	for _, want := range []string{"was a real directory (", link, stale[0], target} {
		if !strings.Contains(repaired.stderr, want) {
			return leStoreTrimFailf("step 2: stderr does not name %q:\n%s", want, repaired.stderr)
		}
	}

	// Step 3 (AC-3): the link is right, so nothing is said or moved.
	quiet, err := le()
	if err != nil {
		return err
	}
	if quiet.exitCode != 0 {
		return leStoreTrimFailf("step 3: `le scratch` exited %d: %s", quiet.exitCode, quiet.stderr)
	}
	if quiet.stderr != "" {
		return leStoreTrimFailf("step 3: a correct link printed on stderr:\n%s", quiet.stderr)
	}
	if again, _ := filepath.Glob(link + ".stale-*"); len(again) != 1 {
		return leStoreTrimFailf("step 3: stale copies %v, want the one from step 2", again)
	}
	if repaired.stdout != quiet.stdout {
		return leStoreTrimFailf("the repair changed stdout:\nrepairing run:\n%s\nquiet run:\n%s", repaired.stdout, quiet.stdout)
	}

	var verdict textbuf.Buffer
	return verdict.Str("OK\n").StdOut()
}

// leCacheLinkIs checks that link is a symbolic link naming target.
func leCacheLinkIs(link, target string) error {
	info, err := os.Lstat(link)
	if err != nil {
		return leStoreTrimFailf("inspect %s: %v", link, err)
	}
	if info.IsDir() {
		return leStoreTrimFailf("%s is a real directory, not the link to %s", link, target)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return leStoreTrimFailf("%s is a %v, not the link to %s", link, info.Mode(), target)
	}
	got, err := os.Readlink(link)
	if err != nil {
		return leStoreTrimFailf("read %s: %v", link, err)
	}
	if filepath.Clean(got) != target {
		return leStoreTrimFailf("%s -> %s, want %s", link, got, target)
	}
	return nil
}
