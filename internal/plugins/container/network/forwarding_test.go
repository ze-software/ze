package network

import (
	"context"
	"slices"
	"testing"
)

// The events the sysctl plugin reads, spelled out so the tests check the
// contract rather than the package's own constants: a default of 1 for each
// family's forwarding key under a source of the family's own, and the clear
// of that source.
const (
	claimIPv4    = `default {"key":"net.ipv4.conf.all.forwarding","value":"1","source":"container-network-ipv4"}`
	claimIPv6    = `default {"key":"net.ipv6.conf.all.forwarding","value":"1","source":"container-network-ipv6"}`
	withdrawIPv4 = `clear-source-defaults {"source":"container-network-ipv4"}`
	withdrawIPv6 = `clear-source-defaults {"source":"container-network-ipv6"}`
)

// The network bodies the forwarding tests apply.
const (
	ipv4Network = `{"network":{"lan":{"prefix":["10.88.0.0/24"]}}}`
	dualNetwork = `{"network":{"lan":{"prefix":["10.88.0.0/24","fd00:88::/64"]}}}`
)

// applyExpecting applies cfg through m and fails the test unless the events
// emitted so far are want.
func applyExpecting(t *testing.T, m *manager, engine *fakeEngine, cfg Config, want []string) {
	t.Helper()
	if err := m.apply(context.Background(), cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := engine.emitted(); !slices.Equal(got, want) {
		t.Fatalf("events emitted %q, want %q", got, want)
	}
}

// TestApplyClaimsForwardingOfEachFamily: a network's traffic beyond its
// bridge is forwarded, so the apply claims the forwarding default of each
// family a network has a prefix of, and of no other: an IPv4 network claims
// IPv4 forwarding alone, a dual-stack one claims both.
//
// Method: each config is applied through a fresh manager on the fake engine,
// which records the sysctl events.
func TestApplyClaimsForwardingOfEachFamily(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want []string
	}{
		{"an IPv4 network", ipv4Network, []string{claimIPv4}},
		{"a dual-stack network", dualNetwork, []string{claimIPv4, claimIPv6}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := newFakeEngine()
			m := newManager()
			m.receive(engine)
			applyExpecting(t, m, engine, mustParse(t, tc.body), tc.want)
		})
	}
}

// TestApplyWithdrawsForwardingNoNetworkNeeds: a claim changes only when the
// families the networks use change. A second apply of the same config emits
// nothing, because the sysctl plugin keeps a claim until it is withdrawn and
// warns again of one the operator's config overrides. A family no network
// uses any more is withdrawn alone; removing the last network withdraws the
// rest; a config with no network emits nothing it did not claim before, on a
// fresh manager too.
//
// Method: one manager on the fake engine applies a dual-stack network, the
// same again, an IPv4 network, no network twice; a fresh manager applies no
// network.
func TestApplyWithdrawsForwardingNoNetworkNeeds(t *testing.T) {
	engine := newFakeEngine()
	m := newManager()
	m.receive(engine)
	applyExpecting(t, m, engine, mustParse(t, dualNetwork), []string{claimIPv4, claimIPv6})
	applyExpecting(t, m, engine, mustParse(t, dualNetwork), []string{claimIPv4, claimIPv6})
	applyExpecting(t, m, engine, mustParse(t, ipv4Network), []string{claimIPv4, claimIPv6, withdrawIPv6})
	applyExpecting(t, m, engine, Config{}, []string{claimIPv4, claimIPv6, withdrawIPv6, withdrawIPv4})
	applyExpecting(t, m, engine, Config{}, []string{claimIPv4, claimIPv6, withdrawIPv6, withdrawIPv4})

	fresh := newFakeEngine()
	idle := newManager()
	idle.receive(fresh)
	applyExpecting(t, idle, fresh, Config{}, nil)
}
