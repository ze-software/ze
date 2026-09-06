//go:build ruleguard

// Design: docs/contributing/ze-go-style.md -- modern standard-library idioms
// golangci-lint cannot reach through gocritic's own checkers.
//
// Detail: gocritic loads this file at runtime through its ruleguard checker.
// The build tag keeps it out of every Ze build, and the leading dot on
// .golangci/ makes the Go toolchain skip the directory, so nothing here is
// compiled, vendored, or linted.
//
// Related: .golangci.yml -- gocritic enabled-checks and ruleguard rules path.

package ruleguard

import "github.com/quasilyte/go-ruleguard/dsl"

// modernSort replaces the sort package's type-specific entry points with
// slices.Sort.
//
// sort.Strings(x) is sort.Sort(sort.StringSlice(x)): every comparison and
// every swap goes through sort.Interface, so each one is a dynamic call.
// slices.Sort is generic, so the compiler emits the comparison directly.
// The orders are identical, NaN included: slices.Sort compares with cmp.Less,
// which places NaN before every other float, and that is what sort.Float64s
// documents.
//
// The x/tools modernize suite carries no rule for this, and no other linter
// does either. Without this file the three calls are invisible.
func modernSort(m dsl.Matcher) {
	m.Match(`sort.Strings($s)`).
		Report(`sort.Strings sorts through sort.Interface -- use slices.Sort($s)`).
		Suggest(`slices.Sort($s)`)

	m.Match(`sort.Ints($s)`).
		Report(`sort.Ints sorts through sort.Interface -- use slices.Sort($s)`).
		Suggest(`slices.Sort($s)`)

	m.Match(`sort.Float64s($s)`).
		Report(`sort.Float64s sorts through sort.Interface -- use slices.Sort($s)`).
		Suggest(`slices.Sort($s)`)
}
