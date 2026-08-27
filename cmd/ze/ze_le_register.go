// Design: docs/architecture/system-architecture.md -- the ze_le crossing
//
// One blank import is the whole seam: internal/le registers the `le` root and
// composes every development tool. No default build sets this tag.

//go:build ze_le

package main

import _ "github.com/ze-software/ze/internal/le"
