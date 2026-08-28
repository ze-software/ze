// Design: docs/architecture/core-design.md -- a tool can be present when it is not on PATH
//
// Two table rows require more than a PATH lookup. Experience showed why these
// exceptions are necessary:
//
//	e2fsprogs   is searched by DIRECTORY on every platform. Homebrew does not
//	            link a keg-only formula to PATH. Debian does not include
//	            /usr/sbin in a non-root user's PATH. Thus, a lookup can fail
//	            when the tools are installed and operational.
//
//	            Both native consumers specify the directories directly:
//	            e2fsprogsDirs here and e2fsSearchDirs in internal/appliance.
//	            Therefore, a PATH-based probe reported the tools as missing
//	            although the build used them. Setup then reported pending forever.
//
//	staticcheck must be one exact VERSION. A different version on PATH runs but
//	            disagrees. This result is worse than an absent tool.

package setup

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// staticcheckProbeTimeout bounds `staticcheck -version`, which prints a string
// and exits.
const staticcheckProbeTimeout = 5 * time.Second

// Probe reports whether a tool is present and usable.
//
// The code selects the two special cases by name instead of by a function in
// the table. Therefore, a test can read the table without an import of this
// code.
func (s *Setup) Probe(tool Tool) bool {
	switch tool.Name {
	case toolE2fsprogs:
		return s.probeE2fsprogs()
	case toolStaticcheck:
		return s.probeStaticcheck(tool)
	}

	// A probe list with no names would pass every-name logic without a name.
	// Therefore, a row with no executable is not present. No row has this
	// condition today. The guard prevents a silent pass in the future.
	if len(tool.Probe) == 0 {
		return false
	}

	if tool.ProbeAny {
		return slices.ContainsFunc(tool.Probe, s.Shell.Present)
	}
	for _, name := range tool.Probe {
		if !s.Shell.Present(name) {
			return false
		}
	}
	return true
}

// probeStaticcheck reports whether the staticcheck on PATH is the version this
// repository pins.
//
// A version mismatch is a false green: the tool runs, reports different
// findings than CI will, and nothing says why.
func (s *Setup) probeStaticcheck(tool Tool) bool {
	path, ok := s.Shell.Which(tool.Probe[0])
	if !ok {
		return false
	}
	result := s.Shell.Run(Cmd{Argv: []string{path, versionArgument}, Timeout: staticcheckProbeTimeout})
	if !result.OK() {
		return false
	}
	return staticcheckVersionMatches(strings.TrimSpace(result.Out))
}

// staticcheckVersionMatches reports whether a `staticcheck -version` line names
// the pinned release.
//
// The script used `staticcheck <version>( \([^)]+\))?` as a regular
// expression. This code uses three checks for the same structure. It also keeps
// the version as a constant instead of inserting a fragment into a pattern. A
// valid line contains the name and the version. One nonempty parenthesized build
// identifier can follow them.
func staticcheckVersionMatches(line string) bool {
	var tb textbuf.Buffer
	want := tb.Str(toolStaticcheck).Byte(' ').Str(StaticcheckVersion).String()
	if line == want {
		return true
	}
	rest, ok := strings.CutPrefix(line, want)
	if !ok {
		return false
	}
	inner, ok := strings.CutPrefix(rest, " (")
	if !ok {
		return false
	}
	inner, ok = strings.CutSuffix(inner, ")")
	return ok && inner != "" && !strings.Contains(inner, ")")
}

// homebrewPrefixKey is the variable `brew shellenv` exports, and the one source
// that knows a relocated install.
const homebrewPrefixKey = "HOMEBREW_PREFIX"

// brewPrefixes returns the Homebrew prefixes that exist on this host. It orders
// them from most authoritative to least authoritative.
//
// Homebrew has no single prefix. It uses /opt/homebrew on Apple Silicon and
// /usr/local on Intel. An operator can also select a different location. A
// check of only the first location reported a correct e2fsprogs installation
// as missing on an Intel Mac. Setup then offered to install it again.
//
//  1. HOMEBREW_PREFIX, exported by `brew shellenv`.
//
//  2. The brew binary at <prefix>/bin/brew. Do NOT resolve its symlinks. On
//     Intel, /usr/local/bin/brew links to /usr/local/Homebrew/bin/brew. A
//     resolved link gives the wrong prefix on the machines that need this
//     check.
//
//  3. The two documented defaults on macOS ONLY. These defaults support a PATH
//     that does not contain the changes from shellenv. /usr/local exists on
//     almost every Linux host, but it is not the Homebrew prefix there. If the
//     code always includes it, /usr/local/sbin precedes the distribution's own
//     tools.
//
// The same resolution is in the Go function brewPrefixes
// (internal/appliance/homebrew.go) and the Python function brew_prefixes
// (internal/le/setup/install.go). internal/le/setup/setup_test.go verifies
// that the copies give one result.
func (s *Setup) brewPrefixes() []string {
	var candidates []string
	if exported := s.env(homebrewPrefixKey); exported != "" {
		candidates = append(candidates, exported)
	}
	if brew, ok := s.Shell.Which("brew"); ok {
		candidates = append(candidates, filepath.Dir(filepath.Dir(brew)))
	}
	if s.goos() == osDarwin {
		candidates = append(candidates, "/opt/homebrew", "/usr/local")
	}

	var prefixes []string
	seen := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			prefixes = append(prefixes, candidate)
		}
	}
	return prefixes
}

// versionPart is one segment of a Cellar version directory, ordered by NUMBER
// rather than by spelling. A segment that is not a number sorts BELOW every
// number, which is what makes 1.47.4 outrank 1.47.rc1.
type versionPart struct {
	number int
	text   string
}

// cellarVersionKey returns the sort key of a Cellar sbin directory.
//
// Plain string order puts 1.47.10 lower than 1.47.4. Thus, it selects the
// previous month's e2fsprogs when a formula first has a two-digit patch number.
// A Homebrew revision suffix is different. 1.47.4_1 has four numeric segments
// and correctly ranks higher than 1.47.4. This function uses the same keys as
// version_key in internal/le/setup/install.go.
func cellarVersionKey(sbin string) []versionPart {
	version := filepath.Base(filepath.Dir(sbin))
	fields := strings.FieldsFunc(version, func(r rune) bool {
		return r == '.' || r == '_' || r == '-'
	})
	key := make([]versionPart, 0, len(fields))
	for _, field := range fields {
		if number, err := strconv.Atoi(field); err == nil && isDigits(field) {
			key = append(key, versionPart{number: number})
			continue
		}
		key = append(key, versionPart{number: -1, text: field})
	}
	return key
}

// isDigits reports whether every rune of s is a decimal digit, which is what
// Python's str.isdigit decides for the ASCII the version strings hold.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// cellarNewerFirst reports whether a sorts before b, newest version first.
func cellarNewerFirst(a, b string) bool {
	left, right := cellarVersionKey(a), cellarVersionKey(b)
	for i := 0; i < len(left) && i < len(right); i++ {
		if left[i].number != right[i].number {
			return left[i].number > right[i].number
		}
		if left[i].text != right[i].text {
			return left[i].text > right[i].text
		}
	}
	return len(left) > len(right)
}

// e2fsprogsDirs returns every directory that can contain mkfs.ext4 and debugfs.
// It orders the best directory first.
//
// e2fsprogs is keg-only, so Homebrew does not link it to PATH. Therefore, a
// PATH lookup fails even when e2fsprogs is installed correctly. Its binaries
// are in <prefix>/opt/e2fsprogs/sbin, the stable link for the current version.
// They are also in <prefix>/Cellar/e2fsprogs/<version>/sbin. An interrupted
// upgrade can leave them in the Cellar without a link.
//
// This function is separate from probeE2fsprogs so that a test can verify WHERE
// the probe looks. The boolean cannot show this information. The list ends with
// /usr/sbin and /sbin. Thus, on a Linux host with e2fsprogs, the boolean is true
// even if both Homebrew branches are deleted.
func (s *Setup) e2fsprogsDirs() []string {
	var dirs []string
	for _, prefix := range s.brewPrefixes() {
		dirs = append(dirs, filepath.Join(prefix, "opt", toolE2fsprogs, "sbin"))

		cellar, err := filepath.Glob(filepath.Join(prefix, "Cellar", toolE2fsprogs, "*", "sbin"))
		if err == nil {
			sort.Slice(cellar, func(i, j int) bool { return cellarNewerFirst(cellar[i], cellar[j]) })
			dirs = append(dirs, cellar...)
		}
		dirs = append(dirs, filepath.Join(prefix, "sbin"))
	}
	return append(dirs, "/usr/sbin", "/sbin")
}

// probeE2fsprogs reports whether ONE directory contains BOTH e2fsprogs tools.
//
// Both tools are required. The appliance image build formats /perm with
// mkfs.ext4 and then injects credentials with debugfs. A one-tool probe can pass
// when the directory contains only the first tool, and the build then fails
// later in `runGokBuild` (`internal/appliance`).
func (s *Setup) probeE2fsprogs() bool {
	for _, dir := range s.e2fsprogsDirs() {
		if isFile(filepath.Join(dir, "mkfs.ext4")) && isFile(filepath.Join(dir, "debugfs")) {
			return true
		}
	}
	return false
}

// isFile reports whether path names a regular file, following symlinks the way
// Python's Path.is_file does.
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// hostGOOS is this build's operating system, named once so the seam in Setup
// has something to default to.
var hostGOOS = runtime.GOOS
