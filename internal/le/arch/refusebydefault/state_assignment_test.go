// VALIDATES: literal boolean field writes into a visibly typed state value
// are refused, with lexical scope, changed-line and annotation handling.
// PREVENTS: constructing an empty state and then assigning a plausible answer
// past the composite-literal check, or refusing an ordinary config field.
package archrefusebydefault

import (
	"slices"
	"testing"

	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// TestLiteralStateAssignments follows known source types rather than field
// names, and pairs each literal with its own assignment target.
func TestLiteralStateAssignments(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{name: "local true", source: `func answer() PeerState {
	s := PeerState{}
	s.Running = true // want: state-from-literal
	return s
}`},
		{name: "local false", source: `func answer() PeerState {
	s := PeerState{}
	s.Running = false // want: state-from-literal
	return s
}`},
		{name: "pointer literal", source: `func answer() *PeerState {
	s := &PeerState{}
	s.Running = true // want: state-from-literal
	return s
}`},
		{name: "new pointer", source: `func answer() *PeerState {
	s := new(PeerState)
	(*s).Running = false // want: state-from-literal
	return s
}`},
		{name: "typed var", source: `func answer() PeerState {
	var s PeerState
	s.Running = true // want: state-from-literal
	return s
}`},
		{name: "typed parameter", source: `func answer(s *PeerHealth) {
	s.Running = true // want: state-from-literal
}`},
		{name: "typed pointer var", source: `func answer() {
	var s *PeerHealth
	s.Running = false // want: state-from-literal
}`},
		{name: "qualified parameter", source: `func answer(s *remote.PeerStatus) {
	s.Running = false // want: state-from-literal
}`},
		{name: "named result", source: `func answer() (s PeerStatus) {
	s.Running = true // want: state-from-literal
	return
}`},
		{name: "receiver", source: `func (s *PeerState) answer() {
	s.Running = true // want: state-from-literal
}`},
		{name: "package var", source: `var s PeerState
func answer() {
	s.Running = true // want: state-from-literal
}`},
		{name: "copied binding", source: `func answer(input *PeerState) {
	s := input
	s.Running = true // want: state-from-literal
}`},
		{name: "parallel binding", source: `func answer() {
	c, s := PeerConfig{}, PeerState{}
	c.Running, s.Running = measured(), true // want: state-from-literal
}`},
		{name: "parallel var binding", source: `func answer() {
	var c, s = PeerConfig{}, PeerState{}
	c.Running = true
	s.Running = false // want: state-from-literal
}`},
		{name: "parallel write mapping", source: `func answer(s PeerState, c PeerConfig) {
	s.Running, c.Running = measured(), true
	c.Running, s.Running = true, measured()
	s.Running, c.Running = true, measured() // want: state-from-literal
}`},
		{name: "ordinary config same field", source: `func answer(c PeerConfig) {
	c.Running = true
	s := PeerConfig{}
	s.Running = false
}`},
		{name: "measured state field", source: `func answer(s PeerState) {
	s.Running = measured()
}`},
		{name: "shadowed local", source: `func answer(s PeerState) {
	{
		s := PeerConfig{}
		s.Running = true
	}
	s.Running = false // want: state-from-literal
}`},
		{name: "initializer scope", source: `func answer(s PeerState) {
	if s := (PeerConfig{}); permitted() {
		s.Running = true
	}
	s.Running = false // want: state-from-literal
}`},
		{name: "same scope short declaration", source: `func answer(s PeerState) {
	s, count := opaque(), 1
	s.Running = true // want: state-from-literal
	_ = count
}`},
		{name: "range shadows state", source: `func answer(s PeerState, configs []PeerConfig) {
	for _, s := range configs {
		s.Running = true
	}
	s.Running = false // want: state-from-literal
}`},
		{name: "shadowed parameter", source: `func answer(s PeerState) {
	visit(func(s PeerConfig) { s.Running = true })
	visit(func() {
		s.Running = true // want: state-from-literal
	})
}`},
		{name: "unknown call result", source: `func answer() {
	s := opaque()
	s.Running = true
}`},
		{name: "shadowed new", source: `func answer(new func(any) PeerConfig) {
	s := new(PeerState{})
	s.Running = true
}`},
		{name: "initializer closure sees outer name", source: `func answer(s PeerState) {
	{
		s := func() PeerConfig {
			s.Running = true // want: state-from-literal
			return PeerConfig{}
		}()
		s.Running = true
	}
}`},
		{name: "var initializer sees outer name", source: `func answer(s PeerState) {
	{
		var s = func() PeerConfig {
			s.Running = true // want: state-from-literal
			return PeerConfig{}
		}()
		s.Running = true
	}
}`},
		{name: "shadowed true is measured", source: `func answer(s PeerState) {
	true := measured()
	s.Running = true
}`},
	}
	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			source := "package fixture\ntype PeerState struct { Running bool }\n" +
				"type PeerStatus struct { Running bool }\ntype PeerHealth struct { Running bool }\n" +
				"type PeerConfig struct { Running bool }\n" + current.source + "\n"
			tree := t.TempDir()
			rel := "internal/fixture/state.go"
			writeSource(t, tree, rel, source)
			report, err := Check(tree, repochanged.ChangedLines{rel: every})
			if err != nil {
				t.Fatal(err)
			}
			if got, want := rows(report.Findings), markers(t, source, "want"); !slices.Equal(got, want) {
				t.Fatalf("findings = %v, want %v", got, want)
			}
		})
	}
}

// TestLiteralStateAssignmentScopeAndGuard keeps assignment findings on their
// changed lines and applies the existing named-guard escape to those lines.
func TestLiteralStateAssignmentScopeAndGuard(t *testing.T) {
	source := `package fixture
type PeerState struct { Running bool }
func Stop() {}
func answer(s *PeerState) {
	s.Running = true // want: state-from-literal
	// refuse-by-default: guards Stop -- Stop reads false as the stopped state
	s.Running = false // accepted: state-from-literal
}
`
	tree := t.TempDir()
	rel := "internal/fixture/state.go"
	writeSource(t, tree, rel, source)
	for _, current := range []struct {
		name    string
		lines   []repochanged.LineSpan
		refused []marked
		guarded []marked
	}{
		{name: "all", lines: every, refused: markers(t, source, "want"), guarded: markers(t, source, "accepted")},
		{name: "declaration only", lines: []repochanged.LineSpan{{From: 2, To: 2}}},
		{name: "write only", lines: []repochanged.LineSpan{{From: 5, To: 5}}, refused: markers(t, source, "want")},
		{name: "guarded only", lines: []repochanged.LineSpan{{From: 7, To: 7}}, guarded: markers(t, source, "accepted")},
	} {
		t.Run(current.name, func(t *testing.T) {
			report, err := Check(tree, repochanged.ChangedLines{rel: current.lines})
			if err != nil {
				t.Fatal(err)
			}
			if got := rows(report.Findings); !slices.Equal(got, current.refused) {
				t.Fatalf("refused = %v, want %v", got, current.refused)
			}
			if got := rows(report.Annotated); !slices.Equal(got, current.guarded) {
				t.Fatalf("annotated = %v, want %v", got, current.guarded)
			}
			for _, finding := range report.Annotated {
				if finding.Guard != "Stop" {
					t.Errorf("guard = %q, want Stop", finding.Guard)
				}
			}
		})
	}
}
