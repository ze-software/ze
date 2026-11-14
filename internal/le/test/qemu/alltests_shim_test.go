package testqemu

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/test/runner"
)

// VALIDATES: the shim holds a guest-local COPY of each daemon binary, with the
// source's bytes and a mode the dropped netns user can exec, never a link into
// the 9p share.
// PREVENTS: test/plugin/iface-tunnel-kinds.ci and tunnel-ttl-default.ci timing
// out at their 20s budget with ze writing no stderr and creating no tunnel. A
// daemon exec'd through a link into the share page-faults its own text over 9p,
// one host round trip per fault, and the guest keeps no page cache for a 9p
// file nothing maps: every `ze start` took about 7300 major faults, measured in
// the guest on 2026-11-01. On a loaded host those faults alone outlasted the
// budget, so ze was still paging itself in when the runner killed it.
func TestTheShimCopiesTheDaemonsOffTheShare(t *testing.T) {
	run := vmFixture(t)
	if err := run.shim(); err != nil {
		t.Fatalf("shim: %v", err)
	}
	for name, source := range map[string]string{
		zeName:             run.ZeBin,
		zeStrippedName:     run.StrippedBin,
		zeStrippedFlowName: run.StrippedFlowBin,
	} {
		requireShimCopy(t, filepath.Join(run.BinDir, name), filepath.Join(run.Workspace, source))
	}
}

// VALIDATES: ze-fakefilter is copied like the daemons when it exists beside
// ZE_BIN.
// PREVENTS: the one forked test plugin still paging over the share while the
// daemon that forks it no longer does.
func TestTheShimCopiesTheFakeFilterPluginWhenItExists(t *testing.T) {
	run := vmFixture(t)
	writeExecutable(t, run.Workspace, filepath.Join("bin", runner.FakeFilterBinary))
	if err := run.shim(); err != nil {
		t.Fatalf("shim: %v", err)
	}
	requireShimCopy(t, filepath.Join(run.BinDir, runner.FakeFilterBinary),
		filepath.Join(run.Workspace, "bin", runner.FakeFilterBinary))
}

// VALIDATES: ze-upgrade, the stamped second DUT build PrepareGuest writes
// beside ZE_BIN, is copied into the shim under its canonical name.
// PREVENTS: test/ui/native-install-upgrade.ci failing in the guest with
// "ze-upgrade is not on the child PATH" although the host built it.
func TestTheShimCopiesTheUpgradeBuildWhenItExists(t *testing.T) {
	run := vmFixture(t)
	writeExecutable(t, run.Workspace, filepath.Join("bin", runner.UpgradeBinary))
	if err := run.shim(); err != nil {
		t.Fatalf("shim: %v", err)
	}
	requireShimCopy(t, filepath.Join(run.BinDir, runner.UpgradeBinary),
		filepath.Join(run.Workspace, "bin", runner.UpgradeBinary))
}

// VALIDATES: a later shim in the same guest holds the source's CURRENT bytes,
// even when the rebuilt source kept the old one's size and modification time.
// PREVENTS: a guest running a stale daemon after the host replaced it through
// `cp -p`, `rsync -t`, an archive extract or a share with coarse timestamps. A
// reuse key of size and time (the first version of this shim) skipped exactly
// that copy, and any key that settles content identity has to read the source
// over the share, which costs what the copy costs (ze-test-judge probe,
// 2026-11-02).
func TestTheShimRecopiesADaemonRebuiltWithTheSameSizeAndTime(t *testing.T) {
	run := vmFixture(t)
	if err := run.shim(); err != nil {
		t.Fatalf("first shim: %v", err)
	}
	source := filepath.Join(run.Workspace, run.ZeBin)
	info, err := os.Stat(source)
	if err != nil {
		t.Fatalf("stat the source: %v", err)
	}
	body, err := os.ReadFile(source) //nolint:gosec // the test's own fixture file
	if err != nil {
		t.Fatalf("read the source: %v", err)
	}
	body[len(body)-1] ^= 0xff
	if err := os.WriteFile(source, body, 0o700); err != nil { //nolint:gosec // fixture binary, executable by its owner
		t.Fatalf("rewrite the source: %v", err)
	}
	if err := os.Chtimes(source, info.ModTime(), info.ModTime()); err != nil {
		t.Fatalf("restore the source's time: %v", err)
	}
	if after, err := os.Stat(source); err != nil || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		t.Fatalf("the rewritten source must keep size %d and time %v: %v %v", info.Size(), info.ModTime(), after, err)
	}

	if err := run.shim(); err != nil {
		t.Fatalf("second shim: %v", err)
	}
	requireShimCopy(t, filepath.Join(run.BinDir, zeName), source)
}

// VALIDATES: a later shim replaces a copy whose source changed in size and time.
// PREVENTS: a rebuilt daemon being shadowed by the previous run's copy.
func TestTheShimRecopiesAChangedDaemon(t *testing.T) {
	run := vmFixture(t)
	if err := run.shim(); err != nil {
		t.Fatalf("first shim: %v", err)
	}
	source := writeBinary(t, run.Workspace, run.ZeBin, append(staticELF(), 'x'))
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(source, later, later); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	if err := run.shim(); err != nil {
		t.Fatalf("second shim: %v", err)
	}
	requireShimCopy(t, filepath.Join(run.BinDir, zeName), source)
}

// requireShimCopy fails unless target is a regular file holding source's bytes,
// executable by every user (the per-test namespace suites exec it as an
// ordinary user).
func requireShimCopy(t *testing.T, target, source string) {
	t.Helper()
	info, err := os.Lstat(target)
	if err != nil {
		t.Errorf("the shim holds no %s: %v", target, err)
		return
	}
	if !info.Mode().IsRegular() {
		t.Errorf("%s is %v, want a regular file copied off the 9p share", target, info.Mode().Type())
		return
	}
	if info.Mode().Perm()&0o005 != 0o005 {
		t.Errorf("%s is %v, want it readable and executable by the dropped user", target, info.Mode().Perm())
	}
	want, err := os.ReadFile(source) //nolint:gosec // the test's own fixture file
	if err != nil {
		t.Fatalf("read %s: %v", source, err)
	}
	got, err := os.ReadFile(target) //nolint:gosec // the test's own shim file
	if err != nil {
		t.Fatalf("read %s: %v", target, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s holds %d bytes that differ from %s", target, len(got), source)
	}
}
