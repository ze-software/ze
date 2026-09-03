package irr

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
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

// VALIDATES: parseRegistryDelegation extracts ASN records with correct RIR and range.
// PREVENTS: wrong RIR assignment or range calculation.
func TestParseRegistryDelegation(t *testing.T) {
	entries, err := parseRegistryDelegation(strings.NewReader(fakeDelegation))
	if err != nil {
		t.Fatalf("parseRegistryDelegation: %v", err)
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
func TestParseRegistryDelegationSkipsReserved(t *testing.T) {
	entries, err := parseRegistryDelegation(strings.NewReader(fakeDelegation))
	if err != nil {
		t.Fatalf("parseRegistryDelegation: %v", err)
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

// buildTestTable creates a rirTable from the fake delegation data.
func buildTestTable(t *testing.T) *rirTable {
	t.Helper()
	entries, err := parseRegistryDelegation(strings.NewReader(fakeDelegation))
	if err != nil {
		t.Fatalf("parseRegistryDelegation: %v", err)
	}
	slices.SortFunc(entries, func(a, b RIREntry) int {
		return cmp.Compare(a.Start, b.Start)
	})
	collapsed, collapseErr := collapseRanges(entries)
	if collapseErr != nil {
		t.Fatalf("collapseRanges: %v", collapseErr)
	}
	return &rirTable{entries: collapsed}
}

// VALIDATES: rirForASN returns correct RIR for known allocations.
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
			e := table.rirForASN(tt.asn)
			if tt.wantNil {
				if e != nil {
					t.Errorf("rirForASN(%d) = %+v, want nil", tt.asn, e)
				}
				return
			}
			if e == nil {
				t.Fatalf("rirForASN(%d) = nil, want RIR=%s", tt.asn, tt.wantRIR)
				return // unreachable, satisfies staticcheck SA5011
			}
			if e.RIR != tt.wantRIR {
				t.Errorf("rirForASN(%d).RIR = %q, want %q", tt.asn, e.RIR, tt.wantRIR)
			}
			if e.Whois == "" {
				t.Errorf("rirForASN(%d).Whois is empty", tt.asn)
			}
		})
	}
}

// VALIDATES: whoisForASN returns the correct whois server.
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
		got := table.whoisForASN(tt.asn)
		if got != tt.wantWhois {
			t.Errorf("whoisForASN(%d) = %q, want %q", tt.asn, got, tt.wantWhois)
		}
	}
}

// VALIDATES: FetchDelegationTable returns an error for an unreachable server.
// PREVENTS: silent empty table on network failure.
func TestFetchDelegationTableUnreachable(t *testing.T) {
	// Save and restore original URLs.
	origURLs := rirDelegationURLs
	rirDelegationURLs = []string{"http://127.0.0.1:1/nonexistent"}
	defer func() { rirDelegationURLs = origURLs }()

	_, _, err := FetchDelegationTable(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

// VALIDATES: a delegation table file names each registry by its token, and the
// parser turns that token into the two interned constants.
// PREVENTS: a per-entry string allocation for each of the 11,000 ranges, and a
// table that carries a registry name Ze has no whois host for.
func TestARegistryTokenBecomesTheInternedConstants(t *testing.T) {
	table, err := parseRIRTable(strings.NewReader(seedHeader + "1 10 ripencc\n"))
	if err != nil {
		t.Fatalf("parseRIRTable: %v", err)
	}

	entry := table.rirForASN(5)
	if entry == nil {
		t.Fatal("AS5 is in no range of a table that holds 1-10")
		return // unreachable, satisfies staticcheck SA5011
	}
	if entry.RIR != RIRRIPE {
		t.Errorf("RIR not interned: %q", entry.RIR)
	}
	if entry.Whois != WhoisRIPE {
		t.Errorf("whois host not interned: %q", entry.Whois)
	}
}

// VALIDATES: parseRegistryDelegation handles malformed lines gracefully.
// PREVENTS: panic on truncated or corrupted delegation files.
func TestParseRegistryDelegationMalformed(t *testing.T) {
	input := "ripencc|DE|asn|3333|1|20000101|assigned\n" +
		"short|line\n" +
		"#comment\n" +
		"\n" +
		"ripencc|GB|asn|bad_number|1|20000101|assigned\n" +
		"ripencc|GB|asn|5000|0|20000101|assigned\n" + // zero count
		"ripencc|GB|asn|6000|1|20000101|available\n" + // not allocated/assigned
		"unknown|GB|asn|7000|1|20000101|assigned\n" + // unknown registry
		"ripencc|GB|asn|8000|1|20000101|assigned\n"

	entries, err := parseRegistryDelegation(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parseRegistryDelegation: %v", err)
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

// seedHeader is the smallest well-formed header a delegation table file can
// carry: the parser needs the generation date and nothing else.
const seedHeader = "# Generated: 2026-08-16\n"

// VALIDATES: the shipped delegation file parses and holds the whole table.
// PREVENTS: a truncated or corrupted seed shipping unnoticed and answering
// "unallocated" for every AS number.
func TestTheEmbeddedSeedParses(t *testing.T) {
	table, err := seedTable()
	if err != nil {
		t.Fatalf("seedTable: %v", err)
	}

	// The five registries held 11,256 ranges when the seed was generated. A
	// floor well under that catches a truncated file without failing on the
	// ordinary shrink a refresh produces when ranges merge.
	const rangesMin = 10000
	if table.Len() < rangesMin {
		t.Errorf("seed holds %d ranges, want at least %d", table.Len(), rangesMin)
	}

	if table.generated.IsZero() {
		t.Error("seed carries no generation date")
	}

	// AS3333 is RIPE NCC's own AS number and AS7018 is AT&T's, so both are
	// allocated in every vintage of the table.
	known := []struct {
		asn   uint32
		rir   string
		whois string
	}{
		{3333, RIRRIPE, WhoisRIPE},
		{7018, RIRARIN, WhoisARIN},
	}
	for _, want := range known {
		entry := table.rirForASN(want.asn)
		if entry == nil {
			t.Errorf("AS%d is in no range of the seed", want.asn)
			continue
		}
		if entry.RIR != want.rir || entry.Whois != want.whois {
			t.Errorf("AS%d = %s/%s, want %s/%s", want.asn, entry.RIR, entry.Whois, want.rir, want.whois)
		}
	}

	// Every entry names one of the five interned constants, so the table holds
	// five registry strings rather than one for each range.
	for i := range table.entries {
		entry := &table.entries[i]
		token, known := tokenForRegistry(entry.RIR)
		if !known {
			t.Fatalf("entry %d = %+v: the registry is none of the five", i, entry)
		}
		if rirWhois[token] != entry.Whois {
			t.Fatalf("entry %d = %+v: RIR and whois host disagree", i, entry)
		}
	}
}

// VALIDATES: every unreadable line in a delegation table file is an error that
// reaches the caller, and no such file yields a table.
// PREVENTS: a half-parsed or empty table answering "this AS number is in no
// delegated range" when the truth is that nothing could be read
// (ai/rules/principles.md).
func TestAMalformedLineIsAnErrorNotAnEmptyTable(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		wantIn string
	}{
		{"two fields", seedHeader + "1 6\n", "1 6"},
		{"four fields", seedHeader + "1 6 arin extra\n", "1 6 arin extra"},
		{"non-numeric start", seedHeader + "one 6 arin\n", "one 6 arin"},
		{"start above uint32", seedHeader + "4294967296 4294967296 arin\n", "4294967296 4294967296 arin"},
		{"end above uint32", seedHeader + "1 4294967296 arin\n", "1 4294967296 arin"},
		{"unknown registry token", seedHeader + "1 6 bogus\n", "bogus"},
		{"end before start", seedHeader + "6 1 arin\n", "6 1 arin"},
		{"out of order", seedHeader + "10 20 arin\n5 6 arin\n", "5 6 arin"},
		{"overlapping ranges", seedHeader + "10 20 arin\n15 25 ripencc\n", "15 25 ripencc"},
		{"no generation date", "# Source: none\n1 6 arin\n", "Generated"},
		{"unreadable generation date", "# Generated: yesterday\n1 6 arin\n", "yesterday"},
		{"no ranges at all", seedHeader, "no range"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table, err := parseRIRTable(strings.NewReader(tt.body))
			if err == nil {
				t.Fatalf("parseRIRTable accepted %q", tt.body)
			}
			if table != nil {
				t.Errorf("parseRIRTable returned a table with an error: %d ranges", table.Len())
			}
			if !strings.Contains(err.Error(), tt.wantIn) {
				t.Errorf("error %q does not name %q", err, tt.wantIn)
			}
		})
	}
}

// VALIDATES: "the table says nobody holds this AS number" and "the table could
// not be read" are two answers a caller can tell apart.
// PREVENTS: an unreadable table reporting every AS number as unallocated.
func TestAnASNInNoRangeIsDistinctFromAnUnreadableTable(t *testing.T) {
	// AS65535 is reserved, so no registry delegation covers it.
	const reservedASN = 65535

	_, err := RegistryForASN(reservedASN)
	if !errors.Is(err, ErrASNUnallocated) {
		t.Fatalf("RegistryForASN(%d) = %v, want ErrASNUnallocated", reservedASN, err)
	}

	table, parseErr := parseRIRTable(strings.NewReader(seedHeader + "nonsense\n"))
	if parseErr == nil {
		t.Fatal("parseRIRTable accepted a malformed table")
	}
	if table != nil {
		t.Fatal("an unreadable file produced a table")
	}
	if errors.Is(parseErr, ErrASNUnallocated) {
		t.Errorf("an unreadable table reports %v, the answer reserved for a readable one", parseErr)
	}

	// The same lookup over a table that cannot be read reports the table, and
	// AS3333 is delegated, so nothing but the unreadable source can produce
	// this answer.
	unreadable := func() (*rirTable, error) {
		return parseRIRTable(strings.NewReader(seedHeader + "nonsense\n"))
	}
	_, lookupErr := registryForASN(3333, unreadable)
	if lookupErr == nil {
		t.Fatal("a lookup over an unreadable table answered")
	}
	if errors.Is(lookupErr, ErrASNUnallocated) {
		t.Errorf("an unreadable table reports AS3333 as undelegated: %v", lookupErr)
	}
	if !strings.Contains(lookupErr.Error(), "nonsense") {
		t.Errorf("the lookup error does not name the offending line: %v", lookupErr)
	}
}

// VALIDATES: a delegation record whose start plus count minus one exceeds a
// uint32 is refused, and the record that ends exactly at the maximum is kept.
// PREVENTS: a wrapped range that claims every low AS number for one registry.
//
// Boundary: AS4294967295 is the last valid AS number, so a record ending there
// is the max valid case and a record ending one above it is invalid.
func TestAnOversizedRangeIsRefused(t *testing.T) {
	const input = "arin|US|asn|7018|1|19950101|assigned\n" +
		"arin|US|asn|4294967286|10|20000101|assigned\n" + // ends at 4294967295
		"arin|US|asn|4294967290|10|20000101|assigned\n" // ends at 4294967299

	entries, err := parseRegistryDelegation(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parseRegistryDelegation: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2: %v", len(entries), entries)
	}
	if entries[1].Start != 4294967286 || entries[1].End != 4294967295 {
		t.Errorf("entries[1] = %+v, want 4294967286-4294967295", entries[1])
	}
	for _, e := range entries {
		if e.Start == 4294967290 {
			t.Errorf("the oversized record was kept: %+v", e)
		}
	}
}

// VALIDATES: the ranges a collapse produces, and the ranges the shipped seed
// carries, are both sorted by start and hold no overlap.
// PREVENTS: a binary search over unsorted or overlapping ranges answering the
// wrong registry, or missing an allocated AS number.
func TestRangesAreSortedAndDisjointAfterCollapse(t *testing.T) {
	entries, err := parseRegistryDelegation(strings.NewReader(fakeDelegation))
	if err != nil {
		t.Fatalf("parseRegistryDelegation: %v", err)
	}
	slices.SortFunc(entries, func(a, b RIREntry) int {
		return cmp.Compare(a.Start, b.Start)
	})
	collapsed, collapseErr := collapseRanges(entries)
	if collapseErr != nil {
		t.Fatalf("collapseRanges: %v", collapseErr)
	}
	assertSortedAndDisjoint(t, "collapsed", collapsed)

	table, seedErr := seedTable()
	if seedErr != nil {
		t.Fatalf("seedTable: %v", seedErr)
	}
	assertSortedAndDisjoint(t, "seed", table.entries)
}

// assertSortedAndDisjoint reports every range that starts before the previous
// range ends, which is the precondition rirForASN's binary search needs.
func assertSortedAndDisjoint(t *testing.T, source string, entries []RIREntry) {
	t.Helper()
	for i := 1; i < len(entries); i++ {
		if entries[i].Start > entries[i-1].End {
			continue
		}
		t.Errorf("%s: range %d (%d-%d) is not after range %d (%d-%d)",
			source, i, entries[i].Start, entries[i].End,
			i-1, entries[i-1].Start, entries[i-1].End)
	}
}

// VALIDATES: what RenderDelegationTable writes, parseDelegationTable reads back
// as the same date and the same ranges.
// PREVENTS: the two halves of one format drifting apart, which is the defect
// that let a second parser exist at all: a refresh or a generator would write a
// file the lookup then reports as unreadable.
func TestARenderedTableParsesBackToWhatItHeld(t *testing.T) {
	want := DelegationTable{
		Generated: time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC),
		Ranges: []RIREntry{
			{Start: 1, End: 9, RIR: RIRARIN, Whois: WhoisARIN},
			{Start: 10, End: 10, RIR: RIRRIPE, Whois: WhoisRIPE},
			{Start: 4294967290, End: 4294967295, RIR: RIRLACNIC, Whois: WhoisLACNIC},
		},
	}

	body, err := RenderDelegationTable(want)
	if err != nil {
		t.Fatalf("RenderDelegationTable: %v", err)
	}

	got, err := parseDelegationTable(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("parseDelegationTable over the rendered table: %v\n%s", err, body)
	}

	if !got.Generated.Equal(want.Generated) {
		t.Errorf("the table parses back as generated %s, want %s", got.Generated, want.Generated)
	}
	if !slices.Equal(got.Ranges, want.Ranges) {
		t.Errorf("the table parses back as\n%+v\nwant\n%+v", got.Ranges, want.Ranges)
	}
}

// VALIDATES: the rendered header carries the generation date and one Source
// line for each registry file, and each range is start, end and token.
// PREVENTS: a file whose header says nothing about where its data came from,
// and a silent change of the line format the shipped seed is written in.
func TestTheRenderedTableIsByteExact(t *testing.T) {
	body, err := RenderDelegationTable(DelegationTable{
		Generated: time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC),
		Ranges: []RIREntry{
			{Start: 1, End: 9, RIR: RIRRIPE, Whois: WhoisRIPE},
			{Start: 10, End: 11, RIR: RIRARIN, Whois: WhoisARIN},
		},
	})
	if err != nil {
		t.Fatalf("RenderDelegationTable: %v", err)
	}

	want := delegationTableHeader +
		"# Generated: 2026-08-26\n" +
		"# Source: https://ftp.ripe.net/pub/stats/ripencc/delegated-ripencc-extended-latest\n" +
		"# Source: https://ftp.arin.net/pub/stats/arin/delegated-arin-extended-latest\n" +
		"# Source: https://ftp.apnic.net/pub/stats/apnic/delegated-apnic-extended-latest\n" +
		"# Source: https://ftp.afrinic.net/pub/stats/afrinic/delegated-afrinic-extended-latest\n" +
		"# Source: https://ftp.lacnic.net/pub/stats/lacnic/delegated-lacnic-extended-latest\n" +
		"1 9 ripencc\n" +
		"10 11 arin\n"

	if string(body) != want {
		t.Errorf("the table renders as\n%q\nwant\n%q", body, want)
	}
}

// VALIDATES: the renderer refuses every table the parser refuses, and writes
// nothing when it does.
// PREVENTS: a generator or a refresh replacing readable data with a file that
// parses to an error, which reports every AS number as unanswerable
// (ai/rules/principles.md).
func TestARenderRefusesWhatTheParserRefuses(t *testing.T) {
	dated := time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		table  DelegationTable
		wantIn string
	}{
		{"no generation date", DelegationTable{Ranges: []RIREntry{{Start: 1, End: 9, RIR: RIRARIN}}}, "no generation date"},
		{"no range at all", DelegationTable{Generated: dated}, "no range"},
		{"end before start", DelegationTable{Generated: dated, Ranges: []RIREntry{{Start: 9, End: 1, RIR: RIRARIN}}}, "ends below its start"},
		{"out of order", DelegationTable{Generated: dated, Ranges: []RIREntry{
			{Start: 10, End: 20, RIR: RIRARIN},
			{Start: 5, End: 6, RIR: RIRARIN},
		}}, "starts at or before AS20"},
		{"overlapping ranges", DelegationTable{Generated: dated, Ranges: []RIREntry{
			{Start: 10, End: 20, RIR: RIRARIN},
			{Start: 15, End: 25, RIR: RIRRIPE},
		}}, "starts at or before AS20"},
		{"unknown registry", DelegationTable{Generated: dated, Ranges: []RIREntry{{Start: 1, End: 9, RIR: "BOGUS"}}}, "BOGUS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := RenderDelegationTable(tt.table)
			if err == nil {
				t.Fatalf("RenderDelegationTable accepted %+v", tt.table)
			}
			if body != nil {
				t.Errorf("a refused render answered %d bytes", len(body))
			}
			if !strings.Contains(err.Error(), tt.wantIn) {
				t.Errorf("error %q does not name %q", err, tt.wantIn)
			}
		})
	}
}

// VALIDATES: rendering the shipped seed reproduces the committed file byte for
// byte, header included.
// PREVENTS: `./le iana-asn write` rewriting the whole file on its next run for
// a formatting reason, which buries the change of data in a change of shape.
func TestTheShippedSeedIsWhatTheRendererWrites(t *testing.T) {
	table, err := parseDelegationTable(strings.NewReader(seedDelegation))
	if err != nil {
		t.Fatalf("parseDelegationTable over the shipped seed: %v", err)
	}

	body, err := RenderDelegationTable(table)
	if err != nil {
		t.Fatalf("RenderDelegationTable over the shipped seed: %v", err)
	}

	if string(body) == seedDelegation {
		return
	}
	// The file is 11,266 lines, so report the first line that differs rather
	// than both copies.
	got := strings.Split(string(body), "\n")
	want := strings.Split(seedDelegation, "\n")
	for i := range min(len(got), len(want)) {
		if got[i] != want[i] {
			t.Fatalf("line %d renders as %q, and the shipped seed carries %q", i+1, got[i], want[i])
		}
	}
	t.Fatalf("the render holds %d lines and the shipped seed %d", len(got), len(want))
}
