// Design: docs/architecture/meta/role.md -- only the selected OPEN owns learned role state.
// Related: plugin_fixture_14.go -- actual role export through adj-rib-in and rs.
// Related: atomic_aggregate_policy.go -- configured-listener socket and UPDATE helpers.
package fixture

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const roleIncomingSource = "127.0.0.2"
const roleIncomingDest = "127.0.0.3"

func init() {
	Register("plugin/role-incoming-open-isolation", roleIncomingOpenIsolation, StopsDaemon)
}

// roleIncomingOpenIsolation keeps both original sockets through every wire
// assertion. The source's explicit customer export set makes a competing
// destination OPEN observable as useful traffic, rather than a private map.
func roleIncomingOpenIsolation(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("role-incoming-open-isolation requires the runner's listener address")
	}
	return Observe(ctx, "role-incoming-watch", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := roleIncomingWaitActive(ctx, plugin); err != nil {
			return err
		}
		source, err := roleIncomingConnect(ctx, args[0], roleIncomingSource, 65001)
		if err != nil {
			return fmt.Errorf("establish source Customer: %w", err)
		}
		defer source.Close() //nolint:errcheck // Every assertion precedes release of this owned socket.
		dest, err := roleIncomingConnect(ctx, args[0], roleIncomingDest, 65002)
		if err != nil {
			return fmt.Errorf("establish destination Customer: %w", err)
		}
		defer dest.Close() //nolint:errcheck // Every assertion precedes release of this owned socket.
		if err := p12WaitPeersEOR(ctx, plugin, "source-peer", "dest-peer"); err != nil {
			return err
		}
		for _, conn := range []net.Conn{source, dest} {
			initial, err := atomicAggregatePolicyRead(conn)
			if err != nil {
				return fmt.Errorf("read initial End-of-RIB: %w", err)
			}
			if len(initial.NLRI)+len(initial.WithdrawnRoutes)+len(initial.PathAttributes) != 0 {
				return errors.New("initial sync was not an empty IPv4 End-of-RIB")
			}
		}
		if err := roleIncomingUseful(source, dest, 0); err != nil {
			return fmt.Errorf("baseline Customer export: %w", err)
		}
		for index, candidate := range []struct {
			name string
			role []byte
			code byte
			sub  byte
		}{
			{"mismatched Provider", []byte{0}, 2, 11},
			{"roleless", nil, 6, 7},
			{"valid Customer", []byte{3}, 6, 7},
		} {
			if err := roleIncomingCandidate(ctx, args[0], candidate.role, candidate.code, candidate.sub); err != nil {
				return fmt.Errorf("competing %s OPEN: %w", candidate.name, err)
			}
			if err := roleIncomingUseful(source, dest, byte(index+1)); err != nil {
				return fmt.Errorf("original Customer export after %s: %w", candidate.name, err)
			}
			if err := roleIncomingUnchanged(ctx, plugin); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "OK: competing %s OPEN received exact %d/%d and EOF; original Customer socket received fresh OTC-stamped route\n", candidate.name, candidate.code, candidate.sub)
		}
		return nil
	})
}

// OnAllPluginsReady freezes the plugin registry; its fanout runs concurrently
// with RebuildRunningPeers and StartPeers. Neither an early TCP listener nor a
// published Idle Session proves the configured passive peers started.
func roleIncomingWaitActive(ctx context.Context, plugin *sdk.Plugin) error {
	var last map[string]map[string]any
	var lastErr error
	if Poll(ctx, WaitAttempts(20, 50*time.Millisecond, 80), 50*time.Millisecond, func() bool {
		last, lastErr = peerRows03(ctx, plugin, "*")
		if lastErr != nil {
			return false
		}
		for _, address := range []string{roleIncomingSource, roleIncomingDest} {
			row := last[address]
			if row == nil || row["fsm-state"] != "ACTIVE" {
				return false
			}
		}
		return true
	}) {
		return nil
	}
	if lastErr != nil {
		return fmt.Errorf("configured passive peers did not reach ACTIVE: %w", lastErr)
	}
	return fmt.Errorf("configured passive peers did not reach ACTIVE before first dial: %v", last)
}

func roleIncomingDial(ctx context.Context, address, source string) (net.Conn, error) {
	dialer := net.Dialer{Timeout: WaitBudget(5, time.Second), LocalAddr: &net.TCPAddr{IP: net.ParseIP(source)}}
	conn, err := dialer.DialContext(ctx, "tcp4", address)
	if err != nil {
		return nil, err
	}
	if err := conn.SetDeadline(time.Now().Add(WaitBudget(20, 12*time.Second))); err != nil {
		return nil, errors.Join(err, conn.Close())
	}
	return conn, nil
}

func roleIncomingOpen(conn net.Conn, asn uint16, role []byte) error {
	open := &message.Open{Version: 4, MyAS: asn, HoldTime: 0, BGPIdentifier: 0x02020202}
	if role != nil {
		open.OptionalParams = []byte{2, 3, 9, 1, role[0]}
	}
	packet := message.PackTo(open, nil)
	n, err := conn.Write(packet)
	if err != nil {
		return err
	}
	if n != len(packet) {
		return io.ErrShortWrite
	}
	return nil
}

func roleIncomingConnect(ctx context.Context, address, source string, asn uint16) (net.Conn, error) {
	var conn net.Conn
	var err error
	if !Poll(ctx, WaitAttempts(60, 100*time.Millisecond, 30), 100*time.Millisecond, func() bool {
		conn, err = roleIncomingDial(ctx, address, source)
		// refuse-by-default: guards Poll -- failed listener readiness retries; the last error is returned below.
		return err == nil
	}) {
		return nil, fmt.Errorf("listener %s did not accept %s: %w", address, source, err)
	}
	failed := true
	defer func() {
		if failed {
			_ = conn.Close() //nolint:errcheck // Failed handshake owns this socket.
		}
	}()
	if err := roleIncomingOpen(conn, asn, []byte{3}); err != nil {
		return nil, err
	}
	if err := delayOpenFixtureRead(conn, 1); err != nil {
		return nil, err
	}
	if err := delayOpenFixtureRead(conn, 4); err != nil {
		return nil, err
	}
	if _, err := conn.Write(message.PackTo(&message.Keepalive{}, nil)); err != nil {
		return nil, err
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	failed = false
	return conn, nil
}

// EOF can precede the candidate reader's deferred admission-slot release.
// Later positive-policy controls retry only an exact transient capacity6/5;
// they still require an OPEN-admitted collision6/7 before proving preservation.
func roleIncomingCandidate(ctx context.Context, address string, role []byte, code, sub byte) error {
	var last error
	ready := Poll(ctx, WaitAttempts(30, 50*time.Millisecond, 10), 50*time.Millisecond, func() bool {
		got, err := roleIncomingRefusal(ctx, address, role)
		if err != nil {
			last = err
			return true // Stop polling a wire failure; return it below.
		}
		want := roleIncomingNotification(code, sub)
		if bytes.Equal(got, want) {
			last = nil
			return true
		}
		last = fmt.Errorf("received %x, want exact %x then EOF", got, want)
		if code == 6 && sub == 7 && bytes.Equal(got, roleIncomingNotification(6, 5)) {
			return false // Readiness only: the previous bounded candidate still owns its slot.
		}
		return true // An actual wrong policy/collision response is never retried.
	})
	if !ready {
		return fmt.Errorf("previous incoming slot never became available: %w", last)
	}
	return last
}

func roleIncomingRefusal(ctx context.Context, address string, role []byte) ([]byte, error) {
	conn, err := roleIncomingDial(ctx, address, roleIncomingDest)
	if err != nil {
		return nil, err
	}
	defer conn.Close() //nolint:errcheck // io.ReadAll below must observe EOF before release.
	if err := roleIncomingOpen(conn, 65002, role); err != nil {
		return nil, err
	}
	// With DelayOpen zero an admitted Session sends its local OPEN before
	// validating the remote OPEN. Capacity refusal occurs before Session.Accept
	// and is the only permitted response without that first OPEN.
	var header [message.HeaderLen]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return nil, err
	}
	hdr, err := message.ParseHeader(header[:])
	if err != nil {
		return nil, err
	}
	if err := hdr.ValidateLengthWithMax(false); err != nil {
		return nil, err
	}
	if header[18] != 1 {
		// At most the two NOTIFICATION body octets plus one forbidden extra
		// octet: exactly21 bytes therefore also proves the socket reached EOF.
		tail, err := io.ReadAll(io.LimitReader(conn, 3))
		if err != nil {
			return nil, err
		}
		packet := append(header[:], tail...)
		if !bytes.Equal(packet, roleIncomingNotification(6, 5)) {
			return nil, fmt.Errorf("response without local OPEN=%x, want only pre-Session capacity6/5 and EOF", packet)
		}
		return packet, nil
	}
	body := make([]byte, int(hdr.Length)-message.HeaderLen)
	if _, err := io.ReadFull(conn, body); err != nil {
		return nil, err
	}
	open, err := message.UnpackOpen(body)
	if err != nil {
		return nil, err
	}
	if open.Version != 4 || open.MyAS != 65000 || open.BGPIdentifier != 0x01020304 || open.Len(nil) != int(hdr.Length) {
		return nil, fmt.Errorf("invalid local OPEN before refusal: version=%d AS=%d identifier=%08x length=%d", open.Version, open.MyAS, open.BGPIdentifier, hdr.Length)
	}
	if err := open.ValidateHoldTime(); err != nil {
		return nil, err
	}
	if _, err := capability.ParseFromOptionalParams(open.OptionalParams, open.ExtendedParams); err != nil {
		return nil, fmt.Errorf("invalid local OPEN parameters: %w", err)
	}
	// Exactly one OPEN was consumed. A repeated OPEN, KEEPALIVE, UPDATE, wrong
	// notification or extra octet cannot match the caller's exact21-byte verdict.
	packet, err := io.ReadAll(io.LimitReader(conn, 22))
	if bytes.Equal(packet, roleIncomingNotification(6, 5)) {
		return nil, errors.New("capacity6/5 followed an admitted Session OPEN")
	}
	return packet, err
}

func roleIncomingNotification(code, sub byte) []byte {
	return []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0, 21, 3, code, sub}
}

func roleIncomingUseful(source, dest net.Conn, phase byte) error {
	prefix := []byte{24, 198, 51, phase}
	if err := atomicAggregatePolicySend(source, prefix, false, 10); err != nil {
		return err
	}
	update, err := atomicAggregatePolicyRead(dest)
	if err != nil {
		return fmt.Errorf("fresh route %s did not reach the original destination socket: %w", netip.PrefixFrom(netip.AddrFrom4([4]byte{198, 51, phase, 0}), 24), err)
	}
	if !bytes.Equal(update.NLRI, prefix) {
		return fmt.Errorf("original socket received NLRI %x, want %x", update.NLRI, prefix)
	}
	if len(update.WithdrawnRoutes) != 0 {
		return fmt.Errorf("original socket withdrew %x instead of useful export", update.WithdrawnRoutes)
	}
	// RFC 9234 Section 5 assigns OTC Attribute Type Code35.
	_, _, otc, found := attribute.AttrFind(update.PathAttributes, 35)
	if !found {
		return errors.New("customer export omitted OTC")
	}
	if !bytes.Equal(otc, binary.BigEndian.AppendUint32(nil, 65000)) {
		return fmt.Errorf("customer export OTC=%x, want ASN65000", otc)
	}
	return nil
}

func roleIncomingUnchanged(ctx context.Context, plugin *sdk.Plugin) error {
	for _, peer := range []string{"source-peer", "dest-peer"} {
		rows, err := peerRows03(ctx, plugin, peer)
		if err != nil {
			return err
		}
		if len(rows) != 1 {
			return fmt.Errorf("%s detail returned %d peers", peer, len(rows))
		}
		for _, row := range rows {
			if row["fsm-state"] != bgpStateEstablished {
				return fmt.Errorf("%s lost Established: %v", peer, row)
			}
			if plugin14Number(row["connections-established"]) != 1 || plugin14Number(row["connections-dropped"]) != 0 {
				return fmt.Errorf("%s reconnected or dropped: %v", peer, row)
			}
		}
	}
	return nil
}
