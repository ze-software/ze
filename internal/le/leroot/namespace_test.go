// VALIDATES: dispatch resolves a command of more than one word, bounds what it
// offers the matcher, and answers a bare namespace token with its members.
// PREVENTS: a value further along the line read as a command word. A pipe
// operator read as a namespace member. A namespace token refused as a typo.
// A second resolver drifting from this one.
package leroot

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
)

// probeMeta is the metadata every probe in this file registers with.
func probeMeta(description string) registry.Meta {
	return registry.Meta{Description: description, Mode: "offline", Section: registry.SectionTest}
}

// registerProbe registers one command and reports the arguments it received.
func registerProbe(t *testing.T, name string, group Group) *[]string {
	t.Helper()
	got := new([]string)
	Register(name, group, func(args []string) (any, int) {
		*got = args
		return map[string]any{"probe": name}, 0
	}, probeMeta("a namespace probe"))
	RegisterShape(name, command.ShapeMap)
	return got
}

func TestDispatchResolvesATwoWordCommand(t *testing.T) {
	got := registerProbe(t, "probe-ns member", GroupGate)

	code := 0
	captureStdout(t, func() { code = Dispatch("le", []string{"probe-ns", "member", "run"}) })
	if code != 0 {
		t.Fatalf("Dispatch answered %d, want 0", code)
	}
	if len(*got) != 1 || (*got)[0] != "run" {
		t.Errorf("the tool received %v, want just the trailing word", *got)
	}
}

// TestDispatchPrefersTheLongerCommand covers a namespace root that is also a
// command of its own, which `verify`, `site` and `repository` all are. The
// member must win over the root when both are registered.
func TestDispatchPrefersTheLongerCommand(t *testing.T) {
	root := registerProbe(t, "probe-both", GroupWorkflow)
	member := registerProbe(t, "probe-both member", GroupGate)

	captureStdout(t, func() { Dispatch("le", []string{"probe-both", "member", "x"}) })
	if len(*member) != 1 || (*member)[0] != "x" {
		t.Errorf("the member received %v, want the trailing word", *member)
	}
	if len(*root) != 0 {
		t.Errorf("the root ran with %v; the longer command should have won", *root)
	}
}

// TestDispatchBoundsTheLookupAtTwoWords is the guard against a value being read
// as a command word. `le job run label x command le verify lint` offers nine
// words, and only the first two may ever reach the matcher.
func TestDispatchBoundsTheLookupAtTwoWords(t *testing.T) {
	deep := registerProbe(t, "probe-deep one two", GroupGate)
	shallow := registerProbe(t, "probe-deep one", GroupGate)

	captureStdout(t, func() { Dispatch("le", []string{"probe-deep", "one", "two"}) })
	if len(*deep) != 0 {
		t.Errorf("a three-word command resolved with %v; the bound is two", *deep)
	}
	if len(*shallow) != 1 || (*shallow)[0] != "two" {
		t.Errorf("the two-word command received %v, want the third word as its argument", *shallow)
	}
}

// TestDispatchKeepsThePipeWordOutOfTheLookup proves the chain is never offered
// to the matcher. A tool named after an operator would otherwise be reachable
// by typing the operator.
func TestDispatchKeepsThePipeWordOutOfTheLookup(t *testing.T) {
	got := registerProbe(t, "probe-pipe", GroupReport)

	code := 0
	captureStdout(t, func() { code = Dispatch("le", []string{"probe-pipe", "|", "json"}) })
	if code != 0 {
		t.Fatalf("Dispatch answered %d, want 0", code)
	}
	if len(*got) != 0 {
		t.Errorf("the tool received %v, want nothing: the chain is not its argument", *got)
	}
}

func TestBareNamespaceTokenListsItsMembers(t *testing.T) {
	registerProbe(t, "probe-bare alpha", GroupGate)
	registerProbe(t, "probe-bare beta", GroupGate)

	code := 0
	stderr := captureStderr(t, func() { code = Dispatch("le", []string{"probe-bare"}) })
	if code != 1 {
		t.Errorf("a bare namespace token answered %d, want 1", code)
	}
	for _, want := range []string{"alpha", "beta", "namespace"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("the refusal does not name %q: %s", want, stderr)
		}
	}
}

// TestAnUnknownCommandIsStillUnknown keeps the two refusals apart. A token that
// holds no members is a typo, and saying "namespace" about it would be a lie.
func TestAnUnknownCommandIsStillUnknown(t *testing.T) {
	code := 0
	stderr := captureStderr(t, func() { code = Dispatch("le", []string{"probe-nothing-holds-this"}) })
	if code != 1 {
		t.Errorf("an unknown command answered %d, want 1", code)
	}
	if !strings.Contains(stderr, "unknown command") {
		t.Errorf("the refusal does not say unknown command: %s", stderr)
	}
	if strings.Contains(stderr, "is a namespace") {
		t.Errorf("an unknown command was called a namespace: %s", stderr)
	}
}

func TestMembersAnswersOnlyTheTokensFamily(t *testing.T) {
	registerProbe(t, "probe-members one", GroupGate)
	registerProbe(t, "probe-members two", GroupGate)
	registerProbe(t, "probe-members-not-a-member", GroupGate)

	held := members("probe-members")
	if len(held) != 2 {
		t.Fatalf("members answered %v, want the two members alone", held)
	}
	for _, name := range held {
		if strings.Contains(name, "-") {
			t.Errorf("members answered %q; a sibling whose name merely starts the same is not a member", name)
		}
	}
}

// TestLookupCommandResolvesAWholeName is the contract the verification
// dispatcher shares with Dispatch. It reaches a command by NAME rather than
// from argv, and a namespaced stage Identity is exactly that.
func TestLookupCommandResolvesAWholeName(t *testing.T) {
	registerProbe(t, "probe-lookup member", GroupGate)

	if LookupCommand("probe-lookup member") == nil {
		t.Error("LookupCommand did not resolve a two-word name")
	}
	if LookupCommand("probe-lookup") != nil {
		t.Error("LookupCommand resolved a prefix of a command as a command")
	}
	if LookupCommand("probe-lookup member extra") != nil {
		t.Error("LookupCommand resolved a name with a trailing word")
	}
}
