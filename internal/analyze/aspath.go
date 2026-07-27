// Design: (none -- research/analysis tool)
//
// Analyzes AS_PATH suffix sharing across MRT dumps to determine whether
// a reversed trie would be effective for AS_PATH dedup.
//
// Measures: unique full paths, unique suffixes at each depth, trie node
// count, and the compression ratio vs naive per-path storage.
//
// Output: JSON to stdout, human summary to stderr.
package analyze

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

type aspathAnalysis struct {
	Files      []string
	TotalPaths uint64

	UniquePaths uint64
	TotalASNs   uint64

	LengthDist map[int]uint64

	trie     *trieNode
	pathSeen map[uint64]bool
}

type trieNode struct {
	children map[uint32]*trieNode
	count    uint64
}

func newTrieNode() *trieNode {
	return &trieNode{children: make(map[uint32]*trieNode)}
}

func newASPathAnalysis() *aspathAnalysis {
	return &aspathAnalysis{
		LengthDist: make(map[int]uint64),
		trie:       newTrieNode(),
		pathSeen:   make(map[uint64]bool),
	}
}

func runASPath(args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, `ze-analyze aspath -- AS_PATH suffix sharing analysis

Analyzes MRT files to measure how much AS_PATH data can be compressed
using a reversed trie (suffix sharing). Builds a trie where shared
suffixes collapse into shared nodes.

Processes both TABLE_DUMP_V2 (RIB snapshots) and BGP4MP (live updates).
JSON output to stdout; human-readable summary to stderr.

Usage:
  ze-analyze aspath <file.gz> [file2.gz ...]

Examples:
  ze-analyze aspath test/internet/latest-bview.gz 2>/dev/null | jq .
  ze-analyze aspath test/internet/latest-bview.gz >/dev/null
`)
		return 1
	}

	st := newASPathAnalysis()
	var damaged malformedCounter

	for _, fname := range args {
		st.Files = append(st.Files, fname)
		if err := processMRTFile(fname, mrtHandler{
			OnPeerIndex: func(_ []byte) {},
			OnRIB: func(data []byte, subtype uint16) {
				damaged.note(forEachRIBEntry(data, subtype, func(_ uint16, attrs []byte) {
					aspathAnalyzeRoute(attrs, st)
				}))
			},
			OnBGP4MP: func(data []byte, subtype uint16, _ uint32) {
				body, _ := extractBGP4MPUpdate(subtype, data)
				if body == nil {
					return
				}
				attrs := extractUpdateAttrs(body)
				if attrs != nil {
					aspathAnalyzeRoute(attrs, st)
				}
			},
		}); err != nil {
			fmt.Fprintf(os.Stderr, "error processing %s: %v\n", fname, err)
			return 1
		}
	}

	aspathPrintJSON(os.Stdout, st)
	aspathPrintSummary(os.Stderr, st)
	damaged.report(os.Stderr)

	return 0
}

func aspathAnalyzeRoute(attrs []byte, st *aspathAnalysis) {
	iterateAttrs(attrs, func(_, typeCode uint8, value []byte) {
		if typeCode != attrASPath && typeCode != attrAS4Path {
			return
		}
		asns := parseASPathASNs(value)
		if len(asns) == 0 {
			return
		}

		st.TotalPaths++
		st.TotalASNs += uint64(len(asns))
		st.LengthDist[len(asns)]++

		h := hashASPath(asns)
		if !st.pathSeen[h] {
			st.pathSeen[h] = true
			st.UniquePaths++
		}

		insertReversed(st.trie, asns)
	})
}

func parseASPathASNs(value []byte) []uint32 {
	var asns []uint32
	off := 0
	for off < len(value) {
		if off+2 > len(value) {
			break
		}
		// segType := value[off] // 1=AS_SET, 2=AS_SEQUENCE
		segLen := int(value[off+1])
		off += 2
		for range segLen {
			if off+4 > len(value) {
				return asns
			}
			asns = append(asns, binary.BigEndian.Uint32(value[off:off+4]))
			off += 4
		}
	}
	return asns
}

func hashASPath(asns []uint32) uint64 {
	var h uint64 = 14695981039346656037 // FNV-1a offset basis
	for _, asn := range asns {
		b := [4]byte{}
		binary.BigEndian.PutUint32(b[:], asn)
		for _, c := range b {
			h ^= uint64(c)
			h *= 1099511628211 // FNV-1a prime
		}
	}
	return h
}

func insertReversed(root *trieNode, asns []uint32) {
	node := root
	for i := len(asns) - 1; i >= 0; i-- {
		asn := asns[i]
		child, ok := node.children[asn]
		if !ok {
			child = newTrieNode()
			node.children[asn] = child
		}
		child.count++
		node = child
	}
}

func countTrieNodes(node *trieNode) uint64 {
	var total uint64
	for _, child := range node.children {
		total++
		total += countTrieNodes(child)
	}
	return total
}

func trieDepthStats(node *trieNode, depth int, stats map[int]uint64) {
	for _, child := range node.children {
		stats[depth]++
		trieDepthStats(child, depth+1, stats)
	}
}

func trieFanoutStats(node *trieNode, fanout map[int]uint64) {
	n := len(node.children)
	if n > 0 {
		fanout[n]++
	}
	for _, child := range node.children {
		trieFanoutStats(child, fanout)
	}
}

type aspathJSONOutput struct {
	Files       []string              `json:"files"`
	TotalPaths  uint64                `json:"total-paths"`
	UniquePaths uint64                `json:"unique-paths"`
	PathDedup   float64               `json:"path-dedup-rate"`
	TotalASNs   uint64                `json:"total-asns"`
	TrieNodes   uint64                `json:"trie-nodes"`
	NaiveASNs   uint64                `json:"naive-asn-slots"`
	TrieRatio   float64               `json:"trie-compression-ratio"`
	LengthDist  map[int]uint64        `json:"length-distribution"`
	DepthStats  map[int]uint64        `json:"trie-depth-node-count"`
	FanoutDist  map[string]uint64     `json:"trie-fanout-distribution"`
	TopOrigins  []aspathJSONOriginASN `json:"top-origin-asns"`
}

type aspathJSONOriginASN struct {
	ASN   uint32 `json:"asn"`
	Count uint64 `json:"count"`
}

func aspathPrintJSON(w io.Writer, st *aspathAnalysis) {
	trieNodes := countTrieNodes(st.trie)

	depthStats := make(map[int]uint64)
	trieDepthStats(st.trie, 0, depthStats)

	fanout := make(map[int]uint64)
	trieFanoutStats(st.trie, fanout)
	fanoutDist := make(map[string]uint64)
	for f, c := range fanout {
		var bucket string
		switch {
		case f == 1:
			bucket = "1"
		case f <= 5:
			bucket = "2-5"
		case f <= 20:
			bucket = "6-20"
		case f <= 100:
			bucket = "21-100"
		case f <= 1000:
			bucket = "101-1000"
		default:
			bucket = "1001+"
		}
		fanoutDist[bucket] += c
	}

	naiveASNs := uint64(0)
	for length, count := range st.LengthDist {
		naiveASNs += uint64(length) * count //nolint:gosec // length is small
	}

	pathDedup := 0.0
	if st.TotalPaths > 0 {
		pathDedup = float64(st.TotalPaths-st.UniquePaths) / float64(st.TotalPaths)
	}

	trieRatio := 0.0
	if naiveASNs > 0 {
		trieRatio = float64(naiveASNs-trieNodes) / float64(naiveASNs)
	}

	origins := collectOrigins(st.trie)

	out := &aspathJSONOutput{
		Files:       st.Files,
		TotalPaths:  st.TotalPaths,
		UniquePaths: st.UniquePaths,
		PathDedup:   pathDedup,
		TotalASNs:   st.TotalASNs,
		TrieNodes:   trieNodes,
		NaiveASNs:   naiveASNs,
		TrieRatio:   trieRatio,
		LengthDist:  st.LengthDist,
		DepthStats:  depthStats,
		FanoutDist:  fanoutDist,
		TopOrigins:  origins,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "error encoding JSON: %v\n", err)
	}
}

func collectOrigins(root *trieNode) []aspathJSONOriginASN {
	type pair struct {
		asn   uint32
		count uint64
	}
	var origins []pair
	for asn, child := range root.children {
		origins = append(origins, pair{asn, child.count})
	}
	sort.Slice(origins, func(i, j int) bool { return origins[i].count > origins[j].count })
	if len(origins) > 20 {
		origins = origins[:20]
	}
	result := make([]aspathJSONOriginASN, len(origins))
	for i, o := range origins {
		result[i] = aspathJSONOriginASN{ASN: o.asn, Count: o.count}
	}
	return result
}

func aspathPrintSummary(w io.Writer, st *aspathAnalysis) {
	trieNodes := countTrieNodes(st.trie)

	naiveASNs := uint64(0)
	for length, count := range st.LengthDist {
		naiveASNs += uint64(length) * count //nolint:gosec // length is small
	}

	pathDedup := 0.0
	if st.TotalPaths > 0 {
		pathDedup = float64(st.TotalPaths-st.UniquePaths) / float64(st.TotalPaths) * 100
	}

	trieRatio := 0.0
	if naiveASNs > 0 {
		trieRatio = float64(naiveASNs-trieNodes) / float64(naiveASNs) * 100
	}

	wf(w, "\n=== AS_PATH Suffix Sharing Analysis ===\n")
	wf(w, "\nThis analysis builds a reversed trie over all AS_PATHs to measure\n")
	wf(w, "how much suffix sharing exists. High compression means a trie-based\n")
	wf(w, "storage would save significant memory vs storing full paths.\n\n")
	wf(w, "Files: %s\n", textbuf.Join(st.Files, ", "))
	wf(w, "Total AS_PATHs:  %s\n", formatNumber(st.TotalPaths))
	wf(w, "Unique AS_PATHs: %s (%5.2f%% whole-path dedup)\n", formatNumber(st.UniquePaths), pathDedup)

	wf(w, "\nSTORAGE COMPARISON:\n")
	wf(w, "  Naive (one ASN slot per path entry): %s ASN slots\n", formatNumber(naiveASNs))
	wf(w, "  Reversed trie nodes:                 %s nodes\n", formatNumber(trieNodes))
	wf(w, "  Compression ratio:                   %5.2f%%\n", trieRatio)
	wf(w, "  Memory: naive ~%s vs trie ~%s (4 bytes/ASN + pointers)\n",
		formatBytes(naiveASNs*4), formatBytes(trieNodes*4))

	wf(w, "\nPATH LENGTH DISTRIBUTION:\n")
	wf(w, "  %-10s %12s %8s\n", "Length", "Count", "Pct")
	wf(w, "  %s\n", strings.Repeat("-", 33))
	lengths := make([]int, 0, len(st.LengthDist))
	for l := range st.LengthDist {
		lengths = append(lengths, l)
	}
	sort.Ints(lengths)
	for _, l := range lengths {
		c := st.LengthDist[l]
		pct := float64(c) / float64(st.TotalPaths) * 100
		wf(w, "  %-10d %12s %7.2f%%\n", l, formatNumber(c), pct)
	}

	depthStats := make(map[int]uint64)
	trieDepthStats(st.trie, 0, depthStats)
	wf(w, "\nTRIE DEPTH (depth 0 = origin ASN, closer to root = more shared):\n")
	wf(w, "  %-10s %12s %8s\n", "Depth", "Nodes", "Pct")
	wf(w, "  %s\n", strings.Repeat("-", 33))
	depths := make([]int, 0, len(depthStats))
	for d := range depthStats {
		depths = append(depths, d)
	}
	sort.Ints(depths)
	for _, d := range depths {
		c := depthStats[d]
		pct := float64(c) / float64(trieNodes) * 100
		wf(w, "  %-10d %12s %7.2f%%\n", d, formatNumber(c), pct)
	}

	wf(w, "\nTOP ORIGIN ASNs (trie root children, most paths):\n")
	origins := collectOrigins(st.trie)
	for _, o := range origins {
		wf(w, "  AS%-10d %12s paths\n", o.ASN, formatNumber(o.Count))
	}

	fanout := make(map[int]uint64)
	trieFanoutStats(st.trie, fanout)
	single, multi := uint64(0), uint64(0)
	for f, c := range fanout {
		if f == 1 {
			single += c
		} else {
			multi += c
		}
	}
	wf(w, "\nFANOUT: %s single-child nodes (chain), %s multi-child (branch points)\n",
		formatNumber(single), formatNumber(multi))
	if single+multi > 0 {
		chainPct := float64(single) / float64(single+multi) * 100
		wf(w, "  Chain nodes: %.1f%% -- these could be collapsed (path compression)\n", chainPct)
	}
}
