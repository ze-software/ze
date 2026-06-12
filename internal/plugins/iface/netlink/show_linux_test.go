//go:build linux

package ifacenetlink

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
)

func TestListInterfaces(t *testing.T) {
	b := &netlinkBackend{}
	ifaces, err := b.ListInterfaces()
	require.NoError(t, err)
	require.NotEmpty(t, ifaces, "should find at least one interface")

	var found bool
	for i := range ifaces {
		if ifaces[i].Name != "lo" {
			continue
		}
		found = true
		assert.Equal(t, "up", ifaces[i].State)
		assert.Greater(t, ifaces[i].MTU, 0)
		assert.Greater(t, ifaces[i].Index, 0)
		break
	}
	assert.True(t, found, "loopback interface not found")
}

func TestGetInterface(t *testing.T) {
	b := &netlinkBackend{}
	info, err := b.GetInterface("lo")
	require.NoError(t, err)
	require.NotNil(t, info)

	assert.Equal(t, "lo", info.Name)
	assert.Equal(t, "up", info.State)
	assert.Greater(t, info.Index, 0)
	assert.Greater(t, info.MTU, 0)
	assert.NotEmpty(t, info.Addresses, "loopback should have at least 127.0.0.1")
	assert.NotNil(t, info.Stats)
}

func TestGetInterfaceNotFound(t *testing.T) {
	b := &netlinkBackend{}
	_, err := b.GetInterface("nonexistent_iface99")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent_iface99")
}

func TestGetInterfaceInvalidName(t *testing.T) {
	b := &netlinkBackend{}
	_, err := b.GetInterface("")
	require.Error(t, err)

	_, err = b.GetInterface("a/b")
	require.Error(t, err)
}

// TestLinkToInfoVLANQoSMaps verifies the QoS maps the kernel reports on a VLAN
// link are copied into InterfaceInfo so show output exposes them, and that a
// VLAN without maps yields nil (no JSON keys emitted).
//
// VALIDATES: spec-vlan-qos-map AC-7 -- show interface displays QoS maps.
// PREVENTS: maps configured but invisible to operators.
func TestLinkToInfoVLANQoSMaps(t *testing.T) {
	vlan := &netlink.Vlan{
		LinkAttrs:     netlink.LinkAttrs{Name: "eth0.100", Index: 7, ParentIndex: 2, MTU: 1500},
		VlanId:        100,
		IngressQosMap: map[uint32]uint32{6: 6},
		EgressQosMap:  map[uint32]uint32{0: 0, 6: 6},
	}
	info := linkToInfo(vlan)
	assert.Equal(t, 100, info.VlanID)
	assert.Equal(t, map[uint32]uint32{6: 6}, info.IngressQoSMap)
	assert.Equal(t, map[uint32]uint32{0: 0, 6: 6}, info.EgressQoSMap)

	bare := &netlink.Vlan{
		LinkAttrs: netlink.LinkAttrs{Name: "eth0.200", Index: 8, ParentIndex: 2, MTU: 1500},
		VlanId:    200,
	}
	bareInfo := linkToInfo(bare)
	assert.Nil(t, bareInfo.IngressQoSMap)
	assert.Nil(t, bareInfo.EgressQoSMap)
}

// TestParseLinkSpeedDuplex covers the sysfs value handling for the flow-export
// sFlow ifSpeed/ifDirection fields: a real ethernet link, a down link (kernel
// reports "-1" / "unknown"), and garbage. Split from the file reads so the
// sanitisation is testable without /sys.
func TestParseLinkSpeedDuplex(t *testing.T) {
	cases := []struct {
		name       string
		speedRaw   string
		duplexRaw  string
		wantSpeed  int
		wantDuplex string
	}{
		{"ethernet full", "10000\n", "full\n", 10000, "full"},
		{"ethernet half", "100\n", "half\n", 100, "half"},
		{"down link", "-1\n", "unknown\n", 0, ""},
		{"empty", "", "", 0, ""},
		{"garbage speed", "notanumber", "full", 0, "full"},
		{"zero speed", "0", "full", 0, "full"},
		{"unknown duplex", "1000", "weird", 1000, ""},
	}
	for _, c := range cases {
		gotSpeed, gotDuplex := parseLinkSpeedDuplex(c.speedRaw, c.duplexRaw)
		assert.Equal(t, c.wantSpeed, gotSpeed, "%s: speed", c.name)
		assert.Equal(t, c.wantDuplex, gotDuplex, "%s: duplex", c.name)
	}
}

// TestLinkSpeedDuplexLoopback verifies the backend method returns the unknown
// pair for loopback (no /sys speed/duplex), confirming the read path is wired
// and degrades cleanly. The generic ListInterfaces/GetInterface output no longer
// carries speed/duplex at all (removed from InterfaceInfo), so only this
// explicit accessor performs the sysfs read.
func TestLinkSpeedDuplexLoopback(t *testing.T) {
	b := &netlinkBackend{}
	speed, duplex := b.LinkSpeedDuplex("lo")
	assert.Equal(t, 0, speed)
	assert.Equal(t, "", duplex)
}
