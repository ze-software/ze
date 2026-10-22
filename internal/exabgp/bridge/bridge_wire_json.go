// Design: docs/architecture/exabgp-bridge.md -- the ExaBGP JSON the bridge writes
// Related: bridge_event.go -- Event.ExabgpJSON, the one UPDATE renderer this feeds
// Related: bridge_event_wire.go -- overlayWire, which reads the members ze's JSON states lossily
//
// The file carries the BGP gate because it parses an UPDATE with the BGP
// engine: it names internal/component/bgp/types, and that package is listed
// under ze_bgp so an always-on importer cannot pin the BGP engine into a build
// compiled without it (feature-gates.txt, the ze_bgp section).

//go:build ze_bgp

package bridge

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/format"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
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

	// Encoding is what the session NEGOTIATED, and it decides how the UPDATE's
	// own bytes are read rather than how they are described.
	//
	// RFC 7911 Section 3 puts a 4-octet Path Identifier ahead of each NLRI when
	// ADD-PATH was negotiated for that family, and nothing in the message says
	// so: a decoder that does not know reads the identifier's octets as prefix
	// bytes and produces routes the peer never sent. A nil value means no
	// capability beyond four-octet AS numbers, which is what an API-originated
	// message carries.
	Encoding *capability.EncodingCaps
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
	contextID, err := sessionContextID(facts)
	if err != nil {
		return nil, err
	}
	var wire wireu.WireUpdate
	wireu.InitWireUpdate(&wire, payload, contextID)
	attrs, attrErr := wire.Attrs()
	if attrErr != nil {
		return nil, fmt.Errorf("exabgp-bridge: parse update attributes: %w", attrErr)
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

	// The parsed ze document, with the UPDATE's bytes beside it as a `full`
	// event carries them, so the one ExaBGP renderer (Event.ExabgpJSON) reads
	// both, as it does for the runner's events.
	rendered := format.AppendMessage(nil, peer, message, bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON,
		Format:   plugin.FormatParsed,
	})
	var zebgp map[string]any
	if err := json.Unmarshal(rendered, &zebgp); err != nil {
		return nil, fmt.Errorf("exabgp-bridge: the ze document does not parse: %w", err)
	}
	event := ReadEvent(zebgp)
	event.Packet = Packet{code: packetCodes[msgTypeUpdate], body: hex.EncodeToString(payload)}
	event.attrContext = contextID
	return event.ExabgpJSON(FormatV6JSON), nil
}

// sessionContextID answers the registered EncodingContext for what the session
// negotiated, and the API context when it negotiated nothing.
//
// The direction is SEND, read from the speaker whose message this is: RFC 7911
// Section 4 makes ADD-PATH asymmetric, so the question the decoder needs
// answered is whether that speaker SENDS path identifiers for the family, not
// whether it would accept them.
func sessionContextID(facts SessionFacts) (bgpctx.ContextID, error) {
	if facts.Encoding == nil {
		return bgpctx.APIContextID, nil
	}
	id, err := bgpctx.Registry.Register(
		bgpctx.NewEncodingContext(nil, facts.Encoding, bgpctx.DirectionSend),
	)
	if err != nil {
		return bgpctx.APIContextID, fmt.Errorf("exabgp-bridge: register session context: %w", err)
	}
	return id, nil
}

// exabgpUpdateObject answers the `update` object of a rendered document, and
// false when the document states none, which every non-UPDATE message does.
func exabgpUpdateObject(document map[string]any) (map[string]any, bool) {
	neighbor, ok := document["neighbor"].(map[string]any)
	if !ok {
		return nil, false
	}
	message, ok := neighbor["message"].(map[string]any)
	if !ok {
		return nil, false
	}
	update, ok := message["update"].(map[string]any)
	return update, ok
}

// exabgpVolatileMembers are the top-level members that differ between two
// correct runs, so a comparison drops them rather than pinning them.
//
// It is upstream's own list: qa/bin/functional's _cleanup removes exactly these
// before it compares two ExaBGP documents. `counter` is theirs too and ze writes
// none; it is listed so a future line carrying one is dropped rather than newly
// mismatched.
var exabgpVolatileMembers = []string{"exabgp", exabgpMemberTime, "host", "pid", "ppid", "counter"}

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
		neighbor["direction"] = modeReceive
	case "out":
		neighbor["direction"] = modeSend
	}
	return document
}
