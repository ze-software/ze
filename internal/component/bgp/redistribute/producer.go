// Design: docs/architecture/core-design.md -- BGP redistribution source bridge
// Related: internal/component/bgp/plugins/rib/events -- BGP best-path changes
// Related: internal/core/redistevents -- generic redistribution route-change events

package redistribute

import (
	"log/slog"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/pkg/ze"
)

const protocolName = "bgp"

// unknownActionSkips counts best-change entries the bridge could not map to a
// redistribution action (an action outside add/update/withdraw). Such entries
// are skipped with a warn log rather than the old silent drop, so a future
// RouteAction enumerant reaching the bridge surfaces loudly instead of
// vanishing from redistribution (spec R-2). Package-level atomic because the
// bridge has no injected metrics registry; the warn log is the production
// signal and this counter is the assertable evidence that the skip was counted,
// not silently dropped (read by the same-package bridge test).
var unknownActionSkips atomic.Uint64

// ProtocolID is the generic redistribution protocol identity for BGP best paths.
var ProtocolID = redistevents.RegisterProtocol(protocolName)

// RouteChange is the generic redistribution event emitted from BGP RIB best changes.
var RouteChange = events.Register[*redistevents.RouteChangeBatch](protocolName, redistevents.EventType)

func init() {
	redistevents.RegisterProducer(ProtocolID)
}

func EmitBestChange(bus ze.EventBus, in *ribevents.BestChangeBatch) {
	if bus == nil || in == nil || len(in.Changes) == 0 {
		return
	}
	out := redistevents.AcquireBatch()
	defer redistevents.ReleaseBatch(out)
	out.Protocol = ProtocolID
	out.AFI = uint16(in.Family.AFI)
	out.SAFI = uint8(in.Family.SAFI)
	for i := range in.Changes {
		entry, ok := convertBestChange(&in.Changes[i])
		if ok {
			out.Entries = append(out.Entries, entry)
		}
	}
	if len(out.Entries) == 0 {
		return
	}
	_, _ = RouteChange.Emit(bus, out)
}

func convertBestChange(in *ribevents.BestChangeEntry) (redistevents.RouteChangeEntry, bool) {
	if in == nil || !in.Prefix.IsValid() {
		return redistevents.RouteChangeEntry{}, false
	}
	var action redistevents.RouteAction
	switch in.Action {
	case routeaction.Add, routeaction.Update:
		action = redistevents.ActionAdd
	case routeaction.Withdraw:
		action = redistevents.ActionRemove
	default:
		// Not add/update/withdraw: the bridge cannot map it to a redistribution
		// action. Skip loudly (count + warn) instead of the old silent drop, so
		// a RouteAction that reaches the bridge unmapped fails visibly rather
		// than vanishing from redistribution with no diagnostic (spec R-2).
		unknownActionSkips.Add(1)
		// Log the raw code alongside the stringer: an unmapped enumerant's
		// String() falls back to "unspecified", so the numeric code is what
		// actually identifies the offending action.
		slog.Warn("bgp redistribute bridge: unmapped best-change action, skipping entry",
			"action", in.Action, "action_code", uint8(in.Action), "prefix", in.Prefix)
		return redistevents.RouteChangeEntry{}, false
	}
	// Carry every field the source populates. Metric and OriginAS are set on
	// add/update by the RIB best-path selection (rib_bestchange.go) and are 0 on
	// withdraw (where the consumer ignores them); copying unconditionally keeps
	// the bridge lossless without a per-action branch.
	return redistevents.RouteChangeEntry{
		Action:   action,
		Prefix:   in.Prefix,
		NextHop:  in.NextHop,
		Metric:   in.Metric,
		OriginAS: in.OriginAS,
	}, true
}
