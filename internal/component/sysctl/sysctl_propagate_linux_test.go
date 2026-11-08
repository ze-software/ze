//go:build linux

package sysctl

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// propagatingBackend is a fakeBackend that copies a write of
// net.ipv6.conf.all.forwarding into every device's forwarding and into
// default.forwarding, and zeroes every force_forwarding on a write of 0, as
// net/ipv6/addrconf.c addrconf_fixup_forwarding and addrconf_forward_change
// do. It records every write after the test's setup.
type propagatingBackend struct {
	*fakeBackend
	writes []string
}

func (p *propagatingBackend) write(key, value string) error {
	p.writes = append(p.writes, key+"="+value)
	p.values[key] = value
	if key != "net.ipv6.conf.all.forwarding" {
		return nil
	}
	for k := range p.values {
		if !strings.HasPrefix(k, "net.ipv6.conf.") {
			continue
		}
		if strings.HasSuffix(k, ".forwarding") {
			p.values[k] = value
		}
		if value == "0" && strings.HasSuffix(k, ".force_forwarding") {
			p.values[k] = "0"
		}
	}
	return nil
}

func newPropagatingStore(values map[string]string) (*store, *propagatingBackend) {
	pb := &propagatingBackend{fakeBackend: newFakeBackend()}
	for k, v := range values {
		pb.values[k] = v
	}
	return newStore(pb, slogutil.DiscardLogger()), pb
}

// TestGlobalIPv6ForwardingReappliesManagedDevices proves that after the global
// IPv6 forwarding switch changes, a device the store manages gets its value
// back, and every other device keeps the value the kernel copied into it.
//
// VALIDATES: eth1's unit says forwarding false (store default 0); eth0 and
// default are managed by nobody. After all.forwarding goes to 1, eth1 is 0
// again while eth0 and default keep the kernel's 1, and eth2, managed at 1,
// is never rewritten (a write of 1 flushes RA default routes). After the
// source withdraws, all.forwarding is 0, eth2 is re-applied to 1, and eth0
// keeps the kernel's 0.
// PREVENTS: a unit with ipv6 forwarding false turned into a router by the
// switch another unit needed, and Ze overriding the router role the kernel
// gives every unmanaged device while it forwards (FRR and VyOS let it stand).
func TestGlobalIPv6ForwardingReappliesManagedDevices(t *testing.T) {
	s, pb := newPropagatingStore(map[string]string{
		"net.ipv6.conf.all.forwarding":     "0",
		"net.ipv6.conf.default.forwarding": "0",
		"net.ipv6.conf.eth0.forwarding":    "0",
		"net.ipv6.conf.eth1.forwarding":    "1",
		"net.ipv6.conf.eth2.forwarding":    "0",
	})
	if _, err := s.setDefault("net.ipv6.conf.eth1.forwarding", "0", "interface"); err != nil {
		t.Fatalf("eth1 default: %v", err)
	}
	if _, err := s.setDefault("net.ipv6.conf.eth2.forwarding", "1", "interface"); err != nil {
		t.Fatalf("eth2 default: %v", err)
	}
	pb.writes = nil

	if _, err := s.setDefault("net.ipv6.conf.all.forwarding", "1", "interface-ipv6-forwarding"); err != nil {
		t.Fatalf("all default: %v", err)
	}
	want := map[string]string{
		"net.ipv6.conf.all.forwarding":     "1",
		"net.ipv6.conf.default.forwarding": "1",
		"net.ipv6.conf.eth0.forwarding":    "1",
		"net.ipv6.conf.eth1.forwarding":    "0",
		"net.ipv6.conf.eth2.forwarding":    "1",
	}
	for k, v := range want {
		if got := pb.values[k]; got != v {
			t.Errorf("after switching on: %s = %q, want %q", k, got, v)
		}
	}
	for _, w := range pb.writes {
		if strings.HasPrefix(w, "net.ipv6.conf.eth2.") {
			t.Errorf("eth2, already at its managed 1, was rewritten (%s): a write of 1 flushes RA default routes", w)
		}
		if strings.HasPrefix(w, "net.ipv6.conf.eth0.") {
			t.Errorf("unmanaged eth0 was rewritten (%s): the kernel's copy must stand", w)
		}
	}

	s.clearSourceDefaults("interface-ipv6-forwarding")
	want = map[string]string{
		"net.ipv6.conf.all.forwarding":  "0",
		"net.ipv6.conf.eth0.forwarding": "0",
		"net.ipv6.conf.eth1.forwarding": "0",
		"net.ipv6.conf.eth2.forwarding": "1",
	}
	for k, v := range want {
		if got := pb.values[k]; got != v {
			t.Errorf("after switching off: %s = %q, want %q", k, got, v)
		}
	}
}

// TestRestoreAllPutsPerDeviceOriginalsLast proves that the clean-stop restore
// writes the global switch before the per-device keys, so each device ends at
// its own original rather than at the global original the kernel copied.
//
// VALIDATES: eth1 was 1 before Ze and Ze kept it 1; all.forwarding was 0.
// After restoreAll, all.forwarding is 0 and eth1 is 1.
// PREVENTS: a map-order restore that writes all.forwarding=0 last and zeroes
// a device the operator had forwarding before Ze started.
func TestRestoreAllPutsPerDeviceOriginalsLast(t *testing.T) {
	for range 20 { // Map iteration order varies; repeat so a wrong order shows.
		s, pb := newPropagatingStore(map[string]string{
			"net.ipv6.conf.all.forwarding":  "0",
			"net.ipv6.conf.eth1.forwarding": "1",
		})
		if _, err := s.setDefault("net.ipv6.conf.eth1.forwarding", "0", "interface"); err != nil {
			t.Fatalf("eth1 default: %v", err)
		}
		if _, err := s.setDefault("net.ipv6.conf.all.forwarding", "1", "interface-ipv6-forwarding"); err != nil {
			t.Fatalf("all default: %v", err)
		}
		s.restoreAll()
		if got := pb.values["net.ipv6.conf.all.forwarding"]; got != "0" {
			t.Fatalf("all.forwarding = %q after restore, want the original 0", got)
		}
		if got := pb.values["net.ipv6.conf.eth1.forwarding"]; got != "1" {
			t.Fatalf("eth1.forwarding = %q after restore, want its original 1", got)
		}
	}
}
