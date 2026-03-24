package irr

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"testing"
)

// fakeDelegation is a minimal RIR delegation file with ASN records.
const fakeDelegation = `2.3|testreg|20260324|5|19830101|20260324|+0000
testreg|*|asn|*|5|summary
ripencc|DE|asn|3333|1|20000101|assigned
ripencc|GB|asn|60000|100|20100101|allocated
arin|US|asn|7018|1|19950101|assigned
arin|US|asn|397213|1|20160315|assigned
apnic|JP|asn|4608|10|19960101|allocated
apnic|AU|asn|131072|1024|20100901|assigned
afrinic|ZA|asn|37100|50|20050101|allocated
lacnic|BR|asn|28000|100|20030101|allocated
arin|US|asn|64512|256|20000101|reserved
`

// VALIDATES: parseDelegation extracts ASN records with correct RIR and range.
// PREVENTS: wrong RIR assignment or range calculation.
func TestParseDelegation(t *testing.T) {
	entries, err := parseDelegation(strings.NewReader(fakeDelegation))
	if err != nil {
		t.Fatalf("parseDelegation: %v", err)
	}

	// Should skip summary line and reserved line.
	// Expect 8 allocated/assigned records.
	if len(entries) != 8 {
		t.Fatalf("got %d entries, want 8", len(entries))
	}

	// Check first entry (RIPE, ASN 3333).
	e := entries[0]
	if e.Start != 3333 || e.End != 3333 || e.RIR != "RIPE" || e.Whois != "whois.ripe.net" {
		t.Errorf("entries[0] = %+v, want RIPE ASN 3333", e)
	}

	// Check APNIC 32-bit range.
	found := false
	for _, entry := range entries {
		if entry.Start == 131072 && entry.End == 132095 && entry.RIR == "APNIC" {
			found = true
			break
		}
	}
	if !found {
		t.Error("missing APNIC 32-bit range 131072-132095")
	}
}

// VALIDATES: reserved ASN ranges are excluded from the table.
// PREVENTS: reserved/private ASNs appearing as allocated.
func TestParseDelegationSkipsReserved(t *testing.T) {
	entries, err := parseDelegation(strings.NewReader(fakeDelegation))
	if err != nil {
		t.Fatalf("parseDelegation: %v", err)
	}

	for _, e := range entries {
		if e.Start >= 64512 && e.Start <= 64767 {
			t.Errorf("reserved ASN %d should be excluded, got %+v", e.Start, e)
		}
	}
}

// VALIDATES: collapseRanges merges adjacent ranges with the same RIR.
// PREVENTS: bloated table with thousands of single-ASN entries.
func TestCollapseRanges(t *testing.T) {
	input := []RIREntry{
		{100, 100, "RIPE", "whois.ripe.net"},
		{101, 101, "RIPE", "whois.ripe.net"},
		{102, 102, "RIPE", "whois.ripe.net"},
		{200, 200, "ARIN", "whois.arin.net"},
		{201, 205, "ARIN", "whois.arin.net"},
		{300, 300, "RIPE", "whois.ripe.net"},
	}

	result, err := collapseRanges(input)
	if err != nil {
		t.Fatalf("collapseRanges: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("got %d ranges, want 3: %v", len(result), result)
	}

	// First: RIPE 100-102 (three singles merged).
	if result[0].Start != 100 || result[0].End != 102 || result[0].RIR != "RIPE" {
		t.Errorf("result[0] = %+v, want RIPE 100-102", result[0])
	}

	// Second: ARIN 200-205 (adjacent merged).
	if result[1].Start != 200 || result[1].End != 205 || result[1].RIR != "ARIN" {
		t.Errorf("result[1] = %+v, want ARIN 200-205", result[1])
	}

	// Third: RIPE 300 (different RIR breaks the chain).
	if result[2].Start != 300 || result[2].End != 300 || result[2].RIR != "RIPE" {
		t.Errorf("result[2] = %+v, want RIPE 300-300", result[2])
	}
}

// buildTestTable creates an RIRTable from the fake delegation data.
func buildTestTable(t *testing.T) *RIRTable {
	t.Helper()
	entries, err := parseDelegation(strings.NewReader(fakeDelegation))
	if err != nil {
		t.Fatalf("parseDelegation: %v", err)
	}
	slices.SortFunc(entries, func(a, b RIREntry) int {
		return cmp.Compare(a.Start, b.Start)
	})
	collapsed, collapseErr := collapseRanges(entries)
	if collapseErr != nil {
		t.Fatalf("collapseRanges: %v", collapseErr)
	}
	return &RIRTable{entries: collapsed}
}

// VALIDATES: RIRForASN returns correct RIR for known allocations.
// PREVENTS: wrong RIR for well-known ASNs.
func TestRIRForASN(t *testing.T) {
	table := buildTestTable(t)

	tests := []struct {
		name    string
		asn     uint32
		wantRIR string
		wantNil bool
	}{
		{"RIPE single", 3333, "RIPE", false},
		{"ARIN single", 7018, "ARIN", false},
		{"APNIC range start", 4608, "APNIC", false},
		{"APNIC range end", 4617, "APNIC", false},
		{"LACNIC range", 28050, "LACNIC", false},
		{"AFRINIC range", 37120, "AFRINIC", false},
		{"ARIN 32-bit", 397213, "ARIN", false},
		{"APNIC 32-bit", 131100, "APNIC", false},
		{"not allocated", 99999, "", true},
		{"reserved private", 64512, "", true},
		{"zero", 0, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := table.RIRForASN(tt.asn)
			if tt.wantNil {
				if e != nil {
					t.Errorf("RIRForASN(%d) = %+v, want nil", tt.asn, e)
				}
				return
			}
			if e == nil {
				t.Fatalf("RIRForASN(%d) = nil, want RIR=%s", tt.asn, tt.wantRIR)
				return // unreachable, satisfies staticcheck SA5011
			}
			if e.RIR != tt.wantRIR {
				t.Errorf("RIRForASN(%d).RIR = %q, want %q", tt.asn, e.RIR, tt.wantRIR)
			}
			if e.Whois == "" {
				t.Errorf("RIRForASN(%d).Whois is empty", tt.asn)
			}
		})
	}
}

// VALIDATES: WhoisForASN returns the correct whois server.
// PREVENTS: empty whois server for allocated ASNs.
func TestWhoisForASN(t *testing.T) {
	table := buildTestTable(t)

	tests := []struct {
		asn       uint32
		wantWhois string
	}{
		{3333, "whois.ripe.net"},
		{7018, "whois.arin.net"},
		{4608, "whois.apnic.net"},
		{37100, "whois.afrinic.net"},
		{28000, "whois.lacnic.net"},
		{0, ""},
		{64512, ""},
	}

	for _, tt := range tests {
		got := table.WhoisForASN(tt.asn)
		if got != tt.wantWhois {
			t.Errorf("WhoisForASN(%d) = %q, want %q", tt.asn, got, tt.wantWhois)
		}
	}
}

// VALIDATES: LoadRIRTable returns error for unreachable server.
// PREVENTS: silent empty table on network failure.
func TestLoadRIRTableUnreachable(t *testing.T) {
	// Save and restore original URLs.
	origURLs := rirDelegationURLs
	rirDelegationURLs = []string{"http://127.0.0.1:1/nonexistent"}
	defer func() { rirDelegationURLs = origURLs }()

	_, err := LoadRIRTable(context.Background())
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

// VALIDATES: SeedRIRTable returns a non-empty table with known ASNs.
// PREVENTS: broken generated rir_table.go going unnoticed.
func TestSeedRIRTable(t *testing.T) {
	table := SeedRIRTable()
	if table.Len() == 0 {
		t.Fatal("seed table is empty")
	}

	// AS3333 is RIPE NCC's own ASN -- always allocated.
	e := table.RIRForASN(3333)
	if e == nil {
		t.Fatal("seed table missing AS3333 (RIPE)")
		return // unreachable, satisfies staticcheck SA5011
	}
	if e.RIR != RIRRIPE {
		t.Errorf("AS3333 RIR = %q, want %q", e.RIR, RIRRIPE)
	}

	// AS7018 is AT&T -- always allocated by ARIN.
	e = table.RIRForASN(7018)
	if e == nil {
		t.Fatal("seed table missing AS7018 (ARIN)")
		return // unreachable, satisfies staticcheck SA5011
	}
	if e.RIR != RIRARIN {
		t.Errorf("AS7018 RIR = %q, want %q", e.RIR, RIRARIN)
	}
}

// VALIDATES: InternRIREntry replaces strings with interned constants.
// PREVENTS: per-entry string allocations when loading from zefs.
func TestInternRIREntry(t *testing.T) {
	e := RIREntry{Start: 1, End: 10, RIR: "RIPE", Whois: "whois.ripe.net"}
	if !InternRIREntry(&e) {
		t.Fatal("InternRIREntry returned false for valid RIR")
	}
	// Verify the string pointers are the constants (same address).
	if e.RIR != RIRRIPE {
		t.Errorf("RIR not interned: %q", e.RIR)
	}
	if e.Whois != WhoisRIPE {
		t.Errorf("Whois not interned: %q", e.Whois)
	}
}

// VALIDATES: InternRIREntry returns false for unknown RIR.
// PREVENTS: accepting garbage data when loading from external source.
func TestInternRIREntryUnknown(t *testing.T) {
	e := RIREntry{Start: 1, End: 10, RIR: "BOGUS", Whois: "whois.bogus.net"}
	if InternRIREntry(&e) {
		t.Error("InternRIREntry returned true for unknown RIR")
	}
}

// VALIDATES: parseDelegation handles malformed lines gracefully.
// PREVENTS: panic on truncated or corrupted delegation files.
func TestParseDelegationMalformed(t *testing.T) {
	input := "ripencc|DE|asn|3333|1|20000101|assigned\n" +
		"short|line\n" +
		"#comment\n" +
		"\n" +
		"ripencc|GB|asn|bad_number|1|20000101|assigned\n" +
		"ripencc|GB|asn|5000|0|20000101|assigned\n" + // zero count
		"ripencc|GB|asn|6000|1|20000101|available\n" + // not allocated/assigned
		"unknown|GB|asn|7000|1|20000101|assigned\n" + // unknown registry
		"ripencc|GB|asn|8000|1|20000101|assigned\n"

	entries, err := parseDelegation(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parseDelegation: %v", err)
	}

	// Should only get ASN 3333 and 8000 (all others filtered).
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2: %v", len(entries), entries)
	}
	if entries[0].Start != 3333 {
		t.Errorf("entries[0].Start = %d, want 3333", entries[0].Start)
	}
	if entries[1].Start != 8000 {
		t.Errorf("entries[1].Start = %d, want 8000", entries[1].Start)
	}
}

// VALIDATES: collapseRanges handles empty input.
// PREVENTS: panic on nil/empty slice.
func TestCollapseRangesEmpty(t *testing.T) {
	result, err := collapseRanges(nil)
	if err != nil {
		t.Fatalf("collapseRanges: %v", err)
	}
	if result != nil {
		t.Errorf("got %v, want nil", result)
	}
}

// VALIDATES: collapseRanges handles single entry.
// PREVENTS: off-by-one in collapse loop.
func TestCollapseRangesSingle(t *testing.T) {
	input := []RIREntry{{100, 200, RIRRIPE, WhoisRIPE}}
	result, err := collapseRanges(input)
	if err != nil {
		t.Fatalf("collapseRanges: %v", err)
	}
	if len(result) != 1 || result[0].Start != 100 || result[0].End != 200 {
		t.Errorf("got %v, want [{100 200 RIPE ...}]", result)
	}
}

// VALIDATES: collapseRanges handles overlapping ranges (not just adjacent).
// PREVENTS: overlapping entries creating gaps in lookup.
func TestCollapseRangesOverlap(t *testing.T) {
	input := []RIREntry{
		{100, 150, RIRRIPE, WhoisRIPE},
		{120, 200, RIRRIPE, WhoisRIPE}, // overlaps
	}
	result, err := collapseRanges(input)
	if err != nil {
		t.Fatalf("collapseRanges: %v", err)
	}
	if len(result) != 1 || result[0].Start != 100 || result[0].End != 200 {
		t.Errorf("got %v, want [{100 200 RIPE ...}]", result)
	}
}

// VALIDATES: collapseRanges rejects unsorted input.
// PREVENTS: silent wrong results from unsorted data.
func TestCollapseRangesUnsorted(t *testing.T) {
	input := []RIREntry{
		{200, 200, RIRARIN, WhoisARIN},
		{100, 100, RIRRIPE, WhoisRIPE}, // out of order
	}
	_, err := collapseRanges(input)
	if err == nil {
		t.Fatal("expected error for unsorted input")
	}
}
