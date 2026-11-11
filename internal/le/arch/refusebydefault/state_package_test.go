// VALIDATES: inferred package state bindings do not depend on declaration
// order, and unknown or cyclic initializer chains produce no guessed type.
// PREVENTS: a forward copy hiding a literal state-member assignment.
package archrefusebydefault

import (
	"slices"
	"testing"

	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// TestLiteralStatePackageBindings follows package initialization dependencies
// before judging any function that reads the resulting bindings.
func TestLiteralStatePackageBindings(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{name: "forward copy", source: `var copied = original
var original = PeerState{}
func answer() {
	copied.Running = true // want: state-from-literal
}`},
		{name: "forward chain", source: `var copied = middle
var middle = original
var original = &PeerState{}
func answer() {
	copied.Running = false // want: state-from-literal
}`},
		{name: "reverse chain", source: `var original = &PeerState{}
var middle = original
var copied = middle
func answer() {
	copied.Running = false // want: state-from-literal
}`},
		{name: "paired forward declaration", source: `var config, copied = PeerConfig{}, original
var original = PeerState{}
func answer() {
	config.Running = true
	copied.Running = true // want: state-from-literal
}`},
		{name: "explicit forward type", source: `var copied = original
var original PeerState
func answer() {
	copied.Running = true // want: state-from-literal
}`},
		{name: "ordinary config chain", source: `var copied = middle
var middle = original
var original = PeerConfig{}
func answer() {
	copied.Running = true
}`},
		{name: "unknown call chain", source: `var copied = middle
var middle = original
var original = opaque()
func answer() {
	copied.Running = true
}`},
		// ScanFile checks parsed syntax without Go's package type checking.
		// Even an invalid initializer cycle must finish without guessing.
		{name: "untyped initializer cycle", source: `var copied = original
var original = copied
func answer() {
	copied.Running = true
}`},
	}
	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			source := "package fixture\ntype PeerState struct { Running bool }\n" +
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
