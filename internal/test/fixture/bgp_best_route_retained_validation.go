// Design: docs/functional-tests.md -- GR-retained ordinary winners reach real sockets.
// Related: bgp_best_route_transit.go -- exact UPDATE and API fence helpers.
// Related: docs/guide/graceful-restart.md -- retention and purge lifecycle.
// Related: docs/guide/rpki.md -- live received-route validation.
package fixture

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	bgpRetainedSubject = "10.254.0.0/24"
	bgpRetainedRelease = "retained-sync.release"
	bgpRetainedVRPs    = "retained-vrps.txt"
)

func bgpRetainedDriver(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("retained validation requires <internal|forked> <BGP address> <purge|expiry>")
	}
	if args[0] != sysribPlacementInternal {
		if args[0] != sysribPlacementForked {
			return fmt.Errorf("retained validation: unknown placement %q", args[0])
		}
	}
	if args[2] != "purge" {
		if args[2] != "expiry" {
			return fmt.Errorf("retained validation: unknown retirement %q", args[2])
		}
	}
	return Observe(ctx, "bgp-retained-test", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := bgpRetainedScenario(ctx, plugin, args[1], args[2]); err != nil {
			return err
		}
		_, err := fmt.Fprintf(os.Stderr, "OK: retained ordinary %s %s actual RTR and final socket lifecycle\n", args[0], args[2])
		return err
	})
}

// bgpRetainedScenario asks one joined question: can a real retained received
// route change eligibility through the real RTR cache and ordinary deciding
// RIB, reach a newly established socket, and retire without resurrection?
// No recorder callback or injected learned candidate stands in for that path.
func bgpRetainedScenario(ctx context.Context, plugin *sdk.Plugin, address, retirement string) error {
	// The expiry variant waits for the real advertised protocol timer. The
	// purge variant keeps a long window so the successor's EOR owns removal.
	restart := uint16(120)
	if retirement == "expiry" {
		restart = 45
	}
	source, err := bgpRetainedConnect(ctx, address, restart, false)
	if err != nil {
		return err
	}
	defer source.Close() //nolint:errcheck // Scenario owns the useful result.
	if err := p12WaitPeersEOR(ctx, plugin, peerNameSource); err != nil {
		return err
	}
	if err := bgpRetainedEOR(source); err != nil {
		return err
	}
	if err := rfc8212ReloadTopology(ctx, plugin); err != nil {
		return err
	}
	nlri := []byte{24, 10, 254, 0}
	path := []byte{2, 2, 0xfd, 0xe8, 0xfd, 0xe9}
	if err := bgpBestRouteSend(source, nlri, false, false); err != nil {
		return err
	}
	original, err := bgpRetainedWaitRoute(ctx, plugin, bgpRetainedSubject, 2, true)
	if err != nil {
		return err
	}
	if err := original.valid(); err != nil {
		return err
	}
	if original.Peer != addrLoopbackSecond {
		return fmt.Errorf("original source address=%q", original.Peer)
	}
	// The engine widens the received two-octet AS_PATH before publishing
	// AttrsWire; this command exposes those canonical stored attributes.
	// Exact equality across later retained phases still uses this original.
	if original.Attributes != "4001010040020602010000fde9400304c0000201" {
		return fmt.Errorf("received attributes=%q", original.Attributes)
	}
	if original.NLRI != "180afe00" {
		return fmt.Errorf("received NLRI=%q", original.NLRI)
	}
	if original.NextHop != "c0000201" {
		return fmt.Errorf("received NEXT_HOP=%q", original.NextHop)
	}
	if err := rfc8212ReloadBest(ctx, plugin, bgpRetainedSubject, ""); err != nil {
		return err
	}
	if plugin15RPKICount(ctx, plugin) != 0 {
		return errors.New("initial RTR gate was not held")
	}
	if err := source.Close(); err != nil {
		return err
	}
	gr, err := bgpRetainedGRState(ctx, plugin, original)
	if err != nil {
		return err
	}
	if err := requireRestartTime(gr, int(restart)); err != nil {
		return err
	}
	if !Poll(ctx, 80, 100*time.Millisecond, func() bool { return llgrRouteLevel(ctx, plugin, bgpRetainedSubject, 1) }) {
		return errors.New("original received RIB route was not marked GR stale")
	}
	if err := bgpRetainedSame(ctx, plugin, original, 2, true); err != nil {
		return fmt.Errorf("after original physical DOWN: %w", err)
	}

	// This recipient did not exist while the source was live. Its initial EOR
	// and same-socket API fence distinguish absent ineligible replay from a
	// recipient which was never usable.
	target, err := atomicAggregatePolicyConnect(ctx, address, netip.MustParseAddr(addrLoopbackThird), 65002)
	if err != nil {
		return err
	}
	defer target.Close() //nolint:errcheck // Scenario owns the useful result.
	if err := p12WaitPeersEOR(ctx, plugin, "destination"); err != nil {
		return err
	}
	if err := bgpRetainedEOR(target); err != nil {
		return fmt.Errorf("ineligible retained replay: %w", err)
	}
	if err := bgpBestRouteAPI(ctx, plugin, target, "10.253.0.0/24", []byte{24, 10, 253, 0}, []byte{2, 1, 0xfd, 0xe8}); err != nil {
		return err
	}
	if err := os.WriteFile(bgpRetainedRelease, nil, 0o600); err != nil {
		return err
	}
	if err := bgpRetainedSerial(ctx, plugin, 1); err != nil {
		return err
	}
	if err := bgpRetainedSame(ctx, plugin, original, 1, false); err != nil {
		return err
	}
	if err := rfc8212ReloadBest(ctx, plugin, bgpRetainedSubject, addrLoopbackSecond); err != nil {
		return err
	}
	if err := bgpRetainedWinner(target, nlri, path); err != nil {
		return fmt.Errorf("retained winner after real initial cache sync: %w", err)
	}

	// A distinct live physical source remains useful through all subsequent
	// retained-source validation and cleanup operations.
	control, err := atomicAggregatePolicyConnect(ctx, address, netip.MustParseAddr(addrLoopbackFourth), 65001)
	if err != nil {
		return err
	}
	defer control.Close() //nolint:errcheck // Scenario owns the useful result.
	if err := p12WaitPeersEOR(ctx, plugin, "control"); err != nil {
		return err
	}
	if err := bgpRetainedEOR(control); err != nil {
		return err
	}
	controlNLRI := []byte{24, 10, 252, 0}
	if err := bgpBestRouteSend(control, controlNLRI, false, false); err != nil {
		return err
	}
	if _, err := bgpRetainedWaitRoute(ctx, plugin, "10.252.0.0/24", 1, false); err != nil {
		return err
	}
	if err := rfc8212ReloadBest(ctx, plugin, "10.252.0.0/24", addrLoopbackFourth); err != nil {
		return err
	}
	if err := bgpBestRouteExpect(target, controlNLRI, nil, path); err != nil {
		return err
	}
	if err := bgpRetainedCache(ctx, plugin, 2, 65999); err != nil {
		return err
	}
	if err := bgpRetainedSame(ctx, plugin, original, 3, true); err != nil {
		return err
	}
	if err := rfc8212ReloadBest(ctx, plugin, bgpRetainedSubject, ""); err != nil {
		return err
	}
	if err := bgpBestRouteExpect(target, nil, nlri, nil); err != nil {
		return fmt.Errorf("live ROA rejection of DOWN retained source: %w", err)
	}
	if err := bgpRetainedFence(ctx, plugin, target, 1); err != nil {
		return err
	}
	if err := bgpRetainedCache(ctx, plugin, 3, 65001); err != nil {
		return err
	}
	if err := bgpRetainedSame(ctx, plugin, original, 1, false); err != nil {
		return err
	}
	if err := rfc8212ReloadBest(ctx, plugin, bgpRetainedSubject, addrLoopbackSecond); err != nil {
		return err
	}
	if err := bgpRetainedWinner(target, nlri, path); err != nil {
		return fmt.Errorf("restored original retained winner: %w", err)
	}

	if retirement == "purge" {
		if err := bgpRetainedPurge(ctx, plugin, address, original); err != nil {
			return err
		}
	}
	// The expiry variant waits on authoritative absence, never on an elapsed
	// sleep. Its 45-second GR timer is the behavior this branch proves.
	if err := bgpRetainedGone(ctx, plugin); err != nil {
		return err
	}
	if err := rfc8212ReloadBest(ctx, plugin, bgpRetainedSubject, ""); err != nil {
		return err
	}
	if err := bgpBestRouteExpect(target, nil, nlri, nil); err != nil {
		return fmt.Errorf("retention retirement wire withdrawal: %w", err)
	}
	// Both new End-of-Data serials complete AFTER the acknowledged retirement.
	// The unchanged live control and same-socket fence make late resurrection
	// observable as an unexpected packet, not as a guessed quiet interval.
	if err := bgpRetainedCache(ctx, plugin, 4, 65999); err != nil {
		return err
	}
	if _, err := bgpRetainedWaitRoute(ctx, plugin, "10.252.0.0/24", 3, true); err != nil {
		return err
	}
	if err := rfc8212ReloadBest(ctx, plugin, "10.252.0.0/24", ""); err != nil {
		return err
	}
	if err := bgpBestRouteExpect(target, nil, controlNLRI, nil); err != nil {
		return fmt.Errorf("post-retirement cache callback control withdrawal: %w", err)
	}
	if err := bgpRetainedCache(ctx, plugin, 5, 65001); err != nil {
		return err
	}
	if _, err := bgpRetainedWaitRoute(ctx, plugin, "10.252.0.0/24", 1, false); err != nil {
		return err
	}
	if err := rfc8212ReloadBest(ctx, plugin, "10.252.0.0/24", addrLoopbackFourth); err != nil {
		return err
	}
	if err := bgpBestRouteExpect(target, controlNLRI, nil, path); err != nil {
		return fmt.Errorf("post-retirement cache callback control restoration: %w", err)
	}
	if err := bgpRetainedGone(ctx, plugin); err != nil {
		return err
	}
	if err := bgpRetainedFence(ctx, plugin, target, 2); err != nil {
		return err
	}

	// Reusing the address MUST create a fresh physical source, whose route is
	// useful after the old lease has been sealed. It must not borrow old facts.
	replacement, err := bgpRetainedConnect(ctx, address, 120, false)
	if err != nil {
		return err
	}
	defer replacement.Close() //nolint:errcheck // Scenario owns the useful result.
	if err := p12WaitPeersEOR(ctx, plugin, peerNameSource); err != nil {
		return err
	}
	if err := bgpRetainedEOR(replacement); err != nil {
		return fmt.Errorf("replacement source initial EOR: %w", err)
	}
	if err := bgpBestRouteSend(replacement, nlri, false, false); err != nil {
		return err
	}
	fresh, err := bgpRetainedWaitRoute(ctx, plugin, bgpRetainedSubject, 1, false)
	if err != nil {
		return err
	}
	if err := fresh.valid(); err != nil {
		return err
	}
	if fresh.Owner != original.Owner {
		return errors.New("reconnect unexpectedly replaced the configured Peer owner")
	}
	if fresh.Connection == original.Connection {
		return errors.New("replacement reused original physical connection")
	}
	if fresh.Receipt == original.Receipt {
		return errors.New("replacement reused original source Session receipt")
	}
	if fresh.Message == original.Message {
		return errors.New("replacement reused original UPDATE message")
	}
	if err := bgpRetainedWinner(target, nlri, path); err != nil {
		return fmt.Errorf("useful replacement: %w", err)
	}
	if err := bgpBestRouteSend(replacement, nlri, false, true); err != nil {
		return err
	}
	if err := rfc8212ReloadBest(ctx, plugin, bgpRetainedSubject, ""); err != nil {
		return err
	}
	if err := bgpBestRouteExpect(target, nil, nlri, nil); err != nil {
		return err
	}
	return bgpRetainedFence(ctx, plugin, target, 3)
}

// bgpRetainedPurge exercises RFC 4724 Section 4.2 through the successor's
// real OPEN and EOR. No replacement route refreshes the original retained path.
// This helper MUST close its empty successor before the later useful reconnect;
// its caller MUST retain the original snapshot for the post-retirement checks.
func bgpRetainedPurge(ctx context.Context, plugin *sdk.Plugin, address string, original bgpRetainedRoute) error {
	restarted, err := bgpRetainedConnect(ctx, address, 120, true)
	if err != nil {
		return err
	}
	defer restarted.Close() //nolint:errcheck // Every exit releases the test-owned socket.
	if err := bgpRetainedEOR(restarted); err != nil {
		return fmt.Errorf("restarted source initial EOR: %w", err)
	}
	if err := bgpRetainedSame(ctx, plugin, original, 1, false); err != nil {
		return fmt.Errorf("preserving the F-bit before successor EOR: %w", err)
	}
	// Empty reachable and withdrawn NLRI with no attributes is the IPv4
	// End-of-RIB marker (RFC 4724 Section 2), not a route withdrawal.
	if err := bgpBestRouteSend(restarted, nil, false, true); err != nil {
		return fmt.Errorf("restarted source EOR: %w", err)
	}
	// Prove EOR retired the original passive record while this successor
	// is still established. Its later DOWN cannot stand in for that cleanup.
	if err := bgpRetainedGone(ctx, plugin); err != nil {
		return fmt.Errorf("successor EOR purge: %w", err)
	}
	if err := restarted.Close(); err != nil {
		return err
	}
	// The source is configured passive. Active means its preceding Session
	// completed its DOWN and joins before this next accept cycle started.
	if err := p12WaitPeerField(ctx, plugin, p12Peer{name: peerNameSource, addr: original.Peer}, "state",
		func(value any) bool { return value == stateActive }, "empty GR successor to finish its DOWN"); err != nil {
		return err
	}
	return quiesce(ctx, plugin)
}

// bgpRetainedGRState reads the status key from the recorded source owner,
// never from its display address. The status row proves the GR timer; the
// following received-route comparison separately checks physical provenance.
func bgpRetainedGRState(ctx context.Context, plugin *sdk.Plugin, original bgpRetainedRoute) (map[string]any, error) {
	var row map[string]any
	result := pollCommand13(ctx, plugin, "show bgp rib status", grSenderFactsAttempts, grSenderFactsDelay,
		func(result commandResult13) bool {
			row = nil
			if !done13(result) {
				return false
			}
			states, ok := result.object()["gr-state"].(map[string]any)
			if !ok {
				return false
			}
			candidate, ok := states[original.Owner].(map[string]any)
			if !ok {
				return false
			}
			if candidate["peer-address"] != original.Peer {
				return false
			}
			// Retention first publishes an unmarked cycle. Wait for the
			// stale-marking transition before checking its exact timer.
			if candidate["stale-at"] == nil || candidate["stale-at"] == "0001-01-01T00:00:00Z" {
				return false
			}
			row = candidate
			return true
		})
	fmt.Fprintln(os.Stderr, "retained-validation: show bgp rib status =", result.status, string(result.raw))
	if err := requireDone13("show bgp rib status", result); err != nil {
		return nil, err
	}
	if row == nil {
		return nil, fmt.Errorf("no matching gr-state row observed for original owner %s at %s: %s", original.Owner, original.Peer, result.raw)
	}
	return row, nil
}

// bgpRetainedRoute is the existing revalidate command's immutable original
// received record, not a constructed forwarding grant. The B25 command emits
// large physical identities as decimal strings to preserve their exact bits.
type bgpRetainedRoute struct {
	Peer       string `json:"peer"`
	Owner      string `json:"peer-owner"`
	Connection string `json:"connection-id"`
	Receipt    string `json:"source-receipt"`
	Message    uint64 `json:"msg-id"`
	Attributes string `json:"attr-hex"`
	NextHop    string `json:"nhop-hex"`
	NLRI       string `json:"nlri-hex"`
	State      uint8  `json:"validation-state"`
	Ineligible bool   `json:"ineligible"`
}

func (r bgpRetainedRoute) valid() error {
	for name, value := range map[string]string{"owner": r.Owner, "connection": r.Connection, "receipt": r.Receipt} {
		n, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("original %s=%q: %w", name, value, err)
		}
		if n == 0 {
			return fmt.Errorf("original %s is unrecorded", name)
		}
	}
	if r.Message == 0 {
		return errors.New("original UPDATE message is unrecorded")
	}
	if r.Attributes == "" {
		return errors.New("original UPDATE attributes are empty")
	}
	return nil
}

func bgpRetainedRoutes(ctx context.Context, plugin *sdk.Plugin, prefix string) ([]bgpRetainedRoute, error) {
	raw, err := requireDone02(ctx, plugin, "request bgp adj-rib-in revalidate ipv4/unicast "+prefix)
	if err != nil {
		return nil, err
	}
	var answer struct {
		Routes []bgpRetainedRoute `json:"routes"`
	}
	if err := decode02(raw, &answer); err != nil {
		return nil, err
	}
	return answer.Routes, nil
}

func bgpRetainedWaitRoute(ctx context.Context, plugin *sdk.Plugin, prefix string, state uint8, ineligible bool) (bgpRetainedRoute, error) {
	var route bgpRetainedRoute
	var last error
	if Poll(ctx, 80, 100*time.Millisecond, func() bool {
		rows, err := bgpRetainedRoutes(ctx, plugin, prefix)
		if err != nil {
			last = err
			return false
		}
		if len(rows) != 1 {
			last = fmt.Errorf("%s has %d original received rows", prefix, len(rows))
			return false
		}
		route = rows[0]
		if route.State != state {
			last = fmt.Errorf("%s state=%d want %d", prefix, route.State, state)
			return false
		}
		if route.Ineligible != ineligible {
			last = fmt.Errorf("%s ineligible=%t want %t", prefix, route.Ineligible, ineligible)
			return false
		}
		return true
	}) {
		return route, nil
	}
	return route, fmt.Errorf("original received record: %w", last)
}

func bgpRetainedSame(ctx context.Context, plugin *sdk.Plugin, original bgpRetainedRoute, state uint8, ineligible bool) error {
	now, err := bgpRetainedWaitRoute(ctx, plugin, bgpRetainedSubject, state, ineligible)
	if err != nil {
		return err
	}
	now.State = original.State
	now.Ineligible = original.Ineligible
	if now != original {
		return fmt.Errorf("original source identity or bytes changed: before=%+v after=%+v", original, now)
	}
	return nil
}

func bgpRetainedGone(ctx context.Context, plugin *sdk.Plugin) error {
	var last error
	if Poll(ctx, 600, 100*time.Millisecond, func() bool {
		rows, err := bgpRetainedRoutes(ctx, plugin, bgpRetainedSubject)
		if err != nil {
			last = err
			return false
		}
		if len(rows) != 0 {
			last = fmt.Errorf("retired source still has %d received rows", len(rows))
			return false
		}
		return true
	}) {
		return nil
	}
	return fmt.Errorf("retained source retirement: %w", last)
}

func bgpRetainedSerial(ctx context.Context, plugin *sdk.Plugin, serial uint32) error {
	var last error
	if Poll(ctx, 80, 100*time.Millisecond, func() bool {
		raw, err := requireDone02(ctx, plugin, "show bgp rpki cache")
		if err != nil {
			last = err
			return false
		}
		var answer struct {
			Caches []struct {
				Serial uint32 `json:"serial"`
				Synced bool   `json:"synced"`
			} `json:"cache-servers"`
		}
		if err := decode02(raw, &answer); err != nil {
			last = err
			return false
		}
		if len(answer.Caches) != 1 {
			last = errors.New("expected one actual RTR cache")
			return false
		}
		if !answer.Caches[0].Synced {
			last = errors.New("RTR sync incomplete")
			return false
		}
		if answer.Caches[0].Serial != serial {
			last = fmt.Errorf("RTR serial=%d want %d", answer.Caches[0].Serial, serial)
			return false
		}
		return true
	}) {
		return nil
	}
	return fmt.Errorf("actual RTR serial publication: %w", last)
}

func bgpRetainedCache(ctx context.Context, plugin *sdk.Plugin, serial, subjectASN uint32) error {
	controlASN := uint32(65001)
	if serial == 4 {
		controlASN = 65999
	}
	text := fmt.Sprintf("%d\n10.254.0.0/24,24,%d\n10.252.0.0/24,24,%d\n", serial, subjectASN, controlASN)
	// Atomic rename prevents the concurrently polling mock from reading a
	// partial snapshot. Both names belong to this functional run's cwd.
	if err := os.WriteFile(bgpRetainedVRPs+".next", []byte(text), 0o600); err != nil {
		return err
	}
	if err := os.Rename(bgpRetainedVRPs+".next", bgpRetainedVRPs); err != nil {
		return err
	}
	return bgpRetainedSerial(ctx, plugin, serial)
}

func bgpRetainedFence(ctx context.Context, plugin *sdk.Plugin, target net.Conn, phase byte) error {
	if err := quiesce(ctx, plugin); err != nil {
		return err
	}
	if err := rfc8212ReloadBest(ctx, plugin, "10.252.0.0/24", addrLoopbackFourth); err != nil {
		return fmt.Errorf("unaffected physical control: %w", err)
	}
	return bgpBestRouteAPI(ctx, plugin, target, fmt.Sprintf("10.253.%d.0/24", phase), []byte{24, 10, 253, phase}, []byte{2, 1, 0xfd, 0xe8})
}

func bgpRetainedEOR(conn net.Conn) error {
	update, err := atomicAggregatePolicyRead(conn)
	if err != nil {
		return err
	}
	if len(update.NLRI) != 0 {
		return fmt.Errorf("initial EOR carried announcements %x", update.NLRI)
	}
	if len(update.WithdrawnRoutes) != 0 {
		return fmt.Errorf("initial EOR carried withdrawals %x", update.WithdrawnRoutes)
	}
	if len(update.PathAttributes) != 0 {
		return fmt.Errorf("initial EOR carried attributes %x", update.PathAttributes)
	}
	return nil
}

// bgpRetainedWinner requires actual useful bytes and conventional GR's stale
// treatment: GR must not decorate the original community-free route as LLGR.
func bgpRetainedWinner(conn net.Conn, nlri, path []byte) error {
	update, err := atomicAggregatePolicyRead(conn)
	if err != nil {
		return err
	}
	if !bytes.Equal(update.NLRI, nlri) {
		return fmt.Errorf("retained wire NLRI=%x want %x", update.NLRI, nlri)
	}
	if len(update.WithdrawnRoutes) != 0 {
		return fmt.Errorf("retained winner carried withdrawals %x", update.WithdrawnRoutes)
	}
	_, _, origin, found := attribute.AttrFind(update.PathAttributes, attribute.AttrOrigin)
	if !found {
		return errors.New("retained winner omitted ORIGIN")
	}
	if !bytes.Equal(origin, []byte{0}) {
		return fmt.Errorf("retained wire ORIGIN=%x", origin)
	}
	_, _, actual, found := attribute.AttrFind(update.PathAttributes, attribute.AttrASPath)
	if !found {
		return errors.New("retained winner omitted AS_PATH")
	}
	if !bytes.Equal(actual, path) {
		return fmt.Errorf("retained wire AS_PATH=%x want %x", actual, path)
	}
	_, _, _, communities := attribute.AttrFind(update.PathAttributes, attribute.AttrCommunity)
	if communities {
		return errors.New("conventional GR decorated the original community-free winner")
	}
	return nil
}

// bgpRetainedConnect adds only the source's real code-64 capability to the
// existing zero-hold-time handshake. Caller MUST close the returned socket;
// this helper MUST close it on every failed handshake.
func bgpRetainedConnect(ctx context.Context, address string, restart uint16, restarting bool) (net.Conn, error) {
	dialer := net.Dialer{Timeout: time.Second, LocalAddr: net.TCPAddrFromAddrPort(netip.AddrPortFrom(netip.MustParseAddr(addrLoopbackSecond), 0))}
	var conn net.Conn
	var dialErr error
	if !Poll(ctx, 60, 100*time.Millisecond, func() bool { conn, dialErr = dialer.DialContext(ctx, "tcp4", address); return dialErr == nil }) {
		return nil, fmt.Errorf("GR source listener: %w", dialErr)
	}
	failed := true
	defer func() {
		if failed {
			_ = conn.Close()
		}
	}() //nolint:errcheck // Failed handshake owns this socket.
	if err := conn.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return nil, err
	}
	// RFC 4724 Sections 3 and 4.1: a cold OPEN has R=0/F=0. The
	// restarting source sets R=1/F=1 for its preserved IPv4 forwarding state.
	restartFlags := byte(restart >> 8)
	forwardFlags := byte(0)
	if restarting {
		restartFlags |= 0x80
		forwardFlags = 0x80
	}
	optional := []byte{2, 8, 64, 6, restartFlags, byte(restart), 0, 1, 1, forwardFlags}
	open := &message.Open{Version: 4, MyAS: 65001, HoldTime: 0, BGPIdentifier: binary.BigEndian.Uint32([]byte{127, 0, 0, 2}), OptionalParams: optional}
	if _, err := conn.Write(message.PackTo(open, nil)); err != nil {
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
