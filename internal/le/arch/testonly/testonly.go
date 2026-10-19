// Design: docs/architecture/module-tiers.md -- test-only helper packages
// Related: ../tier/testonly.go -- the import gate that refuses a shipped importer
// Related: ../../doc/wiring/wiring.go -- the wiring check that exempts their exports
//
// Package testonly declares the helper packages only tests may import. It is
// a leaf, so every check that honors the declaration reads this one list.
package testonly

// Packages are module-relative helper packages that import `testing` and
// start real containers. A _test.go file imports them, and so may tooling
// under internal/le; a shipped file elsewhere never does.
var Packages = [...]string{"internal/le/interoplab/labtest"}

// Owner answers the declared test-only package that rel, a module-relative
// package or file path, names or lies under, and false when it is none.
func Owner(rel string) (string, bool) {
	for _, helper := range Packages {
		if under(rel, helper) {
			return helper, true
		}
	}
	return "", false
}

// under reports whether rel names dir or a descendant. It slices rather than
// concatenating dir+"/", so a query allocates nothing
// (`ai/rules/performance.md`).
func under(rel, dir string) bool {
	if rel == dir {
		return true
	}
	return len(rel) > len(dir) && rel[:len(dir)] == dir && rel[len(dir)] == '/'
}
