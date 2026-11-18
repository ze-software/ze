// Design: docs/architecture/l2tp/subscriber-session-model.md -- observing current pool ownership

package l2tppool

import (
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
)

type poolShowSnapshot struct {
	Status         string                            `json:"status"`
	Gateway        string                            `json:"gateway"`
	RangeStart     string                            `json:"range-start"`
	RangeEnd       string                            `json:"range-end"`
	DNSPrimary     string                            `json:"dns-primary"`
	DNSSecondary   string                            `json:"dns-secondary"`
	Total          uint32                            `json:"total"`
	Allocated      uint32                            `json:"allocated"`
	Available      uint32                            `json:"available"`
	Sessions       []sessionAlloc                    `json:"sessions"`
	IPv6PD         *poolShowPrefixSnapshot           `json:"ipv6-pd"`
	NamedPools     map[string]poolShowSnapshot       `json:"named-pools"`
	NamedIPv6Pools map[string]poolShowPrefixSnapshot `json:"named-ipv6-pools"`
}

type poolShowPrefixSnapshot struct {
	Block            string `json:"block"`
	DelegationLength int    `json:"delegation-length"`
	Total            uint32 `json:"total"`
	Allocated        uint32 `json:"allocated"`
	Available        uint32 `json:"available"`
}

func poolShowRead(t *testing.T, pool *poolPlugin) poolShowSnapshot {
	t.Helper()
	encoded, err := json.Marshal(pool.showPool())
	if err != nil {
		t.Fatal(err)
	}
	var result poolShowSnapshot
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

// TestPoolShowIncludesNamedReservationsAfterReload observes both named pool
// families through the command's JSON answer after carry changes their names.
func TestPoolShowIncludesNamedReservationsAfterReload(t *testing.T) {
	pool := &poolPlugin{}
	transaction := pool.configTransaction()
	if err := transaction.Configure(reloadPoolSection(t, map[string]any{
		"ipv4":            reloadIPv4("10.0.0.10", "10.0.0.11"),
		"named-pool":      map[string]any{"gold": reloadIPv4("10.0.1.10", "10.0.1.11")},
		"named-ipv6-pool": map[string]any{"silver": reloadIPv6("2001:db8:100::/60", 64)},
	})); err != nil {
		t.Fatal(err)
	}
	l2tp.StoreSessionMetadata(subscriber.SessionID(subscriber.AccessL2TP, uint64(65001), 1), &l2tp.AuthMetadata{FramedPool: "gold"})
	t.Cleanup(func() { l2tp.ClearSessionMetadata(subscriber.SessionID(subscriber.AccessL2TP, uint64(65001), 1)) })
	if answer := pool.handle(ppp.EventIPRequest{SubscriberID: subscriber.SessionID(subscriber.AccessL2TP, uint64(65001), 1), TunnelID: 65001, SessionID: 1, Family: ppp.AddressFamilyIPv4}); !answer.Accept {
		t.Fatal(answer.Reason)
	}
	if answer := pool.handlePrefix(l2tp.PrefixRequest{SubscriberID: subscriber.SessionID(subscriber.AccessL2TP, 65001, 2), TunnelID: 65001, SessionID: 2, PoolName: "silver"}); !answer.OK {
		t.Fatal(answer.Reason)
	}
	if err := transaction.Verify(reloadPoolSection(t, map[string]any{
		"ipv4":            reloadIPv4("10.0.0.10", "10.0.0.11"),
		"named-pool":      map[string]any{"bronze": reloadIPv4("10.0.1.10", "10.0.1.12")},
		"named-ipv6-pool": map[string]any{"indigo": reloadIPv6("2001:db8:100::/59", 64)},
	})); err != nil {
		t.Fatal(err)
	}
	if err := transaction.Apply(); err != nil {
		t.Fatal(err)
	}
	result := poolShowRead(t, pool)
	if result.Total != 2 || result.Allocated != 0 {
		t.Fatalf("default totals = %d/%d, want 2/0", result.Total, result.Allocated)
	}
	address, present := result.NamedPools["bronze"]
	if !present {
		t.Fatal("named IPv4 pool bronze is absent from the command answer")
	}
	if address.Total != 3 || address.Allocated != 1 || address.Available != 2 {
		t.Fatalf("carried named IPv4 totals = %+v, want 3/1/2", address)
	}
	if address.RangeStart != "10.0.1.10" || address.RangeEnd != "10.0.1.12" {
		t.Fatalf("named IPv4 range = %s-%s", address.RangeStart, address.RangeEnd)
	}
	prefix, present := result.NamedIPv6Pools["indigo"]
	if !present {
		t.Fatal("named IPv6 pool indigo is absent from the command answer")
	}
	if prefix.Block != "2001:db8:100::/59" || prefix.DelegationLength != 64 {
		t.Fatalf("named IPv6 block = %+v", prefix)
	}
	if prefix.Total != 32 || prefix.Allocated != 1 || prefix.Available != 31 {
		t.Fatalf("carried named IPv6 totals = %+v, want 32/1/31", prefix)
	}
}

// TestPoolShowNamedOnlyPools requires a useful answer when named pools exist
// without either default, instead of the old false no-pool status.
func TestPoolShowNamedOnlyPools(t *testing.T) {
	pool := &poolPlugin{}
	if err := pool.configTransaction().Configure(reloadPoolSection(t, map[string]any{
		"named-pool":      map[string]any{"gold": reloadIPv4("10.0.1.10", "10.0.1.11")},
		"named-ipv6-pool": map[string]any{"silver": reloadIPv6("2001:db8:100::/60", 64)},
	})); err != nil {
		t.Fatal(err)
	}
	result := poolShowRead(t, pool)
	if result.Status != "" {
		t.Fatalf("named-only pools reported %q", result.Status)
	}
	if named, present := result.NamedPools["gold"]; !present || named.Total != 2 {
		t.Fatalf("named-only IPv4 answer = %+v", result.NamedPools)
	}
	if named, present := result.NamedIPv6Pools["silver"]; !present || named.Total != 16 {
		t.Fatalf("named-only IPv6 answer = %+v", result.NamedIPv6Pools)
	}
}

// TestPoolShowPreservesDefaultFields keeps the existing scalar fields and
// default IPv6 summary while absent named maps remain omitted on the wire.
func TestPoolShowPreservesDefaultFields(t *testing.T) {
	pool := &poolPlugin{}
	ipv4 := reloadIPv4("10.0.0.10", "10.0.0.11")
	ipv4["dns-primary"] = "192.0.2.53"
	ipv4["dns-secondary"] = "198.51.100.53"
	if err := pool.configTransaction().Configure(reloadPoolSection(t, map[string]any{
		"ipv4":    ipv4,
		"ipv6-pd": reloadIPv6("2001:db8:100::/60", 64),
	})); err != nil {
		t.Fatal(err)
	}
	allocate(t, pool, 65002, 1, subscriber.AccessL2TP)
	if answer := pool.handlePrefix(l2tp.PrefixRequest{SubscriberID: subscriber.SessionID(subscriber.AccessL2TP, 65002, 2), TunnelID: 65002, SessionID: 2}); !answer.OK {
		t.Fatal(answer.Reason)
	}
	result := poolShowRead(t, pool)
	if result.Gateway != "10.0.0.1" || result.RangeStart != "10.0.0.10" || result.RangeEnd != "10.0.0.11" {
		t.Fatalf("default address fields = %+v", result)
	}
	if result.DNSPrimary != "192.0.2.53" || result.DNSSecondary != "198.51.100.53" {
		t.Fatalf("default DNS fields = %+v", result)
	}
	if result.Total != 2 || result.Allocated != 1 || result.Available != 1 || len(result.Sessions) != 1 {
		t.Fatalf("default allocation fields = %+v", result)
	}
	if result.IPv6PD == nil || result.IPv6PD.Total != 16 || result.IPv6PD.Allocated != 1 || result.IPv6PD.Available != 15 {
		t.Fatalf("default IPv6 fields = %+v", result.IPv6PD)
	}
	encoded, err := json.Marshal(pool.showPool())
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"named-pools", "named-ipv6-pools"} {
		if _, present := fields[field]; present {
			t.Fatalf("empty %s is present in the command answer", field)
		}
	}
}
