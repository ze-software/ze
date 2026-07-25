// Design: plan/learned/1015-cp-survival-5-detect-5-characterization.md -- Stage-2 characterization.
// Related: doctor.go -- warns when neither flow source these queries use is configured.
// Related: metrics.go -- per-family and fallback counters incremented here.
// On the rate-trigger active transition the detector (off the onRate hot path)
// resolves the victim prefix from trafficusage and emits the fast AttackDetected,
// then queries the flowexport recent-flow ring, classifies the attack family and
// the narrowest discriminating VectorTuple (proto / ports / TCP flags), ranks the
// top sources, computes source-address entropy, and emits AttackCharacterized so
// responders install a surgical rule. Both emits fall back gracefully: an absent
// trafficusage yields an empty target + generic-flood (never worse than before),
// and an absent flow source simply skips AttackCharacterized (responders keep the
// coarse AttackDetected).

package detect

import (
	"cmp"
	"context"
	"encoding/json"
	"math"
	"net/netip"
	"sort"
	"time"

	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// statusDone mirrors plugin.StatusDone ("done"). Duplicated as a local constant
// to keep the detector runtime path free of an internal/component/plugin import.
const statusDone = "done"

// Classifier heuristic constants (NetHawk Refinement 2, field-tested). Percentage
// cut-offs of the attack's packet mix; seeds, not protocol constants.
const (
	protoICMP = 1
	protoTCP  = 6
	protoUDP  = 17

	udpDominantPct  = 80 // UDP share for a UDP-flood / reflection verdict
	tcpDominantPct  = 80 // TCP share for a SYN-flood verdict (with the flag check)
	icmpDominantPct = 50 // ICMP share for an ICMP-flood verdict
	topPortPct      = 50 // a single dst/src port must own this share to enter the vector
	synHalfOpenPct  = 50 // half-open share of TCP flows for a SYN-flood verdict

	tcpFlagSYN = 0x02 // TCP SYN bit set in VectorTuple.TCPFlags for a SYN flood
)

// tcpHalfOpenStates are the nf_conntrack_tcp states of an incomplete handshake:
// a SYN flood parks many connections here (SYN_SENT=1, SYN_RECV=2, SYN_SENT2=9).
var tcpHalfOpenStates = map[uint8]bool{1: true, 2: true, 9: true}

// reflectionPorts are UDP source ports of common amplification services
// (NetHawk Refinement 1, extended). A UDP flood whose dominant source port is in
// this table is reflection/amplification. One constant, here beside the
// classifier (plugin self-containment), not spread across packages.
var reflectionPorts = map[uint16]bool{
	53:    true, // DNS
	123:   true, // NTP
	161:   true, // SNMP
	389:   true, // CLDAP/LDAP
	1900:  true, // SSDP
	11211: true, // Memcached
	19:    true, // CharGEN
	111:   true, // RPC/portmap
	137:   true, // NetBIOS-NS
	5353:  true, // mDNS
	3702:  true, // WS-Discovery
	1434:  true, // MS-SQL resolution
	520:   true, // RIPv1
}

// dispatchFunc is the engine-routed command dispatcher (Plugin.DispatchCommand).
// Injected into the detector so tests can stub the source responses without a
// live engine.
type dispatchFunc func(ctx context.Context, command string) (status string, data json.RawMessage, err error)

// flowRecord is one recent conntrack flow parsed from `show flow recent`. Value
// type; only the fields the classifier needs.
type flowRecord struct {
	SrcAddr  netip.Addr
	DstAddr  netip.Addr
	SrcPort  uint16
	DstPort  uint16
	Protocol uint8
	Packets  uint64
	TCPState uint8
	LastMs   uint64 // Unix ms of the most recent packet, for the characterize-window filter
}

// characterizeAndEmit runs once per attack on its own goroutine (off the onRate
// hot path). It resolves the victim, emits the fast AttackDetected, then
// characterizes from the recent-flow ring and emits AttackCharacterized. Stale
// emits (the attack cleared or a newer one started mid-query) are dropped via the
// attackGen generation guard.
func (d *detector) characterizeAndEmit(gen uint64, ifaceName string, peakPps, peakBps, threshold float64) {
	timeout := time.Duration(d.cfg.CharacterizeTimeout) * time.Millisecond
	ctx, cancel := context.WithTimeout(d.ctx, timeout)
	defer cancel()

	severity := ddosevent.GradeSeverity(peakPps, threshold)

	target := ddosevent.VectorTuple{}
	if prefix, ok := d.characterizeTarget(ctx, ifaceName); ok {
		target.DstPrefix = prefix
	}

	// Traffic policy, emit stage: only the victim (destination) is known here, so
	// source-matching rules are evaluated later, at the characterization stage. A
	// detection-scope exemption suppresses the attack entirely (no event, no incident).
	emitOutcome := d.cfg.Policy.evaluate(target.DstPrefix, nil)
	if emitOutcome.Suppress {
		incPolicySuppressed(scopeDetection)
		return
	}
	if emitOutcome.SuppressMitigation {
		incPolicySuppressed(scopeMitigation)
	}
	direction := d.classifyDirection(target.DstPrefix)
	incDirection(direction)

	if !d.emitDetected(gen, ifaceName, target, severity, direction, emitOutcome.SuppressMitigation, peakPps, peakBps) {
		return
	}

	// Stage 2: refine into a characterized vector from the recent-flow ring.
	d.characterizeFromFlows(ctx, gen, ifaceName, target.DstPrefix, peakPps, peakBps, threshold, severity)
}

// emitDetected emits AttackDetected iff the attack generation is still gen,
// holding d.mu across the check AND the (synchronous) emit. Returns false when
// stale. The lock is what makes the generation guard sound: emitCleared runs its
// Cleared emit under d.mu on the rate tick, so holding d.mu here prevents a
// Cleared from slipping between the check and the emit -- which would otherwise
// leave ddos-local with a drop and no matching Cleared (a permanent stuck rule,
// since max-mitigation-duration is not enforced there). The slow source queries
// already ran off the lock, so this critical section is just the emit.
func (d *detector) emitDetected(gen uint64, ifaceName string, target ddosevent.VectorTuple, severity ddosevent.Severity, direction ddosevent.Direction, suppressMitigation bool, peakPps, peakBps float64) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.attackGen != gen {
		return false
	}
	if _, err := ddosevent.Detected.Emit(d.bus, &ddosevent.AttackDetected{
		Interface:          ifaceName,
		Target:             target,
		Family:             ddosevent.FamilyGenericFlood,
		Severity:           severity,
		Direction:          direction,
		SuppressMitigation: suppressMitigation,
		PeakRxPps:          peakPps,
		PeakRxBps:          peakBps,
		Observable:         true,
	}); err != nil {
		logger().Warn("ddos-detect: emit detected failed", "error", err)
	}
	// Detected has now reached subscribers; allow onRate to emit Ongoing.
	d.detectedEmitted.Store(true)
	return true
}

// emitCharacterized emits AttackCharacterized iff the attack generation is still
// gen, holding d.mu across the check and the emit (see emitDetected for why the
// lock is load-bearing). Returns false when stale or the emit errored.
func (d *detector) emitCharacterized(gen uint64, ev *ddosevent.AttackCharacterized) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.attackGen != gen {
		return false
	}
	if _, err := ddosevent.Characterized.Emit(d.bus, ev); err != nil {
		logger().Warn("ddos-detect: emit characterized failed", "error", err)
		return false
	}
	return true
}

// characterizeRetryInterval paces awaitClassifiableFlows' poll of the recent-flow
// ring. A package var so tests can shrink it; 150ms is well under the default
// 3s characterize-timeout (≈20 polls) yet coarse enough that the forced conntrack
// dump has time to land between polls.
var characterizeRetryInterval = 150 * time.Millisecond

// characterizeFromFlows resolves the attack's flow set (polling the recent-flow
// ring until it reflects the attack), classifies it, and emits AttackCharacterized.
// It emits nothing (leaving responders on the coarse AttackDetected) when no flow
// source is reachable or no flows arrive -- never worse than before. When
// trafficusage gave no victim it derives one from the dominant destination.
func (d *detector) characterizeFromFlows(ctx context.Context, gen uint64, ifaceName string, victim netip.Prefix, peakPps, peakBps, threshold float64, severity ddosevent.Severity) {
	if !d.cfg.CharacterizeEnable {
		return
	}

	flows, victim, ok := d.awaitClassifiableFlows(ctx, gen, victim)
	if !ok {
		incFallback()
		return
	}

	family, vec, topSources, entropy := classifyFlows(flows, d.cfg.TopNSources)
	vec.DstPrefix = victim

	// Traffic policy, characterization stage: sources are known now, so re-evaluate
	// (source rules can match here). The characterized event is authoritative -- if a
	// source rule flips the decision to exempt, responders withdraw any drop the fast
	// AttackDetected installed. Both Suppress and SuppressMitigation map to "do not
	// mitigate" here; the incident is already recorded by the fast path.
	charOutcome := d.cfg.Policy.evaluate(victim, topSources)
	direction := d.classifyDirection(victim)

	if entropy >= d.cfg.EntropyThreshold {
		logger().Info("ddos-detect: distributed attack (high source entropy)",
			"entropy", entropy, "threshold", d.cfg.EntropyThreshold, "target", victim)
	}

	// Confidence combines the peak/threshold ratio, family specificity, and source
	// spread into a 0-100 score so observability, the dashboard reporter, and
	// responders can distinguish a real attack from a borderline spike.
	confidence := ddosevent.GradeConfidence(peakPps, threshold, family, entropy, d.cfg.EntropyThreshold)

	if d.emitCharacterized(gen, &ddosevent.AttackCharacterized{
		Interface:          ifaceName,
		Target:             vec,
		Family:             family,
		TopSources:         topSources,
		Severity:           severity,
		SourceEntropy:      entropy,
		Confidence:         confidence,
		Direction:          direction,
		SuppressMitigation: charOutcome.Suppress || charOutcome.SuppressMitigation,
		PeakRxPps:          peakPps,
		PeakRxBps:          peakBps,
		Observable:         true,
	}) {
		incCharacterize(family)
	}
}

// awaitClassifiableFlows polls the recent-flow ring until it yields a
// classifiable (non-generic) attack or the characterize budget (ctx) expires,
// then returns the best flow set seen and its victim. AttackDetected, emitted
// just before this runs, asks flow-export for an immediate conntrack dump, but
// the ring can still lag it by a dump -- and at the operator's active-timeout
// (default 60s) a single read would see only pre-attack state. Polling within the
// budget lets the ring warm so the confidence path (AC-9) actually classifies
// instead of always falling back to generic-flood.
//
// A hard source absence (no dispatch wired, or the query errors) returns
// immediately with ok=false: no dump is coming for a retry to wait on, so the
// coarse AttackDetected simply stands. A present-but-empty or not-yet-dominant
// ring is retried. Returns ok=false when no usable flows ever arrive.
func (d *detector) awaitClassifiableFlows(ctx context.Context, gen uint64, victim netip.Prefix) ([]flowRecord, netip.Prefix, bool) {
	var (
		bestFlows  []flowRecord
		bestVictim = victim
		haveFlows  bool
	)

	ticker := time.NewTicker(characterizeRetryInterval)
	defer ticker.Stop()

	for {
		// Stop as soon as the attack this characterization belongs to has cleared
		// or been superseded: the emit would be dropped by the generation guard
		// anyway, and continuing to force dumps for a dead attack is pure waste.
		if !d.genCurrent(gen) {
			return nil, victim, false
		}

		flows, ok := d.queryRecentFlows(ctx, victim)
		if !ok {
			return nil, victim, false // source absent: no retry can help
		}
		// Drop flows older than the characterize-window (flows with no timestamp
		// are kept -- absence of a timestamp is not evidence of staleness).
		flows = filterByWindow(flows, d.cfg.CharacterizeWindow, time.Now())

		v := victim
		if !v.IsValid() {
			// Without a trafficusage victim, derive one from the flow set.
			if dv, ok := dominantDestination(flows); ok {
				v = dv
				flows = filterByDst(flows, v)
			}
		}

		if len(flows) > 0 {
			bestFlows, bestVictim, haveFlows = flows, v, true
			if family, _, _, _ := classifyFlows(flows, d.cfg.TopNSources); family != ddosevent.FamilyGenericFlood {
				return flows, v, true // a specific family: the ring is warm
			}
		}

		select {
		case <-ctx.Done():
			// Budget spent: emit whatever we have (a genuine generic-flood still
			// carries a confidence score); ok=false only when nothing ever arrived.
			return bestFlows, bestVictim, haveFlows
		case <-ticker.C:
		}
	}
}

// genCurrent reports whether the attack generation is still gen, read under d.mu
// so it observes emitCleared/onAttackStart's advance of attackGen consistently.
func (d *detector) genCurrent(gen uint64) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.attackGen == gen
}

// filterByWindow drops flows last seen before now-window. A flow with no
// timestamp (LastMs == 0) is kept: absence of a timestamp is not evidence of
// staleness. A non-positive window disables the filter.
func filterByWindow(flows []flowRecord, windowSec int, now time.Time) []flowRecord {
	if windowSec <= 0 {
		return flows
	}
	cutoffMs := now.Add(-time.Duration(windowSec) * time.Second).UnixMilli()
	if cutoffMs < 0 {
		return flows
	}
	cutoff := uint64(cutoffMs)
	out := make([]flowRecord, 0, len(flows))
	for i := range flows {
		if flows[i].LastMs == 0 || flows[i].LastMs >= cutoff {
			out = append(out, flows[i])
		}
	}
	return out
}

// characterizeTarget asks trafficusage for the attacked interface and returns its
// dominant destination as a host prefix. ok is false when no dispatch is wired,
// the source is absent/errors, or it reports no destination -- the caller then
// emits the generic-flood fallback.
func (d *detector) characterizeTarget(ctx context.Context, ifaceName string) (netip.Prefix, bool) {
	if d.dispatch == nil {
		return netip.Prefix{}, false
	}

	var tb textbuf.Buffer
	tb.Str("show traffic usage name ").Str(ifaceName)

	status, data, err := d.dispatch(ctx, tb.String())
	if err != nil || status != statusDone {
		d.sourceAbsentOnce.Do(func() {
			logger().Warn("ddos-detect: flow source unavailable; characterization falling back to generic-flood",
				"command", "show traffic usage", "status", status, "error", err)
		})
		return netip.Prefix{}, false
	}
	return parseTopDestination(data)
}

// queryRecentFlows asks flowexport for recent flows (filtered to victim when it is
// valid) and parses them. ok is false when no dispatch is wired or the source is
// absent/errors; an empty-but-present result returns (nil, true).
func (d *detector) queryRecentFlows(ctx context.Context, victim netip.Prefix) ([]flowRecord, bool) {
	if d.dispatch == nil {
		return nil, false
	}

	var tb textbuf.Buffer
	tb.Str("show flow recent")
	if victim.IsValid() {
		tb.Str(" dst ").Str(victim.String())
	}

	status, data, err := d.dispatch(ctx, tb.String())
	if err != nil || status != statusDone {
		d.flowSourceAbsentOnce.Do(func() {
			logger().Warn("ddos-detect: flow-recent source unavailable; keeping coarse target",
				"command", "show flow recent", "status", status, "error", err)
		})
		return nil, false
	}
	return parseFlowRecords(data), true
}

// parseTopDestination picks the highest-byte destination from a
// "show traffic-usage name <iface>" response and returns it as a host prefix
// (/32 for IPv4, /128 for IPv6). trafficusage records destinations in egress-ips
// and is IPv4-only today; IPv6 targets arrive from the flowexport tap. Malformed
// or empty input yields ok=false (caller falls back).
func parseTopDestination(data json.RawMessage) (netip.Prefix, bool) {
	var resp struct {
		EgressIPs []struct {
			IP    string `json:"ip"`
			Bytes uint64 `json:"bytes"`
		} `json:"egress-ips"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return netip.Prefix{}, false
	}

	var bestIP string
	var bestBytes uint64
	found := false
	for _, e := range resp.EgressIPs {
		if e.IP == "" {
			continue
		}
		if !found || e.Bytes > bestBytes {
			found = true
			bestBytes = e.Bytes
			bestIP = e.IP
		}
	}
	if !found {
		return netip.Prefix{}, false
	}

	addr, err := netip.ParseAddr(bestIP)
	if err != nil {
		return netip.Prefix{}, false
	}
	return netip.PrefixFrom(addr, addr.BitLen()), true
}

// parseFlowRecords decodes the `show flow recent` JSON list. Malformed rows are
// skipped rather than failing the whole set (the response is untrusted input).
func parseFlowRecords(data json.RawMessage) []flowRecord {
	var raw []struct {
		SrcAddr  string `json:"src-addr"`
		DstAddr  string `json:"dst-addr"`
		SrcPort  int    `json:"src-port"`
		DstPort  int    `json:"dst-port"`
		Protocol int    `json:"protocol"`
		Packets  uint64 `json:"packets"`
		TCPState int    `json:"tcp-state"`
		LastMs   uint64 `json:"last-ms"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	out := make([]flowRecord, 0, len(raw))
	for i := range raw {
		src, err1 := netip.ParseAddr(raw[i].SrcAddr)
		dst, err2 := netip.ParseAddr(raw[i].DstAddr)
		if err1 != nil || err2 != nil {
			continue
		}
		out = append(out, flowRecord{
			SrcAddr:  src,
			DstAddr:  dst,
			SrcPort:  uint16(raw[i].SrcPort),
			DstPort:  uint16(raw[i].DstPort),
			Protocol: uint8(raw[i].Protocol),
			Packets:  raw[i].Packets,
			TCPState: uint8(raw[i].TCPState),
			LastMs:   raw[i].LastMs,
		})
	}
	return out
}

// classifyFlows examines flows (already narrowed to the victim) and returns the
// attack family, the narrowest discriminating VectorTuple (proto + ports + TCP
// flags, DstPrefix left to the caller), the top-N source addresses by packet
// volume, and the Shannon entropy of the source distribution. An empty set, or a
// mix with no dominant signature, classifies as generic-flood with a best-effort
// vector. Fragment floods have no on-box conntrack signal (defrag runs before
// conntrack) and therefore fall here into generic (see spec Known Limitations).
func classifyFlows(flows []flowRecord, topN int) (ddosevent.AttackFamily, ddosevent.VectorTuple, []netip.Addr, float64) {
	var vec ddosevent.VectorTuple
	if len(flows) == 0 {
		return ddosevent.FamilyGenericFlood, vec, nil, 0
	}

	var totalPkts, tcpPkts, tcpHalfOpenPkts uint64
	protoPkts := map[uint8]uint64{}
	srcPortPkts := map[uint16]uint64{}
	dstPortPkts := map[uint16]uint64{}
	srcPkts := map[netip.Addr]uint64{}

	for i := range flows {
		f := &flows[i]
		// A SYN flood's half-open flows often report zero/low conntrack counters;
		// count each flow at least once so packet-share reflects flow-share.
		p := f.Packets
		if p == 0 {
			p = 1
		}
		totalPkts += p
		protoPkts[f.Protocol] += p
		srcPkts[f.SrcAddr] += p
		if f.Protocol == protoTCP {
			tcpPkts += p
			if tcpHalfOpenStates[f.TCPState] {
				tcpHalfOpenPkts += p
			}
		}
		if f.SrcPort != 0 {
			srcPortPkts[f.SrcPort] += p
		}
		if f.DstPort != 0 {
			dstPortPkts[f.DstPort] += p
		}
	}

	entropy := sourceEntropy(srcPkts, totalPkts)
	topSources := rankTopSources(srcPkts, topN)

	domProto, domProtoPkts := topKey(protoPkts)
	family := ddosevent.FamilyGenericFlood

	switch {
	case domProto == protoUDP && pctOf(domProtoPkts, totalPkts) >= udpDominantPct:
		vec.Proto = protoUDP
		if sp, spPkts := topKey(srcPortPkts); pctOf(spPkts, totalPkts) >= topPortPct && reflectionPorts[sp] {
			family = ddosevent.FamilyReflection
			vec.SrcPort = sp
		} else {
			family = ddosevent.FamilyUDPFlood
			if dp, dpPkts := topKey(dstPortPkts); pctOf(dpPkts, totalPkts) >= topPortPct {
				vec.DstPort = dp
			}
		}
	case domProto == protoTCP && pctOf(domProtoPkts, totalPkts) >= tcpDominantPct:
		vec.Proto = protoTCP
		// SYN flood requires BOTH TCP dominance AND a half-open majority -- never
		// TCP share alone (the NetHawk weakness, spec R-3).
		if pctOf(tcpHalfOpenPkts, tcpPkts) >= synHalfOpenPct {
			family = ddosevent.FamilySYNFlood
			vec.TCPFlags = tcpFlagSYN
		}
		if dp, dpPkts := topKey(dstPortPkts); pctOf(dpPkts, totalPkts) >= topPortPct {
			vec.DstPort = dp
		}
	case domProto == protoICMP && pctOf(domProtoPkts, totalPkts) >= icmpDominantPct:
		family = ddosevent.FamilyICMPFlood
		vec.Proto = protoICMP
	}

	return family, vec, topSources, entropy
}

// sourceEntropy is the Shannon entropy (bits) of the per-source packet
// distribution: 0 for a single source, higher (up to log2(N)) as the flood
// spreads across sources -- the distributed/spoofed annotation.
func sourceEntropy(bySource map[netip.Addr]uint64, total uint64) float64 {
	if total == 0 {
		return 0
	}
	var h float64
	for _, n := range bySource {
		if n == 0 {
			continue
		}
		p := float64(n) / float64(total)
		h -= p * math.Log2(p)
	}
	if h < 0 {
		h = 0 // guard tiny negative from float rounding at p==1
	}
	return h
}

// rankTopSources returns the n highest-volume source addresses, ties broken by
// address order for determinism.
func rankTopSources(bySource map[netip.Addr]uint64, n int) []netip.Addr {
	type kv struct {
		addr netip.Addr
		pkts uint64
	}
	arr := make([]kv, 0, len(bySource))
	for a, p := range bySource {
		arr = append(arr, kv{a, p})
	}
	sort.Slice(arr, func(i, j int) bool {
		if arr[i].pkts != arr[j].pkts {
			return arr[i].pkts > arr[j].pkts
		}
		return arr[i].addr.Less(arr[j].addr)
	})
	if n > len(arr) {
		n = len(arr)
	}
	out := make([]netip.Addr, 0, n)
	for i := range n {
		out = append(out, arr[i].addr)
	}
	return out
}

// dominantDestination returns the highest-volume destination as a host prefix,
// used to derive the victim when trafficusage supplied none.
func dominantDestination(flows []flowRecord) (netip.Prefix, bool) {
	dstPkts := map[netip.Addr]uint64{}
	for i := range flows {
		p := flows[i].Packets
		if p == 0 {
			p = 1
		}
		dstPkts[flows[i].DstAddr] += p
	}
	top := rankTopSources(dstPkts, 1)
	if len(top) == 0 {
		return netip.Prefix{}, false
	}
	a := top[0]
	return netip.PrefixFrom(a, a.BitLen()), true
}

// filterByDst keeps only flows destined into dst.
func filterByDst(flows []flowRecord, dst netip.Prefix) []flowRecord {
	out := make([]flowRecord, 0, len(flows))
	for i := range flows {
		if dst.Contains(flows[i].DstAddr) {
			out = append(out, flows[i])
		}
	}
	return out
}

// topKey returns the map key with the largest value, ties broken by the smaller
// key for determinism.
func topKey[K cmp.Ordered](m map[K]uint64) (bestKey K, bestVal uint64) {
	first := true
	for k, v := range m {
		if first || v > bestVal || (v == bestVal && k < bestKey) {
			bestKey, bestVal = k, v
			first = false
		}
	}
	return bestKey, bestVal
}

// pctOf returns part as an integer percentage of total (0 when total is 0).
func pctOf(part, total uint64) int {
	if total == 0 {
		return 0
	}
	return int(part * 100 / total)
}
