// Design: docs/architecture/core-design.md -- withdrawal tracking for route reflector
// Overview: rr.go -- RouteReflector uses withdrawal map for peer-down cleanup
//
// RFC 4456 Section 8: when a client peer goes down, the route reflector
// must withdraw all routes previously advertised from that peer.
// This file tracks announced/withdrawn NLRIs per source peer so that
// handleStateDown can hand them to the engine as a source-DOWN recovery, which
// withdraws them from the remaining peers.

package rr

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/textparse"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// withdrawalKey includes the original RECEIVE owner, so address aliases can
// retain an identical native route without overwriting one another.
type withdrawalKey struct {
	owner      uint64
	connection uint64
	route      string
}

// withdrawalInfo carries the original RECEIVE provenance, never a DOWN owner.
type withdrawalInfo struct {
	Family   family.Family
	Prefix   string // Text NLRI, or native hex when WireForm is true.
	WireForm bool
	AddPath  bool // Native hex carries a Path Identifier, including zero.
	// Generation is the session-generation of the peer-down whose departure
	// carries this route, set by handleStateDown. A merged departure keeps
	// each route's own, so an older session's route never borrows a newer
	// bound (enqueueDeparture).
	Generation        uint64
	SourceReceipt     uint64
	SourceOwner       uint64
	SourcePolicyEpoch uint64
	ConnectionID      uint64
	MsgID             uint64
	PathID            uint32
	NativeHex         string
	NoLLGR            bool
	Retained          bool
	Closing           bool
}

// updateWithdrawalMapWire updates the withdrawal map from raw wire UPDATE data.
// Uses NLRIIterator for zero-allocation NLRI walking on IPv4/IPv6 unicast.
// Falls back to NLRIs() (allocating) for non-unicast families.
// Caller must hold rr.withdrawalMu.
func (rr *routeReflector) updateWithdrawalMapWire(sourcePeer string, msg *bgptypes.RawMessage, connection uint64) error {
	if msg.WireUpdate == nil {
		return errors.New("bgp-rr: received UPDATE has no native payload")
	}
	enc, known := msg.SourceEncodingContext()
	if !known {
		return errors.New("bgp-rr: received UPDATE encoding context is unknown")
	}
	noLLGR, err := wireNoLLGR(msg.AttrsWire)
	if err != nil {
		return err
	}
	records := []inventoryRecord{}
	collect := func(fam family.Family, raw []byte, withdraw bool) error {
		addPath := false
		if enc != nil {
			addPath = enc.AddPath(fam)
		}
		split := nlrisplit.Get(fam)
		if withdraw {
			split = nlrisplit.GetWithdraw(fam)
		}
		if split == nil {
			return nlrisplit.ErrUnsupported
		}
		// Validate the complete section before visiting it; no malformed suffix
		// may turn an otherwise rejected UPDATE into partial current membership.
		if _, err := split(raw, addPath, nil); err != nil {
			return err
		}
		var parseErr error
		_, err := split(raw, addPath, func(raw []byte) bool {
			pathID, native, err := nlri.SplitPathID(raw, addPath)
			if err != nil {
				parseErr = err
				return false
			}
			info := withdrawalInfo{Family: fam, PathID: pathID, AddPath: addPath, SourceOwner: msg.SourceOwner, ConnectionID: connection,
				SourceReceipt: msg.SourceReceipt, MsgID: msg.MessageID, SourcePolicyEpoch: msg.SourcePolicyEpoch, NoLLGR: noLLGR}
			record, err := nativeInventory(info, native, withdraw)
			if err != nil {
				parseErr = err
				return false
			}
			records = append(records, record)
			return true
		})
		if err != nil {
			return err
		}
		return parseErr
	}
	wu := msg.WireUpdate
	mpDel, err := wu.MPUnreach()
	if err != nil {
		return err
	}
	if mpDel != nil {
		if err := collect(mpDel.Family(), mpDel.WithdrawnBytes(), true); err != nil {
			return err
		}
	}
	bodyDel, err := wu.Withdrawn()
	if err != nil {
		return err
	}
	if len(bodyDel) > 0 {
		if err := collect(family.IPv4Unicast, bodyDel, true); err != nil {
			return err
		}
	}
	mpAdd, err := wu.MPReach()
	if err != nil {
		return err
	}
	if mpAdd != nil {
		if err := collect(mpAdd.Family(), mpAdd.NLRIBytes(), false); err != nil {
			return err
		}
	}
	bodyAdd, err := wu.NLRI()
	if err != nil {
		return err
	}
	if len(bodyAdd) > 0 {
		if err := collect(family.IPv4Unicast, bodyAdd, false); err != nil {
			return err
		}
	}
	for _, record := range records {
		rr.applyInventoryLocked(sourcePeer, record)
	}
	return nil
}

// isUnicast returns true for IPv4/IPv6 unicast families where NLRIIterator
// prefix bytes can be converted to netip.Prefix directly (zero-alloc path).
func isUnicast(f family.Family) bool {
	return f == family.IPv4Unicast || f == family.IPv6Unicast
}

// walkUnicastNLRIs walks NLRIs via iterator and updates the withdrawal map.
// It builds one route-key string per NLRI from the raw prefix bytes, and an
// announcement also allocates the stored withdrawal text.
// addPath MUST be the ADD-PATH receive state of fam on the source session,
// the same value the iterator was built with.
//
// RFC 7911 Section 5: "a new advertisement for a given address prefix and a
// given Path Identifier replaces a previous advertisement for the same address
// prefix and Path Identifier." Under ADD-PATH the identifier is part of the
// route's identity, so it is part of the key, and the peer-down withdrawal goes
// out as the native NLRI with its identifier, as the MPLS-label arm does.
func (rr *routeReflector) walkUnicastNLRIs(sourcePeer string, fam family.Family, iter *nlri.NLRIIterator, addPath bool, action string, sourcePolicyEpoch, sourceReceipt, sourceOwner, connection, msgID uint64, noLLGR bool) {
	famName := fam.String()
	isV6 := fam.AFI == family.AFIIPv6
	for {
		prefix, pathID, ok := iter.Next()
		if !ok {
			break
		}
		key := prefixBytesToKey(prefix, isV6)
		if key == "" {
			continue
		}
		var tb textbuf.Buffer
		tb.Str(famName).Byte('|').Str(key)
		if addPath {
			tb.Str("|addpath|").Uint32(pathID)
		}
		routeKey := tb.String()
		switch action {
		case actionAdd:
			if rr.withdrawals[sourcePeer] == nil {
				rr.withdrawals[sourcePeer] = make(map[withdrawalKey]withdrawalInfo)
			}
			if addPath {
				rr.putWithdrawalLocked(sourcePeer, withdrawalKey{owner: sourceOwner, connection: connection, route: routeKey}, withdrawalInfo{
					Family: fam, PathID: pathID, NativeHex: hex.EncodeToString(prefix), Prefix: addPathUnicastHex(pathID, prefix),
					WireForm: true, AddPath: true,
					SourceOwner: sourceOwner, SourceReceipt: sourceReceipt, ConnectionID: connection, MsgID: msgID, NoLLGR: noLLGR, SourcePolicyEpoch: sourcePolicyEpoch,
				})
				continue
			}
			rr.putWithdrawalLocked(sourcePeer, withdrawalKey{owner: sourceOwner, connection: connection, route: routeKey}, withdrawalInfo{Family: fam, PathID: pathID, NativeHex: hex.EncodeToString(prefix), Prefix: tb.Reset().Str("prefix ").Str(key).String(), SourceOwner: sourceOwner, SourceReceipt: sourceReceipt, ConnectionID: connection, MsgID: msgID, NoLLGR: noLLGR, SourcePolicyEpoch: sourcePolicyEpoch})
		case actionDel:
			if rr.withdrawals[sourcePeer] != nil {
				rr.removeWithdrawalLocked(sourcePeer, withdrawalKey{owner: sourceOwner, connection: connection, route: routeKey}, msgID)
			}
		}
	}
}

// addPathUnicastHex answers the hex of one ADD-PATH unicast NLRI: the Path
// Identifier, then the length octet and prefix octets NLRIIterator.Next gave.
//
// RFC 7911 Section 3: "In order to carry the Path Identifier in an UPDATE
// message, the NLRI encoding MUST be extended by prepending the Path Identifier
// field, which is of four octets."
//
//	+--------------------------------+
//	| Path Identifier (4 octets)     |  offset 0
//	+--------------------------------+
//	| Length (1 octet)               |  offset 4
//	+--------------------------------+
//	| Prefix (variable)              |  offset 5
//	+--------------------------------+
func addPathUnicastHex(pathID uint32, prefix []byte) string {
	var id [4]byte
	binary.BigEndian.PutUint32(id[:], pathID)
	var tb textbuf.Buffer
	return tb.Hex(id[:]).Hex(prefix).String()
}

// prefixBytesToKey converts raw NLRI prefix bytes from NLRIIterator to a route key string.
// Input: [bitLen, addr_bytes...] from NLRIIterator.Next().
func prefixBytesToKey(prefix []byte, isV6 bool) string {
	if len(prefix) == 0 {
		return ""
	}
	bitLen := int(prefix[0])
	addrBytes := prefix[1:]
	if isV6 {
		var addr [16]byte
		copy(addr[:], addrBytes)
		p := netip.PrefixFrom(netip.AddrFrom16(addr), bitLen)
		return p.Masked().String()
	}
	var addr [4]byte
	copy(addr[:], addrBytes)
	p := netip.PrefixFrom(netip.AddrFrom4(addr), bitLen)
	return p.Masked().String()
}

// walkNLRIsAllocating updates the withdrawal map using parsed NLRI objects.
// Used for non-unicast families where raw prefix bytes need family-specific decoding.
func (rr *routeReflector) walkNLRIsAllocating(sourcePeer string, fam family.Family, nlris []nlri.NLRI, err error, action string, sourcePolicyEpoch, sourceReceipt, sourceOwner, connection, msgID uint64, noLLGR bool) {
	if err != nil || len(nlris) == 0 {
		return
	}
	if fam.SAFI == family.SAFIVPN {
		// RFC 8277 Section 2.4: VPN identity excludes Compatibility.
		rr.walkVPNNLRIs(sourcePeer, fam, nlris, action, sourcePolicyEpoch, sourceReceipt, sourceOwner, connection, msgID, noLLGR)
		return
	}
	familyStr := fam.String()
	switch action {
	case actionAdd:
		if rr.withdrawals[sourcePeer] == nil {
			rr.withdrawals[sourcePeer] = make(map[withdrawalKey]withdrawalInfo)
		}
		var tb textbuf.Buffer
		for _, n := range nlris {
			s := n.String()
			tb.Reset().Str(familyStr).Byte('|').Str(nlriKey(s))
			info := withdrawalInfo{PathID: n.PathID(), Family: fam, Prefix: s, SourceOwner: sourceOwner, SourceReceipt: sourceReceipt, ConnectionID: connection, MsgID: msgID, NoLLGR: noLLGR, SourcePolicyEpoch: sourcePolicyEpoch}
			if wire, ok := n.(*nlri.WireNLRI); ok {
				_, native, splitErr := nlri.SplitPathID(wire.Bytes(), wire.HasAddPath())
				if splitErr != nil {
					continue
				}
				info.NativeHex = hex.EncodeToString(native)
			}
			if fam.SAFI == family.SAFIMPLSLabel {
				if wire, ok := n.(*nlri.WireNLRI); ok && wire.HasAddPath() {
					// The prefix alone does not identify a negotiated path.
					// Keep the announcement for native peer-down withdrawal.
					tb.Str("|addpath|").Uint32(wire.PathID())
					var native textbuf.Buffer
					info.Prefix = native.Hex(wire.Bytes()).String()
					info.WireForm = true
					info.AddPath = true
				}
			}
			rr.putWithdrawalLocked(sourcePeer, withdrawalKey{owner: sourceOwner, connection: connection, route: tb.String()}, info)
		}
	case actionDel:
		if rr.withdrawals[sourcePeer] != nil {
			var tb textbuf.Buffer
			for _, n := range nlris {
				tb.Reset().Str(familyStr).Byte('|').Str(nlriKey(n.String()))
				if fam.SAFI == family.SAFIMPLSLabel {
					if aware, ok := n.(nlri.AddPathAware); ok && aware.HasAddPath() {
						// Withdrawal INETs retain negotiation even for PathID zero.
						tb.Str("|addpath|").Uint32(n.PathID())
					}
				}
				rr.removeWithdrawalLocked(sourcePeer, withdrawalKey{owner: sourceOwner, connection: connection, route: tb.String()}, msgID)
			}
		}
	}
}

// walkVPNNLRIs retains native bytes separately from the registered RD/prefix
// identity. Callers MUST hold withdrawalMu, as for walkNLRIsAllocating.
// RFC 8277 Section 2.4: "Upon reception, the value of the Compatibility field
// MUST be ignored." RFC 7911 identifiers, including zero, remain part of the key.
func (rr *routeReflector) walkVPNNLRIs(sourcePeer string, fam family.Family, nlris []nlri.NLRI, action string, sourcePolicyEpoch, sourceReceipt, sourceOwner, connection, msgID uint64, noLLGR bool) {
	familyName := fam.String()
	keyFunc := nlrisplit.GetPrefixKey(fam)
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	var tb textbuf.Buffer
	for _, n := range nlris {
		wire, ok := n.(*nlri.WireNLRI)
		if !ok {
			logger().Warn("VPN inventory requires native NLRI", "family", familyName)
			continue
		}
		pathID, payload, err := nlri.SplitPathID(wire.Bytes(), wire.HasAddPath())
		if err != nil {
			logger().Warn("VPN inventory path identifier rejected", "family", familyName, "error", err)
			continue
		}
		key, err := keyFunc(payload, scratch[:], action == actionDel)
		if err != nil {
			logger().Warn("VPN inventory identity rejected", "family", familyName, "error", err)
			continue
		}
		routeKey := tb.Reset().Str(familyName).Byte('|').Bool(wire.HasAddPath()).Byte('|').Uint32(pathID).Byte('|').Hex(key).String()
		switch action {
		case actionAdd:
			if rr.withdrawals[sourcePeer] == nil {
				rr.withdrawals[sourcePeer] = make(map[withdrawalKey]withdrawalInfo)
			}
			rr.putWithdrawalLocked(sourcePeer, withdrawalKey{owner: sourceOwner, connection: connection, route: routeKey}, withdrawalInfo{
				Family: fam, PathID: pathID, NativeHex: hex.EncodeToString(payload), Prefix: tb.Reset().Hex(wire.Bytes()).String(),
				WireForm: true, AddPath: wire.HasAddPath(),
				SourceOwner: sourceOwner, SourceReceipt: sourceReceipt, ConnectionID: connection, MsgID: msgID, NoLLGR: noLLGR, SourcePolicyEpoch: sourcePolicyEpoch,
			})
		case actionDel:
			rr.removeWithdrawalLocked(sourcePeer, withdrawalKey{owner: sourceOwner, connection: connection, route: routeKey}, msgID)
		}
	}
}

// nlriKey extracts the compact routing key from an NLRI string.
// Strips the "prefix " type keyword since it is redundant within a family.
func nlriKey(s string) string {
	if after, ok := strings.CutPrefix(s, "prefix "); ok {
		return after
	}
	return s
}

// --- Text-path withdrawal tracking (fork-mode fallback) ---

// familyOperation represents a single add or del operation for a family.
type familyOperation struct {
	action  string // "add" or "del"
	pathID  uint32
	addPath bool
	nlris   []string
}

// updateWithdrawalMapText updates the withdrawal map from text-parsed NLRI operations.
// Caller must hold rr.withdrawalMu. The incoming map is keyed by family.Family,
// which withdrawalInfo.Family keeps, so the peer-down retention query needs no
// name lookup.
func (rr *routeReflector) updateWithdrawalMapText(sourcePeer string, ops map[family.Family][]familyOperation, sourcePolicyEpoch, sourceReceipt, sourceOwner, connection, msgID uint64, noLLGR bool) {
	for fam, familyOps := range ops {
		famName := fam.String()
		for _, op := range familyOps {
			switch op.action {
			case actionAdd:
				if rr.withdrawals[sourcePeer] == nil {
					rr.withdrawals[sourcePeer] = make(map[withdrawalKey]withdrawalInfo)
				}
				for _, s := range op.nlris {
					if s != "" {
						routeKey := textRouteKey(famName, s, op.pathID, op.addPath)
						rr.putWithdrawalLocked(sourcePeer, withdrawalKey{owner: sourceOwner, connection: connection, route: routeKey}, withTextNative(withdrawalInfo{PathID: op.pathID, AddPath: op.addPath, Family: fam, Prefix: s, SourceOwner: sourceOwner, SourceReceipt: sourceReceipt, ConnectionID: connection, MsgID: msgID, NoLLGR: noLLGR, SourcePolicyEpoch: sourcePolicyEpoch}))
					}
				}
			case actionDel:
				if rr.withdrawals[sourcePeer] != nil {
					for _, s := range op.nlris {
						if s != "" {
							rr.removeWithdrawalLocked(sourcePeer, withdrawalKey{owner: sourceOwner, connection: connection, route: textRouteKey(famName, s, op.pathID, op.addPath)}, msgID)
						}
					}
				}
			}
		}
	}
}

// parseTextNLRIOps extracts family operations (add/del + NLRIs) from a text UPDATE.
//
// Format: "peer <addr> remote as <n> <dir> update <id> <attrs> [next <nh>] nlri <fam> add|del <nlris> ..."
//
// Key-dispatch loop processes keywords sequentially, resolving aliases via textparse.ResolveAlias:
// - Attribute keywords (origin, path, pref, etc.): skip value(s)
// - "nlri": consume family, extract action (add/del) and collect NLRI tokens until next keyword.
// The returned map is keyed by family.Family; unregistered family names are dropped.
func parseTextNLRIOps(text string) map[family.Family][]familyOperation {
	result := make(map[family.Family][]familyOperation)
	s := textparse.NewScanner(strings.TrimRight(text, "\n"))

	// Skip header: peer <addr> remote as <n> <dir> update <id>
	for i := 0; i < 8 && !s.Done(); i++ {
		s.Next()
	}

	// Key-dispatch loop.
	for !s.Done() {
		raw, ok := s.Next()
		if !ok {
			break
		}
		tok := textparse.ResolveAlias(raw)

		switch tok {
		case textparse.KWSourcePolicyEpoch, textparse.KWSourceReceipt, textparse.KWPeerOwner, textparse.KWConnectionID:
			s.Next() // Provenance was parsed before updating the inventory.
		case textparse.KWNextHop:
			s.Next() // consume the address

		case textparse.KWNLRI:
			famTok, ok := s.Next()
			if !ok || !strings.Contains(famTok, "/") {
				continue
			}
			fam, famOK := family.LookupFamily(famTok)
			if !famOK {
				continue
			}

			var pathID uint32
			var addPath bool
			// Optional path-id modifier.
			next, ok := s.Peek()
			if !ok {
				continue
			}
			if textparse.ResolveAlias(next) == textparse.KWPathInformation {
				s.Next() // consume "info"/"path-information"
				idToken, valid := s.Next()
				if !valid {
					continue
				}
				id, parseErr := strconv.ParseUint(idToken, 10, 32)
				if parseErr != nil {
					continue
				}
				pathID = uint32(id)
				addPath = true
				if _, ok = s.Peek(); !ok {
					continue
				}
			}

			// Action: add or del.
			action, ok := s.Next()
			if !ok || (action != actionAdd && action != actionDel) {
				continue
			}

			// Collect NLRI tokens until next top-level keyword or end.
			var nlriTokens []string
			for !s.Done() {
				next, ok := s.Peek()
				if !ok || textparse.IsTopLevelKeyword(next) {
					break
				}
				tok, _ := s.Next()
				nlriTokens = append(nlriTokens, tok)
			}

			nlris := buildNLRIEntries(nlriTokens)
			if len(nlris) > 0 {
				result[fam] = append(result[fam], familyOperation{action: action, nlris: nlris, pathID: pathID, addPath: addPath})
			}

		// Attribute keywords: consume their values.
		case textparse.KWOrigin, textparse.KWMED, textparse.KWLocalPreference,
			textparse.KWAggregator, textparse.KWOriginatorID, textparse.KWAIGP:
			s.Next()
		case textparse.KWASPath, textparse.KWCommunity, textparse.KWLargeCommunity,
			textparse.KWExtendedCommunity, textparse.KWClusterList:
			s.Next()
		case textparse.KWAtomicAggregate:
			// flag, no value
		}
	}

	return result
}

// buildNLRIEntries splits collected tokens into individual NLRI strings.
// Accepts two formats:
//   - Comma: "prefix 10.0.0.0/24,10.0.1.0/24" -- type keyword + comma-separated values.
//   - Keyword boundary: "prefix 10.0.0.0/24 prefix 10.0.1.0/24" -- repeated type keyword.
func buildNLRIEntries(tokens []string) []string {
	if len(tokens) == 0 {
		return nil
	}

	// Check for comma in any token.
	for i, tok := range tokens {
		if !strings.Contains(tok, ",") {
			continue
		}
		typePrefix := textbuf.Join(tokens[:i], " ")
		var nlris []string
		var tb textbuf.Buffer
		for part := range strings.SplitSeq(tok, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				if typePrefix == "" {
					nlris = append(nlris, part)
				} else {
					nlris = append(nlris, tb.Reset().Str(typePrefix).Byte(' ').Str(part).String())
				}
			}
		}
		return nlris
	}

	// No commas -- check for keyword boundary (repeated type keywords).
	if textparse.NLRITypeKeywords[tokens[0]] {
		var nlris []string
		var current []string
		for _, tok := range tokens {
			if tok == tokens[0] && len(current) > 0 {
				nlris = append(nlris, textbuf.Join(current, " "))
				current = nil
			}
			current = append(current, tok)
		}
		if len(current) > 0 {
			nlris = append(nlris, textbuf.Join(current, " "))
		}
		return nlris
	}

	// Single complex NLRI: join all tokens.
	return []string{textbuf.Join(tokens, " ")}
}
