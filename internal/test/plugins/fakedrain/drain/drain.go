// Design: docs/architecture/api/process-protocol.md -- the stop drain RPC
// Related: ../fakedrain.go -- the in-process plugin that answers with Responder
// Related: ../../../fixture/register_drain.go -- the external mode, over the same responder

// Package drain is the drain responder the fakedrain plugin and the drain
// fixtures share. It registers nothing: the fixture package is linked into
// the le binary, so it imports this package and never fakedrain itself, whose
// init would put a test plugin in that binary's registry.
package drain

import (
	"context"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// Budget is the declared DrainBudget in seconds, above Hold so the daemon
// waits for the answer rather than cutting the drain short.
const Budget = 5

// Hold is how long Responder holds a drain before it answers, so a .ci can
// see the daemon wait for the answer.
const Hold = time.Second

// Responder registers on p a drain handler that logs each drain it receives
// with its kind (and, for a restart, its cause), holds it for Hold, then answers. It returns the report the
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
		Log("drain-fixture: drain received kind=" + input.Kind.String() + causeText(input.RestartCause) +
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

// causeText is the " cause=<cause>" a restart-kind drain carries, or "" for
// a stop, whose drain carries no cause.
func causeText(cause rpc.DrainRestartCause) string {
	if cause == "" {
		return ""
	}
	return " cause=" + string(cause)
}

// Log writes one line to stderr, which the daemon carries for the .ci
// expectations in both modes.
func Log(line string) {
	os.Stderr.WriteString(line + "\n") //nolint:errcheck // a lost line fails the .ci expectation that reads it
}
