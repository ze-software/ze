// Design: (none -- research/analysis tool)
//
// Analyzes MRT BGP4MP dumps to measure UPDATE message density:
// how many NLRIs per UPDATE, and how many UPDATEs per second.
//
// This data directly informs per-peer channel sizing for the forward pool:
// if most UPDATEs carry 1 NLRI, the channel absorbs updates not prefixes.
// The per-second distribution reveals what "burst" means in production.
package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"sort"
)

// secondSample records the update count for one timestamp.
type secondSample struct {
	ts    uint32
	count int
}

// peerSecond tracks per-peer-AS per-second counts for burst detection.
type peerSecond struct {
	ts    uint32
	count int
}

type densityStats struct {
	announced map[int]int // announced NLRIs per UPDATE -> count
	withdrawn map[int]int // withdrawn NLRIs per UPDATE -> count
	total     map[int]int // total NLRIs per UPDATE -> count
	perSecond map[int]int // UPDATEs in a second -> number of seconds with that count

	// Ordered time series for burst detection.
	series []secondSample

	// Per-peer-AS tracking: accumulates (ts, count) per source peer.
	peerBins   map[uint32]*peerBinTracker // peer_as -> tracker
	peerTotals map[uint32]int             // peer_as -> total updates

	totalUpdates int
	totalAnnNLRI int
	totalWdNLRI  int

	// Per-second tracking using MRT timestamps.
	lastTS     uint32
	currentBin int
}

// peerBinTracker accumulates per-second counts for one peer AS.
type peerBinTracker struct {
	lastTS     uint32
	currentBin int
	series     []peerSecond
}

func newDensityStats() *densityStats {
	return &densityStats{
		announced:  make(map[int]int),
		withdrawn:  make(map[int]int),
		total:      make(map[int]int),
		perSecond:  make(map[int]int),
		peerBins:   make(map[uint32]*peerBinTracker),
		peerTotals: make(map[uint32]int),
	}
}

func runDensity(args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, `ze-analyse density -- measure UPDATE message density and burst patterns

Processes BGP4MP records from MRT files to produce two distributions:
  1. NLRIs per UPDATE: how many prefixes does each UPDATE carry?
  2. UPDATEs per second: what does a burst look like?

This tells you whether the forward pool channel should count updates or prefixes,
and what P50/P95/P99 burst rates look like on real Internet traffic.

Usage:
  ze-analyse density <updates.gz> [updates2.gz ...]

Examples:
  ze-analyse density test/internet/ripe-updates.*.gz
  ze-analyse density test/internet/rv-updates.*.gz
`)
		return 1
	}

	st := newDensityStats()

	for _, fname := range args {
		if err := processMRTFile(fname, mrtHandler{
			OnBGP4MP: func(data []byte, subtype uint16, ts uint32) {
				densityProcessBGP4MP(data, subtype, ts, st)
			},
		}); err != nil {
			fmt.Fprintf(os.Stderr, "error processing %s: %v\n", fname, err)
		}
	}

	// Flush last second bin.
	if st.currentBin > 0 {
		st.perSecond[st.currentBin]++
		st.series = append(st.series, secondSample{st.lastTS, st.currentBin})
	}

	// Flush per-peer trackers.
	for _, pt := range st.peerBins {
		if pt.currentBin > 0 {
			pt.series = append(pt.series, peerSecond{pt.lastTS, pt.currentBin})
		}
	}

	printDensityResults(st)
	return 0
}

func densityProcessBGP4MP(data []byte, subtype uint16, ts uint32, st *densityStats) {
	body, peerASN := extractBGP4MPUpdate(subtype, data)
	if body == nil {
		return
	}

	// Track per-peer-AS per-second counts.
	if peerASN > 0 {
		st.peerTotals[peerASN]++
		pt, ok := st.peerBins[peerASN]
		if !ok {
			pt = &peerBinTracker{}
			st.peerBins[peerASN] = pt
		}
		switch {
		case pt.lastTS == 0:
			pt.lastTS = ts
			pt.currentBin = 1
		case ts == pt.lastTS:
			pt.currentBin++
		default:
			pt.series = append(pt.series, peerSecond{pt.lastTS, pt.currentBin})
			pt.lastTS = ts
			pt.currentBin = 1
		}
	}

	annCount, wdCount := countUpdateNLRIs(body)

	st.totalUpdates++
	st.totalAnnNLRI += annCount
	st.totalWdNLRI += wdCount
	st.announced[annCount]++
	st.withdrawn[wdCount]++
	st.total[annCount+wdCount]++

	// Per-second binning using MRT timestamp.
	switch {
	case st.lastTS == 0:
		st.lastTS = ts
		st.currentBin = 1
	case ts == st.lastTS:
		st.currentBin++
	default:
		// Flush completed second to histogram and time series.
		if st.currentBin > 0 {
			st.perSecond[st.currentBin]++
			st.series = append(st.series, secondSample{st.lastTS, st.currentBin})
		}
		// Guard against backward timestamps (files processed out of order).
		if ts < st.lastTS {
			st.lastTS = ts
			st.currentBin = 1
			return
		}
		gap := min(int(ts-st.lastTS), 3600) // cap at 1 hour to avoid runaway loop on file boundaries
		for range gap - 1 {
			st.perSecond[0]++
		}
		st.lastTS = ts
		st.currentBin = 1
	}
}

// countUpdateNLRIs parses an UPDATE body and returns (announced, withdrawn) NLRI counts.
// Counts from all four locations: withdrawn field, trailing NLRI, MP_REACH, MP_UNREACH.
func countUpdateNLRIs(body []byte) (announced, withdrawn int) {
	if len(body) < 4 {
		return 0, 0
	}

	// Withdrawn routes (IPv4 unicast).
	wdLen := int(binary.BigEndian.Uint16(body[0:2]))
	off := 2
	if off+wdLen > len(body) {
		return 0, 0
	}
	withdrawn = countPackedPrefixes(body[off : off+wdLen])
	off += wdLen

	// Path attributes.
	if off+2 > len(body) {
		return announced, withdrawn
	}
	attrLen := int(binary.BigEndian.Uint16(body[off : off+2]))
	off += 2
	if off+attrLen > len(body) {
		return announced, withdrawn
	}
	attrEnd := off + attrLen

	// Scan attributes for MP_REACH and MP_UNREACH.
	iterateAttrs(body[off:attrEnd], func(_, typeCode uint8, value []byte) {
		switch typeCode {
		case attrMPReachNLRI:
			announced += countMPReachNLRI(value)
		case attrMPUnreachNLRI:
			withdrawn += countMPUnreachNLRI(value)
		}
	})
	off = attrEnd

	// Trailing NLRI field (IPv4 unicast announcements).
	if off < len(body) {
		announced += countPackedPrefixes(body[off:])
	}

	return announced, withdrawn
}

// countMPReachNLRI counts NLRIs inside an MP_REACH_NLRI attribute value.
// Format: AFI(2) + SAFI(1) + NH_len(1) + NH(var) + reserved(1) + NLRI(rest).
func countMPReachNLRI(val []byte) int {
	if len(val) < 5 {
		return 0
	}
	nhLen := int(val[3])
	off := 4 + nhLen + 1 // AFI(2)+SAFI(1)+NHlen(1) + NH + reserved(1)
	if off > len(val) {
		return 0
	}
	return countPackedPrefixes(val[off:])
}

// countMPUnreachNLRI counts NLRIs inside an MP_UNREACH_NLRI attribute value.
// Format: AFI(2) + SAFI(1) + withdrawn_routes(rest).
func countMPUnreachNLRI(val []byte) int {
	if len(val) < 3 {
		return 0
	}
	return countPackedPrefixes(val[3:])
}

func printDensityResults(st *densityStats) {
	// Header with context.
	fmt.Println("BGP UPDATE Density Analysis")
	fmt.Println("===========================")
	fmt.Println()
	fmt.Println("This analysis measures how many prefixes (NLRIs) each BGP UPDATE message")
	fmt.Println("carries, and how many UPDATEs arrive per second. These numbers directly")
	fmt.Println("inform per-peer buffer sizing: if most UPDATEs carry 1 NLRI, then the")
	fmt.Println("buffer absorbs N updates, not N prefixes. The per-second distribution")
	fmt.Println("shows what 'burst' means on real Internet traffic.")
	fmt.Println()
	fmt.Printf("Total UPDATEs analyzed: %s\n", formatNumber(uint64(st.totalUpdates))) //nolint:gosec // non-negative
	fmt.Printf("Total announced NLRIs: %s\n", formatNumber(uint64(st.totalAnnNLRI)))  //nolint:gosec // non-negative
	fmt.Printf("Total withdrawn NLRIs: %s\n", formatNumber(uint64(st.totalWdNLRI)))   //nolint:gosec // non-negative
	fmt.Println()

	printDensityDist("Announced NLRIs per UPDATE", st.announced, "NLRIs", st.totalUpdates,
		"How many prefixes are announced in each UPDATE message.\n"+
			"A high count at 1 means most UPDATEs carry a single prefix (common).\n"+
			"Higher counts indicate batched announcements (convergence events).")

	printDensityDist("Withdrawn NLRIs per UPDATE", st.withdrawn, "NLRIs", st.totalUpdates,
		"How many prefixes are withdrawn in each UPDATE message.\n"+
			"0 means the UPDATE only announced (no withdrawals). Most UPDATEs are\n"+
			"announcements; withdrawals cluster during peer-down events.")

	printDensityDist("Total NLRIs (announced + withdrawn) per UPDATE", st.total, "NLRIs", st.totalUpdates,
		"Combined prefix count per UPDATE. This is the effective 'weight' of each\n"+
			"message for buffer sizing purposes.")

	// Per-second distribution -- split into active (>0 updates) and idle (0 updates).
	totalSeconds := 0
	activeSeconds := 0
	idleSeconds := st.perSecond[0]
	for k, count := range st.perSecond {
		totalSeconds += count
		if k > 0 {
			activeSeconds += count
		}
	}

	// Build active-only distribution for the table.
	activeDist := make(map[int]int, len(st.perSecond))
	for k, v := range st.perSecond {
		if k > 0 {
			activeDist[k] = v
		}
	}
	printDensityDist("UPDATEs per second (active seconds only)", activeDist, "UPD/s", activeSeconds,
		"How many UPDATE messages arrived in each 1-second window, excluding idle\n"+
			"seconds (0 updates). The P95 and P99 values represent burst peaks that\n"+
			"the per-peer channel must absorb without hitting the overflow pool.")

	// Summary statistics.
	fmt.Println("== Summary ==")
	fmt.Println()
	fmt.Println("Key numbers for forward pool channel sizing:")
	fmt.Println()
	fmt.Println("| Metric | Value |")
	fmt.Println("|--------|-------|")
	fmt.Printf("| Total UPDATEs | %s |\n", formatNumber(uint64(st.totalUpdates)))         //nolint:gosec // non-negative
	fmt.Printf("| Total announced NLRIs | %s |\n", formatNumber(uint64(st.totalAnnNLRI))) //nolint:gosec // non-negative
	fmt.Printf("| Total withdrawn NLRIs | %s |\n", formatNumber(uint64(st.totalWdNLRI)))  //nolint:gosec // non-negative
	if st.totalUpdates > 0 {
		avgAnn := float64(st.totalAnnNLRI) / float64(st.totalUpdates)
		avgWd := float64(st.totalWdNLRI) / float64(st.totalUpdates)
		fmt.Printf("| Avg announced/UPDATE | %.2f |\n", avgAnn)
		fmt.Printf("| Avg withdrawn/UPDATE | %.2f |\n", avgWd)
		fmt.Printf("| Avg total NLRIs/UPDATE | %.2f |\n", avgAnn+avgWd)
	}
	if totalSeconds > 0 && activeSeconds > 0 {
		fmt.Printf("| Observation window | %s seconds |\n", formatNumber(uint64(totalSeconds))) //nolint:gosec // non-negative
		fmt.Printf("| Active seconds (>0 UPD) | %s |\n", formatNumber(uint64(activeSeconds)))   //nolint:gosec // non-negative
		fmt.Printf("| Idle seconds (0 UPD) | %s |\n", formatNumber(uint64(idleSeconds)))        //nolint:gosec // non-negative
		fmt.Printf("| Avg UPDATEs/active second | %.1f |\n", float64(st.totalUpdates)/float64(activeSeconds))
		fmt.Printf("| Peak UPDATEs/second | %d |\n", densityMaxKey(st.perSecond))
		fmt.Printf("| P50 UPDATEs/active second | %d |\n", densityPercentile(activeDist, activeSeconds, 50))
		fmt.Printf("| P95 UPDATEs/active second | %d |\n", densityPercentile(activeDist, activeSeconds, 95))
		fmt.Printf("| P99 UPDATEs/active second | %d |\n", densityPercentile(activeDist, activeSeconds, 99))
	}
	fmt.Println()

	// Burst vs maintenance analysis.
	printBurstAnalysis(st)
}

// burstRun represents a consecutive run of active seconds.
type burstRun struct {
	startTS  uint32
	duration int // seconds
	updates  int // total updates in this run
}

func printBurstAnalysis(st *densityStats) {
	if len(st.peerBins) == 0 {
		return
	}

	fmt.Println("== Setup vs Maintenance Analysis (per source peer) ==")
	fmt.Println()
	fmt.Println("BGP traffic has two modes: setup (session establishment, full table dump)")
	fmt.Println("and maintenance (steady-state churn, individual route changes). The per-peer")
	fmt.Println("channel handles maintenance; the overflow pool handles setup bursts.")
	fmt.Println()
	fmt.Println("Each source peer is analysed independently. A 'run' is consecutive seconds")
	fmt.Println("of updates from that peer. Runs with >100 updates = setup. Fewer = maintenance.")
	fmt.Println()

	const setupThreshold = 100

	// Aggregate across all peers.
	var allMaintSeconds int
	var allSetupSeconds int
	var allMaintUpdates int
	var allSetupUpdates int
	maintPerSec := make(map[int]int) // per-peer-second rate -> count of seconds
	setupPerSec := make(map[int]int)

	type peerSummary struct {
		asn          uint32
		total        int
		maintUpdates int
		setupUpdates int
		maintSeconds int
		setupSeconds int
		setupRunPeak int // largest setup run
	}
	var peers []peerSummary

	for asn, pt := range st.peerBins {
		runs := detectPeerRuns(pt.series)
		ps := peerSummary{asn: asn, total: st.peerTotals[asn]}

		for _, r := range runs {
			if r.updates >= setupThreshold {
				ps.setupUpdates += r.updates
				ps.setupSeconds += r.duration
				if r.updates > ps.setupRunPeak {
					ps.setupRunPeak = r.updates
				}
				// Record per-second rates for setup runs.
				rate := r.updates / max(r.duration, 1)
				setupPerSec[rate] += r.duration
			} else {
				ps.maintUpdates += r.updates
				ps.maintSeconds += r.duration
				// Record per-second rate for each second in maintenance.
				rate := r.updates / max(r.duration, 1)
				maintPerSec[rate] += r.duration
			}
		}

		allMaintUpdates += ps.maintUpdates
		allSetupUpdates += ps.setupUpdates
		allMaintSeconds += ps.maintSeconds
		allSetupSeconds += ps.setupSeconds
		peers = append(peers, ps)
	}

	totalUpdates := allMaintUpdates + allSetupUpdates

	// Overall summary.
	fmt.Println("| Mode | Peer-seconds | Updates | Pct of Traffic |")
	fmt.Println("|------|-------------|---------|----------------|")
	if allMaintUpdates > 0 {
		pct := float64(allMaintUpdates) / float64(totalUpdates) * 100
		fmt.Printf("| Maintenance | %s | %s | %.1f%% |\n",
			formatNumber(uint64(allMaintSeconds)), formatNumber(uint64(allMaintUpdates)), pct) //nolint:gosec // non-negative
	}
	if allSetupUpdates > 0 {
		pct := float64(allSetupUpdates) / float64(totalUpdates) * 100
		fmt.Printf("| Setup/Burst | %s | %s | %.1f%% |\n",
			formatNumber(uint64(allSetupSeconds)), formatNumber(uint64(allSetupUpdates)), pct) //nolint:gosec // non-negative
	}
	fmt.Println()

	// Top peers by setup volume.
	sort.Slice(peers, func(i, j int) bool { return peers[i].setupUpdates > peers[j].setupUpdates })
	setupPeers := 0
	maintOnlyPeers := 0
	for _, p := range peers {
		if p.setupUpdates > 0 {
			setupPeers++
		} else {
			maintOnlyPeers++
		}
	}
	fmt.Printf("Peers with setup bursts: %d (sending full tables or large convergence events)\n", setupPeers)
	fmt.Printf("Maintenance-only peers: %d (steady-state churn only)\n\n", maintOnlyPeers)

	if setupPeers > 0 {
		fmt.Println("Top peers by setup volume (full table dumps / convergence):")
		fmt.Println()
		fmt.Println("| Peer AS | Setup Updates | Setup Seconds | Peak Run | Maint Updates |")
		fmt.Println("|---------|-------------|---------------|----------|---------------|")
		lim := min(setupPeers, 15)
		for i := range lim {
			p := peers[i]
			if p.setupUpdates == 0 {
				break
			}
			fmt.Printf("| AS%d | %s | %d sec | %s | %s |\n",
				p.asn,
				formatNumber(uint64(p.setupUpdates)), //nolint:gosec // non-negative
				p.setupSeconds,
				formatNumber(uint64(p.setupRunPeak)), //nolint:gosec // non-negative
				formatNumber(uint64(p.maintUpdates))) //nolint:gosec // non-negative
		}
		fmt.Println()
	}

	// Maintenance rate distribution (per-peer-second).
	if allMaintSeconds > 0 {
		fmt.Println("== Maintenance Rate Distribution (per source peer per second) ==")
		fmt.Println()
		fmt.Println("This is the rate each individual peer sends during steady-state operation.")
		fmt.Println("These numbers directly size the per-destination-peer channel: a channel")
		fmt.Println("that absorbs the P95 maintenance rate avoids overflow during normal churn.")
		fmt.Println()
		printDensityDist("Maintenance UPD/s per peer", maintPerSec, "UPD/s", allMaintSeconds,
			"Per-second rate for individual peers during maintenance periods.\n"+
				"Most peer-seconds have very low rates (1-5 UPD/s). This is normal BGP churn.")
	}

	// Setup rate distribution.
	if allSetupSeconds > 0 {
		fmt.Println("== Setup Rate Distribution (per source peer per second) ==")
		fmt.Println()
		fmt.Println("This is the rate each individual peer sends during table dumps or convergence.")
		fmt.Println("These rates will overflow the per-peer channel immediately. The overflow pool")
		fmt.Println("and block-backed BufMux absorb the excess.")
		fmt.Println()
		printDensityDist("Setup UPD/s per peer", setupPerSec, "UPD/s", allSetupSeconds,
			"Per-second rate for individual peers during setup/convergence bursts.")
	}

	// Channel sizing recommendation.
	fmt.Println("== Channel Sizing Recommendation ==")
	fmt.Println()
	fmt.Println("The per-peer channel only needs to absorb maintenance traffic.")
	fmt.Println("Setup bursts overflow by design -- the overflow pool handles them.")
	fmt.Println()
	if allMaintSeconds > 0 {
		p50 := densityPercentile(maintPerSec, allMaintSeconds, 50)
		p95 := densityPercentile(maintPerSec, allMaintSeconds, 95)
		p99 := densityPercentile(maintPerSec, allMaintSeconds, 99)
		fmt.Println("Per-peer maintenance rate:")
		fmt.Printf("  P50: %d UPD/s, P95: %d UPD/s, P99: %d UPD/s\n\n", p50, p95, p99)
		switch {
		case p95 <= 16:
			fmt.Println("A channel of 16 absorbs P95 maintenance traffic per peer.")
		case p95 <= 64:
			fmt.Printf("A channel of %d absorbs P95 maintenance traffic per peer.\n", ((p95/16)+1)*16)
		default:
			fmt.Printf("A channel of %d absorbs P95 maintenance traffic per peer.\n", ((p95/64)+1)*64)
		}
		fmt.Println("Setup/convergence bursts will overflow to the pool (by design).")
	}
	fmt.Println()
}

// detectPeerRuns groups a per-peer time series into consecutive runs.
func detectPeerRuns(series []peerSecond) []burstRun {
	if len(series) == 0 {
		return nil
	}

	var runs []burstRun
	current := burstRun{startTS: series[0].ts, duration: 1, updates: series[0].count}

	for i := 1; i < len(series); i++ {
		gap := series[i].ts - series[i-1].ts
		if gap <= 1 {
			current.duration++
			current.updates += series[i].count
		} else {
			runs = append(runs, current)
			current = burstRun{startTS: series[i].ts, duration: 1, updates: series[i].count}
		}
	}
	runs = append(runs, current)
	return runs
}

func printDensityDist(title string, dist map[int]int, label string, total int, explanation string) {
	if len(dist) == 0 || total == 0 {
		return
	}

	fmt.Printf("== %s ==\n", title)
	fmt.Println()
	fmt.Println(explanation)
	fmt.Println()
	fmt.Printf("| %s | Count | Percent | Cumulative |\n", label)
	fmt.Println("|-------|-------|---------|------------|")

	keys := sortedIntKeys(dist)
	cumulative := 0
	for _, k := range keys {
		cumulative += dist[k]
		pct := float64(dist[k]) * 100 / float64(total)
		cum := float64(cumulative) * 100 / float64(total)
		fmt.Printf("| %d | %d | %.2f%% | %.2f%% |\n", k, dist[k], pct, cum)
	}
	fmt.Println()
}

func sortedIntKeys(m map[int]int) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

func densityMaxKey(m map[int]int) int {
	mx := 0
	for k := range m {
		if k > mx {
			mx = k
		}
	}
	return mx
}

func densityPercentile(dist map[int]int, total, pct int) int {
	target := int(math.Ceil(float64(total) * float64(pct) / 100))
	keys := sortedIntKeys(dist)
	cumulative := 0
	for _, k := range keys {
		cumulative += dist[k]
		if cumulative >= target {
			return k
		}
	}
	if len(keys) > 0 {
		return keys[len(keys)-1]
	}
	return 0
}
