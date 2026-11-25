// Design: docs/architecture/plugin/rib-storage-design.md -- per-route reflector membership.
// Related: withdrawal.go -- inventory provenance is captured from actual received input.
package rr

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/textparse"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func wireNoLLGR(attrs *attribute.AttributesWire) (bool, error) {
	if attrs == nil {
		return false, nil
	}
	raw, err := attrs.GetRaw(attribute.AttrCommunity)
	if err != nil {
		return false, err
	}
	communities, err := attribute.ParseCommunities(raw)
	if err != nil {
		return false, err
	}
	return communities.Contains(attribute.CommunityNoLLGR), nil
}

// textNoLLGR examines only an actual COMMUNITY value, never metadata, an AS
// path or an extended community. The sender's configured effective text is the
// same post-import event used by Adj-RIB-In.
func textNoLLGR(text string) bool {
	fields := strings.Fields(text)
	for index := 8; index < len(fields); index++ {
		if textparse.ResolveAlias(fields[index]) != textparse.KWCommunity {
			continue
		}
		for next := index + 1; next < len(fields); next++ {
			if textparse.IsTopLevelKeyword(fields[next]) {
				break
			}
			value := strings.Trim(fields[next], "[]")
			for token := range strings.SplitSeq(value, ",") {
				switch token {
				case "no-llgr", "65535:7", "4294901767", "0xffff0007":
					return true
				}
			}
		}
	}
	return false
}

func nativeKeyFor(address string, key withdrawalKey, info withdrawalInfo) (nativeLookupKey, bool) {
	if info.NativeHex == "" {
		return nativeLookupKey{}, false
	}
	return nativeLookupKey{
		address:    address,
		owner:      key.owner,
		connection: key.connection,
		fam:        info.Family,
		pathID:     info.PathID,
		nativeHex:  strings.ToLower(info.NativeHex),
	}, true
}

func (rr *routeReflector) setWithdrawalEntryLocked(address string, key withdrawalKey, info withdrawalInfo) {
	if rr.withdrawals == nil {
		rr.withdrawals = make(map[string]map[withdrawalKey]withdrawalInfo)
	}
	entries := rr.withdrawals[address]
	if entries == nil {
		entries = make(map[withdrawalKey]withdrawalInfo)
		rr.withdrawals[address] = entries
	}
	if rr.routeConn == nil {
		rr.routeConn = make(map[ownerRouteKey]uint64)
	}
	if rr.nativeIndex == nil {
		rr.nativeIndex = make(map[nativeLookupKey]withdrawalKey)
	}
	if rr.sourceCounts == nil {
		rr.sourceCounts = make(map[peerIdentity]int)
	}
	if rr.addressTracked == nil {
		rr.addressTracked = make(map[string]int)
	}
	if old, existed := entries[key]; existed {
		if oldNK, ok := nativeKeyFor(address, key, old); ok {
			if rr.nativeIndex[oldNK] == key {
				delete(rr.nativeIndex, oldNK)
			}
		}
	} else {
		rr.addressTracked[address]++
		if source, err := newPeerIdentity(address, key.owner, key.connection); err == nil {
			rr.sourceCounts[source]++
		}
	}
	entries[key] = info
	rr.routeConn[ownerRouteKey{address: address, owner: key.owner, route: key.route}] = key.connection
	if nk, ok := nativeKeyFor(address, key, info); ok {
		rr.nativeIndex[nk] = key
	}
}

func (rr *routeReflector) deleteWithdrawalLocked(address string, key withdrawalKey) {
	entries := rr.withdrawals[address]
	old, existed := entries[key]
	if !existed {
		return
	}
	delete(entries, key)
	if rr.nativeIndex != nil {
		if oldNK, ok := nativeKeyFor(address, key, old); ok {
			if rr.nativeIndex[oldNK] == key {
				delete(rr.nativeIndex, oldNK)
			}
		}
	}
	if rr.routeConn != nil {
		ork := ownerRouteKey{address: address, owner: key.owner, route: key.route}
		if rr.routeConn[ork] == key.connection {
			delete(rr.routeConn, ork)
		}
	}
	if rr.sourceCounts != nil {
		if source, err := newPeerIdentity(address, key.owner, key.connection); err == nil {
			if next := rr.sourceCounts[source] - 1; next > 0 {
				rr.sourceCounts[source] = next
			} else {
				delete(rr.sourceCounts, source)
			}
		}
	}
	if rr.addressTracked != nil {
		if next := rr.addressTracked[address] - 1; next > 0 {
			rr.addressTracked[address] = next
		} else {
			delete(rr.addressTracked, address)
		}
	}
	if len(entries) == 0 {
		delete(rr.withdrawals, address)
		delete(rr.addressTracked, address)
	}
}

func (rr *routeReflector) sourceMemberLocked(source peerIdentity) bool {
	addr := source.address.String()
	entries := rr.withdrawals[addr]
	if len(entries) == 0 {
		return false
	}
	if rr.addressTracked[addr] == len(entries) {
		return rr.sourceCounts[source] > 0
	}
	for key := range entries {
		if key.owner != source.owner {
			continue
		}
		if key.connection == source.connection {
			return true
		}
	}
	return false
}

func (rr *routeReflector) lookupNativeLocked(address string, owner, connection uint64, famName string, pathID uint32, nlriHex string, exactCase bool) (withdrawalInfo, bool) {
	if nlriHex == "" {
		return withdrawalInfo{}, false
	}
	entries := rr.withdrawals[address]
	if len(entries) == 0 {
		return withdrawalInfo{}, false
	}
	fam, known := family.LookupFamily(famName)
	if !known {
		return withdrawalInfo{}, false
	}
	if rr.addressTracked[address] == len(entries) {
		key, ok := rr.nativeIndex[nativeLookupKey{
			address:    address,
			owner:      owner,
			connection: connection,
			fam:        fam,
			pathID:     pathID,
			nativeHex:  strings.ToLower(nlriHex),
		}]
		if !ok {
			return withdrawalInfo{}, false
		}
		info, present := entries[key]
		if !present {
			return withdrawalInfo{}, false
		}
		if exactCase {
			if info.NativeHex != nlriHex {
				return withdrawalInfo{}, false
			}
		}
		return info, true
	}
	for key, info := range entries {
		if key.owner != owner {
			continue
		}
		if key.connection != connection {
			continue
		}
		if info.Family != fam {
			continue
		}
		if info.PathID != pathID {
			continue
		}
		if exactCase {
			if info.NativeHex == nlriHex {
				return info, true
			}
		} else if strings.EqualFold(info.NativeHex, nlriHex) {
			return info, true
		}
	}
	return withdrawalInfo{}, false
}

// storedMemberLocked requires exact route identity as well as its Session's
// receipt. A retained A keeping that receipt alive never admits removed B.
func (rr *routeReflector) storedMemberLocked(route rpc.StoredRoute) bool {
	if route.SourceOwner == 0 {
		return false
	}
	if route.SourceConnectionID == 0 {
		return false
	}
	if route.SourceReceipt == 0 {
		return false
	}
	info, found := rr.lookupNativeLocked(route.SourcePeer, route.SourceOwner, route.SourceConnectionID, route.Family, route.PathID, route.NLRIHex, false)
	if !found {
		return false
	}
	if info.Closing {
		return false
	}
	if info.SourceReceipt != route.SourceReceipt {
		return false
	}
	return info.MsgID == route.MsgID
}

// putWithdrawalLocked orders only the same original owner's current path.
// A new physical connection replaces its older retained row. Caller MUST hold
// withdrawalMu; no lock crosses I/O and no completed connection history exists.
func (rr *routeReflector) putWithdrawalLocked(address string, key withdrawalKey, info withdrawalInfo) {
	entries := rr.withdrawals[address]
	if entries == nil {
		entries = make(map[withdrawalKey]withdrawalInfo)
		rr.withdrawals[address] = entries
	}
	if rr.addressTracked[address] == len(entries) {
		ork := ownerRouteKey{address: address, owner: key.owner, route: key.route}
		if priorConn, exists := rr.routeConn[ork]; exists {
			prior := withdrawalKey{owner: key.owner, connection: priorConn, route: key.route}
			if held, present := entries[prior]; present {
				if priorConn > key.connection {
					return
				}
				if priorConn == key.connection {
					if held.Closing {
						return
					}
					if held.MsgID > info.MsgID {
						return
					}
					rr.setWithdrawalEntryLocked(address, key, info)
					return
				}
				rr.deleteWithdrawalLocked(address, prior)
				rr.enqueueSupersededLocked(address, key.owner, priorConn)
			}
		}
		rr.setWithdrawalEntryLocked(address, key, info)
		return
	}
	for prior, held := range entries {
		if prior.owner != key.owner {
			continue
		}
		if prior.route != key.route {
			continue
		}
		if prior.connection > key.connection {
			return
		}
		if prior.connection == key.connection {
			if held.Closing {
				return
			}
			if held.MsgID > info.MsgID {
				return
			}
		}
		delete(entries, prior)
		if prior.connection < key.connection {
			rr.enqueueSupersededLocked(address, prior.owner, prior.connection)
		}
	}
	entries[key] = info
}

func (rr *routeReflector) enqueueSupersededLocked(address string, owner, connection uint64) {
	source, err := newPeerIdentity(address, owner, connection)
	if err != nil {
		rr.noteWorkFailure("superseded source inventory", err)
		return
	}
	rr.oweCleanupLocked(source)
	// The existing departure lane owns prompt release of the last old row;
	// this empty prune carries no new destination write and no history.
	if !rr.enqueueDeparture(source, make(map[withdrawalKey]withdrawalInfo)) {
		rr.noteWorkFailure("superseded source release", errors.New("departure worker stopped before old lease cleanup"))
	}
}

func (rr *routeReflector) removeWithdrawalLocked(address string, key withdrawalKey, msgID uint64) {
	entries := rr.withdrawals[address]
	held, present := entries[key]
	if !present {
		return
	}
	if held.MsgID != 0 {
		if msgID != 0 {
			if held.MsgID > msgID {
				return
			}
		}
	}
	rr.deleteWithdrawalLocked(address, key)
}

// currentStoredRoute asks the existing declared receive-store command even
// when optional validation is disabled. A stale page must prove the current
// original ingress, not merely that its Session receipt remains retained.
// storedEligibility separates a current rejection from an original receive
// projection that has not applied the caller's minimum UPDATE frontier.
type storedEligibility uint8

const (
	storedEligibilityUnspecified storedEligibility = iota
	storedEligibilityRejected
	storedEligibilityAccepted
	storedEligibilityPending
)

func (rr *routeReflector) currentStoredRoute(ctx context.Context, route ribevents.ValidationRoute, msgID, afterMsgID uint64) (storedEligibility, error) {
	rr.mu.RLock()
	command := rr.receiveGate
	rr.mu.RUnlock()
	if command == "" {
		return storedEligibilityUnspecified, errors.New("bgp-rr: current receive-store eligibility command has not been announced")
	}
	body, err := json.Marshal(ribevents.EligibilityRequest{Queries: []ribevents.EligibilityQuery{{Route: route, MsgID: msgID, AfterMsgID: afterMsgID}}})
	if err != nil {
		return storedEligibilityUnspecified, err
	}
	status, data, err := rr.dispatchCommand(ctx, command, string(body))
	if err != nil {
		return storedEligibilityUnspecified, err
	}
	if status != statusDone {
		return storedEligibilityUnspecified, errors.New("bgp-rr: current eligibility command did not complete")
	}
	var answer ribevents.EligibilityAnswer
	if err := json.Unmarshal(data, &answer); err != nil {
		return storedEligibilityUnspecified, err
	}
	if answer.Pending {
		if len(answer.Answers) != 0 {
			return storedEligibilityUnspecified, errors.New("bgp-rr: pending eligibility answer also claims a route verdict")
		}
		return storedEligibilityPending, nil
	}
	if len(answer.Answers) != 1 {
		return storedEligibilityUnspecified, errors.New("bgp-rr: current eligibility answer does not match its one route")
	}
	if answer.Answers[0] {
		return storedEligibilityAccepted, nil
	}
	return storedEligibilityRejected, nil
}

// inventoryRecord is one fully parsed native inventory mutation. It carries
// only owned strings; parsing a whole UPDATE precedes every mutation.
type inventoryRecord struct {
	key      withdrawalKey
	info     withdrawalInfo
	withdraw bool
}

func nativeInventory(info withdrawalInfo, raw []byte, withdraw bool) (inventoryRecord, error) {
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	key, err := nlrisplit.GetPrefixKey(info.Family)(raw, scratch[:], withdraw)
	if err != nil {
		return inventoryRecord{}, err
	}
	info.NativeHex = hex.EncodeToString(raw)
	info.WireForm = true
	info.Prefix = info.NativeHex
	if info.AddPath {
		info.Prefix = addPathUnicastHex(info.PathID, raw)
	}
	route := info.Family.String() + "|" + strconv.FormatUint(uint64(info.PathID), 10) + "|" + hex.EncodeToString(key)
	return inventoryRecord{key: withdrawalKey{owner: info.SourceOwner, connection: info.ConnectionID, route: route}, info: info, withdraw: withdraw}, nil
}

func (rr *routeReflector) applyInventoryLocked(address string, record inventoryRecord) {
	if !record.withdraw {
		rr.putWithdrawalLocked(address, record.key, record.info)
		return
	}
	entries := rr.withdrawals[address]
	// A withdrawn labeled NLRI was normalized at parsing, so its semantic
	// key matches the exact withdrawalKey directly without scanning.
	if info, present := entries[record.key]; present {
		if info.MsgID <= record.info.MsgID {
			rr.deleteWithdrawalLocked(address, record.key)
		}
	}
}

func storedInventory(route rpc.StoredRoute) (withdrawalKey, withdrawalInfo, ribevents.ValidationRoute, error) {
	fam, known := family.LookupFamily(route.Family)
	if !known {
		return withdrawalKey{}, withdrawalInfo{}, ribevents.ValidationRoute{}, errors.New("bgp-rr: replay names an unknown family")
	}
	address, err := netip.ParseAddr(route.SourcePeer)
	if err != nil {
		return withdrawalKey{}, withdrawalInfo{}, ribevents.ValidationRoute{}, err
	}
	raw, err := hex.DecodeString(route.NLRIHex)
	if err != nil {
		return withdrawalKey{}, withdrawalInfo{}, ribevents.ValidationRoute{}, err
	}
	pathID := route.PathID
	switch route.NLRIFraming {
	case rpc.NLRIFramingPrefixOnly:
	case rpc.NLRIFramingSourceWire:
		return withdrawalKey{}, withdrawalInfo{}, ribevents.ValidationRoute{}, errors.New("bgp-rr: stored replay must carry explicit prefix-only framing")
	default:
		return withdrawalKey{}, withdrawalInfo{}, ribevents.ValidationRoute{}, errors.New("bgp-rr: stored replay has no explicit NLRI framing")
	}
	split := nlrisplit.Get(fam)
	if route.Withdraw {
		split = nlrisplit.GetWithdraw(fam)
	}
	if split == nil {
		return withdrawalKey{}, withdrawalInfo{}, ribevents.ValidationRoute{}, nlrisplit.ErrUnsupported
	}
	count, err := split(raw, false, nil)
	if err != nil {
		return withdrawalKey{}, withdrawalInfo{}, ribevents.ValidationRoute{}, err
	}
	if count != 1 {
		return withdrawalKey{}, withdrawalInfo{}, ribevents.ValidationRoute{}, errors.New("bgp-rr: replay row must contain exactly one native route")
	}
	attrs, err := hex.DecodeString(route.AttrHex)
	if err != nil {
		return withdrawalKey{}, withdrawalInfo{}, ribevents.ValidationRoute{}, err
	}
	noLLGR, err := wireNoLLGR(attribute.NewAttributesWire(attrs, 0))
	if err != nil {
		return withdrawalKey{}, withdrawalInfo{}, ribevents.ValidationRoute{}, err
	}
	info := withdrawalInfo{Family: fam, PathID: pathID, AddPath: true, SourceOwner: route.SourceOwner, ConnectionID: route.SourceConnectionID,
		SourceReceipt: route.SourceReceipt, MsgID: route.MsgID, SourcePolicyEpoch: route.SourcePolicyEpoch, NoLLGR: noLLGR}
	record, err := nativeInventory(info, raw, route.Withdraw)
	if err != nil {
		return withdrawalKey{}, withdrawalInfo{}, ribevents.ValidationRoute{}, err
	}
	validation := ribevents.ValidationRoute{Peer: address, PeerOwner: route.SourceOwner, ConnectionID: route.SourceConnectionID, Family: fam, PathID: pathID}
	if isUnicast(fam) {
		parsed, rest, err := nlri.ParseINET(fam.AFI, fam.SAFI, raw, false)
		if err != nil {
			return withdrawalKey{}, withdrawalInfo{}, ribevents.ValidationRoute{}, err
		}
		if len(rest) != 0 {
			return withdrawalKey{}, withdrawalInfo{}, ribevents.ValidationRoute{}, errors.New("bgp-rr: replay has trailing NLRI")
		}
		validation.Prefix, err = netip.ParsePrefix(parsed.String())
		if err != nil {
			return withdrawalKey{}, withdrawalInfo{}, ribevents.ValidationRoute{}, err
		}
	} else if nlrisplit.KeysByCIDR(fam) {
		var scratch [nlrisplit.PrefixKeyScratchSize]byte
		cidr, err := nlrisplit.RouteCIDR(fam, raw, scratch[:], route.Withdraw)
		if err != nil {
			return withdrawalKey{}, withdrawalInfo{}, ribevents.ValidationRoute{}, err
		}
		validation.Prefix, err = netip.ParsePrefix(prefixBytesToKey(cidr, fam.AFI == family.AFIIPv6))
		if err != nil {
			return withdrawalKey{}, withdrawalInfo{}, ribevents.ValidationRoute{}, err
		}
	} else {
		validation.NLRI = string(raw)
	}
	return record.key, record.info, validation, nil
}

func (rr *routeReflector) textRecords(ctx context.Context, ops map[family.Family][]familyOperation, epoch, receipt, owner, connection, msgID uint64, noLLGR bool) ([]inventoryRecord, error) {
	records := []inventoryRecord{}
	// A mixed withdrawal/announcement preserves RFC 4271 ordering independent
	// of how family sections happened to be printed in a native TEXT event.
	for _, action := range []string{actionDel, actionAdd} {
		for fam, operations := range ops {
			for _, op := range operations {
				if op.action != action {
					continue
				}
				for _, text := range op.nlris {
					info := withdrawalInfo{Family: fam, PathID: op.pathID, AddPath: op.addPath, Prefix: text, SourceOwner: owner, ConnectionID: connection,
						SourceReceipt: receipt, MsgID: msgID, SourcePolicyEpoch: epoch, NoLLGR: noLLGR}
					prefixInfo := withTextNative(info)
					var raw []byte
					if isUnicast(fam) {
						var err error
						raw, err = hex.DecodeString(prefixInfo.NativeHex)
						if err != nil {
							return nil, err
						}
						if len(raw) == 0 {
							return nil, errors.New("bgp-rr: native TEXT unicast has no valid prefix")
						}
					} else {
						tokens := strings.Fields(text)
						encoded := ""
						if len(tokens) == 2 {
							if tokens[0] == "raw" {
								encoded = tokens[1]
							}
						}
						if encoded == "" {
							var err error
							encoded, err = rr.plugin.EncodeNLRI(ctx, fam.String(), tokens)
							if err != nil {
								return nil, err
							}
						}
						var err error
						raw, err = hex.DecodeString(encoded)
						if err != nil {
							return nil, err
						}
					}
					split := nlrisplit.Get(fam)
					if action == actionDel {
						split = nlrisplit.GetWithdraw(fam)
					}
					if split == nil {
						return nil, nlrisplit.ErrUnsupported
					}
					count, err := split(raw, false, nil)
					if err != nil {
						return nil, err
					}
					if count != 1 {
						return nil, errors.New("bgp-rr: native TEXT section entry is not one complete route")
					}
					record, err := nativeInventory(info, raw, action == actionDel)
					if err != nil {
						return nil, err
					}
					records = append(records, record)
				}
			}
		}
	}
	return records, nil
}

func textRouteKey(family, prefix string, pathID uint32, addPath bool) string {
	var buf textbuf.Buffer
	buf.Str(family).Byte('|').Str(nlriKey(prefix))
	if addPath {
		buf.Str("|addpath|").Uint32(pathID)
	}
	return buf.String()
}

func withTextNative(info withdrawalInfo) withdrawalInfo {
	prefix, err := netip.ParsePrefix(nlriKey(info.Prefix))
	if err != nil {
		return info
	}
	var native [17]byte
	native[0] = byte(prefix.Bits())
	count := (prefix.Bits() + 7) / 8
	copy(native[1:], prefix.Masked().Addr().AsSlice()[:count])
	info.NativeHex = hex.EncodeToString(native[:count+1])
	if info.AddPath {
		info.Prefix = addPathUnicastHex(info.PathID, native[:count+1])
		info.WireForm = true
	}
	return info
}
