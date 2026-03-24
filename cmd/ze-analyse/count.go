// Design: (none -- research/analysis tool)
//
// Counts BGP attributes per route to produce a distribution table.
// Shows how many routes have N attributes -- useful for understanding
// attribute set complexity in real routing tables.
package main

import (
	"fmt"
	"os"
	"sort"
)

func runCountAttrs(args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, `ze-analyse count-attrs -- attribute count distribution per route

Processes TABLE_DUMP_V2 RIB entries and counts how many path attributes each
route carries. The distribution reveals the typical attribute set size,
which affects per-route memory and cache key width.

Usage:
  ze-analyse count-attrs <rib.gz> [rib2.gz ...]

Examples:
  ze-analyse count-attrs test/internet/latest-bview.gz
  ze-analyse count-attrs test/internet/rib.*.gz
`)
		return 1
	}

	counts := make(map[int]int) // attrCount -> frequency
	total := 0

	for _, fname := range args {
		if err := processMRTFile(fname, mrtHandler{
			OnRIB: func(data []byte, subtype uint16) {
				forEachRIBEntry(data, subtype, func(_ uint16, attrs []byte) {
					n := countAttrs(attrs)
					counts[n]++
					total++
				})
			},
		}); err != nil {
			fmt.Fprintf(os.Stderr, "error processing %s: %v\n", fname, err)
		}
	}

	// Print results.
	fmt.Println("BGP Attribute Count Distribution")
	fmt.Println("================================")
	fmt.Println()
	fmt.Println("How many path attributes does each route carry? Most routes have 3-5")
	fmt.Println("attributes (ORIGIN, AS_PATH, NEXT_HOP, optional MED/LOCAL_PREF/COMMUNITY).")
	fmt.Println("Routes with 6+ usually carry communities or extended attributes.")
	fmt.Println()
	fmt.Printf("Total routes: %s\n\n", formatNumber(uint64(total))) //nolint:gosec // non-negative

	keys := make([]int, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	fmt.Println("| Attrs | Count | Percent | Cumulative |")
	fmt.Println("|-------|-------|---------|------------|")

	cumulative := 0
	for _, k := range keys {
		cumulative += counts[k]
		pct := float64(counts[k]) * 100 / float64(total)
		cum := float64(cumulative) * 100 / float64(total)
		fmt.Printf("| %d | %d | %.2f%% | %.2f%% |\n", k, counts[k], pct, cum)
	}

	return 0
}
