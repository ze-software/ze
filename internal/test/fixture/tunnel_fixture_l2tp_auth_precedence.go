// Design: docs/architecture/l2tp/subscriber-session-model.md -- prioritized subscriber auth handler selection.
// Related: register_l2tp_auth_precedence.go -- registers l2tp/auth-precedence.
// Related: tunnel_fixture_l2tp_ppp.go -- shared proxy-LCP tunnel, CHAP and IPCP peer.
// Related: tunnel_fixture_l2tp_radius.go -- tunnelRadiusAccounting mock and radius-rx.log.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	authPrecedenceModeRadius = "radius"
	authPrecedenceModeLocal  = "local"
)

// tunnelL2TPAuthPrecedence drives one real proxy-LCP CHAP-MD5 and IPCP L2TP
// session and asserts which backend authenticated it.
//
// Arguments: <l2tp-port> <radius|local>.
//   - radius: requires the mock RADIUS server's radius-rx.log to record an
//     Access-Request carrying User-Name=subscriber@ze.
//   - local: completes CHAP-MD5 and IPCP against l2tp-auth-local with no
//     RADIUS server configured.
func tunnelL2TPAuthPrecedence(ctx context.Context, args []string) error {
	port, err := tunnelArgPort(args, 0)
	if err != nil {
		return err
	}
	if len(args) != 2 {
		return errors.New("l2tp/auth-precedence requires <l2tp-port> <radius|local>")
	}
	mode := args[1]
	switch mode {
	case authPrecedenceModeRadius, authPrecedenceModeLocal:
	default:
		return fmt.Errorf("invalid mode %q; use radius or local", mode)
	}

	peer, err := tunnelL2TPOpenProxySession(ctx, port, 1500, 1500)
	if err != nil {
		return err
	}
	defer peer.conn.Close() //nolint:errcheck // fixture teardown

	if err := peer.negotiateIPCP(ctx); err != nil {
		return err
	}

	var line textbuf.Buffer
	if mode == authPrecedenceModeRadius {
		_, err = peer.waitRadius(ctx, "Access-Request for "+tunnelCHAPPeerName, 10*time.Second, func(entry string) bool {
			if !strings.HasPrefix(entry, "RADIUS-RX Access-Request ") {
				return false
			}
			user, found := tunnelRadiusValue(entry, "User-Name")
			return found && user == tunnelCHAPPeerName
		})
		if err != nil {
			return err
		}
		if err := line.Str("OK: RADIUS server received Access-Request for ").Str(tunnelCHAPPeerName).Byte('\n').StdOut(); err != nil {
			return err
		}
	} else {
		if err := line.Str("OK: local auth admitted CHAP session for ").Str(tunnelCHAPPeerName).Byte('\n').StdOut(); err != nil {
			return err
		}
	}

	stop := append(tunnelL2TPAVP(true, 0, tunnelL2TPU16(4)), tunnelL2TPAVP(true, 9, tunnelL2TPU16(0x0321))...)
	stop = append(stop, tunnelL2TPAVP(true, 1, append(tunnelL2TPU16(1), tunnelL2TPU16(0)...))...)
	peer.sendControl(stop, 0)
	return nil
}
