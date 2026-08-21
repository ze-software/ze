//go:build linux

// Design: docs/research/l2tpv2-ze-integration.md -- kernel netlink diagnostic
//
// Standalone diagnostic: creates an L2TP kernel tunnel using the same
// vishvananda/netlink library and attribute encoding as Ze. Run inside
// a network namespace with connectivity to isolate library vs process issues.
//
// Usage (inside QEMU VM):
//   CGO_ENABLED=0 ip netns exec zens go run -buildvcs=false scripts/evidence/l2tp-tunnel-diag/main.go \
//       -local 172.30.0.1 -remote 172.30.0.2 -sport 1701 -dport 1702

package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"

	"github.com/vishvananda/netlink"
	nl "github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

const (
	l2tpCmdTunnelCreate = 1
	// l2tpCmdTunnelGet is L2TP_CMD_TUNNEL_GET (linux/l2tp.h). Dumped with
	// NLM_F_DUMP it lists every tunnel the kernel holds, which is the readback
	// this diagnostic prints instead of forking iproute2.
	l2tpCmdTunnelGet = 4
	genlL2TPVersion  = 1

	l2tpAttrEncapType    = 2
	l2tpAttrProtoVersion = 7
	l2tpAttrConnID       = 9
	l2tpAttrPeerConnID   = 10
	l2tpAttrIPSAddr      = 24
	l2tpAttrIPDAddr      = 25
	l2tpAttrUDPSPort     = 26
	l2tpAttrUDPDPort     = 27
)

func main() {
	localIP := flag.String("local", "172.30.0.1", "local IP")
	remoteIP := flag.String("remote", "172.30.0.2", "remote IP")
	sport := flag.Int("sport", 1701, "source UDP port")
	dport := flag.Int("dport", 1702, "destination UDP port")
	tunnelID := flag.Int("tid", 1, "local tunnel ID")
	peerTID := flag.Int("ptid", 100, "peer tunnel ID")
	flag.Parse()

	family, err := netlink.GenlFamilyGet("l2tp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve l2tp genl family: %v\n", err)
		os.Exit(1)
	}
	say("l2tp genl family ID: %d\n", family.ID)

	saddr := parseIPv4(*localIP)
	daddr := parseIPv4(*remoteIP)

	req := nl.NewNetlinkRequest(int(family.ID), unix.NLM_F_ACK)
	// Zero the reserved field explicitly; nl.Genlmsg leaks stack garbage.
	var genlHdr [4]byte
	genlHdr[0] = l2tpCmdTunnelCreate
	genlHdr[1] = genlL2TPVersion
	req.AddData((*genlBytes)(&genlHdr))
	req.AddData(nl.NewRtAttr(l2tpAttrConnID, nl.Uint32Attr(uint32(*tunnelID))))
	req.AddData(nl.NewRtAttr(l2tpAttrPeerConnID, nl.Uint32Attr(uint32(*peerTID))))
	req.AddData(nl.NewRtAttr(l2tpAttrProtoVersion, nl.Uint8Attr(3)))
	req.AddData(nl.NewRtAttr(l2tpAttrEncapType, nl.Uint16Attr(0)))
	req.AddData(nl.NewRtAttr(l2tpAttrUDPSPort, nl.Uint16Attr(uint16(*sport))))
	req.AddData(nl.NewRtAttr(l2tpAttrUDPDPort, nl.Uint16Attr(uint16(*dport))))
	req.AddData(nl.NewRtAttr(l2tpAttrIPSAddr, saddr[:]))
	req.AddData(nl.NewRtAttr(l2tpAttrIPDAddr, daddr[:]))

	raw := req.Serialize()
	say("netlink message (%d bytes):\n%s\n", len(raw), hex.Dump(raw))

	_, err = req.Execute(unix.NETLINK_GENERIC, 0)
	if err != nil {
		say("FAILED: %v\n", err)
		say("errno interpretation: ERANGE=%d EINVAL=%d EEXIST=%d ENOENT=%d EADDRNOTAVAIL=%d\n",
			unix.ERANGE, unix.EINVAL, unix.EEXIST, unix.ENOENT, unix.EADDRNOTAVAIL)
		os.Exit(1)
	}

	fmt.Println("SUCCESS: tunnel created")
	showTunnels(family.ID)
}

// showTunnels prints the kernel's own list of L2TP tunnels, read over the same
// generic netlink socket the create used.
//
// It does NOT fork `ip l2tp show tunnel`. This diagnostic exists to isolate the
// netlink library from the process around it, so shelling out to iproute2 would
// answer with a second implementation's view and would report nothing at all in
// a namespace or an appliance image that carries no iproute2. The dump is the
// same mechanism as the create, one command byte apart.
func showTunnels(familyID uint16) {
	req := nl.NewNetlinkRequest(int(familyID), unix.NLM_F_DUMP)
	var genlHdr [4]byte
	genlHdr[0] = l2tpCmdTunnelGet
	genlHdr[1] = genlL2TPVersion
	req.AddData((*genlBytes)(&genlHdr))

	msgs, err := req.Execute(unix.NETLINK_GENERIC, 0)
	if err != nil {
		say("note: L2TP_CMD_TUNNEL_GET dump failed: %v\n", err)
		return
	}
	say("kernel reports %d tunnel message(s):\n", len(msgs))
	for i, msg := range msgs {
		say("  [%d] %d bytes:\n%s", i, len(msg), hex.Dump(msg))
	}
}

type genlBytes [4]byte

func (g *genlBytes) Len() int          { return 4 }
func (g *genlBytes) Serialize() []byte { return g[:] }

// say prints one diagnostic line to stdout.
//
// It exists so this file states its output intent once. fmt.Printf is refused
// on a Ze path (ai/rules/performance.md), fmt.Fprintf to os.Stdout is the
// allowed CLI-output form, and a failed write to stdout is nothing a
// diagnostic can act on: the reader has already gone.
func say(format string, args ...any) {
	fmt.Fprintf(os.Stdout, format, args...) //nolint:errcheck // a failed stdout write is not actionable in a one-shot diagnostic
}

// parseIPv4 reads a dotted-quad into four bytes. A string that is not one
// leaves the address zero and says so, because a diagnostic that silently
// tunnels to 0.0.0.0 reports a kernel failure that is really a typo.
func parseIPv4(s string) [4]byte {
	var b [4]byte
	fields, err := fmt.Sscanf(s, "%d.%d.%d.%d", &b[0], &b[1], &b[2], &b[3])
	if err != nil || fields != 4 {
		say("FAILED: %q is not a dotted-quad IPv4 address\n", s)
		os.Exit(1)
	}
	return b
}
