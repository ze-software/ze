package policyview

import (
	"errors"
	"strings"
	"testing"
)

// TestFieldWriterNotLinked proves Stage 1's judge refuses a declared field the
// filter-update message cannot carry in this binary, naming the field and the
// plugin that would carry it, rather than sending the field empty.
//
// VALIDATES: AC-29 (the refusal half) and R-10. This package's test binary
// links no BGP plugin, so the community formatters of bgp-filter-community
// and the originator-id formatter of bgp-rr are absent here, as they are in a
// build without those plugins.
// PREVENTS: a community filter reading "the route carries no community" from
// a field Ze could not write.
func TestFieldWriterNotLinked(t *testing.T) {
	cases := []struct {
		name  string
		field string
		wants []string // substrings the refusal names; none for an accepted field
	}{
		{name: "community needs its plugin", field: "community", wants: []string{`"community"`, "bgp-filter-community"}},
		{name: "originator-id needs its plugin", field: "originator-id", wants: []string{`"originator-id"`, "bgp-rr"}},
		{name: "not a BGP field", field: "metric", wants: []string{`"metric"`, "is not a field of a BGP route"}},
		{name: "an engine field", field: "prefix"},
		{name: "an engine attribute", field: "as-path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckFieldWriter(tc.field)
			if len(tc.wants) == 0 {
				if err != nil {
					t.Fatalf("CheckFieldWriter(%q) = %v, want nil", tc.field, err)
				}
				return
			}
			if !errors.Is(err, ErrNoWriter) {
				t.Fatalf("CheckFieldWriter(%q) = %v, want ErrNoWriter", tc.field, err)
			}
			if errors.Is(err, ErrNotBGPField) != (tc.field == "metric") {
				t.Fatalf("CheckFieldWriter(%q) = %v: ErrNotBGPField only for a field no BGP route carries", tc.field, err)
			}
			for _, want := range tc.wants {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("CheckFieldWriter(%q) = %q, want it to name %s", tc.field, err, want)
				}
			}
		})
	}
}

// TestEveryReaderHasAWriter proves every field the view reads can also travel
// in a filter-update, so a field an in-process member reads is never refused
// to a forked one.
//
// VALIDATES: D-1 (one vocabulary on both paths, FR-1).
// PREVENTS: a reader registered with no writer, which Stage 1 would refuse as
// "not a field of a BGP route".
func TestEveryReaderHasAWriter(t *testing.T) {
	for field := range readerTable.slots {
		if _, ok := writerTable[field.String()]; !ok {
			t.Errorf("field %s has a reader and no writer", field)
		}
	}
	if len(writerTable) != len(readerTable.slots) {
		t.Errorf("%d writers for %d readers", len(writerTable), len(readerTable.slots))
	}
}
