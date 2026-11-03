package bgp

import (
	"fmt"
	"strings"
	"testing"
)

// TestASAbsentVerdictFailsClosed proves the AS-absence check reads FRR's
// segments[].list and refuses every answer that does not show the ASNs a path
// carries. Method: one valid route accepted, then the absent ASN as a number
// at the end of the list, as an asdot string, and in a second path, then each
// malformed answer, each of which must be refused.
func TestASAbsentVerdictFailsClosed(t *testing.T) {
	const prefix = "10.0.0.0/24"
	accepted := fmt.Sprintf(frrNoASRoute, prefix, "65000 65010", "65000,\n              65010")
	if err := asAbsentVerdict(accepted, prefix, "64512"); err != nil {
		t.Fatalf("route without AS 64512 refused: %v", err)
	}
	secondPath := `{"prefix":"10.0.0.0/24","paths":[` +
		`{"aspath":{"segments":[{"type":"as-sequence","list":[65000]}]}},` +
		`{"aspath":{"segments":[{"type":"as-set","list":[65001]},{"type":"as-sequence","list":[65000,4200000000]}]}}]}`
	for name, case_ := range map[string]struct {
		output string
		asn    string
		reason string
	}{
		"last list number":   {fmt.Sprintf(frrNoASRoute, prefix, "65000 64512", "65000,\n              64512"), "64512", "contains AS 64512"},
		"asdot list string":  {fmt.Sprintf(frrNoASRoute, prefix, "65000 64086.59904", `65000, "64086.59904"`), "4200000000", "contains AS 4200000000"},
		"second path":        {secondPath, "4200000000", "contains AS 4200000000"},
		"not json":           {prefix + " 65000", "64512", "not FRR route JSON"},
		"no paths":           {`{"prefix":"10.0.0.0/24","paths":[]}`, "64512", "no paths"},
		"no aspath":          {`{"prefix":"10.0.0.0/24","paths":[{}]}`, "64512", "no aspath"},
		"no segments":        {`{"prefix":"10.0.0.0/24","paths":[{"aspath":{"string":"","segments":[]}}]}`, "64512", "no segments"},
		"empty lists":        {`{"prefix":"10.0.0.0/24","paths":[{"aspath":{"segments":[{"list":[]}]}}]}`, "64512", "list no ASN"},
		"unreadable element": {`{"prefix":"10.0.0.0/24","paths":[{"aspath":{"segments":[{"list":[true]}]}}]}`, "64512", "not an ASN"},
		"unreadable asn":     {accepted, "AS64512", "not an ASN"},
	} {
		t.Run(name, func(t *testing.T) {
			err := asAbsentVerdict(case_.output, prefix, case_.asn)
			if err == nil {
				t.Fatalf("answer accepted:\n%s", case_.output)
			}
			if !strings.Contains(err.Error(), case_.reason) {
				t.Fatalf("refusal %q does not say %q", err, case_.reason)
			}
		})
	}
}
