// Design: docs/architecture/core-design.md -- BGP redistribution source bridge
// Related: internal/component/bgp/plugins/rib/events -- BGP best-path changes
// Related: internal/core/redistevents -- generic redistribution route-change events

package redistribute

import (
	ribevents "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/events"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

const protocolName = "bgp"

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
	case bgptypes.RouteActionAdd, bgptypes.RouteActionUpdate:
		action = redistevents.ActionAdd
	case bgptypes.RouteActionWithdraw:
		action = redistevents.ActionRemove
	default:
		return redistevents.RouteChangeEntry{}, false
	}
	return redistevents.RouteChangeEntry{
		Action:  action,
		Prefix:  in.Prefix,
		NextHop: in.NextHop,
	}, true
}
