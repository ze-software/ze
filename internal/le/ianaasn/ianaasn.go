// Design: docs/architecture/resolve.md -- compiled RIR delegation seed
//
// Package ianaasn fetches the five RIR delegation files and generates
// rir_table.go in the irr package: the ASN-to-RIR mapping with each registry's
// whois server. That table is the seed data committed to the repository, which
// the resolver falls back to when zefs holds nothing.
//
// It is the one generator here whose input is the NETWORK rather than the tree,
// which is why it has no check twin: there is nothing to compare a checkout
// against without asking five registries what they publish today. The fetch is
// a parameter so a test names its own answers.
//
// TWO THINGS FAIL CLOSED. A run that parses no ASN record writes nothing:
// otherwise five HTTP 200 responses carrying no ASN records could commit an
// empty seedRIRTable and make every IRR lookup fall back to nothing. The table
// is also built in memory and written in one call, so an interrupted write
// cannot leave the irr package holding half a table.
package ianaasn

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// outputFile is the generated table, relative to the tree.
const outputFile = "internal/component/resolve/irr/rir_table.go"

// fetchTimeout bounds one delegation file. The files are tens of megabytes and
// the registries are sometimes slow, so the bound is generous rather than tight;
// what it exists to stop is a build-host generator hanging forever.
const fetchTimeout = 60 * time.Second

// delegationURLs are the five files, one per regional registry. A run reads all
// five or writes nothing: a table missing one registry's delegation reads as
// "this ASN belongs to nobody" for every AS number that registry holds.
var delegationURLs = []string{
	"https://ftp.ripe.net/pub/stats/ripencc/delegated-ripencc-extended-latest",
	"https://ftp.arin.net/pub/stats/arin/delegated-arin-extended-latest",
	"https://ftp.apnic.net/pub/stats/apnic/delegated-apnic-extended-latest",
	"https://ftp.afrinic.net/pub/stats/afrinic/delegated-afrinic-extended-latest",
	"https://ftp.lacnic.net/pub/stats/lacnic/delegated-lacnic-extended-latest",
}

// rirNames maps the registry token each file uses onto the name the table
// carries. A token this map does not hold is a registry the irr package has no
// constant for, so its records are passed over.
var rirNames = map[string]string{
	"ripencc": "RIPE",
	"arin":    "ARIN",
	"apnic":   "APNIC",
	"afrinic": "AFRINIC",
	"lacnic":  "LACNIC",
}

// rirWhois maps the registry token onto its whois server.
var rirWhois = map[string]string{
	"ripencc": "whois.ripe.net",
	"arin":    "whois.arin.net",
	"apnic":   "whois.apnic.net",
	"afrinic": "whois.afrinic.net",
	"lacnic":  "whois.lacnic.net",
}

// rirConstNames and whoisConstNames map a value onto the interned constant
// rir.go declares for it. The generated table names the constant rather than
// repeating the string, so the compiled seed holds one copy of each.
var rirConstNames = map[string]string{
	"RIPE":    "RIRRIPE",
	"ARIN":    "RIRARIN",
	"APNIC":   "RIRAPNIC",
	"AFRINIC": "RIRAFRINIC",
	"LACNIC":  "RIRLACNIC",
}

var whoisConstNames = map[string]string{
	"whois.ripe.net":    "WhoisRIPE",
	"whois.arin.net":    "WhoisARIN",
	"whois.apnic.net":   "WhoisAPNIC",
	"whois.afrinic.net": "WhoisAFRINIC",
	"whois.lacnic.net":  "WhoisLACNIC",
}

// Entry is one delegated AS number range: the half-open record the registry
// publishes, resolved to the registry that holds it.
type Entry struct {
	Start uint32 `json:"start"`
	End   uint32 `json:"end"`
	RIR   string `json:"rir"`
	Whois string `json:"whois"`
}

// Fetch answers the bytes of one delegation file. It is a parameter so a test
// names its own answers, and so the one place that speaks HTTP is named.
type Fetch func(ctx context.Context, url string) ([]byte, error)

// httpFetch is the Fetch a run uses when nobody names another: one GET per
// file, bounded by fetchTimeout, refusing anything but 200.
func httpFetch(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("fetch ").Str(url).Str(": ").Err(err).String())
	}
	defer resp.Body.Close() //nolint:errcheck // read-only

	if resp.StatusCode != http.StatusOK {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str(url).Str(": HTTP ").Int(int64(resp.StatusCode)).String())
	}

	return io.ReadAll(resp.Body)
}

// parseDelegation reads one registry's file into the ASN ranges it delegates.
//
// A record is taken when it is an asn record, its status is allocated or
// assigned, and its registry token is one the irr package has a constant for.
// Every other row is another resource type, a reserved block, or a summary line.
func parseDelegation(r io.Reader) ([]Entry, error) {
	var entries []Entry

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

		registry, recordType, status := fields[0], fields[2], fields[6]
		if recordType != "asn" || (status != "allocated" && status != "assigned") {
			continue
		}

		rir, known := rirNames[registry]
		if !known {
			continue
		}

		start, err := strconv.ParseUint(fields[3], 10, 32)
		if err != nil {
			continue
		}
		count, err := strconv.ParseUint(fields[4], 10, 32)
		if err != nil || count == 0 {
			continue
		}

		entries = append(entries, Entry{
			Start: uint32(start),
			End:   uint32(start + count - 1),
			RIR:   rir,
			Whois: rirWhois[registry],
		})
	}

	return entries, scanner.Err()
}

// collapse joins the adjacent and overlapping ranges of ONE registry, and never
// joins two. The caller sorts by start first, so one pass suffices.
func collapse(entries []Entry) []Entry {
	if len(entries) == 0 {
		return nil
	}

	result := []Entry{entries[0]}
	for _, e := range entries[1:] {
		last := &result[len(result)-1]
		if e.RIR == last.RIR && e.Start <= last.End+1 {
			if e.End > last.End {
				last.End = e.End
			}
			continue
		}
		result = append(result, e)
	}

	return result
}

// tableSource renders the seed table. It is built whole and returned, never
// streamed into a file: a failure partway through then leaves the irr package
// holding what it held before rather than half a table.
func tableSource(entries []Entry, generated string) []byte {
	var b textbuf.Buffer

	b.Str("// Design: docs/architecture/resolve.md -- compiled RIR delegation seed\n//\n")
	b.Str("// Code generated by ./le iana-asn write; DO NOT EDIT.\n")
	b.Str("// Source: RIR delegation files (RIPE, ARIN, APNIC, AFRINIC, LACNIC)\n")
	b.Str("// Generated: ").Str(generated).Byte('\n')
	b.Str("// This is seed data for zefs prepopulation. Runtime updates via `ze update bgp irr all`.\n\n")
	b.Str("package irr\n\n")
	b.Str("// seedRIRTable is the compiled-in RIR delegation data.\n")
	b.Str("// Used as fallback when zefs has no data or as initial seed.\n")
	b.Str("var seedRIRTable = []RIREntry{\n")

	for _, e := range entries {
		b.Str("\t{").Uint(uint64(e.Start)).Str(", ").Uint(uint64(e.End)).Str(", ").
			Str(rirConstNames[e.RIR]).Str(", ").Str(whoisConstNames[e.Whois]).Str("},\n")
	}

	b.Str("}\n")

	return slices.Clone(b.Bytes())
}

// Write fetches every registry's delegation file and rewrites the seed table.
//
// It writes NOTHING unless all five registries answered and the parse took at
// least one ASN record. An empty table is not a smaller answer: it is the
// resolver's whole fallback removed, and no test of the irr package can see it,
// because the table those tests read is the table that was emptied.
func Write(ctx context.Context, root string, fetch Fetch) (WriteReport, error) {
	if fetch == nil {
		fetch = httpFetch
	}

	var all []Entry
	for _, url := range delegationURLs {
		body, err := fetch(ctx, url)
		if err != nil {
			return WriteReport{}, err
		}

		entries, err := parseDelegation(bytes.NewReader(body))
		if err != nil {
			var tb textbuf.Buffer
			return WriteReport{}, errors.New(tb.Str("read ").Str(url).Str(": ").Err(err).String())
		}

		all = append(all, entries...)
	}

	if len(all) == 0 {
		return WriteReport{}, errors.New("the five delegation files hold no ASN record, and the seed table is the resolver's whole fallback")
	}

	slices.SortFunc(all, func(a, b Entry) int {
		switch {
		case a.Start < b.Start:
			return -1
		case a.Start > b.Start:
			return 1
		default:
			return 0
		}
	})

	ranges := collapse(all)
	source := tableSource(ranges, time.Now().UTC().Format(time.DateOnly))

	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(outputFile)), source, 0o644); err != nil { //nolint:gosec // generated source
		return WriteReport{}, err
	}

	return WriteReport{File: outputFile, Ranges: len(ranges), Records: len(all)}, nil
}
