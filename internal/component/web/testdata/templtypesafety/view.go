// Design: (none -- fixture package for TestTemplComponentTypeSafety)

// Package templtypesafety is the fixture that proves a templ component reads
// its view model at compile time.
//
// The package sits under testdata, so no build and no lint run reaches it.
// TestTemplComponentTypeSafety loads it twice. The first load takes the source
// as written and must report no error. The second load applies an overlay that
// renames one field of pageView. That load must report a type error, because
// page_templ.go still reads the old name. AC-1 of
// plan/spec-web-templ-migration.md asks for that failure.
//
// Every name here is unexported on purpose. An exported name in a non-test file
// under internal/ owes a production caller to make ze-verify-wiring-docs, and a
// fixture can never have one.
package templtypesafety

// pageView is the view model that page renders.
type pageView struct {
	Title string
	Rows  []pageRow
}

// pageRow is one key and value pair of pageView.
type pageRow struct {
	Key   string
	Value string
}

// page comes from page.templ, and page_templ.go is the generated form of it.
// This reference names the component from the file the test overlays, so the
// fixture reads as one unit and holds no unused symbol.
var _ = page
