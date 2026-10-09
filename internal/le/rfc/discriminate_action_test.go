// Design: docs/architecture/core-design.md -- recording a proof that was observed
// Overview: discriminate_action.go -- the overlay this file's cases are taken from
//
// The cases cover checker source selection and the import prune after a
// producer's body is replaced by a halt. A wrong scenario selects the wrong
// proof; an orphaned import fails the build before the proof can run.
package rfc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDropOrphanedImportsRemovesWhatTheHaltOrphaned feeds the shape a revert
// produces -- one function whose body is gone, and imports only that body used
// -- and asserts the orphans go while the still-used import stays.
//
// The disabled function takes no argument on purpose. A revert replaces the
// BODY and leaves the signature, so an import a parameter type names is still
// used and MUST survive; only an import the body alone reached is orphaned.
//
// VALIDATES: the overlay a revert hands the compiler builds.
// PREVENTS: the eleven RFC 2865 proofs that could not be recorded on
// 2026-09-01, each refused for "context imported and not used" rather than for
// anything about whether its test discriminates.
func TestDropOrphanedImportsRemovesWhatTheHaltOrphaned(t *testing.T) {
	source := `package radius

import (
	"context"
	"fmt"
	"strings"
)

func Authenticate() error {
	panic("BUG: disabled")
}

func Name(raw string) string {
	return strings.TrimSpace(raw)
}
`
	pruned := dropOrphanedImports("authenticator.go", source)

	if strings.Contains(pruned, `"context"`) {
		t.Errorf("context survived the prune, so the overlay does not build:\n%s", pruned)
	}
	if strings.Contains(pruned, `"fmt"`) {
		t.Errorf("fmt survived the prune, so the overlay does not build:\n%s", pruned)
	}
	if !strings.Contains(pruned, `"strings"`) {
		t.Errorf("strings was dropped while Name still calls it:\n%s", pruned)
	}
}

// TestRevertMarkerIsInsideTheHalt pins the fragment the run's output is matched
// on to the halt the break actually writes.
//
// VALIDATES: a crash the break caused is still recognized as the break's.
// PREVENTS: the two drifting apart, which would leave killedByTheBreak matching
// nothing and every goroutine-served producer unprovable again, with no failure
// saying why.
func TestRevertMarkerIsInsideTheHalt(t *testing.T) {
	if !strings.Contains(revertBody, revertMarker) {
		t.Errorf("the halt %q no longer carries the marker %q, so a run it kills is attributed to nothing",
			revertBody, revertMarker)
	}
}

// TestDropOrphanedImportsKeepsAnImportWithNoName asserts a blank and a dot
// import survive, and that a file needing no prune is returned byte for byte.
//
// VALIDATES: an import reached by no name is never judged unused.
// PREVENTS: dropping a driver registration or a dot import, each of which
// carries an effect no identifier in the file reveals.
func TestDropOrphanedImportsKeepsAnImportWithNoName(t *testing.T) {
	source := `package radius

import (
	_ "embed"
	. "strings"
)

func Name(raw string) string {
	return TrimSpace(raw)
}
`
	pruned := dropOrphanedImports("attr.go", source)

	if pruned != source {
		t.Errorf("a file with nothing to prune was rewritten:\n%s", pruned)
	}
}

// TestDropOrphanedImportsIgnoresAFieldNamedLikeAPackage feeds a file whose only
// call through `bytes` sat in the disabled body while a struct field is also
// named `bytes`, and asserts the import goes: a field is never a package use.
//
// VALIDATES: the prune counts only the left side of a selector.
// PREVENTS: the refusal measured 2026-09-21 on
// internal/component/l2tp/reactor.go::resolveTieBreakerLocked, where
// `pkt.bytes` kept "bytes" imported and unused and the overlay failed to build.
func TestDropOrphanedImportsIgnoresAFieldNamedLikeAPackage(t *testing.T) {
	source := `package l2tp

import (
	"bytes"
)

type packet struct {
	bytes []byte
}

func compare(pkt packet) int {
	panic("BUG: disabled")
}

func size(pkt packet) int {
	return len(pkt.bytes)
}
`
	pruned := dropOrphanedImports("reactor.go", source)

	if strings.Contains(pruned, `"bytes"`) {
		t.Errorf("bytes survived the prune on the strength of the field pkt.bytes:\n%s", pruned)
	}
	if !strings.Contains(pruned, "len(pkt.bytes)") {
		t.Errorf("the field read was rewritten:\n%s", pruned)
	}
}

// TestDropOrphanedImportsKeepsAMajorVersionImportInUse: "math/rand/v2" is
// reached as rand, not v2, because Go names a module's major-version suffix
// after the element before it. Read as v2, the prune found no use and dropped
// an import the rest of the file still called, so the overlay failed to build
// (measured 2026-09-28 on internal/component/bfd/engine/engine.go).
func TestDropOrphanedImportsKeepsAMajorVersionImportInUse(t *testing.T) {
	source := `package engine

import (
	"fmt"
	"math/rand/v2"
)

func disabled() string {
	panic("BUG: disabled")
}

func pick() uint32 {
	return rand.Uint32()
}
`
	pruned := dropOrphanedImports("engine.go", source)

	if !strings.Contains(pruned, `"math/rand/v2"`) {
		t.Errorf("math/rand/v2 was dropped although rand.Uint32 still uses it:\n%s", pruned)
	}
	if strings.Contains(pruned, `"fmt"`) {
		t.Errorf("fmt, used nowhere, survived the prune:\n%s", pruned)
	}
}

// TestInteropScenarioOfConstantDeclarations keeps ordinary single declarations
// and grouped declarations usable through the recorder's source resolver.
func TestInteropScenarioOfConstantDeclarations(t *testing.T) {
	const source = "internal/le/interoplab/bgp/check_fixture.go"
	tests := []struct {
		name        string
		declaration string
	}{
		{
			name:        "single",
			declaration: "\tconst name = \"fixture-scenario\"\n",
		},
		{
			name:        "grouped",
			declaration: "\tconst (\n\t\tname = \"fixture-scenario\"\n\t)\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := t.TempDir()
			if err := os.MkdirAll(filepath.Join(tree, interopScenarioRel, "fixture-scenario"), 0o755); err != nil {
				t.Fatal(err)
			}
			reader := newTextReader(map[string]string{
				source: "package bgp\n\nfunc checkFixture() {\n" + tt.declaration + "}\n",
			})
			record := DiscriminationRecord{Unit: source + "::checkFixture"}
			scenario, err := interopScenarioOf(tree, reader, newScopeIndex(), record)
			if err != nil {
				t.Fatal(err)
			}
			if scenario != "fixture-scenario" {
				t.Fatalf("selected scenario = %q, want fixture-scenario", scenario)
			}
		})
	}
}
