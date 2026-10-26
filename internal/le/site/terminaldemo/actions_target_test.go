package siteterminaldemo

import (
	"slices"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// actionRow answers the published row of verb, so a test reads the grammar a
// reader of `le site terminal-demo help` sees, never a second copy of it.
func actionRow(t *testing.T, verb string) leaction.Row {
	t.Helper()
	for _, row := range Actions().Actions {
		if row.Verb == verb {
			return row
		}
	}
	t.Fatalf("le %s holds no action %s", area, verb)
	return leaction.Row{}
}

// targetParameter answers the one declared parameter of verb that carries the
// target value slot.
func targetParameter(t *testing.T, verb string) leaction.Parameter {
	t.Helper()
	for _, parameter := range actionRow(t, verb).Parameters {
		if parameter.Value == targetValue {
			return parameter
		}
	}
	t.Fatalf("%s declares no %q parameter", verb, targetValue)
	return leaction.Parameter{}
}

// sortedKeys answers the targets a dispatch map answers, sorted.
func sortedKeys(targets map[string]func() (any, int)) []string {
	keys := make([]string, 0, len(targets))
	for key := range targets {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// VALIDATES: check declares exactly the targets artifacts, release and
// validation, and build exactly binaries and image, each as a required enum in
// the positional slot; the dispatch map behind each action answers exactly the
// declared targets. Method: read the published action rows and compare their
// declared Values with the fixed sets and with the keys of checkTargets and
// buildTargets.
// PREVENTS: a target dropped from the grammar (release, which is what a tag
// ships, no longer reachable), a target the grammar accepts that has no answer
// and so reaches targetAnswer's BUG panic, and a target that runs without
// being named.
func TestCheckAndBuildDeclareExactlyTheirTargets(t *testing.T) {
	cases := []struct {
		verb    string
		want    []string
		targets map[string]func() (any, int)
	}{
		{"check", []string{"artifacts", "release", "validation"}, checkTargets},
		{"build", []string{"binaries", "image"}, buildTargets},
	}
	for _, tc := range cases {
		parameter := targetParameter(t, tc.verb)
		if parameter.Keyword != "" {
			t.Errorf("%s takes its target behind the keyword %q, want the positional slot", tc.verb, parameter.Keyword)
		}
		if parameter.Kind != leaction.KindEnum {
			t.Errorf("%s target kind = %v, want enum", tc.verb, parameter.Kind)
		}
		if parameter.Requirement != leaction.Required {
			t.Errorf("%s target requirement = %v, want Required", tc.verb, parameter.Requirement)
		}
		declared := slices.Clone(parameter.Values)
		slices.Sort(declared)
		if !slices.Equal(declared, tc.want) {
			t.Errorf("%s declares the targets %v, want %v", tc.verb, declared, tc.want)
		}
		if answered := sortedKeys(tc.targets); !slices.Equal(answered, declared) {
			t.Errorf("%s answers the targets %v, but declares %v", tc.verb, answered, declared)
		}
	}
}

// VALIDATES: check and build refuse a line that names no target, or a target
// they do not declare, with exit 2 and no payload, before any renderer or
// builder runs. Method: dispatch each line through Answer, the entry point le
// calls.
// PREVENTS: a missing target defaulting to one property or artifact, and an
// undeclared word reaching targetAnswer, whose panic would crash le.
func TestCheckAndBuildRefuseAMissingOrUndeclaredTarget(t *testing.T) {
	for _, args := range [][]string{
		{"check"},
		{"check", "bogus"},
		{"build"},
		{"build", "bogus"},
	} {
		payload, code := Answer(args)
		if code != 2 {
			t.Errorf("%v answered code %d, want 2", args, code)
		}
		if payload != nil {
			t.Errorf("%v answered the payload %v", args, payload)
		}
	}
}

// VALIDATES: every declared check and build target is accepted by the parser
// and read back as the target. Method: Parse, which reads a line exactly as
// Answer does and runs nothing.
// PREVENTS: a target listed in help that the parser then refuses.
func TestEveryDeclaredTargetParses(t *testing.T) {
	table := actionTable()
	for _, verb := range []string{"check", "build"} {
		for _, target := range targetParameter(t, verb).Values {
			parsed, err := table.Parse(verb, []string{target})
			if err != nil {
				t.Errorf("%s %s is refused: %v", verb, target, err)
				continue
			}
			if got := parsed.One(targetValue); got != target {
				t.Errorf("%s %s reads the target %q", verb, target, got)
			}
		}
	}
}

// VALIDATES: render declares the all switch, and the parser accepts `render
// all` alone as that switch with no demo named. Method: read the published
// render row, then Parse the line, which runs no render.
// PREVENTS: the all switch being removed while TestRenderAllWithADemoIdIsRefused
// stays green, because the parser then refuses `all` as an unknown word with
// the same exit 2, so `render all` no longer records the gallery and no test
// says so.
func TestRenderAllAloneIsAccepted(t *testing.T) {
	var declared bool
	for _, parameter := range actionRow(t, "render").Parameters {
		if parameter.Keyword != renderAll {
			continue
		}
		declared = true
		if parameter.Kind != leaction.KindSwitch {
			t.Errorf("render %s kind = %v, want switch", renderAll, parameter.Kind)
		}
	}
	if !declared {
		t.Errorf("render declares no %q switch", renderAll)
	}
	parsed, err := actionTable().Parse("render", []string{"all"})
	if err != nil {
		t.Fatalf("render all is refused by the parser: %v", err)
	}
	if !parsed.Has(renderAll) {
		t.Error("render all parses without the all switch set")
	}
	if parsed.Has(demoKeyword) {
		t.Errorf("render all parses with a demo named: %q", parsed.One(demoKeyword))
	}
}
