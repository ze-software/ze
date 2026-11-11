// Design: docs/architecture/testing/ci-workflows.md -- an independently built CodeQL positive control.
// This package is never imported by le's bootstrap; the tracked build population
// must select it for CodeQL's database to contain its Go AST.
package codeqlsentinel

// AuthoredSentinel is the authored-source positive control for CI extraction.
func AuthoredSentinel() int { return 17 }
