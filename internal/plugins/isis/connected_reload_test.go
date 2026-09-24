// Design: docs/architecture/isis/isis-11-redistribution.md -- connected-prefix origination.

package isis

import (
	"fmt"
	"maps"
	"net/netip"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
)

// Only the OS address source is substituted. Configuration, logical-name
// resolution, connected-prefix selection, origination and wire decoding are real.
type connectedPrefixBackend struct{ iface.Backend }

func (*connectedPrefixBackend) GetInterface(name string) (*iface.InterfaceInfo, error) {
	var addresses []iface.AddrInfo
	switch name {
	case "isis-conn-a":
		addresses = []iface.AddrInfo{
			{Address: "192.0.2.9", PrefixLength: 24, Family: "ipv4"},
			{Address: "2001:db8:1::9", PrefixLength: 64, Family: "ipv6"},
			{Address: "fe80::9", PrefixLength: 64, Family: "ipv6"},
		}
	case "isis-conn-b":
		addresses = []iface.AddrInfo{
			{Address: "198.51.100.9", PrefixLength: 24, Family: "ipv4"},
			{Address: "2001:db8:2::9", PrefixLength: 64, Family: "ipv6"},
		}
	default:
		return nil, fmt.Errorf("unknown test interface %s", name)
	}
	return &iface.InterfaceInfo{Name: name, OsName: name, State: "up", MTU: 1500, Addresses: addresses}, nil
}

func (*connectedPrefixBackend) Close() error { return nil }

var registerConnectedPrefixBackend = sync.OnceValue(func() error {
	return iface.RegisterBackend("isis-connected-prefix-test", func() (iface.Backend, error) {
		return &connectedPrefixBackend{}, nil
	})
})

func TestISISReloadRefreshesConnectedPrefixes(t *testing.T) {
	if err := registerConnectedPrefixBackend(); err != nil {
		t.Fatal(err)
	}
	previous := iface.ActiveBackendName()
	if err := iface.LoadBackend("isis-connected-prefix-test"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := iface.CloseBackend(); err != nil {
			t.Error(err)
		}
		if previous != "" {
			if err := iface.LoadBackend(previous); err != nil {
				t.Error(err)
			}
		}
	})
	eng := newEngine(transport.New(&fakeBackend{}))
	t.Cleanup(eng.shutdown)
	eng.setConfig(Config{})

	const activate = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1-l2","interfaces":{"interface":{
"isis-conn-a":{"passive":"true","level":"l1-l2","metric":"10","address-family":{"ipv4-unicast":{},"ipv6-unicast":{}}},
"isis-conn-b":{"enabled":"false","passive":"true","level":"l2","metric":"37","address-family":{"ipv4-unicast":{},"ipv6-unicast":{}}}}}}}`
	const replace = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1-l2","interfaces":{"interface":{
"isis-conn-a":{"enabled":"false","passive":"true","level":"l1-l2","metric":"10","address-family":{"ipv4-unicast":{},"ipv6-unicast":{}}},
"isis-conn-b":{"passive":"true","level":"l2","metric":"37","address-family":{"ipv4-unicast":{},"ipv6-unicast":{}}}}}}}`
	const v4Only = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1-l2","interfaces":{"interface":{
"isis-conn-b":{"passive":"true","level":"l2","metric":"9","address-family":{"ipv4-unicast":{}}}}}}}`
	for _, step := range []struct {
		name   string
		config string
		l1     map[netip.Prefix]uint32
		l2     map[netip.Prefix]uint32
	}{
		{
			name: "first NET", config: activate,
			l1: map[netip.Prefix]uint32{netip.MustParsePrefix("192.0.2.0/24"): 10, netip.MustParsePrefix("2001:db8:1::/64"): 10},
			l2: map[netip.Prefix]uint32{netip.MustParsePrefix("192.0.2.0/24"): 10, netip.MustParsePrefix("2001:db8:1::/64"): 10},
		},
		{
			name: "replace passive interface", config: replace,
			l2: map[netip.Prefix]uint32{netip.MustParsePrefix("198.51.100.0/24"): 37, netip.MustParsePrefix("2001:db8:2::/64"): 37},
		},
		{
			name: "withdraw family and change metric", config: v4Only,
			l2: map[netip.Prefix]uint32{netip.MustParsePrefix("198.51.100.0/24"): 9},
		},
	} {
		reconcileTo(t, eng, step.config)
		if got := connectedLSPPrefixes(t, eng, lsdb.Level1); !maps.Equal(got, step.l1) {
			t.Fatalf("%s L1 prefixes = %v, want %v", step.name, got, step.l1)
		}
		if got := connectedLSPPrefixes(t, eng, lsdb.Level2); !maps.Equal(got, step.l2) {
			t.Fatalf("%s L2 prefixes = %v, want %v", step.name, got, step.l2)
		}
	}
}

func connectedLSPPrefixes(t *testing.T, eng *engine, level lsdb.Level) map[netip.Prefix]uint32 {
	t.Helper()
	prefixes := make(map[netip.Prefix]uint32)
	for _, raw := range eng.lsdb.RawSnapshot(level) {
		pdu, err := packet.DecodePDU(raw)
		if err != nil {
			t.Fatal(err)
		}
		for _, tlv := range pdu.LSP.TLVs {
			switch tlv.Type {
			case packet.TLVExtendedIPReach:
				reach, err := packet.DecodeExtendedIPReachTLV(tlv.Value)
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range reach.Entries {
					prefixes[entry.Prefix] = entry.Metric.Value()
				}
			case packet.TLVIPv6Reachability:
				reach, err := packet.DecodeIPv6ReachabilityTLV(tlv.Value)
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range reach.Entries {
					prefixes[entry.Prefix] = entry.Metric.Value()
				}
			}
		}
		packet.ReleaseTLVs(pdu.LSP.TLVs)
	}
	return prefixes
}
