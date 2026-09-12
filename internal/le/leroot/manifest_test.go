// VALIDATES: le's whole command surface is one payload, and its text rendering
// is byte for byte the root help a reader sees today.
// PREVENTS: a root help written beside the payload rather than derived from it,
// and an area that is reachable but unpublished.
package leroot

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leaction"
)

// manifestProbeRoots is a fixed command set: one command in each of two groups,
// and one that named no group at all. The text rendering of these three is
// deterministic, which a manifest built from the live registry is not, because
// every test in this package registers probes into it.
func manifestProbeRoots(t *testing.T) []registry.RootCommand {
	t.Helper()
	setGroup("manifest-probe-alpha", GroupWorkflow)
	setGroup("manifest-probe-gate", GroupGate)
	return []registry.RootCommand{
		{Name: "manifest-probe-alpha", Meta: registry.Meta{Description: "the first probe"}},
		{Name: "manifest-probe-gate", Meta: registry.Meta{Description: "the second probe"}},
		{Name: "manifest-probe-loose", Meta: registry.Meta{Description: "a probe that named no group"}},
	}
}

// rootHelpToday is what the root help printed for manifestProbeRoots before the
// manifest existed, captured from helpfmt through the sections builder on
// 2026-09-12. A reader's page is what this spec preserves, so the bytes are
// pinned rather than described.
const rootHelpToday = "le - the Ze repository and development entry point\n" +
	"\n" +
	"Usage:\n" +
	"  le <command> [options] [| json | yaml | table]\n" +
	"\n" +
	"Workflow (you type these while working):\n" +
	"  manifest-probe-alpha the first probe\n" +
	"\n" +
	"Gates (judge the tree, answer a verdict):\n" +
	"  manifest-probe-gate the second probe\n" +
	"\n" +
	"Ungrouped:\n" +
	"  manifest-probe-loose a probe that named no group\n" +
	"\n"

func TestManifestTextIsTheRootHelpAReaderSeesToday(t *testing.T) {
	text := manifestFrom("le", manifestProbeRoots(t)).Text()
	if text != rootHelpToday {
		t.Errorf("the manifest renders\n%q\nwant the page a reader sees today\n%q", text, rootHelpToday)
	}

	// Usage is the same page on stderr, so the refusal a reader meets and the
	// payload the root answers cannot describe two different command sets.
	page := captureStderr(t, func() { Usage("le") })
	if page != manifestOf("le").Text() {
		t.Errorf("Usage renders\n%q\nand the manifest renders\n%q", page, manifestOf("le").Text())
	}
}

func TestManifestNamesEveryRegisteredAreaAndItsGroup(t *testing.T) {
	area := leaction.New("manifest-area-probe", leaction.Action{
		Verb: "run", Why: "run the probe over one scope",
		Parameters: []leaction.Parameter{
			{Keyword: "scope", Value: "packages", Requirement: leaction.Required},
			{Keyword: "timeout", Value: "duration", Requirement: leaction.Optional},
		},
		AnswerArgs: func(leaction.Arguments) (any, int) { return nil, 0 },
	})
	Register(area.Name(), GroupSuite, area.Answer, registry.Meta{
		Description: "an area that declares its actions",
		Mode:        "offline", Section: registry.SectionTest,
	})
	RegisterActions(area.Name(), area.Actions)

	manifest := manifestOf("le")
	if manifest.Program != "le" {
		t.Errorf("the manifest names the program %q, want le", manifest.Program)
	}
	if len(manifest.Areas) < 1 {
		t.Fatal("the manifest names no area at all")
	}

	var found *ManifestArea
	for index, published := range manifest.Areas {
		if published.Name == area.Name() {
			found = &manifest.Areas[index]
			break
		}
	}
	if found == nil {
		t.Fatalf("the manifest does not name %q", area.Name())
	}
	if found.Group != GroupSuite {
		t.Errorf("the area's group is %q, want %q", found.Group, GroupSuite)
	}
	if found.Description != "an area that declares its actions" {
		t.Errorf("the area's description is %q", found.Description)
	}
	if len(found.Actions) != 1 || found.Actions[0].Verb != "run" {
		t.Fatalf("the area publishes %v, want its one action", found.Actions)
	}
	if len(found.Actions[0].Parameters) != 2 {
		t.Fatalf("the action publishes %v, want both keywords", found.Actions[0].Parameters)
	}
	if found.Actions[0].Parameters[0].Requirement != leaction.Required {
		t.Error("the manifest drops the requiredness the action declared")
	}

	// A-2: every le payload encodes, and the keys are the ones a reader of
	// `./le | json` reads.
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("the manifest does not encode: %v", err)
	}
	for _, key := range []string{`"program"`, `"summary"`, `"areas"`, `"name"`, `"group"`,
		`"description"`, `"actions"`, `"parameters"`, `"keyword"`, `"required"`, `"repeat"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("the encoded manifest carries no %s key", key)
		}
	}
}
