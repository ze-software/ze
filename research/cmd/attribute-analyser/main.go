// mrt-analyze reads MRT dumps and analyzes BGP attribute repetition patterns
// for caching optimization decisions.
//
// Output: JSON to stdout, human summary to stderr
package main

import (
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net"
	"os"
	"sort"
	"strings"
)

// MRT types (RFC 6396)
const (
	MRT_TABLE_DUMP_V2 = 13
	MRT_BGP4MP        = 16
	MRT_BGP4MP_ET     = 17
)

// TABLE_DUMP_V2 subtypes
const (
	PEER_INDEX_TABLE   = 1
	RIB_IPV4_UNICAST   = 2
	RIB_IPV4_MULTICAST = 3
	RIB_IPV6_UNICAST   = 4
	RIB_IPV6_MULTICAST = 5
	RIB_GENERIC        = 6
)

// BGP4MP subtypes
const (
	BGP4MP_MESSAGE           = 1
	BGP4MP_MESSAGE_AS4       = 4
	BGP4MP_MESSAGE_LOCAL     = 6
	BGP4MP_MESSAGE_AS4_LOCAL = 7
)

// BGP Path Attribute Type Codes
const (
	ATTR_ORIGIN           = 1
	ATTR_AS_PATH          = 2
	ATTR_NEXT_HOP         = 3
	ATTR_MED              = 4
	ATTR_LOCAL_PREF       = 5
	ATTR_ATOMIC_AGGREGATE = 6
	ATTR_AGGREGATOR       = 7
	ATTR_COMMUNITY        = 8
	ATTR_ORIGINATOR_ID    = 9
	ATTR_CLUSTER_LIST     = 10
	ATTR_MP_REACH_NLRI    = 14
	ATTR_MP_UNREACH_NLRI  = 15
	ATTR_EXT_COMMUNITY    = 16
	ATTR_AS4_PATH         = 17
	ATTR_AS4_AGGREGATOR   = 18
	ATTR_LARGE_COMMUNITY  = 32
	ATTR_OTC              = 35 // RFC 9234 Only to Customer
)

var attrNames = map[uint8]string{
	ATTR_ORIGIN:           "ORIGIN",
	ATTR_AS_PATH:          "AS_PATH",
	ATTR_NEXT_HOP:         "NEXT_HOP",
	ATTR_MED:              "MED",
	ATTR_LOCAL_PREF:       "LOCAL_PREF",
	ATTR_ATOMIC_AGGREGATE: "ATOMIC_AGGREGATE",
	ATTR_AGGREGATOR:       "AGGREGATOR",
	ATTR_COMMUNITY:        "COMMUNITY",
	ATTR_ORIGINATOR_ID:    "ORIGINATOR_ID",
	ATTR_CLUSTER_LIST:     "CLUSTER_LIST",
	ATTR_MP_REACH_NLRI:    "MP_REACH_NLRI",
	ATTR_MP_UNREACH_NLRI:  "MP_UNREACH_NLRI",
	ATTR_EXT_COMMUNITY:    "EXT_COMMUNITY",
	ATTR_AS4_PATH:         "AS4_PATH",
	ATTR_AS4_AGGREGATOR:   "AS4_AGGREGATOR",
	ATTR_LARGE_COMMUNITY:  "LARGE_COMMUNITY",
	ATTR_OTC:              "OTC",
}

// Stats holds all analysis statistics
type Stats struct {
	Files        []string
	TotalUpdates uint64

	// Per-attribute stats
	Attributes map[uint8]*AttrStats

	// Bundle stats (excluding AS_PATH)
	Bundles *BundleStats

	// Bundle stats (including AS_PATH)
	BundlesWithASPath *BundleStats

	// Bundle stats (excluding AS_PATH + communities + MP_REACH/UNREACH)
	BundlesNoComm *BundleStats

	// Bundle stats (minimal: only ORIGIN + NEXT_HOP + LOCAL_PREF)
	BundlesMinimal *BundleStats

	// Per-peer stats
	Peers map[uint16]*PeerStats

	// Consecutive hit tracking (temporal locality)
	ConsecutiveHits       uint64 // bundle without AS_PATH matches previous
	ConsecutiveHitsWithAS uint64 // bundle with AS_PATH matches previous
	prevBundleHash        uint64 // previous bundle hash (excl AS_PATH)
	prevBundleHashWithAS  uint64 // previous bundle hash (incl AS_PATH)
	hasPrev               bool   // whether we have a previous to compare

	// Extended consecutive hit tracking (excluding more attributes)
	ConsecutiveHitsNoComm  uint64 // excl AS_PATH + COMMUNITY + LARGE_COMMUNITY + EXT_COMMUNITY
	ConsecutiveHitsMinimal uint64 // only ORIGIN + NEXT_HOP + LOCAL_PREF
	prevBundleHashNoComm   uint64
	prevBundleHashMinimal  uint64

	// Run length tracking for minimal bundle
	currentRunLength uint64            // current consecutive hit run
	RunLengths       map[string]uint64 // "1", "2", "3", "4", "5", "6-10", "11-20", "21+" -> count

	// Global individual community tracking
	GlobalCommunities         map[uint32]uint64 // community -> count across ALL updates
	GlobalLargeCommunities    map[string]uint64 // large community -> count
	GlobalExtCommunities      map[uint64]uint64 // ext community -> count
	UpdatesWithCommunity      uint64            // updates that have any community attr
	UpdatesWithLargeCommunity uint64            // updates that have large community attr

	// Peer table from PEER_INDEX_TABLE
	PeerTable map[uint16]*PeerInfo
}

// AttrStats tracks statistics for a single attribute type
type AttrStats struct {
	Code   uint8
	Name   string
	Total  uint64
	Bytes  uint64
	Values map[uint64]uint64 // hash -> count
}

// BundleStats tracks unique attribute bundles (excluding AS_PATH)
type BundleStats struct {
	Total  uint64
	Values map[uint64]uint64 // hash -> count
}

// PeerInfo holds info from PEER_INDEX_TABLE
type PeerInfo struct {
	Index  uint16
	IP     string
	ASN    uint32
	IsIPv6 bool
}

// PeerStats tracks per-peer statistics
type PeerStats struct {
	Info             *PeerInfo // peer info from PEER_INDEX_TABLE
	Updates          uint64
	Bundles          map[uint64]uint64 // bundle hash -> count
	LocalPrefs       map[uint32]uint64 // LOCAL_PREF value -> count
	MEDs             map[uint32]uint64 // MED value -> count
	Communities      map[uint32]uint64 // individual community -> count
	LargeCommunities map[string]uint64 // "global:local1:local2" -> count
	ExtCommunities   map[uint64]uint64 // ext community -> count
}

// CommunityFreq for JSON output
type CommunityFreq struct {
	Community string  `json:"community"`
	Count     uint64  `json:"count"`
	Frequency float64 `json:"frequency"`
}

// JSONOutput is the JSON output format
type JSONOutput struct {
	Files               []string                            `json:"files"`
	TotalUpdates        uint64                              `json:"total_updates"`
	Attributes          map[string]*JSONAttrStats           `json:"attributes"`
	Bundles             *JSONBundleStats                    `json:"bundles"`
	BundlesWithASPath   *JSONBundleStats                    `json:"bundles_with_aspath"`
	ConsecutiveHits     *JSONConsecutiveStats               `json:"consecutive_hits"`
	PerPeer             map[string]*JSONPeerStats           `json:"per_peer"`
	CommunityExtraction map[string]*JSONCommunityExtraction `json:"community_extraction"`
}

type JSONConsecutiveStats struct {
	WithoutASPath     uint64  `json:"without_aspath"`
	WithoutASPathRate float64 `json:"without_aspath_rate"`
	WithASPath        uint64  `json:"with_aspath"`
	WithASPathRate    float64 `json:"with_aspath_rate"`
}

type JSONAttrStats struct {
	Code         uint8   `json:"code"`
	Total        uint64  `json:"total"`
	Unique       uint64  `json:"unique"`
	Bytes        uint64  `json:"bytes"`
	CacheHitRate float64 `json:"cache_hit_rate"`
}

type JSONBundleStats struct {
	Total        uint64  `json:"total"`
	Unique       uint64  `json:"unique"`
	CacheHitRate float64 `json:"cache_hit_rate"`
}

type JSONPeerStats struct {
	Updates       uint64  `json:"updates"`
	UniqueBundles uint64  `json:"unique_bundles"`
	CacheHitRate  float64 `json:"cache_hit_rate"`
}

type JSONCommunityExtraction struct {
	Updates               uint64          `json:"updates"`
	SessionConstant       []CommunityFreq `json:"session_constant"`
	Variable              []CommunityFreq `json:"variable"`
	PotentialSavingsBytes uint64          `json:"potential_savings_bytes"`
	UniqueLocalPrefs      uint64          `json:"unique_local_prefs"`
	UniqueMEDs            uint64          `json:"unique_meds"`
}

func newStats() *Stats {
	return &Stats{
		Attributes:             make(map[uint8]*AttrStats),
		Bundles:                &BundleStats{Values: make(map[uint64]uint64)},
		BundlesWithASPath:      &BundleStats{Values: make(map[uint64]uint64)},
		BundlesNoComm:          &BundleStats{Values: make(map[uint64]uint64)},
		BundlesMinimal:         &BundleStats{Values: make(map[uint64]uint64)},
		Peers:                  make(map[uint16]*PeerStats),
		GlobalCommunities:      make(map[uint32]uint64),
		GlobalLargeCommunities: make(map[string]uint64),
		GlobalExtCommunities:   make(map[uint64]uint64),
		PeerTable:              make(map[uint16]*PeerInfo),
		RunLengths:             make(map[string]uint64),
	}
}

func newAttrStats(code uint8) *AttrStats {
	name := attrNames[code]
	if name == "" {
		name = fmt.Sprintf("UNKNOWN_%d", code)
	}
	return &AttrStats{
		Code:   code,
		Name:   name,
		Values: make(map[uint64]uint64),
	}
}

func newPeerStats() *PeerStats {
	return &PeerStats{
		Bundles:          make(map[uint64]uint64),
		LocalPrefs:       make(map[uint32]uint64),
		MEDs:             make(map[uint32]uint64),
		Communities:      make(map[uint32]uint64),
		LargeCommunities: make(map[string]uint64),
		ExtCommunities:   make(map[uint64]uint64),
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <file.gz> [file2.gz ...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Analyzes MRT dumps for BGP attribute repetition patterns.\n")
		fmt.Fprintf(os.Stderr, "Output: JSON to stdout, human summary to stderr\n")
		os.Exit(1)
	}

	stats := newStats()

	for _, filename := range os.Args[1:] {
		stats.Files = append(stats.Files, filename)
		if err := processMRT(filename, stats); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing %s: %v\n", filename, err)
			os.Exit(1)
		}
	}

	// Output JSON to stdout
	jsonOut := buildJSONOutput(stats)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(jsonOut); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		os.Exit(1)
	}

	// Output human summary to stderr
	printHumanSummary(os.Stderr, stats)
}

func processMRT(filename string, stats *Stats) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	var reader io.Reader = f

	// Check if gzipped
	if strings.HasSuffix(filename, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer gz.Close()
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
			if err := processTableDumpV2(subtype, data, stats); err != nil {
				return err
			}
		case MRT_BGP4MP, MRT_BGP4MP_ET:
			offset := 0
			if mrtType == MRT_BGP4MP_ET {
				offset = 4 // skip microseconds
			}
			if err := processBGP4MP(subtype, data[offset:], stats); err != nil {
				return err
			}
		}
	}
	return nil
}

func processTableDumpV2(subtype uint16, data []byte, stats *Stats) error {
	switch subtype {
	case PEER_INDEX_TABLE:
		parsePeerIndexTable(data, stats)
		return nil
	case RIB_IPV4_UNICAST, RIB_IPV4_MULTICAST:
		return processRIBEntry(data, 4, stats)
	case RIB_IPV6_UNICAST, RIB_IPV6_MULTICAST:
		return processRIBEntry(data, 6, stats)
	case RIB_GENERIC:
		return processRIBGeneric(data, stats)
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

		// Peer BGP ID (4 bytes) - skip
		if offset+4 > len(data) {
			break
		}
		offset += 4

		// Peer IP Address (4 or 16 bytes)
		ipLen := 4
		if isIPv6 {
			ipLen = 16
		}
		if offset+ipLen > len(data) {
			break
		}
		peerIP := net.IP(data[offset : offset+ipLen]).String()
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
			IP:     peerIP,
			ASN:    asn,
			IsIPv6: isIPv6,
		}
	}
}

func processRIBEntry(data []byte, afi int, stats *Stats) error {
	if len(data) < 7 {
		return nil
	}

	prefixLen := data[4]
	prefixBytes := (int(prefixLen) + 7) / 8

	offset := 5 + prefixBytes
	if offset+2 > len(data) {
		return nil
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

		analyzeAttributes(attrs, peerIndex, stats)
	}

	return nil
}

func processRIBGeneric(data []byte, stats *Stats) error {
	if len(data) < 11 {
		return nil
	}

	nlriLen := data[7]
	offset := 8 + int(nlriLen)
	if offset+2 > len(data) {
		return nil
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

		analyzeAttributes(attrs, peerIndex, stats)
	}

	return nil
}

func processBGP4MP(subtype uint16, data []byte, stats *Stats) error {
	var asSize int

	switch subtype {
	case BGP4MP_MESSAGE, BGP4MP_MESSAGE_LOCAL:
		asSize = 2
	case BGP4MP_MESSAGE_AS4, BGP4MP_MESSAGE_AS4_LOCAL:
		asSize = 4
	default:
		return nil
	}

	if len(data) < asSize*2+4 {
		return nil
	}

	offset := asSize*2 + 4
	afi := binary.BigEndian.Uint16(data[asSize*2+2 : asSize*2+4])

	ipSize := 4
	if afi == 2 {
		ipSize = 16
	}
	offset += ipSize * 2

	if offset+19 > len(data) {
		return nil
	}

	offset += 16 // skip marker
	msgLen := binary.BigEndian.Uint16(data[offset : offset+2])
	msgType := data[offset+2]
	offset += 3

	if msgType != 2 { // Only UPDATE
		return nil
	}

	bodyLen := int(msgLen) - 19
	if offset+bodyLen > len(data) {
		return nil
	}

	updateBody := data[offset : offset+bodyLen]
	attrs := extractAttrsFromUpdate(updateBody)
	if attrs != nil {
		// BGP4MP doesn't have peer_index, use 0xFFFF as marker
		analyzeAttributes(attrs, 0xFFFF, stats)
	}

	return nil
}

func extractAttrsFromUpdate(update []byte) []byte {
	if len(update) < 4 {
		return nil
	}

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

func analyzeAttributes(attrs []byte, peerIndex uint16, stats *Stats) {
	stats.TotalUpdates++

	// Get or create peer stats
	peer, ok := stats.Peers[peerIndex]
	if !ok {
		peer = newPeerStats()
		stats.Peers[peerIndex] = peer
		// Link peer info if available
		if info, exists := stats.PeerTable[peerIndex]; exists {
			peer.Info = info
		}
	}
	peer.Updates++

	// Hash for bundle (excluding AS_PATH)
	bundleHasher := fnv.New64a()
	// Hash for bundle (including AS_PATH)
	bundleHasherWithAS := fnv.New64a()
	// Hash for bundle (excluding AS_PATH + all communities)
	bundleHasherNoComm := fnv.New64a()
	// Hash for bundle (only ORIGIN + NEXT_HOP + LOCAL_PREF)
	bundleHasherMinimal := fnv.New64a()

	offset := 0
	for offset < len(attrs) {
		if offset+3 > len(attrs) {
			break
		}

		flags := attrs[offset]
		typeCode := attrs[offset+1]
		offset += 2

		var attrLen int
		if flags&0x10 != 0 { // Extended length
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

		// Track attribute stats
		attrStats, ok := stats.Attributes[typeCode]
		if !ok {
			attrStats = newAttrStats(typeCode)
			stats.Attributes[typeCode] = attrStats
		}
		attrStats.Total++
		attrStats.Bytes += uint64(attrLen)

		// Hash attribute value
		h := fnv.New64a()
		h.Write(attrValue)
		hash := h.Sum64()
		attrStats.Values[hash]++

		// Add to bundle hash WITH AS_PATH (all real attributes, excluding MP_REACH/UNREACH)
		// MP_REACH/UNREACH are NLRI encoded as attributes (RFC 4760 hack), not real attributes
		if typeCode != ATTR_MP_REACH_NLRI && typeCode != ATTR_MP_UNREACH_NLRI {
			bundleHasherWithAS.Write([]byte{typeCode})
			bundleHasherWithAS.Write(attrValue)
		}

		// Add to bundle hash (excluding AS_PATH, AS4_PATH, MP_REACH, MP_UNREACH)
		// MP_REACH/UNREACH contain NLRI prefixes - unique per UPDATE
		if typeCode != ATTR_AS_PATH && typeCode != ATTR_AS4_PATH &&
			typeCode != ATTR_MP_REACH_NLRI && typeCode != ATTR_MP_UNREACH_NLRI {
			bundleHasher.Write([]byte{typeCode})
			bundleHasher.Write(attrValue)
		}

		// Add to bundle hash (excluding AS_PATH + all communities + MP_REACH/UNREACH)
		if typeCode != ATTR_AS_PATH && typeCode != ATTR_AS4_PATH &&
			typeCode != ATTR_MP_REACH_NLRI && typeCode != ATTR_MP_UNREACH_NLRI &&
			typeCode != ATTR_COMMUNITY && typeCode != ATTR_LARGE_COMMUNITY && typeCode != ATTR_EXT_COMMUNITY {
			bundleHasherNoComm.Write([]byte{typeCode})
			bundleHasherNoComm.Write(attrValue)
		}

		// Add to minimal bundle hash (only ORIGIN + NEXT_HOP + LOCAL_PREF)
		if typeCode == ATTR_ORIGIN || typeCode == ATTR_NEXT_HOP || typeCode == ATTR_LOCAL_PREF {
			bundleHasherMinimal.Write([]byte{typeCode})
			bundleHasherMinimal.Write(attrValue)
		}

		// Extract per-peer community data
		extractCommunityData(typeCode, attrValue, peer, stats)
	}

	// Track bundles
	bundleHash := bundleHasher.Sum64()
	bundleHashWithAS := bundleHasherWithAS.Sum64()
	bundleHashNoComm := bundleHasherNoComm.Sum64()
	bundleHashMinimal := bundleHasherMinimal.Sum64()

	stats.Bundles.Total++
	stats.Bundles.Values[bundleHash]++
	stats.BundlesWithASPath.Total++
	stats.BundlesWithASPath.Values[bundleHashWithAS]++
	stats.BundlesNoComm.Total++
	stats.BundlesNoComm.Values[bundleHashNoComm]++
	stats.BundlesMinimal.Total++
	stats.BundlesMinimal.Values[bundleHashMinimal]++
	peer.Bundles[bundleHash]++

	// Track consecutive hits (temporal locality)
	if stats.hasPrev {
		if bundleHash == stats.prevBundleHash {
			stats.ConsecutiveHits++
		}
		if bundleHashWithAS == stats.prevBundleHashWithAS {
			stats.ConsecutiveHitsWithAS++
		}
		if bundleHashNoComm == stats.prevBundleHashNoComm {
			stats.ConsecutiveHitsNoComm++
		}
		if bundleHashMinimal == stats.prevBundleHashMinimal {
			stats.ConsecutiveHitsMinimal++
			stats.currentRunLength++
		} else {
			// Run ended, record the length
			if stats.currentRunLength > 0 {
				bucket := runLengthBucket(stats.currentRunLength)
				stats.RunLengths[bucket]++
			}
			stats.currentRunLength = 0
		}
	}
	stats.prevBundleHash = bundleHash
	stats.prevBundleHashWithAS = bundleHashWithAS
	stats.prevBundleHashNoComm = bundleHashNoComm
	stats.prevBundleHashMinimal = bundleHashMinimal
	stats.hasPrev = true
}

func runLengthBucket(length uint64) string {
	switch {
	case length == 1:
		return "1"
	case length == 2:
		return "2"
	case length == 3:
		return "3"
	case length == 4:
		return "4"
	case length == 5:
		return "5"
	case length >= 6 && length <= 10:
		return "6-10"
	case length >= 11 && length <= 20:
		return "11-20"
	default:
		return "21+"
	}
}

func extractCommunityData(typeCode uint8, value []byte, peer *PeerStats, stats *Stats) {
	switch typeCode {
	case ATTR_LOCAL_PREF:
		if len(value) >= 4 {
			lp := binary.BigEndian.Uint32(value)
			peer.LocalPrefs[lp]++
		}

	case ATTR_MED:
		if len(value) >= 4 {
			med := binary.BigEndian.Uint32(value)
			peer.MEDs[med]++
		}

	case ATTR_COMMUNITY:
		// Communities are 4 bytes each
		stats.UpdatesWithCommunity++
		for i := 0; i+4 <= len(value); i += 4 {
			comm := binary.BigEndian.Uint32(value[i : i+4])
			peer.Communities[comm]++
			stats.GlobalCommunities[comm]++
		}

	case ATTR_LARGE_COMMUNITY:
		// Large communities are 12 bytes each (global:local1:local2)
		stats.UpdatesWithLargeCommunity++
		for i := 0; i+12 <= len(value); i += 12 {
			global := binary.BigEndian.Uint32(value[i : i+4])
			local1 := binary.BigEndian.Uint32(value[i+4 : i+8])
			local2 := binary.BigEndian.Uint32(value[i+8 : i+12])
			key := fmt.Sprintf("%d:%d:%d", global, local1, local2)
			peer.LargeCommunities[key]++
			stats.GlobalLargeCommunities[key]++
		}

	case ATTR_EXT_COMMUNITY:
		// Extended communities are 8 bytes each
		for i := 0; i+8 <= len(value); i += 8 {
			ext := binary.BigEndian.Uint64(value[i : i+8])
			peer.ExtCommunities[ext]++
			stats.GlobalExtCommunities[ext]++
		}
	}
}

func buildJSONOutput(stats *Stats) *JSONOutput {
	out := &JSONOutput{
		Files:               stats.Files,
		TotalUpdates:        stats.TotalUpdates,
		Attributes:          make(map[string]*JSONAttrStats),
		PerPeer:             make(map[string]*JSONPeerStats),
		CommunityExtraction: make(map[string]*JSONCommunityExtraction),
	}

	// Attributes
	for code, attr := range stats.Attributes {
		unique := uint64(len(attr.Values))
		hitRate := 0.0
		if attr.Total > 0 {
			hitRate = float64(attr.Total-unique) / float64(attr.Total)
		}
		out.Attributes[attr.Name] = &JSONAttrStats{
			Code:         code,
			Total:        attr.Total,
			Unique:       unique,
			Bytes:        attr.Bytes,
			CacheHitRate: hitRate,
		}
	}

	// Bundles (without AS_PATH)
	unique := uint64(len(stats.Bundles.Values))
	hitRate := 0.0
	if stats.Bundles.Total > 0 {
		hitRate = float64(stats.Bundles.Total-unique) / float64(stats.Bundles.Total)
	}
	out.Bundles = &JSONBundleStats{
		Total:        stats.Bundles.Total,
		Unique:       unique,
		CacheHitRate: hitRate,
	}

	// Bundles (with AS_PATH)
	uniqueWithAS := uint64(len(stats.BundlesWithASPath.Values))
	hitRateWithAS := 0.0
	if stats.BundlesWithASPath.Total > 0 {
		hitRateWithAS = float64(stats.BundlesWithASPath.Total-uniqueWithAS) / float64(stats.BundlesWithASPath.Total)
	}
	out.BundlesWithASPath = &JSONBundleStats{
		Total:        stats.BundlesWithASPath.Total,
		Unique:       uniqueWithAS,
		CacheHitRate: hitRateWithAS,
	}

	// Consecutive hits (temporal locality)
	comparisons := stats.TotalUpdates - 1 // first update has nothing to compare to
	consRateWithout := 0.0
	consRateWith := 0.0
	if comparisons > 0 {
		consRateWithout = float64(stats.ConsecutiveHits) / float64(comparisons)
		consRateWith = float64(stats.ConsecutiveHitsWithAS) / float64(comparisons)
	}
	out.ConsecutiveHits = &JSONConsecutiveStats{
		WithoutASPath:     stats.ConsecutiveHits,
		WithoutASPathRate: consRateWithout,
		WithASPath:        stats.ConsecutiveHitsWithAS,
		WithASPathRate:    consRateWith,
	}

	// Per-peer stats and community extraction
	for peerIdx, peer := range stats.Peers {
		peerKey := fmt.Sprintf("%d", peerIdx)

		uniqueBundles := uint64(len(peer.Bundles))
		peerHitRate := 0.0
		if peer.Updates > 0 {
			peerHitRate = float64(peer.Updates-uniqueBundles) / float64(peer.Updates)
		}
		out.PerPeer[peerKey] = &JSONPeerStats{
			Updates:       peer.Updates,
			UniqueBundles: uniqueBundles,
			CacheHitRate:  peerHitRate,
		}

		// Community extraction
		ce := &JSONCommunityExtraction{
			Updates:          peer.Updates,
			UniqueLocalPrefs: uint64(len(peer.LocalPrefs)),
			UniqueMEDs:       uint64(len(peer.MEDs)),
		}

		// Analyze regular communities
		var sessionConstant, variable []CommunityFreq
		var potentialSavings uint64

		for comm, count := range peer.Communities {
			freq := float64(count) / float64(peer.Updates)
			cf := CommunityFreq{
				Community: formatCommunity(comm),
				Count:     count,
				Frequency: freq,
			}
			if freq >= 0.9 {
				sessionConstant = append(sessionConstant, cf)
				// Savings: instead of 4 bytes * count, we store 4 bytes once
				potentialSavings += (count - 1) * 4
			} else {
				variable = append(variable, cf)
			}
		}

		// Add large communities
		for comm, count := range peer.LargeCommunities {
			freq := float64(count) / float64(peer.Updates)
			cf := CommunityFreq{
				Community: comm,
				Count:     count,
				Frequency: freq,
			}
			if freq >= 0.9 {
				sessionConstant = append(sessionConstant, cf)
				potentialSavings += (count - 1) * 12
			} else {
				variable = append(variable, cf)
			}
		}

		// Sort by frequency descending
		sort.Slice(sessionConstant, func(i, j int) bool {
			return sessionConstant[i].Frequency > sessionConstant[j].Frequency
		})
		sort.Slice(variable, func(i, j int) bool {
			return variable[i].Frequency > variable[j].Frequency
		})

		// Limit variable to top 10
		if len(variable) > 10 {
			variable = variable[:10]
		}

		ce.SessionConstant = sessionConstant
		ce.Variable = variable
		ce.PotentialSavingsBytes = potentialSavings

		out.CommunityExtraction[peerKey] = ce
	}

	return out
}

func formatCommunity(comm uint32) string {
	high := comm >> 16
	low := comm & 0xFFFF
	return fmt.Sprintf("%d:%d", high, low)
}

func printHumanSummary(w io.Writer, stats *Stats) {
	fmt.Fprintf(w, "\n=== MRT Attribute Analysis ===\n")
	fmt.Fprintf(w, "Files: %s\n", strings.Join(stats.Files, ", "))
	fmt.Fprintf(w, "Total UPDATEs: %s\n\n", formatNumber(stats.TotalUpdates))

	// Sort attributes by cache hit rate
	type attrSort struct {
		code    uint8
		name    string
		unique  uint64
		total   uint64
		bytes   uint64
		hitRate float64
	}
	var sorted []attrSort
	for code, attr := range stats.Attributes {
		unique := uint64(len(attr.Values))
		hitRate := 0.0
		if attr.Total > 0 {
			hitRate = float64(attr.Total-unique) / float64(attr.Total) * 100
		}
		sorted = append(sorted, attrSort{code, attr.Name, unique, attr.Total, attr.Bytes, hitRate})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].hitRate > sorted[j].hitRate
	})

	fmt.Fprintf(w, "ATTRIBUTE REPETITION (sorted by cache hit rate):\n")
	fmt.Fprintf(w, "  %-18s %12s %12s %9s %12s\n", "Attr", "Unique", "Total", "Hit Rate", "Bytes")
	fmt.Fprintf(w, "  %s\n", strings.Repeat("─", 67))
	for _, a := range sorted {
		marker := ""
		if a.code == ATTR_AS_PATH {
			marker = " ← verify"
		}
		fmt.Fprintf(w, "  %-18s %12s %12s %8.3f%% %12s%s\n",
			a.name, formatNumber(a.unique), formatNumber(a.total),
			a.hitRate, formatBytes(a.bytes), marker)
	}

	// Bytes distribution
	fmt.Fprintf(w, "\nBYTES DISTRIBUTION:\n")
	var totalBytes uint64
	for _, attr := range stats.Attributes {
		totalBytes += attr.Bytes
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].bytes > sorted[j].bytes
	})
	for _, a := range sorted {
		if a.bytes == 0 {
			continue
		}
		pct := float64(a.bytes) / float64(totalBytes) * 100
		fmt.Fprintf(w, "  %-18s %12s (%5.1f%%)\n", a.name, formatBytes(a.bytes), pct)
	}

	// Bundle analysis
	fmt.Fprintf(w, "\nBUNDLE ANALYSIS:\n")

	// Without AS_PATH
	unique := uint64(len(stats.Bundles.Values))
	hitRate := 0.0
	if stats.Bundles.Total > 0 {
		hitRate = float64(stats.Bundles.Total-unique) / float64(stats.Bundles.Total) * 100
	}
	fmt.Fprintf(w, "  Without AS_PATH:\n")
	fmt.Fprintf(w, "    Unique bundles: %s / %s total\n", formatNumber(unique), formatNumber(stats.Bundles.Total))
	fmt.Fprintf(w, "    Cache hit rate: %.2f%%\n", hitRate)

	// With AS_PATH
	uniqueWithAS := uint64(len(stats.BundlesWithASPath.Values))
	hitRateWithAS := 0.0
	if stats.BundlesWithASPath.Total > 0 {
		hitRateWithAS = float64(stats.BundlesWithASPath.Total-uniqueWithAS) / float64(stats.BundlesWithASPath.Total) * 100
	}
	fmt.Fprintf(w, "  With AS_PATH:\n")
	fmt.Fprintf(w, "    Unique bundles: %s / %s total\n", formatNumber(uniqueWithAS), formatNumber(stats.BundlesWithASPath.Total))
	fmt.Fprintf(w, "    Cache hit rate: %.2f%%\n", hitRateWithAS)

	// Consecutive hit analysis (temporal locality)
	fmt.Fprintf(w, "\nCONSECUTIVE HIT ANALYSIS (temporal locality):\n")
	fmt.Fprintf(w, "  If we cache just the LAST bundle seen, what's the hit rate?\n")
	comparisons := stats.TotalUpdates - 1
	consRateWith := 0.0
	consRateWithout := 0.0
	consRateNoComm := 0.0
	consRateMinimal := 0.0
	if comparisons > 0 {
		consRateWith = float64(stats.ConsecutiveHitsWithAS) / float64(comparisons) * 100
		consRateWithout = float64(stats.ConsecutiveHits) / float64(comparisons) * 100
		consRateNoComm = float64(stats.ConsecutiveHitsNoComm) / float64(comparisons) * 100
		consRateMinimal = float64(stats.ConsecutiveHitsMinimal) / float64(comparisons) * 100
	}
	// Get unique bundle counts for each strategy (reuse uniqueWithAS from above)
	uniqueNoAS := uint64(len(stats.Bundles.Values))
	uniqueNoComm := uint64(len(stats.BundlesNoComm.Values))
	uniqueMinimal := uint64(len(stats.BundlesMinimal.Values))

	fmt.Fprintf(w, "\n  Note: MP_REACH/UNREACH always excluded (NLRI, not real attributes)\n")
	fmt.Fprintf(w, "\n  %-45s %12s %12s %8s\n", "Exclusion Strategy", "Unique", "Consec Hits", "Rate")
	fmt.Fprintf(w, "  %s\n", strings.Repeat("─", 80))
	fmt.Fprintf(w, "  %-45s %12s %12s %7.2f%%\n",
		"All real attributes (incl AS_PATH)",
		formatNumber(uniqueWithAS),
		formatNumber(stats.ConsecutiveHitsWithAS), consRateWith)
	fmt.Fprintf(w, "  %-45s %12s %12s %7.2f%%\n",
		"Exclude AS_PATH",
		formatNumber(uniqueNoAS),
		formatNumber(stats.ConsecutiveHits), consRateWithout)
	fmt.Fprintf(w, "  %-45s %12s %12s %7.2f%%\n",
		"Exclude AS_PATH + Communities",
		formatNumber(uniqueNoComm),
		formatNumber(stats.ConsecutiveHitsNoComm), consRateNoComm)
	fmt.Fprintf(w, "  %-45s %12s %12s %7.2f%%\n",
		"Minimal (only ORIGIN + NEXT_HOP + LOCAL_PREF)",
		formatNumber(uniqueMinimal),
		formatNumber(stats.ConsecutiveHitsMinimal), consRateMinimal)

	// Run length distribution for minimal bundle
	// Record final run if any
	if stats.currentRunLength > 0 {
		bucket := runLengthBucket(stats.currentRunLength)
		stats.RunLengths[bucket]++
	}

	fmt.Fprintf(w, "\n  Run length distribution (minimal bundle consecutive hits):\n")
	fmt.Fprintf(w, "  %-12s %12s\n", "Run Length", "Count")
	fmt.Fprintf(w, "  %s\n", strings.Repeat("─", 26))
	buckets := []string{"1", "2", "3", "4", "5", "6-10", "11-20", "21+"}
	for _, b := range buckets {
		count := stats.RunLengths[b]
		if count > 0 {
			fmt.Fprintf(w, "  %-12s %12s\n", b, formatNumber(count))
		}
	}

	/*
	   Diagram: consecutive updates often share same attributes

	   UPDATE[n]   UPDATE[n+1]   Match?
	   ─────────   ──────────    ──────
	   AS_PATH: A   AS_PATH: A    ✓  }
	   ORIGIN: I    ORIGIN: I     ✓  } Same bundle
	   NH: 1.2.3.4  NH: 1.2.3.4   ✓  }
	   COMM: X,Y    COMM: X,Y     ✓  }

	   UPDATE[n+1]  UPDATE[n+2]   Match?
	   ──────────   ──────────    ──────
	   AS_PATH: A   AS_PATH: B    ✗  ← AS_PATH changed
	   ORIGIN: I    ORIGIN: I     ✓  }
	   NH: 1.2.3.4  NH: 1.2.3.4   ✓  } Same without AS_PATH!
	   COMM: X,Y    COMM: X,Y     ✓  }
	*/

	// Per-peer summary with ASN info
	fmt.Fprintf(w, "\nPER-PEER ANALYSIS (top 10 by volume):\n")
	fmt.Fprintf(w, "%s\n", strings.Repeat("═", 90))

	type peerAnalysis struct {
		idx           uint16
		asn           uint32
		ip            string
		updates       uint64
		uniqueBundles uint64
		hitRate       float64
		ownedComms    int // communities matching ASN:xxx
		totalComms    int // total unique communities
		ownedPct      float64
		defaultComms  []string // communities >90% frequency
	}

	var peers []peerAnalysis
	for idx, peer := range stats.Peers {
		pa := peerAnalysis{
			idx:           idx,
			updates:       peer.Updates,
			uniqueBundles: uint64(len(peer.Bundles)),
			totalComms:    len(peer.Communities),
		}
		if peer.Updates > 0 {
			pa.hitRate = float64(peer.Updates-pa.uniqueBundles) / float64(peer.Updates) * 100
		}

		// Get ASN and IP from peer info
		if peer.Info != nil {
			pa.asn = peer.Info.ASN
			pa.ip = peer.Info.IP
		}

		// Count "owned" communities (ASN:xxx pattern)
		for comm, count := range peer.Communities {
			commASN := comm >> 16
			if pa.asn != 0 && commASN == pa.asn {
				pa.ownedComms++
			}
			// Track default communities (>90%)
			freq := float64(count) / float64(peer.Updates)
			if freq >= 0.90 {
				pa.defaultComms = append(pa.defaultComms, formatCommunity(comm))
			}
		}
		if pa.totalComms > 0 {
			pa.ownedPct = float64(pa.ownedComms) / float64(pa.totalComms) * 100
		}

		peers = append(peers, pa)
	}

	sort.Slice(peers, func(i, j int) bool {
		return peers[i].updates > peers[j].updates
	})

	limit := 10
	if len(peers) < limit {
		limit = len(peers)
	}

	var avgHitRate float64
	for i := 0; i < limit; i++ {
		p := peers[i]
		avgHitRate += p.hitRate

		// Header for each peer
		asnStr := "unknown"
		if p.asn != 0 {
			asnStr = fmt.Sprintf("AS%d", p.asn)
		}
		fmt.Fprintf(w, "\n  Peer %d: %s (%s)\n", p.idx, asnStr, p.ip)
		fmt.Fprintf(w, "  %s\n", strings.Repeat("─", 60))
		fmt.Fprintf(w, "    Routes: %s | Bundle hit rate: %.1f%%\n",
			formatNumber(p.updates), p.hitRate)
		fmt.Fprintf(w, "    Communities: %d unique | %d owned (%s:xxx) = %.0f%%\n",
			p.totalComms, p.ownedComms, asnStr, p.ownedPct)

		// Show default communities
		if len(p.defaultComms) > 0 {
			fmt.Fprintf(w, "    Defaults (>90%%): ")
			maxShow := 5
			if len(p.defaultComms) < maxShow {
				maxShow = len(p.defaultComms)
			}
			for j := 0; j < maxShow; j++ {
				if j > 0 {
					fmt.Fprintf(w, ", ")
				}
				fmt.Fprintf(w, "%s", p.defaultComms[j])
			}
			if len(p.defaultComms) > maxShow {
				fmt.Fprintf(w, " (+%d more)", len(p.defaultComms)-maxShow)
			}
			fmt.Fprintf(w, "\n")
		}

		// Infer relationship type
		relType := "Unknown"
		if p.ownedPct >= 80 {
			relType = "Transit/Provider (adds own communities to all routes)"
		} else if p.ownedPct >= 40 {
			relType = "Peer (adds some own communities)"
		} else if p.ownedPct < 10 && p.totalComms > 0 {
			relType = "Route Server/Collector (passes through communities)"
		}
		fmt.Fprintf(w, "    Inferred type: %s\n", relType)
	}

	if limit > 0 {
		avgHitRate /= float64(limit)
	}

	fmt.Fprintf(w, "\n%s\n", strings.Repeat("═", 90))

	// Community extraction summary
	fmt.Fprintf(w, "\nCOMMUNITY EXTRACTION ANALYSIS:\n")
	var totalSessionConstant, totalVariable int
	var totalSavings uint64
	for _, peer := range stats.Peers {
		for _, count := range peer.Communities {
			freq := float64(count) / float64(peer.Updates)
			if freq >= 0.9 {
				totalSessionConstant++
				totalSavings += (count - 1) * 4
			} else {
				totalVariable++
			}
		}
		for _, count := range peer.LargeCommunities {
			freq := float64(count) / float64(peer.Updates)
			if freq >= 0.9 {
				totalSessionConstant++
				totalSavings += (count - 1) * 12
			} else {
				totalVariable++
			}
		}
	}
	fmt.Fprintf(w, "  Session-constant communities (>90%%): %d\n", totalSessionConstant)
	fmt.Fprintf(w, "  Variable communities (<90%%): %d\n", totalVariable)
	fmt.Fprintf(w, "  Potential savings from extraction: %s\n", formatBytes(totalSavings))

	// LOCAL_PREF and MED analysis
	var totalLPUnique, totalMEDUnique int
	for _, peer := range stats.Peers {
		totalLPUnique += len(peer.LocalPrefs)
		totalMEDUnique += len(peer.MEDs)
	}
	fmt.Fprintf(w, "  Unique LOCAL_PREF values across all peers: %d\n", totalLPUnique)
	fmt.Fprintf(w, "  Unique MED values across all peers: %d\n", totalMEDUnique)

	// Insights
	fmt.Fprintf(w, "\nINSIGHTS:\n")
	for _, a := range sorted {
		if a.code == ATTR_AS_PATH {
			if a.hitRate >= 80 {
				fmt.Fprintf(w, "  - AS_PATH: %.1f%% cache hit (better than expected, worth caching!)\n", a.hitRate)
			} else if a.hitRate >= 50 {
				fmt.Fprintf(w, "  - AS_PATH: %.1f%% cache hit (moderate, consider caching)\n", a.hitRate)
			} else {
				fmt.Fprintf(w, "  - AS_PATH: %.1f%% cache hit (volatile as expected)\n", a.hitRate)
			}
			break
		}
	}
	fmt.Fprintf(w, "  - Per-peer avg hit rate: %.1f%% vs global %.1f%%\n", avgHitRate, hitRate)
	if avgHitRate > hitRate+1 {
		fmt.Fprintf(w, "    → Session-specific caching provides benefit\n")
	} else {
		fmt.Fprintf(w, "    → Global caching sufficient\n")
	}

	// Individual community breakdown
	fmt.Fprintf(w, "\nINDIVIDUAL COMMUNITY BREAKDOWN:\n")
	fmt.Fprintf(w, "  Total unique communities: %s\n", formatNumber(uint64(len(stats.GlobalCommunities))))
	fmt.Fprintf(w, "  Total unique large communities: %s\n", formatNumber(uint64(len(stats.GlobalLargeCommunities))))
	fmt.Fprintf(w, "  Updates with COMMUNITY attr: %s (%.1f%%)\n",
		formatNumber(stats.UpdatesWithCommunity),
		float64(stats.UpdatesWithCommunity)/float64(stats.TotalUpdates)*100)
	fmt.Fprintf(w, "  Updates with LARGE_COMMUNITY attr: %s (%.1f%%)\n",
		formatNumber(stats.UpdatesWithLargeCommunity),
		float64(stats.UpdatesWithLargeCommunity)/float64(stats.TotalUpdates)*100)

	// Top 20 most common communities
	type commCount struct {
		comm  string
		count uint64
		pct   float64
	}
	var topComms []commCount
	for comm, count := range stats.GlobalCommunities {
		pct := float64(count) / float64(stats.UpdatesWithCommunity) * 100
		topComms = append(topComms, commCount{formatCommunity(comm), count, pct})
	}
	sort.Slice(topComms, func(i, j int) bool {
		return topComms[i].count > topComms[j].count
	})

	fmt.Fprintf(w, "\n  Top 20 COMMUNITY values (by occurrence):\n")
	fmt.Fprintf(w, "    %-20s %12s %8s\n", "Community", "Count", "% of updates")
	fmt.Fprintf(w, "    %s\n", strings.Repeat("─", 44))
	topLimit := 20
	if len(topComms) < topLimit {
		topLimit = len(topComms)
	}
	for i := 0; i < topLimit; i++ {
		c := topComms[i]
		fmt.Fprintf(w, "    %-20s %12s %7.2f%%\n", c.comm, formatNumber(c.count), c.pct)
	}

	// Top 20 large communities
	var topLargeComms []commCount
	for comm, count := range stats.GlobalLargeCommunities {
		pct := float64(count) / float64(stats.UpdatesWithLargeCommunity) * 100
		topLargeComms = append(topLargeComms, commCount{comm, count, pct})
	}
	sort.Slice(topLargeComms, func(i, j int) bool {
		return topLargeComms[i].count > topLargeComms[j].count
	})

	if len(topLargeComms) > 0 {
		fmt.Fprintf(w, "\n  Top 20 LARGE_COMMUNITY values (by occurrence):\n")
		fmt.Fprintf(w, "    %-30s %12s %8s\n", "Large Community", "Count", "% of updates")
		fmt.Fprintf(w, "    %s\n", strings.Repeat("─", 54))
		topLimit = 20
		if len(topLargeComms) < topLimit {
			topLimit = len(topLargeComms)
		}
		for i := 0; i < topLimit; i++ {
			c := topLargeComms[i]
			fmt.Fprintf(w, "    %-30s %12s %7.2f%%\n", c.comm, formatNumber(c.count), c.pct)
		}
	}

	// Caching strategy recommendations
	fmt.Fprintf(w, "\n"+strings.Repeat("═", 70)+"\n")
	fmt.Fprintf(w, "CACHING STRATEGY RECOMMENDATIONS:\n")
	fmt.Fprintf(w, strings.Repeat("═", 70)+"\n")

	fmt.Fprintf(w, "\n1. TRIAL-RUN APPROACH (per-session defaults):\n")
	fmt.Fprintf(w, "   After receiving first ~100 routes from a peer, analyze patterns:\n")
	fmt.Fprintf(w, "   - Identify communities present in >90%% of routes\n")
	fmt.Fprintf(w, "   - Set these as 'session defaults' (assumed present)\n")
	fmt.Fprintf(w, "   - Only encode ABSENCE (negative marker) for the exceptions\n")
	fmt.Fprintf(w, "   - On NOTIFICATION: cache resets, but learned defaults persist\n")
	fmt.Fprintf(w, "     (can be reused on reconnect to same peer)\n")

	fmt.Fprintf(w, "\n2. ATTRIBUTE-SPECIFIC STRATEGIES:\n")
	// Count unique LOCAL_PREFs properly
	lpSet := make(map[uint32]bool)
	medSet := make(map[uint32]bool)
	for _, peer := range stats.Peers {
		for lp := range peer.LocalPrefs {
			lpSet[lp] = true
		}
		for med := range peer.MEDs {
			medSet[med] = true
		}
	}
	fmt.Fprintf(w, "   LOCAL_PREF:  Only %d unique values → small lookup table\n", len(lpSet))
	fmt.Fprintf(w, "   MED:         Only %d unique values → cache per session\n", len(medSet))
	nhUnique := 0
	if nh := stats.Attributes[ATTR_NEXT_HOP]; nh != nil {
		nhUnique = len(nh.Values)
	}
	fmt.Fprintf(w, "   NEXT_HOP:    Only %d unique → global cache with high hit rate\n", nhUnique)
	commSets := 0
	if cs := stats.Attributes[ATTR_COMMUNITY]; cs != nil {
		commSets = len(cs.Values)
	}
	fmt.Fprintf(w, "   COMMUNITY:   %d unique sets, but %d unique values\n",
		commSets, len(stats.GlobalCommunities))
	fmt.Fprintf(w, "               → Cache individual values + reconstruct sets\n")

	fmt.Fprintf(w, "\n3. BUNDLE CACHING TIERS:\n")
	fmt.Fprintf(w, "   Tier 1 (size=1):   Last-seen (excl AS_PATH) → %.2f%% hit rate\n", consRateWithout)
	fmt.Fprintf(w, "   Tier 1a:           Last-seen (excl AS+COMM) → %.2f%% hit rate\n", consRateNoComm)
	fmt.Fprintf(w, "   Tier 1b:           Last-seen (minimal)      → %.2f%% hit rate\n", consRateMinimal)
	fmt.Fprintf(w, "   Tier 2 (size=N):   Per-peer LRU cache       → ~%.1f%% hit rate\n", avgHitRate)
	fmt.Fprintf(w, "   Tier 3 (global):   All unique bundles       → %.2f%% hit rate\n", hitRate)
	fmt.Fprintf(w, "   Recommendation:    Tier 2 (per-peer) with size ~1000\n")

	fmt.Fprintf(w, "\n4. MEMORY vs PARSE TRADE-OFF:\n")
	bundleMemory := uint64(len(stats.Bundles.Values)) * 64 // rough estimate: 64 bytes per cached bundle
	savedParses := stats.Bundles.Total - uint64(len(stats.Bundles.Values))
	fmt.Fprintf(w, "   Cache memory (est): %s for %s bundles\n",
		formatBytes(bundleMemory), formatNumber(uint64(len(stats.Bundles.Values))))
	fmt.Fprintf(w, "   Saved parses:       %s (%.1f%% of total)\n",
		formatNumber(savedParses), float64(savedParses)/float64(stats.Bundles.Total)*100)

	fmt.Fprintf(w, "\n")
}

func formatNumber(n uint64) string {
	s := fmt.Sprintf("%d", n)
	result := make([]byte, 0, len(s)+(len(s)-1)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
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
