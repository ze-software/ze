// Detail: the summaries these tests pin are the only rendering the analyze
// commands do by hand, so a chain that drops a separator has no other check.

package analyze

import (
	"io"
	"os"
	"testing"
)

// captureStd runs write with one of the process's standard files replaced by a
// pipe, and returns what the file received. The redirect is process-wide, so a
// test that calls it must not run in parallel. Each caller writes less than the
// pipe buffer, so the read after the write cannot deadlock.
func captureStd(t *testing.T, file **os.File, write func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	previous := *file
	*file = writer
	write()
	*file = previous
	if err := writer.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return string(out)
}

// TestStatsSummaryRendering pins every line printStatsSummary writes: the
// thousands separators, the column each count starts in, and the blank line
// before each section.
func TestStatsSummaryRendering(t *testing.T) {
	st := &mrtStats{
		TypeCounts:   map[string]uint64{"bgp4mp": 12345, "table-dump-v2": 7},
		AFICounts:    map[string]uint64{"ipv4-unicast": 999999, "a-very-long-afi-name-past-twenty": 1},
		BGPMsgTypes:  map[string]uint64{"update": 42},
		PeerCount:    3,
		AddPathCount: 1000,
		RIBEntries:   250000,
		MinTimestamp: 1600000000,
		MaxTimestamp: 1600003600,
		TotalRecords: 9876543,
	}
	// The long AFI name is past the 20-column pad, which widens its row rather
	// than truncating the name.
	const want = "\n" +
		"MRT Statistics Summary\n" +
		"=====================\n" +
		"\n" +
		"Total records: 9,876,543\n" +
		"RIB entries:   250,000\n" +
		"Peers:         3\n" +
		"Add-path:      1,000\n" +
		"Timestamp range: 1600000000 - 1600003600\n" +
		"\n" +
		"Records by type:\n" +
		"  bgp4mp               12,345\n" +
		"  table-dump-v2        7\n" +
		"\n" +
		"AFI breakdown:\n" +
		"  a-very-long-afi-name-past-twenty 1\n" +
		"  ipv4-unicast         999,999\n" +
		"\n" +
		"BGP message types:\n" +
		"  update               42\n"

	if got := captureStd(t, &os.Stderr, func() { printStatsSummary(st) }); got != want {
		t.Fatalf("stats summary rendering\n got %q\nwant %q", got, want)
	}
}

// TestCommYAMLHeaderRendering pins the comment header commPrintYAML writes
// before the ASN table, including the rounded threshold percentage.
func TestCommYAMLHeaderRendering(t *testing.T) {
	st := &commAnalysis{Files: []string{"a.mrt", "b.mrt"}, TotalRoutes: 123456}
	// 98.5 prints as 98: Go rounds half to even, and the header has always
	// printed it that way.
	const want = "# Per-ASN community defaults from MRT analysis\n" +
		"#\n" +
		"# Communities that appear in nearly every route from a given ASN can be\n" +
		"# assumed as 'defaults' in a cache. Only exceptions (routes missing a\n" +
		"# default community) need explicit encoding, saving wire bytes.\n" +
		"#\n" +
		"# Source: a.mrt, b.mrt\n" +
		"# Total routes analyzed: 123456\n" +
		"# Threshold for defaults: 98%\n" +
		"# Minimum routes per ASN: 50\n" +
		"# Mode: RAW (all communities as seen by route server)\n" +
		"\n"

	got := captureStd(t, &os.Stdout, func() { commPrintYAML(st, 0.985, 50, false) })
	if len(got) < len(want) || got[:len(want)] != want {
		t.Fatalf("YAML header rendering\n got %q\nwant prefix %q", got, want)
	}
}
