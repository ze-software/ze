package filter_family

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestFamilyMixedSections checks the configured family in every reachability
// section, with expected verdicts on the typed slot and forked batch evaluator.
func TestFamilyMixedSections(t *testing.T) {
	setInstances(t, map[string]*familyFilter{
		"v4":    {name: "v4", family: family.IPv4Unicast, action: actionRemove},
		"v6":    {name: "v6", family: family.IPv6Unicast, action: actionRemove},
		"kill4": {name: "kill4", family: family.IPv4Unicast, action: actionTearDown},
		"kill6": {name: "kill6", family: family.IPv6Unicast, action: actionTearDown},
	})
	const base = "400101004002040201fde940030401010101"
	const reach6 = "800e1c0002011020010db8000000000000000000000001003020010db80001"
	const reach4 = "800e0d000101040101010100180a0001"
	const unreach6 = "800f0a0002013020010db80002"
	const unreach4 = "800f07000101180a0002"
	cases := []struct {
		name, filter, direction, attrs, announce, withdrawn string
		action                                              sdk.FilterAction
		strip                                               string
		teardown                                            bool
	}{
		{"legacy announce beside MP", "v4", "import", base + reach6, "180a0001", "", sdk.FilterModify, "ipv4/unicast", false},
		{"legacy withdraw beside MP", "v4", "export", base + reach6, "", "180a0002", sdk.FilterModify, "ipv4/unicast", false},
		{"both legacy sections beside MP", "v4", "import", base + reach6, "180a0001", "180a0002", sdk.FilterModify, "ipv4/unicast", false},
		{"MP unreach target after other reach", "v6", "import", base + reach4 + unreach6, "", "", sdk.FilterModify, "ipv6/unicast", false},
		{"MP reach target before other unreach", "v6", "export", base + reach6 + unreach4, "", "", sdk.FilterModify, "ipv6/unicast", false},
		{"MP unreach v4 target", "v4", "import", base + reach6 + unreach4, "", "", sdk.FilterModify, "ipv4/unicast", false},
		{"pure legacy", "v4", "import", base, "180a0001", "180a0002", sdk.FilterReject, "", false},
		{"pure MP", "v6", "export", base + reach6 + unreach6, "", "", sdk.FilterReject, "", false},
		{"no target", "v4", "import", base + reach6, "", "", sdk.FilterAccept, "", false},
		{"tear down legacy mixed", "kill4", "import", base + reach6, "180a0001", "", sdk.FilterReject, "", true},
		{"tear down second MP", "kill6", "import", base + reach4 + unreach6, "", "", sdk.FilterReject, "", true},
		{"no export teardown", "kill6", "export", base + reach4 + unreach6, "", "", sdk.FilterAccept, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decode := func(s string) []byte {
				b, err := hex.DecodeString(s)
				if err != nil {
					t.Fatal(err)
				}
				return b
			}
			body := buildUpdate(decode(tc.attrs), decode(tc.announce))
			withdrawn := decode(tc.withdrawn)
			body = append(append([]byte{byte(len(withdrawn) >> 8), byte(len(withdrawn))}, withdrawn...), body[2:]...)
			eval := func(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
				out, err := evaluateFamilyFilter(route)
				if err != nil {
					return out, err
				}
				if out.Action != tc.action || out.Teardown != tc.teardown {
					t.Errorf("verdict = %+v, want action %s teardown %v", out, tc.action, tc.teardown)
				}
				if len(out.Raw) != 0 {
					t.Error("family removal must be an intent, not a raw rewrite")
				}
				if tc.strip != "" {
					want, err := json.Marshal(tc.strip)
					if err != nil {
						t.Fatal(err)
					}
					if len(out.Sets) != 1 || out.Sets[0].Field != fieldFamily || out.Sets[0].Action != rpc.FilterSetActionRemove || string(out.Sets[0].Value) != string(want) {
						t.Errorf("sets = %+v, want family remove %s", out.Sets, want)
					}
				} else if len(out.Sets) != 0 {
					t.Errorf("unexpected sets: %+v", out.Sets)
				}
				if tc.teardown && (out.NotifyCode != 6 || out.NotifySubcode != 5) {
					t.Errorf("notification = %d/%d", out.NotifyCode, out.NotifySubcode)
				}
				return out, nil
			}
			if !sdk.CheckFilterConformance(t, "mixed-family", eval, []sdk.ConformanceRoute{{Input: rpc.FilterUpdateInput{Filter: tc.filter, Direction: tc.direction, Raw: body}}}) {
				t.Fatal("mixed family verdict differs across transports")
			}
		})
	}
}
