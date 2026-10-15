// Design: docs/architecture/api/process-protocol.md -- the stop drain RPC
// Related: register.go -- the in-process registration
// Related: drain/drain.go -- the drain responder both modes share
// Related: ../../fixture/register_drain.go -- the external mode, over the same responder

// Package fakedrain is the drain-capable test plugin. It runs in-process as
// `internal fakedrain` and externally through the fixture
// plugin/drain-responder; both modes answer the stop drain with
// drain.Responder, so a .ci asserts the same lines whichever transport carried
// the drain. Its init registers the plugin, so only a ze built with the zetest tag
// imports this package; a fixture imports the registration-free drain package instead.
package fakedrain

import (
	"context"
	"net"

	"github.com/ze-software/ze/internal/test/plugins/fakedrain/drain"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// Name is the plugin's registry name.
const Name = "fakedrain"

// runPlugin is the in-process entry point.
func runPlugin(conn net.Conn) int {
	p := sdk.NewWithConn(Name, conn)
	defer p.Close() //nolint:errcheck // test plugin teardown, so a close failure changes no assertion

	answered := drain.Responder(p)
	err := p.Run(context.Background(), sdk.Registration{DrainBudget: drain.Budget})
	if !answered() {
		drain.Log("drain-fixture: FAIL connection closed before the drain was answered")
		return 1
	}
	drain.Log("drain-fixture: connection closed after the drain")
	if err != nil {
		return 1
	}
	return 0
}
