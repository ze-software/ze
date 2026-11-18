// Design: docs/architecture/plugin/rib-storage-design.md -- current retained RS membership.
// Related: target_snapshot.go -- recheck at the actual bound relay capture.
package rs

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/ze-software/ze/internal/component/bgp/textparse"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"net/netip"
	"strings"
)

func wireNoLLGR(attrs *attribute.AttributesWire) bool {
	if attrs == nil {
		return false
	}
	raw, err := attrs.GetRaw(attribute.AttrCommunity)
	if err != nil {
		return false
	}
	for off := 0; off+4 <= len(raw); off += 4 {
		if attribute.Community(binary.BigEndian.Uint32(raw[off:])) == attribute.CommunityNoLLGR {
			return true
		}
	}
	return false
}

// The complete text event renders the standard community as a comma list.
// Only the community attribute token can supply this accepted property.
func textNoLLGR(text string) bool {
	scanner := textparse.NewScanner(text)
	for range 8 {
		scanner.Next()
	}
	for !scanner.Done() {
		token, _ := scanner.Next()
		if textparse.ResolveAlias(token) != textparse.KWCommunity {
			continue
		}
		values, _ := scanner.Next()
		for _, value := range strings.Split(values, ",") {
			if value == "65535:7" || value == "no-llgr" {
				return true
			}
		}
	}
	return false
}

// stampCurrentRowsLocked stores accepted scalar route facts in the existing
// current inventory. It never stores a session-wide eligibility substitute.
func (rs *routeServer) stampCurrentRowsLocked(source workerKey, message, receipt uint64, noLLGR bool) {
	for native, entry := range rs.withdrawals[source.address.String()] {
		if entry.owner != source.owner {
			continue
		}
		if entry.connectionID != source.connectionID {
			continue
		}
		if entry.sourceReceipt != receipt {
			continue
		}
		// Only rows changed by this UPDATE get its generation. Older rows retain
		// their own community value even though they share this source receipt.
		if entry.msgID != 0 {
			continue
		}
		entry.msgID = message
		entry.noLLGR = noLLGR
		rs.withdrawals[source.address.String()][native] = entry
	}
}

func textInventoryEntry(fam family.Family, text string, pathID uint32, addPath, withdraw bool, epoch, receipt, owner, connection uint64) (withdrawalKey, withdrawalEntry, error) {
	entry := withdrawalEntry{owner: owner, connectionID: connection, sourcePolicyEpoch: epoch, sourceReceipt: receipt}
	if payload, raw := strings.CutPrefix(text, "raw "); raw {
		route := rpc.StoredRoute{Family: fam.String(), SourceOwner: owner, NLRIHex: payload, PathID: pathID, NLRIFraming: rpc.NLRIFramingPrefixOnly, Withdraw: withdraw}
		native, err := storedKey(route, false)
		if err != nil {
			return withdrawalKey{}, entry, err
		}
		native.addPath = addPath
		wire, err := hex.DecodeString(payload)
		if err != nil {
			return withdrawalKey{}, entry, err
		}
		if addPath {
			framed := make([]byte, 4+len(wire))
			binary.BigEndian.PutUint32(framed, pathID)
			copy(framed[4:], wire)
			wire = framed
		}
		entry.wire = hex.EncodeToString(wire)
		entry.addPath = addPath
		return native, entry, nil
	}
	text = strings.TrimPrefix(text, "prefix ")
	prefix, err := netip.ParsePrefix(text)
	if err != nil {
		return withdrawalKey{}, entry, err
	}
	return withdrawalKey{owner: owner, fam: fam, prefix: prefix.Masked(), pathID: pathID, addPath: addPath}, entry, nil
}

func sameNativeMembership(a, b withdrawalKey) bool {
	if a.owner != b.owner || a.fam != b.fam || a.pathID != b.pathID {
		return false
	}
	if a.prefix.IsValid() || b.prefix.IsValid() {
		return a.prefix == b.prefix
	}
	return a.nlriStr == b.nlriStr
}

// storedKey uses registered native key and framing functions. source-wire is
// admitted only with this original actor's recorded ADD-PATH framing; absence
// never guesses from the current address or treats identifier zero as absent.
func storedKey(route rpc.StoredRoute, addPath bool) (withdrawalKey, error) {
	fam, known := family.LookupFamily(route.Family)
	if !known {
		return withdrawalKey{}, errors.New("route-server stored route has unknown family")
	}
	raw, err := hex.DecodeString(route.NLRIHex)
	if err != nil {
		return withdrawalKey{}, err
	}
	pathID := route.PathID
	switch route.NLRIFraming {
	case rpc.NLRIFramingPrefixOnly:
	case rpc.NLRIFramingSourceWire:
		pathID, raw, err = nlri.SplitPathID(raw, addPath)
		if err != nil {
			return withdrawalKey{}, err
		}
	default:
		return withdrawalKey{}, errors.New("route-server stored route has no original framing")
	}
	walk := nlrisplit.Get(fam)
	if route.Withdraw {
		walk = nlrisplit.GetWithdraw(fam)
	}
	if walk == nil {
		return withdrawalKey{}, errors.New("route-server stored route has no registered native framer")
	}
	count, err := walk(raw, false, nil)
	if err != nil {
		return withdrawalKey{}, err
	}
	if count != 1 {
		return withdrawalKey{}, errors.New("route-server membership requires one original native route")
	}
	key := withdrawalKey{owner: route.SourceOwner, fam: fam, pathID: pathID}
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	if isUnicast(fam) || fam == family.IPv4Multicast || fam == family.IPv6Multicast || nlrisplit.KeysByCIDR(fam) {
		cidr := raw
		if nlrisplit.KeysByCIDR(fam) {
			cidr, err = nlrisplit.RouteCIDR(fam, raw, scratch[:], route.Withdraw)
		}
		if err != nil {
			return withdrawalKey{}, err
		}
		prefix, ok := nlri.WirePrefixToKey(cidr, fam)
		if !ok {
			return withdrawalKey{}, errors.New("route-server stored route has invalid CIDR identity")
		}
		key.prefix = prefix
		return key, nil
	}
	{
		native, err := nlrisplit.GetPrefixKey(fam)(raw, scratch[:], route.Withdraw)
		if err != nil {
			return withdrawalKey{}, err
		}
		key.nlriStr = string(native)
		key.wireForm = true
		return key, nil
	}

}

// currentStoredRouteLocked is called with withdrawalMu. A SourceReceipt proves
// source facts only; the exact original physical route must still be current.
// A retained A cannot authorize removed B from the same receipt.
func (rs *routeServer) currentStoredRouteLocked(route rpc.StoredRoute) bool {
	source, err := newWorkerKey(route.SourcePeer, route.SourceOwner, route.SourceConnectionID)
	if err != nil {
		return false
	}
	rs.mu.RLock()
	actor := rs.peers[source]
	if actor == nil {
		rs.mu.RUnlock()
		return false
	}
	if !actor.sourceLease || actor.sourceReceipt != route.SourceReceipt {
		rs.mu.RUnlock()
		return false
	}
	scopes := actor.cleanup
	up, pending, sourceRetained, keep := actor.Up, actor.downPending, actor.sourceRetained, actor.retainedFamilies
	rs.mu.RUnlock()
	entries := rs.withdrawals[source.address.String()]
	for native, entry := range entries {
		if entry.owner != source.owner || entry.connectionID != source.connectionID {
			continue
		}
		key, err := storedKey(route, entry.addPath)
		if err != nil {
			continue
		}
		if !sameNativeMembership(native, key) {
			continue
		}
		if !up && !entry.retained {
			if !pending || !sourceRetained || !keep[native.fam] {
				return false
			}
		}
		if entry.sourceReceipt != route.SourceReceipt {
			return false
		}
		if entry.sourcePolicyEpoch != route.SourcePolicyEpoch {
			return false
		}
		if entry.msgID != route.MsgID {
			return false
		}
		for _, scope := range scopes {
			if cleanupMatches(scope, native, entry) {
				return false
			}
		}
		return true
	}
	return false
}

// sourceReservation is one captured actor's pending cold admission. Only
// prepareStoredSources produces it. Its zero value owns no reservation.
type sourceReservation struct {
	source workerKey
	actor  *PeerState
}

// prepareStoredSources acquires only from a still-live original captured actor.
// It runs outside withdrawalMu, so a held engine call cannot block the finite
// cleanup handler. The admission decision is rechecked under that lock below.
// After success, the caller MUST call finishStoredReservations exactly once
// after its final adoption/output attempt, including an empty or refused page.
func (rs *routeServer) prepareStoredSources(ctx context.Context, routes []rpc.StoredRoute) ([]sourceReservation, error) {
	var reservations []sourceReservation
	seen := make(map[workerKey]bool)
	for _, route := range routes {
		if route.Withdraw {
			continue
		}
		source, err := newWorkerKey(route.SourcePeer, route.SourceOwner, route.SourceConnectionID)
		if err != nil {
			rs.warnLifecycle("stored route has no original physical source", "code", lifecycleDoctorCode, "fix", lifecycleDoctorFix, "error", err)
			continue
		}
		if seen[source] {
			continue
		}
		seen[source] = true
		rs.mu.RLock()
		actor := rs.peers[source]
		rs.mu.RUnlock()
		if actor == nil {
			continue
		}
		actor.leaseMu.Lock()
		rs.mu.RLock()
		adopt := actor.Up && actor.adoptionOpen
		rs.mu.RUnlock()
		if adopt {
			err = rs.acquireSourceLocked(source, route.SourceReceipt)
			if err == nil {
				rs.mu.Lock()
				actor.pendingAdoptions++
				rs.mu.Unlock()
				reservations = append(reservations, sourceReservation{source: source, actor: actor})
			} else {
				rs.warnLifecycle("stored route source lease was not acquired", "code", lifecycleDoctorCode, "fix", lifecycleDoctorFix, "error", err, "source-owner", source.owner, "source-connection", source.connectionID)
			}
		}
		actor.leaseMu.Unlock()
		if err := ctx.Err(); err != nil {
			releaseErr := rs.finishStoredReservations(ctx, reservations)
			return nil, errors.Join(err, releaseErr)
		}
	}
	return reservations, nil
}

// finishStoredReservations MUST pair every successful prepareStoredSources.
// It releases the same actor's zero-row lease after its last reservation; a
// stale page grants no reason to keep an empty source. RPCs hold no map lock.
func (rs *routeServer) finishStoredReservations(ctx context.Context, reservations []sourceReservation) error {
	rs.mu.Lock()
	for _, reservation := range reservations {
		if reservation.actor == nil {
			continue
		}
		if reservation.actor.pendingAdoptions != 0 {
			reservation.actor.pendingAdoptions--
		}
	}
	rs.mu.Unlock()
	var result error
	for _, reservation := range reservations {
		if reservation.actor == nil {
			continue
		}
		result = errors.Join(result, rs.releaseEmptySource(ctx, reservation.source))
	}
	return result
}

var errColdRevisionChanged = errors.New("route-server original inventory changed during cold receive lookup")

// coldRouteCheck is one query of one existing live actor revision. It is
// valid only while withdrawalMu proves that exact revision is still current.
type coldRouteCheck struct {
	actor    *PeerState
	source   workerKey
	revision uint64
}

func (rs *routeServer) noteGateCommands(text string) bool {
	flow, matched, err := ribevents.DecodeFlowSpecGate(text)
	if matched {
		if err != nil {
			rs.warnLifecycle("bgp-rs: flowspec-gate refused", "code", lifecycleDoctorCode, "fix", lifecycleDoctorFix, "err", err)
			return true
		}
		rs.mu.Lock()
		rs.flowSpecGate = flow.Command
		rs.mu.Unlock()
		ribevents.NoteFlowSpecGate(flow)
		return true
	}
	gate, matched, err := ribevents.DecodeValidationGate(text)
	if !matched {
		return false
	}
	if err != nil {
		rs.warnLifecycle("receive-store gate announcement refused", "code", lifecycleDoctorCode, "fix", lifecycleDoctorFix, "error", err)
		return true
	}
	rs.mu.Lock()
	rs.receiveGate = gate.Command
	rs.mu.Unlock()
	ribevents.NoteValidationGate(gate)
	return true
}

// checkColdRoutes MUST run outside withdrawalMu. Its caller MUST recheck every
// returned actor revision under that lock before adoption/output enrollment.
// This asks the current store even when optional validation is disabled.
func (rs *routeServer) checkColdRoutes(ctx context.Context, routes []rpc.StoredRoute) (map[int]coldRouteCheck, error) {
	checked := make(map[int]coldRouteCheck)
	for index, route := range routes {
		if route.Withdraw {
			continue
		}
		source, err := newWorkerKey(route.SourcePeer, route.SourceOwner, route.SourceConnectionID)
		if err != nil {
			return nil, err
		}
		rs.withdrawalMu.Lock()
		current := rs.currentStoredRouteLocked(route)
		rs.mu.RLock()
		actor := rs.peers[source]
		command := rs.receiveGate
		flowCommand := rs.flowSpecGate
		live := actor != nil && actor.Up && actor.adoptionOpen && actor.sourceLease
		var revision, processed uint64
		if live {
			revision, processed = actor.inventoryRevision, actor.processedMsgID
		}
		rs.mu.RUnlock()
		rs.withdrawalMu.Unlock()
		if current {
			continue
		}
		if !live {
			continue
		}
		if _, bounded := ctx.Deadline(); !bounded {
			return nil, errors.New("route-server cold receive lookup requires a finite replay deadline")
		}
		if route.NLRIFraming != rpc.NLRIFramingPrefixOnly {
			return nil, errors.New("route-server cold replay requires the original prefix-only framing")
		}
		native, err := storedKey(route, false)
		if err != nil {
			return nil, err
		}
		raw, err := hex.DecodeString(route.NLRIHex)
		if err != nil {
			return nil, err
		}
		query := ribevents.ValidationRoute{Peer: source.address, PeerOwner: source.owner, ConnectionID: source.connectionID, Family: native.fam, Prefix: native.prefix, PathID: native.pathID, NLRI: string(raw)}
		if ribevents.IsFlowSpec(native.fam) {
			eligible, err := rs.currentColdFlowSpec(ctx, flowCommand, query, route, processed)
			if err != nil {
				return nil, err
			}
			if eligible {
				checked[index] = coldRouteCheck{actor: actor, source: source, revision: revision}
			}
			continue
		}
		if command == "" {
			return nil, errors.New("route-server current receive-store command has not been announced")
		}
		request := ribevents.EligibilityRequest{Queries: []ribevents.EligibilityQuery{{Route: query, MsgID: route.MsgID, AfterMsgID: processed}}}
		payload, err := json.Marshal(request)
		if err != nil {
			return nil, err
		}
		var answer ribevents.EligibilityAnswer
		// The finite replay context bounds catch-up. Pending is structured unknown,
		// not a verdict: neither a stale page nor another source's progress can win.
		for {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			status, data, err := rs.dispatchCommand(ctx, command, string(payload))
			if err != nil {
				return nil, err
			}
			if status != statusDone {
				return nil, errors.New("route-server current receive-store query did not complete")
			}
			answer = ribevents.EligibilityAnswer{}
			if err := json.Unmarshal(data, &answer); err != nil {
				return nil, err
			}
			if !answer.Pending {
				break
			}
			if len(answer.Answers) != 0 {
				return nil, errors.New("route-server pending eligibility query also carried a route verdict")
			}
		}
		if len(answer.Answers) != 1 {
			return nil, errors.New("route-server eligibility answer does not match its one original path")
		}
		if answer.Answers[0] {
			checked[index] = coldRouteCheck{actor: actor, source: source, revision: revision}
		}
	}
	return checked, nil
}

// currentColdFlowSpec asks the mandatory selecting RIB rather than the optional
// receive store, which holds no FlowSpec route. Both stores use the caller's
// finite deadline and original-source progress before issuing a current answer.
func (rs *routeServer) currentColdFlowSpec(ctx context.Context, command string, key ribevents.ValidationRoute, route rpc.StoredRoute, processed uint64) (bool, error) {
	if command == "" {
		return false, errors.New("route-server mandatory FlowSpec command has not been announced")
	}
	request := ribevents.FlowSpecRequest{Queries: []ribevents.FlowSpecQuery{{Operation: ribevents.FlowSpecPathOf, Route: key, MsgID: route.MsgID, AfterMsgID: processed}}}
	payload, err := json.Marshal(request)
	if err != nil {
		return false, err
	}
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		status, data, err := rs.dispatchCommand(ctx, command, string(payload))
		if err != nil {
			return false, err
		}
		if status != statusDone {
			return false, errors.New("route-server mandatory FlowSpec query did not complete")
		}
		var answer ribevents.FlowSpecAnswer
		if err := json.Unmarshal(data, &answer); err != nil {
			return false, err
		}
		if answer.Pending {
			if len(answer.Results) != 0 {
				return false, errors.New("route-server pending FlowSpec query also carried a route verdict")
			}
			if len(answer.Routes) != 0 {
				return false, errors.New("route-server pending FlowSpec query also carried routes")
			}
			continue
		}
		if len(answer.Results) != 1 {
			return false, errors.New("route-server FlowSpec answer does not match its one original path")
		}
		result := answer.Results[0]
		if !result.Verdict {
			return false, nil
		}
		if result.Path.MsgID != route.MsgID {
			return false, nil
		}
		if result.Path.SourceReceipt != route.SourceReceipt {
			return false, nil
		}
		if result.Path.SourcePolicyEpoch != route.SourcePolicyEpoch {
			return false, nil
		}
		return true, nil
	}
}

// adoptStoredRouteLocked admits an actual received-store PrefixOnly row while
// its original UP is live and this actor's cold admission is still open. B6's
// authenticated native producer splits and strips source framing before this
// page; ownerless/manual SourceWire data never enters this branch.
func (rs *routeServer) adoptStoredRouteLocked(source workerKey, route rpc.StoredRoute) bool {
	if route.NLRIFraming != rpc.NLRIFramingPrefixOnly || route.MsgID == 0 {
		return false
	}
	rs.mu.RLock()
	actor := rs.peers[source]
	admitted := actor != nil && actor.Up && actor.adoptionOpen && actor.sourceLease && actor.sourceReceipt == route.SourceReceipt
	rs.mu.RUnlock()
	if !admitted {
		return false
	}
	native, err := storedKey(route, false)
	if err != nil {
		return false
	}
	attrs, err := hex.DecodeString(route.AttrHex)
	if err != nil {
		return false
	}
	// Command framing is normalized explicitly: every native withdrawal carries
	// the original scalar identifier including zero, independent of whether the
	// source context negotiated it. This flag describes these command bytes.
	raw, err := hex.DecodeString(route.NLRIHex)
	if err != nil {
		return false
	}
	framed := make([]byte, 4+len(raw))
	binary.BigEndian.PutUint32(framed, route.PathID)
	copy(framed[4:], raw)
	entry := withdrawalEntry{owner: source.owner, connectionID: source.connectionID, sourceReceipt: route.SourceReceipt, sourcePolicyEpoch: route.SourcePolicyEpoch, msgID: route.MsgID, noLLGR: wireNoLLGR(attribute.NewAttributesWire(attrs, 0)), wire: hex.EncodeToString(framed), addPath: true}
	if rs.withdrawals[source.address.String()] == nil {
		rs.withdrawals[source.address.String()] = make(map[withdrawalKey]withdrawalEntry)
	}
	rs.withdrawals[source.address.String()][native] = entry
	return true
}

func (rs *routeServer) admitStoredRoutesLocked(ctx context.Context, routes []rpc.StoredRoute, checked map[int]coldRouteCheck) ([]rpc.StoredRoute, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Validate the whole copied page before putting any cold membership. A
	// concurrent UPDATE is safe to retry because no output was admitted yet.
	for _, check := range checked {
		rs.mu.RLock()
		same := rs.peers[check.source] == check.actor
		rs.mu.RUnlock()
		if !same {
			return nil, errColdRevisionChanged
		}
		if check.actor.inventoryRevision != check.revision {
			return nil, errColdRevisionChanged
		}
	}
	selected := make([]rpc.StoredRoute, 0, len(routes))
	for index, route := range routes {
		if route.Withdraw {
			if route.SourceOwner == 0 || route.SourceConnectionID == 0 {
				rs.warnLifecycle("stored withdrawal has no original source actor", "code", lifecycleDoctorCode, "fix", lifecycleDoctorFix)
				continue
			}
			selected = append(selected, route)
			continue
		}
		if rs.currentStoredRouteLocked(route) {
			selected = append(selected, route)
			continue
		}
		check, queried := checked[index]
		if !queried {
			continue
		}
		if rs.adoptStoredRouteLocked(check.source, route) {
			selected = append(selected, route)
		}
	}
	return selected, nil
}
