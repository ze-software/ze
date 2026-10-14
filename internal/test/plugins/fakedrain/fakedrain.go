// Design: docs/architecture/api/process-protocol.md -- the stop drain RPC
// Related: register.go -- the in-process registration
// Related: ../../fixture/register_drain.go -- the external mode, over the same responder

// Package fakedrain is the drain-capable test plugin. It runs in-process as
// `internal fakedrain` and externally through the fixture
// plugin/drain-responder; both modes answer the stop drain with Responder, so
// a .ci asserts the same lines whichever transport carried the drain.
package fakedrain

import (
	"context"
	"net"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// Name is the plugin's registry name.
const Name = "fakedrain"

// Budget is the declared DrainBudget in seconds, above Hold so the daemon
// waits for the answer rather than cutting the drain short.
const Budget = 5

// Hold is how long Responder holds a drain before it answers, so a .ci can
// see the daemon wait for the answer.
const Hold = time.Second

// Responder registers on p a drain handler that logs each drain it receives
// with its kind, holds it for Hold, then answers. It returns the report the
// caller MUST read when Run returns: whether a drain was answered before the
// connection closed. A stop that closed the connection with no drain
// answered is the failure the plugin that lives it reports.
func Responder(p *sdk.Plugin) (answered func() bool) {
	return ResponderWith(p, nil)
}

// ResponderWith is Responder that also runs onReceived, when it is not nil,
// right after logging each drain it receives and before holding it: the hook
// where a fixture checks what must still hold while the drain runs.
func ResponderWith(p *sdk.Plugin, onReceived func()) (answered func() bool) {
	var done atomic.Bool
	p.OnDrain(func(ctx context.Context, input rpc.DrainInput) error {
		deadlineSeconds := int(time.Until(input.Deadline).Round(time.Second) / time.Second)
		Log("drain-fixture: drain received kind=" + input.Kind.String() +
			" deadline-in=" + strconv.Itoa(deadlineSeconds) + "s")
		if onReceived != nil {
			onReceived()
		}
		select {
		case <-time.After(Hold):
		case <-ctx.Done():
			Log("drain-fixture: FAIL connection closed while the drain was held")
			return ctx.Err()
		}
		done.Store(true)
		Log("drain-fixture: drain answered")
		return nil
	})
	return done.Load
}

// Log writes one line to stderr, which the daemon carries for the .ci
// expectations in both modes.
func Log(line string) {
	os.Stderr.WriteString(line + "\n") //nolint:errcheck // a lost line fails the .ci expectation that reads it
}

// runPlugin is the in-process entry point.
func runPlugin(conn net.Conn) int {
	p := sdk.NewWithConn(Name, conn)
	defer p.Close() //nolint:errcheck // test plugin teardown, so a close failure changes no assertion

	answered := Responder(p)
	err := p.Run(context.Background(), sdk.Registration{DrainBudget: Budget})
	if !answered() {
		Log("drain-fixture: FAIL connection closed before the drain was answered")
		return 1
	}
	Log("drain-fixture: connection closed after the drain")
	if err != nil {
		return 1
	}
	return 0
}
