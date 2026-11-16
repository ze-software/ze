// Design: docs/architecture/route-selection.md -- ingress, selection and kernel evidence
// Related: sysrib_recursive_daemon_fixture.go -- common topology and kernel readers
// RFC: rfc/short/rfc4271.md -- Sections 9.1.2 and 9.1.2.1
package fixture

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	rfc4271NextHopLink      = "zesrnh"
	rfc4271NextHopConnected = "198.18.230.0/24"
	rfc4271NextHopSubject   = "198.19.150.0/24"
	rfc4271NextHopControl   = "198.19.151.0/24"
	rfc4271NextHopCoverA    = "10.250.1.0/24"
	rfc4271NextHopCoverB    = "10.250.2.0/24"
	rfc4271NextHopPeerA     = "127.0.0.2"
	rfc4271NextHopPeerB     = "127.0.0.3"
	rfc4271NextHopA         = "10.250.1.1"
	rfc4271NextHopB         = "10.250.2.1"
	rfc4271NextHopDirect    = "198.18.230.3"
)

// The subjects arrive only through real BGP sockets. Only the interior covers
// are installed through the SDK; no command fabricates received BGP RIB state.
func rfc4271NextHopKernelDriver(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("next-hop kernel fixture needs a placement and BGP listen address")
	}
	if err := importPlacementArgs(args[:1], 1); err != nil {
		return err
	}
	return Observe(ctx, "rfc4271-next-hop-test", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := sysribExecutionPlacement(ctx, plugin, "internal"); err != nil {
			return err
		}
		if err := sysribExecutionPluginPlacement(ctx, plugin, "bgp-rib", "bgp-rib", args[0]); err != nil {
			return err
		}
		if err := importCreateLink(ctx, rfc4271NextHopLink, "198.18.230.254/24"); err != nil {
			return err
		}
		// The carrier joins daemon shutdown before its cleanup driver removes
		// this device, so deletion cannot hide a stale Ze route.
		if err := waitRIBWinner(ctx, plugin, rfc4271NextHopConnected, "connected"); err != nil {
			return err
		}
		coverA := rpc.RouteInstallEntry{Protocol: protocolOSPF, AFI: 1, SAFI: 1, Prefix: rfc4271NextHopCoverA,
			NextHop: "198.18.230.1", Interface: rfc4271NextHopLink, OnLink: true, Metric: 10, RouteType: 1}
		coverB := coverA
		coverB.Prefix, coverB.NextHop, coverB.Metric = rfc4271NextHopCoverB, "198.18.230.2", 0
		if err := sysribInstall(ctx, plugin, coverB); err != nil {
			return err
		}
		if err := waitRIBWinner(ctx, plugin, coverB.Prefix, protocolOSPF); err != nil {
			return err
		}

		peerA, err := rfc4271NextHopConnect(ctx, plugin, args[1], rfc4271NextHopPeerA, 0x02020202)
		if err != nil {
			return err
		}
		defer peerA.Close() //nolint:errcheck // Socket cleanup follows the state assertions.
		peerB, err := rfc4271NextHopConnect(ctx, plugin, args[1], rfc4271NextHopPeerB, 0x03030303)
		if err != nil {
			return err
		}
		defer peerB.Close() //nolint:errcheck // Socket cleanup follows the state assertions.
		// RFC 4271 Sections 4.3 and 9.1.2: the unresolved path has the higher
		// LOCAL_PREF, so only admission before ranking can preserve the alternate.
		for _, update := range []struct {
			conn       net.Conn
			prefix     string
			nextHop    string
			preference uint32
		}{
			{peerA, rfc4271NextHopSubject, rfc4271NextHopA, 200},
			{peerB, rfc4271NextHopSubject, rfc4271NextHopB, 100},
			{peerB, rfc4271NextHopControl, rfc4271NextHopDirect, 100},
		} {
			if err := rfc4271NextHopUpdate(update.conn, update.prefix, update.nextHop, update.preference); err != nil {
				return err
			}
		}
		if err := rfc4271NextHopReceived(ctx, plugin); err != nil {
			return err
		}
		if err := rfc4271NextHopWinner(ctx, plugin, rfc4271NextHopSubject, rfc4271NextHopPeerB, rfc4271NextHopB, coverB.NextHop); err != nil {
			return fmt.Errorf("unresolved higher-LOCAL_PREF input shadowed the reachable zero-cost alternative: %w", err)
		}
		if err := rfc4271NextHopSurvivor(ctx, plugin); err != nil {
			return err
		}

		// Every later step changes only interior reachability. Neither socket
		// sends another UPDATE, reconnects, or reinjects a stored route.
		if err := sysribInstall(ctx, plugin, coverA); err != nil {
			return err
		}
		if err := rfc4271NextHopWinner(ctx, plugin, rfc4271NextHopSubject, rfc4271NextHopPeerA, rfc4271NextHopA, coverA.NextHop); err != nil {
			return err
		}
		if err := sysribRemove(ctx, plugin, coverA); err != nil {
			return err
		}
		if err := rfc4271NextHopWinner(ctx, plugin, rfc4271NextHopSubject, rfc4271NextHopPeerB, rfc4271NextHopB, coverB.NextHop); err != nil {
			return err
		}
		if err := sysribRemove(ctx, plugin, coverB); err != nil {
			return err
		}
		if err := rfc4271NextHopAbsent(ctx, plugin); err != nil {
			return err
		}
		if err := rfc4271NextHopReceived(ctx, plugin); err != nil {
			return fmt.Errorf("resolution loss discarded the received inputs: %w", err)
		}
		if err := rfc4271NextHopSurvivor(ctx, plugin); err != nil {
			return err
		}
		if err := sysribInstall(ctx, plugin, coverA); err != nil {
			return err
		}
		if err := rfc4271NextHopWinner(ctx, plugin, rfc4271NextHopSubject, rfc4271NextHopPeerA, rfc4271NextHopA, coverA.NextHop); err != nil {
			return fmt.Errorf("restored reachability did not recover retained input: %w", err)
		}
		if err := rfc4271NextHopSurvivor(ctx, plugin); err != nil {
			return err
		}
		return importSay("real " + args[0] + " BGP ingress excludes unresolved preference, replaces and removes the Loc-RIB and kernel winner on cover loss, restores retained input without UPDATEs, and preserves a directly reachable BGP control")
	})
}

func rfc4271NextHopConnect(ctx context.Context, plugin *sdk.Plugin, address, peer string, identifier uint32) (net.Conn, error) {
	source, err := netip.ParseAddr(peer)
	if err != nil {
		return nil, err
	}
	dialer := net.Dialer{Timeout: 500 * time.Millisecond,
		LocalAddr: net.TCPAddrFromAddrPort(netip.AddrPortFrom(source, 0))}
	var conn net.Conn
	var last error
	if !Poll(ctx, 60, 100*time.Millisecond, func() bool {
		conn, last = dialer.DialContext(ctx, "tcp4", address)
		return last == nil
	}) {
		return nil, fmt.Errorf("BGP listener %s did not accept source %s: %w", address, peer, last)
	}
	failed := true
	defer func() {
		if failed {
			_ = conn.Close() //nolint:errcheck // A failed handshake owns and closes this socket.
		}
	}()
	if err := conn.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return nil, err
	}
	// RFC 4271 Sections 4.2 and 4.4: zero negotiated HoldTime keeps these
	// sockets established without a fixture keepalive worker or timed sleeps.
	open := &message.Open{Version: 4, MyAS: 65001, HoldTime: 0, BGPIdentifier: identifier}
	if _, err := conn.Write(message.PackTo(open, nil)); err != nil {
		return nil, err
	}
	if err := delayOpenFixtureRead(conn, 1); err != nil {
		return nil, fmt.Errorf("BGP source %s did not receive OPEN: %w", peer, err)
	}
	if err := delayOpenFixtureRead(conn, 4); err != nil {
		return nil, fmt.Errorf("BGP source %s did not receive KEEPALIVE: %w", peer, err)
	}
	if _, err := conn.Write(message.PackTo(&message.Keepalive{}, nil)); err != nil {
		return nil, err
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		rows, err := peerRows03(ctx, plugin, peer)
		if err != nil {
			return false
		}
		for _, row := range rows {
			if row["fsm-state"] == bgpStateEstablished {
				return true
			}
		}
		return false
	}) {
		return nil, fmt.Errorf("BGP source %s completed the wire handshake but never reached Established", peer)
	}
	failed = false
	return conn, nil
}

// rfc4271NextHopUpdate sends one actual two-octet-ASN iBGP UPDATE. Its bounded
// inputs are fixture constants: one /24, one IPv4 next hop and one preference.
func rfc4271NextHopUpdate(conn net.Conn, prefix, nextHop string, preference uint32) error {
	pfx, err := netip.ParsePrefix(prefix)
	if err != nil {
		return err
	}
	hop, err := netip.ParseAddr(nextHop)
	if err != nil {
		return err
	}
	if !pfx.Addr().Is4() {
		return errors.New("next-hop kernel fixture requires an IPv4 prefix")
	}
	if pfx.Bits() != 24 {
		return errors.New("next-hop kernel fixture requires a /24 prefix")
	}
	if !hop.Is4() {
		return errors.New("next-hop kernel fixture requires an IPv4 next hop")
	}
	nh, destination := hop.As4(), pfx.Addr().As4()
	// RFC 4271 Section 4.3: ORIGIN IGP, AS_SEQUENCE 65002, NEXT_HOP,
	// LOCAL_PREF, followed by the announced IPv4 /24.
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 4, 2, 1, 0xfd, 0xea,
		0x40, 3, 4, nh[0], nh[1], nh[2], nh[3], 0x40, 5, 4}
	attrs = binary.BigEndian.AppendUint32(attrs, preference)
	packet := make([]byte, message.HeaderLen)
	for index := range 16 {
		packet[index] = 0xff
	}
	packet[18] = 2
	packet = binary.BigEndian.AppendUint16(packet, 0)
	packet = binary.BigEndian.AppendUint16(packet, uint16(len(attrs)))
	packet = append(packet, attrs...)
	packet = append(packet, 24, destination[0], destination[1], destination[2])
	binary.BigEndian.PutUint16(packet[16:18], uint16(len(packet)))
	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	n, err := conn.Write(packet)
	if err != nil {
		return err
	}
	if n != len(packet) {
		return io.ErrShortWrite
	}
	return conn.SetWriteDeadline(time.Time{})
}

func rfc4271NextHopReceived(ctx context.Context, plugin *sdk.Plugin) error {
	var last string
	if Poll(ctx, importPollAttempts, importPollDelay, func() bool {
		data, err := requireDone02(ctx, plugin, "show bgp rib received")
		if err != nil {
			last = err.Error()
			return false
		}
		last = text02(data)
		var answer struct {
			Routes []struct{ Peer, Prefix string } `json:"routes"`
		}
		if err := decode02(data, &answer); err != nil {
			last = err.Error()
			return false
		}
		if len(answer.Routes) != 3 {
			return false
		}
		expected := map[string]bool{rfc4271NextHopPeerA + " " + rfc4271NextHopSubject: false,
			rfc4271NextHopPeerB + " " + rfc4271NextHopSubject: false,
			rfc4271NextHopPeerB + " " + rfc4271NextHopControl: false}
		for _, row := range answer.Routes {
			key := row.Peer + " " + row.Prefix
			if _, exists := expected[key]; !exists {
				return false
			}
			expected[key] = true
		}
		for _, found := range expected {
			if !found {
				return false
			}
		}
		return true
	}) {
		return nil
	}
	return fmt.Errorf("show bgp rib received did not retain all three real wire inputs: %s", last)
}

// An empty peer requests absence of a best path, never a fallback identity.
func rfc4271NextHopBest(ctx context.Context, plugin *sdk.Plugin, prefix, peer string) error {
	var last string
	if Poll(ctx, importPollAttempts, importPollDelay, func() bool {
		data, err := requireDone02(ctx, plugin, "show bgp rib best")
		if err != nil {
			last = err.Error()
			return false
		}
		last = text02(data)
		var answer struct {
			BestPath []struct {
				Prefix   string `json:"prefix"`
				BestPeer string `json:"best-peer"`
			} `json:"best-path"`
		}
		if err := decode02(data, &answer); err != nil {
			last = err.Error()
			return false
		}
		for _, row := range answer.BestPath {
			if row.Prefix == prefix {
				return row.BestPeer == peer
			}
		}
		return peer == ""
	}) {
		return nil
	}
	return fmt.Errorf("show bgp rib best: %s wanted peer %q, observed %s", prefix, peer, last)
}

func rfc4271NextHopWinner(ctx context.Context, plugin *sdk.Plugin, prefix, peer, nextHop, gateway string) error {
	if err := rfc4271NextHopBest(ctx, plugin, prefix, peer); err != nil {
		return err
	}
	if _, err := waitCandidates(ctx, plugin, prefix, func(row candidateRow) bool {
		path, found := candidatePathOf(row, protocolBGP)
		if !found {
			return false
		}
		if !path.Selected {
			return false
		}
		return path.NextHop == nextHop
	}); err != nil {
		return err
	}
	return importKernel(ctx, prefix, 1, "via "+gateway+" ", "dev "+rfc4271NextHopLink+" ", "proto 250 ")
}

func rfc4271NextHopAbsent(ctx context.Context, plugin *sdk.Plugin) error {
	if err := rfc4271NextHopBest(ctx, plugin, rfc4271NextHopSubject, ""); err != nil {
		return err
	}
	var last string
	if !Poll(ctx, importPollAttempts, importPollDelay, func() bool {
		data, err := requireDone02(ctx, plugin, "show rib candidates prefix "+rfc4271NextHopSubject)
		if err != nil {
			last = err.Error()
			return false
		}
		last = text02(data)
		var rows []candidateRow
		if err := decode02(data, &rows); err != nil {
			last = err.Error()
			return false
		}
		return len(rows) == 0
	}) {
		return fmt.Errorf("unresolved BGP subject remained in Loc-RIB candidates: %s", last)
	}
	return importKernel(ctx, rfc4271NextHopSubject, 0)
}

func rfc4271NextHopSurvivor(ctx context.Context, plugin *sdk.Plugin) error {
	return rfc4271NextHopWinner(ctx, plugin, rfc4271NextHopControl, rfc4271NextHopPeerB,
		rfc4271NextHopDirect, rfc4271NextHopDirect)
}

func rfc4271NextHopKernelEmpty(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("next-hop kernel-empty fixture takes no arguments")
	}
	for _, prefix := range []string{rfc4271NextHopSubject, rfc4271NextHopControl,
		rfc4271NextHopCoverA, rfc4271NextHopCoverB, rfc4271NextHopConnected} {
		if err := importKernel(ctx, prefix, 0); err != nil {
			return err
		}
	}
	return importSay("next-hop fixture prefixes are absent from every kernel table")
}

// This runs after daemon/await-exit. The fixture-owned device and OS route must
// survive while every Ze route is absent, before deleting that device is allowed.
func rfc4271NextHopLinkCleanup(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("next-hop post-stop link cleanup takes no arguments")
	}
	for _, prefix := range []string{rfc4271NextHopSubject, rfc4271NextHopControl, rfc4271NextHopCoverA, rfc4271NextHopCoverB} {
		if err := importKernel(ctx, prefix, 0); err != nil {
			return err
		}
	}
	if err := importKernel(ctx, rfc4271NextHopConnected, 1, "dev "+rfc4271NextHopLink+" ", "proto 2 "); err != nil {
		return err
	}
	if err := importCommandRun(ctx, "link", "show", "dev", rfc4271NextHopLink); err != nil {
		return err
	}
	if err := importDeleteLink(ctx, rfc4271NextHopLink); err != nil {
		return err
	}
	return importSay("joined daemon removed Ze next-hop routes while the fixture device and OS route survived before owned cleanup")
}
