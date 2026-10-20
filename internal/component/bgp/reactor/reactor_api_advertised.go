// Design: docs/architecture/api/commands.md -- clear bgp rib advertised
// RFC: rfc/short/rfc4271.md -- Section 9.2, Update-Send Process
// Related: adj_rib_out.go -- the per-session table this empties
// Related: peer_static_wire.go -- the static route set a clear also forgets
package reactor

import (
	"context"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/route"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// WithdrawAdvertised withdraws every route each selected peer's current
// session holds in its Adj-RIB-Out, limited to one family when afi is not
// zero (AFI 0 is reserved, so it never names a family and stands for every
// family here). It answers the count of withdrawn routes.
//
// RFC 4271 Section 9.2: "All newly installed routes and all newly unfeasible
// routes for which there is no replacement route SHALL be advertised to its
// peers by means of an UPDATE message." A route the operator clears from the
// Adj-RIB-Out is unfeasible toward that peer, with no replacement.
//
// The table holds keys and no attributes, which is enough to withdraw and not
// enough to resend: the resend is bgp-rib's `clear bgp rib out` replay. Each
// withdrawal goes through the local withdraw rail (WithdrawNLRIBatch), so the
// Adj-RIB-Out drops the key on the write and bgp-rib drops its Adj-RIB-Out copy
// on the sent event, and a later replay does not bring the route back.
//
// The peer's static route set is forgotten too, so a reload sends the
// configured routes again rather than reading them as already delivered.
//
// No selected peer is an error (route.ErrNoPeersMatch): the clear did nothing,
// and the caller must be able to tell that from a table that was empty.
func (a *reactorAPIAdapter) WithdrawAdvertised(ctx context.Context, sel *selector.Selector, afi uint16, safi uint8, sender plugin.Sender) (int, error) {
	a.r.mu.RLock()
	peers, err := a.getMatchingPeersSel(sel, announceOrigin(sender))
	a.r.mu.RUnlock()
	if err != nil {
		return 0, err
	}
	if len(peers) == 0 {
		return 0, route.ErrNoPeersMatch
	}
	only := family.Family{AFI: family.AFI(afi), SAFI: family.SAFI(safi)}

	withdrawn := 0
	var errs []error
	for _, peer := range peers {
		count, peerErr := a.withdrawPeerAdvertised(ctx, peer, only, sender)
		withdrawn += count
		if peerErr != nil {
			errs = append(errs, peerErr)
		}
	}
	return withdrawn, errors.Join(errs...)
}

// withdrawPeerAdvertised withdraws one peer's Adj-RIB-Out. staticMu is held
// across the withdrawal and the reset of the static set, so a reload delta
// (deliverStaticRouteDelta) never reads a set the wire no longer holds.
func (a *reactorAPIAdapter) withdrawPeerAdvertised(ctx context.Context, peer *Peer, only family.Family, sender plugin.Sender) (int, error) {
	peer.staticMu.Lock()
	defer peer.staticMu.Unlock()

	session := peer.currentSession()
	if session == nil {
		return 0, nil // No connection, so nothing is advertised on one.
	}
	address := peer.Settings().Address
	batches, err := peer.adjOut.withdrawalBatches(session, only)
	if err != nil {
		return 0, fmt.Errorf("clear advertised: peer %s: %w", address, err)
	}

	target, err := selector.Parse(address.String())
	if err != nil {
		return 0, fmt.Errorf("clear advertised: peer %s: %w", address, err)
	}
	withdrawn := 0
	for i := range batches {
		if err := a.WithdrawNLRIBatch(ctx, target, batches[i], sender); err != nil {
			return withdrawn, fmt.Errorf("clear advertised: peer %s family %s: %w", address, batches[i].Family, err)
		}
		withdrawn += len(batches[i].NLRIs)
	}
	if peer.staticWire.session == session {
		peer.staticWire.routes = nil
	}
	return withdrawn, nil
}

// withdrawalBatches snapshots the routes the table holds for session, one
// batch per family, each NLRI in its withdrawal form and owning its bytes.
// A table bound to another session holds nothing this session advertised.
// Safe for concurrent use: it reads the table under mu.
func (a *adjRIBOut) withdrawalBatches(session *Session, only family.Family) ([]bgptypes.NLRIBatch, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.session != session {
		return nil, nil
	}
	byFamily := make(map[family.Family]int)
	var batches []bgptypes.NLRIBatch
	for key, entry := range a.routes {
		fam := key.path.family
		if only.AFI != 0 && fam != only {
			continue
		}
		wire, err := adjOutWithdrawalNLRI(key, entry)
		if err != nil {
			return nil, err
		}
		n, err := nlri.NewWireNLRI(fam, wire, key.addPath)
		if err != nil {
			return nil, err
		}
		index, ok := byFamily[fam]
		if !ok {
			index = len(batches)
			byFamily[fam] = index
			batches = append(batches, bgptypes.NLRIBatch{Family: fam})
		}
		batches[index].NLRIs = append(batches[index].NLRIs, n)
	}
	return batches, nil
}

// adjOutWithdrawalNLRI rebuilds the withdrawn NLRI one table entry names,
// path identifier first when the key carries one, in a slice of its own.
//
// A labeled family (RFC 8277, RFC 3107) keys a route by its prefix with the
// label stack removed, and a forwarded route keeps no exact NLRI, so the key
// alone cannot be written back as the announcement. The withdrawal needs no
// stack (RFC 8277 Section 2.4, Figure 4):
//
//	+--------+-----------------------------+---------------------------------+
//	| Length | Compatibility 0x800000 (3)  | RD (8, SAFI 128 only) | Prefix ~ |
//	+--------+-----------------------------+---------------------------------+
//
// The rebuilt bytes are keyed again and compared with the entry's key, so a
// family whose key drops wire information this function does not restore is
// refused by name rather than withdrawn as some other route.
func adjOutWithdrawalNLRI(key adjOutKey, entry *adjRIBOutRoute) ([]byte, error) {
	keyBytes := key.path.nlri[:key.path.length]
	if key.path.overflow != "" {
		keyBytes = []byte(key.path.overflow)
	}
	head := 0
	if key.addPath {
		head = 4
	}
	var wire []byte
	switch {
	case nlrisplit.CarriesCompatibility(key.path.family):
		if len(keyBytes) == 0 {
			return nil, fmt.Errorf("family %s: empty table key", key.path.family)
		}
		if int(keyBytes[0])+24 > 255 {
			return nil, fmt.Errorf("family %s: a %d-bit key has no room for the Compatibility field", key.path.family, keyBytes[0])
		}
		wire = make([]byte, head+len(keyBytes)+3)
		// RFC 8277 Section 2.4: "Upon transmission, the Compatibility field
		// SHOULD be set to 0x800000." The length octet counts it: "to determine
		// the prefix length of a prefix in an MP_UNREACH_NLRI, the length of the
		// Compatibility field must be subtracted from the length of the NLRI."
		wire[head] = keyBytes[0] + 24
		wire[head+1], wire[head+2], wire[head+3] = 0x80, 0x00, 0x00
		copy(wire[head+4:], keyBytes[1:])
	case entry.exactNLRI != nil:
		return append([]byte(nil), entry.exactNLRI...), nil
	default:
		wire = make([]byte, head+len(keyBytes))
		copy(wire[head:], keyBytes)
	}
	if key.addPath {
		id := key.path.received
		wire[0], wire[1], wire[2], wire[3] = byte(id>>24), byte(id>>16), byte(id>>8), byte(id)
	}
	var check fwdPathKey
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	if err := fwdPathKeyFor(&check, key.path.family, key.path.received, wire[head:], true, scratch[:]); err != nil {
		return nil, fmt.Errorf("family %s: the table key cannot be written back as a withdrawal: %w", key.path.family, err)
	}
	if check != key.path {
		return nil, fmt.Errorf("family %s: the table key cannot be written back as a withdrawal", key.path.family)
	}
	return wire, nil
}
