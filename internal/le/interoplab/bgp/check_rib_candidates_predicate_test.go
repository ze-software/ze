package bgp

import (
	"net/netip"
	"strings"
	"testing"
)

// VALIDATES: ribCandidatesVerdict binds every judged field to one path, and
// demands exactly the wanted paths.
// Method: Ze-shaped JSON answers for the OSPF scenario's tagged prefix, one
// correct and one per way a substring check would have been fooled.
// PREVENTS: the static path's 150 standing for the OSPF path's distance, a
// selected flag read from the wrong path, a missing or extra path, and a
// hidden-by or chain read from another path passing.
func TestRIBCandidatesVerdict(t *testing.T) {
	prefix := netip.MustParsePrefix("10.88.1.0/24")
	wants := []ribCandidateWant{
		{source: sourceOSPF, distanceJudged: true, distance: 200},
		{source: sourceStatic, selected: true, distanceJudged: true, distance: 150},
	}
	const good = `[{"prefix":"10.88.1.0/24","family":"ipv4/unicast","paths":[
		{"source":"static","instance":0,"distance":150,"metric":0,"selected":true,"hidden":false},
		{"source":"ospf","instance":0,"distance":200,"metric":20,"selected":false,"hidden":false}]}]`
	if err := ribCandidatesVerdict(good, prefix, wants); err != nil {
		t.Fatalf("correct answer refused: %v", err)
	}
	for _, tc := range []struct {
		name   string
		output string
		reason string
	}{
		{"distances swapped", `[{"prefix":"10.88.1.0/24","paths":[
			{"source":"static","distance":200,"selected":true},{"source":"ospf","distance":150}]}]`, "no path of"},
		{"selected on the other path", `[{"prefix":"10.88.1.0/24","paths":[
			{"source":"static","distance":150},{"source":"ospf","distance":200,"selected":true}]}]`, "no path of"},
		{"missing path", `[{"prefix":"10.88.1.0/24","paths":[{"source":"ospf","distance":200}]}]`, "holds 1 paths"},
		{"extra path", `[{"prefix":"10.88.1.0/24","paths":[
			{"source":"static","distance":150,"selected":true},{"source":"ospf","distance":200},{"source":"bgp","distance":20}]}]`, "holds 3 paths"},
		{"no entry", `[{"prefix":"10.88.2.0/24","paths":[]}]`, "no entry for"},
		{"entry twice", `[{"prefix":"10.88.1.0/24","paths":[]},{"prefix":"10.88.1.0/24","paths":[]}]`, "twice"},
		{"not json", `show rib candidates: unknown argument`, "decode"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ribCandidatesVerdict(tc.output, prefix, wants)
			if err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.reason)
			}
		})
	}
	hidden := []ribCandidateWant{{source: sourceISIS, hidden: true, hiddenBy: hiddenBySource, chain: isisImportAttachment}}
	isisPrefix := netip.MustParsePrefix("10.77.1.0/24")
	if err := ribCandidatesVerdict(`[{"prefix":"10.77.1.0/24","paths":[
		{"source":"isis","distance":115,"hidden":true,"hidden-by":"source","chain":"isis import"}]}]`, isisPrefix, hidden); err != nil {
		t.Fatalf("hidden IS-IS path refused: %v", err)
	}
	for _, output := range []string{
		`[{"prefix":"10.77.1.0/24","paths":[{"source":"isis","hidden":false}]}]`,
		`[{"prefix":"10.77.1.0/24","paths":[{"source":"isis","hidden":true,"hidden-by":"central","chain":"rib import"}]}]`,
		`[{"prefix":"10.77.1.0/24","paths":[{"source":"isis","hidden":true,"hidden-by":"source"}]}]`,
	} {
		if err := ribCandidatesVerdict(output, isisPrefix, hidden); err == nil {
			t.Fatalf("wrong hidden verdict accepted: %s", output)
		}
	}
}

// VALIDATES: frrISISRouteVerdict accepts only a selected IS-IS route through
// the named next hop.
// Method: FRR-shaped `show ip route <p> json` answers.
// PREVENTS: a route FRR-B learned some other way (another protocol, another
// next hop, not selected) standing as proof that Ze flooded the LSP.
func TestFRRISISRouteVerdict(t *testing.T) {
	prefix := netip.MustParsePrefix("10.77.1.0/24")
	nextHop := netip.MustParseAddr("10.102.0.2")
	good := `{"10.77.1.0/24":[{"prefix":"10.77.1.0/24","protocol":"isis","selected":true,"nexthops":[{"ip":"10.102.0.2","interfaceName":"eth1"}]}]}`
	if err := frrISISRouteVerdict(good, prefix, nextHop); err != nil {
		t.Fatalf("correct answer refused: %v", err)
	}
	for _, output := range []string{
		`{}`,
		`{"10.77.1.0/24":[{"protocol":"connected","selected":true,"nexthops":[{"ip":"10.102.0.2"}]}]}`,
		`{"10.77.1.0/24":[{"protocol":"isis","selected":false,"nexthops":[{"ip":"10.102.0.2"}]}]}`,
		`{"10.77.1.0/24":[{"protocol":"isis","selected":true,"nexthops":[{"ip":"10.101.0.3"}]}]}`,
		`not json`,
	} {
		if err := frrISISRouteVerdict(output, prefix, nextHop); err == nil {
			t.Fatalf("wrong route accepted: %s", output)
		}
	}
}
