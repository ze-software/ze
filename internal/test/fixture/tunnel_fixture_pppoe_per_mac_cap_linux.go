//go:build linux

// Design: docs/architecture/l2tp/bng-5-pppoe.md -- the per-MAC session cap
// (spec-pppoe-padr-replay-allocates-unbounded-sessions).
// Related: register_pppoe_per_mac_cap_linux.go -- registers tunnelPPPoEPerMACCap.
// Related: tunnel_fixture_pppoe_linux.go -- tunnelPPPoEOpen, tunnelPPPoEDiscover,
// the raw AF_PACKET client this personality reuses.
// RFC: rfc/short/rfc2516.md -- PPPoE discovery and PADR admission; this file
// carries no independent enforcement, only a client proving the AC's.

package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// tunnelPPPoECookieRolloverWait is longer than the one-second window
// GenerateCookie truncates its embedded timestamp to (cookie.go), so two
// PADI/PADO round trips separated by this wait carry two different AC-Cookies
// for the same MAC pair. Without the wait, two round trips inside the same
// second would carry an IDENTICAL cookie, and the second PADR would answer
// through the dedup branch (matchLiveCookie, server.go) instead of reaching
// the per-MAC cap this fixture means to exercise.
const tunnelPPPoECookieRolloverWait = 1100 * time.Millisecond

// tunnelPPPoEPerMACCap proves AC-1: an AC configured with max-sessions-per-mac
// 1 admits one session from a MAC and refuses a second, genuinely distinct
// session (its own PADI/PADO round trip, past the one-second cookie boundary
// so the two AC-Cookies differ) from the same MAC. RFC 2516 places no
// per-peer session limit (rfc/short/rfc2516.md), so the refusal is Ze's own
// resource policy, not a protocol objection: the PADS must carry
// TagACSystemError and session ID 0x0000 (server.go, admitPerMACCap).
func tunnelPPPoEPerMACCap(ctx context.Context, _ []string) error {
	interfaceName := os.Getenv("TEST_IFACE")
	if interfaceName == "" {
		interfaceName = tunnelPPPoEDefaultInterface
	}
	wire, err := tunnelPPPoEOpen(interfaceName)
	if err != nil {
		return err
	}
	defer wire.close()

	server, tags, err := tunnelPPPoEDiscover(ctx, wire, []byte{0x60, 0x60})
	if err != nil {
		return fmt.Errorf("no PADO received for the first session: %w", err)
	}
	cookie := tags[tunnelPPPoEACCookie]
	if cookie == nil {
		return errors.New("PADO missing AC-Cookie")
	}
	_, sid, _, err := wire.exchange(ctx, server,
		tunnelPPPoEPacket(tunnelPPPoEPADR, cookie, []byte{0x60, 0x60}, ""), tunnelPPPoEPADS, 6)
	if err != nil {
		return fmt.Errorf("no PADS received for the first session: %w", err)
	}
	if sid == 0 {
		return errors.New("first session was refused (PADS session id 0); the cap should admit one session per MAC")
	}
	var tb textbuf.Buffer
	tb.Str("OK: first session admitted, session-id=").Uint(uint64(sid)).Byte('\n')
	if err := tb.StdOut(); err != nil {
		return err
	}

	select {
	case <-time.After(tunnelPPPoECookieRolloverWait):
	case <-ctx.Done():
		return ctx.Err()
	}

	server, tags, err = tunnelPPPoEDiscover(ctx, wire, []byte{0x61, 0x61})
	if err != nil {
		return fmt.Errorf("no PADO received for the second session: %w", err)
	}
	secondCookie := tags[tunnelPPPoEACCookie]
	if secondCookie == nil {
		return errors.New("PADO missing AC-Cookie for the second session")
	}
	if bytes.Equal(secondCookie, cookie) {
		return errors.New("second AC-Cookie is identical to the first; the cookie rollover wait did not cross a second boundary")
	}

	_, badSID, badTags, err := wire.exchange(ctx, server,
		tunnelPPPoEPacket(tunnelPPPoEPADR, secondCookie, []byte{0x61, 0x61}, ""), tunnelPPPoEPADS, 6)
	if err != nil {
		return fmt.Errorf("no PADS received for the second session: %w", err)
	}
	if badSID != 0 {
		return fmt.Errorf("second session from the same MAC was admitted with session id %d, want refusal at the per-MAC cap", badSID)
	}
	if _, ok := badTags[tunnelPPPoEACSystemError]; !ok {
		return fmt.Errorf("PADS refusing the second session carries no AC-System-Error tag: %#v", badTags)
	}
	tb.Reset().Str("OK: second session from the same MAC refused at the per-MAC cap\n")
	return tb.StdOut()
}
