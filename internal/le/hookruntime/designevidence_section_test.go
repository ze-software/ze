// Design: docs/contributing/spec-workflow.md -- the sections the design gate reads, and a name under a file
// VALIDATES: the design gate reads a section only from a heading that opens its
// line, so `### Current Behavior` under a historical record and prose quoting
// `## Current Behavior` select nothing while the spec's own section is still
// judged; and a name under a root-level regular file is judged as written: the
// GitHub check name `ze/release-evidence-v1` under the `ze` launcher names no
// producer, and a source name under it is package-relative shorthand, while a
// name that climbs out of the checkout still refuses; the daemon-proof gate
// classifies the same names by their spelling.
// PREVENTS: every edit of spec-release-distribution.md being refused because a
// check name in its historical Data Flow was read as Current Behavior and its
// resolution failed with ENOTDIR (plan/learned/HOOK-FRICTION.md, 2026-11-15).
package hookruntime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	underFileLauncher = "ze"
	underFileTracked  = "gokrazy/ze/probe.go"
	neverHeldProducer = "internal/le/never_held.go"
)

// underFileFixture is a checkout whose root holds the regular file ze, whose
// one commit tracks underFileTracked, and in which evidenceProbeB is read in
// this context.
func underFileFixture(t *testing.T) string {
	t.Helper()
	root := gitFixture(t)
	writeHookFixture(t, root, underFileLauncher, "#!/bin/sh\n")
	writeHookFixture(t, root, underFileTracked, shorthandContent)
	for _, args := range [][]string{{"add", "--", underFileTracked}, {"commit", "-q", "-m", "add probe"}} {
		command := exec.CommandContext(t.Context(), "git", append([]string{"-C", root, "-c", "user.name=probe", "-c", "user.email=probe@example.invalid"}, args...)...)
		command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	writeHookFixture(t, root, evidenceProbeB, "package other\n")
	recordObservedRead(t, root, filepath.Join(root, evidenceProbeB), "package other\n", 1, 2, 2)
	return root
}

// TestDesignEvidenceReadsOnlyAHeadingThatOpensItsLine checks that a path git
// never held refuses under the spec's own `## Current Behavior`, and is not
// judged under a deeper `### Current Behavior` or after prose that quotes the
// heading.
//
// MUTATION: (internal/le/hookruntime/lifecycle.go, "if !strings.HasPrefix(trimmed, heading) {", "if !strings.Contains(line, heading) {")
// turns the historical-record and quoted-in-prose rows red: the path is judged again.
// MUTATION: (internal/le/hookruntime/lifecycle.go, "if !strings.HasPrefix(trimmed, heading) {", "if trimmed != heading {")
// turns the own-section-with-suffix row red: the template's `(MANDATORY)` heading opens nothing.
func TestDesignEvidenceReadsOnlyAHeadingThatOpensItsLine(t *testing.T) {
	cited := "- [ ] `" + neverHeldProducer + "` - planned\n"
	for _, test := range []struct {
		name string
		text string
		want int
	}{
		{"own-section", "## Current Behavior\n" + cited, 2},
		{"own-section-with-suffix", "## Current Behavior (MANDATORY)\n" + cited, 2},
		{"historical-record", "## Historical source record\n\n### Current Behavior (MANDATORY)\n" + cited, 0},
		{"quoted-in-prose", "## Task\nAdd the table between `## Current Behavior` and `## Data Flow`.\n" + cited, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := underFileFixture(t)
			message := observedDesignGate(t, root, test.text, test.want)
			if test.want == 0 {
				return
			}
			if !strings.Contains(message, neverHeldProducer+": inspect producer") || !strings.Contains(message, "git history never deleted it") {
				t.Fatalf("the spec's own Current Behavior did not judge its path: %s", message)
			}
		})
	}
}

// TestDesignEvidenceJudgesANameUnderAFileAsWritten checks that a name whose
// ancestor is the root-level ze file is no resolution failure: a name that is
// no source kind is skipped, and a source name is matched as shorthand, so the
// tracked gokrazy/ze/probe.go must be read before the edit lands.
//
// A name that climbs out of the checkout to a file there keeps its refusal.
//
// MUTATION: (internal/le/hookruntime/designevidence.go, "if !errors.Is(err, syscall.ENOTDIR) {", "if true {")
// turns the check-name and shorthand rows red: each name refuses as a producer that cannot be resolved.
// MUTATION: (internal/le/hookruntime/designevidence.go, "if !insideRoot(spelled) {", "if false {")
// turns the climbs-out row red: a name outside the checkout is skipped as no source kind.
func TestDesignEvidenceJudgesANameUnderAFileAsWritten(t *testing.T) {
	t.Run("climbs-out", func(t *testing.T) {
		root := underFileFixture(t)
		writeHookFixture(t, filepath.Dir(root), "outside", "not a directory\n")
		message := observedDesignGate(t, root, "## Current Behavior\nThe `../outside/check-v1` name.\n", 2)
		if !strings.Contains(message, "../outside/check-v1: resolve producer") {
			t.Fatalf("a name outside the checkout is not refused for confinement: %s", message)
		}
	})
	t.Run("check-name", func(t *testing.T) {
		root := underFileFixture(t)
		observedDesignGate(t, root, "## Current Behavior\nThe `ze/release-evidence-v1` commit check.\n", 0)
	})
	t.Run("source-name-is-shorthand", func(t *testing.T) {
		root := underFileFixture(t)
		named := "## Current Behavior\n- [ ] `ze/probe.go` - the probe\n"
		message := observedDesignGate(t, root, named, 2)
		if !strings.Contains(message, "Missing current subject evidence: "+underFileTracked) {
			t.Fatalf("a source name under the ze file is not read as shorthand: %s", message)
		}
		recordObservedRead(t, root, filepath.Join(root, underFileTracked), shorthandContent, 1, 2, 2)
		observedDesignGate(t, root, named, 0)
	})
}

// TestDaemonProofJudgesANameUnderAFileByItsSpelling checks the proof gate on
// the same names: the check name under the ze file owes no proof, a source
// name under it is judged as daemon code from the checkout root, and a name
// that climbs out of the checkout still refuses as unresolvable.
//
// MUTATION: (internal/le/hookruntime/specproof.go, "path, err := checkoutRelativeBelowFile(context{root: root, path: named})", "path, err := checkoutRelative(context{root: root, path: named})")
// turns the check-name and source-name rows red: each refuses as a changed source that cannot be resolved.
func TestDaemonProofJudgesANameUnderAFileByItsSpelling(t *testing.T) {
	root := t.TempDir()
	writeHookFixture(t, root, underFileLauncher, "#!/bin/sh\n")
	writeHookFixture(t, filepath.Dir(root), "outside", "not a directory\n")
	for _, test := range []struct {
		name  string
		named string
		want  string
	}{
		{"check-name", "ze/release-evidence-v1", ""},
		{"source-name", "ze/probe.go", "Judged as daemon code, from the checkout root: ze/probe.go."},
		{"climbs-out", "../outside/check-v1", "Daemon-path proof cannot resolve changed source ../outside/check-v1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			problem := daemonProofProblem(root, resolutionHeader+"## Files to Modify\n`"+test.named+"`\n")
			if test.want == "" {
				if problem != "" {
					t.Fatalf("%s owes no proof, got: %s", test.named, problem)
				}
				return
			}
			if !strings.Contains(problem, test.want) {
				t.Fatalf("%s: want %q, got: %s", test.named, test.want, problem)
			}
		})
	}
}
