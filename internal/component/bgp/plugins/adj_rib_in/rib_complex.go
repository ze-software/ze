// Design: docs/architecture/plugin/rib-storage-design.md — Adj-RIB-In raw hex storage
// Related: rib.go — the structured ingest path that calls these
// Related: compact_key.go — routeKeyFromNLRI, the key these store under
// RFC: rfc/short/rfc7911.md -- Section 3, the Path Identifier split from each NLRI
// RFC: rfc/short/rfc8277.md -- Section 2.4, what a withdrawal must match

package adj_rib_in

import (
	"encoding/hex"
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// installComplexNLRIs stores the NLRIs of a family whose encoding is not the
// plain [length][prefix] one (labeled unicast, VPN, EVPN, FlowSpec and the rest),
// one route per NLRI. The family's registered splitter frames each NLRI and
// routeKeyFromNLRI keys it, so neither step reads label or RD octets as an
// address. Caller must hold r.mu.
//
// Each route stores its own NLRI bytes, label stack and RD included, with the
// ADD-PATH identifier moved to PathID: the relay rebuilds the route as the source
// sent it, and a withdrawal of one route leaves the others in place.
//
// nhopHex is the MP_REACH next-hop field as the source framed it. It used to be
// an address STRING, which lost the link-local half of an RFC 2545 Section 3
// pair before it ever reached storage.
func (r *AdjRIBInManager) installComplexNLRIs(peerAddr netip.Addr, fam family.Family, data []byte, addPath bool, attrHex, nhopHex string, msgID uint64) {
	if attrHex == "" {
		return
	}
	if nhopHex == "" {
		return
	}
	split := nlrisplit.Get(fam)
	if split == nil {
		logger().Warn("received routes not stored: the family has no registered NLRI splitter, so its routes cannot be told apart; register one in nlrisplit",
			"peer", peerAddr, "family", fam)
		return
	}
	_, err := split(data, addPath, func(raw []byte) bool {
		// RFC 7911 Section 3
		pathID, payload, err := nlri.SplitPathID(raw, addPath)
		if err != nil {
			return false
		}
		rk, err := r.routeKeyFromNLRI(fam, payload, pathID, false)
		if err != nil {
			logger().Debug("received NLRI not stored: it names no route of its family",
				"peer", peerAddr, "family", fam, "error", err)
			return true
		}
		if r.ribIn[peerAddr] == nil {
			r.ribIn[peerAddr] = newSeqMap()
		}
		r.seqCounter++
		r.ribIn[peerAddr].Put(rk, r.seqCounter, &RawRoute{
			Family:      fam,
			AttrHex:     attrHex,
			NHopHex:     nhopHex,
			NLRIHex:     hex.EncodeToString(payload),
			PathID:      pathID,
			NLRIFraming: rpc.NLRIFramingPrefixOnly,
			MsgID:       msgID,
		})
		return true
	})
	if err != nil {
		logger().Warn("received NLRI section is malformed: the routes after the corruption are not stored",
			"peer", peerAddr, "family", fam, "error", err)
	}
}

// removeComplexNLRIs withdraws the NLRIs of a family installComplexNLRIs
// stores, framed by the family's withdrawal splitter and keyed the same way, so
// a withdrawal names exactly the route its announcement stored.
// Caller must hold r.mu.
func (r *AdjRIBInManager) removeComplexNLRIs(peerAddr netip.Addr, fam family.Family, data []byte, addPath bool) {
	split := nlrisplit.GetWithdraw(fam)
	if split == nil {
		logger().Warn("received withdrawals not applied: the family has no registered NLRI splitter; register one in nlrisplit",
			"peer", peerAddr, "family", fam)
		return
	}
	_, err := split(data, addPath, func(raw []byte) bool {
		// RFC 7911 Section 3
		pathID, payload, err := nlri.SplitPathID(raw, addPath)
		if err != nil {
			return false
		}
		// RFC 8277 Section 2.4: "When using an MP_UNREACH_NLRI attribute to
		// withdraw a route whose NLRI was previously specified in an
		// MP_REACH_NLRI attribute, the lengths and values of the respective
		// prefixes must match, and the respective AFI/SAFIs must match. If the
		// procedures of [RFC7911] are being used, the respective values of the
		// "path identifier" fields must match as well." The key is the family,
		// the path identifier and the route the NLRI names, built as the
		// announcement's was, so the withdrawal matches on exactly those.
		rk, err := r.routeKeyFromNLRI(fam, payload, pathID, true)
		if err != nil {
			logger().Debug("withdrawn NLRI ignored: it names no route of its family",
				"peer", peerAddr, "family", fam, "error", err)
			return true
		}
		r.removePending(peerAddr, rk)
		if r.ribIn[peerAddr] != nil {
			r.ribIn[peerAddr].Delete(rk)
		}
		return true
	})
	if err != nil {
		logger().Warn("withdrawn NLRI section is malformed: the withdrawals after the corruption are not applied",
			"peer", peerAddr, "family", fam, "error", err)
	}
}
