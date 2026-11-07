//go:build linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- LCP renegotiation in the network phase
// Related: register_pppoe_client_renegotiation_linux.go -- the registration of the driver below
// Related: internal/test/pppoeac/script_linux.go -- the scripted access concentrator this driver plays
// RFC 1661 Section 4.1 -- see rfc/short/rfc1661.md.

package fixture

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/pppoeac"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	// renegotiationClient is the pppoe-client the .ci configures, and the
	// name of the PPP device Ze creates for it.
	renegotiationClient = "zereneg0"
	// renegotiationAC is the far end of the .ci's netns-link veth pair.
	renegotiationAC = "zeac0"
	// renegotiationSID is the one PPPoE session the access concentrator
	// assigns. Accept answers discovery once, so a client that redialed
	// would never reach a second up state.
	renegotiationSID uint16 = 44
	// renegotiationUser and renegotiationSecret match the .ci's
	// authentication block; CHAP verifies the digest against them.
	renegotiationUser   = "subscriber"
	renegotiationSecret = "secret"
)

// renegotiationEpoch is what one LCP, CHAP and IPCP exchange establishes.
type renegotiationEpoch struct {
	mru         uint16
	magic       uint32
	lcpID       uint8
	chapID      uint8
	ipcpID      uint8
	local, peer netip.Addr // the client's address and the concentrator's
}

// renegotiationClientStatus is the part of one `show interface
// pppoe-client` object the scenario reads.
type renegotiationClientStatus struct {
	Name         string     `json:"name"`
	State        string     `json:"state"`
	PPPInterface string     `json:"ppp-interface"`
	SessionID    uint16     `json:"session-id"`
	LocalIP      netip.Addr `json:"local-ip"`
	PeerIP       netip.Addr `json:"peer-ip"`
}

// pppoeClientRenegotiation proves spec-home-cpe-12 through the daemon: the
// scripted concentrator brings the session up, then sends an LCP
// Configure-Request in the network phase. Ze must answer on the same
// session: its own Configure-Request and a Configure-Ack of the
// concentrator's (session.LCP checks the Ack is unmodified), a fresh CHAP
// Response with the configured secret, and IPCP to a new address. The
// operator then sees, through `show interface pppoe-client`, the new
// address on the same session id and PPP device, and the kernel holds the
// new address pair alone on the same ifindex with the new MTU.
//
// Accept runs once, and nothing else answers discovery, so a client that
// tore the session down and redialed could never reach the second up state.
func pppoeClientRenegotiation(ctx context.Context, plugin *sdk.Plugin) error {
	acCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	fd, peer, err := renegotiationOpenAC()
	if err != nil {
		return err
	}
	defer unix.Close(fd) //nolint:errcheck // The scenario's last read is done; a close error changes no verdict.

	// RFC 2516 Section 5.
	session, err := peer.Accept(acCtx)
	if err != nil {
		return fmt.Errorf("access concentrator: discovery: %w", err)
	}
	first := renegotiationEpoch{mru: 1450, magic: 0x10203040, lcpID: 70, chapID: 80, ipcpID: 90,
		local: netip.MustParseAddr("100.64.91.2"), peer: netip.MustParseAddr("100.64.91.1")}
	if err := renegotiationRun(acCtx, session, first, false); err != nil {
		return fmt.Errorf("access concentrator: first negotiation: %w", err)
	}
	before, err := renegotiationWaitUp(acCtx, plugin, first)
	if err != nil {
		return err
	}

	// RFC 1661 Section 4.1: the concentrator's Configure-Request arrives in
	// the network phase, which is the event under test.
	second := renegotiationEpoch{mru: 1400, magic: 0x50607080, lcpID: 71, chapID: 81, ipcpID: 91,
		local: netip.MustParseAddr("100.64.92.2"), peer: netip.MustParseAddr("100.64.92.1")}
	if err := renegotiationRun(acCtx, session, second, true); err != nil {
		return fmt.Errorf("access concentrator: renegotiation on session %d: %w", renegotiationSID, err)
	}
	after, err := renegotiationWaitUp(acCtx, plugin, second)
	if err != nil {
		return err
	}
	if after.Attrs().Index != before.Attrs().Index {
		return fmt.Errorf("renegotiation replaced PPP device %s: ifindex %d became %d",
			renegotiationClient, before.Attrs().Index, after.Attrs().Index)
	}

	var tb textbuf.Buffer
	tb.Str("OK: pppoe-client ").Str(renegotiationClient).Str(" renegotiated LCP, CHAP and IPCP on session ").
		Uint(uint64(renegotiationSID)).Str(": ").Str(first.local.String()).Str(" became ").
		Str(second.local.String()).Str(" on the same ifindex, MTU ").Uint(uint64(second.mru)).Str("\n").StdErr()
	return nil
}

// renegotiationOpenAC binds the concentrator's AF_PACKET socket to the far
// end of the veth pair. The caller MUST close the returned socket after its
// last read.
func renegotiationOpenAC() (int, *pppoeac.Peer, error) {
	link, err := netlink.LinkByName(renegotiationAC)
	if err != nil {
		return -1, nil, fmt.Errorf("find access concentrator link %s: %w", renegotiationAC, err)
	}
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, int(pppoeac.NetworkOrder(unix.ETH_P_ALL)))
	if err != nil {
		return -1, nil, fmt.Errorf("open access concentrator socket: %w", err)
	}
	addr := &unix.SockaddrLinklayer{Ifindex: link.Attrs().Index, Protocol: pppoeac.NetworkOrder(unix.ETH_P_ALL)}
	if err := unix.Bind(fd, addr); err != nil {
		return -1, nil, errors.Join(fmt.Errorf("bind access concentrator socket to %s: %w", renegotiationAC, err), unix.Close(fd))
	}
	if err := pppoe.SetRecvTimeout(fd, 100*time.Millisecond); err != nil {
		return -1, nil, errors.Join(fmt.Errorf("set access concentrator receive timeout: %w", err), unix.Close(fd))
	}
	var mac [pppoe.EthALen]byte
	copy(mac[:], link.Attrs().HardwareAddr)
	peer, err := pppoeac.New(fd, link.Attrs().Index, mac, renegotiationSID)
	if err != nil {
		return -1, nil, errors.Join(err, unix.Close(fd))
	}
	return fd, peer, nil
}

// renegotiationRun plays one epoch: LCP with CHAP-MD5 as the
// authentication protocol, the CHAP challenge, then IPCP. restart sends the
// concentrator's Configure-Request first, as a renegotiation does.
func renegotiationRun(ctx context.Context, session *pppoeac.Session, epoch renegotiationEpoch, restart bool) error {
	// Wire: MRU (type 1, length 4), Authentication-Protocol CHAP with MD5
	// (type 3, length 5, 0xc223, 5), Magic-Number (type 5, length 6).
	options := []byte{1, 4, 0, 0, 3, 5, 0xc2, 0x23, 5, 5, 6, 0, 0, 0, 0}
	binary.BigEndian.PutUint16(options[2:4], epoch.mru)
	binary.BigEndian.PutUint32(options[11:15], epoch.magic)
	// RFC 1661 Sections 4.1 and 5.1.
	if err := session.LCP(ctx, epoch.lcpID, options, restart); err != nil {
		return fmt.Errorf("LCP: %w", err)
	}
	challenge := []byte{1, 3, 5, epoch.chapID}
	// RFC 1994 Sections 4.1 and 4.2.
	if err := session.CHAP(ctx, epoch.chapID, challenge, renegotiationUser, renegotiationSecret); err != nil {
		return fmt.Errorf("CHAP: %w", err)
	}
	// RFC 1332 Section 3.3. IPCP takes the concentrator's address first.
	if err := session.IPCP(ctx, epoch.ipcpID, epoch.peer, epoch.local); err != nil {
		return fmt.Errorf("IPCP: %w", err)
	}
	return nil
}

// renegotiationWaitUp polls `show interface pppoe-client` until the client
// reports the epoch's addresses in state up on renegotiationSID, then reads
// the kernel: the PPP device carries the epoch's address pair alone and the
// epoch's MTU. It returns the device so the caller can compare ifindexes.
func renegotiationWaitUp(ctx context.Context, plugin *sdk.Plugin, epoch renegotiationEpoch) (netlink.Link, error) {
	var last string
	var clients []renegotiationClientStatus
	reported := Poll(ctx, 80, 250*time.Millisecond, func() bool {
		clients = nil
		if _, err := Dispatch(ctx, plugin, "show interface pppoe-client", &clients); err != nil {
			last = err.Error()
			return false
		}
		if len(clients) != 1 {
			return false
		}
		got := clients[0]
		if got.State != "up" {
			return false
		}
		if got.LocalIP != epoch.local {
			return false
		}
		return got.PeerIP == epoch.peer
	})
	if !reported {
		return nil, fmt.Errorf("show interface pppoe-client never reported %s up with local %s and peer %s: last error %q, answer %+v",
			renegotiationClient, epoch.local, epoch.peer, last, clients)
	}
	got := clients[0]
	if got.SessionID != renegotiationSID {
		return nil, fmt.Errorf("show interface pppoe-client: session-id %d, want %d", got.SessionID, renegotiationSID)
	}
	if got.PPPInterface != renegotiationClient {
		return nil, fmt.Errorf("show interface pppoe-client: ppp-interface %q, want %q", got.PPPInterface, renegotiationClient)
	}

	var link netlink.Link
	var state error
	installed := Poll(ctx, 80, 250*time.Millisecond, func() bool {
		link, state = renegotiationKernelState(epoch)
		return state == nil
	})
	if !installed {
		return nil, fmt.Errorf("PPP device %s after show reported %s: %w", renegotiationClient, epoch.local, state)
	}
	return link, nil
}

// renegotiationKernelState reads the PPP device: exactly one IPv4 address,
// the epoch's local address with the epoch's peer, and the epoch's MTU. One
// address is the point: it proves the previous epoch's pair is gone.
func renegotiationKernelState(epoch renegotiationEpoch) (netlink.Link, error) {
	link, err := netlink.LinkByName(renegotiationClient)
	if err != nil {
		return nil, fmt.Errorf("find %s: %w", renegotiationClient, err)
	}
	if link.Attrs().MTU != int(epoch.mru) {
		return nil, fmt.Errorf("%s MTU %d, want %d", renegotiationClient, link.Attrs().MTU, epoch.mru)
	}
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return nil, fmt.Errorf("list %s addresses: %w", renegotiationClient, err)
	}
	if len(addrs) != 1 {
		return nil, fmt.Errorf("%s holds %d IPv4 addresses %v, want only %s peer %s", renegotiationClient, len(addrs), addrs, epoch.local, epoch.peer)
	}
	local, ok := netip.AddrFromSlice(addrs[0].IP)
	if !ok {
		return nil, fmt.Errorf("%s address %v does not decode", renegotiationClient, addrs[0].IP)
	}
	if local.Unmap() != epoch.local {
		return nil, fmt.Errorf("%s local address %s, want %s", renegotiationClient, local.Unmap(), epoch.local)
	}
	if addrs[0].Peer == nil {
		return nil, fmt.Errorf("%s address %s has no peer, want %s", renegotiationClient, epoch.local, epoch.peer)
	}
	remote, ok := netip.AddrFromSlice(addrs[0].Peer.IP)
	if !ok {
		return nil, fmt.Errorf("%s peer address %v does not decode", renegotiationClient, addrs[0].Peer.IP)
	}
	if remote.Unmap() != epoch.peer {
		return nil, fmt.Errorf("%s peer address %s, want %s", renegotiationClient, remote.Unmap(), epoch.peer)
	}
	return link, nil
}
