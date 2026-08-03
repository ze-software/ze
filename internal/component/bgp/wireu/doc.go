// Package wireu ("wire UPDATE") implements lazy-parsed BGP UPDATE messages
// with zero-copy iterators over wire bytes.
//
// The name is a contraction kept for history: the package predates the
// package-naming glossary (ai/rules/go-standards.md "Package-Naming Glossary"),
// which would name a new package like this after its concern spelled out.
// Kept over renaming (user decision 2026-07-08, spec-layout-3): a rename
// touches ~47 importer files inside the BGP trees that the rib-arch spec
// set is concurrently reworking.
package wireu
