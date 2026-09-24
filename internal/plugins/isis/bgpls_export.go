// Design: docs/architecture/wire/nlri-bgpls.md -- native IS-IS LSDB export.
// Related: lsdb_wiring.go -- publishes after every database mutation.
// RFC 9552 sections 5.2/5.3; RFC 9085 and RFC 9514 -- attribute translation.
package isis

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
	"github.com/ze-software/ze/pkg/ze"
)

var isisLinkState = linkstateevents.RegisterSource(Namespace)
var bgplsGeneration atomic.Uint64

// bgplsSource serializes capture and synchronous delivery, including replay and
// withdrawal. Safe for concurrent use; startBGPLS MUST be paired with stopBGPLS.
// The singleton configured IS-IS process has instance 1; its level is the domain
// area, not an OSPF area descriptor. Identifier zero denotes the default universe.
type bgplsSource struct {
	mu          sync.Mutex
	bus         ze.EventBus
	live        bool
	unsubscribe func()
	builder     bgplsBuilder
	// Last demonstrated unreachable origins survive an unready SPF cache. Only
	// source identities are retained; removed domains and absent origins clear.
	unreachable [2]map[types.SourceID]struct{}
}

// startBGPLS subscribes before publishing. The engine MUST call stopBGPLS before
// teardown so neither a late replay nor an LSDB callback can resurrect its state.
func (e *engine) startBGPLS() {
	e.bgpls.mu.Lock()
	defer e.bgpls.mu.Unlock()
	if e.bgpls.live || e.sink == nil || e.sink.bus == nil {
		return
	}
	e.bgpls.bus = e.sink.bus
	e.bgpls.live = true
	e.bgpls.unsubscribe = linkstateevents.Request.Subscribe(e.bgpls.bus, e.publishBGPLS)
	e.publishBGPLSLocked(false)
}

// stopBGPLS MUST be called for every started source. It synchronizes with any
// in-flight publication, disables replay, then replaces both domains with empty.
func (e *engine) stopBGPLS() {
	e.bgpls.mu.Lock()
	defer e.bgpls.mu.Unlock()
	if !e.bgpls.live {
		return
	}
	e.bgpls.live = false
	e.bgpls.unsubscribe()
	e.bgpls.unsubscribe = nil
	e.publishBGPLSLocked(true)
	e.bgpls.bus = nil
	e.bgpls.builder = bgplsBuilder{}
	clear(e.bgpls.unreachable[:])
}

func (e *engine) publishBGPLS() {
	e.bgpls.mu.Lock()
	defer e.bgpls.mu.Unlock()
	if !e.bgpls.live {
		return
	}
	e.publishBGPLSLocked(false)
}

// The source mutex covers RawSnapshot through Emit. LSDB mutation takes its own
// lock and never calls us while holding it. A later notification therefore reads
// current state rather than delivering a stale snapshot captured before the lock.
func (e *engine) publishBGPLSLocked(withdraw bool) {
	e.mu.Lock()
	present, configured, root := e.cfg.Present(), e.cfg.Level, e.cfg.SystemID
	e.mu.Unlock()
	for _, level := range [...]lsdb.Level{lsdb.Level1, lsdb.Level2} {
		var raw [][]byte
		enabled := !withdraw && present && configFormsLevel(configured, level)
		if enabled {
			raw = e.lsdb.RawSnapshot(level)
		}
		e.bgpls.builder.build(raw)
		snapshot := &e.bgpls.builder.snapshot
		previous := e.bgpls.unreachable[int(level)-1]
		if !enabled {
			clear(previous)
		} else if e.spf != nil {
			for _, state := range e.spf.Reachability() {
				if state.Root != root || uint8(state.Level) != uint8(level) {
					continue
				}
				for source, reached := range state.Nodes {
					if reached {
						delete(previous, source)
						continue
					}
					if _, live := e.bgpls.builder.nodes[source]; !live {
						continue
					}
					if previous == nil {
						previous = make(map[types.SourceID]struct{})
					}
					previous[source] = struct{}{}
				}
				break
			}
		}
		// Missing/mismatched results or origins absent from a completed graph
		// are not evidence of recovery. Retain prior withdrawals, but never
		// retain an origin absent from this LSDB.
		for source := range previous {
			if _, live := e.bgpls.builder.nodes[source]; !live {
				delete(previous, source)
			}
		}
		e.bgpls.unreachable[int(level)-1] = previous
		for _, node := range snapshot.Nodes {
			var source types.SourceID
			copy(source[:], node.ID.RouterID)
			if _, unreachable := previous[source]; unreachable {
				snapshot.Unreachable = append(snapshot.Unreachable, node.ID)
			}
		}
		snapshot.Domain = linkstateevents.Domain{Protocol: linkstateevents.Protocol(level), Instance: 1, Area: uint32(level)}
		snapshot.Generation = bgplsGeneration.Add(1)
		if _, err := isisLinkState.Emit(e.bgpls.bus, snapshot); err != nil {
			e.log.Warn("isis: BGP-LS snapshot delivery failed", "level", level, "error", err)
		}
	}
}

// bgplsBuilder owns reusable snapshot storage. Raw attribute values borrow the
// immutable LSDB bytes; transformed values use arena. Neither may escape Emit.
// Work and storage are bounded by MaxLSPsPerLevel and each stored PDU's length.
type bgplsBuilder struct {
	snapshot         linkstateevents.Snapshot
	arena            []byte
	nodes            map[types.SourceID]int
	prefixes         map[bgplsPrefixKey]int
	widePrefixes     map[bgplsPrefixKey]bool
	srlgs            []bgplsSRLG
	locatorTLVs      []bgplsSRLG
	locatorsByPrefix map[bgplsPrefixKey]bgplsLocator
	linkOrder        []int
	mergedLinks      []linkstateevents.Link
}

type bgplsPrefixKey struct {
	node     types.SourceID
	prefix   netip.Prefix
	topology uint16
}

type bgplsLocator struct {
	algorithm uint8
	conflict  bool
}

type bgplsSRLG struct {
	source types.SourceID
	typ    uint8
	value  []byte
}

func (b *bgplsBuilder) build(raw [][]byte) {
	clear(b.snapshot.Nodes)
	clear(b.snapshot.Links)
	clear(b.snapshot.Prefixes)
	clear(b.snapshot.SIDs)
	clear(b.snapshot.Unreachable)
	b.snapshot.Nodes = b.snapshot.Nodes[:0]
	b.snapshot.Links = b.snapshot.Links[:0]
	b.snapshot.Prefixes = b.snapshot.Prefixes[:0]
	b.snapshot.SIDs = b.snapshot.SIDs[:0]
	b.snapshot.Unreachable = b.snapshot.Unreachable[:0]
	b.arena = b.arena[:0]
	clear(b.nodes)
	clear(b.prefixes)
	clear(b.widePrefixes)
	clear(b.srlgs)
	b.srlgs = b.srlgs[:0]
	clear(b.locatorTLVs)
	b.locatorTLVs = b.locatorTLVs[:0]
	clear(b.locatorsByPrefix)
	if b.nodes == nil {
		b.nodes = make(map[types.SourceID]int)
		b.prefixes = make(map[bgplsPrefixKey]int)
		b.widePrefixes = make(map[bgplsPrefixKey]bool)
		b.locatorsByPrefix = make(map[bgplsPrefixKey]bgplsLocator)
	}
	for _, pdu := range raw {
		decoded, err := packet.DecodePDU(pdu)
		if err != nil {
			continue // A malformed stored PDU contributes no invented objects.
		}
		if decoded.LSP != nil {
			b.lsp(decoded.LSP)
		}
		decoded.Release()
	}
	for _, tlv := range b.locatorTLVs {
		b.locators(nil, tlv.source, tlv.value, true)
	}
	for _, tlv := range b.locatorTLVs {
		node := &b.snapshot.Nodes[b.nodes[tlv.source]]
		if !b.locators(node, tlv.source, tlv.value, false) {
			node.Opaque = append(node.Opaque, b.opaque(27, tlv.value))
		}
	}
	b.validateSRv6()
	b.pseudonodeTopologies()
	b.linkSRLGs()
	b.normalize()
}

func (b *bgplsBuilder) alloc(size int) []byte {
	offset := len(b.arena)
	b.arena = slices.Grow(b.arena, size)
	b.arena = b.arena[:offset+size]
	value := b.arena[offset:]
	clear(value)
	return value
}

func (b *bgplsBuilder) nodeID(source types.SourceID) linkstateevents.NodeID {
	length := types.SystemIDLen
	if source.IsPseudonode() {
		length = types.SourceIDLen
	}
	value := b.alloc(length)
	copy(value, source[:length])
	return linkstateevents.NodeID{RouterID: value}
}

func (b *bgplsBuilder) opaque(typ uint8, value []byte) linkstateevents.Opaque {
	encoded := b.alloc(2 + len(value))
	encoded[0], encoded[1] = typ, byte(len(value))
	copy(encoded[2:], value)
	return linkstateevents.Opaque{Source: linkstateevents.ISIS, Value: encoded}
}

func (b *bgplsBuilder) lsp(lsp *packet.LSP) {
	source := lsp.LSPID.SourceID()
	index, found := b.nodes[source]
	if !found {
		index = len(b.snapshot.Nodes)
		b.nodes[source] = index
		b.snapshot.Nodes = append(b.snapshot.Nodes, linkstateevents.Node{ID: b.nodeID(source)})
	}
	node := &b.snapshot.Nodes[index]
	if lsp.LSPID.LSPNumber() == 0 {
		flags := b.alloc(1)
		if lsp.IsOverloaded() {
			flags[0] |= 0x80
		}
		if lsp.TypeBlock&packet.LSPAttachedMask != 0 {
			flags[0] |= 0x40
		}
		node.Attributes = bgplsAttribute(node.Attributes, 1024, flags)
	}
	for _, tlv := range lsp.TLVs {
		value := tlv.Value
		topology := uint16(0)
		switch tlv.Type {
		case 222, 235, 237, 150:
			if len(value) < 2 {
				node.Opaque = append(node.Opaque, b.opaque(tlv.Type, value))
				continue
			}
			topology = binary.BigEndian.Uint16(value) & 0x0fff
			// RFC 5120 sections 7.2-7.4: "The TLV MUST be ignored if the
			// ID is zero." The standard topology uses its original TLVs.
			if topology == 0 {
				continue
			}
			value = value[2:]
		}
		mapped := true
		switch tlv.Type {
		case packet.TLVAreaAddresses:
			areas, err := packet.DecodeAreaAddressesTLV(value)
			if err != nil {
				mapped = false
				break
			}
			for _, area := range areas.Areas {
				v := b.alloc(area.Len())
				area.WriteTo(v, 0)
				node.Attributes = bgplsAttribute(node.Attributes, 1027, v)
			}
		case packet.TLVDynamicHostname:
			if len(value) == 0 {
				mapped = false
				break
			}
			node.Attributes = bgplsAttribute(node.Attributes, 1026, value)
		case 134, 140:
			length, typ := 4, uint16(1028)
			if tlv.Type == 140 {
				length, typ = 16, 1029
			}
			if len(value) != length {
				mapped = false
				break
			}
			node.Attributes = bgplsAttribute(node.Attributes, typ, value)
		case 229:
			if lsp.LSPID.LSPNumber() != 0 {
				break
			}
			if len(value)%2 != 0 {
				mapped = false
				break
			}
			// RFC 9552 section 5.2.2.1 preserves the node's O/A bits.
			for off := 0; off < len(value); off += 2 {
				v := b.alloc(2)
				binary.BigEndian.PutUint16(v, binary.BigEndian.Uint16(value[off:])&0xcfff)
				node.Attributes = bgplsAttribute(node.Attributes, 263, v)
			}
		case packet.TLVISReachabilityNarrow:
			mapped = b.narrowLinks(node, value)
		case packet.TLVExtendedISReach, 222:
			mapped = b.extendedLinks(node, value, topology)
		case packet.TLVIPInternalReachability, packet.TLVIPExternalReachability:
			mapped = b.narrowPrefixes(node, source, tlv.Type, value)
		case packet.TLVExtendedIPReach, 235:
			mapped = b.ipv4Prefixes(node, source, value, topology)
		case packet.TLVIPv6Reachability, 237:
			mapped = b.ipv6Prefixes(node, source, value, topology)
		case 242:
			mapped = b.capabilities(node, value)
		case 27:
			b.locatorTLVs = append(b.locatorTLVs, bgplsSRLG{source: source, value: value})
		case 149, 150:
			mapped = b.binding(node, source, value, topology)
		case 138, 139:
			b.srlgs = append(b.srlgs, bgplsSRLG{source: source, typ: tlv.Type, value: value})
		case packet.TLVAuthentication, packet.TLVPadding:
			// Authentication material and padding are not topology attributes.
		default:
			mapped = false
		}
		if !mapped {
			node.Opaque = append(node.Opaque, b.opaque(tlv.Type, tlv.Value))
		}
	}
}

// bgplsAttribute coalesces repeated advertisements without dropping independently
// advertised SIDs or area addresses. Node MT values form one array (RFC 9552).
func bgplsAttribute(attrs []linkstateevents.TLV, typ uint16, value []byte) []linkstateevents.TLV {
	for _, attr := range attrs {
		if attr.Type == typ && bytes.Equal(attr.Value, value) {
			return attrs
		}
	}
	return append(attrs, linkstateevents.TLV{Type: typ, Value: value})
}

func (b *bgplsBuilder) narrowLinks(node *linkstateevents.Node, value []byte) bool {
	decoded, err := packet.DecodeNarrowISReachTLV(value)
	if err != nil {
		return false
	}
	for i, entry := range decoded.Entries {
		metric := b.alloc(1)
		metric[0] = entry.DefaultMetricValue
		link := linkstateevents.Link{Local: node.ID, Remote: b.nodeID(entry.Neighbor), Attributes: []linkstateevents.TLV{{Type: 1095, Value: metric}}}
		// The default metric is mapped; retain the optional legacy metrics and
		// virtual/I-E bits with their native TLV context, not an invented TLV.
		if entry.DefaultMetricExternal || decoded.VirtualFlag != 0 || entry.DelayMetric&0x80 == 0 || entry.ExpenseMetric&0x80 == 0 || entry.ErrorMetric&0x80 == 0 {
			v := b.alloc(12)
			v[0] = decoded.VirtualFlag
			copy(v[1:], value[1+i*11:1+(i+1)*11])
			link.Opaque = append(link.Opaque, b.opaque(packet.TLVISReachabilityNarrow, v))
		}
		b.snapshot.Links = append(b.snapshot.Links, link)
	}
	return true
}

func (b *bgplsBuilder) extendedLinks(node *linkstateevents.Node, value []byte, topology uint16) bool {
	decoded, err := packet.DecodeExtendedISReachTLV(value)
	if err != nil {
		return false
	}
	for _, entry := range decoded.Entries {
		metric := b.alloc(3)
		entry.Metric.WriteTo(metric, 0)
		link := linkstateevents.Link{Local: node.ID, Remote: b.nodeID(entry.Neighbor), Topologies: []uint16{topology}, Attributes: []linkstateevents.TLV{{Type: 1095, Value: metric}}}
		for _, sub := range entry.SubTLVs {
			if !b.linkAttribute(&link, sub.Type, sub.Value) {
				link.Opaque = append(link.Opaque, b.opaque(sub.Type, sub.Value))
			}
		}
		b.snapshot.Links = append(b.snapshot.Links, link)
	}
	return true
}

// RFC 9552 section 5.2.2 descriptors and section 5.3.2 attributes share native
// IS-IS value formats except the 24-bit TE metric, expanded to four octets.
func (b *bgplsBuilder) linkAttribute(link *linkstateevents.Link, typ uint8, value []byte) bool {
	var attribute uint16
	length := 0
	switch typ {
	case 4:
		if len(value) != 8 {
			return false
		}
		link.LocalID, link.RemoteID = binary.BigEndian.Uint32(value), binary.BigEndian.Uint32(value[4:])
		link.HasLinkIDs = true
		return true
	case 6, 8, 12, 13:
		length = 4
		if typ == 12 || typ == 13 {
			length = 16
		}
		if len(value) != length {
			return false
		}
		address, ok := netip.AddrFromSlice(value)
		if !ok {
			return false
		}
		// RFC 9552 section 5.2.2: "IPv4/IPv6 link-local addresses MUST NOT
		// be carried in the IPv4/IPv6 interface/neighbor address TLVs".
		if address.IsLinkLocalUnicast() {
			return true
		}
		if typ == 6 || typ == 12 {
			if !slices.Contains(link.LocalAddresses, address) {
				link.LocalAddresses = append(link.LocalAddresses, address)
			}
		} else if !slices.Contains(link.RemoteAddresses, address) {
			link.RemoteAddresses = append(link.RemoteAddresses, address)
		}
		return true
	case 3:
		attribute, length = 1088, 4
	case 9:
		attribute, length = 1089, 4
	case 10:
		attribute, length = 1090, 4
	case 11:
		attribute, length = 1091, 32
	case 18:
		if len(value) != 3 {
			return false
		}
		v := b.alloc(4)
		copy(v[1:], value)
		link.Attributes = bgplsAttribute(link.Attributes, 1092, v)
		return true
	case 20:
		attribute, length = 1093, 2
	case 14:
		attribute = 1173 // RFC 9104 extended administrative groups.
	case 33:
		attribute, length = 1114, 4
	case 34:
		attribute, length = 1115, 8
	case 35:
		attribute, length = 1116, 4
	case 36:
		attribute, length = 1117, 4
	case 37:
		attribute, length = 1118, 4
	case 38:
		attribute, length = 1119, 4
	case 39:
		attribute, length = 1120, 4
	case 15:
		attribute = 267 // RFC 8814 Link MSD.
	case 31, 32:
		minimum := 5
		attribute = 1099
		if typ == 32 {
			minimum, attribute = 11, 1100
		}
		if len(value) != minimum && len(value) != minimum+1 {
			return false
		}
		link.Attributes = bgplsAttribute(link.Attributes, attribute, b.sidValue(value))
		return true
	case 43, 44:
		return b.adjacencySIDv6(link, typ, value)
	default:
		return false
	}
	if length != 0 && len(value) != length {
		return false
	}
	if typ == 14 && (len(value) == 0 || len(value)%4 != 0) {
		return false
	}
	if typ == 15 && (len(value) == 0 || len(value)%2 != 0) {
		return false
	}
	if typ == 20 || (typ >= 33 && typ <= 36) {
		translated := b.alloc(len(value))
		copy(translated, value)
		switch typ {
		case 20:
			translated[0] &= 0x3f
			translated[1] = 0
		case 33, 36:
			translated[0] &= 0x80
		case 34:
			translated[0] &= 0x80
			translated[4] = 0
		case 35:
			translated[0] = 0
		}
		value = translated
	}
	link.Attributes = bgplsAttribute(link.Attributes, attribute, value)
	return true
}

func (b *bgplsBuilder) prefix(node *linkstateevents.Node, source types.SourceID, prefix netip.Prefix, topology uint16) *linkstateevents.Prefix {
	key := bgplsPrefixKey{node: source, prefix: prefix.Masked(), topology: topology}
	if index, exists := b.prefixes[key]; exists {
		return &b.snapshot.Prefixes[index]
	}
	b.prefixes[key] = len(b.snapshot.Prefixes)
	b.snapshot.Prefixes = append(b.snapshot.Prefixes, linkstateevents.Prefix{Node: node.ID, Prefix: key.prefix, Topology: topology})
	return &b.snapshot.Prefixes[len(b.snapshot.Prefixes)-1]
}

func (b *bgplsBuilder) prefixMetric(prefix *linkstateevents.Prefix, metric uint32, down bool) {
	value := b.alloc(4)
	binary.BigEndian.PutUint32(value, metric)
	prefix.Attributes = bgplsSetAttribute(prefix.Attributes, 1155, value)
	flags := b.alloc(1)
	if down {
		flags[0] = 0x80
	}
	prefix.Attributes = bgplsSetAttribute(prefix.Attributes, 1152, flags)
}

func (b *bgplsBuilder) narrowPrefixes(node *linkstateevents.Node, source types.SourceID, typ uint8, value []byte) bool {
	decoded, err := packet.DecodeNarrowIPReachTLV(value, typ == packet.TLVIPExternalReachability)
	if err != nil {
		return false
	}
	for i, entry := range decoded.Entries {
		prefix := b.prefix(node, source, entry.Prefix, 0)
		if !b.widePrefixes[bgplsPrefixKey{node: source, prefix: entry.Prefix.Masked()}] {
			b.prefixMetric(prefix, uint32(entry.DefaultMetricValue), entry.UpDown)
			if decoded.External {
				flags := b.alloc(1)
				flags[0] = 0x80
				prefix.Attributes = bgplsSetAttribute(prefix.Attributes, 1170, flags)
			}
		}
		if entry.ExternalMetric || entry.DelayMetric&0x80 == 0 || entry.ExpenseMetric&0x80 == 0 || entry.ErrorMetric&0x80 == 0 {
			prefix.Opaque = append(prefix.Opaque, b.opaque(typ, value[i*12:(i+1)*12]))
		}
	}
	return true
}

func (b *bgplsBuilder) ipv4Prefixes(node *linkstateevents.Node, source types.SourceID, value []byte, topology uint16) bool {
	decoded, err := packet.DecodeExtendedIPReachTLV(value)
	if err != nil {
		return false
	}
	for _, entry := range decoded.Entries {
		prefix := b.prefix(node, source, entry.Prefix, topology)
		b.widePrefixes[bgplsPrefixKey{node: source, prefix: entry.Prefix.Masked(), topology: topology}] = true
		b.prefixMetric(prefix, entry.Metric.Value(), entry.UpDown)
		prefix.Attributes = slices.DeleteFunc(prefix.Attributes, func(attr linkstateevents.TLV) bool { return attr.Type == 1170 })
		b.prefixAttributes(prefix, entry.SubTLVs, false, false)
	}
	return true
}

func (b *bgplsBuilder) ipv6Prefixes(node *linkstateevents.Node, source types.SourceID, value []byte, topology uint16) bool {
	decoded, err := packet.DecodeIPv6ReachabilityTLV(value)
	if err != nil {
		return false
	}
	for _, entry := range decoded.Entries {
		prefix := b.prefix(node, source, entry.Prefix, topology)
		b.prefixMetric(prefix, entry.Metric.Value(), entry.UpDown)
		b.prefixAttributes(prefix, entry.SubTLVs, true, entry.External)
	}
	return true
}

func (b *bgplsBuilder) prefixAttributes(prefix *linkstateevents.Prefix, subs []packet.SubTLV, ipv6, external bool) {
	var flags []byte
	for _, sub := range subs {
		attribute := uint16(0)
		value := sub.Value
		switch sub.Type {
		case 1:
			if len(value)%4 == 0 {
				attribute = 1153
			}
		case 2:
			if len(value)%8 == 0 {
				attribute = 1154
			}
		case 3:
			if len(value) == 5 || len(value) == 6 {
				attribute, value = 1158, b.sidValue(value)
			}
		case 4:
			if len(value) > 0 {
				flags = b.alloc(len(value))
				copy(flags, value)
				continue
			}
		case 11:
			if len(value) == 4 {
				attribute = 1171
			}
		case 12:
			if len(value) == 16 {
				attribute = 1171
			}
		}
		if attribute == 0 {
			prefix.Opaque = append(prefix.Opaque, b.opaque(sub.Type, sub.Value))
			continue
		}
		prefix.Attributes = bgplsAttribute(prefix.Attributes, attribute, value)
	}
	if external && len(flags) == 0 {
		flags = b.alloc(1)
	}
	if len(flags) > 0 {
		// RFC 9085 section 2.3.2 takes the IPv6 X bit from its fixed header,
		// not the ignored X bit of the native Prefix Attribute Flags sub-TLV.
		if ipv6 {
			flags[0] &^= 0x80
			if external {
				flags[0] |= 0x80
			}
		}
		prefix.Attributes = bgplsSetAttribute(prefix.Attributes, 1170, flags)
	}
}

// RFC 9085 sections 2.2.1/2.2.2/2.3.1 insert two reserved zero octets:
// native [flags:1][weight-or-algorithm:1][SID...] -> [same:2][zero:2][SID...].
func (b *bgplsBuilder) sidValue(native []byte) []byte {
	value := b.alloc(len(native) + 2)
	copy(value[:2], native[:2])
	value[0] &= 0xfc
	copy(value[4:], native[2:])
	if len(native) == 5 || len(native) == 11 {
		value[len(value)-3] &= 0x0f
	}
	return value
}

// RFC 5120 section 3: "there is no change to the pseudo-node LSP
// construction." Its ordinary links are shared by each neighbor's advertised
// MT adjacency to that pseudonode, rather than being confined to topology zero.
func (b *bgplsBuilder) pseudonodeTopologies() {
	type adjacencyKey struct{ node, pseudonode types.SourceID }
	members := make(map[adjacencyKey][]uint16)
	for i := range b.snapshot.Links {
		link := &b.snapshot.Links[i]
		if len(link.Local.RouterID) != 6 || len(link.Remote.RouterID) != 7 {
			continue
		}
		var key adjacencyKey
		copy(key.node[:], link.Local.RouterID)
		copy(key.pseudonode[:], link.Remote.RouterID)
		ids := link.Topologies
		if len(ids) == 0 {
			ids = []uint16{0}
		}
		for _, id := range ids {
			if !slices.Contains(members[key], id) {
				members[key] = append(members[key], id)
			}
		}
	}
	for i := range b.snapshot.Links {
		link := &b.snapshot.Links[i]
		if len(link.Local.RouterID) != 7 || len(link.Remote.RouterID) != 6 {
			continue
		}
		var key adjacencyKey
		copy(key.node[:], link.Remote.RouterID)
		copy(key.pseudonode[:], link.Local.RouterID)
		if ids := members[key]; len(ids) > 0 {
			link.Topologies = ids
		}
	}
}

// capabilities translates Router Capability sub-TLVs. Unknown children retain
// their enclosing TLV 242, including router ID and flooding scope.
func (b *bgplsBuilder) capabilities(node *linkstateevents.Node, value []byte) bool {
	if len(value) < 5 {
		return false
	}
	known := true
	it := packet.NewTLVIterator(value[5:])
	for {
		typ, native, ok := it.Next()
		if !ok {
			break
		}
		attribute := uint16(0)
		translated := native
		switch typ {
		case 2, 22:
			translated = b.srBlock(native)
			if translated != nil {
				attribute = 1034
				translated[0] &= 0xc0
				if typ == 22 {
					attribute = 1036
					translated[0] = 0
				}
			}
		case 19:
			if len(native) > 0 {
				attribute = 1035
			}
		case 23:
			if len(native) > 0 && len(native)%2 == 0 {
				attribute = 266
			}
		case 24:
			if len(native) == 1 {
				attribute = 1037
			}
		case 25:
			if len(native) >= 2 {
				attribute = 1038
				translated = b.alloc(4)
				copy(translated, native[:2])
				// RFC 9514 section 3.1: "Reserved: 2-octet field that
				// MUST be set to 0 when originated and ignored on receipt."
				if len(native) > 2 {
					known = false
				}
			}
		}
		if attribute == 0 {
			known = false
			continue
		}
		// The first advertisement in the lowest-numbered fragment wins for
		// singleton capabilities (RFC 8667 sections 3.1, 3.3, and 3.4).
		if !slices.ContainsFunc(node.Attributes, func(t linkstateevents.TLV) bool { return t.Type == attribute }) {
			node.Attributes = append(node.Attributes, linkstateevents.TLV{Type: attribute, Value: translated})
		}
	}
	return known && it.Err() == nil
}

// RFC 9085 sections 2.1.2/2.1.4:
// native [flags:1] ([range:3][type=1:1][len=3:1][label:3])*
// BGP-LS [flags:1][zero:1] ([range:3][type=1161:2][len=3:2][label:3])*.
func (b *bgplsBuilder) srBlock(native []byte) []byte {
	if len(native) < 9 || (len(native)-1)%8 != 0 {
		return nil
	}
	for off := 1; off < len(native); off += 8 {
		if native[off+3] != 1 || native[off+4] != 3 {
			return nil
		}
		if native[off]|native[off+1]|native[off+2] == 0 {
			return nil
		}
	}
	value := b.alloc(2 + ((len(native)-1)/8)*10)
	value[0] = native[0]
	for source, target := 1, 2; source < len(native); source, target = source+8, target+10 {
		copy(value[target:], native[source:source+3])
		binary.BigEndian.PutUint16(value[target+3:], 1161)
		binary.BigEndian.PutUint16(value[target+5:], 3)
		copy(value[target+7:], native[source+5:source+8])
	}
	return value
}

// RFC 8667 sections 2.4/2.5 bind [flags:1][reserved:1][range:2]
// [prefix-length:1][prefix:ceil(bits/8)][sub-TLVs...] to one Prefix NLRI.
// The Range TLV contains translated Prefix-SIDs, not a routing metric.
func (b *bgplsBuilder) binding(node *linkstateevents.Node, source types.SourceID, value []byte, topology uint16) bool {
	if len(value) < 5 {
		return false
	}
	bits := int(value[4])
	length := (bits + 7) / 8
	ipv6 := value[0]&0x80 != 0
	limit := 32
	if ipv6 {
		limit = 128
	}
	if bits > limit || len(value) < 5+length {
		return false
	}
	var address netip.Addr
	if ipv6 {
		var raw [16]byte
		copy(raw[:], value[5:5+length])
		address = netip.AddrFrom16(raw)
	} else {
		var raw [4]byte
		copy(raw[:], value[5:5+length])
		address = netip.AddrFrom4(raw)
	}
	it := packet.NewTLVIterator(value[5+length:])
	rangeValue := b.alloc(4 + 2*len(value[5+length:]))
	rangeValue[0] = value[0] & 0xf8
	copy(rangeValue[2:], value[2:4])
	offset := 4
	known := true
	for {
		typ, native, ok := it.Next()
		if !ok {
			break
		}
		if typ != 3 || (len(native) != 5 && len(native) != 6) {
			known = false
			continue
		}
		sid := b.sidValue(native)
		binary.BigEndian.PutUint16(rangeValue[offset:], 1158)
		binary.BigEndian.PutUint16(rangeValue[offset+2:], uint16(len(sid)))
		copy(rangeValue[offset+4:], sid)
		offset += 4 + len(sid)
	}
	if it.Err() != nil {
		return false
	}
	if offset > 4 {
		prefix := b.prefix(node, source, netip.PrefixFrom(address, bits), topology)
		prefix.Attributes = bgplsAttribute(prefix.Attributes, 1159, rangeValue[:offset])
	}
	return known
}

// RFC 9352 section 7.1: [MT:2], then entries [metric:4][flags:1]
// [algorithm:1][prefix-length:1][prefix:ceil(bits/8)][sub-length:1][subs...].
func (b *bgplsBuilder) locators(node *linkstateevents.Node, source types.SourceID, value []byte, collect bool) bool {
	if len(value) < 2 {
		return false
	}
	// Validate the entire containing TLV before publishing any entry.
	for off := 2; off < len(value); {
		if len(value)-off < 8 {
			return false
		}
		bits := int(value[off+6])
		// RFC 9352 section 7.1: "The entire TLV MUST be ignored if the
		// Loc-Size is outside this range." The permitted range is 1..128.
		if bits < 1 || bits > 128 {
			return false
		}
		subOffset := off + 7 + (bits+7)/8
		if subOffset >= len(value) {
			return false
		}
		off = subOffset + 1 + int(value[subOffset])
		if off > len(value) {
			return false
		}
		it := packet.NewTLVIterator(value[subOffset+1 : off])
		for {
			_, _, ok := it.Next()
			if !ok {
				break
			}
		}
		if it.Err() != nil {
			return false
		}
	}
	topology := binary.BigEndian.Uint16(value) & 0x0fff
	known := true
	for off := 2; off < len(value); {
		bits := int(value[off+6])
		length := (bits + 7) / 8
		var address [16]byte
		copy(address[:], value[off+7:off+7+length])
		locator := netip.PrefixFrom(netip.AddrFrom16(address), bits).Masked()
		subOffset := off + 7 + length
		end := subOffset + 1 + int(value[subOffset])
		key := bgplsPrefixKey{node: source, prefix: locator, topology: topology}
		state, exists := b.locatorsByPrefix[key]
		if collect {
			if exists && state.algorithm != value[off+5] {
				state.conflict = true
				b.locatorsByPrefix[key] = state
			} else if !exists {
				b.locatorsByPrefix[key] = bgplsLocator{algorithm: value[off+5]}
			}
			off = end
			continue
		}
		// RFC 9352 section 7.2: "If this restriction is not met, all
		// TLVs for that MTID/Locator MUST be ignored." Algorithms must agree.
		if state.conflict {
			off = end
			continue
		}
		prefix := b.prefix(node, source, locator, topology)
		attr := b.alloc(8)
		attr[0], attr[1] = value[off+4]&0x80, value[off+5]
		copy(attr[4:], value[off:off+4])
		prefix.Attributes = bgplsAttribute(prefix.Attributes, 1162, attr)
		it := packet.NewTLVIterator(value[subOffset+1 : end])
		for {
			typ, native, ok := it.Next()
			if !ok {
				break
			}
			if typ == 5 {
				if !b.endSID(node.ID, locator, topology, attr[1], native) {
					prefix.Opaque = append(prefix.Opaque, b.opaque(typ, native))
				}
				continue
			}
			if typ == 4 && slices.ContainsFunc(prefix.Attributes, func(attr linkstateevents.TLV) bool { return attr.Type == 1155 }) {
				// RFC 9352 section 6: "the ones advertised in the Prefix
				// Reachability TLV MUST be preferred."
				continue
			}
			b.prefixAttributes(prefix, []packet.SubTLV{{Type: typ, Value: native}}, false, false)
		}
		if it.Err() != nil {
			known = false
		}
		off = end
	}
	return known
}

// RFC 9352 section 7.2: [flags:1][behavior:2][SID:16][sub-length:1][subs].
// RFC 9514 section 7.1 maps this to [behavior:2][flags:1][algorithm:1].
func (b *bgplsBuilder) endSID(node linkstateevents.NodeID, locator netip.Prefix, topology uint16, algorithm uint8, native []byte) bool {
	if len(native) < 20 || int(native[19])+20 != len(native) {
		return false
	}
	sid := netip.AddrFrom16([16]byte(native[3:19]))
	// RFC 9352 section 7.2: "SRv6 End SIDs that are not allocated from
	// the associated locator MUST be ignored."
	if !locator.Contains(sid) {
		return true
	}
	structure, known, valid := bgplsSIDStructure(native[20:])
	if !valid {
		return true
	}
	behavior := b.alloc(4)
	copy(behavior, native[1:3])
	behavior[3] = algorithm
	attrs := []linkstateevents.TLV{{Type: 1250, Value: behavior}}
	if structure != nil {
		attrs = append(attrs, linkstateevents.TLV{Type: 1252, Value: structure})
	}
	b.snapshot.SIDs = append(b.snapshot.SIDs, linkstateevents.SID{Node: node, SID: sid, Topology: topology, Attributes: attrs})
	return known
}

// RFC 9352 sections 8.1/8.2:
// native [neighbor:0/6][flags:1][algorithm:1][weight:1][behavior:2]
//
//	[SID:16][sub-length:1][subs...]
//
// BGP-LS [behavior:2][flags:1][algorithm:1][weight:1][zero:1]
//
//	[neighbor:0/6][SID:16][16-bit framed sub-TLVs...].
func (b *bgplsBuilder) adjacencySIDv6(link *linkstateevents.Link, typ uint8, native []byte) bool {
	neighborLength, attribute := 0, uint16(1106)
	if typ == 44 {
		neighborLength, attribute = 6, 1107
	}
	if len(native) < neighborLength+22 {
		return false
	}
	header := native[neighborLength:]
	if int(header[21])+22 != len(header) {
		return false
	}
	structure, known, valid := bgplsSIDStructure(header[22:])
	if !valid {
		return true
	}
	length := neighborLength + 22
	if structure != nil {
		length += 8
	}
	value := b.alloc(length)
	copy(value[:2], header[3:5])
	value[2], value[3], value[4] = header[0]&0xe0, header[1], header[2]
	copy(value[6:], native[:neighborLength])
	copy(value[6+neighborLength:], header[5:21])
	if structure != nil {
		offset := 22 + neighborLength
		binary.BigEndian.PutUint16(value[offset:], 1252)
		binary.BigEndian.PutUint16(value[offset+2:], 4)
		copy(value[offset+4:], structure)
	}
	link.Attributes = bgplsAttribute(link.Attributes, attribute, value)
	return known
}

// Unknown sub-sub-TLVs are reported to the caller so it retains the native
// parent envelope. SID attributes have no generic opaque envelope of their own.
func bgplsSIDStructure(subs []byte) (structure []byte, known, valid bool) {
	known = true
	it := packet.NewTLVIterator(subs)
	for {
		typ, value, ok := it.Next()
		if !ok {
			break
		}
		if typ != 1 {
			known = false
			continue
		}
		// RFC 9352 section 9: "If it appears more than once in its parent
		// sub-TLV, the parent sub-TLV MUST be ignored by the receiver."
		if structure != nil || len(value) != 4 {
			return nil, known, false
		}
		// RFC 9514 section 8: "The sum of the LB Length, LN Length,
		// Fun. Length, and Arg. Length MUST be less than or equal to 128."
		if int(value[0])+int(value[1])+int(value[2])+int(value[3]) > 128 {
			return nil, known, false
		}
		structure = value
	}
	return structure, known, it.Err() == nil
}

func bgplsSetAttribute(attrs []linkstateevents.TLV, typ uint16, value []byte) []linkstateevents.TLV {
	for i := range attrs {
		if attrs[i].Type == typ {
			attrs[i].Value = value
			return attrs
		}
	}
	return append(attrs, linkstateevents.TLV{Type: typ, Value: value})
}

// linkSRLGs joins TLVs 138/139 to actual directed adjacencies by their advertised
// endpoint identifiers. An unmatched SRLG remains native node data; it never
// creates an adjacency unsupported by the LSDB.
func (b *bgplsBuilder) linkSRLGs() {
	if len(b.srlgs) == 0 {
		return
	}
	type endpoints struct{ local, remote types.SourceID }
	links := make(map[endpoints][]int)
	for i := range b.snapshot.Links {
		link := &b.snapshot.Links[i]
		var key endpoints
		copy(key.local[:], link.Local.RouterID)
		copy(key.remote[:], link.Remote.RouterID)
		links[key] = append(links[key], i)
	}
	for _, srlg := range b.srlgs {
		value := srlg.value
		minimum := 16
		if srlg.typ == 139 {
			minimum = 24
			if len(value) >= 8 && value[7]&1 != 0 {
				minimum = 40
			}
		}
		matched := false
		if len(value) >= minimum && (len(value)-minimum)%4 == 0 {
			var remote types.SourceID
			copy(remote[:], value[:7])
			for _, i := range links[endpoints{local: srlg.source, remote: remote}] {
				link := &b.snapshot.Links[i]
				if !bgplsSRLGMatches(link, srlg.typ, value) {
					continue
				}
				link.Attributes = bgplsAttribute(link.Attributes, 1096, value[minimum:])
				matched = true
			}
		}
		if !matched {
			node := &b.snapshot.Nodes[b.nodes[srlg.source]]
			node.Opaque = append(node.Opaque, b.opaque(srlg.typ, value))
		}
	}
}

func bgplsSRLGMatches(link *linkstateevents.Link, typ uint8, value []byte) bool {
	if typ == 138 {
		if value[7]&1 == 0 {
			return link.HasLinkIDs && link.LocalID == binary.BigEndian.Uint32(value[8:12]) && link.RemoteID == binary.BigEndian.Uint32(value[12:16])
		}
		return slices.Contains(link.LocalAddresses, netip.AddrFrom4([4]byte(value[8:12]))) &&
			slices.Contains(link.RemoteAddresses, netip.AddrFrom4([4]byte(value[12:16])))
	}
	// RFC 6119 section 4.4: unrecognized flags change identifier semantics.
	if value[7]&0xfe != 0 {
		return false
	}
	if !slices.Contains(link.LocalAddresses, netip.AddrFrom16([16]byte(value[8:24]))) {
		return false
	}
	if value[7]&1 == 0 {
		return true
	}
	return slices.Contains(link.RemoteAddresses, netip.AddrFrom16([16]byte(value[24:40])))
}

// normalize joins split attributes and repeated adjacency advertisements before
// they reach the exporter. In particular, splitting LAN Adj-SIDs across LSPs
// must not make the final fragment replace SIDs from earlier fragments.
func (b *bgplsBuilder) normalize() {
	for i := range b.snapshot.Nodes {
		b.snapshot.Nodes[i].Attributes = b.attributeArrays(b.snapshot.Nodes[i].Attributes, 263)
	}
	for i := range b.snapshot.Prefixes {
		b.snapshot.Prefixes[i].Attributes = b.attributeArrays(b.snapshot.Prefixes[i].Attributes, 1153, 1154)
	}
	for i := range b.snapshot.Links {
		link := &b.snapshot.Links[i]
		b.linkRouterIDs(link)
		slices.SortFunc(link.LocalAddresses, func(a, c netip.Addr) int { return a.Compare(c) })
		slices.SortFunc(link.RemoteAddresses, func(a, c netip.Addr) int { return a.Compare(c) })
		slices.Sort(link.Topologies)
		// Preserve the available link IDs as attributes when addresses define
		// the NLRI (RFC 9552 section 5.2.2), never as extra descriptors.
		if link.HasLinkIDs && len(link.LocalAddresses)+len(link.RemoteAddresses) > 0 {
			value := b.alloc(8)
			binary.BigEndian.PutUint32(value, link.LocalID)
			binary.BigEndian.PutUint32(value[4:], link.RemoteID)
			link.Attributes = bgplsAttribute(link.Attributes, 258, value)
		}
	}
	// Sort indices, not the large Link values, to avoid copying a link at
	// every comparison. The output compaction below copies each link once.
	b.linkOrder = slices.Grow(b.linkOrder[:0], len(b.snapshot.Links))[:len(b.snapshot.Links)]
	order := b.linkOrder
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(a, c int) int { return bgplsCompareLinks(&b.snapshot.Links[a], &b.snapshot.Links[c]) })
	clear(b.mergedLinks)
	merged := b.mergedLinks[:0]
	for _, i := range order {
		link := &b.snapshot.Links[i]
		if len(merged) == 0 || bgplsCompareLinks(&merged[len(merged)-1], link) != 0 {
			merged = append(merged, *link)
			continue
		}
		prior := &merged[len(merged)-1]
		for _, attr := range link.Attributes {
			if attr.Type == 1095 {
				for _, existing := range prior.Attributes {
					if existing.Type == 1095 && len(attr.Value) > len(existing.Value) {
						prior.Attributes = bgplsSetAttribute(prior.Attributes, 1095, attr.Value)
						break
					}
				}
				continue
			}
			prior.Attributes = bgplsAttribute(prior.Attributes, attr.Type, attr.Value)
		}
		prior.Opaque = append(prior.Opaque, link.Opaque...)
	}
	for i := range merged {
		merged[i].Attributes = b.attributeArrays(merged[i].Attributes, 1096)
	}
	b.mergedLinks = b.snapshot.Links
	b.snapshot.Links = merged
}

// linkRouterIDs uses auxiliary IDs actually advertised by the anchor nodes.
// Pseudonodes have their own identity and do not inherit their DIS router IDs.
func (b *bgplsBuilder) linkRouterIDs(link *linkstateevents.Link) {
	for side, id := range [...]linkstateevents.NodeID{link.Local, link.Remote} {
		var source types.SourceID
		copy(source[:], id.RouterID)
		index, ok := b.nodes[source]
		if !ok {
			continue
		}
		for _, attr := range b.snapshot.Nodes[index].Attributes {
			if attr.Type == 1028 || attr.Type == 1029 {
				link.Attributes = bgplsAttribute(link.Attributes, attr.Type+uint16(side*2), attr.Value)
			}
		}
	}
}

// validateSRv6 joins SID advertisements with the complete locator set, including
// locators in other fragments. The bounded 128-prefix lookup avoids a scan of
// the entire database for every adjacency SID.
func (b *bgplsBuilder) validateSRv6() {
	locators := b.locatorsByPrefix
	for i := range b.snapshot.Links {
		link := &b.snapshot.Links[i]
		var source types.SourceID
		copy(source[:], link.Local.RouterID)
		link.Attributes = slices.DeleteFunc(link.Attributes, func(attr linkstateevents.TLV) bool {
			offset := 6
			switch attr.Type {
			case 1106:
			case 1107:
				offset = 12
			default:
				return false
			}
			sid := netip.AddrFrom16([16]byte(attr.Value[offset : offset+16]))
			topology := uint16(0)
			if len(link.Topologies) > 0 {
				topology = link.Topologies[0]
			}
			for bits := 1; bits <= 128; bits++ {
				key := bgplsPrefixKey{node: source, prefix: netip.PrefixFrom(sid, bits).Masked(), topology: topology}
				state, exists := locators[key]
				if exists && !state.conflict && state.algorithm == attr.Value[3] {
					return false
				}
			}
			// RFC 9352 section 8: "SIDs that do not meet this requirement
			// MUST be ignored." The locator's topology/algorithm must match.
			return true
		})
	}
}

func bgplsCompareLinks(a, c *linkstateevents.Link) int {
	if result := bytes.Compare(a.Local.RouterID, c.Local.RouterID); result != 0 {
		return result
	}
	if result := bytes.Compare(a.Remote.RouterID, c.Remote.RouterID); result != 0 {
		return result
	}
	if result := slices.CompareFunc(a.LocalAddresses, c.LocalAddresses, func(x, y netip.Addr) int { return x.Compare(y) }); result != 0 {
		return result
	}
	if result := slices.CompareFunc(a.RemoteAddresses, c.RemoteAddresses, func(x, y netip.Addr) int { return x.Compare(y) }); result != 0 {
		return result
	}
	if len(a.LocalAddresses)+len(a.RemoteAddresses) == 0 {
		if a.HasLinkIDs != c.HasLinkIDs {
			if a.HasLinkIDs {
				return 1
			}
			return -1
		}
		if a.LocalID < c.LocalID {
			return -1
		}
		if a.LocalID > c.LocalID {
			return 1
		}
		if a.RemoteID < c.RemoteID {
			return -1
		}
		if a.RemoteID > c.RemoteID {
			return 1
		}
	}
	// Both missing and explicit zero mean the default topology.
	var zero = [...]uint16{0}
	at, ct := a.Topologies, c.Topologies
	if len(at) == 0 {
		at = zero[:]
	}
	if len(ct) == 0 {
		ct = zero[:]
	}
	return slices.Compare(at, ct)
}

func (b *bgplsBuilder) attributeArrays(attrs []linkstateevents.TLV, types ...uint16) []linkstateevents.TLV {
	for _, typ := range types {
		size, count, first := 0, 0, 0
		for i, attr := range attrs {
			if attr.Type == typ {
				if count == 0 {
					first = i
				}
				count++
				size += len(attr.Value)
			}
		}
		if count < 2 {
			continue
		}
		value := b.alloc(size)
		offset := 0
		for _, attr := range attrs {
			if attr.Type == typ {
				offset += copy(value[offset:], attr.Value)
			}
		}
		attrs[first].Value = value
		attrs = append(attrs[:first+1], slices.DeleteFunc(attrs[first+1:], func(attr linkstateevents.TLV) bool { return attr.Type == typ })...)
	}
	return attrs
}
