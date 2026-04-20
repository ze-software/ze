//go:build linux

package host

import (
	"slices"
	"testing"
)

// VALIDATES: AC-3 — `show host nic` on linux returns one entry per
// physical interface with driver, pci-vendor, pci-device, mac, speed,
// duplex, carrier, rx-queues, tx-queues populated.
// PREVENTS: regression where virtual or pseudo interfaces leak into
// the physical list.
func TestDetectNICs_PhysicalFields(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	nics, err := d.detectNICs(false) // skip ethtool for fixture test
	if err != nil {
		t.Fatalf("DetectNICs: %v", err)
	}
	// Fixture has 4 up NICs + 1 down NIC + docker0 + lo. Only the 5
	// with device/ subdir count; lo and docker0 are filtered.
	if got, want := len(nics), 5; got != want {
		t.Fatalf("len(nics) = %d, want %d — got names %v", got, want, nicNames(nics))
	}
	want := []string{"enp1s0", "enp2s0", "enp3s0", "enp4s0", "enp5s0"}
	if !slices.Equal(nicNames(nics), want) {
		t.Errorf("nic names = %v, want %v", nicNames(nics), want)
	}
	// Verify shape of the first entry.
	nic := nics[0]
	if nic.Driver != "igc" {
		t.Errorf("Driver = %q, want %q", nic.Driver, "igc")
	}
	if nic.PCIVendor != "8086" {
		t.Errorf("PCIVendor = %q, want %q", nic.PCIVendor, "8086")
	}
	if nic.PCIDevice != "125c" {
		t.Errorf("PCIDevice = %q, want %q", nic.PCIDevice, "125c")
	}
	if nic.Transport != NICTransportPCI {
		t.Errorf("Transport = %v, want pci", nic.Transport)
	}
	if nic.MAC == "" {
		t.Error("MAC empty")
	}
	if nic.LinkSpeedMbps != 2500 {
		t.Errorf("LinkSpeedMbps = %d, want 2500", nic.LinkSpeedMbps)
	}
	if nic.Duplex != "full" {
		t.Errorf("Duplex = %q, want full", nic.Duplex)
	}
	if !nic.Carrier {
		t.Error("Carrier = false, want true")
	}
	if nic.RxQueues != 4 {
		t.Errorf("RxQueues = %d, want 4", nic.RxQueues)
	}
	if nic.TxQueues != 4 {
		t.Errorf("TxQueues = %d, want 4", nic.TxQueues)
	}
}

// VALIDATES: AC-4 — virtual interfaces (bridge, veth, tun, loopback,
// docker0) are excluded from the NIC list via the device/ presence
// heuristic.
func TestDetectNICs_FiltersVirtual(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	nics, err := d.detectNICs(false)
	if err != nil {
		t.Fatalf("DetectNICs: %v", err)
	}
	for _, n := range nics {
		if n.Name == "lo" || n.Name == "docker0" {
			t.Errorf("virtual interface %q leaked into physical list", n.Name)
		}
	}
}

// VALIDATES: AC-3 carrier handling — when carrier=0, LinkSpeedMbps
// and Duplex are left zero/empty (kernel returns bogus values for
// down links, so we elide them).
func TestDetectNICs_CarrierDown(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	nics, err := d.detectNICs(false)
	if err != nil {
		t.Fatalf("DetectNICs: %v", err)
	}
	var down *NICInfo
	for i := range nics {
		if nics[i].Name == "enp5s0" {
			down = &nics[i]
			break
		}
	}
	if down == nil {
		t.Fatal("enp5s0 (down fixture) not found")
	}
	if down.Carrier {
		t.Error("Carrier = true, want false")
	}
	if down.LinkSpeedMbps != 0 {
		t.Errorf("LinkSpeedMbps = %d, want 0 on down link", down.LinkSpeedMbps)
	}
	if down.Duplex != "" {
		t.Errorf("Duplex = %q, want empty on down link", down.Duplex)
	}
}

// VALIDATES: uevent parsing extracts DRIVER=<name>, ignores other
// keys, and returns empty string when DRIVER line absent.
func TestParseDriverFromUevent(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"igc", "DRIVER=igc\nPCI_CLASS=20000\n", "igc"},
		{"trailing newline", "PCI_CLASS=20000\nDRIVER=e1000e\n", "e1000e"},
		{"no driver", "PCI_CLASS=20000\nPCI_ID=8086:125C\n", ""},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseDriverFromUevent(tc.in); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func nicNames(nics []NICInfo) []string {
	out := make([]string, len(nics))
	for i := range nics {
		out[i] = nics[i].Name
	}
	return out
}
