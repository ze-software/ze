//go:build zetest

// Design: docs/architecture/testing/ci-format.md -- test-only plugin imports

package main

import (
	// Test-only plugins are available only to ze-test DUT builds.
	_ "github.com/ze-software/ze/internal/test/plugins/all"
)
