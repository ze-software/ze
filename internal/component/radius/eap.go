// Design: docs/guide/radius.md -- RADIUS/EAP admin login
// Related: dict.go -- AttrEAPMessage, the attribute this file frames
// Related: authenticator.go -- the challenge loop that calls both directions
// RFC: rfc/short/rfc3579.md -- Section 3.1 EAP-Message, its ordering and its 253-octet split

package radius

import (
	"errors"
	"fmt"
)

// maxEAPMessageValue is the largest String an EAP-Message attribute can carry.
// RFC 2865 Section 5 gives every attribute a one-octet Length that counts its
// own two header octets, so 255 minus 2.
const maxEAPMessageValue = MaxAttrLen - 2

// appendEAPMessage appends one EAP packet to attrs as one or more consecutive
// EAP-Message attributes.
//
// RFC 3579 Section 3.1: "If multiple EAP-Message attributes are contained
// within an Access-Request or Access-Challenge packet, they MUST be in order
// and they MUST be consecutive attributes in the Access-Request or
// Access-Challenge packet." Appending in one loop, with nothing between the
// pieces, is what makes both true, so no caller may interleave another
// attribute into the run.
//
// The same section explains why the split exists at all: "If multiple
// EAP-Message attributes are present in a packet their values should be
// concatenated; this allows EAP packets longer than 253 octets to be
// transported by RADIUS."
//
// An empty EAP packet is refused rather than sent. RFC 2865 Section 5 states
// that "Text of length zero (0) MUST NOT be sent; omit the entire attribute
// instead", and RFC 3579 Section 3.1 gives EAP-Message a Length of ">= 3",
// which is the two header octets plus at least one octet of String. There is
// also nothing to say: an EAP packet is four octets before its Type.
func appendEAPMessage(attrs []Attr, packet []byte) ([]Attr, error) {
	if len(packet) == 0 {
		return nil, errors.New("radius: refusing to send an empty EAP-Message (RFC 3579 Section 3.1 requires Length >= 3)")
	}
	for off := 0; off < len(packet); off += maxEAPMessageValue {
		end := min(off+maxEAPMessageValue, len(packet))
		attrs = append(attrs, Attr{Type: AttrEAPMessage, Value: packet[off:end]})
	}
	return attrs, nil
}

// eapPacketFrom concatenates the EAP-Message attributes of a received packet
// into the one EAP packet they encode. It returns nil, and no error, when the
// packet carries none.
//
// RFC 3579 Section 3.1: "Where more than one EAP-Message attribute is included,
// it is assumed that the attributes are to be concatenated to form a single EAP
// packet", and "Multiple EAP packets MUST NOT be encoded within EAP-Message
// attributes contained within a single Access-Challenge, Access-Accept,
// Access-Reject or Access-Request packet."
//
// A second RUN of EAP-Message attributes, separated from the first by another
// attribute, is refused. It is the shape a second EAP packet takes on the wire,
// and it is the same shape as a run whose order the sender broke, so the
// requirement above and the ordering requirement are one check. Concatenating
// across the gap would build an EAP packet neither side ever wrote.
func eapPacketFrom(pkt *Packet) ([]byte, error) {
	first, count := -1, 0
	for index, a := range pkt.Attrs {
		if a.Type != AttrEAPMessage {
			continue
		}
		if first < 0 {
			first, count = index, 1
			continue
		}
		if index != first+count {
			return nil, fmt.Errorf(
				"radius: EAP-Message attributes at %d and %d are not consecutive (RFC 3579 Section 3.1)",
				first+count-1, index)
		}
		count++
	}
	if first < 0 {
		return nil, nil
	}

	total := 0
	for _, a := range pkt.Attrs[first : first+count] {
		total += len(a.Value)
	}
	// A run of attributes whose values are all empty decodes to nothing. The
	// decoder would refuse the result anyway, and this says which side is wrong.
	if total == 0 {
		return nil, errors.New("radius: the EAP-Message attributes carry no EAP packet")
	}
	packet := make([]byte, 0, total)
	for _, a := range pkt.Attrs[first : first+count] {
		packet = append(packet, a.Value...)
	}
	return packet, nil
}
