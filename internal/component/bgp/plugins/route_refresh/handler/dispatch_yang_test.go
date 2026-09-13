// Design: docs/architecture/api/commands.md -- the YANG modules this test binary dispatches against
// Related: dispatch_test.go -- the tests that type `request peer <addr> borr`

package handler

// ze-peer-cmd.yang declares the `request peer` container, and gives it a
// mandatory `selector` leaf. ze-refresh-cmd.yang hangs refresh, borr, eorr and
// clear inside that container, and re-declares no selector.
//
// So a binary that registers this plugin's module, and not the peer plugin's,
// builds a `request peer` with no selector. `request peer 192.0.2.1 borr
// ipv4/unicast` then comes back `unknown command`.
//
// The daemon links both under one tag, from one generated composition root
// (internal/component/plugin/all/all_ze_bgp.go, ze_bgp). A test binary links
// what its own imports reach. That is why the module is named here rather than
// in the package's product code. The composition root already decides for the
// daemon, and a second declaration beside it would be free to disagree.
import (
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/peer/yang" // init() registers ze-peer-cmd, which declares the `request peer` selector
)
