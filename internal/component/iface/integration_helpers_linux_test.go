//go:build integration && linux

package iface

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	_ "codeberg.org/thomas-mangin/ze/internal/plugins/ifacenetlink"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// ensureBackendForIntegration loads the netlink backend if not already loaded.
func ensureBackendForIntegration(t *testing.T) {
	t.Helper()
	if GetBackend() != nil {
		return
	}
	if err := LoadBackend("netlink"); err != nil {
		t.Fatalf("load netlink backend: %v", err)
	}
	t.Cleanup(func() { _ = CloseBackend() })
}

// withNetNS creates an ephemeral network namespace, switches into it,
// runs fn, then restores the original namespace in t.Cleanup.
// The current goroutine is locked to its OS thread for the duration.
//
// If namespace creation fails (e.g., missing CAP_NET_ADMIN), the test
// is skipped rather than failed, enabling graceful degradation on
// unprivileged hosts.
func withNetNS(t *testing.T, fn func()) {
	t.Helper()
	ensureBackendForIntegration(t)

	runtime.LockOSThread()

	origNS, err := netns.Get()
	if err != nil {
		t.Skipf("requires CAP_NET_ADMIN: cannot get current namespace: %v", err)
	}

	// Derive namespace name from test name, truncated to fit IFNAMSIZ-like limits.
	nsName := sanitizeNSName(t.Name())

	newNS, err := netns.NewNamed(nsName)
	if err != nil {
		origNS.Close()
		t.Skipf("requires CAP_NET_ADMIN: cannot create namespace: %v", err)
	}

	t.Cleanup(func() {
		// Restore original namespace.
		if restoreErr := netns.Set(origNS); restoreErr != nil {
			t.Errorf("failed to restore original namespace: %v", restoreErr)
		}
		origNS.Close()
		newNS.Close()
		// Delete the named namespace.
		netns.DeleteNamed(nsName) //nolint:errcheck // best-effort cleanup
		runtime.UnlockOSThread()
	})

	fn()
}

// sanitizeNSName derives a valid namespace name from a test name.
// Linux namespace names are limited; we truncate and replace invalid chars.
func sanitizeNSName(testName string) string {
	// Replace slashes and other problematic characters.
	name := strings.NewReplacer(
		"/", "_",
		" ", "_",
		"(", "",
		")", "",
	).Replace(testName)

	// Truncate to 15 characters (IFNAMSIZ - 1).
	if len(name) > 15 {
		name = name[:15]
	}
	return name
}

// collectingBus is a minimal Bus that records published events.
type collectingBus struct {
	mu     sync.Mutex
	events []ze.Event
}

func (b *collectingBus) CreateTopic(string) (ze.Topic, error) { return ze.Topic{}, nil }
func (b *collectingBus) Publish(topic string, payload []byte, metadata map[string]string) {
	b.mu.Lock()
	b.events = append(b.events, ze.Event{Topic: topic, Payload: payload, Metadata: metadata})
	b.mu.Unlock()
}
func (b *collectingBus) Subscribe(string, map[string]string, ze.Consumer) (ze.Subscription, error) {
	return ze.Subscription{}, nil
}
func (b *collectingBus) Unsubscribe(ze.Subscription) {}

func (b *collectingBus) snapshot() []ze.Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	cp := make([]ze.Event, len(b.events))
	copy(cp, b.events)
	return cp
}

// waitForEvent polls the collectingBus events list for an event matching
// the given topic, with timeout. Returns the matching event or fails.
func waitForEvent(t *testing.T, bus *collectingBus, topic string, timeout time.Duration) ze.Event {
	t.Helper()

	deadline := time.Now().Add(timeout)
	seen := 0
	for time.Now().Before(deadline) {
		events := bus.snapshot()
		for i := seen; i < len(events); i++ {
			if events[i].Topic == topic {
				return events[i]
			}
		}
		seen = len(events)
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for event on topic %q (saw %d events)", topic, seen)
	return ze.Event{} // unreachable
}

// linkExists returns true if netlink.LinkByName succeeds for the given name.
func linkExists(name string) bool {
	_, err := netlink.LinkByName(name)
	return err == nil
}

// hasAddress returns true if the address (CIDR notation) appears in the
// address list for the named interface.
func hasAddress(ifaceName, cidr string) bool {
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return false
	}

	target, err := netlink.ParseAddr(cidr)
	if err != nil {
		return false
	}

	addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		return false
	}

	for _, a := range addrs {
		if a.IP.Equal(target.IP) {
			tOnes, tBits := target.Mask.Size()
			aOnes, aBits := a.Mask.Size()
			if tOnes == aOnes && tBits == aBits {
				return true
			}
		}
	}
	return false
}

// requireLinkUp verifies that the named link exists and is administratively up
// (IFF_UP flag set). Fails the test if not.
func requireLinkUp(t *testing.T, name string) {
	t.Helper()
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("link %q not found: %v", name, err)
	}
	attrs := link.Attrs()
	if attrs == nil {
		t.Fatalf("link %q has nil attrs", name)
	}
	if attrs.Flags&1 == 0 { // net.FlagUp == 1
		t.Errorf("link %q is not UP (flags=%x)", name, attrs.Flags)
	}
}

// createDummyForTest creates a dummy interface and registers cleanup.
func createDummyForTest(t *testing.T, name string) {
	t.Helper()
	if err := CreateDummy(name); err != nil {
		t.Fatalf("create dummy %q: %v", name, err)
	}
	t.Cleanup(func() {
		_ = DeleteInterface(name) // best-effort cleanup
	})
}

// createVethForTest creates a veth pair and registers cleanup.
func createVethForTest(t *testing.T, name, peerName string) {
	t.Helper()
	if err := CreateVeth(name, peerName); err != nil {
		t.Fatalf("create veth %q/%q: %v", name, peerName, err)
	}
	t.Cleanup(func() {
		_ = DeleteInterface(name) // deleting one end removes both
	})
}

// requireNoLink verifies that the named link does not exist.
func requireNoLink(t *testing.T, name string) {
	t.Helper()
	if linkExists(name) {
		t.Errorf("link %q should not exist but does", name)
	}
}

// requireAddress verifies that the given CIDR is present on the interface.
func requireAddress(t *testing.T, ifaceName, cidr string) {
	t.Helper()
	if !hasAddress(ifaceName, cidr) {
		t.Errorf("address %q not found on %q", cidr, ifaceName)
		// Print actual addresses for diagnostics.
		link, err := netlink.LinkByName(ifaceName)
		if err != nil {
			return
		}
		addrs, _ := netlink.AddrList(link, netlink.FAMILY_ALL)
		for _, a := range addrs {
			t.Logf("  actual addr: %s", a.IPNet.String())
		}
	}
}

// requireNoAddress verifies that the given CIDR is NOT present on the interface.
func requireNoAddress(t *testing.T, ifaceName, cidr string) {
	t.Helper()
	if hasAddress(ifaceName, cidr) {
		t.Errorf("address %q should not be on %q but is", cidr, ifaceName)
	}
}

// uniqueName returns a short unique interface name suitable for Linux
// (max 15 chars). The suffix ensures no collisions between tests in the
// same namespace.
func uniqueName(prefix string, idx int) string {
	name := fmt.Sprintf("%s%d", prefix, idx)
	if len(name) > 15 {
		name = name[:15]
	}
	return name
}
