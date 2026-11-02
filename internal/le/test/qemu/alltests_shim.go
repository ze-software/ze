// Design: docs/architecture/testing/qemu-integration.md -- what the VM proves
// Related: alltests.go -- the run that builds this directory before its first suite
// Related: guestle.go -- the le link and the job stand-in the directory also holds

package testqemu

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/test/runner"
)

// shim builds the VM-local directory of stable binary names.
//
// Some UI tests exec `ze-stripped` through PATH, and the host cross-compiles to
// arch-suffixed names so that bin/ze stays the host-native binary.
//
// The daemon binaries are COPIED here, never linked into the 9p share. The
// share answers each page fault with one round trip to the host, and the guest
// keeps no page cache for a 9p file that nothing maps, so a daemon exec'd
// through a link paged its own text in over the share at every start: about
// 7300 major faults for each `ze start`, measured in the guest on 2026-11-01.
// How long that takes is decided by the host's load, not by ze, and on a loaded
// host it outlasted a .ci's whole budget: test/plugin/iface-tunnel-kinds.ci timed
// out at 20s with ze still paging itself in, no stderr written and no tunnel
// created. A copy faults from guest memory. le stays a link (guestLeLink): it is
// the running process, whose own mapping keeps its pages resident for the
// children that exec it.
//
// The directory is TRAVERSABLE BY EVERY USER, not only by root. A suite in the
// per-test network namespace runs ze as an ordinary user, ze relays a plugin
// through `le test` on this PATH, and a 0750 root-owned directory answers that
// exec with `/bin/sh: le test: Permission denied`. The plugin then never
// starts and the test times out on a symptom that names neither the directory
// nor the user: measured on 2026-09-05, 18 test/ospf tests timed out that way.
// What the directory holds is the read-only checkout's binaries, so a wider
// directory exposes nothing the guest does not already share.
func (a *allTestsRun) shim() error {
	if err := os.MkdirAll(a.BinDir, 0o755); err != nil { //nolint:gosec // see the comment above: the dropped user must exec through it
		return err
	}
	// MkdirAll leaves an EXISTING directory's mode alone, and a previous run in
	// the same guest made this one 0750.
	if err := os.Chmod(a.BinDir, 0o755); err != nil { //nolint:gosec // same reason
		return err
	}
	for name, source := range map[string]string{
		zeName:             a.workspacePath(a.ZeBin),
		zeStrippedName:     a.workspacePath(a.StrippedBin),
		zeStrippedFlowName: a.workspacePath(a.StrippedFlowBin),
	} {
		if err := copyIntoShim(source, filepath.Join(a.BinDir, name)); err != nil {
			return err
		}
	}
	// ze-fakefilter is the test plugin a .ci forks off the child PATH. The
	// runner puts ze's directory on that PATH, which here is this shim
	// directory, so it comes from beside ZE_BIN, where
	// testfunctional.PrepareGuest builds it. It is never required: only the
	// .ci that forks it needs it. When it is absent it is linked, so that test
	// fails naming the path it should have been built at.
	fakeFilter := a.workspacePath(filepath.Join(filepath.Dir(a.ZeBin), runner.FakeFilterBinary))
	if err := shimOptional(fakeFilter, filepath.Join(a.BinDir, runner.FakeFilterBinary)); err != nil {
		return err
	}
	// The harness is this run's own le. A suite runs as `le test <suite>`
	// through this link, and the runner under it links `le` into its children's
	// PATH the same way.
	if _, err := guestLeLink(a.BinDir); err != nil {
		return err
	}
	_, err := guestJobParent(a.BinDir)
	return err
}

// shimOptional copies source to target when source exists, and otherwise links
// target to the absent source.
func shimOptional(source, target string) error {
	if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
		if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return os.Symlink(source, target)
	}
	return copyIntoShim(source, target)
}

// copyIntoShim places a copy of source at target, executable by every user.
//
// Every shim copies again; no earlier copy is reused. Reuse needs a key that
// says the copy still holds the source's bytes, and size plus modification
// time does not: a daemon replaced through `cp -p`, `rsync -t` or an archive
// extract keeps both, and the guest then ran the stale copy (ze-test-judge
// probe, 2026-11-02). A key that does settle content has to read the whole
// source over the share, which is what the copy itself costs (37s for the four
// binaries on a loaded host, 2026-11-01), and a hash the host writes beside each
// binary goes stale under the same bypasses. The copy is paid once per
// `test qemu all`, outside every .ci's own budget.
//
// The copy is written beside target and renamed over it, so a previous run's
// link or copy is replaced in one step and a daemon still running from the old
// file keeps its own.
func copyIntoShim(source, target string) error {
	in, err := os.Open(source) //nolint:gosec // a binary the run's own knobs name
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck // read-only file, nothing to flush

	partial := target + ".partial"
	if err := writeShimCopy(in, partial); err != nil {
		_ = os.Remove(partial) //nolint:errcheck // the write error is the one reported
		return err
	}
	return os.Rename(partial, target)
}

// writeShimCopy writes everything in into a new file at path, mode 0755.
func writeShimCopy(in io.Reader, path string) error {
	out, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755) //nolint:gosec // the dropped user must exec it
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close() //nolint:errcheck // the copy error is the one reported
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	// OpenFile's mode passes through the umask.
	return os.Chmod(path, 0o755) //nolint:gosec // the dropped user must exec it
}
