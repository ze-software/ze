// Design: docs/architecture/plugin/rib-storage-design.md -- source-owned DOWN recovery.
package rib

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/store"
)

// recoveryRoutes reads the existing sent inventory and selection producer. The
// departed source is excluded, never deleted here: an asynchronous old DOWN MUST
// NOT remove a reconnected session. The received-message cut protects its output.
// RFC 4271 Section 6: "The local system recalculates its best routes for the
// destinations of the routes marked as invalid."
// The existing election therefore supplies each surviving replacement.
func (r *RIBManager) recoveryRoutes(request ribevents.RecoveryRequest) ([]ribevents.RecoveryRoute, error) {
	identities := make(map[ribOutKey]struct{}, len(request.NLRIs))
	for _, raw := range request.NLRIs {
		identity, err := recoveryIdentity(request.Family, raw, request.AddPath, request.PrefixOnly)
		if err != nil {
			return nil, err
		}
		identities[identity] = struct{}{}
	}
	r.peerMu.RLock()
	defer r.peerMu.RUnlock()

	entries := r.ribOut[request.Destination][request.Family]
	if request.SentAddPath {
		// ADD-PATH already advertised the surviving paths. Remove only this
		// source's sent identifiers, including zero; do not elect over them.
		var result []ribevents.RecoveryRoute
		source := request.Source.String()
		for key, sent := range entries {
			match := key
			match.PathID = 0
			if _, affected := identities[match]; !affected {
				continue
			}
			if !sent.AddPath {
				continue
			}
			if sent.SourcePeer != source {
				continue
			}
			if sent.SourceMessageID > request.Cut {
				continue
			}
			// RFC 7911 Section 3: withdrawal names the advertised identifier.
			raw := recoverySentNLRI(request.Family, key, sent)
			result = append(result, ribevents.RecoveryRoute{Source: request.Source, NLRI: raw, SentNLRI: raw, Withdraw: true})
		}
		return result, nil
	}
	var result []ribevents.RecoveryRoute
	for identity := range identities {
		entry, found := entries[identity]
		if !found {
			continue
		}
		routes, err := r.recoveryRouteLocked(request, identity, entry)
		if err != nil {
			return nil, err
		}
		result = append(result, routes...)
	}
	return result, nil
}

// recoveryIdentity normalizes each affected native identity once per batch.
func recoveryIdentity(fam family.Family, raw []byte, addPath, prefixOnly bool) (ribOutKey, error) {
	var identity ribOutKey
	var ok bool
	if prefixOnly {
		if fam.SAFI != family.SAFIMPLSLabel {
			return ribOutKey{}, fmt.Errorf("recovery: prefix-only framing requires labeled unicast")
		}
		_, prefix, valid := parsePrevKey(fam, raw, addPath)
		if !valid {
			return ribOutKey{}, fmt.Errorf("recovery: invalid labeled prefix")
		}
		var scratch [cidrKeyOctetsMax]byte
		identity, ok = ribOutKey{Native: string(store.PrefixToNLRIInto(prefix, scratch[:]))}, true
	} else {
		identity, ok = ribOutRouteKey(fam, raw, addPath)
	}
	if !ok {
		return ribOutKey{}, fmt.Errorf("recovery: invalid NLRI for %s", fam)
	}
	identity.PathID = 0
	return identity, nil
}

// recoveryRouteLocked selects a non-ADD-PATH replacement from retained routes.
// The caller MUST hold peerMu throughout selection and the byte snapshot.
func (r *RIBManager) recoveryRouteLocked(request ribevents.RecoveryRequest, identity ribOutKey, entry ribOutEntry) ([]ribevents.RecoveryRoute, error) {
	if entry.AddPath {
		return nil, nil
	}
	if entry.SourceMessageID > request.Cut {
		return nil, nil
	}

	var candidates []*Candidate
	var prefix netip.Prefix
	var ok bool
	if storage.IsCIDRFamily(request.Family) {
		prefix = identity.Prefix
		if request.Family.SAFI == family.SAFIMPLSLabel {
			// The existing sent key already removed labels and normalized the
			// CIDR. Reuse it instead of parsing the source's label stack again.
			_, prefix, ok = parsePrevKey(request.Family, []byte(identity.Native), false)
			if !ok {
				return nil, fmt.Errorf("recovery: invalid CIDR NLRI")
			}
		}
		candidates = r.gatherPrefixCandidatesLocked(request.Family, prefix)
	} else {
		candidates = r.gatherKeyCandidatesLocked(request.Family, []byte(identity.Native))
	}
	defer releaseCandidates(candidates)
	eligible := 0
	for i, candidate := range candidates {
		if candidate.PeerIP == request.Source {
			continue
		}
		if up, known := r.peerUp[candidate.PeerIP]; known && !up {
			continue
		}
		candidates[eligible], candidates[i] = candidates[i], candidates[eligible]
		eligible++
	}
	// RFC 4271 Section 9.1.2.2: the existing whole-set election chooses the path.
	best := SelectBest(candidates[:eligible])
	// RFC 7911 Section 3: preserve the exact destination-framed withdrawal ID.
	sent := recoverySentNLRI(request.Family, identity, entry)
	if best == nil || best.PeerIP == request.Destination {
		if entry.SourcePeer != request.Source.String() {
			return nil, nil
		}
		return []ribevents.RecoveryRoute{{Source: request.Source, NLRI: sent, SentNLRI: sent, Withdraw: true}}, nil
	}
	// RFC 4271 Section 3.1: a replacement carries its own changed attributes.
	replacement, err := recoveryCandidate(request.Family, prefix, best)
	if err != nil {
		return nil, err
	}
	replacement.SentNLRI = sent
	return []ribevents.RecoveryRoute{replacement}, nil
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
	route := ribevents.RecoveryRoute{Source: best.PeerIP, NLRI: raw, Attributes: attrs, MessageID: best.entry.MsgID}
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
// RFC 8277 Section 2.4: "The value of the Compatibility field SHOULD be set
// to 0x800000." Its three octets replace the complete announcement label stack.
func recoverySentNLRI(fam family.Family, key ribOutKey, entry ribOutEntry) []byte {
	if !key.Prefix.IsValid() {
		if fam.SAFI != family.SAFIMPLSLabel && fam.SAFI != family.SAFIVPN {
			return []byte(entry.NativeNLRI)
		}
		head := 0
		if entry.AddPath {
			head = 4
		}
		// Native identity is [prefix-bits][RD?][prefix], with labels removed.
		// Wire is [path-id?][bits+24][Compatibility(3)][RD?][prefix].
		raw := make([]byte, head+3+len(key.Native))
		if entry.AddPath {
			binary.BigEndian.PutUint32(raw, key.PathID)
		}
		raw[head], raw[head+1] = key.Native[0]+24, 0x80
		copy(raw[head+4:], key.Native[1:])
		return raw
	}
	var scratch [cidrKeyOctetsMax]byte
	head := 0
	if entry.AddPath {
		binary.BigEndian.PutUint32(scratch[:4], key.PathID)
		head = 4
	}
	raw := store.PrefixToNLRIInto(key.Prefix, scratch[head:])
	return bytes.Clone(scratch[:head+len(raw)])
}

// recoveryCommand exposes the same cold batched lookup to an external RIB. Its
// caller MUST successfully drain applied sent deliveries first, outside writeMu.
func (r *RIBManager) recoveryCommand(_ string, args []string) (string, any, error) {
	if len(args) != 8 {
		return statusError, nil, fmt.Errorf("usage: request bgp rib recovery <source> <destination> <family> <nlri-hex,...> <add-path> <prefix-only> <sent-add-path> <cut>")
	}
	source, err := netip.ParseAddr(args[0])
	if err != nil {
		return statusError, nil, fmt.Errorf("recovery source: %w", err)
	}
	destination, err := netip.ParseAddr(args[1])
	if err != nil {
		return statusError, nil, fmt.Errorf("recovery destination: %w", err)
	}
	fam, known := family.LookupFamily(args[2])
	if !known {
		return statusError, nil, fmt.Errorf("recovery: unknown family %q", args[2])
	}
	encoded := strings.Split(args[3], ",")
	nlris := make([][]byte, 0, len(encoded))
	for _, identity := range encoded {
		raw, err := hex.DecodeString(identity)
		if err != nil {
			return statusError, nil, fmt.Errorf("recovery NLRI: %w", err)
		}
		nlris = append(nlris, raw)
	}
	addPath, err := strconv.ParseBool(args[4])
	if err != nil {
		return statusError, nil, fmt.Errorf("recovery ADD-PATH: %w", err)
	}
	prefixOnly, err := strconv.ParseBool(args[5])
	if err != nil {
		return statusError, nil, fmt.Errorf("recovery prefix-only: %w", err)
	}
	sentAddPath, err := strconv.ParseBool(args[6])
	if err != nil {
		return statusError, nil, fmt.Errorf("recovery sent ADD-PATH: %w", err)
	}
	cut, err := strconv.ParseUint(args[7], 10, 64)
	if err != nil {
		return statusError, nil, fmt.Errorf("recovery cut: %w", err)
	}
	// RFC 4271 Section 6: source removal selects withdrawal or replacement.
	routes, err := r.recoveryRoutes(ribevents.RecoveryRequest{Source: source, Destination: destination,
		Family: fam, NLRIs: nlris, AddPath: addPath, PrefixOnly: prefixOnly, SentAddPath: sentAddPath, Cut: cut})
	if err != nil {
		return statusError, nil, err
	}
	return statusDone, routes, nil
}
