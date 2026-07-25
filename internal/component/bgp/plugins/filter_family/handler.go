// Design: docs/architecture/core-design.md -- family-filter per-update decision
// Related: match.go -- family extraction + MP attribute surgery
// Related: config.go -- family-filter instance parsing
// RFC: rfc/short/rfc4760.md -- multiprotocol family removal (Section 6)
// RFC: rfc/short/rfc4271.md -- NOTIFICATION on tear-down (Section 6)
// RFC: rfc/short/rfc4486.md -- Cease / Connection Rejected subcode

package filter_family

import (
	"github.com/ze-software/ze/internal/component/bgp/message"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// dirImport is the RPC FilterUpdateInput.Direction value for received UPDATEs.
const dirImport = "import"

// handleFilterUpdate evaluates one filter-update RPC against the named instance.
//
//   - unknown instance / configure-before-load: reject (fail-closed).
//   - missing or malformed raw body: accept (cannot inspect; BGP-layer validation
//     still applies, and we must not drop valid traffic on a decode failure).
//   - family does not match: accept (AC-6).
//   - remove + match: reject when removal empties the UPDATE (RFC 4760 §6 MP-only,
//     AC-2/AC-4) or a no-MP ipv4/unicast match; otherwise modify with a raw
//     full-payload replacement that drops the MP attribute (AC-3).
//   - tear-down + match (import only): reject + Teardown with a Cease / Connection
//     Rejected NOTIFICATION (RFC 4271, RFC 4486). On export it is accepted
//     (defensive; config validation already forbids tear-down in export, AC-7).
func handleFilterUpdate(in *sdk.FilterUpdateInput) *sdk.FilterUpdateOutput {
	instP := instancesByName.Load()
	if instP == nil {
		logger().Warn("filter-update before configure", "filter", in.Filter, "peer", in.Peer)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}
	inst, ok := (*instP)[in.Filter]
	if !ok {
		logger().Warn("unknown family-filter", "filter", in.Filter, "peer", in.Peer)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}

	payload := in.Raw
	if len(payload) < 4 {
		logger().Debug("filter-family: missing/short raw body, accepting", "filter", in.Filter, "peer", in.Peer)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}
	}

	fam, fromMP, ok := familyFromPayload(payload)
	if !ok {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept} // malformed; leave untouched
	}
	if fam != inst.family {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept} // AC-6: no match
	}

	if inst.action == actionTearDown {
		if in.Direction != dirImport {
			return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept} // export: forbidden, accept defensively
		}
		logger().Info("filter-family tear-down", "filter", in.Filter, "peer", in.Peer, "family", fam.String())
		return &sdk.FilterUpdateOutput{
			Action:        sdk.FilterReject,
			Teardown:      true,
			NotifyCode:    uint8(message.NotifyCease),
			NotifySubcode: message.NotifyCeaseConnectionRejected,
		}
	}

	// actionRemove.
	if !fromMP {
		// Matched ipv4/unicast: a no-MP UPDATE is entirely this family, so removing
		// it empties the UPDATE — suppress it whole.
		logger().Info("filter-family remove (suppress unicast)", "filter", in.Filter, "peer", in.Peer, "family", fam.String())
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}
	newPayload, emptied, ok := stripMPAttrs(payload)
	if !ok {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept} // malformed; leave untouched
	}
	if emptied {
		logger().Info("filter-family remove (suppress)", "filter", in.Filter, "peer", in.Peer, "family", fam.String())
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}
	logger().Info("filter-family remove (strip MP)", "filter", in.Filter, "peer", in.Peer, "family", fam.String())
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Raw: newPayload}
}
