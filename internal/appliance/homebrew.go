// Design: docs/architecture/appliance/builder.md -- Homebrew prefix resolution for the macOS build host

package appliance

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// Homebrew has no single install prefix. It is /opt/homebrew on Apple Silicon,
// /usr/local on Intel, and whatever an operator chose for a relocated install.
//
// Every Homebrew path in this package was written as the Apple Silicon literal.
// That made it correct on one class of Mac and absent on the other. An Intel
// Mac with e2fsprogs and QEMU properly installed reported both missing, and
// `ze appliance build` logged "debugfs not found" with the binary on disk.
//
// So the prefix is ASKED FOR. The question goes to the machine before any
// literal is used.
//
// macOS is the whole subject, and the two defaults are offered ONLY there.
// /usr/local is a directory on essentially every Linux host and it is not
// Homebrew's, so offering it unconditionally would search /usr/local/sbin
// ahead of /usr/sbin on a box that has never seen Homebrew, and hand a
// source-built e2fsprogs to a build that used to get the distribution's.
// HOMEBREW_PREFIX and the brew binary are honored anywhere, so a Linuxbrew
// install is still found: by being asked for, rather than by being guessed.

// brewLookPathFn is a test seam: point it at a fake and brewPrefixes answers
// from that instead of from the machine running the test.
var brewLookPathFn = exec.LookPath

// brewDefaultPrefixes is rung 3, and a test seam for the same reason: this
// developer's Mac HAS a Homebrew at /opt/homebrew holding a real firmware
// image, so the last-resort branch of qemuAARCH64Firmware is unreachable here
// while the defaults are in play. A test empties this to reach it.
var brewDefaultPrefixes = []string{"/opt/homebrew", "/usr/local"}

// brewPrefixes returns the Homebrew prefixes that exist on this host, most
// authoritative first. It returns nothing where Homebrew is not installed.
//
//  1. HOMEBREW_PREFIX, which `brew shellenv` exports. It is the only source
//     that knows about a relocated install, so it wins when it is set.
//  2. The `brew` binary. It lives at <prefix>/bin/brew, so its own location
//     answers the question on any host that has it. This is the one that fires
//     in practice: HOMEBREW_PREFIX is unset in a plain non-login shell, and
//     `make` runs in one (measured on this machine, 2026-08-07).
//  3. The two documented defaults, for a PATH that never sourced shellenv, and
//     on macOS ONLY. See the note above: a Linux box has a /usr/local of its
//     own, and it is not Homebrew's.
//
// Duplicates are dropped, so the usual case returns exactly one directory.
func brewPrefixes() []string {
	candidates := make([]string, 0, 4)
	if p := os.Getenv("HOMEBREW_PREFIX"); p != "" {
		candidates = append(candidates, p)
	}
	if brew, err := brewLookPathFn("brew"); err == nil {
		// NOT resolved through its symlinks, deliberately. On Intel,
		// /usr/local/bin/brew is a link into /usr/local/Homebrew/bin/brew, so
		// resolving it answers /usr/local/Homebrew: the wrong prefix, and wrong
		// on exactly the machines this function exists for. The link's own
		// location is the prefix, on both architectures.
		candidates = append(candidates, filepath.Dir(filepath.Dir(brew)))
	}
	if runtime.GOOS == "darwin" {
		candidates = append(candidates, brewDefaultPrefixes...)
	}

	seen := make(map[string]bool, len(candidates))
	prefixes := make([]string, 0, len(candidates))
	for _, p := range candidates {
		if seen[p] {
			continue
		}
		seen[p] = true
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			prefixes = append(prefixes, p)
		}
	}
	return prefixes
}

// brewKegDirs returns the directories a keg-only formula's binaries can sit in,
// newest Cellar version last-installed-wins, for every resolved prefix.
//
// Homebrew does not symlink a keg-only formula onto PATH, so <prefix>/bin holds
// nothing for it. Two places do: <prefix>/opt/<formula> is the stable symlink
// Homebrew maintains at the current version, and <prefix>/Cellar/<formula>/*
// holds every version installed, including the one the opt link points at.
// Both are searched, because a Cellar-only tree (an interrupted upgrade) has no
// opt link and still holds working binaries.
func brewKegDirs(formula, sub string) []string {
	var dirs []string
	for _, prefix := range brewPrefixes() {
		if opt := filepath.Join(prefix, "opt", formula, sub); isDir(opt) {
			dirs = append(dirs, opt)
		}
		// Homebrew keeps the previous version after an upgrade, so the newest
		// must come first. Glob sorts by SPELLING, which puts 1.47.10 below
		// 1.47.4 and hands back last month's build the first time a formula
		// reaches a two-digit patch. Sort by number instead.
		matches, _ := filepath.Glob(filepath.Join(prefix, "Cellar", formula, "*", sub))
		sort.Slice(matches, func(i, j int) bool {
			return compareCellarVersion(matches[i], matches[j]) > 0
		})
		for _, m := range matches {
			if isDir(m) {
				dirs = append(dirs, m)
			}
		}
	}
	return dirs
}

// compareCellarVersion orders two Cellar paths by their version directory,
// segment by segment as numbers. It returns >0 when a is newer than b.
//
// A segment that is not a number compares below every number, so 1.47.4
// outranks 1.47.rc1. A Homebrew revision suffix is not that case: 1.47.4_1
// splits into four numeric segments and simply outranks 1.47.4.
//
// This orders VERSIONS. It does not claim to parse every string Homebrew can
// put in that directory name, and it differs from the Python copies on two
// shapes Homebrew does not emit: a zero-padded segment (1.47.04) and a doubled
// separator. Both are ordered here and called equal there.
func compareCellarVersion(a, b string) int {
	as := strings.FieldsFunc(filepath.Base(filepath.Dir(a)), isVersionSep)
	bs := strings.FieldsFunc(filepath.Base(filepath.Dir(b)), isVersionSep)
	for i := 0; i < len(as) && i < len(bs); i++ {
		ai, aerr := strconv.Atoi(as[i])
		bi, berr := strconv.Atoi(bs[i])
		switch {
		case aerr == nil && berr == nil && ai != bi:
			if ai > bi {
				return 1
			}
			return -1
		case aerr == nil && berr != nil:
			return 1
		case aerr != nil && berr == nil:
			return -1
		case as[i] != bs[i]:
			return strings.Compare(as[i], bs[i])
		}
	}
	return len(as) - len(bs)
}

func isVersionSep(r rune) bool { return r == '.' || r == '_' || r == '-' }

// qemuAARCH64FirmwareFile is the EDK2 image QEMU boots an arm64 guest from.
// Homebrew, and every Linux distribution that ships it, keep it under
// <prefix>/share/qemu.
const qemuAARCH64FirmwareFile = "edk2-aarch64-code.fd"

// qemuAARCH64FirmwareFallback is what a host with no discoverable firmware is
// handed: the Apple Silicon path, exactly as this code read before the prefix
// was resolved. QEMU's own "could not load PC BIOS" then names it, which is the
// message an operator on that Mac has always seen.
// TestQEMUFirmwareFallbackNamesTheSameFile keeps it agreeing with the basename.
const qemuAARCH64FirmwareFallback = "/opt/homebrew/share/qemu/edk2-aarch64-code.fd"

// qemuAARCH64Firmware returns the path to pass QEMU as -bios.
//
//  1. GOKRAZY_QEMU_AARCH64_BIOS. An operator with an image outside any package
//     manager has nowhere else to say so, so the override stays first.
//  2. Beside the qemu binary itself: <dir of qemu-system-aarch64>/../share/qemu.
//     QEMU ships its firmware inside its own prefix, so this answers for a
//     Homebrew install on either Mac architecture, and for a Linux one, without
//     naming a prefix at all.
//  3. Under a resolved Homebrew prefix, for a PATH that does not carry qemu.
//
// When none of them holds the file, the Apple Silicon path is returned
// unchanged. Nothing else can boot, and QEMU's own "could not load PC BIOS"
// names the path it was handed, which is the message this had before.
func qemuAARCH64Firmware() string {
	if bios := os.Getenv("GOKRAZY_QEMU_AARCH64_BIOS"); bios != "" {
		return bios
	}
	if qemu, err := brewLookPathFn("qemu-system-aarch64"); err == nil {
		// Unresolved, like the brew lookup above: Homebrew links this binary
		// into the Cellar, and following the link lands on the formula's own
		// tree rather than on <prefix>/share/qemu, which is where the firmware
		// is documented to be and where a Linux package puts it too.
		prefix := filepath.Dir(filepath.Dir(qemu))
		if p := filepath.Join(prefix, "share", "qemu", qemuAARCH64FirmwareFile); isFile(p) {
			return p
		}
	}
	if p := brewFile(filepath.Join("share", "qemu", qemuAARCH64FirmwareFile)); p != "" {
		return p
	}
	return qemuAARCH64FirmwareFallback
}

// brewFile returns the first existing <prefix>/<rel>, or "" when no Homebrew
// prefix holds it. A directory of that name is not an answer: the callers hand
// what they get to QEMU as a file.
func brewFile(rel string) string {
	for _, prefix := range brewPrefixes() {
		if p := filepath.Join(prefix, rel); isFile(p) {
			return p
		}
	}
	return ""
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
