package reactor

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// validateQueuedEVPNOrigin applies the wire-builder's sender checks before any
// route enters a disconnected peer's queue. Attributes are already parsed for
// queueing; only RT, ESI Label and ES-Import communities matter to these checks.
func validateQueuedEVPNOrigin(batch bgptypes.NLRIBatch, attrs []attribute.Attribute) error {
	var selected [24]byte
	for _, attr := range attrs {
		communities, ok := attr.(attribute.ExtendedCommunities)
		if !ok {
			continue
		}
		for _, ec := range communities {
			if ec[1] == 2 && (ec[0] == 0 || ec[0] == 1 || ec[0] == 2) {
				copy(selected[:8], ec[:])
			}
			if ec[0] == 6 && ec[1] == 1 {
				copy(selected[8:16], ec[:])
			}
			if ec[0] == 6 && ec[1] == 2 {
				copy(selected[16:], ec[:])
			}
		}
	}
	extCommunities := selected[:]
	if batch.Attrs != nil && len(batch.Attrs.RawWire()) != 0 {
		_, _, extCommunities, _ = attribute.AttrFind(batch.Attrs.RawWire(), attribute.AttrExtCommunity)
	}
	// An EVPN NLRI is a two-octet header followed by at most 255 octets.
	// WriteTo omits an ADD-PATH identifier, even for a WireNLRI carrying one.
	var encoded [257]byte
	for _, route := range batch.NLRIs {
		if route.Len() > len(encoded) {
			return fmt.Errorf("%w: NLRI exceeds the one-octet length field", message.ErrEVPNOrigination)
		}
		n := route.WriteTo(encoded[:], 0)
		if err := message.ValidateEVPNOrigination(encoded[:n], extCommunities, false); err != nil {
			return err
		}
	}
	return nil
}
