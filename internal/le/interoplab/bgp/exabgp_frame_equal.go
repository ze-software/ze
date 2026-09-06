// Design: docs/architecture/testing/interop.md -- comparing what the speaker sent
// Overview: exabgp_server.go -- the mock peer that reads a `.ci` expectation
//
// Two UPDATEs carrying the same path attributes in a different ORDER are the same
// UPDATE. RFC 4271 Section 5 states no order over the attribute block, and the
// ported ExaBGP fixtures do not agree with each other about one: conf-vpn.ci
// writes MP_REACH_NLRI in ascending type-code position and api-attributes-vpn.ci
// writes it last, because upstream's config path and its API path build the block
// differently.
//
// So a byte comparison of the whole frame asserts an order neither the RFC nor
// the corpus fixes. This file compares the attributes as a SET instead, and
// nothing else: every attribute's flags, type, length and value must still match
// exactly, and so must the withdrawn-routes and NLRI fields. A frame that differs
// in any byte other than the order of whole attributes still fails.

package bgp

import (
	"bytes"
	"encoding/binary"
	"slices"
)

// bgpFrameEqual reports whether two BGP frames say the same thing.
//
// It is byte equality first, and equality up to attribute ORDER second. Anything
// that is not an UPDATE, and any UPDATE this cannot decode, falls back to byte
// equality: a comparator that guessed at a malformed frame would answer about a
// frame neither side wrote.
func bgpFrameEqual(actual, wanted []byte) bool {
	if bytes.Equal(actual, wanted) {
		return true
	}
	actualParts, ok := splitUpdateFrame(actual)
	if !ok {
		return false
	}
	wantedParts, ok := splitUpdateFrame(wanted)
	if !ok {
		return false
	}
	if !bytes.Equal(actualParts.withdrawn, wantedParts.withdrawn) {
		return false
	}
	if !bytes.Equal(actualParts.nlri, wantedParts.nlri) {
		return false
	}
	return slices.EqualFunc(sortedAttributes(actualParts.attributes),
		sortedAttributes(wantedParts.attributes), bytes.Equal)
}

// updateFrameParts is one UPDATE cut into the three fields RFC 4271 Section 4.3
// gives it: the withdrawn routes, the path attributes, and the reachable NLRI.
type updateFrameParts struct {
	withdrawn  []byte
	attributes []byte
	nlri       []byte
}

// splitUpdateFrame cuts an UPDATE into its three fields. It reports false for a
// frame that is not an UPDATE, and for one whose declared lengths do not fit.
func splitUpdateFrame(frame []byte) (updateFrameParts, bool) {
	if len(frame) < bgpHeaderLength || frame[18] != bgpUpdate {
		return updateFrameParts{}, false
	}
	body := frame[bgpHeaderLength:]
	if len(body) < 2 {
		return updateFrameParts{}, false
	}
	withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
	if len(body) < 2+withdrawnLen+2 {
		return updateFrameParts{}, false
	}
	withdrawn := body[2 : 2+withdrawnLen]

	attrStart := 2 + withdrawnLen
	attrLen := int(binary.BigEndian.Uint16(body[attrStart : attrStart+2]))
	attrStart += 2
	if len(body) < attrStart+attrLen {
		return updateFrameParts{}, false
	}
	return updateFrameParts{
		withdrawn:  withdrawn,
		attributes: body[attrStart : attrStart+attrLen],
		nlri:       body[attrStart+attrLen:],
	}, true
}

// sortedAttributes cuts an attribute block into whole attributes and sorts them,
// so two blocks holding the same attributes compare equal whatever order each
// speaker wrote them in.
//
// A block it cannot walk answers as one opaque element, so the caller compares
// it byte for byte rather than pretending to have understood it.
func sortedAttributes(block []byte) [][]byte {
	attributes := make([][]byte, 0, 8)
	for offset := 0; offset < len(block); {
		if len(block)-offset < 3 {
			return [][]byte{block}
		}
		flags := block[offset]
		length := int(block[offset+2])
		header := 3
		// RFC 4271 Section 4.3: the fourth high-order bit of the Attribute Flags
		// octet is the Extended Length bit, and it makes the length two octets.
		if flags&0x10 != 0 {
			if len(block)-offset < 4 {
				return [][]byte{block}
			}
			length = int(binary.BigEndian.Uint16(block[offset+2 : offset+4]))
			header = 4
		}
		end := offset + header + length
		if end > len(block) {
			return [][]byte{block}
		}
		attributes = append(attributes, block[offset:end])
		offset = end
	}
	slices.SortFunc(attributes, bytes.Compare)
	return attributes
}
