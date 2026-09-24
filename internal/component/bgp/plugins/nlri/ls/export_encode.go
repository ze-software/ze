// Design: docs/architecture/wire/nlri-bgpls.md -- native routing-state origination
// RFC: rfc/short/rfc9552.md -- topology identity and opaque provenance

package ls

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/core/linkstateevents"
)

// exportedRoute owns both wire slices. No source database memory escapes the
// synchronous snapshot handler. NLRI bytes are the route identity.
type exportedRoute struct {
	nlri       []byte
	attributes []byte
}

type originIdentity struct {
	asn, bgplsID, area, member, bgpID uint32
	flags                             uint8
	router                            string
}

// Ignore descriptor fields which are absent on the wire, so an area-less
// AS-external origin has the same identity wherever it appears in the database.
func nativeOrigin(id linkstateevents.NodeID) originIdentity {
	key := originIdentity{asn: id.ASN, member: id.Confederation, router: string(id.RouterID)}
	if id.HasBGPLSID {
		key.bgplsID = id.BGPLSID
		key.flags |= 1
	}
	if id.HasArea {
		key.area = id.Area
		key.flags |= 2
	}
	if id.BGPRouterID.Is4() {
		addr := id.BGPRouterID.As4()
		key.bgpID = binary.BigEndian.Uint32(addr[:])
	}
	return key
}

func encodeTopology(snapshot *linkstateevents.Snapshot) (map[string]exportedRoute, error) {
	if snapshot.Domain.Protocol < linkstateevents.ISISLevel1 || snapshot.Domain.Protocol > linkstateevents.BGP {
		return nil, fmt.Errorf("unsupported native link-state protocol %d", snapshot.Domain.Protocol)
	}
	if err := validateNativeEPE(snapshot); err != nil {
		return nil, err
	}
	routes := make(map[string]exportedRoute, len(snapshot.Nodes)+len(snapshot.Links)+len(snapshot.Prefixes))
	protocol := BGPLSProtocolID(snapshot.Domain.Protocol)
	identifier := snapshot.Domain.Identifier
	var unreachable map[originIdentity]struct{}
	switch snapshot.Domain.Protocol {
	case linkstateevents.ISISLevel1, linkstateevents.ISISLevel2, linkstateevents.OSPFv2, linkstateevents.OSPFv3:
		if len(snapshot.Unreachable) != 0 {
			unreachable = make(map[originIdentity]struct{}, len(snapshot.Unreachable))
			for _, id := range snapshot.Unreachable {
				unreachable[nativeOrigin(id)] = struct{}{}
			}
		}
	default:
		// Direct, static and BGP domains report no unreachable IGP nodes.
	}
	excluded := func(id linkstateevents.NodeID) bool {
		if len(unreachable) == 0 {
			return false
		}
		_, found := unreachable[nativeOrigin(id)]
		return found
	}
	add := func(wire, encoded []byte) error {
		key := string(wire)
		if previous, exists := routes[key]; exists {
			if !bytes.Equal(previous.attributes, encoded) {
				return errors.New("conflicting attributes for one native topology identity")
			}
		} else if len(routes) == exportRouteLimit {
			return errors.New("native topology expansion exceeds route bound")
		}
		routes[key] = exportedRoute{nlri: wire, attributes: encoded}
		return nil
	}
	for _, node := range snapshot.Nodes {
		if excluded(node.ID) {
			continue
		}
		attributes, err := originateAttributes(snapshot.Domain.Protocol, 1, node.Attributes, node.Opaque)
		if err != nil {
			return nil, err
		}
		nlri := NewBGPLSNode(protocol, identifier, exportNodeID(node.ID))
		if err := add(nlri.Bytes(), attributes); err != nil {
			return nil, err
		}
	}
	for _, link := range snapshot.Links {
		// The advertising node owns the object. A reachable node may still
		// describe a half-link to an unreachable neighbor (RFC 9552 §5.9).
		if excluded(link.Local) {
			continue
		}
		// Topology identities differ; their immutable attribute bytes do not.
		attributes, err := originateAttributes(snapshot.Domain.Protocol, 2, link.Attributes, link.Opaque)
		if err != nil {
			return nil, err
		}
		topologies := link.Topologies
		if len(topologies) == 0 {
			topologies = []uint16{0}
		}
		for i, topology := range topologies {
			if slices.Contains(topologies[:i], topology) {
				continue
			}
			if err := validateExportTopology(snapshot.Domain.Protocol, topology); err != nil {
				return nil, err
			}
			wire, err := encodeNativeLink(protocol, identifier, link, topology)
			if err != nil {
				return nil, err
			}
			if err := add(wire, attributes); err != nil {
				return nil, err
			}
		}
	}
	for _, prefix := range snapshot.Prefixes {
		if excluded(prefix.Node) {
			continue
		}
		if !prefix.Prefix.IsValid() {
			return nil, errors.New("invalid native topology prefix")
		}
		if err := validateExportTopology(snapshot.Domain.Protocol, prefix.Topology); err != nil {
			return nil, err
		}
		masked := prefix.Prefix.Masked()
		reachability := make([]byte, 1+(masked.Bits()+7)/8)
		reachability[0] = byte(masked.Bits())
		copy(reachability[1:], masked.Addr().AsSlice())
		desc := PrefixDescriptor{MultiTopologyID: prefix.Topology, HasMultiTopologyID: true,
			OSPFRouteType: prefix.RouteType, IPReachabilityInfo: reachability}
		var nlri *BGPLSPrefix
		if masked.Addr().Is4() {
			nlri = NewBGPLSPrefixV4(protocol, identifier, exportNodeID(prefix.Node), desc)
		} else {
			nlri = NewBGPLSPrefixV6(protocol, identifier, exportNodeID(prefix.Node), desc)
		}
		attributes, err := originateAttributes(snapshot.Domain.Protocol, uint16(nlri.nlriType), prefix.Attributes, prefix.Opaque)
		if err != nil {
			return nil, err
		}
		if err := add(nlri.Bytes(), attributes); err != nil {
			return nil, err
		}
	}
	for _, sid := range snapshot.SIDs {
		if excluded(sid.Node) {
			continue
		}
		if !sid.SID.Is6() || sid.SID.Is4In6() {
			return nil, errors.New("native SRv6 SID is not IPv6")
		}
		if err := validateExportTopology(snapshot.Domain.Protocol, sid.Topology); err != nil {
			return nil, err
		}
		if !slices.ContainsFunc(sid.Attributes, func(attr linkstateevents.TLV) bool { return attr.Type == 1250 && len(attr.Value) == 4 }) {
			return nil, errors.New("native SRv6 SID lacks Endpoint Behavior")
		}
		nlri := newBGPLSSRv6SID(protocol, identifier, exportNodeID(sid.Node),
			SRv6SIDDescriptor{MultiTopologyID: sid.Topology, SRv6SID: sid.SID.AsSlice()})
		attributes, err := originateAttributes(snapshot.Domain.Protocol, 6, sid.Attributes, nil)
		if err != nil {
			return nil, err
		}
		if err := add(nlri.Bytes(), attributes); err != nil {
			return nil, err
		}
	}
	return routes, nil
}

func encodeNativeLink(protocol BGPLSProtocolID, identifier uint64, link linkstateevents.Link, topology uint16) ([]byte, error) {
	descriptors := make([]linkstateevents.TLV, 0, len(link.LocalAddresses)+len(link.RemoteAddresses)+2)
	for _, addr := range link.LocalAddresses {
		if !addr.IsValid() {
			return nil, errors.New("invalid native local link address")
		}
		if addr.IsLinkLocalUnicast() {
			continue
		}
		kind := uint16(261)
		if addr.Is4() {
			kind = 259
		}
		descriptors = append(descriptors, linkstateevents.TLV{Type: kind, Value: addr.AsSlice()})
	}
	for _, addr := range link.RemoteAddresses {
		if !addr.IsValid() {
			return nil, errors.New("invalid native remote link address")
		}
		if addr.IsLinkLocalUnicast() {
			continue
		}
		kind := uint16(262)
		if addr.Is4() {
			kind = 260
		}
		descriptors = append(descriptors, linkstateevents.TLV{Type: kind, Value: addr.AsSlice()})
	}
	if len(descriptors) == 0 && link.HasLinkIDs {
		ids := make([]byte, 8)
		binary.BigEndian.PutUint32(ids, link.LocalID)
		binary.BigEndian.PutUint32(ids[4:], link.RemoteID)
		descriptors = append(descriptors, linkstateevents.TLV{Type: 258, Value: ids})
	}
	if protocol != BGPLSProtocolID(linkstateevents.BGP) {
		mtid := []byte{byte(topology >> 8), byte(topology)}
		descriptors = append(descriptors, linkstateevents.TLV{Type: 263, Value: mtid})
	}
	slices.SortFunc(descriptors, func(a, b linkstateevents.TLV) int {
		if a.Type != b.Type {
			return int(a.Type) - int(b.Type)
		}
		if len(a.Value) != len(b.Value) {
			return len(a.Value) - len(b.Value)
		}
		return bytes.Compare(a.Value, b.Value)
	})
	base := NewBGPLSLink(protocol, identifier, exportNodeID(link.Local), exportNodeID(link.Remote), LinkDescriptor{})
	length := base.Len()
	for i, desc := range descriptors {
		if i > 0 && desc.Type == descriptors[i-1].Type && bytes.Equal(desc.Value, descriptors[i-1].Value) {
			continue
		}
		length += 4 + len(desc.Value)
	}
	if length-4 > 65535 {
		return nil, errors.New("native Link NLRI exceeds wire length")
	}
	wire := make([]byte, length)
	off := base.WriteTo(wire, 0)
	for i, desc := range descriptors {
		if i > 0 && desc.Type == descriptors[i-1].Type && bytes.Equal(desc.Value, descriptors[i-1].Value) {
			continue
		}
		off += writeTLVBytes(wire, off, desc.Type, desc.Value)
	}
	binary.BigEndian.PutUint16(wire[2:], uint16(length-4))
	return wire, nil
}

func exportNodeID(id linkstateevents.NodeID) NodeDescriptor {
	node := NodeDescriptor{ASN: id.ASN, BGPLSIdentifier: id.BGPLSID, HasBGPLSIdentifier: id.HasBGPLSID,
		OSPFAreaID: id.Area, HasOSPFAreaID: id.HasArea, IGPRouterID: id.RouterID, ConfedMember: id.Confederation}
	if id.BGPRouterID.Is4() {
		b := id.BGPRouterID.As4()
		node.BGPRouterID = binary.BigEndian.Uint32(b[:])
	}
	return node
}

func validateExportTopology(protocol linkstateevents.Protocol, topology uint16) error {
	limit := uint16(4095)
	if protocol == linkstateevents.OSPFv2 || protocol == linkstateevents.OSPFv3 {
		limit = 127
	}
	if topology > limit {
		return fmt.Errorf("topology %d exceeds protocol %d limit %d", topology, protocol, limit)
	}
	return nil
}

// originateAttributes is used only by the native producer. Propagation retains
// received BGP-LS values untouched, as RFC 9552 Section 8.2.2 requires.
func originateAttributes(protocol linkstateevents.Protocol, kind uint16, attributes []linkstateevents.TLV, opaque []linkstateevents.Opaque) ([]byte, error) {
	length := 0
	for _, attr := range attributes {
		if len(attr.Value) > 65535 {
			return nil, errors.New("BGP-LS attribute TLV too long")
		}
		if attr.Type >= 65000 {
			return nil, errors.New("native BGP-LS private-use origination is not enabled")
		}
		if attr.Type == 1025 || attr.Type == 1097 || attr.Type == 1157 {
			return nil, errors.New("opaque native attribute requires source provenance")
		}
		if attr.Type == 1094 && protocol != linkstateevents.Direct && protocol != linkstateevents.Static {
			return nil, errors.New("MPLS protocol mask requires local Direct or Static source")
		}
		length += 4 + len(attr.Value)
	}
	var opaqueType uint16
	switch kind {
	case 1:
		opaqueType = 1025
	case 2:
		opaqueType = 1097
	case 3, 4:
		opaqueType = 1157
	}
	for _, attr := range opaque {
		if !validOpaqueSource(protocol, kind, attr.Source) {
			return nil, fmt.Errorf("opaque provenance %d cannot originate protocol %d NLRI %d", attr.Source, protocol, kind)
		}
		if len(attr.Value) > 65535 {
			return nil, errors.New("opaque BGP-LS attribute too long")
		}
		length += 4 + len(attr.Value)
	}
	if length > 65535 {
		return nil, errors.New("BGP-LS attribute exceeds wire length")
	}
	ordered := make([]linkstateevents.TLV, 0, len(attributes)+len(opaque))
	ordered = append(ordered, attributes...)
	for _, attr := range opaque {
		ordered = append(ordered, linkstateevents.TLV{Type: opaqueType, Value: attr.Value})
	}
	slices.SortFunc(ordered, func(a, b linkstateevents.TLV) int {
		if a.Type != b.Type {
			return int(a.Type) - int(b.Type)
		}
		if len(a.Value) != len(b.Value) {
			return len(a.Value) - len(b.Value)
		}
		return bytes.Compare(a.Value, b.Value)
	})
	wire := make([]byte, length)
	off := 0
	for _, attr := range ordered {
		n := writeTLVBytes(wire, off, attr.Type, attr.Value)
		value := wire[off+4 : off+n]
		if err := clearOriginatedReserved(attr.Type, value); err != nil {
			return nil, err
		}
		off += n
	}
	return wire, nil
}

func clearOriginatedReserved(kind uint16, value []byte) error {
	mask := byte(0xff)
	switch kind {
	case 1024:
		mask = 0xfc
	case 1152:
		mask = 0xf0
	case 1094:
		mask = 0xc0
	case 1038:
		if len(value) != 4 {
			return errors.New("invalid SRv6 Capabilities length")
		}
		clear(value[2:4])
	case 1162:
		if len(value) < 8 {
			return errors.New("invalid SRv6 Locator length")
		}
		clear(value[2:4])
	case 1101, 1102, 1103:
		if len(value) != 7 && len(value) != 8 {
			return errors.New("invalid Peer SID length")
		}
		value[0] &= 0xf0
		clear(value[2:4])
	case 1034, 1036:
		if len(value) < 2 {
			return errors.New("invalid SR capability header")
		}
		value[1] = 0
	case 1250:
		if len(value) != 4 {
			return errors.New("invalid SRv6 endpoint behavior length")
		}
	case 1251:
		if len(value) != 12 {
			return errors.New("invalid SRv6 peer node length")
		}
		value[0] &= 0xe0
		clear(value[2:4])
	case 1252:
		if len(value) != 4 || int(value[0])+int(value[1])+int(value[2])+int(value[3]) > 128 {
			return errors.New("invalid SRv6 SID structure")
		}
	}
	if mask != 0xff {
		if len(value) != 1 {
			return errors.New("invalid BGP-LS flag field length")
		}
		value[0] &= mask
	}
	return nil
}

func validOpaqueSource(protocol linkstateevents.Protocol, kind uint16, source linkstateevents.Provenance) bool {
	if protocol == linkstateevents.ISISLevel1 || protocol == linkstateevents.ISISLevel2 {
		return source == linkstateevents.ISIS
	}
	if protocol != linkstateevents.OSPFv2 && protocol != linkstateevents.OSPFv3 {
		return false
	}
	if kind == 1 {
		return source == linkstateevents.OSPFRouterInformation
	}
	if protocol == linkstateevents.OSPFv2 {
		if kind == 2 {
			return source == linkstateevents.OSPFv2ExtendedLink
		}
		return (kind == 3 || kind == 4) && source == linkstateevents.OSPFv2ExtendedPrefix
	}
	if kind == 2 {
		return source == linkstateevents.OSPFv3ExtendedRouter || source == linkstateevents.OSPFv3ExtendedLink
	}
	return (kind == 3 || kind == 4) && (source == linkstateevents.OSPFv3ExtendedInterAreaPrefix ||
		source == linkstateevents.OSPFv3ExtendedIntraAreaPrefix || source == linkstateevents.OSPFv3ExtendedASExternal || source == linkstateevents.OSPFv3ExtendedNSSA)
}
