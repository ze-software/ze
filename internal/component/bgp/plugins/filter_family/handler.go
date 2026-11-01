// Design: docs/architecture/core-design.md -- family-filter per-update decision
// Related: match.go -- family extraction + whether the family is all the UPDATE carries
// Related: config.go -- family-filter instance parsing
// RFC: rfc/short/rfc4760.md -- multiprotocol family removal (Section 6)
// RFC: rfc/short/rfc4271.md -- NOTIFICATION on tear-down (Section 6)
// RFC: rfc/short/rfc4486.md -- Cease / Connection Rejected subcode

package filter_family

import (
	"encoding/json"
	"strconv"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// dirImport is the route direction (rpc.FilterRoute.Direction) of a received UPDATE.
const dirImport = "import"

// evaluateFamilyFilter judges one route against the named instance, on
// either transport, from the raw body it declared. It reads only the
// instance set installFamilyFilters swaps whole, so it is safe for
// concurrent use and never blocks. It answers every case with a verdict:
//
//   - unknown instance / configure-before-load: reject (fail-closed).
//   - missing or malformed raw body: accept (cannot inspect; BGP-layer validation
//     still applies, and we must not drop valid traffic on a decode failure).
//   - family does not match: accept (AC-6).
//   - remove + match: reject when removal empties the UPDATE (RFC 4760 §6 MP-only,
//     AC-2/AC-4) or a no-MP ipv4/unicast match; otherwise modify with the
//     family-strip intent, which the engine resolves into the body without
//     the family's MP attributes (AC-3).
//   - tear-down + match (import only): reject + Teardown with a Cease / Connection
//     Rejected NOTIFICATION (RFC 4271, RFC 4486). On export it is accepted
//     (defensive; config validation already forbids tear-down in export, AC-7).
func evaluateFamilyFilter(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
	return judgeFamily(route), nil
}

// judgeFamily is evaluateFamilyFilter's verdict.
func judgeFamily(in *sdk.FilterRoute) *sdk.FilterUpdateOutput {
	instP := instancesByName.Load()
	if instP == nil {
		logger().Warn("filter-update before configure", "filter", in.Filter(), "peer", in.Peer())
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}
	inst, ok := (*instP)[in.Filter()]
	if !ok {
		logger().Warn("unknown family-filter", "filter", in.Filter(), "peer", in.Peer())
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}

	payload := in.Raw()
	if len(payload) < 4 {
		logger().Debug("filter-family: missing/short raw body, accepting", "filter", in.Filter(), "peer", in.Peer())
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
		if in.Direction() != dirImport {
			return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept} // export: forbidden, accept defensively
		}
		logger().Info("filter-family tear-down", "filter", in.Filter(), "peer", in.Peer(), "family", fam.String())
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
		logger().Info("filter-family remove (suppress unicast)", "filter", in.Filter(), "peer", in.Peer(), "family", fam.String())
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}
	only, ok := carriesOnlyFamily(payload, fam)
	if !ok {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept} // malformed; leave untouched
	}
	if only {
		// Stripping the family empties the UPDATE: suppress it whole. This is a
		// verdict, not a rewrite, because the engine sends what the strip
		// leaves and an UPDATE with no NLRI must not be sent in its place.
		logger().Info("filter-family remove (suppress)", "filter", in.Filter(), "peer", in.Peer(), "family", fam.String())
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}
	// The family-strip intent (D-5): the engine drops the family's
	// MP_REACH_NLRI and MP_UNREACH_NLRI and keeps every other section, so
	// the plugin writes no UPDATE body. The family read admits the intent.
	logger().Info("filter-family remove (strip MP)", "filter", in.Filter(), "peer", in.Peer(), "family", fam.String())
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Sets: []rpc.FilterSet{{
		Field: fieldFamily, Action: rpc.FilterSetActionRemove, Value: familyValue(fam),
	}}}
}

// familyValue answers fam as the JSON string a family remove carries, such
// as "ipv4/flow". A family name is a registered ASCII token, so quoting it is
// its JSON encoding.
func familyValue(fam family.Family) json.RawMessage {
	return strconv.AppendQuote(nil, fam.String())
}
