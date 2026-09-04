// Design: docs/architecture/core-design.md -- BGP redistribution source bridge
// Related: internal/component/bgp/plugins/rib/events -- BGP best-path changes
// Related: internal/core/redistevents -- generic redistribution route-change events

package redistribute

import (
	"log/slog"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/config/redistribute"
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
	// Carried, not dropped. A zero ReplayID is the incremental path. A nonzero
	// one answers a replay request the orchestrator correlates back to one
	// target. Dropping it would turn a replay into a fan-out to every consumer,
	// re-adding a route each of them already holds.
	out.ReplayID = in.ReplayID
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

// LocRIBPlugin is the registry name of the plugin that holds the Loc-RIB whose
// best-path changes EmitBestChange publishes. A `redistribute` rule naming a
// source this package registers produces nothing until that plugin receives the
// peer's UPDATEs. The derived process binding in
// internal/component/bgp/config/redistribute_binding.go grants them.
//
// The name is spelled here rather than imported, because the BGP engine never
// imports a plugin (ai/rules/architecture.md).
// TestLocRIBPluginNamesARegisteredPlugin compares this copy against the
// registry row it names, so it cannot drift from
// internal/component/bgp/plugins/rib/register.go.
const LocRIBPlugin = "bgp-rib"

// OrchestratorPlugin is the registry name of the plugin that dispatches a
// redistribution batch to a consumer. A `destination bgp` rule reaches a peer's
// wire through that plugin.
//
// A peer grants a process the right to put a message on its wire with
// `attach process <name> { send [ update ] }`
// (Peer.maySend, internal/component/bgp/reactor/send_permission.go).
//
// The name is spelled here for the reason LocRIBPlugin is, and
// TestOrchestratorPluginNamesARegisteredPlugin holds it to the registry row.
const OrchestratorPlugin = "redistribute-orchestrator"

// DestinationIsBGP reports whether a `redistribute` destination name is the
// consumer this package registers, meaning routes bound for a BGP peer's wire.
func DestinationIsBGP(name string) bool {
	return name == bgpConsumerName
}

// SourceIsBGP reports whether a `redistribute` source name resolves to routes
// this package produces, meaning the Loc-RIB's best paths.
//
// It asks the source registry rather than comparing against the three names
// RegisterBGPSources writes. A source a later BGP component registers under the
// same protocol is then answered here, and this function stays as it is
// (ai/rules/principles.md, registration over enumeration).
func SourceIsBGP(name string) bool {
	src, ok := redistribute.LookupSource(name)
	if !ok {
		return false
	}
	return src.Protocol == protocolName
}
