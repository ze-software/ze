// Design: docs/architecture/wire/attributes.md -- pinned IANA special-purpose data.
//
// Package ipregistry classifies addresses against the canonical IANA XML shipped
// with Ze. It has no network or configuration lifecycle. Only the deliberate
// le data ip-special-purpose writer refreshes the snapshots.
package ipregistry

import (
	"bytes"
	_ "embed"
	"encoding/xml"
	"fmt"
	"io"
	"net/netip"
	"slices"
	"strings"
	"time"
)

// SizeMax bounds ingestion of either canonical XML document to one MiB.
const SizeMax = 1 << 20

// Value preserves the registry's blank, boolean and not-applicable distinctions.
// ValueUnspecified represents a present but empty XML field, never an explicit
// false. A missing field or an unknown nonempty token is a Parse error.
type Value uint8

const (
	ValueUnspecified Value = iota
	ValueTrue
	ValueFalse
	ValueNotApplicable
)

// Match distinguishes a successful no-match from input or initialization errors.
type Match uint8

const (
	MatchUnspecified Match = iota // The registry has not been successfully parsed.
	MatchInvalid                  // Invalid address, zone, or wrong registry family.
	MatchUnlisted                 // Valid address, no special-purpose record.
	MatchListed                   // Most-specific record selected, including blanks.
)

// Result contains registry fields only when Match is MatchListed. Callers MUST
// branch on Match before interpreting the fields; invalid/uninitialized input
// is not equivalent to a valid unlisted address. A listed record with blank or
// N/A fields is not a reachability certification and is not explicitly false.
type Result struct {
	Match       Match
	Destination Value
	Forwardable Value
}

// Registry is immutable after Parse and safe for concurrent lookup. The zero
// value is uninitialized: Lookup returns MatchUnspecified rather than allowing
// every address. Parse never returns a registry on failure.
type Registry struct {
	bits    int
	updated string
	entries []entry
}

type entry struct {
	prefix netip.Prefix
	result Result
}

//go:embed iana-ipv4-special-registry.xml
var ipv4XML []byte

//go:embed iana-ipv6-special-registry.xml
var ipv6XML []byte

var (
	ipv4Registry = mustParse(ipv4XML, 32)
	ipv6Registry = mustParse(ipv6XML, 128)
)

// Lookup selects the address's family without unmapping IPv4-mapped IPv6.
// Invalid or zoned input returns MatchInvalid. A valid unlisted address returns
// MatchUnlisted, which means only that this registry places no restriction on
// it. It does not imply global reachability or suitability for another policy.
// Lookup is safe for concurrent use and allocates nothing.
func Lookup(address netip.Addr) Result {
	if address.Is4() {
		return ipv4Registry.Lookup(address)
	}
	return ipv6Registry.Lookup(address)
}

// Lookup returns the most-specific record in this registry, retaining blank
// fields rather than inheriting values from less-specific records. The caller
// MUST inspect Result.Match: invalid, zoned or wrong-family addresses are
// MatchInvalid, distinct from MatchUnlisted for a valid nonmatching address.
// An uninitialized receiver returns MatchUnspecified, including a nil receiver.
// The loop is bounded by the parsed snapshot's entry count; no query allocates.
func (r *Registry) Lookup(address netip.Addr) Result {
	if r == nil {
		return Result{Match: MatchUnspecified}
	}
	if len(r.entries) == 0 {
		return Result{Match: MatchUnspecified}
	}
	if !address.IsValid() {
		return Result{Match: MatchInvalid}
	}
	if address.Zone() != "" {
		return Result{Match: MatchInvalid}
	}
	if address.BitLen() != r.bits {
		return Result{Match: MatchInvalid}
	}
	for i := range r.entries {
		if r.entries[i].prefix.Contains(address) {
			return r.entries[i].result
		}
	}
	return Result{Match: MatchUnlisted}
}

// Updated returns the validated IANA document date, not the local fetch date.
func (r *Registry) Updated() string { return r.updated }

// Len returns the number of prefixes after splitting multi-prefix XML records.
func (r *Registry) Len() int { return len(r.entries) }

// Parse validates one canonical IANA special-purpose document for addressBits
// 32 or 128. XML xref annotations are not boolean text. Missing/unknown fields,
// invalid prefixes, duplicate prefixes and empty tables fail before publication.
// All input and loops are bounded by SizeMax; the returned registry is immutable.
// A parse failure returns nil and an error, never an empty permissive registry.
func Parse(data []byte, addressBits int) (*Registry, error) {
	if len(data) > SizeMax {
		return nil, fmt.Errorf("IANA registry exceeds %d octets", SizeMax)
	}
	id := "iana-ipv4-special-registry"
	switch addressBits {
	case 32:
	case 128:
		id = "iana-ipv6-special-registry"
	default:
		return nil, fmt.Errorf("IANA registry address width %d is unsupported", addressBits)
	}
	var document struct {
		XMLName xml.Name `xml:"http://www.iana.org/assignments registry"`
		ID      string   `xml:"id,attr"`
		Updated string   `xml:"updated"`
		Tables  []struct {
			ID      string `xml:"id,attr"`
			Records []struct {
				Address     string  `xml:"address"`
				Destination *string `xml:"destination"`
				Forwardable *string `xml:"forwardable"`
			} `xml:"record"`
		} `xml:"registry"`
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("parse IANA XML: %w", err)
	}
	// Decode consumes one element, not the complete input. Only XML trailing
	// whitespace/comments/processing instructions may follow that document.
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse IANA XML suffix: %w", err)
		}
		switch value := token.(type) {
		case xml.Comment, xml.ProcInst:
			continue
		case xml.CharData:
			if len(bytes.TrimSpace(value)) == 0 {
				continue
			}
		}
		return nil, fmt.Errorf("unexpected content after IANA registry")
	}
	if document.ID != id {
		return nil, fmt.Errorf("IANA registry identity %q, want %q", document.ID, id)
	}
	if _, err := time.Parse(time.DateOnly, document.Updated); err != nil {
		return nil, fmt.Errorf("IANA registry update date: %w", err)
	}
	if len(document.Tables) != 1 {
		return nil, fmt.Errorf("IANA registry has %d tables, want one", len(document.Tables))
	}
	table := &document.Tables[0]
	if table.ID != id+"-1" {
		return nil, fmt.Errorf("unexpected IANA table %q", table.ID)
	}
	prefixes := 0
	for i := range table.Records {
		prefixes += 1 + strings.Count(table.Records[i].Address, ",")
	}
	registry := &Registry{
		bits: addressBits, updated: document.Updated,
		entries: make([]entry, 0, prefixes),
	}
	seen := make(map[netip.Prefix]bool, len(table.Records))
	for i := range table.Records {
		record := &table.Records[i]
		destination, err := parseValue(record.Destination)
		if err != nil {
			return nil, fmt.Errorf("IANA record %q destination: %w", record.Address, err)
		}
		forwardable, err := parseValue(record.Forwardable)
		if err != nil {
			return nil, fmt.Errorf("IANA record %q forwardable: %w", record.Address, err)
		}
		for text := range strings.SplitSeq(record.Address, ",") {
			prefix, err := netip.ParsePrefix(strings.TrimSpace(text))
			if err != nil {
				return nil, fmt.Errorf("IANA address %q: %w", text, err)
			}
			if prefix.Addr().BitLen() != addressBits {
				return nil, fmt.Errorf("IANA prefix %s has wrong family", prefix)
			}
			if prefix != prefix.Masked() {
				return nil, fmt.Errorf("IANA prefix %s has host bits", prefix)
			}
			if seen[prefix] {
				return nil, fmt.Errorf("duplicate IANA prefix %s", prefix)
			}
			seen[prefix] = true
			registry.entries = append(registry.entries, entry{
				prefix: prefix,
				result: Result{Match: MatchListed, Destination: destination, Forwardable: forwardable},
			})
		}
	}
	if len(registry.entries) == 0 {
		return nil, fmt.Errorf("IANA registry contains no prefixes")
	}
	// Most-specific entries come first, so query stops at its first match.
	slices.SortFunc(registry.entries, func(a, b entry) int {
		if a.prefix.Bits() != b.prefix.Bits() {
			return b.prefix.Bits() - a.prefix.Bits()
		}
		return a.prefix.Addr().Compare(b.prefix.Addr())
	})
	return registry, nil
}

func parseValue(text *string) (Value, error) {
	if text == nil {
		return ValueUnspecified, fmt.Errorf("missing field")
	}
	switch strings.TrimSpace(*text) {
	case "":
		return ValueUnspecified, nil
	case "True":
		return ValueTrue, nil
	case "False":
		return ValueFalse, nil
	case "N/A":
		return ValueNotApplicable, nil
	default:
		return ValueUnspecified, fmt.Errorf("unknown field value %q", *text)
	}
}

func mustParse(data []byte, addressBits int) *Registry {
	registry, err := Parse(data, addressBits)
	if err != nil {
		// Only compiled-in snapshots reach this initialization; neither peers
		// nor runtime files supply bytes to this path. The refresh writer uses
		// Parse's error return before changing either shipped source.
		panic("BUG: invalid embedded IANA registry: " + err.Error())
	}
	return registry
}
