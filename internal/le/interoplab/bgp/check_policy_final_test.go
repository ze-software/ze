package bgp

import (
	"strings"
	"testing"
)

func TestPolicyFinalFRRRouteAttributes(t *testing.T) {
	const prefix = "10.73.1.0/24"
	const valid = `{"prefix":"10.73.1.0/24","paths":[{"metric":77,"valid":true,"aspath":{"string":"65000 65004"},"community":{"list":["65004:99","65000:73"]},"nexthops":[{"metric":0}]}]}`
	if err := requirePolicyFinalFRRRoute(valid, prefix); err != nil {
		t.Fatalf("route MED must not be confused with nexthop metric: %v", err)
	}
	for _, tc := range []struct {
		name   string
		output string
	}{
		{"nexthop metric cannot substitute for MED", strings.ReplaceAll(strings.Replace(valid, `"metric":77`, `"metric":0`, 1), `"metric":0}]`, `"metric":77}]`)},
		{"missing MED", strings.Replace(valid, `"metric":77,`, "", 1)},
		{"wrong prefix", strings.Replace(valid, prefix, "10.73.2.0/24", 1)},
		{"invalid path", strings.Replace(valid, `"valid":true`, `"valid":false`, 1)},
		{"lost source AS", strings.Replace(valid, "65000 65004", "65000", 1)},
		{"lost original community", strings.Replace(valid, `"65004:99",`, "", 1)},
		{"missing modification", strings.Replace(valid, `,"65000:73"`, "", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := requirePolicyFinalFRRRoute(tc.output, prefix); err == nil {
				t.Fatal("incorrect received route satisfied final-policy attribute proof")
			}
		})
	}
}
