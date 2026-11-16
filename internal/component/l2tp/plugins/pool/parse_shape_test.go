// Design: docs/architecture/l2tp/subscriber-session-model.md -- pool verification preserves valid running assignments

package l2tppool

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestPoolVerifyRefusesMalformedPresentBlocks proves malformed present values
// cannot act like an intentional pool deletion. Both verifier entry points
// refuse them, and the installed allocator retains its live reservation.
func TestPoolVerifyRefusesMalformedPresentBlocks(t *testing.T) {
	for _, tc := range []struct{ name, data, path string }{
		{"null-section", `null`, "l2tp"},
		{"scalar-root", `{"l2tp":"bad"}`, "l2tp"},
		{"null-root", `{"l2tp":null}`, "l2tp"},
		{"array-root", `{"l2tp":[]}`, "l2tp"},
		{"scalar-pool", `{"l2tp":{"pool":"bad"}}`, "l2tp pool"},
		{"null-pool", `{"l2tp":{"pool":null}}`, "l2tp pool"},
		{"array-pool", `{"l2tp":{"pool":[]}}`, "l2tp pool"},
		{"scalar-ipv4", `{"l2tp":{"pool":{"ipv4":"bad"}}}`, "l2tp pool ipv4"},
		{"null-ipv4", `{"l2tp":{"pool":{"ipv4":null}}}`, "l2tp pool ipv4"},
		{"scalar-ipv6", `{"l2tp":{"pool":{"ipv6-pd":"bad"}}}`, "l2tp pool ipv6-pd"},
		{"null-ipv6", `{"l2tp":{"pool":{"ipv6-pd":null}}}`, "l2tp pool ipv6-pd"},
		{"scalar-named-list", `{"l2tp":{"pool":{"named-pool":"bad"}}}`, "l2tp pool named-pool"},
		{"null-named-list", `{"l2tp":{"pool":{"named-pool":null}}}`, "l2tp pool named-pool"},
		{"scalar-named-body", `{"l2tp":{"pool":{"named-pool":{"gold":"bad"}}}}`, "l2tp pool named-pool gold"},
		{"null-named-body", `{"l2tp":{"pool":{"named-pool":{"gold":null}}}}`, "l2tp pool named-pool gold"},
		{"scalar-named-ipv6-list", `{"l2tp":{"pool":{"named-ipv6-pool":"bad"}}}`, "l2tp pool named-ipv6-pool"},
		{"null-named-ipv6-list", `{"l2tp":{"pool":{"named-ipv6-pool":null}}}`, "l2tp pool named-ipv6-pool"},
		{"scalar-named-ipv6-body", `{"l2tp":{"pool":{"named-ipv6-pool":{"gold":"bad"}}}}`, "l2tp pool named-ipv6-pool gold"},
		{"null-named-ipv6-body", `{"l2tp":{"pool":{"named-ipv6-pool":{"gold":null}}}}`, "l2tp pool named-ipv6-pool gold"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &poolPlugin{}
			tx := p.configTransaction()
			if err := tx.Configure(poolValidationSection(t, map[string]any{
				"ipv4": poolValidationIPv4("10.0.0.10", "10.0.0.11"),
			})); err != nil {
				t.Fatal(err)
			}
			allocate(t, p, 1, 1, subscriber.AccessL2TP)
			running := p.pool
			candidate := []sdk.ConfigSection{{Root: configRootL2TP, Data: tc.data}}
			err := verifyPoolConfig(candidate)
			if err == nil {
				t.Fatal("registry verifier accepted malformed present config")
			}
			if !strings.Contains(err.Error(), tc.path) {
				t.Fatalf("refusal %q does not name %s", err, tc.path)
			}
			if err := tx.Verify(candidate); err == nil {
				t.Fatal("transaction verifier accepted malformed present config")
			}
			if p.pool != running {
				t.Fatal("rejected candidate replaced the running pool")
			}
			if _, allocated, _ := p.pool.stats(); allocated != 1 {
				t.Fatalf("live allocations = %d, want 1 after verify refusal", allocated)
			}
			if got := allocate(t, p, 1, 2, subscriber.AccessL2TP); got != netip.MustParseAddr("10.0.0.11") {
				t.Fatalf("running pool returned %s after refusal, want 10.0.0.11", got)
			}
		})
	}
}

// TestPoolVerifyAcceptsExplicitRemoval keeps the real engine deletion shapes
// useful. Applying them clears configured pools while preserving each live
// assignment outside the removed range until its session ends.
func TestPoolVerifyAcceptsExplicitRemoval(t *testing.T) {
	for _, data := range []string{
		`{}`,
		`{"l2tp":{}}`,
		`{"l2tp":{"enabled":"false"}}`,
		`{"l2tp":{"pool":{}}}`,
		`{"l2tp":{"pool":{"named-pool":{},"named-ipv6-pool":{}}}}`,
	} {
		t.Run(data, func(t *testing.T) {
			p := &poolPlugin{}
			tx := p.configTransaction()
			if err := tx.Configure(poolValidationSection(t, map[string]any{
				"ipv4": poolValidationIPv4("10.0.0.10", "10.0.0.11"),
			})); err != nil {
				t.Fatal(err)
			}
			address := allocate(t, p, 1, 1, subscriber.AccessL2TP)
			candidate := []sdk.ConfigSection{{Root: configRootL2TP, Data: data}}
			if err := verifyPoolConfig(candidate); err != nil {
				t.Fatal(err)
			}
			if err := tx.Verify(candidate); err != nil {
				t.Fatal(err)
			}
			if err := tx.Apply(); err != nil {
				t.Fatal(err)
			}
			if p.pool != nil {
				t.Fatal("explicit removal left the default pool installed")
			}
			if len(p.namedPools) != 0 {
				t.Fatal("explicit removal left named IPv4 pools installed")
			}
			if p.v6pool != nil {
				t.Fatal("explicit removal left the default IPv6 pool installed")
			}
			if len(p.v6namedPools) != 0 {
				t.Fatal("explicit removal left named IPv6 pools installed")
			}
			stored, ok := p.sessionAddrs.Load(subscriber.SessionID(subscriber.AccessL2TP, uint64(1), 1))
			if !ok {
				t.Fatal("explicit removal discarded a live assignment")
			}
			allocation, ok := stored.(sessionAddr)
			if !ok {
				t.Fatalf("live assignment has type %T, want sessionAddr", stored)
			}
			if got := allocation.addr; got != address {
				t.Fatalf("live assignment = %s, want %s", got, address)
			}
			answer := p.handle(ppp.EventIPRequest{SubscriberID: subscriber.SessionID(subscriber.AccessL2TP, uint64(1), 2), TunnelID: 1, SessionID: 2, Family: ppp.AddressFamilyIPv4})
			if answer.Accept {
				t.Fatal("removed pool admitted a new allocation")
			}
		})
	}
}
