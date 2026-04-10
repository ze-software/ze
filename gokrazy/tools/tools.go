// Design: docs/guide/appliance.md -- gokrazy build tool vendoring

//go:build tools

// Package tools pins the gokrazy build tool (gok) so it can be built
// from vendored source. The actual entry point is cmd/ze-gok/main.go
// which wraps gok with a repo-local GOMODCACHE.
package tools

import _ "github.com/gokrazy/tools/cmd/gok"
