// Design: docs/architecture/plugin/rib-storage-design.md -- source-owned DOWN recovery.
package rib

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/store"
)

// recoveryRoutes answers every requested key with exactly one outcome. An
// excluded source is skipped, never deleted here: an asynchronous old DOWN
// MUST NOT remove a reconnected session.
// RFC 4271 Section 6: "The local system recalculates its best routes for the
// destinations of the routes marked as invalid."
// The existing election therefore supplies each surviving replacement.
func (r *RIBManager) recoveryRoutes(request ribevents.RecoveryRequest) ([]ribevents.RecoveryOutcome, error) {
	identities := make([]ribOutKey, len(request.Keys))
	for i := range request.Keys {
		identity, err := recoveryIdentity(request.Family, request.Keys[i].NLRI, request.AddPath)
		if err != nil {
			return nil, err
		}
		identities[i] = identity
	}
	r.peerMu.RLock()
	defer r.peerMu.RUnlock()

	outcomes := make([]ribevents.RecoveryOutcome, len(request.Keys))
	for i := range request.Keys {
		outcomes[i].NLRI = request.Keys[i].NLRI
	}
	if request.SentAddPath && request.Limit == 0 {
		// RFC 7911 Section 3: ADD-PATH already advertised the surviving paths,
		// so no election runs; each excluded source's sent identifiers,
		// including zero, are withdrawn. A limited request elects below.
		if err := recoverySentAddPathLocked(&request, identities, r.ribOut[request.Destination][request.Family], outcomes); err != nil {
			return nil, err
		}
		return outcomes, nil
	}
	for i := range request.Keys {
		key := &request.Keys[i]
		outcome := &outcomes[i]
		survivors, err := r.recoverySurvivorsLocked(&request, key, identities[i])
		if err != nil {
			return nil, err
		}
		outcome.Survivors = survivors
		outcome.Withdraw = len(survivors) == 0
	}
	return outcomes, nil
}

// recoverySentAddPathLocked answers every key WITHDRAW with the sent
// identifiers an excluded source owns at the destination for the key's prefix.
// It scans the destination's sent family once for the whole request rather
// than once per key. An identifier the departed source's reconnected session
// advertised is named too: the RIB holds no session generation, and the
// reactor's writer withdraws only an identifier no current session owns.
// A sent identity that cannot be written back as a withdrawal fails the whole
// request: the reactor treats a failed lookup as no answer, never as "nothing
// owned". The caller MUST hold peerMu.
func recoverySentAddPathLocked(request *ribevents.RecoveryRequest, identities []ribOutKey, entries map[ribOutKey]ribOutEntry, outcomes []ribevents.RecoveryOutcome) error {
	keysByIdentity := make(map[ribOutKey][]int, len(identities))
	for i, identity := range identities {
		outcomes[i].Withdraw = true
		keysByIdentity[identity] = append(keysByIdentity[identity], i)
	}
	for sentKey, entry := range entries {
		match := sentKey
		match.PathID = 0
		indexes, affected := keysByIdentity[match]
		if !affected {
			continue
		}
		if !entry.AddPath {
			continue
		}
		var sent []byte
		for _, i := range indexes {
			if !recoveryExcludes(&request.Keys[i], entry.SourcePeer) {
				continue
			}
			if sent == nil {
				// RFC 7911 Section 3: withdrawal names the advertised identifier.
				var err error
				if sent, err = recoverySentNLRI(request.Family, sentKey, entry); err != nil {
					return err
				}
			}
			outcomes[i].Sent = append(outcomes[i].Sent, sent)
		}
	}
	return nil
}

// recoveryExcludes reports whether the sent owner is in the key's excluded set.
func recoveryExcludes(key *ribevents.RecoveryKey, sourcePeer string) bool {
	owner, err := netip.ParseAddr(sourcePeer)
	if err != nil {
		// An entry without parsable provenance names no source to exclude.
		return false
	}
	return slices.Contains(key.Excluded, owner)
}

// recoveryIdentity normalizes each affected native identity once per batch.
func recoveryIdentity(fam family.Family, raw []byte, addPath bool) (ribOutKey, error) {
	identity, ok := ribOutRouteKey(fam, raw, addPath)
	if !ok {
		return ribOutKey{}, fmt.Errorf("recovery: invalid NLRI for %s", fam)
	}
	identity.PathID = 0
	return identity, nil
}

// recoverySurvivorsLocked lists the key's candidates in election order, at
// most request.Limit of them when it is nonzero. Every path of every source
// is a candidate, an ADD-PATH source's several identifiers included. Each
// excluded source's candidates, the destination's own, and those of down peers
// are skipped before election. Election needs no sent entry for the key.
// The caller MUST hold peerMu throughout selection and the byte snapshot.
func (r *RIBManager) recoverySurvivorsLocked(request *ribevents.RecoveryRequest, key *ribevents.RecoveryKey, identity ribOutKey) ([]ribevents.RecoveryRoute, error) {
	var candidates []*Candidate
	var prefix netip.Prefix
	var ok bool
	if storage.IsCIDRFamily(request.Family) {
		prefix = identity.Prefix
		if request.Family.SAFI == family.SAFIMPLSLabel {
			// The normalized key already removed labels and the CIDR is
			// canonical. Reuse it instead of parsing the label stack again.
			_, prefix, ok = parsePrevKey(request.Family, []byte(identity.Native), false)
			if !ok {
				return nil, fmt.Errorf("recovery: invalid CIDR NLRI")
			}
		}
		candidates = r.gatherPrefixCandidatesLocked(request.Family, prefix)
	} else {
		// A sent-table identity is not necessarily a receive-storage key:
		// FlowSpec's sent identity omits its native length framing, while
		// storage retains the canonical framed rule. Use the storage boundary
		// on the already validated request NLRI, never a sent-key surrogate.
		raw := key.NLRI
		if request.AddPath {
			raw = raw[4:]
		}
		var scratch [nlrisplit.PrefixKeyScratchSize]byte
		receiveKey := storage.RouteKey(request.Family, raw, scratch[:], false)
		candidates = r.gatherKeyCandidatesLocked(request.Family, receiveKey)
	}
	defer releaseCandidates(candidates)
	eligible := 0
	for i, candidate := range candidates {
		if !r.recoveryEligibleLocked(request, key, candidate) {
			continue
		}
		candidates[eligible], candidates[i] = candidates[i], candidates[eligible]
		eligible++
	}
	if eligible == 0 {
		return nil, nil
	}
	remaining := candidates[:eligible]
	wanted := len(remaining)
	if request.Limit > 0 {
		// draft-abraitis-idr-addpath-paths-limit Section 1: "This indication
		// allows the sender to optimize the transmission of BGP routes by
		// selectively relaying pertinent routes instead of the entire set."
		// The pertinent routes are the election's first Limit choices, so
		// the election stops there rather than ranking the whole set.
		wanted = min(wanted, int(request.Limit))
	}
	survivors := make([]ribevents.RecoveryRoute, 0, wanted)
	// Bounded by the key's candidate count: each pass removes one winner.
	for len(survivors) < wanted {
		// RFC 4271 Section 9.1.2.2: the existing whole-set election chooses
		// the path; repeating it over the rest gives the next choice.
		best := SelectBest(remaining)
		// RFC 4271 Section 3.1: a replacement carries its own changed attributes.
		survivor, err := recoveryCandidate(request.Family, prefix, best)
		if err != nil {
			return nil, err
		}
		survivors = append(survivors, survivor)
		for i, candidate := range remaining {
			if candidate == best {
				last := len(remaining) - 1
				remaining[i], remaining[last] = remaining[last], remaining[i]
				remaining = remaining[:last]
				break
			}
		}
	}
	return survivors, nil
}

// recoveryEligibleLocked keeps a candidate electable for the key. The
// destination never receives its own path back (RFC 4271 Section 9.2).
func (r *RIBManager) recoveryEligibleLocked(request *ribevents.RecoveryRequest, key *ribevents.RecoveryKey, candidate *Candidate) bool {
	if candidate.PeerIP == request.Destination {
		return false
	}
	if slices.Contains(key.Excluded, candidate.PeerIP) {
		return false
	}
	if up, known := r.peerUp[candidate.PeerIP]; known && !up {
		return false
	}
	return true
}

// recoveryCandidate copies the winner's bytes before releasing its retained
// snapshot. No second lookup may silently substitute another route revision.
// RFC 4271 Section 3.1: "The replacement route carries new (changed)
// attributes and has the same address prefix as the original route."
// The retained snapshot supplies both the identity and its attributes.
func recoveryCandidate(fam family.Family, prefix netip.Prefix, best *Candidate) (ribevents.RecoveryRoute, error) {
	attrs, err := best.entry.ToWireBytes()
	if err != nil {
		return ribevents.RecoveryRoute{}, err
	}
	var raw []byte
	if prefix.IsValid() {
		var scratch [cidrKeyOctetsMax]byte
		cidr := candidateNLRI(best, prefix, scratch[:])
		if fam.SAFI == family.SAFIMPLSLabel {
			labels := pool.ResolveLabels(best.labelHandle)
			head := 0
			if best.AddPath {
				head = 4
			}
			raw = make([]byte, len(cidr)+3*len(labels))
			copy(raw[:head], cidr[:head])
			raw[head] = byte(int(cidr[head]) + 24*len(labels))
			// RFC 8277 Section 2.2: Length counts the label stack and prefix.
			// Labels occupy [head+1:head+1+3*N], then the prefix bytes follow.
			n := nlri.WriteLabelValues(raw, head+1, labels)
			copy(raw[head+1+n:], cidr[head+1:])
		} else {
			raw = bytes.Clone(cidr)
		}
	} else {
		raw = framedRouteNLRI(nil, best.Route, best.PathID, best.AddPath)
	}
	route := ribevents.RecoveryRoute{Source: best.PeerIP, NLRI: raw, Attributes: attrs,
		MessageID: best.entry.MsgID, PathID: best.PathID, AddPath: best.AddPath}
	iter := attribute.NewAttrIterator(attrs)
	for code, _, value, valid := iter.Next(); valid; code, _, value, valid = iter.Next() {
		if code == attribute.AttrNextHop && fam == family.IPv4Unicast {
			route.NextHop = bytes.Clone(value)
		}
		if code != attribute.AttrMPReachNLRI || len(value) < 5 {
			continue
		}
		if family.AFI(binary.BigEndian.Uint16(value)) != fam.AFI || family.SAFI(value[2]) != fam.SAFI {
			continue
		}
		length := int(value[3])
		if len(value) >= 4+length {
			route.NextHop = bytes.Clone(value[4 : 4+length])
		}
	}
	return route, nil
}

// recoverySentNLRI retains destination path identity, not source framing.
// RFC 7911 Section 3: "The combination of the address prefix and the Path
// Identifier can be used to identify a route advertised by a BGP speaker."
// A labeled family's native key dropped the label stack, so its withdrawal is
// written by nlrisplit.WithdrawalNLRIFromKey, the one declaration of the RFC
// 8277 Section 2.4 framing; every other family's stored NLRI is already its
// withdrawal framing.
func recoverySentNLRI(fam family.Family, key ribOutKey, entry ribOutEntry) ([]byte, error) {
	if !key.Prefix.IsValid() {
		head := 0
		if entry.AddPath {
			head = 4
		}
		raw := make([]byte, head+3+len(key.Native))
		n, err := nlrisplit.WithdrawalNLRIFromKey(fam, []byte(key.Native), raw[head:])
		if errors.Is(err, nlrisplit.ErrUnsupported) {
			return []byte(entry.NativeNLRI), nil
		}
		if err != nil {
			return nil, fmt.Errorf("recovery: sent %s key %x has no withdrawal framing: %w", fam, key.Native, err)
		}
		if entry.AddPath {
			binary.BigEndian.PutUint32(raw, key.PathID)
		}
		return raw[:head+n], nil
	}
	var scratch [cidrKeyOctetsMax]byte
	head := 0
	if entry.AddPath {
		binary.BigEndian.PutUint32(scratch[:4], key.PathID)
		head = 4
	}
	raw := store.PrefixToNLRIInto(key.Prefix, scratch[head:])
	return bytes.Clone(scratch[:head+len(raw)]), nil
}

// recoveryCommand exposes the same cold batched lookup to an external RIB. Its
// caller MUST successfully drain applied sent deliveries first, outside writeMu.
// The arguments are ribevents.AppendRecoveryArgs's form, the reactor's IPC
// writer's own.
func (r *RIBManager) recoveryCommand(_ string, args []string) (string, any, error) {
	request, err := ribevents.ParseRecoveryArgs(args)
	if err != nil {
		return statusError, nil, err
	}
	// RFC 4271 Section 6: each key is answered with replacements or withdrawal.
	outcomes, err := r.recoveryRoutes(request)
	if err != nil {
		return statusError, nil, err
	}
	return statusDone, outcomes, nil
}
