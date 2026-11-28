// Design: docs/architecture/isis/isis-4-component-config.md -- IS-IS two-level SRv6 locator-originator snapshot publisher
// RFC: rfc/full/rfc9352.txt Sections 6 and 7.1 -- SRv6 Locator TLV (type 27), A-flag, and Prefix Attribute Flags precedence
// RFC: rfc/full/rfc7794.txt Sections 2.1 and 2.2 -- Prefix Attribute Flags (sub-TLV 4) and IPv6 Source Router ID (sub-TLV 12)
// RFC: rfc/full/rfc5308.txt Section 2 -- TLV 236 fixed-header External bit (X)
// Related: bgpls_export.go -- bgplsBuilder.locators and prefixAttributes share the TLV 27 framing and TLV 236 X-bit rule
// Related: lsdb_wiring.go -- publishLSPChange triggers locator-originator publication on every LSDB mutation
// Related: register.go -- declares locator-origin:snapshot send and locator-origin:snapshot-request receive on both registrations

package isis

import (
	"cmp"
	"context"
	"encoding/binary"
	"encoding/json"
	"log/slog"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/locatororigin"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// IS-IS TLV and sub-TLV type codes and Prefix Attribute Flag bit masks used to
// resolve SRv6 locator originators. Wire format of TLV 27 (RFC 9352 Section 7.1):
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|   Type (27)   |     Length    |R|R|R|R|    MTID (12 bits)     |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                          Metric (4 octets)                    |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|   Flags (1)   | Algorithm (1) | Loc-Size (1)  | Locator ...   |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  Sub-TLV-len  | Sub-TLVs (variable)                           |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// Prefix Attribute Flags sub-TLV (type 4, RFC 7794 Section 2.1, RFC 9352 Section 6):
//
//	 0 1 2 3 4 5 6 7
//	+-+-+-+-+-+-+-+-+
//	|X|R|N|E|A| ... |
//	+-+-+-+-+-+-+-+-+
//	X = 0x80 (bit 0), R = 0x40 (bit 1), A = 0x08 (bit 4).
//
// IPv6 Source Router ID sub-TLV (type 12, RFC 7794 Section 2.2): 16-octet IPv6 address.
const (
	tlvSRv6Locator           uint8 = 27
	tlvIPv4TERouterID        uint8 = 134
	tlvIPv6TERouterID        uint8 = 140
	tlvMTIPv6Reachability    uint8 = 237
	subTLVPrefixAttrFlags    uint8 = 4
	subTLVIPv6SourceRouterID uint8 = 12

	prefixAttrFlagX byte = 0x80 // Bit 0: External Prefix Flag (RFC 7794 Section 2.1)
	prefixAttrFlagR byte = 0x40 // Bit 1: Re-advertisement Flag (RFC 7794 Section 2.1)
	prefixAttrFlagA byte = 0x08 // Bit 4: Anycast Flag (RFC 9352 Section 6)

	locatorOriginEmitTimeout = 5 * time.Second
)

type locatorOriginEmitFunc func(ctx context.Context, namespace, eventType, direction, peerAddress, event string) (int, error)

// locatorOriginSource serializes two-level SRv6 locator-originator snapshot
// construction and publication through sdk.Plugin.EmitEvent so internal
// (DirectBridge) and forked/external (TLS RPC) IS-IS run the identical code path.
// Safe for concurrent use; startLocatorOrigin MUST be paired with stopLocatorOrigin.
type locatorOriginSource struct {
	mu           sync.Mutex
	emit         locatorOriginEmitFunc
	live         bool
	generation   uint64
	emitErrFound atomic.Bool
}

// startLocatorOrigin binds the SDK emitter so every LSDB mutation, config
// reconcile, and locator-origin:snapshot-request replay publishes snapshots over
// the EventBus identically across DirectBridge and TLS RPC.
func (e *engine) startLocatorOrigin(p *sdk.Plugin) {
	if p == nil {
		return
	}
	e.startLocatorOriginWithEmit(p.EmitEvent)
}

// startLocatorOriginWithEmit installs a custom emitter. Used by startLocatorOrigin
// and unit tests driving snapshot publication directly.
func (e *engine) startLocatorOriginWithEmit(emit locatorOriginEmitFunc) {
	if emit == nil {
		return
	}
	e.locatorOrigin.mu.Lock()
	defer e.locatorOrigin.mu.Unlock()
	e.locatorOrigin.emit = emit
	e.locatorOrigin.live = true
}

// publishLocatorOrigin builds and emits the current two-level locator-originator
// snapshot. No-op before startLocatorOrigin or after stopLocatorOrigin.
func (e *engine) publishLocatorOrigin() {
	e.locatorOrigin.mu.Lock()
	defer e.locatorOrigin.mu.Unlock()
	if !e.locatorOrigin.live {
		return
	}
	e.publishLocatorOriginLocked(false)
}

// stopLocatorOrigin marks the publisher inactive and emits one final snapshot
// with Active=false so the daemon index clears IS-IS locators on graceful stop.
// Idempotent; safe to call from both OnBye and shutdown.
func (e *engine) stopLocatorOrigin() {
	e.locatorOrigin.mu.Lock()
	defer e.locatorOrigin.mu.Unlock()
	if !e.locatorOrigin.live {
		return
	}
	e.locatorOrigin.live = false
	e.publishLocatorOriginLocked(true)
	e.locatorOrigin.emit = nil
}

// handleLocatorOriginEvent inspects one enveloped SDK event string and replays
// the current locator-originator snapshot when it is a
// locator-origin:snapshot-request signal. Returns true when handled.
func (e *engine) handleLocatorOriginEvent(text string) bool {
	if text == "" {
		return false
	}
	if text[0] != '{' {
		return false
	}
	envelope, err := rpc.ParseEventEnvelope(text)
	if err != nil {
		return false //nolint:nilerr // non-enveloped event is not a locator-origin replay request
	}
	if envelope.Namespace != locatororigin.Namespace {
		return false
	}
	if envelope.Event != locatororigin.EventSnapshotRequest {
		return false
	}
	e.publishLocatorOrigin()
	return true
}

// publishLocatorOriginLocked builds and emits one compact JSON snapshot.
// Caller MUST hold e.locatorOrigin.mu.
func (e *engine) publishLocatorOriginLocked(shutdown bool) {
	emit := e.locatorOrigin.emit
	if emit == nil {
		return
	}
	e.mu.Lock()
	present := e.cfg.Present()
	level := e.cfg.Level
	e.mu.Unlock()

	e.locatorOrigin.generation++
	snap := locatororigin.Snapshot{
		Source:     componentName,
		Generation: e.locatorOrigin.generation,
		Active:     !shutdown && present,
	}
	if snap.Active {
		if e.lsdb != nil {
			var l1Raw, l2Raw [][]byte
			if configFormsLevel(level, lsdb.Level1) {
				l1Raw = e.lsdb.RawSnapshot(lsdb.Level1)
			}
			if configFormsLevel(level, lsdb.Level2) {
				l2Raw = e.lsdb.RawSnapshot(lsdb.Level2)
			}
			snap.Entries = buildLocatorOriginEntries(l1Raw, l2Raw)
		}
	}
	payload, err := json.Marshal(&snap)
	if err != nil {
		e.noteLocatorOriginEmitFailed(shutdown, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), locatorOriginEmitTimeout)
	defer cancel()
	if _, err := emit(ctx, locatororigin.Namespace, locatororigin.EventSnapshot, "", "", string(payload)); err != nil {
		e.noteLocatorOriginEmitFailed(shutdown, err)
	}
}

func (e *engine) noteLocatorOriginEmitFailed(shutdown bool, err error) {
	if shutdown {
		e.log.Debug("isis: locator-origin shutdown snapshot not delivered", "error", err)
		return
	}
	if e.ctx.Err() != nil {
		e.log.Debug("isis: locator-origin snapshot not delivered (shutting down)", "error", err)
		return
	}
	if !e.locatorOrigin.emitErrFound.CompareAndSwap(false, true) {
		return
	}
	issue := opererr.Wrap(err,
		"IS-IS did not publish the locator-origin snapshot event",
		"BGP MUP segment discovery cannot verify SRv6 locator originators against this IS-IS database while the event bus refuses publication",
		showDatabaseCommand)
	const subject = "locator-origin"
	reportErr := issueQueue.Load().RaiseError(diagnostic.CodeDoctorInternalDefect, subject,
		issue.Problem(), issue.Fix(), internalDefectRaiseFailed(e.log, subject, issue.Fix()))
	e.log.LogAttrs(context.Background(), slog.LevelWarn, issue.Problem(),
		issue.LogAttrs(slog.String("code", diagnostic.CodeDoctorInternalDefect),
			slog.String("event", locatororigin.EventSnapshot),
			issuequeue.Outcome(reportErr))...)
}

type systemLocatorInfo struct {
	addresses map[netip.Addr]struct{}
	prefixes  map[netip.Prefix]struct{}
	hasTLV237 bool
}

type levelSystemPrefixKey struct {
	level  lsdb.Level
	system types.SystemID
	prefix netip.Prefix
}

type tlv236FlagInfo struct {
	external        bool
	hasFlagsSubTLV  bool
	rFlag           bool
	aFlag           bool
	sourceRouterIDs []netip.Addr
}

type locatorAlgorithmKey struct {
	level    lsdb.Level
	system   types.SystemID
	topology uint16
	prefix   netip.Prefix
}

type locatorAlgorithmState struct {
	algorithm uint8
	conflict  bool
}

type locatorAd struct {
	level           lsdb.Level
	system          types.SystemID
	topology        uint16
	algorithm       uint8
	prefix          netip.Prefix
	hasFlags        bool
	xFlag           bool
	rFlag           bool
	aFlag           bool
	sourceRouterIDs []netip.Addr
}

// buildLocatorOriginEntries resolves every advertised SRv6 locator across both
// IS-IS levels and all LSP fragments into self-contained locatororigin.LocatorEntry
// records. TLV 27 entry validation and RFC 9352 Section 6 / RFC 7794 Section 2.1 /
// RFC 5308 Section 2 flag precedence mirror bgplsBuilder.locators and
// bgplsBuilder.prefixAttributes in bgpls_export.go so both views share one rule.
func buildLocatorOriginEntries(l1Raw, l2Raw [][]byte) []locatororigin.LocatorEntry {
	systems := make(map[types.SystemID]*systemLocatorInfo)
	tlv236Flags := make(map[levelSystemPrefixKey]*tlv236FlagInfo)
	var ads []locatorAd

	collectLevel := func(level lsdb.Level, raw [][]byte) {
		for _, pdu := range raw {
			decoded, err := packet.DecodePDU(pdu)
			if err != nil {
				continue
			}
			if decoded.LSP == nil {
				decoded.Release()
				continue
			}
			if decoded.LSP.LSPID.SourceID().IsPseudonode() {
				decoded.Release()
				continue
			}
			sysID := decoded.LSP.LSPID.SystemID()
			info := systems[sysID]
			if info == nil {
				info = &systemLocatorInfo{
					addresses: make(map[netip.Addr]struct{}),
					prefixes:  make(map[netip.Prefix]struct{}),
				}
				systems[sysID] = info
			}
			ads = collectLSPForLocatorOrigin(level, sysID, decoded.LSP, info, tlv236Flags, ads)
			decoded.Release()
		}
	}

	collectLevel(lsdb.Level1, l1Raw)
	collectLevel(lsdb.Level2, l2Raw)

	if len(ads) == 0 {
		return nil
	}

	// RFC 9352 Section 7.2: detect algorithm conflicts per (level, system, MTID, locator)
	// mirroring bgplsBuilder.locators in bgpls_export.go.
	algos := make(map[locatorAlgorithmKey]locatorAlgorithmState, len(ads))
	for i := range ads {
		k := locatorAlgorithmKey{
			level:    ads[i].level,
			system:   ads[i].system,
			topology: ads[i].topology,
			prefix:   ads[i].prefix,
		}
		state, exists := algos[k]
		if !exists {
			algos[k] = locatorAlgorithmState{algorithm: ads[i].algorithm}
			continue
		}
		if state.algorithm != ads[i].algorithm {
			state.conflict = true
			algos[k] = state
		}
	}

	byPrefix := make(map[netip.Prefix][]locatorAd)
	for i := range ads {
		ad := ads[i]
		k := locatorAlgorithmKey{
			level:    ad.level,
			system:   ad.system,
			topology: ad.topology,
			prefix:   ad.prefix,
		}
		if algos[k].conflict {
			continue
		}
		applyTLV236Precedence(&ad, tlv236Flags[levelSystemPrefixKey{
			level:  ad.level,
			system: ad.system,
			prefix: ad.prefix,
		}])
		byPrefix[ad.prefix] = append(byPrefix[ad.prefix], ad)
	}

	if len(byPrefix) == 0 {
		return nil
	}

	prefixes := make([]netip.Prefix, 0, len(byPrefix))
	for p := range byPrefix {
		prefixes = append(prefixes, p)
	}
	slices.SortFunc(prefixes, comparePrefixes)

	entries := make([]locatororigin.LocatorEntry, 0, len(prefixes))
	for _, pfx := range prefixes {
		entries = append(entries, resolveLocatorPrefixEntry(pfx, byPrefix[pfx], systems))
	}
	return entries
}

// collectLSPForLocatorOrigin walks one decoded router LSP fragment and records
// its IPv6 addresses (TLV 140, TLV 232, local host /128 TLV 236), IPv4
// addresses and local prefixes (TLV 132, TLV 134, local TLV 135), local TLV 236
// prefixes and flags, TLV 237 presence, and validated TLV 27 SRv6 Locator
// entries.
func collectLSPForLocatorOrigin(
	level lsdb.Level,
	sysID types.SystemID,
	lsp *packet.LSP,
	info *systemLocatorInfo,
	tlv236Flags map[levelSystemPrefixKey]*tlv236FlagInfo,
	ads []locatorAd,
) []locatorAd {
	for _, tlv := range lsp.TLVs {
		switch tlv.Type {
		case packet.TLVIPInterfaceAddress:
			v4If, err := packet.DecodeIPv4InterfaceAddrTLV(tlv.Value)
			if err != nil {
				continue
			}
			for _, addr := range v4If.Addresses {
				info.addresses[addr] = struct{}{}
				info.prefixes[netip.PrefixFrom(addr, packet.MaxIPv4PrefixLen)] = struct{}{}
			}
		case tlvIPv4TERouterID:
			if len(tlv.Value) == packet.IPv4AddrLen {
				addr := netip.AddrFrom4([packet.IPv4AddrLen]byte(tlv.Value))
				info.addresses[addr] = struct{}{}
				info.prefixes[netip.PrefixFrom(addr, packet.MaxIPv4PrefixLen)] = struct{}{}
			}
		case packet.TLVExtendedIPReach:
			v4Reach, err := packet.DecodeExtendedIPReachTLV(tlv.Value)
			if err != nil {
				continue
			}
			for _, entry := range v4Reach.Entries {
				if !isLocalTLV135Entry(entry) {
					continue
				}
				pfx := entry.Prefix.Masked()
				info.prefixes[pfx] = struct{}{}
				if pfx.Bits() == packet.MaxIPv4PrefixLen {
					info.addresses[pfx.Addr()] = struct{}{}
				}
			}
		case tlvIPv6TERouterID:
			if len(tlv.Value) == packet.IPv6AddrLen {
				info.addresses[netip.AddrFrom16([packet.IPv6AddrLen]byte(tlv.Value))] = struct{}{}
			}
		case packet.TLVIPv6InterfaceAddress:
			v6If, err := packet.DecodeIPv6InterfaceAddrTLV(tlv.Value)
			if err != nil {
				continue
			}
			for _, addr := range v6If.Addresses {
				info.addresses[addr] = struct{}{}
			}
		case packet.TLVIPv6Reachability:
			v6Reach, err := packet.DecodeIPv6ReachabilityTLV(tlv.Value)
			if err != nil {
				continue
			}
			for _, entry := range v6Reach.Entries {
				pfx := entry.Prefix.Masked()
				recordTLV236Flags(tlv236Flags, levelSystemPrefixKey{level: level, system: sysID, prefix: pfx}, entry)
				if isLocalTLV236Entry(entry) {
					info.prefixes[pfx] = struct{}{}
					if pfx.Bits() == packet.MaxIPv6PrefixLen {
						info.addresses[pfx.Addr()] = struct{}{}
					}
				}
			}
		case tlvMTIPv6Reachability:
			info.hasTLV237 = true
		case tlvSRv6Locator:
			ads = appendDecodedTLV27(ads, level, sysID, tlv.Value)
		}
	}
	return ads
}

// isLocalTLV236Entry reports whether a TLV 236 IPv6 Reachability entry belongs
// to the advertising system itself rather than being redistributed from another
// protocol (RFC 5308 Section 2 External bit) or leaked between levels (RFC 5308
// Section 2 Up/Down bit or RFC 7794 Section 2.1 sub-TLV 4 R-Flag).
func isLocalTLV236Entry(entry packet.IPv6ReachEntry) bool {
	if entry.External {
		return false
	}
	if entry.UpDown {
		return false
	}
	return !hasSubTLV4Flag(entry.SubTLVs, prefixAttrFlagR)
}

// isLocalTLV135Entry reports whether a TLV 135 Extended IPv4 Reachability entry
// belongs to the advertising system itself rather than being leaked between
// levels (RFC 5305 Section 4.1 Up/Down bit or RFC 7794 Section 2.1 sub-TLV 4
// R-Flag) or redistributed from another protocol (RFC 7794 Section 2.1 sub-TLV
// 4 X-Flag).
func isLocalTLV135Entry(entry packet.ExtIPReachEntry) bool {
	if entry.UpDown {
		return false
	}
	if hasSubTLV4Flag(entry.SubTLVs, prefixAttrFlagR) {
		return false
	}
	return !hasSubTLV4Flag(entry.SubTLVs, prefixAttrFlagX)
}

func hasSubTLV4Flag(subTLVs []packet.SubTLV, mask byte) bool {
	for _, sub := range subTLVs {
		if sub.Type != subTLVPrefixAttrFlags {
			continue
		}
		if len(sub.Value) == 0 {
			continue
		}
		if sub.Value[0]&mask != 0 {
			return true
		}
	}
	return false
}

// recordTLV236Flags records the RFC 5308 Section 2 fixed-header external bit
// and any Prefix Attribute Flags (sub-TLV 4) or IPv6 Source Router ID (sub-TLV
// 12) carried by one TLV 236 entry. Mirrors bgplsBuilder.prefixAttributes in
// bgpls_export.go.
func recordTLV236Flags(out map[levelSystemPrefixKey]*tlv236FlagInfo, key levelSystemPrefixKey, entry packet.IPv6ReachEntry) {
	info := out[key]
	if info == nil {
		info = &tlv236FlagInfo{}
		out[key] = info
	}
	// RFC 5308 Section 2: "If the prefix was distributed into IS-IS from another
	// routing protocol, the external bit SHALL be set to 1."
	if entry.External {
		info.external = true
	}
	for _, sub := range entry.SubTLVs {
		switch sub.Type {
		case subTLVPrefixAttrFlags:
			if len(sub.Value) == 0 {
				continue
			}
			// RFC 7794 Section 2.1: "In TLVs 236 and 237, this flag SHOULD always be
			// sent as 0 and MUST be ignored on receipt." Only R and A come from the
			// TLV 236 sub-TLV; X comes from the fixed header above.
			info.hasFlagsSubTLV = true
			if sub.Value[0]&prefixAttrFlagR != 0 {
				info.rFlag = true
			}
			if sub.Value[0]&prefixAttrFlagA != 0 {
				info.aFlag = true
			}
		case subTLVIPv6SourceRouterID:
			if len(sub.Value) == packet.IPv6AddrLen {
				addr := netip.AddrFrom16([packet.IPv6AddrLen]byte(sub.Value))
				if !slices.Contains(info.sourceRouterIDs, addr) {
					info.sourceRouterIDs = append(info.sourceRouterIDs, addr)
				}
			}
		}
	}
}

// appendDecodedTLV27 validates and decodes one SRv6 Locator TLV (type 27) into
// locatorAd records, mirroring bgplsBuilder.locators in bgpls_export.go.
func appendDecodedTLV27(ads []locatorAd, level lsdb.Level, sysID types.SystemID, value []byte) []locatorAd {
	if len(value) < 2 {
		return ads
	}
	// Validate the entire TLV before accepting any entry.
	// RFC 9352 Section 7.1: "Loc-Size:  1 octet.  Number of bits in the SRv6
	// Locator field, which MUST be from the range (1-128).  The entire TLV MUST
	// be ignored if the Loc-Size is outside this range."
	for off := 2; off < len(value); {
		if len(value)-off < 8 {
			return ads
		}
		bits := int(value[off+6])
		if bits < 1 {
			return ads
		}
		if bits > packet.MaxIPv6PrefixLen {
			return ads
		}
		subOffset := off + 7 + (bits+7)/8
		if subOffset >= len(value) {
			return ads
		}
		end := subOffset + 1 + int(value[subOffset])
		if end > len(value) {
			return ads
		}
		it := packet.NewTLVIterator(value[subOffset+1 : end])
		for {
			_, _, ok := it.Next()
			if !ok {
				break
			}
		}
		if it.Err() != nil {
			return ads
		}
		off = end
	}

	topology := binary.BigEndian.Uint16(value) & 0x0fff
	for off := 2; off < len(value); {
		bits := int(value[off+6])
		octets := (bits + 7) / 8
		var raw [packet.IPv6AddrLen]byte
		copy(raw[:], value[off+7:off+7+octets])
		locator := netip.PrefixFrom(netip.AddrFrom16(raw), bits).Masked()
		subOffset := off + 7 + octets
		end := subOffset + 1 + int(value[subOffset])

		ad := locatorAd{
			level:     level,
			system:    sysID,
			topology:  topology,
			algorithm: value[off+5],
			prefix:    locator,
		}
		it := packet.NewTLVIterator(value[subOffset+1 : end])
		for {
			subType, subVal, ok := it.Next()
			if !ok {
				break
			}
			switch subType {
			case subTLVPrefixAttrFlags:
				if len(subVal) == 0 {
					continue
				}
				ad.hasFlags = true
				ad.xFlag = subVal[0]&prefixAttrFlagX != 0
				ad.rFlag = subVal[0]&prefixAttrFlagR != 0
				ad.aFlag = subVal[0]&prefixAttrFlagA != 0
			case subTLVIPv6SourceRouterID:
				if len(subVal) == packet.IPv6AddrLen {
					addr := netip.AddrFrom16([packet.IPv6AddrLen]byte(subVal))
					if !slices.Contains(ad.sourceRouterIDs, addr) {
						ad.sourceRouterIDs = append(ad.sourceRouterIDs, addr)
					}
				}
			}
		}
		ads = append(ads, ad)
		off = end
	}
	return ads
}

// applyTLV236Precedence applies RFC 9352 Section 6 and RFC 7794 Section 2.1
// precedence between a system's TLV 27 locator entry and its same-level TLV 236
// Prefix Reachability entry for the same prefix. Mirrors bgplsBuilder.locators
// and bgplsBuilder.prefixAttributes in bgpls_export.go.
func applyTLV236Precedence(ad *locatorAd, r236 *tlv236FlagInfo) {
	if r236 == nil {
		return
	}
	// RFC 9352 Section 6: "In case of any inconsistency between the Prefix
	// Attribute Flags advertised in the Locator TLV and in the Prefix
	// Reachability TLV, the ones advertised in the Prefix Reachability TLV MUST
	// be preferred."
	// RFC 5308 Section 2 / RFC 7794 Section 2.1: the TLV 236 fixed-header
	// external bit always decides X whether or not TLV 236 carries sub-TLV 4.
	ad.xFlag = r236.external
	if r236.hasFlagsSubTLV {
		ad.hasFlags = true
		ad.rFlag = r236.rFlag
		ad.aFlag = r236.aFlag
	}
	for _, id := range r236.sourceRouterIDs {
		if !slices.Contains(ad.sourceRouterIDs, id) {
			ad.sourceRouterIDs = append(ad.sourceRouterIDs, id)
		}
	}
}

// resolveLocatorPrefixEntry folds all level-1 and level-2 advertisements of one
// locator prefix into a single LocatorEntry verdict.
func resolveLocatorPrefixEntry(
	prefix netip.Prefix,
	ads []locatorAd,
	systems map[types.SystemID]*systemLocatorInfo,
) locatororigin.LocatorEntry {
	anyAFlag := false
	originators := make(map[types.SystemID]struct{})
	readvertisers := make(map[types.SystemID]struct{})
	missingFlags := make(map[types.SystemID]struct{})
	sourceIDs := make(map[netip.Addr]struct{})

	for i := range ads {
		ad := &ads[i]
		if ad.aFlag {
			anyAFlag = true
		}
		for _, id := range ad.sourceRouterIDs {
			sourceIDs[id] = struct{}{}
		}
		if ad.rFlag {
			readvertisers[ad.system] = struct{}{}
			continue
		}
		if ad.xFlag {
			readvertisers[ad.system] = struct{}{}
			continue
		}
		if !ad.hasFlags {
			missingFlags[ad.system] = struct{}{}
			continue
		}
		originators[ad.system] = struct{}{}
	}

	var tb textbuf.Buffer
	// RFC 9352 Section 6: "The same prefix/SRv6 Locator can be advertised by
	// multiple routers.  If at least one of them sets the A-flag in its
	// advertisement, the prefix/SRv6 Locator SHOULD be considered as anycast."
	if anyAFlag {
		return locatororigin.LocatorEntry{
			Prefix:      prefix,
			Known:       false,
			ReasonClass: locatororigin.ReasonAnycast,
			Reason:      tb.Str("covering locator ").Prefix(prefix).Str(" carries the anycast A-flag (RFC 9352 Section 6)").String(),
		}
	}
	if len(originators) > 1 {
		return locatororigin.LocatorEntry{
			Prefix:      prefix,
			Known:       false,
			ReasonClass: locatororigin.ReasonAnycast,
			Reason:      tb.Str("covering locator ").Prefix(prefix).Str(" is originated by multiple systems (anycast)").String(),
		}
	}
	if len(originators) == 1 {
		sysID := firstSystemID(originators)
		// Same system advertising across L1/L2 (one level carrying flags, one
		// without) does not make the originator ambiguous, but any other
		// distinct system advertising the same locator without Prefix Attribute
		// Flags (RFC 9352 Section 7.1) could be another originator or an
		// unflagged leaker, so the originator cannot be proven.
		delete(missingFlags, sysID)
		if len(missingFlags) > 0 {
			unflaggedSys := firstSystemID(missingFlags).String()
			return locatororigin.LocatorEntry{
				Prefix:          prefix,
				Known:           false,
				Originator:      unflaggedSys,
				SourceRouterIDs: sortedAddrs(sourceIDs),
				ReasonClass:     locatororigin.ReasonMissingFlags,
				Reason:          tb.Str("covering locator ").Prefix(prefix).Str(" from ").Str(unflaggedSys).Str(" carries no Prefix Attribute Flags sub-TLV (RFC 9352 Section 7.1)").String(),
			}
		}
		info := systems[sysID]
		var addrs []netip.Addr
		var prefixes []netip.Prefix
		hasTLV237 := false
		if info != nil {
			addrs = sortedAddrs(info.addresses)
			prefixes = sortedPrefixes(info.prefixes)
			hasTLV237 = info.hasTLV237
		}
		return locatororigin.LocatorEntry{
			Prefix:             prefix,
			Known:              true,
			Originator:         sysID.String(),
			Addresses:          addrs,
			Prefixes:           prefixes,
			SourceRouterIDs:    sortedAddrs(sourceIDs),
			HasUndecodedTLV237: hasTLV237,
			ReasonClass:        locatororigin.ReasonNone,
		}
	}
	// RFC 7794 Section 2.1: R-Flag is "Set when the prefix has been leaked from
	// one level to another (upwards or downwards)" and X-Flag is "Set if the
	// prefix has been redistributed from another protocol." Neither proves the
	// advertising router is the originator; RFC 7794 Section 2.2 IPv6 Source
	// Router ID sub-TLVs travel along in SourceRouterIDs for matching addresses.
	if len(readvertisers) > 0 {
		sysID := firstSystemID(readvertisers).String()
		return locatororigin.LocatorEntry{
			Prefix:          prefix,
			Known:           false,
			Originator:      sysID,
			SourceRouterIDs: sortedAddrs(sourceIDs),
			ReasonClass:     locatororigin.ReasonReadvertised,
			Reason:          tb.Str("covering locator ").Prefix(prefix).Str(" re-advertised by ").Str(sysID).Str(" (R or X set)").String(),
		}
	}
	// RFC 9352 Section 7.1: "The Prefix Attribute Flags sub-TLV MUST be included
	// in the Locator TLV when it is leaked upwards in the hierarchy or
	// originated as a result of the redistribution from another protocol or
	// another IS-IS instance.  If the Prefix Attribute Flags sub-TLV is not
	// included in these cases, receivers will be unable to determine the correct
	// source of the advertisement."
	sysID := firstSystemID(missingFlags).String()
	return locatororigin.LocatorEntry{
		Prefix:          prefix,
		Known:           false,
		Originator:      sysID,
		SourceRouterIDs: sortedAddrs(sourceIDs),
		ReasonClass:     locatororigin.ReasonMissingFlags,
		Reason:          tb.Str("covering locator ").Prefix(prefix).Str(" from ").Str(sysID).Str(" carries no Prefix Attribute Flags sub-TLV (RFC 9352 Section 7.1)").String(),
	}
}

func firstSystemID(set map[types.SystemID]struct{}) types.SystemID {
	var best types.SystemID
	first := true
	for id := range set {
		if first {
			best = id
			first = false
			continue
		}
		if id.String() < best.String() {
			best = id
		}
	}
	return best
}

func sortedAddrs(set map[netip.Addr]struct{}) []netip.Addr {
	if len(set) == 0 {
		return nil
	}
	out := make([]netip.Addr, 0, len(set))
	for a := range set {
		out = append(out, a)
	}
	slices.SortFunc(out, netip.Addr.Compare)
	return out
}

func sortedPrefixes(set map[netip.Prefix]struct{}) []netip.Prefix {
	if len(set) == 0 {
		return nil
	}
	out := make([]netip.Prefix, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	slices.SortFunc(out, comparePrefixes)
	return out
}

func comparePrefixes(a, b netip.Prefix) int {
	return cmp.Or(a.Addr().Compare(b.Addr()), cmp.Compare(a.Bits(), b.Bits()))
}
