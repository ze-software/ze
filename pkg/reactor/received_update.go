package reactor

import (
	"fmt"
	"net/netip"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/zebgp/pkg/api"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/attribute"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
	"codeberg.org/thomas-mangin/zebgp/pkg/rib"
)

// msgIDCounter generates unique message IDs.
// Atomic for concurrent access from multiple peer goroutines.
var msgIDCounter atomic.Uint64

// nextMsgID returns the next unique message ID.
func nextMsgID() uint64 {
	return msgIDCounter.Add(1)
}

// ReceivedUpdate represents an immutable snapshot of a received UPDATE.
// Each UPDATE gets a unique ID; updates to same NLRI create new IDs.
//
// Memory contract: WireUpdate owns the buffer; all derived slices share it.
// Message ID is stored in WireUpdate, accessible via WireUpdate.MessageID().
type ReceivedUpdate struct {
	// WireUpdate contains the UPDATE payload with zero-copy accessors.
	// Provides Payload(), Attrs(), NLRI(), MPReach(), MPUnreach(), SourceCtxID(), MessageID().
	WireUpdate *api.WireUpdate

	// Announces contains announced NLRIs from this UPDATE.
	Announces []nlri.NLRI

	// Withdraws contains withdrawn NLRIs from this UPDATE.
	Withdraws []nlri.NLRI

	// AnnounceWire contains wire bytes for each announced NLRI.
	// One entry per Announces element, same order.
	AnnounceWire [][]byte

	// WithdrawWire contains wire bytes for each withdrawn NLRI.
	// One entry per Withdraws element, same order.
	WithdrawWire [][]byte

	// SourcePeerIP is the IP address of the peer that sent this UPDATE.
	SourcePeerIP netip.Addr

	// ReceivedAt is when this UPDATE was received.
	ReceivedAt time.Time
}

// ConvertToRoutes extracts individual Routes from this UPDATE.
// Used when storing in adj-rib-out for persistence across reconnects.
//
// Returns nil for withdraw-only UPDATEs (no announcements).
// Returns error if attribute parsing fails.
func (ru *ReceivedUpdate) ConvertToRoutes() ([]*rib.Route, error) {
	if len(ru.Announces) == 0 {
		return nil, nil // Withdraw-only UPDATE
	}

	attrsWire := ru.WireUpdate.Attrs()
	if attrsWire == nil {
		return nil, fmt.Errorf("no attributes for announcement")
	}

	// Parse all attributes
	attrs, err := attrsWire.All()
	if err != nil {
		return nil, fmt.Errorf("parsing attributes: %w", err)
	}

	// Extract NextHop and ASPath, separate from other attributes
	// NextHop can be in:
	// - NextHop attribute (IPv4 unicast, RFC 4271)
	// - MP_REACH_NLRI attribute (IPv6, VPN, etc., RFC 4760)
	var nextHop netip.Addr
	var asPath *attribute.ASPath
	var otherAttrs []attribute.Attribute

	for _, attr := range attrs {
		switch a := attr.(type) {
		case *attribute.NextHop:
			// IPv4 unicast next-hop (RFC 4271)
			nextHop = a.Addr
		case *attribute.MPReachNLRI:
			// IPv6/VPN/etc next-hop (RFC 4760)
			// Use first next-hop if available (primary)
			if len(a.NextHops) > 0 {
				nextHop = a.NextHops[0]
			}
			// MP_REACH_NLRI is NOT included in otherAttrs.
			// buildRIBRouteUpdate creates a fresh one from route.NLRI() and route.NextHop().
		case *attribute.ASPath:
			asPath = a
		default:
			otherAttrs = append(otherAttrs, attr)
		}
	}

	// Create Route per announced NLRI
	routes := make([]*rib.Route, 0, len(ru.Announces))
	for i, n := range ru.Announces {
		var nlriWire []byte
		if i < len(ru.AnnounceWire) {
			nlriWire = ru.AnnounceWire[i]
		}

		route := rib.NewRouteWithWireCacheFull(
			n,
			nextHop,
			otherAttrs,
			asPath,
			attrsWire.Packed(), // Attribute wire cache
			nlriWire,           // NLRI wire cache
			ru.WireUpdate.SourceCtxID(),
		)
		routes = append(routes, route)
	}

	return routes, nil
}
