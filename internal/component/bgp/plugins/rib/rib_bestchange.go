// Design: docs/architecture/plugin/rib-storage-design.md -- best-path change tracking
// Overview: rib.go -- RIB plugin core types and event handlers
// Related: bestpath.go -- best-path selection algorithm (RFC 4271 S9.1.2)
// Related: rib_structured.go -- structured event handlers that trigger best-path checks
//
// Real-time best-path tracking and Bus event publishing.
// After each INSERT/REMOVE in handleReceivedStructured, the affected prefix is
// checked for best-path changes. Changes are collected into a batch under the
// RIB lock, then published to the Bus after lock release.
package rib

import (
	"encoding/json"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/pool"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// bestChangeAction represents the type of best-path change.
const (
	bestChangeAdd      = "add"
	bestChangeUpdate   = "update"
	bestChangeWithdraw = "withdraw"
)

// bestChangeTopic is the Bus topic for BGP best-path change events.
const bestChangeTopic = "rib/best-change/bgp"

// bestChangeEntry records one prefix best-path change for batch publishing.
type bestChangeEntry struct {
	Action       string `json:"action"`
	Prefix       string `json:"prefix"`
	NextHop      string `json:"next-hop,omitempty"`
	Priority     int    `json:"priority"`
	Metric       uint32 `json:"metric"`
	ProtocolType string `json:"protocol-type,omitempty"` // "ebgp" or "ibgp"
}

// bestChangeBatch is the JSON payload published to the Bus.
type bestChangeBatch struct {
	Changes []bestChangeEntry `json:"changes"`
}

// bestPathKey identifies a unique prefix in the RIB for best-path tracking.
type bestPathKey struct {
	Family string
	NLRI   string // string(nlriBytes)
}

// bestPathRecord stores the previous best-path state for change detection.
type bestPathRecord struct {
	PeerAddr     string
	Prefix       string // parsed prefix string, stored to avoid re-parsing during replay
	NextHop      string
	Priority     int // admin distance: eBGP=20, iBGP=200
	Metric       uint32
	ProtocolType string // "ebgp" or "ibgp"
}

// prefixBytesForDisplay returns the NLRI bytes suitable for wirePrefixToString.
// For ADD-PATH, strips the 4-byte path-ID prefix that splitNLRIs includes.
func prefixBytesForDisplay(nlriBytes []byte, addPath bool) []byte {
	if addPath && len(nlriBytes) > 4 {
		return nlriBytes[4:]
	}
	return nlriBytes
}

// checkBestPathChange evaluates the best path for a prefix after an insert or remove.
// Compares with the previous best and returns a change entry if the best path changed.
// addPath indicates whether nlriBytes includes a 4-byte path-ID prefix.
// Caller MUST hold r.mu (write lock).
func (r *RIBManager) checkBestPathChange(fam family.Family, nlriBytes []byte, addPath bool) *bestChangeEntry {
	familyStr := fam.String()
	key := bestPathKey{Family: familyStr, NLRI: string(nlriBytes)}

	// Gather candidates from all peers for this prefix.
	candidates := r.gatherCandidates(fam, nlriBytes)
	newBest := SelectBest(candidates)

	prev := r.bestPrev[key]
	displayBytes := prefixBytesForDisplay(nlriBytes, addPath)
	prefix := wirePrefixToString(displayBytes, familyStr)
	if prefix == "" {
		return nil
	}

	if newBest == nil {
		// No candidates remain -- withdraw if we had a previous best.
		if prev != nil {
			delete(r.bestPrev, key)
			return &bestChangeEntry{
				Action: bestChangeWithdraw,
				Prefix: prefix,
			}
		}
		return nil
	}

	// Extract next-hop, priority, and protocol type from the new best.
	nextHop := r.bestCandidateNextHop(fam, nlriBytes, newBest)
	priority := r.adminDistance(newBest)
	protoType := r.protocolType(newBest)
	metric := newBest.MED

	if prev == nil {
		// New best path where none existed before.
		r.bestPrev[key] = &bestPathRecord{
			PeerAddr:     newBest.PeerAddr,
			Prefix:       prefix,
			NextHop:      nextHop,
			Priority:     priority,
			Metric:       metric,
			ProtocolType: protoType,
		}
		return &bestChangeEntry{
			Action:       bestChangeAdd,
			Prefix:       prefix,
			NextHop:      nextHop,
			Priority:     priority,
			Metric:       metric,
			ProtocolType: protoType,
		}
	}

	// Check if the best path actually changed.
	if prev.PeerAddr == newBest.PeerAddr && prev.NextHop == nextHop &&
		prev.Priority == priority && prev.Metric == metric {
		return nil // Same best, no change.
	}

	// Best path changed.
	r.bestPrev[key] = &bestPathRecord{
		PeerAddr:     newBest.PeerAddr,
		NextHop:      nextHop,
		Priority:     priority,
		Metric:       metric,
		ProtocolType: protoType,
	}
	return &bestChangeEntry{
		Action:       bestChangeUpdate,
		Prefix:       prefix,
		NextHop:      nextHop,
		Priority:     priority,
		Metric:       metric,
		ProtocolType: protoType,
	}
}

// protocolType returns "ebgp" or "ibgp" for a candidate based on ASN comparison.
// When LocalASN is 0 (unknown, e.g. before OPEN negotiation completes),
// defaults to "ebgp". This is intentional: routes learned before ASN
// negotiation are assumed external, which is the more common case.
func (r *RIBManager) protocolType(c *Candidate) string {
	if c.LocalASN == 0 || c.PeerASN != c.LocalASN {
		return "ebgp"
	}
	return "ibgp"
}

// adminDistance returns the admin distance for a candidate.
// eBGP = 20, iBGP = 200. Uses protocolType to determine the session type.
func (r *RIBManager) adminDistance(c *Candidate) int {
	if r.protocolType(c) == "ebgp" {
		return 20
	}
	return 200 // iBGP
}

// bestCandidateNextHop extracts the next-hop string for the winning candidate's route entry.
// For IPv4, reads from the NEXT_HOP attribute (code 3).
// For IPv6 and other MP families, extracts from MP_REACH_NLRI (code 14) in OtherAttrs.
// Caller MUST hold r.mu (at least read lock).
func (r *RIBManager) bestCandidateNextHop(fam family.Family, nlriBytes []byte, best *Candidate) string {
	peerRIB := r.ribInPool[best.PeerAddr]
	if peerRIB == nil {
		return ""
	}
	entry, ok := peerRIB.Lookup(fam, nlriBytes)
	if !ok {
		return ""
	}

	// Try IPv4 NEXT_HOP attribute (code 3) first.
	if entry.HasNextHop() {
		data, err := pool.NextHop.Get(entry.NextHop)
		if err == nil {
			return formatNextHop(data)
		}
	}

	// For IPv6/multiprotocol: extract next-hop from MP_REACH_NLRI (code 14) in OtherAttrs.
	// MP_REACH wire format: AFI(2) + SAFI(1) + NH_len(1) + NH(variable) + reserved(1) + NLRIs.
	if entry.HasOtherAttrs() {
		nh := extractMPNextHop(entry)
		if nh != "" {
			return nh
		}
	}

	return ""
}

// extractMPNextHop extracts the next-hop from MP_REACH_NLRI stored in OtherAttrs.
// OtherAttrs format: [type(1)][flags(1)][length_16bit(2)][value(n)]...
// MP_REACH value: AFI(2) + SAFI(1) + NH_len(1) + NH(variable) + ...
func extractMPNextHop(entry storage.RouteEntry) string {
	data, err := pool.OtherAttrs.Get(entry.OtherAttrs)
	if err != nil {
		return ""
	}

	// Walk OtherAttrs to find attribute type code 14 (MP_REACH_NLRI).
	off := 0
	for off+4 <= len(data) {
		typeCode := data[off]
		length := int(data[off+2])<<8 | int(data[off+3])
		off += 4

		if off+length > len(data) {
			break
		}

		if typeCode == 14 { // MP_REACH_NLRI
			value := data[off : off+length]
			// AFI(2) + SAFI(1) + NH_len(1) = 4 bytes minimum.
			if len(value) < 4 {
				return ""
			}
			nhLen := int(value[3])
			if len(value) < 4+nhLen {
				return ""
			}
			nhBytes := value[4 : 4+nhLen]
			// For 32-byte next-hop (IPv6 global + link-local), use the first 16 bytes.
			if nhLen == 32 {
				nhBytes = nhBytes[:16]
			}
			return formatNextHop(nhBytes)
		}

		off += length
	}
	return ""
}

// replayBestPaths publishes the entire current best-path table as a batch event.
// Used for full-table replay when a downstream subscriber requests it via
// the rib/replay-request control topic. Metadata includes "replay": "true".
// Caller MUST NOT hold r.mu.
func (r *RIBManager) replayBestPaths() {
	bus := getBus()
	if bus == nil {
		return
	}

	r.mu.RLock()
	changesByFamily := make(map[string][]bestChangeEntry)
	for key, rec := range r.bestPrev {
		if rec.Prefix == "" {
			continue // Skip entries with unparseable prefixes.
		}
		changesByFamily[key.Family] = append(changesByFamily[key.Family], bestChangeEntry{
			Action:       bestChangeAdd,
			Prefix:       rec.Prefix,
			NextHop:      rec.NextHop,
			Priority:     rec.Priority,
			Metric:       rec.Metric,
			ProtocolType: rec.ProtocolType,
		})
	}
	r.mu.RUnlock()

	for famName, changes := range changesByFamily {
		batch := bestChangeBatch{Changes: changes}
		payload, err := json.Marshal(batch)
		if err != nil {
			logger().Warn("replay marshal failed", "error", err)
			continue
		}
		metadata := map[string]string{
			"protocol": "bgp",
			"family":   famName,
			"replay":   "true",
		}
		bus.Publish(bestChangeTopic, payload, metadata)
	}

	logger().Info("best-path replay published", "families", len(changesByFamily))
}

// replayConsumer implements ze.Consumer for the rib/replay-request topic.
// When a downstream consumer (e.g., sysrib) publishes a replay request,
// the RIB responds by publishing its entire best-path table.
type replayConsumer struct {
	rib *RIBManager
}

// Deliver triggers a full best-path replay.
func (c *replayConsumer) Deliver(_ []ze.Event) error {
	c.rib.replayBestPaths()
	return nil
}

// publishBestChanges marshals a batch of changes and publishes to the Bus.
// Called AFTER the RIB lock is released.
func publishBestChanges(changes []bestChangeEntry, family string) {
	bus := getBus()
	if bus == nil {
		return
	}

	batch := bestChangeBatch{Changes: changes}
	payload, err := json.Marshal(batch)
	if err != nil {
		logger().Warn("best-change marshal failed", "error", err)
		return
	}

	metadata := map[string]string{
		"protocol": "bgp",
		"family":   family,
	}
	bus.Publish(bestChangeTopic, payload, metadata)
}
