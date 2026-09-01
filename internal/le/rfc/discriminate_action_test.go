// Design: docs/architecture/core-design.md -- recording a proof that was observed
// Overview: discriminate_action.go -- the overlay this file's cases are taken from
//
// The cases here cover the prune that runs after a producer's body is replaced
// by a halt. Each one states the source the overlay would carry and asserts
// which imports survive it, because an import the compiler no longer sees used
// fails the BUILD, and a build failure is reported where a red was owed.
package rfc

import (
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
