package scratch

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// VALIDATES: tmp and cache targets use the producer's exact environment and checkout formulas.
// PREVENTS: two checkouts sharing scratch or the durable cache moving with TMPDIR.
func TestTargetsMatchTheProducer(t *testing.T) {
	root := "/work/main"
	manager := New(root, []string{"TMPDIR=/scratch", "HOME=/home/alice", "XDG_CACHE_HOME=/durable"})
	const wantID = "main-f2af6c097ab83376"
	if got := checkoutID(root); got != wantID {
		t.Fatalf("checkout id = %q, want %q", got, wantID)
	}
	if got := manager.scratchTarget(); got != filepath.Join(string(filepath.Separator), "scratch", "ze", checkoutID(root)) {
		t.Errorf("scratch target = %q", got)
	}
	cache, err := manager.cacheTarget()
	if err != nil {
		t.Fatalf("cache target: %v", err)
	}
	if cache != "/durable/ze" {
		t.Errorf("cache target = %q, want /durable/ze", cache)
	}

	manager.Environ = []string{"HOME=/home/alice"}
	cache, err = manager.cacheTarget()
	if err != nil {
		t.Fatalf("home cache target: %v", err)
	}
	if cache != "/home/alice/.cache/ze" {
		t.Errorf("home cache target = %q", cache)
	}
}

// VALIDATES: `le scratch ensure` creates both links and is idempotent.
// PREVENTS: an ensure prerequisite deleting data or reporting drift after its own successful run.
func TestEnsureCreatesBothLinksAndIsIdempotent(t *testing.T) {
	manager := fixtureManager(t)
	first, code := manager.Ensure(false)
	if code != 0 {
		t.Fatalf("first ensure exit = %d, results = %#v", code, first.Results)
	}
	wantFirst := []string{
		"created  tmp -> " + manager.scratchTarget(),
		"created  cache -> " + mustCacheTarget(t, manager),
	}
	assertLines(t, first, wantFirst)
	assertLink(t, filepath.Join(manager.Root, "tmp"), manager.scratchTarget())
	assertLink(t, filepath.Join(manager.Root, "cache"), mustCacheTarget(t, manager))

	second, code := manager.Ensure(false)
	if code != 0 {
		t.Fatalf("second ensure exit = %d", code)
	}
	wantSecond := []string{
		"ok       tmp -> " + manager.scratchTarget(),
		"ok       cache -> " + mustCacheTarget(t, manager),
	}
	assertLines(t, second, wantSecond)
}

// VALIDATES: scratch ensure keeps its ordinary human-readable output unless quiet is named.
// PREVENTS: prerequisite silence becoming the default interactive behavior.
func TestEnsureActionDefaultOutput(t *testing.T) {
	manager := fixtureManager(t)
	var stderr strings.Builder
	report, code := answerEnsure(manager, nil, &stderr)
	if code != 0 {
		t.Fatalf("ensure exit = %d, results = %#v", code, report.Results)
	}
	want := "created  tmp -> " + manager.scratchTarget() + "\n" +
		"created  cache -> " + mustCacheTarget(t, manager) + "\n"
	if report.Quiet || report.Text() != want {
		t.Fatalf("report = %#v, text = %q, want default text %q", report, report.Text(), want)
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

// VALIDATES: scratch ensure quiet performs the same writes and returns a structured silent report.
// PREVENTS: clean prerequisites either becoming noisy or discarding their machine-readable answer.
func TestEnsureActionQuietOutputAndStructure(t *testing.T) {
	manager := fixtureManager(t)
	var stderr strings.Builder
	report, code := answerEnsure(manager, leaction.Arguments{"quiet": {""}}, &stderr)
	if code != 0 {
		t.Fatalf("quiet ensure exit = %d, results = %#v", code, report.Results)
	}
	if !report.Quiet || report.Text() != "" || stderr.String() != "" {
		t.Fatalf("quiet report = %#v, text = %q, stderr = %q", report, report.Text(), stderr.String())
	}
	assertLink(t, filepath.Join(manager.Root, tmpName), manager.scratchTarget())
	assertLink(t, filepath.Join(manager.Root, cacheName), mustCacheTarget(t, manager))

	body, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal quiet report: %v", err)
	}
	var decoded Report
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal quiet report: %v", err)
	}
	if !decoded.Quiet || !slices.Equal(decoded.Results, report.Results) {
		t.Fatalf("decoded report = %#v, want %#v", decoded, report)
	}
}

// VALIDATES: quiet suppresses ordinary success only; filesystem errors remain on stderr with exit 1.
// PREVENTS: clean silently ignoring an ensure failure and continuing against broken scratch paths.
func TestEnsureActionQuietErrorStillSpeaks(t *testing.T) {
	manager := fixtureManager(t)
	scratchBase := filepath.Dir(filepath.Dir(manager.scratchTarget()))
	writeFile(t, scratchBase, "blocks target directory", 0o600)

	var stderr strings.Builder
	report, code := answerEnsure(manager, leaction.Arguments{"quiet": {""}}, &stderr)
	if code != 1 {
		t.Fatalf("quiet ensure exit = %d, want 1; results = %#v", code, report.Results)
	}
	if !report.Quiet || len(report.Results) != 2 || !report.Results[0].Stderr ||
		report.Results[0].Status != "REFUSE" {
		t.Fatalf("error report = %#v", report)
	}
	wantError := report.Results[0].Line + "\n"
	if stderr.String() != wantError {
		t.Fatalf("stderr = %q, want %q", stderr.String(), wantError)
	}
	wantStdout := report.Results[1].Line + "\n"
	if report.Text() != wantStdout {
		t.Fatalf("flagged quiet stdout = %q, want %q", report.Text(), wantStdout)
	}
}

// VALIDATES: `le scratch ensure quiet` dispatches against the checkout and environment fixture.
// PREVENTS: clean routing a plausible spelling that never reaches the path-maintenance action.
func TestEnsureActionFixturePaths(t *testing.T) {
	// managerHere (actions.go) resolves ZE_REPO_ROOT through
	// filepath.EvalSymlinks before hashing it into the checkout ID, so the
	// fixture root must already be canonical or the two spellings of the same
	// directory (t.TempDir()'s /var/folders/... form on macOS versus its
	// resolved /private/var/folders/... form) hash to different targets.
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve temp dir: %v", err)
	}
	root := filepath.Join(base, "checkout")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatalf("mkdir checkout: %v", err)
	}
	t.Setenv("ZE_REPO_ROOT", root)
	t.Setenv("TMPDIR", filepath.Join(base, "scratch"))
	t.Setenv("HOME", filepath.Join(base, "home"))
	t.Setenv("XDG_CACHE_HOME", "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	answer, code := Answer([]string{"ensure", "quiet"})
	report, ok := answer.(Report)
	if code != 0 || !ok || !report.Quiet || report.Text() != "" {
		t.Fatalf("quiet action = (%#v, %d), want silent structured report", answer, code)
	}
	manager := New(root, os.Environ())
	assertLink(t, filepath.Join(root, tmpName), manager.scratchTarget())
	assertLink(t, filepath.Join(root, cacheName), mustCacheTarget(t, manager))
}

// VALIDATES: a real tmp directory is never converted by the ensure action, and gets the exact sentinel.
// PREVENTS: a prerequisite carrying session files to another device or letting go list enter caches.
func TestEnsureRefusesARealDirectoryAndWritesTheSentinel(t *testing.T) {
	manager := fixtureManager(t)
	tmp := filepath.Join(manager.Root, "tmp")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatalf("mkdir tmp: %v", err)
	}
	userWork := filepath.Join(tmp, "commit-session")
	writeFile(t, userWork, "mine", 0o600)

	report, code := manager.Ensure(false)
	if code != 0 {
		t.Fatalf("ensure exit = %d", code)
	}
	want := "SKIP     tmp: a real path exists here; run `./le scratch migrate` to convert it to a symlink"
	if report.Results[0].Line != want || !report.Results[0].Stderr {
		t.Fatalf("tmp result = %#v, want stderr %q", report.Results[0], want)
	}
	if got := readFile(t, userWork); got != "mine" {
		t.Fatalf("user work = %q", got)
	}
	if got := readFile(t, filepath.Join(tmp, "go.mod")); got != Sentinel {
		t.Fatalf("sentinel differs:\n%s", got)
	}

	writeFile(t, filepath.Join(tmp, "go.mod"), "operator sentinel\n", 0o600)
	if _, code := manager.Ensure(false); code != 0 {
		t.Fatalf("second ensure exit = %d", code)
	}
	if got := readFile(t, filepath.Join(tmp, "go.mod")); got != "operator sentinel\n" {
		t.Fatalf("existing sentinel was overwritten: %q", got)
	}
}

// VALIDATES: migration moves only classified tmp artifacts, moves the complete cache, and leaves symlinks.
// PREVENTS: session scripts moving with build output or cache entries being silently dropped.
func TestMigrateMovesSelectiveTmpAndWholeCache(t *testing.T) {
	manager := fixtureManager(t)
	forceDifferentDevices(manager)
	tmp := filepath.Join(manager.Root, "tmp")
	cache := filepath.Join(manager.Root, "cache")
	if err := os.MkdirAll(filepath.Join(tmp, "qemu", "image"), 0o750); err != nil {
		t.Fatalf("mkdir qemu: %v", err)
	}
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatalf("mkdir cache: %v", err)
	}
	writeFile(t, filepath.Join(tmp, "qemu", "image", "disk"), "qemu", 0o640)
	writeFile(t, filepath.Join(tmp, "commit-session-id-work"), "session", 0o600)
	writeFile(t, filepath.Join(cache, "go-build"), "cache", 0o640)

	report, code := manager.Migrate(false)
	if code != 0 {
		t.Fatalf("migrate exit = %d, results = %#v", code, report.Results)
	}
	wantTmp := "migrated tmp: moved 1; qemu -> " + manager.scratchTarget()
	wantCache := "migrated cache: moved 1 entries -> " + mustCacheTarget(t, manager) + "; now a symlink"
	assertLines(t, report, []string{wantTmp, wantCache})
	assertLink(t, filepath.Join(tmp, "qemu"), filepath.Join(manager.scratchTarget(), "qemu"))
	if got := readFile(t, filepath.Join(tmp, "commit-session-id-work")); got != "session" {
		t.Fatalf("session file = %q", got)
	}
	if got := readFile(t, filepath.Join(manager.scratchTarget(), "qemu", "image", "disk")); got != "qemu" {
		t.Fatalf("migrated qemu file = %q", got)
	}
	assertLink(t, cache, mustCacheTarget(t, manager))
	if got := readFile(t, filepath.Join(mustCacheTarget(t, manager), "go-build")); got != "cache" {
		t.Fatalf("migrated cache file = %q", got)
	}
	if got := readFile(t, filepath.Join(tmp, "go.mod")); got != Sentinel {
		t.Fatalf("real tmp sentinel differs:\n%s", got)
	}
}

// VALIDATES: whole-cache migration refuses a target collision before overwriting it.
// PREVENTS: a cache file from this checkout replacing user work already at the durable target.
func TestMigrateRefusesACollisionWithoutOverwritingEitherFile(t *testing.T) {
	manager := fixtureManager(t)
	forceDifferentDevices(manager)
	if err := os.MkdirAll(filepath.Join(manager.Root, "tmp"), 0o755); err != nil {
		t.Fatalf("mkdir tmp: %v", err)
	}
	cache := filepath.Join(manager.Root, "cache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatalf("mkdir cache: %v", err)
	}
	writeFile(t, filepath.Join(cache, "collision"), "source", 0o600)
	target := mustCacheTarget(t, manager)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	writeFile(t, filepath.Join(target, "collision"), "user", 0o600)

	report, code := manager.Migrate(false)
	if code != 1 {
		t.Fatalf("migrate exit = %d, want 1", code)
	}
	want := "REFUSE   cache: collision already exists in the target; resolve manually (moved 0 so far)"
	if report.Results[1].Line != want || !report.Results[1].Stderr {
		t.Fatalf("cache result = %#v, want %q", report.Results[1], want)
	}
	if got := readFile(t, filepath.Join(cache, "collision")); got != "source" {
		t.Errorf("source collision = %q", got)
	}
	if got := readFile(t, filepath.Join(target, "collision")); got != "user" {
		t.Errorf("target collision = %q", got)
	}
}

// VALIDATES: a directory whose children cannot be unlinked refuses before any move starts.
// PREVENTS: copytree leaving a cross-device cache half moved after a mode-0555 module directory.
func TestMigrateRefusesAnUndeletableDirectoryBeforeMoving(t *testing.T) {
	manager := fixtureManager(t)
	forceDifferentDevices(manager)
	if err := os.MkdirAll(filepath.Join(manager.Root, "tmp"), 0o755); err != nil {
		t.Fatalf("mkdir tmp: %v", err)
	}
	cache := filepath.Join(manager.Root, "cache")
	blocked := filepath.Join(cache, "pkg", "mod")
	if err := os.MkdirAll(blocked, 0o755); err != nil {
		t.Fatalf("mkdir blocked: %v", err)
	}
	writeFile(t, filepath.Join(blocked, "module.zip"), "must stay", 0o600)
	writeFile(t, filepath.Join(cache, "first"), "must stay", 0o600)
	if err := os.Chmod(blocked, 0o555); err != nil {
		t.Fatalf("make blocked directory undeletable: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(blocked, 0o755); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Errorf("restore blocked directory permissions: %v", err)
		}
	})
	manager.fs.access = func(path string, _ uint32) error {
		if path == blocked {
			return syscall.EACCES
		}
		return nil
	}

	report, code := manager.Migrate(false)
	if code != 1 {
		t.Fatalf("migrate exit = %d, want 1", code)
	}
	wantPrefix := "REFUSE   cache: pkg/mod is not writable and searchable"
	if !strings.HasPrefix(report.Results[1].Line, wantPrefix) {
		t.Fatalf("cache result = %q, want prefix %q", report.Results[1].Line, wantPrefix)
	}
	if got := readFile(t, filepath.Join(cache, "first")); got != "must stay" {
		t.Fatalf("first entry moved before refusal: %q", got)
	}
	if pathExists(filepath.Join(mustCacheTarget(t, manager), "first")) {
		t.Fatal("target holds an entry despite the preflight refusal")
	}
}

// VALIDATES: cache never follows a HOME-derived target change without the explicit repoint option.
// PREVENTS: a QEMU guest with HOME=/root hijacking the host checkout's durable cache link.
func TestEnsureLeavesAMismatchedCacheLinkAlone(t *testing.T) {
	manager := fixtureManager(t)
	cache := filepath.Join(manager.Root, "cache")
	hostTarget := filepath.Join(filepath.Dir(manager.Root), "host-cache")
	if err := os.Symlink(hostTarget, cache); err != nil {
		t.Fatalf("symlink cache: %v", err)
	}

	report, code := manager.Ensure(false)
	if code != 0 {
		t.Fatalf("ensure exit = %d", code)
	}
	want := "MISMATCH cache -> " + hostTarget + " (expected " + mustCacheTarget(t, manager) +
		"); left as is. Replace the symlink only after confirming this checkout owns it"
	if report.Results[1].Line != want {
		t.Fatalf("cache result = %q, want %q", report.Results[1].Line, want)
	}
	assertLink(t, cache, hostTarget)
}

// VALIDATES: selective tmp migration refuses a destination on the source device.
// PREVENTS: a successful no-op that reports disk was freed while every byte stayed on that disk.
func TestSelectiveMigrationRefusesTheSameDevice(t *testing.T) {
	manager := fixtureManager(t)
	tmp := filepath.Join(manager.Root, "tmp")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatalf("mkdir tmp: %v", err)
	}
	manager.fs.device = func(string) (uint64, error) { return 7, nil }

	report, code := manager.Migrate(false)
	if code != 1 {
		t.Fatalf("migrate exit = %d, want 1", code)
	}
	want := "REFUSE   tmp: " + manager.scratchTarget() +
		" is on the same device, so moving there frees nothing. Point TMPDIR at a directory on another drive and retry"
	if report.Results[0].Line != want {
		t.Fatalf("tmp result = %q, want %q", report.Results[0].Line, want)
	}
}

// VALIDATES: a non-EXDEV move error fails closed with the source unchanged.
// PREVENTS: an operating error being reported as a skipped artifact and an exit-zero migration.
func TestMigrationMoveErrorFailsClosed(t *testing.T) {
	manager := fixtureManager(t)
	forceDifferentDevices(manager)
	source := filepath.Join(manager.Root, "tmp", "qemu")
	writeFile(t, filepath.Join(source, "image"), "source", 0o600)
	cacheTarget := mustCacheTarget(t, manager)
	if err := os.MkdirAll(cacheTarget, 0o755); err != nil {
		t.Fatalf("mkdir cache target: %v", err)
	}
	if err := os.Symlink(cacheTarget, filepath.Join(manager.Root, "cache")); err != nil {
		t.Fatalf("symlink cache: %v", err)
	}
	manager.fs.rename = func(_, _ string) error { return syscall.EIO }

	report, code := manager.Migrate(false)
	if code != 1 {
		t.Fatalf("migrate exit = %d, want 1", code)
	}
	if report.Results[0].Status != "REFUSE" {
		t.Fatalf("tmp result = %#v, want refusal", report.Results[0])
	}
	if got := readFile(t, filepath.Join(source, "image")); got != "source" {
		t.Fatalf("source after failed move = %q", got)
	}
	if pathExists(filepath.Join(manager.scratchTarget(), "qemu")) {
		t.Fatal("failed move published a target")
	}
}

// VALIDATES: EXDEV uses the staged copy path and preserves content, mode, and ownership.
// PREVENTS: shutil-style partial publication or a copied artifact becoming world-readable.
func TestMoveEntryAcrossDevicesPreservesMetadata(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "target")
	writeFile(t, source, "payload", 0o640)
	before, err := os.Lstat(source)
	if err != nil {
		t.Fatalf("stat source: %v", err)
	}
	manager := New(root, os.Environ())
	manager.fs.rename = func(_, _ string) error { return syscall.EXDEV }

	if err := manager.moveEntry(source, target); err != nil {
		t.Fatalf("moveEntry: %v", err)
	}
	if pathExists(source) {
		t.Fatal("source still exists after the copied move")
	}
	if got := readFile(t, target); got != "payload" {
		t.Errorf("target content = %q", got)
	}
	after, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	if after.Mode().Perm() != before.Mode().Perm() {
		t.Errorf("target mode = %o, want %o", after.Mode().Perm(), before.Mode().Perm())
	}
	beforeOwner, ok := before.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("source metadata type = %T, want *syscall.Stat_t", before.Sys())
	}
	afterOwner, ok := after.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("target metadata type = %T, want *syscall.Stat_t", after.Sys())
	}
	if beforeOwner.Uid != afterOwner.Uid || beforeOwner.Gid != afterOwner.Gid {
		t.Errorf("target owner = %d:%d, want %d:%d", afterOwner.Uid, afterOwner.Gid, beforeOwner.Uid, beforeOwner.Gid)
	}
}

// VALIDATES: every scratch action publishes its native name and write flag.
// PREVENTS: help calling a writing action a check.
func TestActionsPublishBothWrites(t *testing.T) {
	list := Actions()
	want := []string{"ensure", "clean", "trim", "migrate"}
	if len(list.Actions) != len(want) {
		t.Fatalf("actions = %#v", list.Actions)
	}
	for index, action := range list.Actions {
		if action.Verb != want[index] || !action.Writes {
			t.Errorf("action %d = %#v, want %q and writes", index, action, want[index])
		}
	}
}

func fixtureManager(t *testing.T) *Manager {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "checkout")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatalf("mkdir checkout: %v", err)
	}
	return New(root, []string{
		"TMPDIR=" + filepath.Join(base, "scratch"),
		"HOME=" + filepath.Join(base, "home"),
	})
}

func forceDifferentDevices(manager *Manager) {
	manager.fs.device = func(path string) (uint64, error) {
		if strings.HasPrefix(path, manager.Root) {
			return 1, nil
		}
		return 2, nil
	}
}

func mustCacheTarget(t *testing.T, manager *Manager) string {
	t.Helper()
	target, err := manager.cacheTarget()
	if err != nil {
		t.Fatalf("cache target: %v", err)
	}
	return target
}

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path) //nolint:gosec // test-owned fixture
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

func assertLink(t *testing.T, link, want string) {
	t.Helper()
	got, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("readlink %s: %v", link, err)
	}
	if got != want {
		t.Errorf("%s -> %q, want %q", link, got, want)
	}
}

func assertLines(t *testing.T, report Report, want []string) {
	t.Helper()
	got := make([]string, 0, len(report.Results))
	for _, result := range report.Results {
		got = append(got, result.Line)
	}
	if !slices.Equal(got, want) {
		t.Errorf("lines = %q, want %q", got, want)
	}
}

// VALIDATES: AC-1 of spec-le-one-go-cache-per-host. A checkout with no cache/
// gets the link to the per-user target and no real directory.
// PREVENTS: activatePatchedGOROOT or a build creating cache/ as a real
// directory, which grows a second Go build cache inside the checkout.
func TestEnsureCacheLinkCreatesMissing(t *testing.T) {
	manager := fixtureManager(t)
	target := mustCacheTarget(t, manager)

	result, err := manager.EnsureCache(false)
	if err != nil {
		t.Fatalf("ensure cache: %v", err)
	}
	if result.Status != "created" {
		t.Fatalf("result = %#v, want created", result)
	}
	if result.Announce() {
		t.Errorf("creating a missing link is announced: %#v", result)
	}
	assertLink(t, filepath.Join(manager.Root, "cache"), target)
	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		t.Errorf("target %s is not a directory: %v", target, err)
	}
}

// VALIDATES: AC-2 and R-7. A real cache/ directory is renamed to
// cache.stale-<timestamp>, the link takes its place, and the announced line
// names both paths and the size; nothing is deleted. A second run finds the
// link and moves nothing. A rename that loses the race to another le, which
// moved the directory and made the link first, is success.
// PREVENTS: the repair deleting a cache another session builds through, or
// failing the action because two le runs repaired at once.
func TestEnsureCacheLinkRenamesRealDirectory(t *testing.T) {
	manager := fixtureManager(t)
	target := mustCacheTarget(t, manager)
	link := filepath.Join(manager.Root, "cache")
	entry := filepath.Join(link, "go-cache", "00", "entry-a")
	writeFile(t, entry, "built", 0o600)

	result, err := manager.EnsureCache(false)
	if err != nil {
		t.Fatalf("ensure cache: %v", err)
	}
	stale, globErr := filepath.Glob(link + ".stale-*")
	if globErr != nil || len(stale) != 1 {
		t.Fatalf("stale copies = %v (%v), want exactly one", stale, globErr)
	}
	if result.Status != "repaired" || !result.Announce() {
		t.Fatalf("result = %#v, want an announced repair", result)
	}
	for _, want := range []string{link, stale[0], target, "M)"} {
		if !strings.Contains(result.Line, want) {
			t.Errorf("repair line %q does not name %q", result.Line, want)
		}
	}
	assertLink(t, link, target)
	moved := filepath.Join(stale[0], "go-cache", "00", "entry-a")
	if got := readFile(t, moved); got != "built" {
		t.Errorf("moved entry = %q, want built", got)
	}

	again, err := manager.EnsureCache(false)
	if err != nil {
		t.Fatalf("second ensure cache: %v", err)
	}
	if again.Status != "ok" || again.Announce() {
		t.Errorf("second result = %#v, want a silent ok", again)
	}
	if after, _ := filepath.Glob(link + ".stale-*"); len(after) != 1 {
		t.Errorf("second run left stale copies %v", after)
	}

	raced := fixtureManager(t)
	racedTarget := mustCacheTarget(t, raced)
	racedLink := filepath.Join(raced.Root, "cache")
	writeFile(t, filepath.Join(racedLink, "go-cache", "entry"), "built", 0o600)
	raced.fs.rename = func(source, _ string) error {
		// The other le wins: it moves the directory and links first.
		if err := os.Rename(source, source+".stale-other"); err != nil {
			return err
		}
		if err := os.MkdirAll(racedTarget, 0o750); err != nil {
			return err
		}
		if err := os.Symlink(racedTarget, source); err != nil {
			return err
		}
		return &os.LinkError{Op: "rename", Old: source, New: source + ".stale-mine", Err: syscall.ENOENT}
	}
	result, err = raced.EnsureCache(false)
	if err != nil {
		t.Fatalf("raced ensure cache: %v", err)
	}
	if result.Status != "ok" || result.Announce() {
		t.Errorf("raced result = %#v, want a silent ok", result)
	}
	assertLink(t, racedLink, racedTarget)
}

// VALIDATES: AC-3. A link to the target written with a trailing slash is the
// correct link: nothing is renamed, repointed or announced.
// PREVENTS: a raw string comparison reporting MISMATCH for a link the owner
// made by hand as ~/.cache/ze/.
func TestEnsureCacheLinkAcceptsTrailingSlash(t *testing.T) {
	manager := fixtureManager(t)
	target := mustCacheTarget(t, manager)
	link := filepath.Join(manager.Root, "cache")
	if err := os.MkdirAll(target, 0o750); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.Symlink(target+"/", link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	result, err := manager.EnsureCache(false)
	if err != nil {
		t.Fatalf("ensure cache: %v", err)
	}
	if result.Status != "ok" || result.Announce() {
		t.Errorf("result = %#v, want a silent ok", result)
	}
	assertLink(t, link, target+"/")
	if stale, _ := filepath.Glob(link + ".stale-*"); len(stale) != 0 {
		t.Errorf("a correct link was moved aside: %v", stale)
	}
}

// TestLauncherLinkMatchesCacheTarget runs the ensure_cache_link function of the
// `le` and the root `ze` scripts, the shell copies of EnsureCache that run
// before any le or ze binary exists, against throwaway checkouts.
//
// VALIDATES: R-6 of spec-le-one-go-cache-per-host. Each script links cache/ to
// the directory cacheTarget names, under XDG_CACHE_HOME and under HOME, and
// renames a real cache/ aside with the spelling EnsureCache uses; the two
// scripts carry the same function body.
// PREVENTS: the launchers and the binary linking cache/ to three places, or a
// first `./ze` building into a real cache/ the next le run then moves aside.
// It lives here, not beside TestBootstrapCacheMatchesTheShellScript in
// internal/le/go/toolchain, because scratch imports that package.
func TestLauncherLinkMatchesCacheTarget(t *testing.T) {
	functions := map[string]string{}
	for _, name := range []string{"le", "ze"} {
		script, err := os.ReadFile(filepath.Join("..", "..", "..", name))
		if err != nil {
			t.Fatalf("read the %s script: %v", name, err)
		}
		functions[name] = launcherFunction(t, string(script), "ensure_cache_link")
		if !strings.Contains(string(script), "\tensure_cache_link\n\tGOCACHE=$root/cache/go-cache\n") {
			t.Errorf("the %s script does not call ensure_cache_link right before it sets GOCACHE under cache/", name)
		}
	}
	if functions["le"] != functions["ze"] {
		t.Errorf("the ze script's ensure_cache_link differs from the le script's:\nle:\n%s\nze:\n%s", functions["le"], functions["ze"])
	}
	for name, function := range functions {
		t.Run(name, func(t *testing.T) { checkLauncherLink(t, function) })
	}
}

// checkLauncherLink runs one script's ensure_cache_link under XDG_CACHE_HOME
// and under HOME: a missing cache/ becomes the link cacheTarget names, and a
// real one is renamed aside, named on stderr, and replaced by the link.
func checkLauncherLink(t *testing.T, function string) {
	t.Helper()
	base := t.TempDir()
	cases := []struct {
		name    string
		environ []string
	}{
		{"xdg", []string{"HOME=" + filepath.Join(base, "home"), "XDG_CACHE_HOME=" + filepath.Join(base, "xdg")}},
		{"home", []string{"HOME=" + filepath.Join(base, "home")}},
	}
	for _, tc := range cases {
		root := filepath.Join(base, tc.name)
		if err := os.Mkdir(root, 0o755); err != nil {
			t.Fatalf("mkdir checkout: %v", err)
		}
		want, err := New(root, tc.environ).cacheTarget()
		if err != nil {
			t.Fatalf("%s: cache target: %v", tc.name, err)
		}
		stderr := runLauncherFunction(t, function, root, tc.environ)
		if stderr != "" {
			t.Errorf("%s: creating the link printed %q", tc.name, stderr)
		}
		assertLink(t, filepath.Join(root, "cache"), want)

		link := filepath.Join(root, "cache")
		if err := os.Remove(link); err != nil {
			t.Fatalf("remove link: %v", err)
		}
		writeFile(t, filepath.Join(link, "go-cache", "entry"), "built", 0o600)
		stderr = runLauncherFunction(t, function, root, tc.environ)
		stale, _ := filepath.Glob(link + ".stale-*")
		if len(stale) != 1 {
			t.Fatalf("%s: stale copies = %v, want one", tc.name, stale)
		}
		if got := readFile(t, filepath.Join(stale[0], "go-cache", "entry")); got != "built" {
			t.Errorf("%s: moved entry = %q", tc.name, got)
		}
		for _, name := range []string{link, stale[0], want} {
			if !strings.Contains(stderr, name) {
				t.Errorf("%s: repair line %q does not name %q", tc.name, stderr, name)
			}
		}
		assertLink(t, link, want)
	}
}

// launcherFunction cuts one function, from its `name() {` line to the closing
// brace at the start of a line, out of a launcher script.
func launcherFunction(t *testing.T, script, name string) string {
	t.Helper()
	start := strings.Index(script, "\n"+name+"() {\n")
	if start < 0 {
		t.Fatalf("the launcher script defines no %s function", name)
	}
	body := script[start+1:]
	end := strings.Index(body, "\n}\n")
	if end < 0 {
		t.Fatalf("the launcher script's %s function has no closing brace", name)
	}
	return body[:end+3]
}

// runLauncherFunction runs function under `sh -eu` with root set as the
// script sets it, and answers its stderr.
func runLauncherFunction(t *testing.T, function, root string, environ []string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "sh", "-euc", function+"root=$1\nensure_cache_link\n", "sh", root) //nolint:gosec // the test chooses the program and its arguments
	command.Env = append([]string{"PATH=" + os.Getenv("PATH")}, environ...)
	var stderr strings.Builder
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("ensure_cache_link: %v\n%s", err, stderr.String())
	}
	return stderr.String()
}
