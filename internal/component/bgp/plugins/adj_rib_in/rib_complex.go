// Design: docs/architecture/plugin/rib-storage-design.md — Adj-RIB-In raw hex storage
// Related: rib.go — the structured ingest path that calls these
// Related: compact_key.go — routeKeyFromNLRI, the key these store under
// RFC: rfc/short/rfc7911.md -- Section 3, the Path Identifier split from each NLRI
// RFC: rfc/short/rfc8277.md -- Section 2.4, what a withdrawal must match

package adj_rib_in

import (
	"encoding/hex"
	"time"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	nlriKeyOpInstall  = "install"
	nlriKeyOpRemove   = "remove"
	nlriKeyWarnWindow = time.Minute
	nlriKeyWarnMax    = 1024
)

// nlriKeyWarnKey scopes complex NLRI key-derivation rate-limiting to one
// source peer, address family, and operation ("install" or "remove").
type nlriKeyWarnKey struct {
	peer      receivedPeer
	family    family.Family
	operation string
}

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
func (r *AdjRIBInManager) installComplexNLRIs(peerAddr receivedPeer, fam family.Family, data []byte, addPath bool, attrHex, nhopHex string, msgID, sourcePolicyEpoch, sourceOwner, connectionID, sourceReceipt uint64) {
	r.installComplexNLRIsWithFilter(peerAddr, fam, data, addPath, attrHex, nhopHex, msgID, sourcePolicyEpoch, sourceOwner, connectionID, sourceReceipt, false, "")
}

func (r *AdjRIBInManager) installComplexNLRIsWithFilter(peerAddr receivedPeer, fam family.Family, data []byte, addPath bool, attrHex, nhopHex string, msgID, sourcePolicyEpoch, sourceOwner, connectionID, sourceReceipt uint64, importFiltered bool, filterStep string) {
	if attrHex == "" {
		return
	}
	if nhopHex == "" {
		return
	}
	split := nlrisplit.Get(fam)
	if split == nil {
		logger().Warn("received routes not stored: the family has no registered NLRI splitter, so its routes cannot be told apart; register one in nlrisplit",
			"peer", peerAddr.address, "peer-owner", peerAddr.owner, "family", fam)
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
			r.reportComplexNLRIKeyErrorLocked(peerAddr, fam, nlriKeyOpInstall, err)
			return true
		}
		if !r.receiveCurrentLocked(peerAddr, rk, connectionID, msgID) {
			return true
		}
		r.putInstalledRouteLocked(peerAddr, rk, &RawRoute{
			Family:            fam,
			AttrHex:           attrHex,
			NHopHex:           nhopHex,
			NLRIHex:           hex.EncodeToString(payload),
			PathID:            pathID,
			NLRIFraming:       rpc.NLRIFramingPrefixOnly,
			HasPathID:         addPath,
			ImportFiltered:    importFiltered,
			FilterStep:        filterStep,
			MsgID:             msgID,
			SourcePolicyEpoch: sourcePolicyEpoch,
			SourceOwner:       sourceOwner,
			SourceReceipt:     sourceReceipt,
			ConnectionID:      connectionID,
		})
		return true
	})
	if err != nil {
		logger().Warn("received NLRI section is malformed: the routes after the corruption are not stored",
			"peer", peerAddr.address, "peer-owner", peerAddr.owner, "family", fam, "error", err)
	}
}

// removeComplexNLRIs withdraws the NLRIs of a family installComplexNLRIs
// stores, framed by the family's withdrawal splitter and keyed the same way, so
// a withdrawal names exactly the route its announcement stored.
// Caller must hold r.mu.
func (r *AdjRIBInManager) removeComplexNLRIs(peerAddr receivedPeer, fam family.Family, data []byte, addPath bool, connectionID, msgID uint64) {
	split := nlrisplit.GetWithdraw(fam)
	if split == nil {
		logger().Warn("received withdrawals not applied: the family has no registered NLRI splitter; register one in nlrisplit",
			"peer", peerAddr.address, "peer-owner", peerAddr.owner, "family", fam)
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
			r.reportComplexNLRIKeyErrorLocked(peerAddr, fam, nlriKeyOpRemove, err)
			return true
		}
		r.removeReceivedLocked(peerAddr, rk, connectionID, msgID)
		return true
	})
	if err != nil {
		logger().Warn("withdrawn NLRI section is malformed: the withdrawals after the corruption are not applied",
			"peer", peerAddr.address, "peer-owner", peerAddr.owner, "family", fam, "error", err)
	}
}

// reportComplexNLRIKeyErrorLocked counts an unkeyable complex NLRI and emits a
// rate-limited WARN plus report-bus error event under
// codeDoctorBGPAdjRIBInNLRIKeyFailed. Caller MUST hold r.mu.
func (r *AdjRIBInManager) reportComplexNLRIKeyErrorLocked(peerAddr receivedPeer, fam family.Family, operation string, err error) {
	incNLRIKeyErrorMetric(peerAddr.address, fam, operation)
	key := nlriKeyWarnKey{peer: peerAddr, family: fam, operation: operation}
	if !r.shouldWarnNLRIKeyLocked(key, time.Now()) {
		return
	}
	peer := peerAddr.address.String()
	familyName := fam.String()
	var msgBuf textbuf.Buffer
	if operation == nlriKeyOpInstall {
		msgBuf.Str("adj-rib-in: received complex NLRI not stored for peer ").Str(peer).Str(" family ").Str(familyName).Str(" because storage key derivation failed: ").Err(err).Str("; remaining NLRIs in the UPDATE continue processing")
	} else {
		msgBuf.Str("adj-rib-in: withdrawn complex NLRI ignored for peer ").Str(peer).Str(" family ").Str(familyName).Str(" because storage key derivation failed: ").Err(err).Str("; remaining withdrawals in the UPDATE continue processing")
	}
	msg := fitReportText(msgBuf.String(), rpc.ReportMessageMax)
	var subjBuf textbuf.Buffer
	subject := fitReportText(subjBuf.Str(peer).Byte('/').Str(familyName).Byte('/').Str(operation).String(), rpc.ReportSubjectMax)
	reportErr := r.issues.RaiseError(codeDoctorBGPAdjRIBInNLRIKeyFailed, subject, msg, fixShowAdjRIBIn, func(cbErr error) {
		logger().Warn("adj-rib-in: failed to report complex NLRI key error to daemon",
			"code", codeDoctorBGPAdjRIBInNLRIKeyFailed,
			"fix", fixShowAdjRIBIn,
			"peer", peer,
			"family", familyName,
			"operation", operation,
			"error", cbErr)
	})
	logger().Warn(msg,
		"code", codeDoctorBGPAdjRIBInNLRIKeyFailed,
		"fix", fixShowAdjRIBIn,
		"peer", peerAddr.address,
		"peer-owner", peerAddr.owner,
		"family", fam,
		"operation", operation,
		"error", err,
		issuequeue.Outcome(reportErr))
}

func fitReportText(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	const suffix = "..."
	if maxBytes <= len(suffix) {
		cut := maxBytes
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		return s[:cut]
	}
	cut := maxBytes - len(suffix)
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + suffix
}

// shouldWarnNLRIKeyLocked enforces a 1-minute per-(peer, family, operation)
// warn window inside a bounded nlriKeyWarnMax-entry map. Caller MUST hold r.mu.
func (r *AdjRIBInManager) shouldWarnNLRIKeyLocked(key nlriKeyWarnKey, now time.Time) bool {
	if r.nlriKeyWarned == nil {
		r.nlriKeyWarned = make(map[nlriKeyWarnKey]time.Time)
	}
	if last, ok := r.nlriKeyWarned[key]; ok {
		if now.Sub(last) < nlriKeyWarnWindow {
			return false
		}
	}
	if _, exists := r.nlriKeyWarned[key]; !exists {
		if len(r.nlriKeyWarned) >= nlriKeyWarnMax {
			for k, t := range r.nlriKeyWarned {
				if now.Sub(t) >= nlriKeyWarnWindow {
					delete(r.nlriKeyWarned, k)
				}
			}
		}
		if len(r.nlriKeyWarned) >= nlriKeyWarnMax {
			var oldestKey nlriKeyWarnKey
			var oldestTime time.Time
			first := true
			for k, t := range r.nlriKeyWarned {
				if first {
					oldestKey, oldestTime, first = k, t, false
				} else if t.Before(oldestTime) {
					oldestKey, oldestTime = k, t
				}
			}
			delete(r.nlriKeyWarned, oldestKey)
		}
	}
	r.nlriKeyWarned[key] = now
	return true
}

// pruneNLRIKeyWarnedPeerLocked drops rate-limit entries for a peer whose
// session went down. Caller MUST hold r.mu.
func (r *AdjRIBInManager) pruneNLRIKeyWarnedPeerLocked(peer receivedPeer) {
	for k := range r.nlriKeyWarned {
		if k.peer == peer {
			delete(r.nlriKeyWarned, k)
		}
	}
}

// pruneNLRIKeyWarnedOwnerLocked drops rate-limit entries for a removed source
// owner. Caller MUST hold r.mu.
func (r *AdjRIBInManager) pruneNLRIKeyWarnedOwnerLocked(owner uint64) {
	for k := range r.nlriKeyWarned {
		if k.peer.owner == owner {
			delete(r.nlriKeyWarned, k)
		}
	}
}
