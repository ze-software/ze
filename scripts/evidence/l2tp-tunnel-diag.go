// Design: docs/research/l2tpv2-ze-integration.md -- kernel netlink diagnostic
//
// Standalone diagnostic: creates an L2TP kernel tunnel using the same
// vishvananda/netlink library and attribute encoding as Ze. Run inside
// a network namespace with connectivity to isolate library vs process issues.
//
// Usage (inside QEMU VM):
//   ip netns exec zens go run -buildvcs=false scripts/evidence/l2tp-tunnel-diag.go \
//       -local 172.30.0.1 -remote 172.30.0.2 -sport 1701 -dport 1702

package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/vishvananda/netlink"
	nl "github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

const (
	l2tpCmdTunnelCreate = 1
	genlL2TPVersion     = 1

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
	fmt.Printf("l2tp genl family ID: %d\n", family.ID)

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
	fmt.Printf("netlink message (%d bytes):\n%s\n", len(raw), hex.Dump(raw))

	_, err = req.Execute(unix.NETLINK_GENERIC, 0)
	if err != nil {
		fmt.Printf("FAILED: %v\n", err)
		fmt.Printf("errno interpretation: ERANGE=%d EINVAL=%d EEXIST=%d ENOENT=%d EADDRNOTAVAIL=%d\n",
			unix.ERANGE, unix.EINVAL, unix.EEXIST, unix.ENOENT, unix.EADDRNOTAVAIL)
		os.Exit(1)
	}

	fmt.Println("SUCCESS: tunnel created")
	cmd := exec.Command("ip", "l2tp", "show", "tunnel")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

type genlBytes [4]byte

func (g *genlBytes) Len() int        { return 4 }
func (g *genlBytes) Serialize() []byte { return g[:] }

func parseIPv4(s string) [4]byte {
	var b [4]byte
	fmt.Sscanf(s, "%d.%d.%d.%d", &b[0], &b[1], &b[2], &b[3])
	return b
}
