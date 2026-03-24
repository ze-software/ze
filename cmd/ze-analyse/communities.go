// Design: (none -- research/analysis tool)
//
// Analyzes MRT dumps to generate per-ASN community defaults. Identifies which
// communities appear in nearly every route from a given ASN, enabling
// pre-configured caching where defaults are assumed present and only
// exceptions (absences) need encoding.
//
// Output: YAML or JSON with per-ASN defaults and savings estimates.
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
)

// commAnalysis holds all community analysis state.
type commAnalysis struct {
	Files       []string
	PeerTable   map[uint16]*mrtPeerInfo
	ASNData     map[uint32]*commASNStats
	TotalRoutes uint64
}

type commASNStats struct {
	ASN              uint32
	Routes           uint64
	Peers            []uint16
	Communities      map[uint32]uint64
	LargeCommunities map[string]uint64
	LocalPrefs       map[uint32]uint64
	TotalCommBytes   uint64
	TotalLCommBytes  uint64
}

func isActionCommunity(comm uint32) bool {
	high := comm >> 16
	return high == 0 || high == 65535 || high == 65281 || high == 65282
}

func runCommunities(args []string) int {
	fs := flag.NewFlagSet("communities", flag.ContinueOnError)
	threshold := fs.Float64("threshold", 0.95, "Minimum frequency to be considered default (0.0-1.0)")
	minRoutes := fs.Int("min-routes", 1000, "Minimum routes from ASN to generate defaults")
	format := fs.String("format", "yaml", "Output format: yaml, json")
	postPolicy := fs.Bool("post-policy", false, "Simulate post-policy view (strip action communities)")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `ze-analyse communities -- per-ASN community default analysis

Identifies communities that appear in nearly every route from a given ASN.
These "defaults" can be assumed present in a cache, encoding only exceptions.

For example, if AS64500 always attaches community 64500:100 to its routes,
the cache stores this as a default and only encodes its absence on the rare
route that lacks it. This saves 4 bytes per route per default community.

Supports both TABLE_DUMP_V2 (RIB snapshots) and BGP4MP (live updates).

Usage:
  ze-analyse communities [options] <file.gz> [file2.gz ...]

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  ze-analyse communities test/internet/latest-bview.gz
  ze-analyse communities --threshold 0.90 --format json test/internet/latest-bview.gz
  ze-analyse communities --post-policy test/internet/latest-bview.gz
`)
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() < 1 {
		fs.Usage()
		return 1
	}
	if *minRoutes < 0 {
		fmt.Fprintf(os.Stderr, "error: --min-routes must be >= 0, got %d\n", *minRoutes)
		return 1
	}

	st := &commAnalysis{
		PeerTable: make(map[uint16]*mrtPeerInfo),
		ASNData:   make(map[uint32]*commASNStats),
	}

	for _, fname := range fs.Args() {
		st.Files = append(st.Files, fname)
		if err := processMRTFile(fname, mrtHandler{
			OnPeerIndex: func(data []byte) {
				st.PeerTable = parsePeerIndexTable(data)
			},
			OnRIB: func(data []byte, subtype uint16) {
				forEachRIBEntry(data, subtype, func(peerIndex uint16, attrs []byte) {
					asn := uint32(0)
					if peer, ok := st.PeerTable[peerIndex]; ok {
						asn = peer.ASN
					}
					if asn == 0 {
						asn = uint32(peerIndex) + 0x10000
					}
					commAnalyzeRoute(attrs, asn, peerIndex, st, *postPolicy)
				})
			},
			OnBGP4MP: func(data []byte, subtype uint16, _ uint32) {
				body, peerASN := extractBGP4MPUpdate(subtype, data)
				if body == nil {
					return
				}
				attrs := extractUpdateAttrs(body)
				if attrs != nil {
					commAnalyzeRoute(attrs, peerASN, 0xFFFF, st, *postPolicy)
				}
			},
		}); err != nil {
			fmt.Fprintf(os.Stderr, "error processing %s: %v\n", fname, err)
			return 1
		}
	}

	switch *format {
	case "yaml":
		commPrintYAML(st, *threshold, *minRoutes, *postPolicy)
	case "json":
		commPrintJSON(st, *threshold, *minRoutes)
	default:
		fmt.Fprintf(os.Stderr, "unknown format: %s\n", *format)
		return 1
	}

	return 0
}

func commAnalyzeRoute(attrs []byte, asn uint32, peerIndex uint16, st *commAnalysis, postPolicy bool) {
	st.TotalRoutes++

	as, ok := st.ASNData[asn]
	if !ok {
		as = &commASNStats{
			ASN:              asn,
			Communities:      make(map[uint32]uint64),
			LargeCommunities: make(map[string]uint64),
			LocalPrefs:       make(map[uint32]uint64),
		}
		st.ASNData[asn] = as
	}
	as.Routes++

	if !slices.Contains(as.Peers, peerIndex) {
		as.Peers = append(as.Peers, peerIndex)
	}

	iterateAttrs(attrs, func(_, typeCode uint8, value []byte) {
		switch typeCode {
		case attrLocalPref:
			if len(value) >= 4 {
				as.LocalPrefs[binary.BigEndian.Uint32(value)]++
			}
		case attrCommunity:
			for i := 0; i+4 <= len(value); i += 4 {
				comm := binary.BigEndian.Uint32(value[i : i+4])
				if postPolicy && isActionCommunity(comm) {
					continue
				}
				as.TotalCommBytes += 4
				as.Communities[comm]++
			}
		case attrLargeCommunity:
			as.TotalLCommBytes += uint64(len(value)) //nolint:gosec // len is non-negative
			for i := 0; i+12 <= len(value); i += 12 {
				g := binary.BigEndian.Uint32(value[i : i+4])
				l1 := binary.BigEndian.Uint32(value[i+4 : i+8])
				l2 := binary.BigEndian.Uint32(value[i+8 : i+12])
				as.LargeCommunities[fmt.Sprintf("%d:%d:%d", g, l1, l2)]++
			}
		}
	})
}

type commFreq struct {
	value string
	count uint64
	freq  float64
}

func commPrintYAML(st *commAnalysis, threshold float64, minRoutes int, postPolicy bool) {
	fmt.Println("# Per-ASN community defaults from MRT analysis")
	fmt.Println("#")
	fmt.Println("# Communities that appear in nearly every route from a given ASN can be")
	fmt.Println("# assumed as 'defaults' in a cache. Only exceptions (routes missing a")
	fmt.Println("# default community) need explicit encoding, saving wire bytes.")
	fmt.Println("#")
	fmt.Printf("# Source: %s\n", strings.Join(st.Files, ", "))
	fmt.Printf("# Total routes analyzed: %d\n", st.TotalRoutes)
	fmt.Printf("# Threshold for defaults: %.0f%%\n", threshold*100)
	fmt.Printf("# Minimum routes per ASN: %d\n", minRoutes)
	if postPolicy {
		fmt.Println("# Mode: POST-POLICY (action communities stripped)")
	} else {
		fmt.Println("# Mode: RAW (all communities as seen by route server)")
	}
	fmt.Println()

	type asnSort struct {
		asn   uint32
		stats *commASNStats
	}
	var sorted []asnSort
	for asn, s := range st.ASNData {
		if s.Routes >= uint64(minRoutes) { //nolint:gosec // minRoutes from flag
			sorted = append(sorted, asnSort{asn, s})
		}
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].stats.Routes > sorted[j].stats.Routes })

	fmt.Println("asn_defaults:")

	var totalSavingsComm, totalSavingsLComm uint64
	var asnsWithDefaults int

	for _, as := range sorted {
		s := as.stats

		var defaults, frequent []commFreq
		for comm, count := range s.Communities {
			f := float64(count) / float64(s.Routes)
			cf := commFreq{fmtComm(comm), count, f}
			if f >= threshold {
				defaults = append(defaults, cf)
			} else if f >= 0.5 {
				frequent = append(frequent, cf)
			}
		}
		sort.Slice(defaults, func(i, j int) bool { return defaults[i].freq > defaults[j].freq })
		sort.Slice(frequent, func(i, j int) bool { return frequent[i].freq > frequent[j].freq })

		var defaultsLarge []commFreq
		for comm, count := range s.LargeCommunities {
			f := float64(count) / float64(s.Routes)
			if f >= threshold {
				defaultsLarge = append(defaultsLarge, commFreq{comm, count, f})
			}
		}
		sort.Slice(defaultsLarge, func(i, j int) bool { return defaultsLarge[i].freq > defaultsLarge[j].freq })

		if len(defaults) == 0 && len(defaultsLarge) == 0 {
			continue
		}
		asnsWithDefaults++

		var commSavings uint64
		for _, dc := range defaults {
			without := s.Routes - dc.count
			if dc.count*4 > without*5 {
				commSavings += dc.count*4 - without*5
			}
		}
		totalSavingsComm += commSavings

		var lcommSavings uint64
		for _, dc := range defaultsLarge {
			without := s.Routes - dc.count
			if dc.count*12 > without*13 {
				lcommSavings += dc.count*12 - without*13
			}
		}
		totalSavingsLComm += lcommSavings

		fmt.Printf("\n  %d:  # AS%d\n", as.asn, as.asn)
		if peer, ok := st.PeerTable[s.Peers[0]]; ok {
			fmt.Printf("    peer-ip: \"%s\"\n", peer.IP)
		}
		fmt.Printf("    routes: %d\n", s.Routes)

		if len(defaults) > 0 {
			fmt.Println("    default-communities:")
			for _, dc := range defaults {
				fmt.Printf("      - \"%s\"  # %.1f%% (%d/%d)\n", dc.value, dc.freq*100, dc.count, s.Routes)
			}
		}
		if len(defaultsLarge) > 0 {
			fmt.Println("    default-large-communities:")
			for _, dc := range defaultsLarge {
				fmt.Printf("      - \"%s\"  # %.1f%%\n", dc.value, dc.freq*100)
			}
		}
		if len(frequent) > 0 && len(frequent) <= 10 {
			fmt.Println("    frequent-communities:  # 50-95%, not defaults but common")
			for _, vc := range frequent {
				if len(frequent) <= 5 || vc.freq >= 0.7 {
					fmt.Printf("      - \"%s\"  # %.1f%%\n", vc.value, vc.freq*100)
				}
			}
		}

		if len(s.LocalPrefs) > 0 {
			var topLP uint32
			var topLPCount uint64
			for lp, count := range s.LocalPrefs {
				if count > topLPCount {
					topLP = lp
					topLPCount = count
				}
			}
			if float64(topLPCount)/float64(s.Routes)*100 >= 90 {
				fmt.Printf("    default-local-pref: %d  # %.1f%%\n", topLP, float64(topLPCount)/float64(s.Routes)*100)
			}
		}

		fmt.Println("    savings:")
		fmt.Printf("      community-bytes-saved: %d\n", commSavings)
		fmt.Printf("      large-community-bytes-saved: %d\n", lcommSavings)
		if s.TotalCommBytes > 0 {
			fmt.Printf("      community-reduction-percent: %.1f\n",
				float64(commSavings)/float64(s.TotalCommBytes)*100)
		}
	}

	// Summary.
	fmt.Println("\nsummary:")
	fmt.Printf("  total-asns-analyzed: %d\n", len(st.ASNData))
	fmt.Printf("  asns-with-defaults: %d\n", asnsWithDefaults)
	fmt.Printf("  total-routes: %d\n", st.TotalRoutes)
	fmt.Println("  estimated-savings:")
	fmt.Printf("    community-bytes: %d\n", totalSavingsComm)
	fmt.Printf("    large-community-bytes: %d\n", totalSavingsLComm)
	fmt.Printf("    total-bytes: %d\n", totalSavingsComm+totalSavingsLComm)

	var totalCommBytes, totalLCommBytes uint64
	for _, s := range st.ASNData {
		totalCommBytes += s.TotalCommBytes
		totalLCommBytes += s.TotalLCommBytes
	}
	if totalCommBytes+totalLCommBytes > 0 {
		fmt.Printf("    reduction-percent: %.1f\n",
			float64(totalSavingsComm+totalSavingsLComm)/float64(totalCommBytes+totalLCommBytes)*100)
	}
}

func commPrintJSON(st *commAnalysis, threshold float64, minRoutes int) {
	fmt.Println("{")

	// Files as JSON array (not comma-joined string).
	fmt.Print("  \"files\": [")
	for i, f := range st.Files {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Printf("%q", f)
	}
	fmt.Println("],")

	fmt.Printf("  \"total-routes\": %d,\n", st.TotalRoutes)
	fmt.Printf("  \"threshold\": %.2f,\n", threshold)
	fmt.Println("  \"asn-defaults\": {")

	type asnSort struct {
		asn   uint32
		stats *commASNStats
	}
	var sorted []asnSort
	for asn, s := range st.ASNData {
		if s.Routes >= uint64(minRoutes) { //nolint:gosec // minRoutes validated >= 0
			sorted = append(sorted, asnSort{asn, s})
		}
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].stats.Routes > sorted[j].stats.Routes })

	// Pre-filter to entries with defaults, so comma logic is correct.
	type asnEntry struct {
		asn      uint32
		routes   uint64
		defaults []string
	}
	var entries []asnEntry
	for _, as := range sorted {
		s := as.stats
		var defaults []string
		for comm, count := range s.Communities {
			if float64(count)/float64(s.Routes) >= threshold {
				defaults = append(defaults, fmtComm(comm))
			}
		}
		if len(defaults) == 0 {
			continue
		}
		sort.Strings(defaults)
		entries = append(entries, asnEntry{as.asn, s.Routes, defaults})
	}

	for idx, e := range entries {
		fmt.Printf("    \"%d\": {\"routes\": %d, \"defaults\": [", e.asn, e.routes)
		for i, c := range e.defaults {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Printf("%q", c)
		}
		fmt.Print("]}")
		if idx < len(entries)-1 {
			fmt.Print(",")
		}
		fmt.Println()
	}

	fmt.Println("  }")
	fmt.Println("}")
}
