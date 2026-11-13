// Design: docs/guide/config-editor.md -- fixes work from any editor position.
// Related: validator.go -- validateWithYANG; config/required.go -- CheckRequired.

package cli

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

const requiredReplayProbe = `module ze-required-replay-probe-conf {
  yang-version 1.1;
  namespace "urn:ze:required-replay-probe";
  prefix probe;
  import ze-extensions { prefix ze; }
  container required-replay {
    list target {
      key "name second";
      ze:required "address";
      leaf name { type string; }
      leaf second { type string; }
      leaf address { type string; }
    }
    list site {
      key "name second";
      leaf name { type string; }
      leaf second { type string; }
      list target {
        key "name";
        ze:required "address";
        leaf name { type string; }
        leaf address { type string; }
      }
    }
    list single {
      key "name";
      ze:required "address";
      leaf name { type string; }
      leaf address { type string; }
    }
    list single-site {
      key "name";
      leaf name { type string; }
      list target {
        key "name";
        ze:required "address";
        leaf name { type string; }
        leaf address { type string; }
      }
    }
  }
}`

// TestPrintedRequiredFixPreservesCompositeEntry replays the actual commit
// warning's fix against its original entry, at the root and in an edit context.
func TestPrintedRequiredFixPreservesCompositeEntry(t *testing.T) {
	t.Cleanup(configyang.RegisterModuleForTest("ze-required-replay-probe-conf.yang", requiredReplayProbe))
	replayRequiredWarningFix(t,
		"required-replay { target alpha { second beta; } }",
		"set required-replay target alpha second beta address <value>",
		[]string{"target", "alpha second beta"},
	)
}

// TestPrintedRequiredFixPreservesCompositeAncestor replays a nested required
// field's warning without changing the ancestor entry's canonical identity.
func TestPrintedRequiredFixPreservesCompositeAncestor(t *testing.T) {
	t.Cleanup(configyang.RegisterModuleForTest("ze-required-replay-probe-conf.yang", requiredReplayProbe))
	replayRequiredWarningFix(t,
		"required-replay { site alpha { second beta; target one {} } }",
		"set required-replay site alpha second beta target one address <value>",
		[]string{"site", "alpha second beta", "target", "one"},
	)
}

// TestPrintedRequiredFixPreservesSingleKey replays the warning for a single
// key containing a space, which must remain one quoted command argument.
func TestPrintedRequiredFixPreservesSingleKey(t *testing.T) {
	t.Cleanup(configyang.RegisterModuleForTest("ze-required-replay-probe-conf.yang", requiredReplayProbe))
	replayRequiredWarningFix(t,
		`required-replay { single "edge routers" {} }`,
		`set required-replay single "edge routers" address <value>`,
		[]string{"single", "edge routers"},
	)
}

// TestPrintedRequiredSetHintPreservesLiteralPipe replays the standalone
// RequiredViolation.SetHint, so a literal pipe key cannot become an operator.
func TestPrintedRequiredSetHintPreservesLiteralPipe(t *testing.T) {
	t.Cleanup(configyang.RegisterModuleForTest("ze-required-replay-probe-conf.yang", requiredReplayProbe))
	m, _ := topTestModel(t)
	fixture, err := config.NewParser(m.editor.schema).Parse(`required-replay { single "|" {} }`)
	if err != nil {
		t.Fatal(err)
	}
	m.editor.tree.SetContainer("required-replay", fixture.GetContainer("required-replay"))
	violations := config.CheckRequired(m.editor.schema, fixture.ToMap())
	if len(violations) != 1 {
		t.Fatalf("violations = %v, want one", violations)
	}
	printed := violations[0].SetHint
	if strings.Count(printed, "<value>") != 1 {
		t.Fatalf("hint must hold exactly one value placeholder: %q", printed)
	}
	if printed != `set required-replay single "|" address <value>` {
		t.Errorf("printed hint = %q", printed)
	}
	line := strings.Replace(printed, "<value>", "ready", 1)
	m = enterLine(t, m, line)
	if m.err != nil {
		t.Fatalf("printed hint %q did not execute: %v", line, m.err)
	}
	entries := m.editor.tree.GetContainer("required-replay").GetList("single")
	if len(entries) != 1 {
		t.Fatalf("single has %d entries, want the original entry only", len(entries))
	}
	entry := entries["|"]
	if entry == nil {
		t.Fatal("the original literal-pipe entry disappeared")
	}
	value, ok := entry.Get("address")
	if !ok {
		t.Error("the original entry address is absent")
	}
	if value != "ready" {
		t.Errorf("the original entry address = %q, want ready", value)
	}
	m, result := enterLineResult(t, m, "commit verify")
	if m.err != nil {
		t.Fatal(m.err)
	}
	if result.statusMessage != "commit verify: the candidate is valid; nothing was applied" {
		t.Errorf("warning survived the standalone hint: %s\n%s", result.statusMessage, result.output)
	}
}

// TestPrintedRequiredSetHintPreservesNumericSuffix replays the raw hint from
// the parsed tree and checks stored identity without serializing that tree.
func TestPrintedRequiredSetHintPreservesNumericSuffix(t *testing.T) {
	t.Cleanup(configyang.RegisterModuleForTest("ze-required-replay-probe-conf.yang", requiredReplayProbe))
	for _, tc := range []struct {
		name      string
		content   string
		entryPath []string
		hint      string
	}{
		{
			name: "single entry", content: `required-replay { single "edge#3" {} }`,
			entryPath: []string{"single", "edge#3"},
			hint:      `set required-replay single "edge#3" address <value>`,
		},
		{
			name: "composite entry", content: `required-replay { target alpha { second "beta#3"; } }`,
			entryPath: []string{"target", "alpha second beta#3"},
			hint:      `set required-replay target alpha second "beta#3" address <value>`,
		},
		{
			name: "single ancestor", content: `required-replay { single-site "edge#3" { target one {} } }`,
			entryPath: []string{"single-site", "edge#3", "target", "one"},
			hint:      `set required-replay single-site "edge#3" target one address <value>`,
		},
		{
			name: "composite ancestor", content: `required-replay { site alpha { second "beta#3"; target one {} } }`,
			entryPath: []string{"site", "alpha second beta#3", "target", "one"},
			hint:      `set required-replay site alpha second "beta#3" target one address <value>`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := topTestModel(t)
			fixture, err := config.NewParser(m.editor.schema).Parse(tc.content)
			if err != nil {
				t.Fatal(err)
			}
			m.editor.tree.SetContainer("required-replay", fixture.GetContainer("required-replay"))
			violations := config.CheckRequired(m.editor.schema, fixture.ToMap())
			if len(violations) != 1 {
				t.Fatalf("violations = %v, want one", violations)
			}
			printed := violations[0].SetHint
			if printed != tc.hint {
				t.Errorf("printed hint = %q, want %q", printed, tc.hint)
			}
			if strings.Count(printed, "<value>") != 1 {
				t.Fatalf("hint must hold exactly one value placeholder: %q", printed)
			}
			line := strings.Replace(printed, "<value>", "ready", 1)
			if _, err := m.dispatchCommand(line); err != nil {
				t.Fatalf("printed hint %q did not execute: %v", line, err)
			}
			entry := m.editor.tree.GetContainer("required-replay")
			for i := 0; i < len(tc.entryPath); i += 2 {
				entries := entry.GetList(tc.entryPath[i])
				if len(entries) != 1 {
					t.Fatalf("list %s has %d entries, want the original entry only", tc.entryPath[i], len(entries))
				}
				entry = entries[tc.entryPath[i+1]]
				if entry == nil {
					t.Fatalf("original entry %q disappeared", tc.entryPath[i+1])
				}
			}
			value, ok := entry.Get("address")
			if !ok {
				t.Error("the original entry address is absent")
			}
			if value != "ready" {
				t.Errorf("the original entry address = %q, want ready", value)
			}
		})
	}
}

// replayRequiredWarningFix substitutes only the operator's missing value in
// the warning printed by commit verify, then enters that exact command.
func replayRequiredWarningFix(t *testing.T, content, wantFix string, entryPath []string) {
	t.Helper()
	for _, nested := range []bool{false, true} {
		t.Run("context="+strconv.FormatBool(nested), func(t *testing.T) {
			m, _ := topTestModel(t)
			fixture, err := config.NewParser(m.editor.schema).Parse(content)
			if err != nil {
				t.Fatal(err)
			}
			m.editor.tree.SetContainer("required-replay", fixture.GetContainer("required-replay"))
			if nested {
				m = enterLine(t, m, "edit bgp peer peer1")
				if m.err != nil {
					t.Fatal(m.err)
				}
			}
			position := slices.Clone(m.contextPath)
			m, result := enterLineResult(t, m, "commit verify")
			if m.err != nil {
				t.Fatal(m.err)
			}
			_, tail, found := strings.Cut(result.output, `missing required field "address" (`)
			if !found {
				t.Fatalf("commit verify did not print the required warning: %s", result.output)
			}
			printed, _, found := strings.Cut(tail, ")")
			if !found {
				t.Fatalf("warning has no complete fix: %s", result.output)
			}
			if strings.Count(printed, "<value>") != 1 {
				t.Fatalf("fix must hold exactly one value placeholder: %q", printed)
			}
			want := wantFix
			if nested {
				want = "top " + want
			}
			if printed != want {
				t.Errorf("printed fix = %q, want %q", printed, want)
			}
			line := strings.Replace(printed, "<value>", "ready", 1)
			m = enterLine(t, m, line)
			if m.err != nil {
				t.Fatalf("printed fix %q did not execute: %v", line, m.err)
			}
			if !slices.Equal(m.contextPath, position) {
				t.Errorf("context = %q, want %q", m.contextPath, position)
			}
			entry := m.editor.tree.GetContainer("required-replay")
			for i := 0; i < len(entryPath); i += 2 {
				entries := entry.GetList(entryPath[i])
				if len(entries) != 1 {
					t.Fatalf("list %s has %d entries, want the original entry only", entryPath[i], len(entries))
				}
				entry = entries[entryPath[i+1]]
				if entry == nil {
					t.Fatalf("original entry %q disappeared", entryPath[i+1])
				}
			}
			value, ok := entry.Get("address")
			if !ok {
				t.Error("original entry address is absent")
			}
			if value != "ready" {
				t.Errorf("original entry address = %q, want ready", value)
			}
			m, result = enterLineResult(t, m, "commit verify")
			if m.err != nil {
				t.Fatal(m.err)
			}
			if result.statusMessage != "commit verify: the candidate is valid; nothing was applied" {
				t.Errorf("warning survived the printed fix: %s\n%s", result.statusMessage, result.output)
			}
		})
	}
}
