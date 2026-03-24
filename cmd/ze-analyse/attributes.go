// Design: (none -- research/analysis tool)
//
// Analyzes MRT dumps for BGP attribute repetition patterns to guide caching
// optimization decisions. Measures per-attribute uniqueness, bundle dedup rates,
// temporal locality (consecutive hit rates), and per-peer community extraction.
//
// Output: JSON to stdout, human summary to stderr.
package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"sort"
	"strings"
)

// attrNames maps BGP attribute type codes to human-readable names.
var attrNames = map[uint8]string{
	attrOrigin:          "ORIGIN",
	attrASPath:          "AS_PATH",
	attrNextHop:         "NEXT_HOP",
	attrMED:             "MED",
	attrLocalPref:       "LOCAL_PREF",
	attrAtomicAggregate: "ATOMIC_AGGREGATE",
	attrAggregator:      "AGGREGATOR",
	attrCommunity:       "COMMUNITY",
	attrOriginatorID:    "ORIGINATOR_ID",
	attrClusterList:     "CLUSTER_LIST",
	attrMPReachNLRI:     "MP_REACH_NLRI",
	attrMPUnreachNLRI:   "MP_UNREACH_NLRI",
	attrExtCommunity:    "EXT_COMMUNITY",
	attrAS4Path:         "AS4_PATH",
	attrAS4Aggregator:   "AS4_AGGREGATOR",
	attrLargeCommunity:  "LARGE_COMMUNITY",
	attrOTC:             "OTC",
}

// attrAnalysis holds all analysis state.
type attrAnalysis struct {
	Files        []string
	TotalUpdates uint64

	Attributes        map[uint8]*attrTypeStats
	Bundles           *bundleStats
	BundlesWithASPath *bundleStats
	BundlesNoComm     *bundleStats
	BundlesMinimal    *bundleStats
	Peers             map[uint16]*attrPeerStats
	PeerTable         map[uint16]*mrtPeerInfo

	ConsecutiveHits        uint64
	ConsecutiveHitsWithAS  uint64
	ConsecutiveHitsNoComm  uint64
	ConsecutiveHitsMinimal uint64
	prevHash               uint64
	prevHashWithAS         uint64
	prevHashNoComm         uint64
	prevHashMinimal        uint64
	hasPrev                bool

	currentRunLength uint64
	RunLengths       map[string]uint64

	GlobalCommunities         map[uint32]uint64
	GlobalLargeCommunities    map[string]uint64
	GlobalExtCommunities      map[uint64]uint64
	UpdatesWithCommunity      uint64
	UpdatesWithLargeCommunity uint64
}

type attrTypeStats struct {
	Code   uint8
	Name   string
	Total  uint64
	Bytes  uint64
	Values map[uint64]uint64
}

type bundleStats struct {
	Total  uint64
	Values map[uint64]uint64
}

type attrPeerStats struct {
	Info             *mrtPeerInfo
	Updates          uint64
	Bundles          map[uint64]uint64
	LocalPrefs       map[uint32]uint64
	MEDs             map[uint32]uint64
	Communities      map[uint32]uint64
	LargeCommunities map[string]uint64
	ExtCommunities   map[uint64]uint64
}

func newAttrAnalysis() *attrAnalysis {
	return &attrAnalysis{
		Attributes:             make(map[uint8]*attrTypeStats),
		Bundles:                &bundleStats{Values: make(map[uint64]uint64)},
		BundlesWithASPath:      &bundleStats{Values: make(map[uint64]uint64)},
		BundlesNoComm:          &bundleStats{Values: make(map[uint64]uint64)},
		BundlesMinimal:         &bundleStats{Values: make(map[uint64]uint64)},
		Peers:                  make(map[uint16]*attrPeerStats),
		PeerTable:              make(map[uint16]*mrtPeerInfo),
		RunLengths:             make(map[string]uint64),
		GlobalCommunities:      make(map[uint32]uint64),
		GlobalLargeCommunities: make(map[string]uint64),
		GlobalExtCommunities:   make(map[uint64]uint64),
	}
}

func newAttrPeerStats() *attrPeerStats {
	return &attrPeerStats{
		Bundles:          make(map[uint64]uint64),
		LocalPrefs:       make(map[uint32]uint64),
		MEDs:             make(map[uint32]uint64),
		Communities:      make(map[uint32]uint64),
		LargeCommunities: make(map[string]uint64),
		ExtCommunities:   make(map[uint64]uint64),
	}
}

func runAttributes(args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, `ze-analyse attributes -- BGP attribute repetition analysis

Analyzes MRT files to measure how often BGP path attributes repeat across routes.
High repetition means caching pays off; low repetition means each route is unique.

Processes both TABLE_DUMP_V2 (RIB snapshots) and BGP4MP (live updates).
JSON output to stdout; human-readable summary to stderr.

Usage:
  ze-analyse attributes <file.gz> [file2.gz ...]

Examples:
  ze-analyse attributes test/internet/latest-bview.gz 2>/dev/null | jq .
  ze-analyse attributes test/internet/latest-bview.gz >/dev/null
`)
		return 1
	}

	st := newAttrAnalysis()

	for _, fname := range args {
		st.Files = append(st.Files, fname)
		if err := processMRTFile(fname, mrtHandler{
			OnPeerIndex: func(data []byte) {
				st.PeerTable = parsePeerIndexTable(data)
			},
			OnRIB: func(data []byte, subtype uint16) {
				forEachRIBEntry(data, subtype, func(peerIndex uint16, attrs []byte) {
					attrAnalyzeRoute(attrs, peerIndex, st)
				})
			},
			OnBGP4MP: func(data []byte, subtype uint16, _ uint32) {
				body, _ := extractBGP4MPUpdate(subtype, data)
				if body == nil {
					return
				}
				attrs := extractUpdateAttrs(body)
				if attrs != nil {
					attrAnalyzeRoute(attrs, 0xFFFF, st)
				}
			},
		}); err != nil {
			fmt.Fprintf(os.Stderr, "error processing %s: %v\n", fname, err)
			return 1
		}
	}

	attrPrintJSON(os.Stdout, st)
	attrPrintSummary(os.Stderr, st)

	return 0
}

func attrAnalyzeRoute(attrs []byte, peerIndex uint16, st *attrAnalysis) {
	st.TotalUpdates++

	peer, ok := st.Peers[peerIndex]
	if !ok {
		peer = newAttrPeerStats()
		st.Peers[peerIndex] = peer
		if info, exists := st.PeerTable[peerIndex]; exists {
			peer.Info = info
		}
	}
	peer.Updates++

	hAll := fnv.New64a()
	hWithAS := fnv.New64a()
	hNoComm := fnv.New64a()
	hMinimal := fnv.New64a()

	iterateAttrs(attrs, func(_, typeCode uint8, value []byte) {
		as, ok := st.Attributes[typeCode]
		if !ok {
			name := attrNames[typeCode]
			if name == "" {
				name = fmt.Sprintf("UNKNOWN_%d", typeCode)
			}
			as = &attrTypeStats{Code: typeCode, Name: name, Values: make(map[uint64]uint64)}
			st.Attributes[typeCode] = as
		}
		as.Total++
		as.Bytes += uint64(len(value)) //nolint:gosec // len is non-negative

		vh := fnv.New64a()
		vh.Write(value) //nolint:errcheck // fnv never fails
		as.Values[vh.Sum64()]++

		if typeCode != attrMPReachNLRI && typeCode != attrMPUnreachNLRI {
			hWithAS.Write([]byte{typeCode}) //nolint:errcheck // fnv never fails
			hWithAS.Write(value)            //nolint:errcheck // fnv never fails
		}
		if typeCode != attrASPath && typeCode != attrAS4Path &&
			typeCode != attrMPReachNLRI && typeCode != attrMPUnreachNLRI {
			hAll.Write([]byte{typeCode}) //nolint:errcheck // fnv never fails
			hAll.Write(value)            //nolint:errcheck // fnv never fails
		}
		if typeCode != attrASPath && typeCode != attrAS4Path &&
			typeCode != attrMPReachNLRI && typeCode != attrMPUnreachNLRI &&
			typeCode != attrCommunity && typeCode != attrLargeCommunity && typeCode != attrExtCommunity {
			hNoComm.Write([]byte{typeCode}) //nolint:errcheck // fnv never fails
			hNoComm.Write(value)            //nolint:errcheck // fnv never fails
		}
		if typeCode == attrOrigin || typeCode == attrNextHop || typeCode == attrLocalPref {
			hMinimal.Write([]byte{typeCode}) //nolint:errcheck // fnv never fails
			hMinimal.Write(value)            //nolint:errcheck // fnv never fails
		}

		attrExtractCommunities(typeCode, value, peer, st)
	})

	bh := hAll.Sum64()
	bhAS := hWithAS.Sum64()
	bhNC := hNoComm.Sum64()
	bhM := hMinimal.Sum64()

	st.Bundles.Total++
	st.Bundles.Values[bh]++
	st.BundlesWithASPath.Total++
	st.BundlesWithASPath.Values[bhAS]++
	st.BundlesNoComm.Total++
	st.BundlesNoComm.Values[bhNC]++
	st.BundlesMinimal.Total++
	st.BundlesMinimal.Values[bhM]++
	peer.Bundles[bh]++

	if st.hasPrev {
		if bh == st.prevHash {
			st.ConsecutiveHits++
		}
		if bhAS == st.prevHashWithAS {
			st.ConsecutiveHitsWithAS++
		}
		if bhNC == st.prevHashNoComm {
			st.ConsecutiveHitsNoComm++
		}
		if bhM == st.prevHashMinimal {
			st.ConsecutiveHitsMinimal++
			st.currentRunLength++
		} else {
			if st.currentRunLength > 0 {
				st.RunLengths[runBucket(st.currentRunLength)]++
			}
			st.currentRunLength = 0
		}
	}
	st.prevHash = bh
	st.prevHashWithAS = bhAS
	st.prevHashNoComm = bhNC
	st.prevHashMinimal = bhM
	st.hasPrev = true
}

func attrExtractCommunities(typeCode uint8, value []byte, peer *attrPeerStats, st *attrAnalysis) {
	switch typeCode {
	case attrLocalPref:
		if len(value) >= 4 {
			peer.LocalPrefs[binary.BigEndian.Uint32(value)]++
		}
	case attrMED:
		if len(value) >= 4 {
			peer.MEDs[binary.BigEndian.Uint32(value)]++
		}
	case attrCommunity:
		st.UpdatesWithCommunity++
		for i := 0; i+4 <= len(value); i += 4 {
			c := binary.BigEndian.Uint32(value[i : i+4])
			peer.Communities[c]++
			st.GlobalCommunities[c]++
		}
	case attrLargeCommunity:
		st.UpdatesWithLargeCommunity++
		for i := 0; i+12 <= len(value); i += 12 {
			g := binary.BigEndian.Uint32(value[i : i+4])
			l1 := binary.BigEndian.Uint32(value[i+4 : i+8])
			l2 := binary.BigEndian.Uint32(value[i+8 : i+12])
			key := fmt.Sprintf("%d:%d:%d", g, l1, l2)
			peer.LargeCommunities[key]++
			st.GlobalLargeCommunities[key]++
		}
	case attrExtCommunity:
		for i := 0; i+8 <= len(value); i += 8 {
			ext := binary.BigEndian.Uint64(value[i : i+8])
			peer.ExtCommunities[ext]++
			st.GlobalExtCommunities[ext]++
		}
	}
}

func runBucket(length uint64) string {
	switch {
	case length <= 5:
		return fmt.Sprintf("%d", length)
	case length <= 10:
		return "6-10"
	case length <= 20:
		return "11-20"
	default:
		return "21+"
	}
}

func fmtComm(c uint32) string {
	return fmt.Sprintf("%d:%d", c>>16, c&0xFFFF)
}

// JSON output types -- kebab-case tags per ze conventions.
type attrJSONOutput struct {
	Files             []string                      `json:"files"`
	TotalUpdates      uint64                        `json:"total-updates"`
	Attributes        map[string]*attrJSONAttr      `json:"attributes"`
	Bundles           *attrJSONBundle               `json:"bundles"`
	BundlesWithASPath *attrJSONBundle               `json:"bundles-with-aspath"`
	ConsecutiveHits   *attrJSONConsec               `json:"consecutive-hits"`
	PerPeer           map[string]*attrJSONPeer      `json:"per-peer"`
	CommunityData     map[string]*attrJSONCommunity `json:"community-extraction"`
}

type attrJSONAttr struct {
	Code         uint8   `json:"code"`
	Total        uint64  `json:"total"`
	Unique       uint64  `json:"unique"`
	Bytes        uint64  `json:"bytes"`
	CacheHitRate float64 `json:"cache-hit-rate"`
}

type attrJSONBundle struct {
	Total        uint64  `json:"total"`
	Unique       uint64  `json:"unique"`
	CacheHitRate float64 `json:"cache-hit-rate"`
}

type attrJSONConsec struct {
	WithoutASPath     uint64  `json:"without-aspath"`
	WithoutASPathRate float64 `json:"without-aspath-rate"`
	WithASPath        uint64  `json:"with-aspath"`
	WithASPathRate    float64 `json:"with-aspath-rate"`
}

type attrJSONPeer struct {
	Updates       uint64  `json:"updates"`
	UniqueBundles uint64  `json:"unique-bundles"`
	CacheHitRate  float64 `json:"cache-hit-rate"`
}

type attrJSONCommunity struct {
	Updates         uint64         `json:"updates"`
	SessionConstant []attrJSONFreq `json:"session-constant"`
	Variable        []attrJSONFreq `json:"variable"`
	SavingsBytes    uint64         `json:"potential-savings-bytes"`
}

type attrJSONFreq struct {
	Community string  `json:"community"`
	Count     uint64  `json:"count"`
	Frequency float64 `json:"frequency"`
}

func attrPrintJSON(w io.Writer, st *attrAnalysis) {
	out := &attrJSONOutput{
		Files:         st.Files,
		TotalUpdates:  st.TotalUpdates,
		Attributes:    make(map[string]*attrJSONAttr),
		PerPeer:       make(map[string]*attrJSONPeer),
		CommunityData: make(map[string]*attrJSONCommunity),
	}

	for code, a := range st.Attributes {
		unique := uint64(len(a.Values))
		hr := 0.0
		if a.Total > 0 {
			hr = float64(a.Total-unique) / float64(a.Total)
		}
		out.Attributes[a.Name] = &attrJSONAttr{Code: code, Total: a.Total, Unique: unique, Bytes: a.Bytes, CacheHitRate: hr}
	}

	out.Bundles = jsonBundle(st.Bundles)
	out.BundlesWithASPath = jsonBundle(st.BundlesWithASPath)

	crW, crA := 0.0, 0.0
	if st.TotalUpdates > 1 {
		comps := float64(st.TotalUpdates - 1)
		crW = float64(st.ConsecutiveHits) / comps
		crA = float64(st.ConsecutiveHitsWithAS) / comps
	}
	out.ConsecutiveHits = &attrJSONConsec{
		WithoutASPath: st.ConsecutiveHits, WithoutASPathRate: crW,
		WithASPath: st.ConsecutiveHitsWithAS, WithASPathRate: crA,
	}

	for idx, p := range st.Peers {
		ub := uint64(len(p.Bundles))
		hr := 0.0
		if p.Updates > 0 {
			hr = float64(p.Updates-ub) / float64(p.Updates)
		}
		out.PerPeer[fmt.Sprintf("%d", idx)] = &attrJSONPeer{Updates: p.Updates, UniqueBundles: ub, CacheHitRate: hr}

		ce := &attrJSONCommunity{Updates: p.Updates}
		var savings uint64
		for c, cnt := range p.Communities {
			f := float64(cnt) / float64(p.Updates)
			jf := attrJSONFreq{Community: fmtComm(c), Count: cnt, Frequency: f}
			if f >= 0.9 {
				ce.SessionConstant = append(ce.SessionConstant, jf)
				savings += (cnt - 1) * 4
			} else {
				ce.Variable = append(ce.Variable, jf)
			}
		}
		for lc, cnt := range p.LargeCommunities {
			f := float64(cnt) / float64(p.Updates)
			jf := attrJSONFreq{Community: lc, Count: cnt, Frequency: f}
			if f >= 0.9 {
				ce.SessionConstant = append(ce.SessionConstant, jf)
				savings += (cnt - 1) * 12
			} else {
				ce.Variable = append(ce.Variable, jf)
			}
		}
		if len(ce.Variable) > 10 {
			sort.Slice(ce.Variable, func(i, j int) bool { return ce.Variable[i].Frequency > ce.Variable[j].Frequency })
			ce.Variable = ce.Variable[:10]
		}
		ce.SavingsBytes = savings
		out.CommunityData[fmt.Sprintf("%d", idx)] = ce
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "error encoding JSON: %v\n", err)
	}
}

func jsonBundle(bs *bundleStats) *attrJSONBundle {
	u := uint64(len(bs.Values))
	hr := 0.0
	if bs.Total > 0 {
		hr = float64(bs.Total-u) / float64(bs.Total)
	}
	return &attrJSONBundle{Total: bs.Total, Unique: u, CacheHitRate: hr}
}

func attrPrintSummary(w io.Writer, st *attrAnalysis) {
	wf(w, "\n=== MRT Attribute Analysis ===\n")
	wf(w, "\nThis analysis shows how often BGP path attributes repeat across routes.\n")
	wf(w, "High cache hit rates mean deduplication is effective; temporal locality\n")
	wf(w, "shows whether a single-entry cache (last-seen) is enough.\n\n")
	wf(w, "Files: %s\n", strings.Join(st.Files, ", "))
	wf(w, "Total UPDATEs: %s\n\n", formatNumber(st.TotalUpdates))

	type row struct {
		code    uint8
		name    string
		unique  uint64
		total   uint64
		bytes   uint64
		hitRate float64
	}
	var rows []row
	for code, a := range st.Attributes {
		u := uint64(len(a.Values))
		hr := 0.0
		if a.Total > 0 {
			hr = float64(a.Total-u) / float64(a.Total) * 100
		}
		rows = append(rows, row{code, a.Name, u, a.Total, a.Bytes, hr})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].hitRate > rows[j].hitRate })

	wf(w, "ATTRIBUTE REPETITION (sorted by cache hit rate):\n")
	wf(w, "  %-18s %12s %12s %9s %12s\n", "Attr", "Unique", "Total", "Hit Rate", "Bytes")
	wf(w, "  %s\n", strings.Repeat("-", 67))
	for _, r := range rows {
		m := ""
		if r.code == attrASPath {
			m = " <- verify"
		}
		wf(w, "  %-18s %12s %12s %8.3f%% %12s%s\n",
			r.name, formatNumber(r.unique), formatNumber(r.total), r.hitRate, formatBytes(r.bytes), m)
	}

	wf(w, "\nBUNDLE ANALYSIS:\n")
	printBundleSummary(w, "Without AS_PATH", st.Bundles)
	printBundleSummary(w, "With AS_PATH", st.BundlesWithASPath)

	comps := st.TotalUpdates - 1
	crWith, crWithout, crNoComm, crMinimal := 0.0, 0.0, 0.0, 0.0
	if comps > 0 {
		crWith = float64(st.ConsecutiveHitsWithAS) / float64(comps) * 100
		crWithout = float64(st.ConsecutiveHits) / float64(comps) * 100
		crNoComm = float64(st.ConsecutiveHitsNoComm) / float64(comps) * 100
		crMinimal = float64(st.ConsecutiveHitsMinimal) / float64(comps) * 100
	}
	wf(w, "\nCONSECUTIVE HIT ANALYSIS (temporal locality):\n")
	wf(w, "  If we cache just the LAST bundle seen, what is the hit rate?\n")
	wf(w, "\n  %-45s %8s\n", "Exclusion Strategy", "Rate")
	wf(w, "  %s\n", strings.Repeat("-", 55))
	wf(w, "  %-45s %7.2f%%\n", "All real attributes (incl AS_PATH)", crWith)
	wf(w, "  %-45s %7.2f%%\n", "Exclude AS_PATH", crWithout)
	wf(w, "  %-45s %7.2f%%\n", "Exclude AS_PATH + Communities", crNoComm)
	wf(w, "  %-45s %7.2f%%\n", "Minimal (only ORIGIN + NEXT_HOP + LOCAL_PREF)", crMinimal)

	if st.currentRunLength > 0 {
		st.RunLengths[runBucket(st.currentRunLength)]++
	}
	wf(w, "\n  Run length distribution (minimal bundle consecutive hits):\n")
	for _, b := range []string{"1", "2", "3", "4", "5", "6-10", "11-20", "21+"} {
		if c := st.RunLengths[b]; c > 0 {
			wf(w, "  %-12s %12s\n", b, formatNumber(c))
		}
	}

	wf(w, "\nPER-PEER ANALYSIS (top 10 by volume):\n")
	type pa struct {
		idx     uint16
		asn     uint32
		ip      string
		updates uint64
		hitRate float64
	}
	var peers []pa
	for idx, p := range st.Peers {
		a := pa{idx: idx, updates: p.Updates}
		ub := uint64(len(p.Bundles))
		if p.Updates > 0 {
			a.hitRate = float64(p.Updates-ub) / float64(p.Updates) * 100
		}
		if p.Info != nil {
			a.asn = p.Info.ASN
			a.ip = p.Info.IP.String()
		}
		peers = append(peers, a)
	}
	sort.Slice(peers, func(i, j int) bool { return peers[i].updates > peers[j].updates })
	lim := min(len(peers), 10)
	for i := range lim {
		p := peers[i]
		asnStr := "unknown"
		if p.asn != 0 {
			asnStr = fmt.Sprintf("AS%d", p.asn)
		}
		wf(w, "  Peer %d: %s (%s) -- %s routes, %.1f%% bundle hit rate\n",
			p.idx, asnStr, p.ip, formatNumber(p.updates), p.hitRate)
	}

	wf(w, "\n")
}

func printBundleSummary(w io.Writer, label string, bs *bundleStats) {
	u := uint64(len(bs.Values))
	hr := 0.0
	if bs.Total > 0 {
		hr = float64(bs.Total-u) / float64(bs.Total) * 100
	}
	wf(w, "  %s: %s unique / %s total (%.2f%% hit rate)\n",
		label, formatNumber(u), formatNumber(bs.Total), hr)
}
