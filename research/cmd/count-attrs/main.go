// count-attrs analyzes MRT TABLE_DUMP_V2 files to count attributes per route.
// Outputs a distribution table showing how many routes have N attributes.
//
// Usage: count-attrs <mrt.gz> [mrt2.gz ...]
//
// Example output:
//
//	Total routes: 19011954
//
//	| Attrs | Count | Percent | Cumulative |
//	|-------|-------|---------|------------|
//	| 3 | 4550673 | 23.94% | 23.94% |
//	| 4 | 7296730 | 38.38% | 62.32% |
//	...
package main

import (
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sort"
)

// MRT types (RFC 6396).
const (
	mrtTableDumpV2 = 13
)

// TABLE_DUMP_V2 subtypes.
const (
	ribIPv4Unicast = 2
	ribIPv6Unicast = 4
	ribGeneric     = 6
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: count-attrs <mrt.gz> [mrt2.gz ...]")
		os.Exit(1)
	}

	counts := make(map[int]int) // attrCount -> frequency
	total := 0

	for _, fname := range os.Args[1:] {
		f, err := os.Open(fname) //nolint:gosec // CLI tool intentionally opens user-provided files
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening %s: %v\n", fname, err)
			continue
		}
		gz, err := gzip.NewReader(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading gzip %s: %v\n", fname, err)
			_ = f.Close()
			continue
		}

		processFile(gz, counts, &total)
		_ = gz.Close()
		_ = f.Close()
	}

	// Print distribution
	fmt.Printf("Total routes: %d\n\n", total)

	keys := make([]int, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	// Print table format
	fmt.Println("| Attrs | Count | Percent | Cumulative |")
	fmt.Println("|-------|-------|---------|------------|")

	cumulative := 0
	for _, k := range keys {
		cumulative += counts[k]
		pct := float64(counts[k]) * 100 / float64(total)
		cum := float64(cumulative) * 100 / float64(total)
		fmt.Printf("| %d | %d | %.2f%% | %.2f%% |\n", k, counts[k], pct, cum)
	}
}

func processFile(r io.Reader, counts map[int]int, total *int) {
	header := make([]byte, 12)
	for {
		_, err := io.ReadFull(r, header)
		if err == io.EOF {
			break
		}
		if err != nil {
			return
		}

		mrtType := binary.BigEndian.Uint16(header[4:6])
		subtype := binary.BigEndian.Uint16(header[6:8])
		length := binary.BigEndian.Uint32(header[8:12])

		data := make([]byte, length)
		_, err = io.ReadFull(r, data)
		if err != nil {
			return
		}

		if mrtType != mrtTableDumpV2 {
			continue
		}

		switch subtype {
		case ribIPv4Unicast, ribIPv6Unicast, ribGeneric:
			processRIB(data, subtype, counts, total)
		}
	}
}

func processRIB(data []byte, subtype uint16, counts map[int]int, total *int) {
	if len(data) < 4 {
		return
	}

	offset := 4 // skip sequence number

	// Skip AFI/SAFI for RIB_GENERIC
	if subtype == ribGeneric {
		if offset+3 > len(data) {
			return
		}
		offset += 3 // AFI (2) + SAFI (1)
	}

	// Skip prefix
	if offset >= len(data) {
		return
	}
	prefixLen := int(data[offset])
	offset++
	prefixBytes := (prefixLen + 7) / 8
	offset += prefixBytes

	// Entry count
	if offset+2 > len(data) {
		return
	}
	entryCount := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2

	// Process each RIB entry
	for i := 0; i < int(entryCount); i++ {
		if offset+8 > len(data) {
			return
		}
		offset += 2 // peer index
		offset += 4 // originated time
		attrLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		offset += 2

		if offset+attrLen > len(data) {
			return
		}

		// Count attributes in this entry
		attrCount := countAttrs(data[offset : offset+attrLen])
		counts[attrCount]++
		*total++

		offset += attrLen
	}
}

// countAttrs counts the number of BGP path attributes in packed wire format.
func countAttrs(attrs []byte) int {
	count := 0
	offset := 0
	for offset < len(attrs) {
		if offset+2 > len(attrs) {
			break
		}
		flags := attrs[offset]
		offset += 2 // flags + type code

		var attrLen int
		if flags&0x10 != 0 { // extended length flag
			if offset+2 > len(attrs) {
				break
			}
			attrLen = int(binary.BigEndian.Uint16(attrs[offset : offset+2]))
			offset += 2
		} else {
			if offset >= len(attrs) {
				break
			}
			attrLen = int(attrs[offset])
			offset++
		}

		offset += attrLen
		count++
	}
	return count
}
