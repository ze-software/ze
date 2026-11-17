//go:build integration && linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- fragment port classification.
// Related: fragment_boundary_integration_linux_test.go -- production operator SPD reconciliation.
// Related: rfc4301_transport_fragment_boundary_integration_linux_test.go -- IPv6 packet builders.
// RFC 4301 Section 7.4 and Appendix D.4 -- see rfc/short/rfc4301.md.
// RFC naming: untagged -- IPv6 raw fragment port classification awaiting native kernel proof.

package engine

import (
	"bytes"
	"errors"
	"maps"
	"net"
	"os"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestRFC4301IPv6RawOutputFragmentsRespectPortPolicies sends complete IPv6
// headers through a local raw socket, not AF_PACKET. BYPASS must match a port
// against a DISCARD fallback; DISCARD must match its port against a BYPASS
// fallback. A classifier that cannot read ports cannot pass either phase.
// RFC 4301 Appendix D.4: "An implementation also MUST permit a user or
// administrator to accept such traffic or reject such traffic using the SPD
// conventions described in Section 4.4.1."
//
//ze:guest-root
func TestRFC4301IPv6RawOutputFragmentsRespectPortPolicies(t *testing.T) {
	lab := newEspPolLab(t)
	ipv6FragmentPolicyAddresses(t, lab)
	lab.enter(t, lab.peer)
	bypassed := transportFragmentListen(t, fragmentBypassPort)
	discarded := transportFragmentListen(t, fragmentDiscardPort)
	adjacent := transportFragmentListen(t, fragmentDiscardPort+1)
	lab.enter(t, lab.ze)
	sender := ipv6FragmentPolicySender(t)
	if err := dataplane.Load("xfrm"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := dataplane.CloseBackend(); err != nil {
			t.Errorf("close the IPv6 fragment policy backend: %v", err)
		}
	})
	entries := map[string]ipsec.SPDPolicy{
		"ipv6-fragment-bypass": {
			Name: "ipv6-fragment-bypass", Action: dataplane.SPActionBypass, Order: 1000,
			Direction: ipsec.SPDDirOut, Protocol: unix.IPPROTO_UDP,
			LocalPrefix: mustCIDRNet(t, transportFragmentRemote+"/128"), LocalPort: ipsec.AnyPort(),
			RemotePrefix: mustCIDRNet(t, transportFragmentLocal+"/128"),
			RemotePort:   ipsec.PortSelector{Form: ipsec.PortSingle, Port: fragmentBypassPort},
		},
		"ipv6-fragment-discard": {
			Name: "ipv6-fragment-discard", Action: dataplane.SPActionDiscard, Order: 1001,
			Direction: ipsec.SPDDirOut, Protocol: unix.IPPROTO_UDP,
			LocalPrefix: mustCIDRNet(t, transportFragmentRemote+"/128"), LocalPort: ipsec.AnyPort(),
			RemotePrefix: mustCIDRNet(t, transportFragmentLocal+"/128"),
			RemotePort:   ipsec.PortSelector{Form: ipsec.PortSingle, Port: fragmentDiscardPort},
		},
		"ipv6-fragment-fallback": {
			Name: "ipv6-fragment-fallback", Action: dataplane.SPActionDiscard, Order: 2000,
			Direction:   ipsec.SPDDirOut,
			LocalPrefix: mustCIDRNet(t, transportFragmentRemote+"/128"), LocalPort: ipsec.AnyPort(),
			RemotePrefix: mustCIDRNet(t, transportFragmentLocal+"/128"), RemotePort: ipsec.AnyPort(),
		},
	}
	// RFC 4301 Sections 4.4.1 and 7.4: apply the operator's actual SPD entries.
	installSPDPolicies(dataplane.Get(), nil, entries, slogutil.DiscardLogger())
	t.Cleanup(func() { removeSPDPolicies(dataplane.Get(), entries, slogutil.DiscardLogger()) })
	fragmentWantOperatorPolicies(t, entries)
	forms := []string{"whole", "initial-first", "tail-first", "atomic"}
	for index, form := range forms {
		body := []byte("IPv6 matched BYPASS " + form)
		before := espPolStat(t, outStepStatPolBlock)
		ipv6FragmentPolicySend(t, sender, fragmentBypassPort, body, form, 0x6f740000+uint32(index), false)
		ipv6FragmentPolicyBody(t, bypassed, body)
		if got := espPolStat(t, outStepStatPolBlock); got != before {
			t.Fatalf("BYPASS %s: XfrmOutPolBlock moved from %d to %d", form, before, got)
		}
	}
	previous := maps.Clone(entries)
	fallback := entries["ipv6-fragment-fallback"]
	fallback.Action = dataplane.SPActionBypass
	entries[fallback.Name] = fallback
	// RFC 4301 Appendix D.4: the same production reconcile changes disposition.
	installSPDPolicies(dataplane.Get(), previous, entries, slogutil.DiscardLogger())
	fragmentWantOperatorPolicies(t, entries)
	for index, form := range forms {
		body := []byte("IPv6 matched DISCARD " + form)
		before := espPolStat(t, outStepStatPolBlock)
		ipv6FragmentPolicySend(t, sender, fragmentDiscardPort, body, form, 0x6f741000+uint32(index), true)
		awaitForwardKernelCounter(t, "IPv6 DISCARD "+form, uint64(before+1),
			func(t *testing.T) uint64 { return uint64(espPolStat(t, outStepStatPolBlock)) })
		// The adjacent destination differs only in its port. It must still pass
		// in the same fragment order, which excludes a blanket fragment refusal.
		control := []byte("IPv6 adjacent permitted port " + form)
		ipv6FragmentPolicySend(t, sender, fragmentDiscardPort+1, control, form, 0x6f742000+uint32(index), false)
		ipv6FragmentPolicyBody(t, adjacent, control)
		if got := espPolStat(t, outStepStatPolBlock); got != before+1 {
			t.Fatalf("DISCARD %s plus permitted neighbor: XfrmOutPolBlock=%d, want %d", form, got, before+1)
		}
		if err := discarded.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
			t.Fatal(err)
		}
		buffer := make([]byte, 256)
		n, _, err := discarded.ReadFromUDP(buffer)
		if err == nil {
			t.Fatalf("DISCARD %s delivered %d forbidden bytes", form, n)
		}
		var timeout net.Error
		if !errors.As(err, &timeout) {
			t.Fatal(err)
		}
		if !timeout.Timeout() {
			t.Fatal(err)
		}
	}
}

// ipv6FragmentPolicyAddresses puts the existing IPv6 builders' source on Ze
// and their destination on the peer. No ESP state or receive policy is installed.
func ipv6FragmentPolicyAddresses(t *testing.T, lab *espPolLab) {
	t.Helper()
	for _, side := range []struct {
		peer          bool
		link, address string
	}{
		{false, espPolZeLink, transportFragmentRemote},
		{true, espPolPeerLink, transportFragmentLocal},
	} {
		if side.peer {
			lab.enter(t, lab.peer)
		}
		for _, conf := range []string{"all", "default", side.link} {
			path := "/proc/sys/net/ipv6/conf/" + conf + "/disable_ipv6"
			if err := os.WriteFile(path, []byte("0"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		mhBoundaryAddress(t, side.link, side.address+"/64")
	}
	peerMAC := espPolLink(t, espPolPeerLink).Attrs().HardwareAddr
	lab.enter(t, lab.ze)
	if err := netlink.NeighAdd(&netlink.Neigh{
		LinkIndex: espPolLink(t, espPolZeLink).Attrs().Index, Family: unix.AF_INET6,
		State: netlink.NUD_PERMANENT, IP: net.ParseIP(transportFragmentLocal), HardwareAddr: peerMAC,
	}); err != nil {
		t.Fatalf("add the private IPv6 output neighbor: %v", err)
	}
}

func ipv6FragmentPolicySender(t *testing.T) int {
	t.Helper()
	fd, err := unix.Socket(unix.AF_INET6, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.IPPROTO_RAW)
	if err != nil {
		t.Fatalf("open the local IPv6 raw output socket: %v", err)
	}
	t.Cleanup(func() {
		if err := unix.Close(fd); err != nil {
			t.Errorf("close the local IPv6 raw output socket: %v", err)
		}
	})
	if err := unix.SetsockoptInt(fd, unix.IPPROTO_IPV6, unix.IPV6_HDRINCL, 1); err != nil {
		t.Fatal(err)
	}
	return fd
}

func ipv6FragmentPolicySend(t *testing.T, fd int, port uint16, body []byte, form string, id uint32, discard bool) {
	t.Helper()
	udp := transportFragmentUDP(port, body)
	packets := [][]byte{transportFragmentIPv6(unix.IPPROTO_UDP, udp)}
	switch form {
	case "whole":
	case "atomic":
		packets = [][]byte{transportFragmentIPv6(unix.IPPROTO_FRAGMENT, transportFragmentHeader(udp, 0, id))}
	case "initial-first", "tail-first":
		// Twenty-four bytes include the full UDP header and a multiple of eight
		// payload bytes. The second fragment contains no transport ports.
		packets = [][]byte{
			transportFragmentIPv6(unix.IPPROTO_FRAGMENT, transportFragmentHeader(udp[:24], 1, id)),
			transportFragmentIPv6(unix.IPPROTO_FRAGMENT, transportFragmentHeader(udp[24:], 24, id)),
		}
		if form == "tail-first" {
			packets[0], packets[1] = packets[1], packets[0]
		}
	default:
		t.Fatalf("unknown IPv6 output form %q", form)
	}
	target := &unix.SockaddrInet6{}
	copy(target.Addr[:], net.ParseIP(transportFragmentLocal).To16())
	for index, packet := range packets {
		err := unix.Sendto(fd, packet, 0, target)
		if err == nil {
			continue
		}
		if discard {
			if errors.Is(err, unix.EPERM) {
				continue
			}
			if errors.Is(err, unix.EACCES) {
				continue
			}
		}
		t.Fatalf("send IPv6 %s port %d fragment %d: %v", form, port, index, err)
	}
}

func ipv6FragmentPolicyBody(t *testing.T, receiver *net.UDPConn, want []byte) {
	t.Helper()
	if err := receiver.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, len(want)+1)
	n, source, err := receiver.ReadFromUDP(buffer)
	if err != nil {
		t.Fatalf("receive the permitted IPv6 raw-output datagram: %v", err)
	}
	if !bytes.Equal(buffer[:n], want) {
		t.Fatalf("IPv6 raw-output payload=%q, want %q", buffer[:n], want)
	}
	if !source.IP.Equal(net.ParseIP(transportFragmentRemote)) {
		t.Fatalf("IPv6 raw-output source=%s, want %s", source.IP, transportFragmentRemote)
	}
	if source.Port != sadSelSourcePort {
		t.Fatalf("IPv6 raw-output source port=%d, want %d", source.Port, sadSelSourcePort)
	}
}
