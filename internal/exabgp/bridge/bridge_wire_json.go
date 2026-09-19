// Design: docs/architecture/exabgp-bridge.md -- the ExaBGP JSON the bridge writes
// Related: bridge_event.go -- ZebgpToExabgpJSON, which this feeds

package bridge

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/component/bgp/format"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
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
	document := ZebgpToExabgpJSON(zebgp)

	// Two members are rebuilt from the PARSED attributes rather than from the
	// document above, because ze's own JSON is lossy for them and the bridge
	// cannot recover what it never received. Reading them here costs nothing:
	// this function already holds the attributes the wire was parsed into.
	//
	// Fixing ze's document instead would change a contract 21 .ci fixtures, 38
	// test files and the looking glass, the CLI decoder, the RIB formatter and
	// the path filters all read. That is a migration, not a rendering fix, and
	// it is not owed here: ze's flattened form is right for a ze reader.
	overlayASPath(document, attrs)
	overlayExtendedCommunities(document, attrs)
	return document, nil
}

// exabgpASPathElements names each AS_PATH segment type the way ExaBGP does
// (src/exabgp/bgp/message/update/attribute/aspath.py).
var exabgpASPathElements = map[attribute.ASPathSegmentType]string{
	attribute.ASSet:            "as-set",
	attribute.ASSequence:       "as-sequence",
	attribute.ASConfedSequence: "as-confed-sequence",
	attribute.ASConfedSet:      "as-confed-set",
}

// overlayASPath replaces the flattened as-path with the SEGMENTS ExaBGP states.
//
// ze writes `[65533]`: every ASN of every segment, in order, with the segment
// boundaries and their types discarded (appendASPathJSON,
// internal/core/bgp/attribute/json.go). A reader of that cannot tell an AS_SET
// from an AS_SEQUENCE, which are different statements about the path: RFC 4271
// Section 4.3 makes a set UNORDERED. ExaBGP keeps both, keyed by segment index:
// {"0": {"element": "as-sequence", "value": [65533]}}.
func overlayASPath(document map[string]any, attrs *attribute.AttributesWire) {
	update, ok := exabgpUpdateObject(document)
	if !ok {
		return
	}
	value, err := attrs.Get(attribute.AttrASPath)
	if err != nil || value == nil {
		return
	}
	path, ok := value.(*attribute.ASPath)
	if !ok {
		return
	}
	segments := make(map[string]any, len(path.Segments))
	for index, segment := range path.Segments {
		asns := make([]any, 0, len(segment.ASNs))
		for _, asn := range segment.ASNs {
			asns = append(asns, float64(asn))
		}
		element, named := exabgpASPathElements[segment.Type]
		if !named {
			// An unnamed segment type is not guessed at: the index is kept so
			// the path's shape survives, and the type is stated as the number
			// the wire carried.
			element = "type-" + strconv.FormatUint(uint64(segment.Type), 10)
		}
		segments[strconv.Itoa(index)] = map[string]any{"element": element, "value": asns}
	}
	if len(segments) == 0 {
		return
	}
	attributeOf(update)["as-path"] = segments
}

// overlayExtendedCommunities restores the numeric value beside each community.
//
// ExaBGP states both halves -- {"string": "target:72:1", "value": 563259191066625}
// -- and ze's document carries the display text alone. The number is the eight
// octets as they sit on the wire, so it is read from the attribute rather than
// reconstructed from the text: a display form is not a wire form, and parsing
// one back is the lossy re-derivation ai/rules/principles.md warns about.
func overlayExtendedCommunities(document map[string]any, attrs *attribute.AttributesWire) {
	update, ok := exabgpUpdateObject(document)
	if !ok {
		return
	}
	raw, err := attrs.GetRaw(attribute.AttrExtCommunity)
	if err != nil || len(raw) == 0 || len(raw)%8 != 0 {
		return
	}
	attrObject := attributeOf(update)
	existing, _ := attrObject["extended-community"].([]any)
	rebuilt := make([]any, 0, len(raw)/8)
	for offset := 0; offset+8 <= len(raw); offset += 8 {
		member := map[string]any{"value": float64(binary.BigEndian.Uint64(raw[offset : offset+8]))}
		// The string ze already rendered, in the order it rendered them, so the
		// two halves describe the same community.
		if index := offset / 8; index < len(existing) {
			if text, isText := existing[index].(string); isText {
				member["string"] = text
			}
		}
		rebuilt = append(rebuilt, member)
	}
	attrObject["extended-community"] = rebuilt
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

// attributeOf answers an update's attribute object, creating it when the update
// carries none, so a caller can write into it unconditionally.
func attributeOf(update map[string]any) map[string]any {
	existing, ok := update["attribute"].(map[string]any)
	if ok {
		return existing
	}
	created := map[string]any{}
	update["attribute"] = created
	return created
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
