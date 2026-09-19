// Design: docs/architecture/exabgp-bridge.md -- the ExaBGP JSON the bridge writes
// Related: bridge_event.go -- ZebgpToExabgpJSON, which this feeds

package bridge

import (
	"encoding/json"
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/format"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// SessionFacts is what a renderer cannot read off the wire: the two ends of the
// session and the identifier this speaker presents.
//
// ExaBGP states all of it on every line (_neighbor,
// src/exabgp/reactor/api/response/json.py), and none of it is in an UPDATE's
// bytes, so a caller comparing a rendered frame against a fixture MUST supply
// what that fixture's own configuration set.
type SessionFacts struct {
	Local    netip.Addr
	Peer     netip.Addr
	LocalAS  uint32
	PeerAS   uint32
	RouterID uint32
}

// WireUpdateToExabgpJSON renders one UPDATE's wire body as the ExaBGP JSON
// document a process attached to that peer would receive.
//
// It exists so a fixture's `:json:` expectation can be checked against the frame
// the speaker actually sent. Until it did, those expectations were read and
// dropped (readExaBGPCase, internal/le/interoplab/bgp/exabgp_server.go), which
// is why two defects in this file's own output reached a verification sweep
// unseen.
//
// The payload is the UPDATE without its 19-octet header, which is what a `.ci`
// raw line carries after its length and type fields.
func WireUpdateToExabgpJSON(payload []byte, facts SessionFacts, direction rpc.MessageDirection) (map[string]any, error) {
	// The API context, not a zero ContextID. An unregistered id resolves to no
	// EncodingContext, so the attribute block decodes to nothing and the render
	// comes out with no attributes and a null next hop -- which looks like a
	// defect in the renderer's subject rather than in its setup.
	var wire wireu.WireUpdate
	wireu.InitWireUpdate(&wire, payload, bgpctx.APIContextID)
	attrs, err := wire.Attrs()
	if err != nil {
		return nil, fmt.Errorf("exabgp-bridge: parse update attributes: %w", err)
	}

	peer := &plugin.PeerInfo{
		Address:         facts.Peer,
		LocalAddress:    facts.Local,
		AddressStr:      facts.Peer.String(),
		LocalAddressStr: facts.Local.String(),
		LocalAS:         facts.LocalAS,
		PeerAS:          facts.PeerAS,
		RouterID:        facts.RouterID,
	}
	message := bgptypes.RawMessage{
		Type:       msgtype.TypeUPDATE,
		RawBytes:   payload,
		Direction:  direction,
		WireUpdate: &wire,
		AttrsWire:  attrs,
	}

	// The ze-native document first, in the one encoding ZebgpToExabgpJSON reads.
	rendered := format.AppendMessage(nil, peer, message, bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON,
		Format:   plugin.FormatParsed,
	})
	var zebgp map[string]any
	if err := json.Unmarshal(rendered, &zebgp); err != nil {
		return nil, fmt.Errorf("exabgp-bridge: the ze document does not parse: %w", err)
	}
	return ZebgpToExabgpJSON(zebgp), nil
}

// exabgpVolatileMembers are the top-level members that differ between two
// correct runs, so a comparison drops them rather than pinning them.
//
// It is upstream's own list: qa/bin/functional's _cleanup removes exactly these
// before it compares two ExaBGP documents. `counter` is theirs too and ze writes
// none; it is listed so a future line carrying one is dropped rather than newly
// mismatched.
var exabgpVolatileMembers = []string{"exabgp", "time", "host", "pid", "ppid", "counter"}

// DropVolatile removes the members no two runs agree on, in place, and answers
// the same map so a caller can chain it.
//
// It also folds `direction`, which is NOT volatile but is stated in two
// vocabularies by one project. The ExaBGP DAEMON writes `receive` and `send`
// (src/exabgp/reactor/protocol.py) and so does ze; the `in` and `out` that
// appear in test/exabgp-compat fixtures come from upstream's own test harness
// (qa/bin/test_json, which calls encoder.update(neighbor, 'in', ...)). Folding
// them is what lets a fixture be compared at all, and it is the honest
// direction to fold: teaching ze to write `in` would make it disagree with the
// daemon it exists to be compatible with.
func DropVolatile(document map[string]any) map[string]any {
	for _, member := range exabgpVolatileMembers {
		delete(document, member)
	}
	neighbor, ok := document["neighbor"].(map[string]any)
	if !ok {
		return document
	}
	switch neighbor["direction"] {
	case "in":
		neighbor["direction"] = "receive"
	case "out":
		neighbor["direction"] = "send"
	}
	return document
}
