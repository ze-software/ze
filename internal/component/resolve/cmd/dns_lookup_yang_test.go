// Design: show_dns.go -- dnsLookups is the one Go declaration of the record types
//
// Goal: prove the record types `show dns lookup` and `clear dns cache record`
// offer an operator are the types the handler can answer, so a word cannot
// exist on one side alone. Method: read each enumeration out of the loaded
// model with configyang.EnumValues, which fails on a leaf that declares no
// enumeration rather than answering an empty set, and compare it with the
// table's keys.

package cmd

import (
	"slices"
	"strings"
	"testing"

	mdns "github.com/miekg/dns"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"

	// The blank import registers ze-resolve-cmd with the loader, which declares
	// the two leaves read below.
	_ "github.com/ze-software/ze/internal/plugins/resolve-cmd/yang"
)

// TestDNSLookupTypesMatchTheModel holds dnsLookups to the model at both leaves
// that spell the set, and to the DNS registry that numbers each type.
func TestDNSLookupTypesMatchTheModel(t *testing.T) {
	for _, leaf := range []string{"show/dns/lookup/type", "clear/dns/cache/record/type"} {
		model, err := configyang.EnumValues(leaf)
		if err != nil {
			t.Fatalf("read the enumeration at %s: %v", leaf, err)
		}
		if !slices.Equal(model, dnsLookupTypes()) {
			t.Errorf("the record types disagree: the model at %s holds %v and dnsLookups holds %v. "+
				"A word only the model carries is refused by the handler, and a word only Go carries is one no operator can ask for",
				leaf, model, dnsLookupTypes())
		}
	}
	for _, qtype := range dnsLookupTypes() {
		if _, numbered := mdns.StringToType[qtype]; !numbered {
			t.Errorf("%q has no RR type number in the DNS registry, so the Ze resolver cannot query it", qtype)
		}
	}
}

// TestDNSLookupRefusesATypeNoLeafHolds proves the refusal is closed on both
// routes: the handler names the accepted words, and the standard-library path
// answers an error rather than an empty record list for a type it never
// looked up.
func TestDNSLookupRefusesATypeNoLeafHolds(t *testing.T) {
	resp, err := handleDNSLookup(nil, []string{"localhost", argType, "SRV"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != plugin.StatusError {
		t.Fatalf("SRV is offered by no leaf, so the handler must refuse it: got status %v", resp.Status)
	}
	for _, qtype := range dnsLookupTypes() {
		if !strings.Contains(resp.Error, qtype) {
			t.Errorf("the refusal %q does not name %s, which an operator may write", resp.Error, qtype)
		}
	}

	records, err := dnsLookupStdlib("localhost", "SRV")
	if err == nil {
		t.Fatalf("dnsLookupStdlib answered %v for SRV with no error, which reads as a name with no records", records)
	}
}
