// Design: docs/architecture/zefs-format.md -- key templates and declared value bounds
package zefs

import "testing"

// TestKeyEntryMatches proves the one template matcher admits exactly the keys
// a placeholder-per-segment reading admits.
//
// Method: a table of patterns and keys, covering a fixed pattern, one and two
// placeholders, a placeholder that would span two segments, an empty
// placeholder, a placeholder inside a segment (gr-fact-{instance}), and a
// literal suffix after the placeholders.
func TestKeyEntryMatches(t *testing.T) {
	cases := []struct {
		pattern string
		key     string
		want    bool
	}{
		{"meta/irr-generation", "meta/irr-generation", true},
		{"meta/irr-generation", "meta/irr-generation/x", false},
		{"meta/irr-set/{as-set}", "meta/irr-set/AS-TEST", true},
		{"meta/irr-set/{as-set}", "meta/irr-set/", false},
		{"meta/irr-set/{as-set}", "meta/irr-set/AS-TEST/extra", false},
		{"meta/irr-data/{server}/{as-set}/{digest}", "meta/irr-data/127.0.0.1:4343/RIPE::AS-TEST/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", true},
		{"meta/irr-data/{server}/{as-set}/{digest}", "meta/irr-data/127.0.0.1:4343/RIPE::AS-TEST", false},
		{"meta/irr-data/{server}/{as-set}/{digest}", "meta/irr-data/whois.radb.net", false},
		{"meta/irr-data/{server}/{as-set}/{digest}", "meta/irr-data//AS-TEST/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", false},
		{"meta/ospf/gr-fact-{instance}", "meta/ospf/gr-fact-default", true},
		{"meta/ospf/gr-fact-{instance}", "meta/ospf/gr-fact-", false},
		{"meta/ssh/{host}/{port}/username", "meta/ssh/router/22/username", true},
		{"meta/ssh/{host}/{port}/username", "meta/ssh/router/22/password", false},
		{"meta/ssh/{host}/{port}/username", "meta/ssh/router/username", false},
	}
	for _, tc := range cases {
		got := KeyEntry{Pattern: tc.pattern}.Matches(tc.key)
		if got != tc.want {
			t.Errorf("KeyEntry{%q}.Matches(%q) = %v, want %v", tc.pattern, tc.key, got, tc.want)
		}
	}
}

// TestLookupFindsTheDeclaredBound proves a concrete key resolves to the entry
// it instantiates, so the bound a registration declares reaches the key.
//
// Method: look up a dataset key, which declares IRRDatasetValueMax, a pointer
// key, which declares none, and a key nothing registers. The largest declared
// bound is the dataset's.
func TestLookupFindsTheDeclaredBound(t *testing.T) {
	entry, found := Lookup("meta/irr-data/whois.radb.net/AS-TEST/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if !found || entry.Pattern != KeyIRRDataset.Pattern || entry.ValueMax != IRRDatasetValueMax {
		t.Fatalf("dataset key: entry=%+v found=%v, want %s with ValueMax %d", entry, found, KeyIRRDataset.Pattern, IRRDatasetValueMax)
	}
	entry, found = Lookup("meta/irr-set/AS-TEST")
	if !found || entry.Pattern != KeyIRRSet.Pattern || entry.ValueMax != 0 {
		t.Fatalf("pointer key: entry=%+v found=%v, want %s with no declared bound", entry, found, KeyIRRSet.Pattern)
	}
	if entry, found := Lookup("meta/unregistered/key"); found {
		t.Fatalf("unregistered key resolved to %+v", entry)
	}
	if got := ValueMaxLargest(); got != IRRDatasetValueMax {
		t.Fatalf("ValueMaxLargest() = %d, want %d", got, IRRDatasetValueMax)
	}
}
