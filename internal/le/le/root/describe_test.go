// Design: docs/architecture/core-design.md -- le's command surface as one tree
// Related: describe.go -- the check these tests pin

package leroot

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// describedRow is an action row whose every item carries text, which each
// case below blanks one item of.
func describedRow() leaction.Row {
	return leaction.Row{
		Verb: "run", Summary: "run the probe", Why: "run the probe, then report what it found",
		Parameters: []leaction.Parameter{
			{Keyword: "step", Kind: leaction.KindEnum, Value: "step", Values: []string{"build", "test"},
				ValueHelp:   map[string]string{"build": "build only", "test": "test only"},
				Requirement: leaction.Optional, Help: "the step to run"},
			{Keyword: "old", Retired: "step"},
		},
	}
}

// TestEveryItemWithoutTextStopsLeNamingIt proves AC-54 (D-P7-describe-all):
// an area with no summary, a pass-through area naming no program, an action
// with no Why, a keyword with no Help, a declared value with no description,
// and a description for a value the keyword does not declare each stop le,
// and the stop names the item and what to fill in. AC-55 (D-P7-short-long)
// adds the short and long texts: an area with no Description, an action with
// no Summary, and a summary, Help or ValueHelp line over 60 characters or over
// one line each stop le too. A fully described area
// passes, and a retired keyword, which carries nothing, is not asked for help.
// Method: each probe registers its own table under its own name and runs the
// check over that one area, so no case depends on what another test
// registered or on run order.
// VALIDATES: requireEveryItemHasText, run once by Dispatch and Tree.
// PREVENTS: a blank line in help, completion, the menu or `le help ai`.
func TestEveryItemWithoutTextStopsLeNamingIt(t *testing.T) {
	meta := registry.Meta{ShortHelp: "a described probe", Description: "A probe area whose every item carries text.",
		Mode: "offline", Section: registry.SectionTest}
	tooLong := strings.Repeat("x", shortMax+1)

	whole := "zz-describe-whole"
	setActions(whole, func() leaction.List { return leaction.List{Area: whole, Actions: []leaction.Row{describedRow()}} })
	requireEveryItemHasText([]registry.RootCommand{{Name: whole, Meta: meta}})

	cases := []struct {
		name  string
		blank func(meta *registry.Meta, list *leaction.List)
		want  []string
	}{
		{"area summary", func(meta *registry.Meta, _ *leaction.List) { meta.ShortHelp = " " },
			[]string{`area "zz-describe-area summary" has no summary`, "Meta ShortHelp"}},
		{"area long summary", func(meta *registry.Meta, _ *leaction.List) { meta.ShortHelp = tooLong },
			[]string{`area "zz-describe-area long summary" has a summary of 61 characters, over the 60`, "Meta ShortHelp"}},
		{"area description", func(meta *registry.Meta, _ *leaction.List) { meta.Description = "" },
			[]string{`area "zz-describe-area description" has no description`, "Meta Description"}},
		{"action summary", func(_ *registry.Meta, list *leaction.List) { list.Actions[0].Summary = "" },
			[]string{`action "zz-describe-action summary run" has no Summary`, "fill Summary"}},
		{"action long summary", func(_ *registry.Meta, list *leaction.List) { list.Actions[0].Summary = tooLong },
			[]string{`action "zz-describe-action long summary run" has a Summary of 61 characters`, "fill Summary"}},
		{"action two-line summary", func(_ *registry.Meta, list *leaction.List) { list.Actions[0].Summary = "run\nthe probe" },
			[]string{`action "zz-describe-action two-line summary run" has a Summary of more than one line`, "fill Summary"}},
		{"long keyword help", func(_ *registry.Meta, list *leaction.List) { list.Actions[0].Parameters[0].Help = tooLong },
			[]string{`parameter "zz-describe-long keyword help run step" has a Help of 61 characters`, "fill Help"}},
		{"long value help", func(_ *registry.Meta, list *leaction.List) {
			list.Actions[0].Parameters[0].ValueHelp = map[string]string{"build": tooLong, "test": "test only"}
		}, []string{`value "build" of "zz-describe-long value help run step" has a ValueHelp line of 61 characters`, "shorten its line"}},
		{"pass-through program", func(_ *registry.Meta, list *leaction.List) {
			*list = leaction.List{Area: list.Area, PassesWords: true}
		}, []string{`pass-through area "zz-describe-pass-through program" names no program`, "RegisterForwarding"}},
		{"action why", func(_ *registry.Meta, list *leaction.List) { list.Actions[0].Why = "" },
			[]string{`action "zz-describe-action why run" has no Why`, "fill Why"}},
		{"keyword help", func(_ *registry.Meta, list *leaction.List) { list.Actions[0].Parameters[0].Help = "" },
			[]string{`parameter "zz-describe-keyword help run step" has no Help`, "fill Help"}},
		{"value help", func(_ *registry.Meta, list *leaction.List) {
			list.Actions[0].Parameters[0].ValueHelp = map[string]string{"build": "build only"}
		}, []string{`value "test" of "zz-describe-value help run step" has no description`, `add "test":`}},
		{"stray value help", func(_ *registry.Meta, list *leaction.List) {
			list.Actions[0].Parameters[0].ValueHelp["deploy"] = "deploy"
		}, []string{`describes "deploy", which is not one of its Values`, "delete that ValueHelp entry"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name := "zz-describe-" + tc.name
			probeMeta := meta
			list := leaction.List{Area: name, Actions: []leaction.Row{describedRow()}}
			tc.blank(&probeMeta, &list)
			setActions(name, func() leaction.List { return list })
			stop := stopOf(func() { requireEveryItemHasText([]registry.RootCommand{{Name: name, Meta: probeMeta}}) })
			if stop == "" {
				t.Fatalf("the check passed an area whose %s is blank", tc.name)
			}
			for _, want := range append(tc.want, "internal/le/zz-describe-"+strings.ReplaceAll(tc.name, " ", "/")+"/") {
				if !strings.Contains(stop, want) {
					t.Errorf("the stop does not say %q: %q", want, stop)
				}
			}
		})
	}
}

// TestEveryTextFaultIsNamedInOneStop proves that one walk names every fault,
// so an author fixes them in one round: an area whose keyword Help and one
// ValueHelp line are both over 60 characters answers two faults from
// TextFaults, and the startup stop carries both, each on its own BUG line.
// Method: register one probe area with two overlong texts and run TextFaults
// and requireEveryItemHasText over it alone.
// VALIDATES: TextFaults, and requireEveryItemHasText stopping with all of it.
// PREVENTS: a stop that names the first fault only, which costs one round per
// fault while ./le is down for every session.
func TestEveryTextFaultIsNamedInOneStop(t *testing.T) {
	name := "zz-describe-every-fault"
	tooLong := strings.Repeat("x", shortMax+1)
	list := leaction.List{Area: name, Actions: []leaction.Row{describedRow()}}
	list.Actions[0].Parameters[0].Help = tooLong
	list.Actions[0].Parameters[0].ValueHelp = map[string]string{"build": tooLong, "test": "test only"}
	setActions(name, func() leaction.List { return list })
	roots := []registry.RootCommand{{Name: name, Meta: registry.Meta{ShortHelp: "a probe",
		Description: "A probe area with two overlong texts.", Mode: "offline", Section: registry.SectionTest}}}

	want := []string{
		`parameter "zz-describe-every-fault run step" has a Help of 61 characters`,
		`value "build" of "zz-describe-every-fault run step" has a ValueHelp line of 61 characters`,
	}
	faults := TextFaults(roots)
	if len(faults) != len(want) {
		t.Fatalf("TextFaults answered %d faults, want %d: %q", len(faults), len(want), faults)
	}
	for index, fault := range faults {
		if !strings.Contains(fault, want[index]) {
			t.Errorf("fault %d does not say %q: %q", index, want[index], fault)
		}
		if !strings.Contains(fault, "(internal/le/zz-describe-every-fault/)") {
			t.Errorf("fault %d names no package: %q", index, fault)
		}
	}

	stop := stopOf(func() { requireEveryItemHasText(roots) })
	if strings.Count(stop, "BUG: le ") != len(want) {
		t.Fatalf("the stop carries %d BUG lines, want %d: %q", strings.Count(stop, "BUG: le "), len(want), stop)
	}
	for _, line := range want {
		if !strings.Contains(stop, line) {
			t.Errorf("the stop does not say %q: %q", line, stop)
		}
	}
}

// stopOf runs check and answers the BUG panic it raised, or "" when it raised
// none.
func stopOf(check func()) (stop string) {
	defer func() {
		stop, _ = recover().(string)
	}()
	check()
	return ""
}
