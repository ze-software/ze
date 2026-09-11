//go:build ruleguard

// Design: docs/contributing/ze-go-style.md -- the idioms and invariants no
// linter knows, which golangci-lint can only reach through gocritic.
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

// crashlogExec keeps every execve behind crashlog.Exec.
//
// crashlog.Init dup2s a pipe onto descriptor 2 and drains it from a goroutine.
// An execve destroys that goroutine, and fd 2 survives into the new image, so
// the replacement program writes its stderr into a pipe nobody reads: the log
// vanishes, and the write blocks forever once 64 KiB have accumulated.
// crashlog.Exec restores the saved descriptor first.
//
// The rule exists because the defect is invisible at the call site and in any
// test that does not arm crashlog. Four of the five execve sites in the tree
// had lost their output this way, and each was written by an author who could
// not have seen it (plan/journal/output-lost-to-an-exit-past-the-flush.md).
func crashlogExec(m dsl.Matcher) {
	m.Match(`syscall.Exec($path, $argv, $environ)`, `unix.Exec($path, $argv, $environ)`).
		Where(!m.File().PkgPath.Matches(`/internal/core/crashlog$`)).
		Report(`an execve past the crash-capture flush leaves the new image writing into a pipe nobody reads -- use crashlog.Exec($path, $argv, $environ)`).
		Suggest(`crashlog.Exec($path, $argv, $environ)`)
}
