package scratch

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
)

// VALIDATES: tmp and cache targets use the producer's exact environment and checkout formulas.
// PREVENTS: two checkouts sharing scratch or the durable cache moving with TMPDIR.
func TestTargetsMatchTheProducer(t *testing.T) {
	root := "/work/main"
	manager := New(root, []string{"TMPDIR=/scratch", "HOME=/home/alice", "XDG_CACHE_HOME=/durable"})
	const wantID = "main-f2af6c097ab83376"
	if got := CheckoutID(root); got != wantID {
		t.Fatalf("checkout id = %q, want %q", got, wantID)
	}
	if got := manager.ScratchTarget(); got != filepath.Join(string(filepath.Separator), "scratch", "ze", CheckoutID(root)) {
		t.Errorf("scratch target = %q", got)
	}
	cache, err := manager.CacheTarget()
	if err != nil {
		t.Fatalf("cache target: %v", err)
	}
	if cache != "/durable/ze" {
		t.Errorf("cache target = %q, want /durable/ze", cache)
	}

	manager.Environ = []string{"HOME=/home/alice"}
	cache, err = manager.CacheTarget()
	if err != nil {
		t.Fatalf("home cache target: %v", err)
	}
	if cache != "/home/alice/.cache/ze" {
		t.Errorf("home cache target = %q", cache)
	}
}

// VALIDATES: ze-scratch-links-ensure creates both links and is idempotent.
// PREVENTS: an ensure prerequisite deleting data or reporting drift after its own successful run.
func TestEnsureCreatesBothLinksAndIsIdempotent(t *testing.T) {
	manager := fixtureManager(t)
	first, code := manager.Ensure(false)
	if code != 0 {
		t.Fatalf("first ensure exit = %d, results = %#v", code, first.Results)
	}
	wantFirst := []string{
		"created  tmp -> " + manager.ScratchTarget(),
		"created  cache -> " + mustCacheTarget(t, manager),
	}
	assertLines(t, first, wantFirst)
	assertLink(t, filepath.Join(manager.Root, "tmp"), manager.ScratchTarget())
	assertLink(t, filepath.Join(manager.Root, "cache"), mustCacheTarget(t, manager))

	second, code := manager.Ensure(false)
	if code != 0 {
		t.Fatalf("second ensure exit = %d", code)
	}
	wantSecond := []string{
		"ok       tmp -> " + manager.ScratchTarget(),
		"ok       cache -> " + mustCacheTarget(t, manager),
	}
	assertLines(t, second, wantSecond)
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
	want := "SKIP     tmp: a real path exists here; run `make ze-scratch-migrate` to convert it to a symlink"
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
	wantTmp := "migrated tmp: moved 1; qemu -> " + manager.ScratchTarget()
	wantCache := "migrated cache: moved 1 entries -> " + mustCacheTarget(t, manager) + "; now a symlink"
	assertLines(t, report, []string{wantTmp, wantCache})
	assertLink(t, filepath.Join(tmp, "qemu"), filepath.Join(manager.ScratchTarget(), "qemu"))
	if got := readFile(t, filepath.Join(tmp, "commit-session-id-work")); got != "session" {
		t.Fatalf("session file = %q", got)
	}
	if got := readFile(t, filepath.Join(manager.ScratchTarget(), "qemu", "image", "disk")); got != "qemu" {
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
		"); left as is. If this checkout is yours, run: python3 scripts/dev/ensure-links.py --repoint-cache"
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
	want := "REFUSE   tmp: " + manager.ScratchTarget() +
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
	if pathExists(filepath.Join(manager.ScratchTarget(), "qemu")) {
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

// VALIDATES: both actions retain their exact gate identities and writes flags.
// PREVENTS: the parity census claiming an unregistered spelling or help calling a write a check.
func TestActionsClaimBothWritingGates(t *testing.T) {
	list := Actions()
	if len(list.Actions) != 2 {
		t.Fatalf("actions = %#v", list.Actions)
	}
	want := []string{"ze-scratch-links-ensure", "ze-scratch-migrate"}
	for index, action := range list.Actions {
		if action.Gate != want[index] || !action.Writes {
			t.Errorf("action %d = %#v, want gate %q and writes", index, action, want[index])
		}
	}
	if !slices.Equal(actions.Gates(), want) {
		t.Errorf("claimed gates = %v, want %v", actions.Gates(), want)
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
	target, err := manager.CacheTarget()
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
