// Design: docs/guide/l2tp.md -- a RADIUS Framed-MTU lowers the IP MTU Ze
// installs on the subscriber's pppN, and `show l2tp sessions` reports it as
// "mtu" beside the LCP-negotiated "mru".
// Related: register_l2tp_framed_mtu.go -- registers this peer and its three
// RADIUS mocks.
// Related: tunnel_fixture_l2tp_ppp.go -- tunnelL2TPOpenProxySession and
// tunnelAccountingPeer, the LAC half of the session this peer drives.
// Related: tunnel_fixture_l2tp_session_clear.go -- sessionClearLink, which keeps
// the link answered while this peer reads the CLI, and sessionClearCLI.
// Related: tunnel_fixture_l2tp_proxy_mru.go -- tunnelProxyPPPLink, the kernel
// read of the pppN interface.
// Related: test/l2tp/radius-framed-mtu.ci, test/l2tp/radius-framed-mtu-absent.ci,
// test/l2tp/radius-framed-mtu-exceeds.ci -- the three carriers of this peer.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// radiusAttrFramedMTU is the RFC 2865 Section 5.12 Framed-MTU attribute,
// type 12, whose Value field is four octets.
const radiusAttrFramedMTU = 12

// framedMTULCPLimit is the PPP transmit limit of every session these carriers
// bring up: the LAC's proxy LCP carries MRU 1500 in both directions, and the
// carriers leave `l2tp mru` at its default of 1500. It is also the "mru" the
// session must keep, because LCP opened before RADIUS answered.
const framedMTULCPLimit = 1500

// tunnelL2TPFramedMTU is the LAC peer of the radius-framed-mtu carriers. It
// brings one CHAP-authenticated PPP session up to its IPCP address, keeps the
// link answered, waits until `show l2tp sessions` reports the session up with
// an installed MTU, and then judges that MTU and the MRU once. It then reads
// the kernel's pppN at the pool gateway and requires the same MTU there, so
// the display and the installed state are each proven.
//
// Arguments: <l2tp-port> <gateway> <want-mtu>. The gateway is the pool
// gateway of the carrier's config, the local address of its pppN; each carrier
// uses its own, so concurrent carriers never read each other's interface. The
// CLI takes the daemon's SSH address and credentials from the environment the
// runner gave this process.
func tunnelL2TPFramedMTU(ctx context.Context, args []string) error {
	port, err := tunnelArgPort(args, 0)
	if err != nil {
		return err
	}
	if len(args) != 3 {
		return errors.New("l2tp/radius-framed-mtu requires <l2tp-port> <gateway> <want-mtu>")
	}
	gateway, err := netip.ParseAddr(args[1])
	if err != nil {
		return fmt.Errorf("invalid gateway %q: %w", args[1], err)
	}
	if !gateway.Is4() {
		return fmt.Errorf("gateway %s is not an IPv4 address; use the carrier's pool gateway", gateway)
	}
	wantMTU, err := strconv.ParseUint(args[2], 10, 16)
	if err != nil {
		return fmt.Errorf("invalid want-mtu %q: %w", args[2], err)
	}
	// RFC 791 makes 68 the smallest MTU an IPv4 interface may carry.
	if wantMTU < 68 {
		return fmt.Errorf("want-mtu %d is below the IPv4 minimum of 68", wantMTU)
	}
	if wantMTU > framedMTULCPLimit {
		return fmt.Errorf("want-mtu %d exceeds the carriers' PPP transmit limit of %d", wantMTU, framedMTULCPLimit)
	}

	peer, err := tunnelL2TPOpenProxySession(ctx, port, framedMTULCPLimit, framedMTULCPLimit)
	if err != nil {
		return err
	}
	defer peer.conn.Close() //nolint:errcheck // fixture teardown

	if err := peer.negotiateIPCP(ctx); err != nil {
		return err
	}
	address, ok := netip.AddrFromSlice(peer.negotiatedAddr.To4())
	if !ok {
		return fmt.Errorf("IPCP settled on %s, which is not an IPv4 address", peer.negotiatedAddr)
	}

	// Deferred after the socket's close, so it runs first: stop MUST be
	// called before the peer's socket is closed.
	link := startSessionClearLink(ctx, peer)
	defer link.stop()

	if err := awaitFramedMTURow(ctx, link, peer.zeSID, address, int(wantMTU)); err != nil {
		return err
	}
	installed, err := tunnelProxyPPPLink(ctx, net.IP(gateway.AsSlice()), int(wantMTU))
	if err != nil {
		return err
	}
	if err := link.failure(); err != nil {
		return err
	}
	var line textbuf.Buffer
	return line.Str("OK: kernel interface ").Str(installed.Name).Str(" holding ").Str(gateway.String()).
		Str(" is up with MTU ").Int(int64(installed.MTU)).Byte('\n').StdOut()
}

// awaitFramedMTURow waits until `show l2tp sessions` lists the session with
// its address and a non-null "mtu", which the feature defines as the session
// being up after NCP, and then judges that one row: "mtu" must be wantMTU and
// "mru" the LCP limit. Waiting for the state and judging it once means a wrong
// MTU is reported as the value seen, not as a wait that never ended.
func awaitFramedMTURow(ctx context.Context, link *sessionClearLink, sid uint16, address netip.Addr, wantMTU int) error {
	what := "session " + sessionIDText(sid) + " with address " + address.String() + " and a non-null mtu"
	var row map[string]any
	err := link.awaitRow(ctx, what, sessionClearShowSessions, "local-sid", func(candidate map[string]any) bool {
		if candidate["local-sid"] != float64(sid) {
			return false
		}
		if candidate["assigned-addr"] != address.String() {
			return false
		}
		if candidate["mtu"] == nil {
			return false
		}
		row = candidate
		return true
	})
	if err != nil {
		return err
	}
	if row["mtu"] != float64(wantMTU) {
		return fmt.Errorf("`show l2tp sessions` reports mtu %v for session %d, want %d: %s",
			row["mtu"], sid, wantMTU, describeJSON(row))
	}
	if row["mru"] != float64(framedMTULCPLimit) {
		return fmt.Errorf("`show l2tp sessions` reports mru %v for session %d, want the LCP-negotiated %d, which RADIUS cannot change: %s",
			row["mru"], sid, framedMTULCPLimit, describeJSON(row))
	}
	var line textbuf.Buffer
	return line.Str("OK: show l2tp sessions reports session ").Uint16(sid).Str(" with mtu ").Int(int64(wantMTU)).
		Str(" and mru ").Int(framedMTULCPLimit).Byte('\n').StdOut()
}
