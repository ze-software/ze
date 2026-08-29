//go:build ze_core

package main

import (
	"sort"
	"strings"
	"testing"

	cli "github.com/ze-software/ze/internal/component/cli/client"
	"github.com/ze-software/ze/internal/component/command/registry"
)

// declaredVerbs answers the top-level words this binary declares a command
// under, read from the two registries a command can arrive through and from
// nothing else.
//
// It is deliberately not yangVerbs. A test that asks the implementation what it
// believes proves only that the implementation agrees with itself. This walks
// the tree and the local registry the way an operator's command reaches them,
// so the two answers can disagree and the test can say so.
func declaredVerbs(t *testing.T) []string {
	t.Helper()

	tree := cli.YANGCommandTree()
	if tree == nil {
		t.Fatal("the YANG command tree must be built")
	}

	claimed := make(map[string]bool, len(tree.Children))
	for _, root := range registry.ListRoot() {
		claimed[root.Name] = true
	}

	seen := make(map[string]bool, len(tree.Children))
	for name := range tree.Children {
		seen[name] = true
	}
	for _, entry := range registry.ListLocal() {
		verb, _, _ := strings.Cut(entry.Path, " ")
		seen[verb] = true
	}

	verbs := make([]string, 0, len(seen))
	for verb := range seen {
		if verb == "" || claimed[verb] {
			continue
		}
		verbs = append(verbs, verb)
	}
	sort.Strings(verbs)
	return verbs
}

// dispatchDefaultFlags gives this test the global flags a fresh process has,
// and gives them back when it ends.
//
// zeFlags is one variable for the whole test binary, and a sibling test leaves
// a config override in it. zeDispatch reads that override before it reads argv,
// so without this the verb under test never reaches the dispatcher at all.
func dispatchDefaultFlags(t *testing.T) {
	t.Helper()
	saved := zeFlags
	t.Cleanup(func() { zeFlags = saved })
	zeFlags = zeGlobalFlags{chaosRate: -1}
}

// VALIDATES: every top-level verb the command tree and the local registry
// declare is dispatched by zeDispatch, in any feature-tag build.
// PREVENTS: a hardcoded verb list going stale, which published six commands in
// the catalog that answered "unknown command" when an operator typed them.
//
// TestEveryDeclaredVerbDispatches proves that every verb this binary declares a
// command under reaches the dispatcher, whatever feature tags the build carries.
//
// The method is the operator's own. `ze <verb> help` goes through zeDispatch,
// which answers 0 after it writes the verb's help page and 1 after it writes
// the unknown-command banner. A hardcoded eight-entry verb map left announce,
// create, debug, peer, system and withdraw declared in YANG, published in the
// catalog, and reachable by nobody, so this asserts the whole declared
// population rather than a list somebody remembered to extend.
func TestEveryDeclaredVerbDispatches(t *testing.T) {
	ensureLocalCommandsRegistered()
	dispatchDefaultFlags(t)

	verbs := declaredVerbs(t)
	if len(verbs) < 8 {
		t.Fatalf("declared verbs = %v, want at least the eight every build carries", verbs)
	}

	for _, verb := range verbs {
		code := 0
		out := captureStderr(t, func() { code = zeDispatch([]string{verb, "help"}) })
		if code != 0 {
			t.Errorf("ze %s help = exit %d, want 0: %q is declared and nothing dispatches it", verb, code, verb)
		}
		if banner, _, _ := strings.Cut(out, "\n"); strings.Contains(banner, "unknown command") {
			t.Errorf("ze %s help wrote %q", verb, banner)
		}
	}
}

// VALIDATES: a word no registry declares still reaches the unknown-command
// banner and exit 1.
// PREVENTS: a derivation that answers true for everything, which would swallow
// every typing mistake into the command resolver and lose the did-you-mean hint.
//
// TestUndeclaredVerbFallsThroughToUsage proves the derived verb set still has
// an outside.
//
// Deriving the set is what makes this test necessary. A derivation that
// answered true for every word would satisfy the test above and route every
// typing mistake into the command resolver, where the operator gets "unknown
// show command" instead of the banner and its did-you-mean hint.
func TestUndeclaredVerbFallsThroughToUsage(t *testing.T) {
	ensureLocalCommandsRegistered()
	dispatchDefaultFlags(t)

	const undeclared = "zzznotaverb"
	for _, verb := range declaredVerbs(t) {
		if verb == undeclared {
			t.Fatalf("the fixture verb %q must not be declared", undeclared)
		}
	}

	code := 0
	out := captureStderr(t, func() { code = zeDispatch([]string{undeclared, "help"}) })
	if code != 1 {
		t.Errorf("ze %s help = exit %d, want 1", undeclared, code)
	}
	if !strings.Contains(out, "unknown command: "+undeclared) {
		t.Errorf("ze %s help must write the unknown-command banner, wrote: %s", undeclared, strings.TrimSpace(out))
	}
}
