// Design: docs/architecture/testing/interop.md -- bounded first-epoch attribution.
package bgp

import (
	"context"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestNSSARejectsFutureCaptureCompletion uses synthetic timestamp inputs, not
// native execution evidence. All non-clock inputs use the clean wire fixture.
// MUTATION: remove finished.After(time.Now()) from nssaFirstEpochProof.finish;
// the future subtest must then fail by false acceptance. Main owns that cut/run.
func TestNSSARejectsFutureCaptureCompletion(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "nssa-epoch-regression")
	for _, test := range []struct {
		name   string
		offset time.Duration
		refuse bool
	}{
		{name: "current"},
		{name: "future", offset: 3 * time.Minute, refuse: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			lab := &nssaWireRegressionLab{mode: "clean", at: time.Now().Add(test.offset)}
			ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
			defer cancel()
			err := checkNSSADefault(ctx, &interoplab.CheckContext{
				Source:  interoplab.ScenarioSource{Name: "ospf-stub-nssa-frr"},
				Network: interoplab.Network{IPv4: netip.MustParsePrefix("172.30.0.0/24")},
				Lab:     lab, Checkout: t.TempDir(),
			})
			if !test.refuse {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil {
				t.Fatal("checker accepted a completed capture from a future epoch")
			}
			if !strings.Contains(err.Error(), "invalid bounded epoch") {
				t.Fatalf("future epoch refused for an unrelated prerequisite: %v", err)
			}
		})
	}
}
