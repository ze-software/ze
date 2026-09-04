// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS attribute selection
// RFC: rfc/short/rfc2866.md -- Table of Attributes (Section 5.13),
// Acct-Terminate-Cause (Section 5.10), Acct-Delay-Time (Section 5.2)
// RFC: rfc/short/rfc2865.md -- Framed-IP-Address (Section 5.8),
// Calling-Station-Id (Section 5.31)
// RFC: rfc/short/rfc2869.md -- NAS-Port-Id (Section 5.17), Event-Timestamp (Section 5.3)
// Related: config.go -- parseConfigFromTree reads the container this parses
// Related: acct.go -- buildAcctPacket filters the accounting list
// Related: handler.go -- buildAccessRequestAttrs filters the Access-Request list

package l2tpauthradius

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/core/configvalue"
)

// packetKind names one packet Ze sends a subscriber's attributes in. Zero is
// packetKindUnspecified, so a field nobody wrote never reads as a real packet.
type packetKind uint8

const (
	packetKindUnspecified packetKind = iota
	packetAccessRequest
	packetAccountingStart
	packetAccountingInterim
	packetAccountingStop
)

// packetKindSet holds one bit per packetKind. Bit zero belongs to
// packetKindUnspecified and is never set, so an empty set names no packet.
type packetKindSet uint8

// packetKindSetAll names every packet kind Ze sends. It is what a bare
// exclusion means: hold the attribute back wherever it would appear. An
// attribute Ze never puts in a given packet is not in that packet's list, so
// naming a kind the attribute cannot reach removes nothing.
const packetKindSetAll = packetKindSet(1<<packetAccessRequest |
	1<<packetAccountingStart |
	1<<packetAccountingInterim |
	1<<packetAccountingStop)

func (s packetKindSet) has(kind packetKind) bool {
	return s&(1<<kind) != 0
}

func (s packetKindSet) with(kind packetKind) packetKindSet {
	return s | 1<<kind
}

// attributeExclusions maps a RADIUS attribute type to the packet kinds an
// operator held it back from. An attribute nobody named is absent from the map,
// and the zero packetKindSet a missing key returns names no packet: absence and
// "excluded from nothing" are the same answer here on purpose, because both
// mean Ze sends the attribute.
//
// A nil map is the deployment that configured no `attributes exclude` container
// at all, and it holds nothing back.
type attributeExclusions map[uint8]packetKindSet

// excludableAttributes maps a container name under `attributes exclude` to the
// RADIUS attribute type it holds back. The YANG module declares the same six
// names (yang/ze-l2tp-auth-radius-conf.yang), and it refuses every other word
// before this map is read, Acct-Status-Type, Acct-Session-Id and the NAS
// identity among them: RFC 2866 Section 5.13 makes those mandatory in an
// Accounting-Request.
//
// A name the schema admits and this map lacks is an error rather than a skipped
// entry, so a leaf added to the module without a line here fails the
// configuration load instead of silently holding nothing back.
var excludableAttributes = map[string]uint8{
	"calling-station-id":   radius.AttrCallingStationID,
	"event-timestamp":      radius.AttrEventTimestamp,
	"acct-delay-time":      radius.AttrAcctDelayTime,
	"acct-terminate-cause": radius.AttrAcctTerminateCause,
	"nas-port-id":          radius.AttrNASPortID,
	"framed-ip-address":    radius.AttrFramedIPAddress,
}

// excludablePacketKinds maps a `packet-type` leaf-list member to its kind. Each
// attribute's leaf-list enumerates only the kinds that attribute can reach, so
// the schema has already refused an illegal pair by the time this is read.
var excludablePacketKinds = map[string]packetKind{
	"access-request":     packetAccessRequest,
	"accounting-start":   packetAccountingStart,
	"accounting-interim": packetAccountingInterim,
	"accounting-stop":    packetAccountingStop,
}

// acctPacketKind names the record type an Acct-Status-Type stands for.
//
// Ze sends Start, Interim-Update and Stop and no other status (radius/dict.go
// defines those three alone), so the last return is unreachable today. It
// answers packetKindUnspecified, which no exclusion ever names, so a status
// added later without a kind beside it sends every attribute rather than
// dropping one the operator did not ask to drop.
func acctPacketKind(statusType uint8) packetKind {
	switch statusType {
	case radius.AcctStatusStart:
		return packetAccountingStart
	case radius.AcctStatusInterimUpdate:
		return packetAccountingInterim
	case radius.AcctStatusStop:
		return packetAccountingStop
	}
	return packetKindUnspecified
}

// excludes reports whether the operator held attrType back from kind.
func (e attributeExclusions) excludes(attrType uint8, kind packetKind) bool {
	return e[attrType].has(kind)
}

// filter removes from attrs every attribute the operator held back from kind,
// and keeps the relative order of the rest.
//
// This is the ONE place a built attribute list is filtered. A condition on each
// append would grow with the attribute list, and it would let an attribute
// added later miss the feature with nothing to say so.
//
// It writes over the slice it is given, so the caller MUST own attrs and MUST
// NOT hold a second reference to it.
func (e attributeExclusions) filter(attrs []radius.Attr, kind packetKind) []radius.Attr {
	if len(e) == 0 {
		return attrs
	}
	kept := attrs[:0]
	for _, attr := range attrs {
		if e.excludes(attr.Type, kind) {
			continue
		}
		kept = append(kept, attr)
	}
	return kept
}

// parseAttributeExclusions reads the `attributes exclude` container into the
// set both packet builders ask. It answers a nil map when the operator
// configured no container, which is the deployment that sends everything.
func parseAttributeExclusions(radiusBlock map[string]any) (attributeExclusions, error) {
	rawAttributes, present := radiusBlock["attributes"]
	if !present || rawAttributes == nil {
		return nil, nil //nolint:nilnil // a nil map is the answer: this deployment holds nothing back
	}
	// A node the schema declares as a container arrives as a map. Anything else
	// is a delivery Ze does not understand, and it is an error rather than an
	// empty answer: an operator who wrote a container and got no exclusions
	// would have no way to tell that from a container Ze silently dropped.
	attributes, ok := rawAttributes.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: attributes: unexpected type %T", Name, rawAttributes)
	}
	rawExclude, present := attributes["exclude"]
	if !present || rawExclude == nil {
		return nil, nil //nolint:nilnil // a nil map is the answer: this deployment holds nothing back
	}
	excluded, ok := rawExclude.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: attributes exclude: unexpected type %T", Name, rawExclude)
	}

	exclusions := make(attributeExclusions, len(excluded))
	for name, raw := range excluded {
		attrType, known := excludableAttributes[name]
		if !known {
			return nil, fmt.Errorf("%s: attributes exclude: unknown attribute %q", Name, name)
		}
		kinds, err := parseExcludedPacketKinds(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: attributes exclude %s: %w", Name, name, err)
		}
		exclusions[attrType] = kinds
	}

	// An empty container is the deployment that named nothing, and it answers
	// the same nil map an absent container does.
	if len(exclusions) == 0 {
		return nil, nil //nolint:nilnil // a nil map is the answer: this deployment holds nothing back
	}
	return exclusions, nil
}

// parseExcludedPacketKinds reads one attribute's entry.
//
// The flag form, `calling-station-id;`, arrives as the string "true" and names
// every packet kind. The block form arrives as a map whose packet-type
// leaf-list names some of them, and an empty leaf-list is the flag form written
// the long way. A presence container also accepts `name word;`, which this
// surface gives no meaning, so any other scalar is refused rather than read as
// one of the two forms.
func parseExcludedPacketKinds(raw any) (packetKindSet, error) {
	switch entry := raw.(type) {
	case map[string]any:
		members := configvalue.LeafList(entry["packet-type"])
		if len(members) == 0 {
			return packetKindSetAll, nil
		}
		var kinds packetKindSet
		for _, member := range members {
			kind, known := excludablePacketKinds[member]
			if !known {
				return 0, fmt.Errorf("unknown packet type %q", member)
			}
			kinds = kinds.with(kind)
		}
		return kinds, nil
	case string:
		if entry == "true" {
			return packetKindSetAll, nil
		}
		return 0, fmt.Errorf("takes no value %q; write the name alone, or a packet-type list under it", entry)
	}
	return 0, fmt.Errorf("unexpected type %T", raw)
}
