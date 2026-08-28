// Design: docs/architecture/core-design.md -- the YANG/handler contract gate
// Overview: contract.go -- the gate these registrations are read by
//
// The BGP command handlers register their RPCs through init(), and they are not
// reached by internal/component/plugin/all. Importing them is what lets the
// contract gate see them, and the tag is what keeps that import out of a build
// with no BGP: `./le tier check` refuses an always-on file that reaches a
// compile-out-able feature (ai/rules/architecture.md).
//
// A build without ze_bgp therefore judges a product without BGP commands, which
// is the right answer for that product. This gate is run with the full feature
// set, as every registry-reading tool in this tree is.

//go:build ze_bgp

package docvalid

import (
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/cache"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/commit"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/monitor"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/peer"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/raw"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/rib"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/update"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/route_refresh/handler"
)
