// Smoke + selftest for scripts/checks/ci_dispatch_commands.go (the //go:build
// ignore dispatch-command call-site gate). The checker is ignore-tagged so it
// is excluded from normal compilation; this test gives the package a buildable
// target and runs the checker as a subprocess.
//
// What it guards: the verb-first CLI migration deleted dispatch keys (the
// dispatcher registers each builtin under its YANG PATH), and nothing checked
// the CALL SITES. Eleven emitters kept sending `peer <n> detail`, `summary`,
// `bgp health` and `daemon reload` after those keys ceased to exist, and six
// `.ci` tests passed anyway. `make ze-cli-grammar-check` proves commands are
// DECLARED verb-first; this proves the repo still SENDS commands that exist.

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoDeadDispatchCommands runs the gate against the live tree and asserts
// every command string the repo sends still resolves to a registered command.
//
// VALIDATES: no .ci/.py/.go emitter dispatches a command the daemon answers
// with ErrUnknownCommand.
// PREVENTS: a command-tree migration silently orphaning its callers again.
func TestNoDeadDispatchCommands(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", "-tags", featureTags(t), "scripts/checks/ci_dispatch_commands.go")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ci-dispatch gate failed (an emitter sends a command that resolves nowhere, or a command string could not be read statically):\n%s", out)
	}
	if !strings.Contains(string(out), "ci-dispatch-check: OK") {
		t.Fatalf("ci_dispatch_commands.go did not report OK:\n%s", out)
	}
}

// TestCIDispatchCommandsSelftest proves the gate can actually fail: the live
// command surface loads, every removed spelling is rejected while its verb-first
// replacement resolves, a dead literal in a fixture is found, a non-literal
// command is reported rather than skipped, and the documented dynamic marker
// exempts a genuinely computed emitter.
//
// VALIDATES: the resolver and the emitter recogniser both discriminate.
// PREVENTS: the gate passing vacuously -- a checker that resolves everything,
// or one whose regexes match nothing, reads identical to a clean tree.
func TestCIDispatchCommandsSelftest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", "-tags", featureTags(t), "scripts/checks/ci_dispatch_commands.go", "--selftest")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ci-dispatch --selftest failed:\n%s", out)
	}
	if !strings.Contains(string(out), "ci-dispatch-check selftest: OK") {
		t.Fatalf("ci_dispatch_commands.go --selftest did not report OK:\n%s", out)
	}
}

// featureTags derives the build tags the gate needs from feature-gates.txt, the
// single source of truth for compile-out-able features. The gate ENUMERATES the
// live command registry, so it must see the same feature set the shipped binary
// has -- with ze_bgp off, `show bgp summary` is simply not registered and every
// legitimate use of it would report as dead. This mirrors the Makefile's
// GO_TEST_TAGS derivation rather than restating the list
// (ai/rules/plugins.md: no consumer is hand-maintained).
func featureTags(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "feature-gates.txt"))
	if err != nil {
		t.Fatalf("read feature-gates.txt: %v", err)
	}
	tags := []string{"ze_core"}
	seen := map[string]bool{"ze_core": true}
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || !strings.HasPrefix(fields[0], "ze_") {
			continue
		}
		if seen[fields[0]] {
			continue
		}
		seen[fields[0]] = true
		tags = append(tags, fields[0])
	}
	if len(tags) < 2 {
		t.Fatal("feature-gates.txt yielded no ze_* tags; the gate would run without any feature")
	}
	return strings.Join(tags, " ")
}
