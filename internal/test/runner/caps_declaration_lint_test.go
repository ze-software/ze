package runner

// VALIDATES: every .ci that runs a privileged iproute2 command declares
//   caps=net-admin, so an unprivileged Linux host SKIPS it with a reason instead
//   of running it and failing.
// PREVENTS: the suite going red for something that is not a defect. Without the
//   declaration the parser records no skip (record_parse.go, case
//   "needs-linux"), so the test runs. `ip link add` is refused for want of
//   CAP_NET_ADMIN, and the daemon never reaches the state the file asserts. The
//   red then names a route or an interface, never the capability, so the reader
//   diagnoses a product defect that is not there.
//
// Nine files carried it on 2026-08-23: the seven in test/static and two in
// test/vrrp. Each creates its devices from a `tmpfs=setup.py` and each declared
// `option=needs-linux` alone.
//
// THE BOUND, so nobody reads more into a green than it says: this check sees
// iproute2 mutations and nothing else. `nft`, `tc`, `sysctl -w` and a daemon
// that opens a raw socket all need capabilities too. A file that needs net-raw
// as well as net-admin is not distinguishable from here. Declaring one
// capability a test really needs is always better than declaring none, and it is
// not a claim that the declaration is complete.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// privilegedIPShell matches a shell-spelled iproute2 mutation: `ip link add`,
// `ip -6 route replace`, and the rest. Read-only forms (`ip link show`) are not
// matched, because reading needs no capability.
//
// The `^[^#\n]*` prefix is what keeps a comment out. It cannot cross a `#`, so
// a line that only DISCUSSES the command never matches. The .ci files discuss it
// often, and the declaration this check enforces is usually explained right
// above the invocation it applies to.
var privilegedIPShell = regexp.MustCompile(
	`(?m)^[^#\n]*\bip\s+(?:-[46]\s+)?(?:link|addr|address|route|rule|neigh|netns)\s+` +
		`(?:add|set|del|delete|replace|change|append)\b`)

// privilegedIPCall matches the same mutations spelled as the one-line Python
// helper every setup.py in this corpus defines: `def ip(*args)` wrapping
// subprocess, then `ip("link", "add", ...)`. Without this arm the check would
// see none of the files it was written for. Not one of them spells the command
// as a shell line.
var privilegedIPCall = regexp.MustCompile(
	`(?m)^[^#\n]*\bip\(\s*"(?:link|addr|address|route|rule|neigh|netns)"\s*,\s*` +
		`"(?:add|set|del|delete|replace|change|append)"`)

// privilegedIPLines returns each line of raw that runs a privileged iproute2
// command, in file order.
func privilegedIPLines(raw string) []string {
	var out []string
	for line := range strings.SplitSeq(raw, "\n") {
		if privilegedIPShell.MatchString(line) || privilegedIPCall.MatchString(line) {
			out = append(out, strings.TrimSpace(line))
		}
	}
	return out
}

// declaresNetAdmin reports whether raw carries `option=needs-linux` with
// net-admin among its caps.
//
// It reads the OPTION LINE rather than searching the file for the token. The
// token also appears in the prose that explains the declaration, so a file that
// only talks about net-admin would otherwise satisfy this check. That is the
// same shape as an assertion satisfied by a mention.
func declaresNetAdmin(raw string) bool {
	for line := range strings.SplitSeq(raw, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "option=needs-linux") {
			continue
		}
		for field := range strings.SplitSeq(line, ":") {
			caps, ok := strings.CutPrefix(field, "caps=")
			if !ok {
				continue
			}
			for c := range strings.SplitSeq(caps, ",") {
				if strings.TrimSpace(c) == capsNetAdmin {
					return true
				}
			}
		}
	}
	return false
}

// undeclaredPrivilegedIP returns one finding per file in corpus that runs a
// privileged iproute2 command and does not declare net-admin.
func undeclaredPrivilegedIP(corpus map[string]string) []string {
	var out []string
	for name, raw := range corpus {
		lines := privilegedIPLines(raw)
		if len(lines) == 0 || declaresNetAdmin(raw) {
			continue
		}
		out = append(out, fmt.Sprintf("%s runs %q", name, lines[0]))
	}
	sort.Strings(out)
	return out
}

// TestCIPrivilegedIPDeclaresNetAdmin walks the .ci corpus and fails when a test
// configures the kernel's network but does not declare the capability it needs.
func TestCIPrivilegedIPDeclaresNetAdmin(t *testing.T) {
	root := repoRootForTest(t)
	testDir := filepath.Join(root, "test")

	corpus := map[string]string{}
	walkErr := filepath.WalkDir(testDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// test/draft/ is invisible to every gate (test/draft/README.md).
		if d.IsDir() && isDraftPath(testDir, p) {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(p, ".ci") {
			return nil
		}
		raw, readErr := os.ReadFile(p) //nolint:gosec // repo test tree
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			rel = p
		}
		corpus[filepath.ToSlash(rel)] = string(raw)
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk %s: %v", testDir, walkErr)
	}

	// A corpus that carries no privileged command at all would make every
	// assertion below vacuous. That is exactly how this check would rot: a
	// helper rename, or a walk that stops finding files, leaves it green
	// forever. Three test/plugin files declared net-admin correctly before this
	// check existed, so the population is never legitimately empty.
	privileged := 0
	for _, raw := range corpus {
		if len(privilegedIPLines(raw)) > 0 {
			privileged++
		}
	}
	if privileged == 0 {
		t.Fatal("no .ci in the corpus runs a privileged iproute2 command: the walk or " +
			"the detector changed, and this check would pass vacuously")
	}

	if missing := undeclaredPrivilegedIP(corpus); len(missing) > 0 {
		t.Errorf("%d .ci file(s) run a privileged iproute2 command without "+
			"declaring `option=needs-linux:caps=net-admin`.\n"+
			"On an unprivileged Linux host each one FAILS instead of skipping, and the "+
			"failure names a route or an interface rather than the missing capability:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// TestPrivilegedIPDetectorDiscriminates pins the detector itself.
//
// The corpus walk above can only report what the detector sees. A detector that
// saw nothing would report a clean corpus, and that reads exactly like a
// genuinely clean corpus. These fixtures are the difference between the two.
func TestPrivilegedIPDetectorDiscriminates(t *testing.T) {
	const declared = "option=needs-linux:caps=net-admin\n" +
		"tmpfs=setup.py:mode=755:terminator=EOF_SETUP\n" +
		"ip(\"link\", \"add\", \"zens0\", \"type\", \"dummy\")\n"
	const undeclared = "option=needs-linux\n" +
		"tmpfs=setup.py:mode=755:terminator=EOF_SETUP\n" +
		"ip(\"link\", \"add\", \"zens0\", \"type\", \"dummy\")\n"
	const shell = "option=needs-linux\ncmd=ip link add zens0 type dummy\n"
	const readOnly = "option=needs-linux\ncmd=ip link show zens0\n"
	// The declaration named in prose only. The check must not accept it: a
	// mention is not a declaration.
	const mentioned = "# needs caps=net-admin one day\noption=needs-linux\n" +
		"ip(\"addr\", \"add\", \"10.0.0.1/24\", \"dev\", \"zens0\")\n"
	// The command named in a comment only. The check must not flag it.
	const commented = "option=needs-linux\n# setup.py would run ip link add zens0\n"
	// A different capability, declared. net-admin is still absent.
	const otherCap = "option=needs-linux:caps=bpf\nip(\"link\", \"set\", \"zens0\", \"up\")\n"

	for _, tc := range []struct {
		name string
		raw  string
		want bool // want a finding
	}{
		{"declared", declared, false},
		{"undeclared-python-call", undeclared, true},
		{"undeclared-shell", shell, true},
		{"read-only-needs-nothing", readOnly, false},
		{"declared-in-prose-only", mentioned, true},
		{"named-in-a-comment-only", commented, false},
		{"a-different-capability", otherCap, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := undeclaredPrivilegedIP(map[string]string{"probe.ci": tc.raw})
			if (len(got) > 0) != tc.want {
				t.Errorf("undeclaredPrivilegedIP = %v, want a finding: %v", got, tc.want)
			}
		})
	}
}
