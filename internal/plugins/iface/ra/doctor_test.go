package ifacera

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// raTree builds a config tree with one advertising unit, in the shape the
// daemon holds it, so the check is driven the way `ze doctor` drives it.
func raTree(t *testing.T, kind, ifaceName string, enabled bool, backend string) *config.Tree {
	t.Helper()
	root := config.NewTree()
	ifaceContainer := root.GetOrCreateContainer("interface")
	if backend != "" {
		ifaceContainer.Set("backend", backend)
	}
	addRAUnit(t, ifaceContainer, kind, ifaceName, "0", enabled)
	return root
}

// addRAUnit adds one unit carrying a router-advertisement container.
func addRAUnit(t *testing.T, ifaceContainer *config.Tree, kind, ifaceName, unit string, enabled bool) {
	t.Helper()
	entry, ok := ifaceContainer.GetList(kind)[ifaceName]
	if !ok {
		entry = config.NewTree()
		ifaceContainer.AddListEntry(kind, ifaceName, entry)
	}
	unitEntry := config.NewTree()
	entry.AddListEntry("unit", unit, unitEntry)
	ra := unitEntry.GetOrCreateContainer("ipv6").GetOrCreateContainer("router-advertisement")
	if enabled {
		ra.Set("enabled", "true")
		return
	}
	ra.Set("enabled", "false")
}

func runForwardingCheck(t *testing.T, tree *config.Tree, forwarding map[string]bool) []diagnostic.Diagnostic {
	t.Helper()
	previous := ipv6ForwardingReader
	t.Cleanup(func() { ipv6ForwardingReader = previous })
	ipv6ForwardingReader = func(device string) (bool, bool) {
		on, known := forwarding[device]
		return on, known
	}
	return checkRAForwarding(diagnostic.DoctorCheckContext{Tree: tree})
}

// VALIDATES: design decision D-6. A unit that advertises while IPv6 forwarding
// is off on its device produces a warning naming the interface, because such a
// router tells hosts to send it traffic it will not forward.
// PREVENTS: a LAN where every host autoconfigures, points its default route at
// Ze, and gets no connectivity, with nothing in the output saying why.
func TestDoctorRAForwarding(t *testing.T) {
	t.Run("warns when forwarding is off on an advertising interface", func(t *testing.T) {
		got := runForwardingCheck(t, raTree(t, "ethernet", "eth0", true, ""), map[string]bool{"eth0": false})

		require.Len(t, got, 1)
		assert.Equal(t, "doctor-iface-ra-forwarding", got[0].Code)
		assert.Equal(t, diagnostic.SeverityWarning, got[0].Severity)
		assert.Contains(t, got[0].Message, "eth0")
		assert.Contains(t, got[0].Message, "forwarding")
	})

	t.Run("silent when forwarding is on", func(t *testing.T) {
		got := runForwardingCheck(t, raTree(t, "ethernet", "eth0", true, ""), map[string]bool{"eth0": true})
		assert.Empty(t, got)
	})

	t.Run("silent when the unit does not advertise", func(t *testing.T) {
		got := runForwardingCheck(t, raTree(t, "ethernet", "eth0", false, ""), map[string]bool{"eth0": false})
		assert.Empty(t, got)
	})

	t.Run("silent when the forwarding state cannot be read", func(t *testing.T) {
		// An unreadable sysctl is not evidence of a problem, so the check says
		// nothing rather than warning about a state it never saw.
		got := runForwardingCheck(t, raTree(t, "ethernet", "eth0", true, ""), map[string]bool{})
		assert.Empty(t, got)
	})

	t.Run("silent on the vpp backend", func(t *testing.T) {
		// The container is netlink-only, so a vpp tree carries no sender whose
		// forwarding state would matter.
		got := runForwardingCheck(t, raTree(t, "ethernet", "eth0", true, "vpp"), map[string]bool{"eth0": false})
		assert.Empty(t, got)
	})

	t.Run("silent with no interface configuration", func(t *testing.T) {
		assert.Empty(t, runForwardingCheck(t, config.NewTree(), map[string]bool{}))
	})

	t.Run("silent when the tree is not a config tree", func(t *testing.T) {
		previous := ipv6ForwardingReader
		t.Cleanup(func() { ipv6ForwardingReader = previous })
		ipv6ForwardingReader = func(string) (bool, bool) { return false, true }
		assert.Empty(t, checkRAForwarding(diagnostic.DoctorCheckContext{Tree: nil}))
	})

	for _, kind := range []string{"ethernet", "veth", "bridge", "dummy"} {
		t.Run("covers a "+kind+" interface", func(t *testing.T) {
			got := runForwardingCheck(t, raTree(t, kind, "if0", true, ""), map[string]bool{"if0": false})
			require.Len(t, got, 1, "an advertising %s unit must be checked too", kind)
			assert.Contains(t, got[0].Message, "if0")
		})
	}

	t.Run("one warning per interface, not per unit", func(t *testing.T) {
		tree := raTree(t, "ethernet", "eth0", true, "")
		addRAUnit(t, tree.GetContainer("interface"), "ethernet", "eth0", "1", true)

		got := runForwardingCheck(t, tree, map[string]bool{"eth0": false})
		assert.Len(t, got, 1, "forwarding is a device property, so two units share one warning")
	})

	t.Run("names every affected interface", func(t *testing.T) {
		tree := raTree(t, "ethernet", "eth0", true, "")
		addRAUnit(t, tree.GetContainer("interface"), "ethernet", "eth1", "0", true)

		got := runForwardingCheck(t, tree, map[string]bool{"eth0": false, "eth1": false})
		require.Len(t, got, 2)
		names := got[0].Message + " " + got[1].Message
		assert.Contains(t, names, "eth0")
		assert.Contains(t, names, "eth1")
	})
}

// VALIDATES: the doctor check is registered with the code it emits, so
// `ze doctor` runs it and `ze explain` can describe it.
// PREVENTS: a check that exists in code and never runs.
func TestDoctorRACheckRegistration(t *testing.T) {
	assert.Equal(t, "iface-ra-forwarding", raForwardingDoctorCheck.Name)
	assert.Equal(t, "iface", raForwardingDoctorCheck.Component)
	assert.Contains(t, raForwardingDoctorCheck.Codes, "doctor-iface-ra-forwarding")
	assert.NotNil(t, raForwardingDoctorCheck.Check)
}
