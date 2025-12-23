// mrt-defaults analyzes MRT dumps to generate per-ASN community defaults
// that can be used for pre-configured caching in BGP implementations.
//
// It parses the PEER_INDEX_TABLE to map peer_index to ASN, then analyzes
// which communities appear frequently per ASN to identify "defaults" that
// could be assumed present (with negative markers for exceptions).
//
// Output: YAML configuration file with per-ASN defaults and savings estimates
package main

import (
	"compress/gzip"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strings"
)

// MRT types (RFC 6396).
const (
	MRT_TABLE_DUMP_V2 = 13
	MRT_BGP4MP        = 16
	MRT_BGP4MP_ET     = 17
)

// TABLE_DUMP_V2 subtypes.
const (
	PEER_INDEX_TABLE   = 1
	RIB_IPV4_UNICAST   = 2
	RIB_IPV4_MULTICAST = 3
	RIB_IPV6_UNICAST   = 4
	RIB_IPV6_MULTICAST = 5
	RIB_GENERIC        = 6
)

// BGP4MP subtypes.
const (
	BGP4MP_STATE_CHANGE      = 0
	BGP4MP_MESSAGE           = 1
	BGP4MP_MESSAGE_AS4       = 4
	BGP4MP_STATE_CHANGE_AS4  = 5
	BGP4MP_MESSAGE_LOCAL     = 6
	BGP4MP_MESSAGE_AS4_LOCAL = 7
)

// BGP attribute types.
const (
	ATTR_LOCAL_PREF      = 5
	ATTR_COMMUNITY       = 8
	ATTR_LARGE_COMMUNITY = 32
)

// PeerInfo holds info from PEER_INDEX_TABLE.
type PeerInfo struct {
	Index  uint16
	BGP_ID net.IP
	IP     net.IP
	ASN    uint32
	IsIPv6 bool
	IsAS4  bool
}

// ASNStats holds per-ASN statistics.
type ASNStats struct {
	ASN              uint32
	Routes           uint64
	Peers            []uint16          // peer indices for this ASN
	Communities      map[uint32]uint64 // community -> count
	LargeCommunities map[string]uint64 // large community -> count
	LocalPrefs       map[uint32]uint64 // local_pref -> count
	TotalCommBytes   uint64            // total bytes in COMMUNITY attrs
	TotalLCommBytes  uint64            // total bytes in LARGE_COMMUNITY attrs
}

// Stats holds all analysis data.
type Stats struct {
	Files       []string
	PeerTable   map[uint16]*PeerInfo // peer_index -> PeerInfo
	ASNData     map[uint32]*ASNStats // ASN -> ASNStats
	TotalRoutes uint64
}

// Config options.
var (
	threshold  = flag.Float64("threshold", 0.95, "Minimum frequency to be considered default (0.0-1.0)")
	minRoutes  = flag.Int("min-routes", 1000, "Minimum routes from ASN to generate defaults")
	format     = flag.String("format", "yaml", "Output format: yaml, json")
	postPolicy = flag.Bool("post-policy", false, "Simulate post-policy view (strip action communities)")
)

// Action community ranges to strip in post-policy mode
// 0:XXXXX       - no-export to specific ASN
// 65535:0-65535 - well-known (NO_EXPORT, NO_ADVERTISE, etc.)
// 65281:XXXXX   - NO_EXPORT sub-confederation (RFC 5765)
// 65282:XXXXX   - NO_PEER (RFC 3765)
func isActionCommunity(comm uint32) bool {
	high := comm >> 16
	// 0:X = no-export to ASN X
	if high == 0 {
		return true
	}
	// 65535:X = well-known communities
	if high == 65535 {
		return true
	}
	// 65281:X and 65282:X = RFC action communities
	if high == 65281 || high == 65282 {
		return true
	}
	return false
}

func main() {
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <file.gz> [file2.gz ...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nGenerates per-ASN community defaults from MRT files.\n")
		fmt.Fprintf(os.Stderr, "Supports both TABLE_DUMP_V2 (RIB dumps) and BGP4MP (updates) formats.\n")
		fmt.Fprintf(os.Stderr, "Format is auto-detected from file contents.\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	stats := &Stats{
		PeerTable: make(map[uint16]*PeerInfo),
		ASNData:   make(map[uint32]*ASNStats),
	}

	for _, filename := range flag.Args() {
		stats.Files = append(stats.Files, filename)
		if err := processMRT(filename, stats); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing %s: %v\n", filename, err)
			os.Exit(1)
		}
	}

	// Output results
	switch *format {
	case "yaml":
		printYAML(stats)
	case "json":
		printJSON(stats)
	default:
		fmt.Fprintf(os.Stderr, "Unknown format: %s\n", *format)
		os.Exit(1)
	}
}

func processMRT(filename string, stats *Stats) error {
	f, err := os.Open(filename) // #nosec G304 -- filename from CLI args
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	var reader io.Reader = f
	if strings.HasSuffix(filename, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer func() { _ = gz.Close() }()
		reader = gz
	}

	header := make([]byte, 12)
	for {
		_, err := io.ReadFull(reader, header)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading header: %w", err)
		}

		mrtType := binary.BigEndian.Uint16(header[4:6])
		subtype := binary.BigEndian.Uint16(header[6:8])
		length := binary.BigEndian.Uint32(header[8:12])

		data := make([]byte, length)
		_, err = io.ReadFull(reader, data)
		if err != nil {
			return fmt.Errorf("reading data: %w", err)
		}

		switch mrtType {
		case MRT_TABLE_DUMP_V2:
			switch subtype {
			case PEER_INDEX_TABLE:
				parsePeerIndexTable(data, stats)
			case RIB_IPV4_UNICAST, RIB_IPV4_MULTICAST:
				processRIBEntry(data, stats)
			case RIB_IPV6_UNICAST, RIB_IPV6_MULTICAST:
				processRIBEntry(data, stats)
			case RIB_GENERIC:
				processRIBGeneric(data, stats)
			}
		case MRT_BGP4MP, MRT_BGP4MP_ET:
			offset := 0
			if mrtType == MRT_BGP4MP_ET {
				offset = 4 // skip microseconds
			}
			processBGP4MP(subtype, data[offset:], stats)
		}
	}
	return nil
}

func parsePeerIndexTable(data []byte, stats *Stats) {
	if len(data) < 6 {
		return
	}

	// Collector BGP ID (4 bytes)
	offset := 4

	// View Name Length (2 bytes) + View Name
	if offset+2 > len(data) {
		return
	}
	viewNameLen := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2 + int(viewNameLen)

	// Peer Count (2 bytes)
	if offset+2 > len(data) {
		return
	}
	peerCount := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2

	// Parse each peer entry
	for i := uint16(0); i < peerCount; i++ {
		if offset+1 > len(data) {
			break
		}

		peerType := data[offset]
		offset++

		isIPv6 := (peerType & 0x01) != 0
		isAS4 := (peerType & 0x02) != 0

		// Peer BGP ID (4 bytes)
		if offset+4 > len(data) {
			break
		}
		bgpID := net.IP(data[offset : offset+4])
		offset += 4

		// Peer IP Address (4 or 16 bytes)
		ipLen := 4
		if isIPv6 {
			ipLen = 16
		}
		if offset+ipLen > len(data) {
			break
		}
		peerIP := net.IP(data[offset : offset+ipLen])
		offset += ipLen

		// Peer AS (2 or 4 bytes)
		var asn uint32
		if isAS4 {
			if offset+4 > len(data) {
				break
			}
			asn = binary.BigEndian.Uint32(data[offset : offset+4])
			offset += 4
		} else {
			if offset+2 > len(data) {
				break
			}
			asn = uint32(binary.BigEndian.Uint16(data[offset : offset+2]))
			offset += 2
		}

		stats.PeerTable[i] = &PeerInfo{
			Index:  i,
			BGP_ID: bgpID,
			IP:     peerIP,
			ASN:    asn,
			IsIPv6: isIPv6,
			IsAS4:  isAS4,
		}
	}
}

func processRIBEntry(data []byte, stats *Stats) {
	if len(data) < 7 {
		return
	}

	prefixLen := data[4]
	prefixBytes := (int(prefixLen) + 7) / 8
	offset := 5 + prefixBytes

	if offset+2 > len(data) {
		return
	}

	entryCount := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2

	for i := uint16(0); i < entryCount; i++ {
		if offset+8 > len(data) {
			break
		}

		peerIndex := binary.BigEndian.Uint16(data[offset : offset+2])
		attrLen := binary.BigEndian.Uint16(data[offset+6 : offset+8])
		offset += 8

		if offset+int(attrLen) > len(data) {
			break
		}

		attrs := data[offset : offset+int(attrLen)]
		offset += int(attrLen)

		// Get ASN for this peer
		asn := uint32(0)
		if peer, ok := stats.PeerTable[peerIndex]; ok {
			asn = peer.ASN
		}
		if asn == 0 {
			asn = uint32(peerIndex) + 0x10000 // fallback: use peer_index as fake ASN
		}

		analyzeRoute(attrs, asn, peerIndex, stats)
	}
}

func processRIBGeneric(data []byte, stats *Stats) {
	if len(data) < 11 {
		return
	}

	nlriLen := data[7]
	offset := 8 + int(nlriLen)

	if offset+2 > len(data) {
		return
	}

	entryCount := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2

	for i := uint16(0); i < entryCount; i++ {
		if offset+8 > len(data) {
			break
		}

		peerIndex := binary.BigEndian.Uint16(data[offset : offset+2])
		attrLen := binary.BigEndian.Uint16(data[offset+6 : offset+8])
		offset += 8

		if offset+int(attrLen) > len(data) {
			break
		}

		attrs := data[offset : offset+int(attrLen)]
		offset += int(attrLen)

		asn := uint32(0)
		if peer, ok := stats.PeerTable[peerIndex]; ok {
			asn = peer.ASN
		}
		if asn == 0 {
			asn = uint32(peerIndex) + 0x10000
		}

		analyzeRoute(attrs, asn, peerIndex, stats)
	}
}

func processBGP4MP(subtype uint16, data []byte, stats *Stats) {
	var asSize int

	switch subtype {
	case BGP4MP_MESSAGE, BGP4MP_MESSAGE_LOCAL:
		asSize = 2
	case BGP4MP_MESSAGE_AS4, BGP4MP_MESSAGE_AS4_LOCAL:
		asSize = 4
	default:
		return // Skip state changes
	}

	// BGP4MP header: peer_as(2/4) + local_as(2/4) + iface(2) + afi(2)
	minLen := asSize*2 + 4
	if len(data) < minLen {
		return
	}

	// Extract peer ASN
	var peerASN uint32
	if asSize == 4 {
		peerASN = binary.BigEndian.Uint32(data[0:4])
	} else {
		peerASN = uint32(binary.BigEndian.Uint16(data[0:2]))
	}

	afi := binary.BigEndian.Uint16(data[asSize*2+2 : asSize*2+4])

	// peer_ip + local_ip
	ipSize := 4
	if afi == 2 { // IPv6
		ipSize = 16
	}
	offset := minLen + ipSize*2

	if offset+19 > len(data) {
		return
	}

	// BGP message: marker(16) + length(2) + type(1) + body
	offset += 16 // skip marker
	msgLen := binary.BigEndian.Uint16(data[offset : offset+2])
	msgType := data[offset+2]
	offset += 3

	// Only process UPDATE messages (type 2)
	if msgType != 2 {
		return
	}

	bodyLen := int(msgLen) - 19 // subtract header
	if offset+bodyLen > len(data) {
		return
	}

	// Extract attributes from UPDATE body
	updateBody := data[offset : offset+bodyLen]
	attrs := extractAttrsFromUpdate(updateBody)
	if attrs == nil {
		return
	}

	// Use peer ASN directly, use 0xFFFF as fake peer index
	analyzeRoute(attrs, peerASN, 0xFFFF, stats)
}

func extractAttrsFromUpdate(update []byte) []byte {
	if len(update) < 4 {
		return nil
	}

	// UPDATE: withdrawn_len(2) + withdrawn + attrs_len(2) + attrs + nlri
	wdLen := binary.BigEndian.Uint16(update[0:2])
	offset := 2 + int(wdLen)
	if offset+2 > len(update) {
		return nil
	}

	attrLen := binary.BigEndian.Uint16(update[offset : offset+2])
	offset += 2
	if offset+int(attrLen) > len(update) {
		return nil
	}

	return update[offset : offset+int(attrLen)]
}

func analyzeRoute(attrs []byte, asn uint32, peerIndex uint16, stats *Stats) {
	stats.TotalRoutes++

	// Get or create ASN stats
	asnStats, ok := stats.ASNData[asn]
	if !ok {
		asnStats = &ASNStats{
			ASN:              asn,
			Communities:      make(map[uint32]uint64),
			LargeCommunities: make(map[string]uint64),
			LocalPrefs:       make(map[uint32]uint64),
		}
		stats.ASNData[asn] = asnStats
	}
	asnStats.Routes++

	// Track peer indices for this ASN
	found := false
	for _, p := range asnStats.Peers {
		if p == peerIndex {
			found = true
			break
		}
	}
	if !found {
		asnStats.Peers = append(asnStats.Peers, peerIndex)
	}

	// Parse attributes
	offset := 0
	for offset < len(attrs) {
		if offset+3 > len(attrs) {
			break
		}

		flags := attrs[offset]
		typeCode := attrs[offset+1]
		offset += 2

		var attrLen int
		if flags&0x10 != 0 {
			if offset+2 > len(attrs) {
				break
			}
			attrLen = int(binary.BigEndian.Uint16(attrs[offset : offset+2]))
			offset += 2
		} else {
			if offset+1 > len(attrs) {
				break
			}
			attrLen = int(attrs[offset])
			offset++
		}

		if offset+attrLen > len(attrs) {
			break
		}

		attrValue := attrs[offset : offset+attrLen]
		offset += attrLen

		switch typeCode {
		case ATTR_LOCAL_PREF:
			if len(attrValue) >= 4 {
				lp := binary.BigEndian.Uint32(attrValue)
				asnStats.LocalPrefs[lp]++
			}

		case ATTR_COMMUNITY:
			for i := 0; i+4 <= len(attrValue); i += 4 {
				comm := binary.BigEndian.Uint32(attrValue[i : i+4])
				// In post-policy mode, skip action communities (they'd be consumed by RS)
				if *postPolicy && isActionCommunity(comm) {
					continue
				}
				asnStats.TotalCommBytes += 4
				asnStats.Communities[comm]++
			}

		case ATTR_LARGE_COMMUNITY:
			asnStats.TotalLCommBytes += uint64(attrLen) // #nosec G115 -- attrLen is small
			for i := 0; i+12 <= len(attrValue); i += 12 {
				global := binary.BigEndian.Uint32(attrValue[i : i+4])
				local1 := binary.BigEndian.Uint32(attrValue[i+4 : i+8])
				local2 := binary.BigEndian.Uint32(attrValue[i+8 : i+12])
				key := fmt.Sprintf("%d:%d:%d", global, local1, local2)
				asnStats.LargeCommunities[key]++
			}
		}
	}
}

func formatCommunity(comm uint32) string {
	return fmt.Sprintf("%d:%d", comm>>16, comm&0xFFFF)
}

type commFreq struct {
	value string
	count uint64
	freq  float64
}

func printYAML(stats *Stats) {
	fmt.Println("# Auto-generated per-ASN community defaults")
	fmt.Println("# Source:", strings.Join(stats.Files, ", "))
	fmt.Printf("# Total routes analyzed: %d\n", stats.TotalRoutes)
	fmt.Printf("# Threshold for defaults: %.0f%%\n", *threshold*100)
	fmt.Printf("# Minimum routes per ASN: %d\n", *minRoutes)
	if *postPolicy {
		fmt.Println("# Mode: POST-POLICY (action communities stripped)")
		fmt.Println("#   Stripped: 0:X (no-export to ASN), 65535:X (well-known),")
		fmt.Println("#             65281:X, 65282:X (RFC action communities)")
	} else {
		fmt.Println("# Mode: RAW (all communities as seen by route server)")
	}
	fmt.Println()

	// Sort ASNs by route count
	type asnSort struct {
		asn   uint32
		stats *ASNStats
	}
	var sorted []asnSort
	for asn, s := range stats.ASNData {
		if s.Routes >= uint64(*minRoutes) { //nolint:gosec // minRoutes from CLI flag
			sorted = append(sorted, asnSort{asn, s})
		}
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].stats.Routes > sorted[j].stats.Routes
	})

	fmt.Println("asn_defaults:")

	var totalSavingsComm, totalSavingsLComm uint64
	var asnsWithDefaults int

	for _, as := range sorted {
		asn := as.asn
		s := as.stats

		// Find default communities (>threshold frequency)
		var defaultComms []commFreq
		var variableComms []commFreq
		for comm, count := range s.Communities {
			freq := float64(count) / float64(s.Routes)
			cf := commFreq{formatCommunity(comm), count, freq}
			if freq >= *threshold {
				defaultComms = append(defaultComms, cf)
			} else if freq >= 0.5 {
				variableComms = append(variableComms, cf)
			}
		}
		sort.Slice(defaultComms, func(i, j int) bool {
			return defaultComms[i].freq > defaultComms[j].freq
		})
		sort.Slice(variableComms, func(i, j int) bool {
			return variableComms[i].freq > variableComms[j].freq
		})

		// Find default large communities
		var defaultLComms []commFreq
		for comm, count := range s.LargeCommunities {
			freq := float64(count) / float64(s.Routes)
			if freq >= *threshold {
				defaultLComms = append(defaultLComms, commFreq{comm, count, freq})
			}
		}
		sort.Slice(defaultLComms, func(i, j int) bool {
			return defaultLComms[i].freq > defaultLComms[j].freq
		})

		// Skip if no defaults found
		if len(defaultComms) == 0 && len(defaultLComms) == 0 {
			continue
		}
		asnsWithDefaults++

		// Calculate savings
		// Without defaults: encode all communities every time
		// With defaults: encode nothing for routes matching defaults, only encode diffs

		// Estimate: for each default community, we save 4 bytes per route that has it
		var commSavings uint64
		for _, dc := range defaultComms {
			// Routes with this community don't need to encode it
			// But routes WITHOUT it need a "negative" marker (1 byte flag + 4 bytes value = 5 bytes)
			// Net savings = (count * 4) - ((routes - count) * 5)
			routesWithout := s.Routes - dc.count
			if dc.count*4 > routesWithout*5 {
				commSavings += dc.count*4 - routesWithout*5
			}
		}
		totalSavingsComm += commSavings

		var lcommSavings uint64
		for _, dc := range defaultLComms {
			routesWithout := s.Routes - dc.count
			if dc.count*12 > routesWithout*13 {
				lcommSavings += dc.count*12 - routesWithout*13
			}
		}
		totalSavingsLComm += lcommSavings

		// Print YAML for this ASN
		fmt.Printf("\n  %d:  # AS%d\n", asn, asn)

		// Add peer info if available
		if peer, ok := stats.PeerTable[s.Peers[0]]; ok {
			fmt.Printf("    peer_ip: \"%s\"\n", peer.IP)
		}
		fmt.Printf("    routes: %d\n", s.Routes)
		fmt.Printf("    peers: %d\n", len(s.Peers))

		if len(defaultComms) > 0 {
			fmt.Println("    default_communities:")
			for _, dc := range defaultComms {
				fmt.Printf("      - \"%s\"  # %.1f%% (%d/%d)\n",
					dc.value, dc.freq*100, dc.count, s.Routes)
			}
		}

		if len(defaultLComms) > 0 {
			fmt.Println("    default_large_communities:")
			for _, dc := range defaultLComms {
				fmt.Printf("      - \"%s\"  # %.1f%%\n", dc.value, dc.freq*100)
			}
		}

		if len(variableComms) > 0 && len(variableComms) <= 10 {
			fmt.Println("    frequent_communities:  # 50-95%, not defaults but common")
			for _, vc := range variableComms {
				if len(variableComms) <= 5 || vc.freq >= 0.7 {
					fmt.Printf("      - \"%s\"  # %.1f%%\n", vc.value, vc.freq*100)
				}
			}
		}

		// LOCAL_PREF
		if len(s.LocalPrefs) > 0 {
			var topLP uint32
			var topLPCount uint64
			for lp, count := range s.LocalPrefs {
				if count > topLPCount {
					topLP = lp
					topLPCount = count
				}
			}
			lpFreq := float64(topLPCount) / float64(s.Routes) * 100
			if lpFreq >= 90 {
				fmt.Printf("    default_local_pref: %d  # %.1f%%\n", topLP, lpFreq)
			}
		}

		// Savings
		fmt.Println("    savings:")
		fmt.Printf("      community_bytes_saved: %d\n", commSavings)
		fmt.Printf("      large_community_bytes_saved: %d\n", lcommSavings)
		if s.TotalCommBytes > 0 {
			fmt.Printf("      community_reduction_percent: %.1f\n",
				float64(commSavings)/float64(s.TotalCommBytes)*100)
		}
	}

	// Summary
	fmt.Println("\n# ════════════════════════════════════════════════════════════════")
	fmt.Println("summary:")
	fmt.Printf("  total_asns_analyzed: %d\n", len(stats.ASNData))
	fmt.Printf("  asns_with_defaults: %d\n", asnsWithDefaults)
	fmt.Printf("  asns_below_threshold: %d\n", len(stats.ASNData)-len(sorted))
	fmt.Printf("  total_routes: %d\n", stats.TotalRoutes)
	fmt.Println()
	fmt.Println("  estimated_savings:")
	fmt.Printf("    community_bytes: %d\n", totalSavingsComm)
	fmt.Printf("    large_community_bytes: %d\n", totalSavingsLComm)
	fmt.Printf("    total_bytes: %d\n", totalSavingsComm+totalSavingsLComm)

	// Calculate total community bytes for percentage
	var totalCommBytes, totalLCommBytes uint64
	for _, s := range stats.ASNData {
		totalCommBytes += s.TotalCommBytes
		totalLCommBytes += s.TotalLCommBytes
	}
	if totalCommBytes+totalLCommBytes > 0 {
		fmt.Printf("    reduction_percent: %.1f\n",
			float64(totalSavingsComm+totalSavingsLComm)/float64(totalCommBytes+totalLCommBytes)*100)
	}

	// Print simulation results
	fmt.Println()
	fmt.Println("# ════════════════════════════════════════════════════════════════")
	fmt.Println("# SIMULATION RESULTS")
	fmt.Println("# ════════════════════════════════════════════════════════════════")
	fmt.Println("#")
	fmt.Println("# Scenario comparison for community attribute handling:")
	fmt.Println("#")
	fmt.Printf("# 1. NO CACHING (baseline):\n")
	fmt.Printf("#    Parse all communities for every route\n")
	fmt.Printf("#    Total bytes parsed: %s\n", formatBytes(totalCommBytes+totalLCommBytes))
	fmt.Println("#")
	fmt.Printf("# 2. WITH PRE-CONFIGURED DEFAULTS:\n")
	fmt.Printf("#    Assume default communities are present\n")
	fmt.Printf("#    Only encode differences (extras or absences)\n")
	fmt.Printf("#    Bytes saved: %s\n", formatBytes(totalSavingsComm+totalSavingsLComm))
	if totalCommBytes+totalLCommBytes > 0 {
		fmt.Printf("#    Reduction: %.1f%%\n",
			float64(totalSavingsComm+totalSavingsLComm)/float64(totalCommBytes+totalLCommBytes)*100)
	}
	fmt.Println("#")
	fmt.Println("# How to use these defaults:")
	fmt.Println("#   1. Load this YAML at session start")
	fmt.Println("#   2. For each route from ASN X:")
	fmt.Println("#      - Start with default_communities for ASN X")
	fmt.Println("#      - Parse only the delta (additions/removals)")
	fmt.Println("#   3. On NOTIFICATION: cache resets, reload defaults")
	fmt.Println("#")
}

func printJSON(stats *Stats) {
	// Simplified JSON output - just the essentials
	fmt.Println("{")
	fmt.Printf("  \"files\": %q,\n", strings.Join(stats.Files, ", "))
	fmt.Printf("  \"total_routes\": %d,\n", stats.TotalRoutes)
	fmt.Printf("  \"threshold\": %.2f,\n", *threshold)
	fmt.Println("  \"asn_defaults\": {")

	// Sort and filter ASNs
	type asnSort struct {
		asn   uint32
		stats *ASNStats
	}
	var sorted []asnSort
	for asn, s := range stats.ASNData {
		if s.Routes >= uint64(*minRoutes) { //nolint:gosec // minRoutes from CLI flag
			sorted = append(sorted, asnSort{asn, s})
		}
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].stats.Routes > sorted[j].stats.Routes
	})

	for idx, as := range sorted {
		s := as.stats

		var defaultComms []string
		for comm, count := range s.Communities {
			if float64(count)/float64(s.Routes) >= *threshold {
				defaultComms = append(defaultComms, formatCommunity(comm))
			}
		}

		if len(defaultComms) == 0 {
			continue
		}

		sort.Strings(defaultComms)

		comma := ","
		if idx == len(sorted)-1 {
			comma = ""
		}

		fmt.Printf("    \"%d\": {\"routes\": %d, \"defaults\": [", as.asn, s.Routes)
		for i, c := range defaultComms {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Printf("%q", c)
		}
		fmt.Printf("]}%s\n", comma)
	}

	fmt.Println("  }")
	fmt.Println("}")
}

func formatBytes(b uint64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.1f GB", float64(b)/GB)
	case b >= MB:
		return fmt.Sprintf("%.1f MB", float64(b)/MB)
	case b >= KB:
		return fmt.Sprintf("%.1f KB", float64(b)/KB)
	default:
		return fmt.Sprintf("%d B", b)
	}
}
