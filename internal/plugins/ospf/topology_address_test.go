// Design: docs/architecture/ospf/ospf-4-component-config.md -- isolated live address inventory.
package ospf

import (
	"fmt"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
)

const topologyAddressBackendName = "ospf-topology-address-test"

// topologyAddressBackend gives fake raw transports a complete address inventory.
// Unknown devices still fail, so an omitted fixture device cannot look healthy.
type topologyAddressBackend struct{ rfc7474AddressBackend }

func (*topologyAddressBackend) GetInterface(name string) (*iface.InterfaceInfo, error) {
	info := rfc7474Eth0()
	info.Name = name
	info.OsName = name
	info.AddressesRead = true
	switch name {
	case "eth0":
		// Keep the existing fixture's 192.0.2.1/24 source address.
	case "eth1":
		info.Index = 2
		info.Addresses[0].Address = "198.51.100.1"
	case "eth2":
		info.Index = 3
		info.Addresses[0].Address = "203.0.113.1"
	case "lo":
		info.Index = 4
		info.Type = "loopback"
		info.Addresses[0].Address = "10.0.0.1"
		info.Addresses[0].PrefixLength = 32
	default:
		return nil, fmt.Errorf("unknown topology test interface %s", name)
	}
	return &info, nil
}

// GetLink returns link facts without pretending that a link lookup read addresses.
func (b *topologyAddressBackend) GetLink(name string) (*iface.InterfaceInfo, error) {
	info, err := b.GetInterface(name)
	if err != nil {
		return nil, err
	}
	info.Addresses = nil
	info.AddressesRead = false
	return info, nil
}

func (b *topologyAddressBackend) ListInterfaces() ([]iface.InterfaceInfo, error) {
	var infos []iface.InterfaceInfo
	for _, name := range []string{"eth0", "eth1", "eth2", "lo"} {
		info, err := b.GetInterface(name)
		if err != nil {
			return nil, err
		}
		infos = append(infos, *info)
	}
	return infos, nil
}

var registerTopologyAddressBackend = sync.OnceValue(func() error {
	return iface.RegisterBackend(topologyAddressBackendName, func() (iface.Backend, error) {
		return &topologyAddressBackend{}, nil
	})
})

// installTopologyAddressBackend supplies the OS inventory that an in-memory raw
// transport cannot provide. Engines MUST stop before its cleanup restores the backend.
// Tests using this helper MUST NOT run in parallel: the active iface backend is global.
func installTopologyAddressBackend(t *testing.T) {
	t.Helper()
	if err := registerTopologyAddressBackend(); err != nil {
		t.Fatal(err)
	}
	previous := iface.ActiveBackendName()
	if err := iface.LoadBackend(topologyAddressBackendName); err != nil {
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
	for _, name := range []string{"eth0", "eth1", "eth2", "lo"} {
		addresses, err := iface.Addresses(name)
		if err != nil {
			t.Fatalf("precondition: %s address inventory: %v", name, err)
		}
		if len(addresses) != 1 {
			t.Fatalf("precondition: %s addresses = %+v, want one fixture address", name, addresses)
		}
	}
}
