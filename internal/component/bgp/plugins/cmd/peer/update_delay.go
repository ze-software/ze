// Design: docs/architecture/api/commands.md — `show bgp update-delay`
// Related: internal/component/bgp/reactor/update_delay.go — the hold this reports
// RFC: rfc/short/rfc4724.md — the deferral this command makes visible

package peer

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// cmdBgpUpdateDelay is the command path an operator types.
const cmdBgpUpdateDelay = "show bgp update-delay"

// The JSON keys this command's payload carries, in the order registerColumns
// declares them. Each one is the kebab-case tag of a plugin.UpdateDelayStatus
// field, so the struct and the column order cannot drift apart silently.
const (
	fieldUpdateDelayConfigured     = "configured"
	fieldUpdateDelayHolding        = "holding"
	fieldUpdateDelayReleased       = "released"
	fieldUpdateDelayReason         = "reason"
	fieldUpdateDelayExpectedPeers  = "expected-peers"
	fieldUpdateDelayPeersHeld      = "peers-held"
	fieldUpdateDelayPeersConverged = "peers-converged"
	fieldUpdateDelayMaxDelay       = "max-delay-seconds"
	fieldUpdateDelayEstablishWait  = "establish-wait-seconds"
)

// handleBgpUpdateDelay answers `show bgp update-delay`: whether this speaker is
// withholding its initial routing update, why, and what it is waiting for.
//
// It exists because a holding daemon and a broken one look identical from
// outside. The engine logs the same facts, at INFO, which the default WARN level
// suppresses, so an operator watching a speaker that has come up and advertised
// nothing has no way to tell "converging as configured" from "wedged". That is
// the question this command answers, and it is why the payload carries the
// counts rather than a single word: `expected-peers` against `peers-converged`
// says WHICH neighbor has not finished.
//
// Takes no argument. The hold is a property of the speaker, not of a peer.
func handleBgpUpdateDelay(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	if ctx == nil || ctx.Reactor() == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  errReactorNotAvailable.Error(),
		}, errReactorNotAvailable
	}

	status := ctx.Reactor().UpdateDelayStatus()

	// Structured data, not a formatted string, so `| json`, `| yaml` and
	// `| table` each render it (ai/rules/cli.md).
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{
		fieldUpdateDelayConfigured:     status.Configured,
		fieldUpdateDelayHolding:        status.Holding,
		fieldUpdateDelayReleased:       status.Released,
		fieldUpdateDelayReason:         status.Reason,
		fieldUpdateDelayExpectedPeers:  status.ExpectedPeers,
		fieldUpdateDelayPeersHeld:      status.PeersHeld,
		fieldUpdateDelayPeersConverged: status.PeersConverged,
		fieldUpdateDelayMaxDelay:       status.MaxDelaySeconds,
		fieldUpdateDelayEstablishWait:  status.EstablishWaitSeconds,
	}}, nil
}
