// Design: docs/contributing/running-commands.md -- heavy work is declared where its verb registers
// Related: le/root/heavy.go -- HeavyLine, the predicate this census judges
// Related: register.go -- the composition root that links every area judged here

package le

import (
	"slices"
	"strings"
	"testing"

	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// censusArea is one row of the census of heavy actions: an area with an
// action table, and which of its actions start heavy work. When every is
// set, each row of the table is heavy but the ones light names; otherwise
// the rows heavy names are heavy and every other row is light.
type censusArea struct {
	area  string
	every bool
	heavy []string
	light []string
}

// heavyCensus is the census of spec-main-thread-schedules-heavy-work
// (AC-10), one entry per area that holds an action table. It names the
// expected verdicts and nothing else: the declarations are the areas' own,
// and the test reads each area's registered table, so a row an area gains is
// judged too.
var heavyCensus = []censusArea{
	{area: "verify", heavy: []string{"current", "worktree", "changed"}},
	{area: "verify deps", every: true},
	{area: "verify evidence", every: true},
	{area: "go lint", every: true},
	{area: "go staticcheck", heavy: []string{"check"}},
	{area: "go vet-platforms", every: true},
	{area: "go module", every: true},
	{area: "go stdpatch", every: true},
	{area: "test unit", every: true},
	{area: "test functional", every: true, light: []string{"list", "preview"}},
	{area: "test integration", every: true},
	{area: "test qemu", every: true},
	{area: "test deployment", every: true},
	{area: "test fuzz", heavy: []string{"run"}},
	{area: "test attribution", heavy: []string{"report"}},
	{area: "test redwalk", heavy: []string{"check"}},
	{area: "test netlab", heavy: []string{"run", "check", "update"}},
	{area: "rfc", heavy: []string{"check", "record", "discriminate"}},
	{area: "repo compiles", heavy: []string{"check", "selftest"}},
	{area: "repo bootstraps", heavy: []string{"check"}},
	{area: "repo codeql", heavy: []string{"check"}},
	{area: "build host-driver", heavy: []string{"run"}},
	{area: "build installer", every: true},
	{area: "build preview", heavy: []string{"run"}},
	{area: "chaos selftest", every: true},
	{area: "perf", heavy: []string{"run", "filter", "send"}},
	{area: "doc yang-contract", heavy: []string{"doc-drift"}},
	{area: "feature", heavy: []string{"run"}},
	{area: "site", heavy: []string{"build", "config-tree", "talk"}},
	{area: "site terminal-demo", heavy: []string{"build", "render", "pty"}},
	{area: "setup", heavy: []string{"install", "tools", "proto", "claude-server", "web", "appliance-modules"}},
	{area: "setup docker host", heavy: []string{"build", "start", "stop"}},
	{area: "setup docker kernel", heavy: []string{"install", "apparmor"}},
	{area: "weekly", heavy: []string{"report"}},
	{area: "job", heavy: []string{"run"}},
	{area: "ai hooks", heavy: []string{"unit"}},
	{area: "test stress-repro", every: true},
	// Areas the census read and found light, kept here so a heavy
	// declaration added to one of them is a decision and not an accident.
	{area: "doc wiring"},
	{area: "repo"},
	{area: "repo changed"},
	{area: "scratch"},
	{area: "verify status"},
	{area: "verify summary"},
	{area: "spec current"},
	{area: "commit"},
	{area: "test weakened"},
}

// TestCensusActionsDeclareHeavy asks whether every action the census calls
// heavy answers heavy to leroot.HeavyLine, and every other action of the same
// area answers light. Its method: this package links every area
// (register.go), so the test reads each census area's registered table and
// judges `<area> <verb>` for each row, then the areas with no rows and the
// lines an agent types, with their keywords.
//
// VALIDATES: spec-main-thread-schedules-heavy-work AC-10: each heavy action
// declares itself where its verb registers, a mixed area's light actions stay
// light, and a listing or a help page starts nothing.
// PREVENTS: a heavy action that no declaration covers, which a subagent could
// then start, and a light action swept up by its area's declaration.
func TestCensusActionsDeclareHeavy(t *testing.T) {
	for _, row := range heavyCensus {
		list, declared := leroot.ActionsOf(row.area)
		if !declared {
			t.Errorf("census area %q registers no action table: the census names an area that is gone or renamed", row.area)
			continue
		}
		if list.PassesWords {
			t.Errorf("census area %q passes its words on: it belongs in the forwarding census below", row.area)
			continue
		}
		verbs := make([]string, 0, len(list.Actions))
		for _, action := range list.Actions {
			verbs = append(verbs, action.Verb)
			want := slices.Contains(row.heavy, action.Verb)
			if row.every {
				want = !slices.Contains(row.light, action.Verb)
			}
			line := row.area + " " + action.Verb
			if got := leroot.HeavyLine(strings.Fields(line)); got != want {
				t.Errorf("HeavyLine(%q) = %v, want %v", line, got, want)
			}
		}
		for _, verb := range slices.Concat(row.heavy, row.light) {
			if !slices.Contains(verbs, verb) {
				t.Errorf("census area %q holds no action %q: the census names an action that is gone or renamed", row.area, verb)
			}
		}
	}

	// Areas with no rows: every word is another program's, so the area
	// declares itself, and a help word reaches the program and starts nothing.
	for _, row := range []struct {
		area  string
		heavy bool
	}{
		{"chaos run", true},
		{"exec", true},
		{"build gokrazy", true},
		{"test fail-syscall", true},
		{"test scale", true},
		{"test local-data-coverage", true},
		{"test bgp", true},
		{"test ospf", true},
		// Read and found light: they start no build, test or container.
		{"mrt", false},
		{"test interop-bgp", false},
		{"test peer", false},
		{"mock rpki", false},
	} {
		if !leroot.Forwards(row.area) {
			t.Errorf("census area %q does not pass its words on: the census names an area that is gone or changed shape", row.area)
			continue
		}
		line := row.area + " probe word"
		if got := leroot.HeavyLine(strings.Fields(line)); got != row.heavy {
			t.Errorf("HeavyLine(%q) = %v, want %v", line, got, row.heavy)
		}
		help := row.area + " --help"
		if leroot.HeavyLine(strings.Fields(help)) {
			t.Errorf("HeavyLine(%q) = true, want false: a help page starts nothing", help)
		}
	}

	// The lines as an agent types them, keywords included.
	for _, row := range []struct {
		line  string
		heavy bool
	}{
		{"test unit pkg path ./x", true},
		{"test unit pkg path ./x run TestX vet", true},
		{"test unit all", true},
		{"go lint run", true},
		{"go lint run scope ./internal/le/...", true},
		{"rfc record stem rfc4271 id x", true},
		{"rfc check", true},
		{"job run label x command echo", true},
		{"job run label unit-pkg quiet command go version", true},
		{"exec go version", true},
		{"verify changed", true},
		{"verify worktree commit HEAD keep", true},
		{"verify deps unit-race-changed part 1 of 2", true},
		{"test bgp x", true},
		{"test functional gating", true},
		{"test integration interop", true},
		// The cleanup removes containers on the Docker daemon every session
		// shares, so the area declares it heavy with the rest.
		{"test integration clean", true},
		{"test integration clean dry", true},
		{"repo check generated", true},
		{"repo check generated | json", true},
		{"--name session test unit all", true},
		// The light actions of the mixed areas, and what starts no work.
		{"verify status check", false},
		{"verify summary record", false},
		{"verify reds file x.go", false},
		{"verify list", false},
		{"verify", false},
		{"test functional list", false},
		{"test functional preview", false},
		{"test functional", false},
		{"test integration", false},
		{"test integration --help", false},
		{"test qemu", false},
		{"test unit", false},
		{"rfc approve unit x reason y", false},
		{"rfc selftest", false},
		{"test weakened approve unit x reason y", false},
		{"repo check", false},
		{"repo check tree", false},
		{"repo update", false},
		{"repo changed", false},
		{"repo codeql packages tags x", false},
		{"job report", false},
		{"job", false},
		{"spec current", false},
		{"commit create", false},
		{"scratch clean", false},
		{"scratch trim", false},
		{"doc wiring", false},
		{"doc wiring check", false},
		{"weekly", false},
		{"mrt statistics x", false},
		{"help test", false},
		{"help test unit", false},
		{"test unit all --help", false},
		{"test functional --help", false},
		{"go lint run --help", false},
	} {
		if got := leroot.HeavyLine(strings.Fields(row.line)); got != row.heavy {
			t.Errorf("HeavyLine(%q) = %v, want %v", row.line, got, row.heavy)
		}
	}
}
