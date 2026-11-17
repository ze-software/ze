//go:build integration && linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- immutable SA identities in fragment queues.
// Related: rfc4301_any_scope_fragment_context_integration_linux_test.go -- ANY scopes and exact SAD counters.
// Related: rfc4301_ipv6_scope_fragment_context_integration_linux_test.go -- IPv6 packet builders and receiver.
// This is an ordinary lifetime regression, not an RFC conformance claim.

package engine

import (
	"bytes"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestFragmentSAReplacementKeepsQueuedIdentity queues an authenticated ANY
// fragment, removes its SA, and recreates the same public SAD identity and key.
// The replacement cannot complete that queue. Its own whole and fresh fragment
// controls must deliver, with exact receiving-SA counts in each generation.
//
//ze:guest-root
func TestFragmentSAReplacementKeepsQueuedIdentity(t *testing.T) {
	for _, family := range []int{unix.AF_INET, unix.AF_INET6} {
		name := "ipv4"
		if family == unix.AF_INET6 {
			name = "ipv6"
		}
		t.Run(name, func(t *testing.T) {
			lab := newEspPolLab(t)
			pairs, inside := fragmentSALifetimeReceiver(t, family)
			espPolSPD(t)
			children, sa := rfcScopeChildren(t, pairs)
			current := children[0]
			t.Cleanup(func() { removeChildSA(current, dataplane.Get(), slogutil.DiscardLogger()) })
			rfcScopeWantState(t, current, pairs, true, false)
			rfcScopeWantState(t, current, pairs, false, false)
			sender := newEspPolSender(t, lab, current)
			anyScopeFragmentWantPackets(t, sender)
			for order, tailFirst := range []bool{false, true} {
				identifier := uint32(0x6b720000 + order*16)
				whole, _, body := fragmentSALifetimePackets(family, identifier)
				fragmentSALifetimeSend(t, family, sender, whole)
				if !fragmentSALifetimeDelivered(t, family, inside, body) {
					t.Fatalf("tail-first=%v: original SA lost its whole control", tailFirst)
				}
				anyScopeFragmentWantPackets(t, sender)

				_, parts, body := fragmentSALifetimePackets(family, identifier+1)
				first, last := 0, 1
				if tailFirst {
					first, last = 1, 0
				}
				requests := fragmentSALifetimeReassemblyRequests(t, family)
				fragmentSALifetimeSend(t, family, sender, parts[first])
				if fragmentSALifetimeDelivered(t, family, inside, body) {
					t.Fatalf("tail-first=%v: a single fragment delivered a complete datagram", tailFirst)
				}
				anyScopeFragmentWantPackets(t, sender)
				if got := fragmentSALifetimeReassemblyRequests(t, family); got != requests+1 {
					t.Fatalf("first fragment reassembly requests=%d, want %d", got, requests+1)
				}
				before := fragmentSALifetimeState(t, current)
				removeChildSA(current, dataplane.Get(), slogutil.DiscardLogger())
				fragmentSALifetimeWantAbsent(t, current)
				replacement, err := createFirstChildSA(sa, current.ESPGroup, espPolZeOuter, espPolPeerOuter,
					current.IfID, dataplane.Get(), slogutil.DiscardLogger())
				if err != nil {
					t.Fatalf("recreate the same public Child SA identity: %v", err)
				}
				current = replacement
				if !current.ESPInstalled {
					t.Fatal("the replacement Child did not reach the kernel")
				}
				fragmentSALifetimeWantEquivalent(t, before, fragmentSALifetimeState(t, current))
				rfcScopeWantState(t, current, pairs, true, false)
				rfcScopeWantState(t, current, pairs, false, false)
				// A new logical SA starts a new replay and packet-count generation.
				// The existing encoder is usable because the actual key is unchanged.
				sender.seq = 0
				anyScopeFragmentWantPackets(t, sender)
				fragmentSALifetimeSend(t, family, sender, parts[last])
				if fragmentSALifetimeDelivered(t, family, inside, body) {
					t.Fatalf("tail-first=%v: replacement SA completed the removed SA's queued datagram", tailFirst)
				}
				anyScopeFragmentWantPackets(t, sender)

				whole, _, control := fragmentSALifetimePackets(family, identifier+2)
				fragmentSALifetimeSend(t, family, sender, whole)
				if !fragmentSALifetimeDelivered(t, family, inside, control) {
					t.Fatalf("tail-first=%v: replacement SA lost its whole control", tailFirst)
				}
				anyScopeFragmentWantPackets(t, sender)
				_, fresh, control := fragmentSALifetimePackets(family, identifier+3)
				fragmentSALifetimeSend(t, family, sender, fresh[first])
				fragmentSALifetimeSend(t, family, sender, fresh[last])
				if !fragmentSALifetimeDelivered(t, family, inside, control) {
					t.Fatalf("tail-first=%v: replacement SA lost its own fresh fragment control", tailFirst)
				}
				anyScopeFragmentWantPackets(t, sender)
				t.Logf("tail-first=%v: same-SPI/key replacement kept old and new queues separate", tailFirst)
			}
		})
	}
}

func fragmentSALifetimeReceiver(t *testing.T, family int) ([]tsPair, *net.UDPConn) {
	t.Helper()
	if family == unix.AF_INET6 {
		for _, conf := range []string{"all", "default", "lo"} {
			path := "/proc/sys/net/ipv6/conf/" + conf + "/disable_ipv6"
			if err := os.WriteFile(path, []byte("0"), 0o600); err != nil {
				t.Fatalf("enable IPv6 in the private SA-lifetime receiver: %v", err)
			}
		}
		mhBoundaryAddress(t, "lo", transportFragmentLocal+"/128")
		return []tsPair{{
			I: tsSelector{Net: mustCIDRNet(t, transportFragmentLocal+"/128"), Port: ipsec.AnyPort()},
			R: tsSelector{Net: mustCIDRNet(t, transportFragmentRemote+"/128"), Port: ipsec.AnyPort()},
		}}, transportFragmentListen(t, transportFragmentPort)
	}
	return []tsPair{{
		I: tsSelector{Net: mustCIDRNet(t, "10.2.0.0/16"), Port: ipsec.AnyPort()},
		R: tsSelector{Net: mustCIDRNet(t, "10.1.0.0/16"), Port: ipsec.AnyPort()},
	}}, sadSelListen(t, sadSelInnerLocal, sadSelPort)
}

// fragmentSALifetimePackets uses the existing small, valid ANY-scope datagrams.
// Each identifier is unique across partial and complete controls in one lab.
func fragmentSALifetimePackets(family int, identifier uint32) (whole []byte, parts [2][]byte, body []byte) {
	if family == unix.AF_INET6 {
		body = ipv6ScopeFragmentBody(identifier)
		udp := transportFragmentUDP(transportFragmentPort, body)
		whole = transportFragmentIPv6(unix.IPPROTO_UDP, udp)
		parts = [2][]byte{
			transportFragmentIPv6(unix.IPPROTO_FRAGMENT, transportFragmentHeader(udp[:16], 1, identifier)),
			transportFragmentIPv6(unix.IPPROTO_FRAGMENT, transportFragmentHeader(udp[16:], 16, identifier)),
		}
		return whole, parts, body
	}
	whole = fragmentContextPacket(sadSelPort, uint16(identifier))
	first, last := fragSplit(whole)
	fragmentPacketID(first, uint16(identifier))
	fragmentPacketID(last, uint16(identifier))
	return whole, [2][]byte{first, last}, whole[28:]
}

func fragmentSALifetimeSend(t *testing.T, family int, sender *espPolSender, packet []byte) {
	t.Helper()
	if family == unix.AF_INET6 {
		ipv6ScopeFragmentSend(t, sender, packet)
		return
	}
	sender.sendEncap(t, packet)
}

func fragmentSALifetimeDelivered(t *testing.T, family int, inside *net.UDPConn, body []byte) bool {
	t.Helper()
	if family == unix.AF_INET6 {
		return ipv6ScopeFragmentDelivered(t, inside, body)
	}
	return fragmentContextDelivered(t, inside, body)
}

// fragmentSALifetimeReassemblyRequests reads the creating thread's namespace.
// IPv4 snmp_seq_show_ipstats emits paired Ip: names and values, including
// ReasmReqds. A missing field or a malformed pair cannot count as no requests.
func fragmentSALifetimeReassemblyRequests(t *testing.T, family int) uint64 {
	t.Helper()
	if family == unix.AF_INET6 {
		return transportFragmentReassembly(t, "Ip6ReasmReqds")
	}
	raw, err := os.ReadFile("/proc/thread-self/net/snmp")
	if err != nil {
		t.Fatalf("read the private IPv4 reassembly requests: %v", err)
	}
	var names []string
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] != "Ip:" {
			continue
		}
		if names == nil {
			names = fields
			continue
		}
		if len(fields) != len(names) {
			t.Fatal("the private IPv4 SNMP names and values have different lengths")
		}
		for i, name := range names {
			if name != "ReasmReqds" {
				continue
			}
			value, err := strconv.ParseUint(fields[i], 10, 64)
			if err != nil {
				t.Fatalf("parse the private IPv4 ReasmReqds: %v", err)
			}
			return value
		}
		break
	}
	t.Fatal("ReasmReqds is absent from the private IPv4 SNMP values")
	return 0
}

func fragmentSALifetimeState(t *testing.T, child *ChildSA) *netlink.XfrmState {
	t.Helper()
	state, err := netlink.XfrmStateGet(&netlink.XfrmState{
		Dst: child.LocalAddr, Spi: int(child.InboundSPI), Proto: netlink.XFRM_PROTO_ESP,
	})
	if err != nil {
		t.Fatalf("read the actual SA-lifetime state: %v", err)
	}
	return state
}

func fragmentSALifetimeWantAbsent(t *testing.T, child *ChildSA) {
	t.Helper()
	for _, identity := range []struct {
		spi uint32
		dst net.IP
	}{{child.InboundSPI, child.LocalAddr}, {child.OutboundSPI, child.RemoteAddr}} {
		_, err := netlink.XfrmStateGet(&netlink.XfrmState{
			Dst: identity.dst, Spi: int(identity.spi), Proto: netlink.XFRM_PROTO_ESP,
		})
		if err == nil {
			t.Fatalf("removed SA %#x is still installed", identity.spi)
		}
		if errors.Is(err, unix.ESRCH) {
			continue
		}
		if errors.Is(err, unix.ENOENT) {
			continue
		}
		t.Fatalf("read removed SA %#x: %v", identity.spi, err)
	}
}

// fragmentSALifetimeWantEquivalent checks the real replacement rather than
// inferring identity and key preservation from the constructor arguments.
func fragmentSALifetimeWantEquivalent(t *testing.T, before, after *netlink.XfrmState) {
	t.Helper()
	if before.Spi != after.Spi {
		t.Fatal("the replacement changed the public inbound SPI")
	}
	if !before.Src.Equal(after.Src) {
		t.Fatal("the replacement changed the outer source")
	}
	if !before.Dst.Equal(after.Dst) {
		t.Fatal("the replacement changed the outer destination")
	}
	if before.Mode != after.Mode {
		t.Fatal("the replacement changed the tunnel mode")
	}
	if before.Proto != after.Proto {
		t.Fatal("the replacement changed the IPsec protocol")
	}
	if before.ReplayWindow != after.ReplayWindow {
		t.Fatal("the replacement changed the replay window")
	}
	if before.Reqid != after.Reqid {
		t.Fatal("the replacement changed the request id")
	}
	if before.Ifid != after.Ifid {
		t.Fatal("the replacement changed the XFRM interface id")
	}
	if before.Aead == nil {
		t.Fatal("the original state has no AEAD key")
	}
	if after.Aead == nil {
		t.Fatal("the replacement state has no AEAD key")
	}
	if before.Aead.Name != after.Aead.Name {
		t.Fatal("the replacement changed the AEAD transform")
	}
	if before.Aead.ICVLen != after.Aead.ICVLen {
		t.Fatal("the replacement changed the AEAD integrity length")
	}
	if !bytes.Equal(before.Aead.Key, after.Aead.Key) {
		t.Fatal("the replacement changed the actual AEAD key")
	}
	if before.Encap == nil {
		t.Fatal("the original state has no ESP-in-UDP template")
	}
	if after.Encap == nil {
		t.Fatal("the replacement has no ESP-in-UDP template")
	}
	if before.Encap.Type != after.Encap.Type {
		t.Fatal("the replacement changed the ESP-in-UDP form")
	}
	if before.Encap.SrcPort != after.Encap.SrcPort {
		t.Fatal("the replacement changed the ESP-in-UDP source port")
	}
	if before.Encap.DstPort != after.Encap.DstPort {
		t.Fatal("the replacement changed the ESP-in-UDP destination port")
	}
	if !before.Encap.OriginalAddress.Equal(after.Encap.OriginalAddress) {
		t.Fatal("the replacement changed the ESP-in-UDP original address")
	}
}
