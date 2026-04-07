// Design: (none -- predates documentation)
//
// RIR lookup: maps an ASN to its Regional Internet Registry and whois server.
// Two data sources:
//   - Seed data: compiled-in from scripts/codegen/iana_asn.go (committed to repo)
//   - Runtime update: downloaded from RIR delegation files via LoadRIRTable
//
// The seed data provides offline operation. `ze update bgp irr` refreshes
// from the 5 RIR delegation files and stores in zefs.
//
// Related: client.go -- IRR whois client
package irr

import (
	"bufio"
	"cmp"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Interned RIR names. All RIREntry.RIR fields point to these constants,
// so 10K+ entries share 5 string allocations instead of 10K+.
const (
	RIRRIPE    = "RIPE"
	RIRARIN    = "ARIN"
	RIRAPNIC   = "APNIC"
	RIRAFRINIC = "AFRINIC"
	RIRLACNIC  = "LACNIC"
)

// Interned whois servers. Same interning rationale as RIR names.
const (
	WhoisRIPE    = "whois.ripe.net"
	WhoisARIN    = "whois.arin.net"
	WhoisAPNIC   = "whois.apnic.net"
	WhoisAFRINIC = "whois.afrinic.net"
	WhoisLACNIC  = "whois.lacnic.net"
)

// rirWhois maps delegation file registry names to interned whois servers.
var rirWhois = map[string]string{
	"ripencc": WhoisRIPE,
	"arin":    WhoisARIN,
	"apnic":   WhoisAPNIC,
	"afrinic": WhoisAFRINIC,
	"lacnic":  WhoisLACNIC,
}

// rirNames maps delegation file registry names to interned canonical names.
var rirNames = map[string]string{
	"ripencc": RIRRIPE,
	"arin":    RIRARIN,
	"apnic":   RIRAPNIC,
	"afrinic": RIRAFRINIC,
	"lacnic":  RIRLACNIC,
}

// internedRIR maps any RIR name string to its interned constant.
// Used when loading from zefs or any io.Reader to avoid per-entry allocations.
var internedRIR = map[string]string{
	RIRRIPE: RIRRIPE, RIRARIN: RIRARIN, RIRAPNIC: RIRAPNIC,
	RIRAFRINIC: RIRAFRINIC, RIRLACNIC: RIRLACNIC,
}

// internedWhois maps any whois server string to its interned constant.
var internedWhois = map[string]string{
	WhoisRIPE: WhoisRIPE, WhoisARIN: WhoisARIN, WhoisAPNIC: WhoisAPNIC,
	WhoisAFRINIC: WhoisAFRINIC, WhoisLACNIC: WhoisLACNIC,
}

// InternRIREntry replaces the RIR and Whois strings in an entry with
// their interned constants. Returns false if the RIR is unknown.
func InternRIREntry(e *RIREntry) bool {
	rir, ok := internedRIR[e.RIR]
	if !ok {
		return false
	}
	e.RIR = rir
	if whois, wOk := internedWhois[e.Whois]; wOk {
		e.Whois = whois
	}
	return true
}

// Delegation file URLs for each RIR.
var rirDelegationURLs = []string{
	"https://ftp.ripe.net/pub/stats/ripencc/delegated-ripencc-extended-latest",
	"https://ftp.arin.net/pub/stats/arin/delegated-arin-extended-latest",
	"https://ftp.apnic.net/pub/stats/apnic/delegated-apnic-extended-latest",
	"https://ftp.afrinic.net/pub/stats/afrinic/delegated-afrinic-extended-latest",
	"https://ftp.lacnic.net/pub/stats/lacnic/delegated-lacnic-extended-latest",
}

// RIREntry describes an ASN range allocated to a Regional Internet Registry.
type RIREntry struct {
	Start uint32 // First ASN in range
	End   uint32 // Last ASN in range (inclusive)
	RIR   string // Registry name: RIPE, ARIN, APNIC, AFRINIC, LACNIC
	Whois string // Whois server for this range
}

// RIRTable holds the ASN-to-RIR mapping. Thread-safe after loading.
type RIRTable struct {
	entries []RIREntry
	mu      sync.RWMutex
}

// SeedRIRTable returns an RIRTable using the compiled-in seed data.
// No network access needed. Use this for offline operation or as initial state.
func SeedRIRTable() *RIRTable {
	return &RIRTable{entries: seedRIRTable}
}

// LoadRIRTable downloads all 5 RIR delegation files and builds a fresh
// ASN-to-RIR lookup table. Used by `ze update bgp irr` to refresh data.
func LoadRIRTable(ctx context.Context) (*RIRTable, error) {
	client := &http.Client{Timeout: 60 * time.Second}

	var allEntries []RIREntry

	for _, delegationURL := range rirDelegationURLs {
		entries, err := fetchDelegation(ctx, client, delegationURL)
		if err != nil {
			return nil, fmt.Errorf("irr: load RIR table: %w", err)
		}
		allEntries = append(allEntries, entries...)
	}

	slices.SortFunc(allEntries, func(a, b RIREntry) int {
		return cmp.Compare(a.Start, b.Start)
	})

	collapsed, err := collapseRanges(allEntries)
	if err != nil {
		return nil, fmt.Errorf("irr: load RIR table: %w", err)
	}
	return &RIRTable{entries: collapsed}, nil
}

// RIRForASN returns the RIR entry for the given ASN, or nil if the ASN
// is not allocated to any RIR (reserved, unallocated, or documentation range).
// The returned pointer is valid for the lifetime of the RIRTable. If the table
// is replaced (e.g., by a runtime update), old pointers remain valid but stale.
func (t *RIRTable) RIRForASN(asn uint32) *RIREntry {
	t.mu.RLock()
	defer t.mu.RUnlock()

	lo, hi := 0, len(t.entries)-1
	for lo <= hi {
		mid := lo + (hi-lo)/2
		e := &t.entries[mid]
		switch {
		case asn < e.Start:
			hi = mid - 1
		case asn > e.End:
			lo = mid + 1
		case asn >= e.Start && asn <= e.End: // found: ASN within range
			return e
		}
	}
	return nil
}

// WhoisForASN returns the whois server for the given ASN's RIR.
// Returns empty string if the ASN is not allocated.
func (t *RIRTable) WhoisForASN(asn uint32) string {
	if e := t.RIRForASN(asn); e != nil {
		return e.Whois
	}
	return ""
}

// Len returns the number of collapsed ranges in the table.
func (t *RIRTable) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.entries)
}

// fetchDelegation downloads a single RIR delegation file and extracts ASN records.
func fetchDelegation(ctx context.Context, client *http.Client, delegationURL string) ([]RIREntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, delegationURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request for %s: %w", delegationURL, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", delegationURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %d", delegationURL, resp.StatusCode)
	}

	// Limit response to 20 MB (largest RIR file is ~10 MB).
	const maxDelegationSize = 20 << 20
	limited := io.LimitReader(resp.Body, maxDelegationSize)
	return parseDelegation(limited)
}

// parseDelegation extracts ASN records from a delegation file body.
// Format: registry|cc|type|start|value|date|status (with optional opaque-id).
// Records with unknown registries, parse errors, or non-ASN types are silently skipped.
func parseDelegation(r io.Reader) ([]RIREntry, error) {
	var entries []RIREntry
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || line[0] == '#' {
			continue
		}

		fields := strings.Split(line, "|")
		if len(fields) < 7 {
			continue
		}

		registry := fields[0]
		recType := fields[2]
		startStr := fields[3]
		countStr := fields[4]
		status := fields[6]

		if recType != "asn" {
			continue
		}
		if status != "allocated" && status != "assigned" {
			continue
		}

		rir, knownRIR := rirNames[registry]
		if !knownRIR {
			continue
		}
		whois := rirWhois[registry]

		start, err := strconv.ParseUint(startStr, 10, 32)
		if err != nil {
			continue
		}
		count, err := strconv.ParseUint(countStr, 10, 32)
		if err != nil || count == 0 {
			continue
		}
		// Guard against overflow: start + count - 1 must fit in uint32.
		if start+count-1 > math.MaxUint32 {
			continue
		}

		entries = append(entries, RIREntry{
			Start: uint32(start),
			End:   uint32(start + count - 1),
			RIR:   rir,
			Whois: whois,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read delegation: %w", err)
	}

	return entries, nil
}

// collapseRanges merges adjacent or overlapping ranges with the same RIR.
// Input MUST be sorted by Start. Returns error if unsorted input detected.
func collapseRanges(entries []RIREntry) ([]RIREntry, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	for i := 1; i < len(entries); i++ {
		if entries[i].Start < entries[i-1].Start {
			return nil, fmt.Errorf("collapseRanges: unsorted input at index %d: %d < %d", i, entries[i].Start, entries[i-1].Start)
		}
	}

	result := make([]RIREntry, 0, len(entries)/4)
	current := entries[0]

	for _, e := range entries[1:] {
		if e.RIR == current.RIR && e.Start <= current.End+1 {
			if e.End > current.End {
				current.End = e.End
			}
			continue
		}
		result = append(result, current)
		current = e
	}
	result = append(result, current)

	return result, nil
}
