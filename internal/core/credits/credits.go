// Design: docs/architecture/core-design.md -- section 19, the core-leaf registries
// Related: internal/plugins/thanks/thanks.go -- the commands that answer from it
//
// Package credits is the record of the third-party works this binary carries:
// the list `ze thanks` answers and the licence texts `ze thanks licence`
// prints. It copies the core-leaf registry shape of internal/core/ipsecinventory:
// it imports the standard library only, a package that carries a work which is
// not a Go module declares it present from its own init() (DeclarePresent), and
// a reader asks for value-type records.
//
// The records are generated from website/data/dependencies.json into
// internal/core/credits/data, which this package embeds. Until that data is
// compiled in, every reader answers ErrNoData, never an empty list: an empty
// list would read as a binary that carries no third-party work
// (ai/rules/principles.md).

package credits

import (
	"errors"
)

// Work is one credited third-party work as this binary carries it. It is a
// self-contained value type: it crosses a component boundary and holds no
// pointer into this package's state.
type Work struct {
	// Name is the work's exact name: the module path of a Go module, or the
	// declared name of a work that is not one. It is what `| name` selects on.
	Name string
	// Version is the version this binary carries.
	Version string
	// Group is the heading of the curated list the work is listed under.
	Group string
	// Licence is the licence identifier the declaration records.
	Licence string
	// Copyright is the copyright holder the declaration records.
	Copyright string
	// Why is the sentence that says why Ze uses the work.
	Why string
	// Source is where the work's source can be obtained.
	Source string
}

// ErrNoData is the answer of every reader when no generated credits data is
// compiled into this binary. It is distinct from any answer a binary with data
// gives, so a reader never mistakes it for a list with no work in it.
var ErrNoData = errors.New("no credits data is compiled into this binary")

// ErrUnknownWork is the answer of LicenceOf for a name that names no work this
// binary carries. Callers match it with errors.Is; the error LicenceOf returns
// names the work asked for.
var ErrUnknownWork = errors.New("no work of that name is carried by this binary")

// DeclarePresent records that the calling package carries the work named name,
// a work that is not a Go module (a web asset, a copied or ported source).
//
// It is called from the carrying package's init(), with a literal name. A name
// the generated data does not hold is a defect in the declaration or in the
// data, found before the binary answers anything, so it panics with BUG as the
// other core registries do for a registration they refuse. No generated data
// is compiled into this binary, so it holds no name and every declaration is
// refused.
func DeclarePresent(name string) {
	panic("BUG: credits.DeclarePresent(" + name + "): " + ErrNoData.Error() +
		", so it holds no work of that name")
}

// Works answers one record per third-party work this binary carries, or
// ErrNoData. Safe for concurrent use.
func Works() ([]Work, error) {
	return nil, ErrNoData
}

// LicenceDocument answers the licence document of every work this binary
// carries, as the bytes the works ship, or ErrNoData. Safe for concurrent use.
func LicenceDocument() ([]byte, error) {
	return nil, ErrNoData
}

// LicenceOf answers the licence document of the one work whose name is exactly
// its argument, ErrUnknownWork when no carried work has that name, or
// ErrNoData. With no data compiled in there is no name to match, so the
// argument is not read. Safe for concurrent use.
func LicenceOf(_ string) ([]byte, error) {
	return nil, ErrNoData
}
