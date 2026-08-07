package appliance

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakePATH plants each named binary under <dir>/bin and points the lookup seam
// at that directory ONLY, answering per name.
//
// Per name matters: brewPrefixes looks up `brew` and qemuAARCH64Firmware looks
// up `qemu-system-aarch64` through the same seam. A fake that returned one path
// whatever it was asked put both binaries at the same prefix, so the
// beside-qemu branch and the brew-prefix branch produced the same answer and
// deleting either left the test green.
func fakePATH(t *testing.T, dir string, binaries ...string) string {
	t.Helper()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", bin, err)
	}
	for _, name := range binaries {
		p := filepath.Join(bin, name)
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil { //nolint:gosec // test fixture stands in for an executable
			t.Fatalf("write %s: %v", p, err)
		}
	}
	old := brewLookPathFn
	brewLookPathFn = func(name string) (string, error) {
		p := filepath.Join(bin, name)
		if _, err := os.Stat(p); err != nil {
			return "", err
		}
		return p, nil
	}
	t.Cleanup(func() { brewLookPathFn = old })
	return dir
}

// fakeBrewAt is fakePATH for the common case: a Homebrew prefix with `brew` in
// it and nothing else.
func fakeBrewAt(t *testing.T, dir string) string {
	t.Helper()
	return fakePATH(t, dir, "brew")
}

// TestBrewPrefixFollowsTheInstall proves the prefix is read off the machine
// rather than assumed.
//
// VALIDATES: an install anywhere other than /opt/homebrew is found.
// PREVENTS:  the Apple Silicon literal that was written into every Homebrew
//
//	path in this package. On an Intel Mac the prefix is /usr/local, so
//	e2fsprogs and the QEMU firmware read as absent with both properly
//	installed, and `ze appliance build` reported "debugfs not found"
//	with the binary on disk.
func TestBrewPrefixFollowsTheInstall(t *testing.T) {
	t.Setenv("HOMEBREW_PREFIX", "")
	dir := fakeBrewAt(t, t.TempDir())

	got := brewPrefixes()
	if len(got) == 0 || got[0] != dir {
		t.Fatalf("brewPrefixes() = %v, want it to lead with the brew binary's own prefix %q", got, dir)
	}
}

func TestBrewPrefixPrefersTheExportedVariable(t *testing.T) {
	exported := t.TempDir()
	t.Setenv("HOMEBREW_PREFIX", exported)
	fakeBrewAt(t, t.TempDir())

	got := brewPrefixes()
	if len(got) == 0 || got[0] != exported {
		t.Fatalf("brewPrefixes() = %v, want HOMEBREW_PREFIX %q first; it is the only source that knows a relocated install", got, exported)
	}
}

// A prefix that does not exist must not be offered, or every caller pays a stat
// for a path that cannot hold anything.
func TestBrewPrefixDropsWhatIsNotThere(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "no-such-prefix")
	t.Setenv("HOMEBREW_PREFIX", absent)
	dir := fakeBrewAt(t, t.TempDir())

	for _, p := range brewPrefixes() {
		if p == absent {
			t.Fatalf("brewPrefixes() offered %q, which does not exist", absent)
		}
	}
	if got := brewPrefixes(); len(got) == 0 || got[0] != dir {
		t.Fatalf("brewPrefixes() = %v, want the real prefix %q to lead once the absent one is dropped", got, dir)
	}
}

func TestBrewPrefixDoesNotRepeatItself(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOMEBREW_PREFIX", dir)
	fakeBrewAt(t, dir)

	seen := 0
	for _, p := range brewPrefixes() {
		if p == dir {
			seen++
		}
	}
	if seen != 1 {
		t.Fatalf("brewPrefixes() listed %q %d times, want 1", dir, seen)
	}
}

func TestBrewFileFindsAFileUnderThePrefix(t *testing.T) {
	t.Setenv("HOMEBREW_PREFIX", "")
	dir := fakeBrewAt(t, t.TempDir())

	rel := filepath.Join("share", "qemu", "ze-fake-firmware.fd")
	want := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(want), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(want, []byte("fd"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if got := brewFile(rel); got != want {
		t.Errorf("brewFile(%q) = %q, want %q", rel, got, want)
	}
	if got := brewFile(filepath.Join("share", "qemu", "ze-absent.fd")); got != "" {
		t.Errorf("brewFile of an absent file = %q, want empty", got)
	}
}

// TestE2FSSearchDirsCoverTheResolvedPrefix is the reason the helper exists: the
// keg-only e2fsprogs directories must be looked for under the prefix this host
// actually has, not under the Apple Silicon one.
//
// Homebrew does not link a keg-only formula onto PATH, so <prefix>/opt/<name>
// and <prefix>/Cellar/<name>/<version> are the only places its binaries appear.
func TestE2FSSearchDirsCoverTheResolvedPrefix(t *testing.T) {
	t.Setenv("HOMEBREW_PREFIX", "")
	dir := fakeBrewAt(t, t.TempDir())

	optSbin := filepath.Join(dir, "opt", "e2fsprogs", "sbin")
	cellarSbin := filepath.Join(dir, "Cellar", "e2fsprogs", "1.47.4", "sbin")
	for _, d := range []string{optSbin, cellarSbin} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	dirs := e2fsSearchDirs()
	for _, want := range []string{optSbin, cellarSbin, filepath.Join(dir, "sbin")} {
		if !slices.Contains(dirs, want) {
			t.Errorf("e2fsSearchDirs() = %v, missing %q", dirs, want)
		}
	}
}

// The fallback and the basename must name one file. They are separate constants
// because the fallback has to be a whole literal path, so nothing but this
// keeps them from drifting apart in an edit.
func TestQEMUFirmwareFallbackNamesTheSameFile(t *testing.T) {
	if !strings.HasSuffix(qemuAARCH64FirmwareFallback, "/"+qemuAARCH64FirmwareFile) {
		t.Errorf("fallback %q does not end in the firmware file %q", qemuAARCH64FirmwareFallback, qemuAARCH64FirmwareFile)
	}
}

// The Cellar can hold several versions at once. Homebrew keeps the previous one
// after an upgrade, so a search that stopped at the first match would keep
// pointing at 1.47.3 forever.
func TestE2FSSearchDirsPreferTheNewestCellarVersion(t *testing.T) {
	t.Setenv("HOMEBREW_PREFIX", "")
	dir := fakeBrewAt(t, t.TempDir())

	older := filepath.Join(dir, "Cellar", "e2fsprogs", "1.47.3", "sbin")
	newer := filepath.Join(dir, "Cellar", "e2fsprogs", "1.47.4", "sbin")
	for _, d := range []string{older, newer} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	dirs := e2fsSearchDirs()
	olderAt, newerAt := -1, -1
	for i, d := range dirs {
		switch d {
		case older:
			olderAt = i
		case newer:
			newerAt = i
		}
	}
	if newerAt == -1 {
		t.Fatalf("e2fsSearchDirs() = %v, missing the newest Cellar version %q", dirs, newer)
	}
	if olderAt != -1 && olderAt < newerAt {
		t.Errorf("e2fsSearchDirs() put %q before %q; the newest version must win", older, newer)
	}
}

// plantFirmware writes a fake EDK2 image at <prefix>/share/qemu and returns it.
func plantFirmware(t *testing.T, prefix string) string {
	t.Helper()
	p := filepath.Join(prefix, "share", "qemu", qemuAARCH64FirmwareFile)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte("fd"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

// TestQEMUFirmwareComesFromBesideTheBinary drives the branch that makes this
// work outside Homebrew at all.
//
// QEMU ships its firmware inside its own prefix, so the qemu binary's location
// answers for a Linux package and a source build as well as for either Mac.
// The fixture puts qemu at a DIFFERENT prefix from brew, which is the only
// arrangement that tells this branch apart from the brew-prefix one below.
func TestQEMUFirmwareComesFromBesideTheBinary(t *testing.T) {
	t.Setenv("HOMEBREW_PREFIX", "")
	t.Setenv("GOKRAZY_QEMU_AARCH64_BIOS", "")
	qemuPrefix := fakePATH(t, t.TempDir(), "qemu-system-aarch64")
	want := plantFirmware(t, qemuPrefix)

	if got := qemuAARCH64Firmware(); got != want {
		t.Errorf("qemuAARCH64Firmware() = %q, want the image beside the qemu binary %q", got, want)
	}
}

// TestQEMUFirmwareFallsBackToTheBrewPrefix drives the branch below it: no qemu
// on PATH, so the answer has to come from a resolved Homebrew prefix.
func TestQEMUFirmwareFallsBackToTheBrewPrefix(t *testing.T) {
	t.Setenv("GOKRAZY_QEMU_AARCH64_BIOS", "")
	prefix := t.TempDir()
	t.Setenv("HOMEBREW_PREFIX", prefix)
	fakePATH(t, t.TempDir()) // a PATH with neither brew nor qemu on it
	want := plantFirmware(t, prefix)

	if got := qemuAARCH64Firmware(); got != want {
		t.Errorf("qemuAARCH64Firmware() = %q, want the image under the brew prefix %q", got, want)
	}
}

// TestQEMUFirmwareFallbackWhenNothingHoldsIt: the last resort is the literal
// this code carried before the prefix was resolved, so QEMU's own "could not
// load PC BIOS" names the path an operator on that Mac has always seen.
//
// The defaults are emptied for the duration. Without that, this developer's own
// /opt/homebrew holds a real firmware image, brewFile returns it, and the
// assertion passes on a value the fallback branch never produced. The first
// version of this test also accepted ANY existing file, so pointing the branch
// at /etc/hosts kept it green.
func TestQEMUFirmwareFallbackWhenNothingHoldsIt(t *testing.T) {
	t.Setenv("GOKRAZY_QEMU_AARCH64_BIOS", "")
	t.Setenv("HOMEBREW_PREFIX", t.TempDir())
	fakePATH(t, t.TempDir())

	old := brewDefaultPrefixes
	brewDefaultPrefixes = nil
	t.Cleanup(func() { brewDefaultPrefixes = old })

	if got := qemuAARCH64Firmware(); got != qemuAARCH64FirmwareFallback {
		t.Errorf("qemuAARCH64Firmware() = %q, want exactly the fallback %q", got, qemuAARCH64FirmwareFallback)
	}
}

// TestQEMUFirmwareOperatorOverrideWins: an operator with an image outside any
// package manager has nowhere else to say so, so the variable outranks
// everything, including a firmware that does exist beside the binary.
func TestQEMUFirmwareOperatorOverrideWins(t *testing.T) {
	t.Setenv("HOMEBREW_PREFIX", "")
	qemuPrefix := fakePATH(t, t.TempDir(), "qemu-system-aarch64")
	plantFirmware(t, qemuPrefix)

	t.Setenv("GOKRAZY_QEMU_AARCH64_BIOS", "/operator/said/so.fd")
	if got := qemuAARCH64Firmware(); got != "/operator/said/so.fd" {
		t.Errorf("qemuAARCH64Firmware() = %q, want the operator's override to win", got)
	}
}

// TestBrewFileRefusesADirectory: the callers hand what they get to QEMU as a
// file, and a directory of that name is not an answer.
func TestBrewFileRefusesADirectory(t *testing.T) {
	prefix := t.TempDir()
	t.Setenv("HOMEBREW_PREFIX", prefix)
	rel := filepath.Join("share", "qemu", "ze-fake-dir.fd")
	if err := os.MkdirAll(filepath.Join(prefix, rel), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := brewFile(rel); got != "" {
		t.Errorf("brewFile of a DIRECTORY = %q, want empty", got)
	}
}

// TestCellarVersionsSortByNumber: Glob sorts by spelling, which puts 1.47.10
// below 1.47.4 and hands back the older build the first time a formula reaches
// a two-digit patch.
func TestCellarVersionsSortByNumber(t *testing.T) {
	t.Setenv("HOMEBREW_PREFIX", "")
	dir := fakeBrewAt(t, t.TempDir())
	// The fake prefix ONLY: this developer's real /opt/homebrew has an
	// e2fsprogs of its own, and its directories would land in the same slice.
	old := brewDefaultPrefixes
	brewDefaultPrefixes = nil
	t.Cleanup(func() { brewDefaultPrefixes = old })

	// {1.47.4, 1.47.10, 1.47.9} does NOT discriminate: Glob's ascending
	// spelling order already puts 1.47.10 first, so deleting the sort keeps a
	// first-element assertion green. 1.47.40 is what separates them, and the
	// whole order is asserted rather than the head.
	versions := []string{"1.47.4", "1.47.40", "1.47.9", "1.47.10"}
	for _, v := range versions {
		if err := os.MkdirAll(filepath.Join(dir, "Cellar", "e2fsprogs", v, "sbin"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	var want []string
	for _, v := range []string{"1.47.40", "1.47.10", "1.47.9", "1.47.4"} {
		want = append(want, filepath.Join(dir, "Cellar", "e2fsprogs", v, "sbin"))
	}
	if got := brewKegDirs("e2fsprogs", "sbin"); !slices.Equal(got, want) {
		t.Errorf("brewKegDirs() = %v, want newest first %v", got, want)
	}
}
