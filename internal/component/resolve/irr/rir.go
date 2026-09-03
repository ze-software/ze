// Design: docs/architecture/resolve.md -- the RIR delegation table
//
// RIR lookup: maps an ASN to its Regional Internet Registry and whois server.
// Two data sources, one format and one parser:
//   - The shipped seed: rir-delegation.txt, embedded by seed.go
//   - A runtime refresh: the five registry delegation files, read by
//     FetchDelegationTable and stored in the managed zefs store
//
// The seed data provides offline operation. `update resolve rir` refreshes from
// the 5 RIR delegation files and stores the result in the managed zefs store
// under the meta/rir/delegation key. The stored copy answers the lookup when it
// is newer than the seed, which stored.go decides.
//
// RegistryForASN is the one entry point every caller uses.
//
// Two formats meet here, and this package owns both. What a registry publishes
// is read by parseRegistryDelegation, and what Ze ships and stores is written
// by RenderDelegationTable and read by parseDelegationTable. `./le iana-asn
// write` calls into this package rather than declaring a second parser:
// the two copies that used to exist each held a guard the other lacked.
//
// Related: seed.go -- the embedded seed and its lazily parsed accessor
// Related: client.go -- IRR whois client
package irr

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
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

// Registry tokens, as the delegation files themselves write them. Every table
// below is keyed by one, an operator names a source by one, and the YANG
// enumeration offers the same five, so the token is declared here once rather
// than spelled again in each map.
const (
	registryRIPE    = "ripencc"
	registryARIN    = "arin"
	registryAPNIC   = "apnic"
	registryAFRINIC = "afrinic"
	registryLACNIC  = "lacnic"
)

// rirWhois maps delegation file registry names to interned whois servers.
var rirWhois = map[string]string{
	registryRIPE:    WhoisRIPE,
	registryARIN:    WhoisARIN,
	registryAPNIC:   WhoisAPNIC,
	registryAFRINIC: WhoisAFRINIC,
	registryLACNIC:  WhoisLACNIC,
}

// rirNames maps delegation file registry names to interned canonical names.
var rirNames = map[string]string{
	registryRIPE:    RIRRIPE,
	registryARIN:    RIRARIN,
	registryAPNIC:   RIRAPNIC,
	registryAFRINIC: RIRAFRINIC,
	registryLACNIC:  RIRLACNIC,
}

// publishedDelegation names the file each registry publishes, in the order a
// refresh reads them.
//
// The token is carried beside the URL rather than left implicit in its path,
// because an operator names a source BY registry and a run has to say which
// of the five a URL replaces (system { rir { delegation-source } }).
var publishedDelegation = []struct {
	token string
	url   string
}{
	{registryRIPE, "https://ftp.ripe.net/pub/stats/ripencc/delegated-ripencc-extended-latest"},
	{registryARIN, "https://ftp.arin.net/pub/stats/arin/delegated-arin-extended-latest"},
	{registryAPNIC, "https://ftp.apnic.net/pub/stats/apnic/delegated-apnic-extended-latest"},
	{registryAFRINIC, "https://ftp.afrinic.net/pub/stats/afrinic/delegated-afrinic-extended-latest"},
	{registryLACNIC, "https://ftp.lacnic.net/pub/stats/lacnic/delegated-lacnic-extended-latest"},
}

// delegationSourceURLs answers the URL a run reads for each registry: the one
// an operator named for it, and the file that registry publishes otherwise.
//
// A source keyed by a token no registry spells is an ERROR rather than a
// source nobody reads. An operator who misspells a registry has named no
// mirror, and a run that quietly read all five published files would report
// success over data they never asked for (ai/rules/principles.md).
func delegationSourceURLs(sources map[string]string) ([]string, error) {
	for token := range sources {
		if _, known := rirNames[token]; !known {
			return nil, fmt.Errorf("delegation source names %q, which no registry spells: %s",
				token, strings.Join(registryTokens(), ", "))
		}
	}

	urls := make([]string, 0, len(publishedDelegation))
	for _, published := range publishedDelegation {
		if configured, named := sources[published.token]; named {
			urls = append(urls, configured)
			continue
		}
		urls = append(urls, published.url)
	}
	return urls, nil
}

// registryTokens answers the five tokens, sorted, for a message that has to
// name what a caller could have written instead.
func registryTokens() []string {
	tokens := make([]string, 0, len(publishedDelegation))
	for _, published := range publishedDelegation {
		tokens = append(tokens, published.token)
	}
	slices.Sort(tokens)
	return tokens
}

// RIREntry describes an ASN range allocated to a Regional Internet Registry.
type RIREntry struct {
	Start uint32 // First ASN in range
	End   uint32 // Last ASN in range (inclusive)
	RIR   string // Registry name: RIPE, ARIN, APNIC, AFRINIC, LACNIC
	Whois string // Whois server for this range
}

// ErrASNUnallocated reports that the delegation table was read and the AS
// number sits in no delegated range. An unreadable table is a different
// answer and never produces this error: "nobody holds this AS number" and
// "I could not find out" MUST stay distinguishable (ai/rules/principles.md).
var ErrASNUnallocated = errors.New("irr: AS number is in no delegated range")

// RegistryForASN answers which Regional Internet Registry holds asn, and is
// the entry point the host CLI and both daemon handlers call.
//
// It has three outcomes, and a caller MUST branch on all three:
//   - an entry and a nil error: this registry holds the AS number.
//   - a zero entry and ErrASNUnallocated: the table was read and the AS number
//     lies in no delegated range.
//   - a zero entry and any other error: the table could not be read, so
//     nothing at all is known about the AS number.
func RegistryForASN(asn uint32) (RIREntry, error) {
	return registryForASN(asn, delegationTable)
}

// registryForASN answers from whichever table source it is given, and holds
// the three outcomes RegistryForASN contracts.
//
// The source is a parameter because the unreadable outcome is unreachable
// through the shipped seed, which parses. A caller that hands it a source it
// cannot read is how that branch is exercised.
func registryForASN(asn uint32, source func() (*rirTable, error)) (RIREntry, error) {
	table, err := source()
	if err != nil {
		return RIREntry{}, fmt.Errorf("irr: cannot answer AS%d: %w", asn, err)
	}

	entry := table.rirForASN(asn)
	if entry == nil {
		return RIREntry{}, fmt.Errorf("AS%d: %w", asn, ErrASNUnallocated)
	}
	return *entry, nil
}

// rirTable holds the ASN-to-RIR mapping in the form the lookup searches.
//
// Safe for concurrent use, because it is IMMUTABLE. parseRIRTable is its one
// constructor, it fills both fields before it hands the pointer out, and
// nothing writes either field afterwards. A refresh does not mutate a table:
// delegationTable answers a table parsed from whichever source won, so a new
// answer is a new table and every pointer into an old one stays true to the
// data it was read from.
type rirTable struct {
	entries []RIREntry
	// generated is the date the registries' data was collected. The lookup
	// prefers the newer of the shipped seed and the stored copy, so a table
	// with no date is never built.
	generated time.Time
}

// DelegationTable is one whole delegation table: the date the registries' data
// was collected, and the ranges they delegate.
//
// It is what one table file holds, and the shipped seed and a refreshed copy
// hold the same thing. RenderDelegationTable writes it and parseDelegationTable
// reads it, so what one produces the other accepts.
type DelegationTable struct {
	// Generated is the date the registries' data was collected. The lookup
	// prefers the newer of the shipped seed and a stored copy, so a table
	// with no date is never built.
	Generated time.Time
	// Ranges are sorted by Start and never overlap, which is the precondition
	// the lookup's binary search needs.
	Ranges []RIREntry
	// Sources are the URLs this table was read from, in registry order. They
	// are the table's PROVENANCE: an operator who points a registry at a
	// mirror can read back which file each range came from, and a table that
	// claimed the registries' own URLs while reading a mirror would be a lie
	// the header tells.
	Sources []string
}

// delegationTableHeader opens every table file. The file is data, met by a
// reader who does not have this package open beside it, so the header names
// the producer, the line format and the consequence of an edit.
//
// RenderDelegationTable writes the Generated: and Source: lines after it, and
// parseDelegationTable reads the first of those two back.
const delegationTableHeader = "" +
	"# Ze RIR delegation table, written by ./le iana-asn write. One line for\n" +
	"# each range: <start> <end> <registry-token>. The registry display name\n" +
	"# and its whois host are Go constants in rir.go, so this file carries the\n" +
	"# token alone.\n" +
	"#\n" +
	"# An edit here is lost at the next run, which reads the five source files\n" +
	"# below and rewrites the whole table.\n" +
	"#\n"

// parseDelegationTable reads a delegation table file: comment lines carrying
// the generation date and the source URLs, then one line for each range as
// start, end and registry token.
//
// A line it cannot read is an error, and an error returns no table at all. A
// caller MUST NOT receive a partly read table it would take for a whole one
// (ai/rules/principles.md). The ranges MUST arrive sorted by start and MUST
// NOT overlap, which is the precondition rirForASN's binary search needs.
//
// It stays unexported because no package outside this one reads a table: the
// generator writes one through RenderDelegationTable, and this package reads
// both the embedded seed and the stored copy.
func parseDelegationTable(r io.Reader) (DelegationTable, error) {
	var table DelegationTable

	scanner := bufio.NewScanner(r)
	for number := 1; scanner.Scan(); number++ {
		text := scanner.Text()
		if text == "" {
			continue
		}

		if text[0] == '#' {
			if source, named := sourceStamp(text); named {
				table.Sources = append(table.Sources, source)
				continue
			}
			stamp, dated := generationStamp(text)
			if !dated {
				continue
			}
			date, err := time.Parse(time.DateOnly, stamp)
			if err != nil {
				return DelegationTable{}, fmt.Errorf("delegation table line %d: generation date %q: %w", number, stamp, err)
			}
			table.Generated = date
			continue
		}

		entry, err := parseRIRRange(text)
		if err != nil {
			return DelegationTable{}, fmt.Errorf("delegation table line %d: %q: %w", number, text, err)
		}
		if len(table.Ranges) > 0 && entry.Start <= table.Ranges[len(table.Ranges)-1].End {
			return DelegationTable{}, fmt.Errorf("delegation table line %d: %q: starts at or before AS%d, where the previous range ends",
				number, text, table.Ranges[len(table.Ranges)-1].End)
		}
		table.Ranges = append(table.Ranges, entry)
	}
	if err := scanner.Err(); err != nil {
		return DelegationTable{}, fmt.Errorf("read delegation table: %w", err)
	}

	if table.Generated.IsZero() {
		return DelegationTable{}, errors.New("delegation table carries no Generated: date")
	}
	if len(table.Ranges) == 0 {
		return DelegationTable{}, errors.New("delegation table holds no range")
	}
	return table, nil
}

// RenderDelegationTable writes the bytes parseDelegationTable reads: the
// header, the generation date, one Source: line for each registry file, and
// one line for each range.
//
// It refuses what parseDelegationTable refuses, so a table it returns is a
// table this package reads back. A file nothing can read is not a smaller
// answer: it is every lookup reporting an unreadable table. That is why an
// absent date, an empty range set, an out-of-order range and an unknown
// registry each stop the render rather than reaching the file.
//
// It is exported for the generator that writes the shipped table
// (internal/le/ianaasn). A stored copy carries these same bytes.
func RenderDelegationTable(table DelegationTable) ([]byte, error) {
	if table.Generated.IsZero() {
		return nil, errors.New("delegation table: no generation date")
	}
	if len(table.Ranges) == 0 {
		return nil, errors.New("delegation table: no range")
	}
	if len(table.Sources) == 0 {
		return nil, errors.New("delegation table: no source, so the file would not say where its ranges were read from")
	}

	// A range line is two AS numbers of ten digits at most, the longest token,
	// two spaces and a newline. Asking for the whole file once keeps the
	// eleven thousand appends of the shipped table to one allocation.
	const rangeLineMax = 10 + 1 + 10 + 1 + 7 + 1

	var out textbuf.Buffer
	out.Grow(len(delegationTableHeader) + len(table.Ranges)*rangeLineMax)

	out.Str(delegationTableHeader)
	out.Str("# Generated: ").Str(table.Generated.Format(time.DateOnly)).Byte('\n')
	for _, source := range table.Sources {
		out.Str("# Source: ").Str(source).Byte('\n')
	}

	for i, entry := range table.Ranges {
		if entry.End < entry.Start {
			return nil, fmt.Errorf("delegation table: the range AS%d to AS%d ends below its start", entry.Start, entry.End)
		}
		if i > 0 && entry.Start <= table.Ranges[i-1].End {
			return nil, fmt.Errorf("delegation table: AS%d starts at or before AS%d, where the previous range ends",
				entry.Start, table.Ranges[i-1].End)
		}
		token, known := tokenForRegistry(entry.RIR)
		if !known {
			return nil, fmt.Errorf("delegation table: the range AS%d to AS%d names %q, which no registry token spells",
				entry.Start, entry.End, entry.RIR)
		}
		out.Uint(uint64(entry.Start)).Byte(' ').Uint(uint64(entry.End)).Byte(' ').Str(token).Byte('\n')
	}

	return slices.Clone(out.Bytes()), nil
}

// tokenForRegistry answers the token a delegation file spells a registry name
// as, and reports whether the name is one of the five.
//
// It reads rirNames backwards rather than holding a second map, so a registry
// and its token stay declared once.
func tokenForRegistry(name string) (string, bool) {
	for token, rir := range rirNames {
		if rir == name {
			return token, true
		}
	}
	return "", false
}

// parseRIRTable reads a delegation table file into the form the lookup
// searches. It is the one constructor of rirTable, so every table carries the
// date the stored-copy precedence compares.
func parseRIRTable(r io.Reader) (*rirTable, error) {
	table, err := parseDelegationTable(r)
	if err != nil {
		return nil, err
	}
	return &rirTable{entries: table.Ranges, generated: table.Generated}, nil
}

// sourceStamp reads the Source: value out of a comment line, and reports
// whether the line carries one.
//
// The parser keeps these rather than passing over them, so a table read back
// still says where it came from: RenderDelegationTable writes what this reads,
// and a stored copy's provenance survives a parse.
func sourceStamp(comment string) (string, bool) {
	value, ok := strings.CutPrefix(strings.TrimSpace(strings.TrimPrefix(comment, "#")), "Source:")
	if !ok {
		return "", false
	}
	return strings.TrimSpace(value), true
}

// generationStamp reads the Generated: value out of a comment line, and
// reports whether the line carries one.
func generationStamp(comment string) (string, bool) {
	value, ok := strings.CutPrefix(strings.TrimSpace(strings.TrimPrefix(comment, "#")), "Generated:")
	if !ok {
		return "", false
	}
	return strings.TrimSpace(value), true
}

// parseRIRRange reads one range line: the first AS number, the last AS number
// and the registry token, separated by single spaces. The registry's display
// name and its whois host are constants in this file, so the line carries the
// token alone and every entry shares those five strings.
func parseRIRRange(text string) (RIREntry, error) {
	const wantFields = "want three fields: start, end and registry token"

	start, rest, split := strings.Cut(text, " ")
	if !split {
		return RIREntry{}, errors.New(wantFields)
	}
	end, token, split := strings.Cut(rest, " ")
	if !split {
		return RIREntry{}, errors.New(wantFields)
	}
	if strings.Contains(token, " ") {
		return RIREntry{}, errors.New(wantFields)
	}

	first, err := strconv.ParseUint(start, 10, 32)
	if err != nil {
		return RIREntry{}, fmt.Errorf("range start: %w", err)
	}
	last, err := strconv.ParseUint(end, 10, 32)
	if err != nil {
		return RIREntry{}, fmt.Errorf("range end: %w", err)
	}
	if last < first {
		return RIREntry{}, fmt.Errorf("range ends at AS%d, below its start AS%d", last, first)
	}

	rir, known := rirNames[token]
	if !known {
		return RIREntry{}, fmt.Errorf("unknown registry token %q", token)
	}
	return RIREntry{Start: uint32(first), End: uint32(last), RIR: rir, Whois: rirWhois[token]}, nil
}

// DelegationFetch answers the body of one registry delegation file. It is a
// parameter of FetchDelegationTable so a caller names its own answers, and so
// the one place that speaks HTTPS is named once. The reader is closed by
// whoever asked for it.
type DelegationFetch func(ctx context.Context, delegationURL string) (io.ReadCloser, error)

// maxDelegationSize bounds one delegation file. The largest registry publishes
// about ten megabytes, and the read stops at twice that: a body with no end
// MUST NOT be able to exhaust the process that asked for it.
const maxDelegationSize = 20 << 20

// fetchTimeout bounds one delegation file, the request and the body read
// together. The registries are sometimes slow, so the bound is generous rather
// than tight; what it exists to stop is a refresh that never returns.
const fetchTimeout = 60 * time.Second

// delegationClient is the client httpDelegationFetch reads with. One client
// for the five files shares one connection pool.
var delegationClient = &http.Client{Timeout: fetchTimeout}

// FetchDelegationTable reads the five registry delegation files and builds the
// table every writer of this format publishes. `update resolve rir` stores
// what it answers, and `./le iana-asn write` renders the shipped seed from it.
// One recipe is what keeps a guard off one path and on the other: the parse,
// the collapse and the render are this package's, and this is their one
// caller.
//
// It answers TWO facts about one run, and they are not the same fact. The
// table carries the collapsed ranges a lookup searches. The record count is
// how many delegation records the five files yielded BEFORE the collapse, so a
// reader can tell a run that read the whole world from one that read a
// fraction of it. The count is not a field of DelegationTable, because a table
// read back from a file has no such count, and a zero there would read as "no
// records" rather than "never fetched" (ai/rules/principles.md).
//
// It is ALL OR NOTHING, and each half of that is a fail-closed guard. A
// registry that does not answer, or answers something the parser refuses,
// stops the run and names its URL, so the previous table stays whole (AC-5). A
// run that reached all five and took no ASN record from them stops as well: an
// empty table is not a smaller answer, it is every AS number becoming
// unanswerable (AC-6).
//
// The sources map names, per registry token, a URL to read instead of the file
// that registry publishes. A registry with no entry is read from its published
// file, so a nil map is the daemon nobody configured, and a token no registry
// spells is an error rather than a source nobody reads.
//
// A nil fetch reads the files over HTTPS.
func FetchDelegationTable(ctx context.Context, sources map[string]string, fetch DelegationFetch) (DelegationTable, int, error) {
	if fetch == nil {
		fetch = httpDelegationFetch
	}

	urls, err := delegationSourceURLs(sources)
	if err != nil {
		return DelegationTable{}, 0, err
	}

	var records []RIREntry
	for _, delegationURL := range urls {
		entries, err := fetchDelegationRecords(ctx, fetch, delegationURL)
		if err != nil {
			return DelegationTable{}, 0, err
		}
		records = append(records, entries...)
	}

	if len(records) == 0 {
		return DelegationTable{}, 0, errors.New("the five delegation files hold no ASN record, and this table is what every ASN-to-registry lookup reads")
	}

	slices.SortFunc(records, func(a, b RIREntry) int {
		return cmp.Compare(a.Start, b.Start)
	})

	ranges, err := collapseRanges(records)
	if err != nil {
		return DelegationTable{}, 0, err
	}

	// The URLs this run read travel with the table, so the file it becomes
	// says where its ranges came from rather than where they usually come from.
	// The registries answered now, so now is the date this data was collected.
	return DelegationTable{Generated: time.Now().UTC(), Ranges: ranges, Sources: urls}, len(records), nil
}

// fetchDelegationRecords reads the ASN records out of one registry's file.
//
// It names that file in whatever goes wrong, because an operator told a
// refresh failed can act only if they know which registry failed it.
func fetchDelegationRecords(ctx context.Context, fetch DelegationFetch, delegationURL string) ([]RIREntry, error) {
	body, err := fetch(ctx, delegationURL)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", delegationURL, err)
	}
	defer func() { _ = body.Close() }()

	entries, err := parseRegistryDelegation(io.LimitReader(body, maxDelegationSize))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", delegationURL, err)
	}
	return entries, nil
}

// httpDelegationFetch is the fetch a run uses when nobody names another: one
// GET for each file, bounded by fetchTimeout, refusing anything but 200. The
// caller names the URL in the error, so this one does not repeat it.
func httpDelegationFetch(ctx context.Context, delegationURL string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, delegationURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := delegationClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return resp.Body, nil
}

// rirForASN answers the entry whose range holds asn, or nil when no range
// does: reserved, unallocated, or a documentation block.
//
// The ranges are sorted by Start and never overlap, which parseDelegationTable
// and collapseRanges each guarantee, so one binary search decides it. The
// returned pointer is into the table's own slice and the table is immutable,
// so it stays true for as long as the caller holds it.
func (t *rirTable) rirForASN(asn uint32) *RIREntry {
	lo, hi := 0, len(t.entries)-1
	for lo <= hi {
		mid := lo + (hi-lo)/2
		e := &t.entries[mid]
		switch {
		case asn < e.Start:
			hi = mid - 1
		case asn > e.End:
			lo = mid + 1
		default:
			// Neither below the start nor above the end, so the range holds it.
			return e
		}
	}
	return nil
}

// whoisForASN returns the whois server for the given ASN's RIR.
// Returns empty string if the ASN is not allocated.
func (t *rirTable) whoisForASN(asn uint32) string {
	if e := t.rirForASN(asn); e != nil {
		return e.Whois
	}
	return ""
}

// Len answers how many collapsed ranges the table holds.
func (t *rirTable) Len() int {
	return len(t.entries)
}

// parseRegistryDelegation reads the ASN records out of one delegation file, in
// the format the five registries publish:
// registry|cc|type|start|value|date|status, with an optional opaque id.
//
// A record it cannot use is passed over: another resource type, a reserved
// block, a summary line, an unknown registry, a number it cannot read, and a
// range whose last AS number would not fit in a uint32. Passing over is this
// format's own semantics, and it is the opposite of parseDelegationTable,
// which refuses a line it cannot read. Ze owns its own table and answers for
// every line of it, while a registry publishes rows Ze has no use for.
//
// FetchDelegationTable is its only caller, so both writers of the Ze table
// reach it: `update resolve rir` and `./le iana-asn write`.
func parseRegistryDelegation(r io.Reader) ([]RIREntry, error) {
	var entries []RIREntry
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || line[0] == '#' {
			continue
		}

		fields, ok := parseDelegationFields(line)
		if !ok {
			continue
		}

		if fields.recType != "asn" {
			continue
		}
		if fields.status != "allocated" && fields.status != "assigned" {
			continue
		}

		rir, knownRIR := rirNames[fields.registry]
		if !knownRIR {
			continue
		}
		whois := rirWhois[fields.registry]

		start, err := strconv.ParseUint(fields.start, 10, 32)
		if err != nil {
			continue
		}
		count, err := strconv.ParseUint(fields.count, 10, 32)
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

type delegationFields struct {
	registry string
	recType  string
	start    string
	count    string
	status   string
}

func parseDelegationFields(line string) (delegationFields, bool) {
	var fields delegationFields
	field := 0
	for value := range strings.SplitSeq(line, "|") {
		switch field {
		case 0:
			fields.registry = value
		case 2:
			fields.recType = value
		case 3:
			fields.start = value
		case 4:
			fields.count = value
		case 6:
			fields.status = value
		}
		field++
		if field >= 7 {
			return fields, true
		}
	}
	return delegationFields{}, false
}

// collapseRanges merges the adjacent and overlapping ranges of one registry,
// and never merges two.
//
// The caller MUST sort by Start first, and unsorted input is an error rather
// than a table that quietly loses ranges. The result is what the lookup's
// binary search and RenderDelegationTable both need: sorted and disjoint.
//
// FetchDelegationTable is its only caller, so both writers of the Ze table
// collapse the same way: `update resolve rir` and `./le iana-asn write`.
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
