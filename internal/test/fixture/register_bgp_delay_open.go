// Design: docs/architecture/behavior/fsm.md — DelayOpen on a real daemon socket.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/bgp-delay-open", bgpDelayOpen)
}

// bgpDelayOpen proves both release conditions through the configured listener:
// an early remote OPEN, then a silent connection whose DelayOpenTimer expires.
// The observer MUST keep each socket until its state assertion has completed.
func bgpDelayOpen(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("bgp-delay-open requires the runner-assigned listener address")
	}
	return Observe(ctx, "bgp-delay-open", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := delayOpenFixtureExchange(ctx, plugin, args[0], false); err != nil {
			return err
		}
		// Wait for the first read loop to retire before opening a distinct
		// cycle; otherwise the new socket is a collision candidate.
		if err := delayOpenFixtureState(ctx, plugin, "ACTIVE"); err != nil {
			return err
		}
		if err := delayOpenFixtureExchange(ctx, plugin, args[0], true); err != nil {
			return err
		}
		var out textbuf.Buffer
		return out.Str("OK: DelayOpen held OPEN, remote OPEN and timer expiry both established\n").StdErr()
	})
}

func delayOpenFixtureExchange(ctx context.Context, plugin *sdk.Plugin, address string, expire bool) error {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("dial DelayOpen peer: %w", err)
	}
	defer conn.Close() //nolint:errcheck // the assertions carry the useful failure
	if err := conn.SetDeadline(time.Now().Add(12 * time.Second)); err != nil {
		return err
	}
	if expire {
		if err := delayOpenFixtureRead(conn, 1); err != nil {
			return fmt.Errorf("DelayOpen expiry OPEN: %w", err)
		}
		if err := delayOpenFixtureState(ctx, plugin, "OPENSENT"); err != nil {
			return err
		}
	} else {
		// This deadline measures the forbidden wire action, not readiness or
		// scheduling. The configured wait is five seconds, so any OPEN here
		// is the disabled/ignored leaf rather than an acceptable early expiry.
		if err := conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
			return err
		}
		var octet [1]byte
		n, readErr := conn.Read(octet[:])
		var timeout net.Error
		if n != 0 || !errors.As(readErr, &timeout) || !timeout.Timeout() {
			return fmt.Errorf("DelayOpen wrote before peer OPEN: bytes=%d error=%v", n, readErr)
		}
		if err := conn.SetReadDeadline(time.Now().Add(12 * time.Second)); err != nil {
			return err
		}
		if err := delayOpenFixtureState(ctx, plugin, "ACTIVE"); err != nil {
			return err
		}
	}
	open := &message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 0x02020202}
	if _, err := conn.Write(message.PackTo(open, nil)); err != nil {
		return err
	}
	if !expire {
		if err := delayOpenFixtureRead(conn, 1); err != nil {
			return fmt.Errorf("remote-triggered OPEN: %w", err)
		}
	}
	if err := delayOpenFixtureRead(conn, 4); err != nil {
		return fmt.Errorf("OPEN must be followed by KEEPALIVE: %w", err)
	}
	if err := delayOpenFixtureState(ctx, plugin, "OPENCONFIRM"); err != nil {
		return err
	}
	if _, err := conn.Write(message.PackTo(&message.Keepalive{}, nil)); err != nil {
		return err
	}
	return delayOpenFixtureState(ctx, plugin, "ESTABLISHED")
}

func delayOpenFixtureState(ctx context.Context, plugin *sdk.Plugin, want string) error {
	var last any
	var lastErr error
	// A static peer's first retry uses 3.75..5 seconds of backoff; its
	// connect-retry leaf does not shorten that wait.
	if Poll(ctx, 160, 50*time.Millisecond, func() bool {
		rows, err := peerRows03(ctx, plugin, "delay")
		lastErr = err
		if err != nil {
			return false
		}
		for _, row := range rows {
			last = row["fsm-state"]
			if last == want {
				return true
			}
		}
		return false
	}) {
		return nil
	}
	return fmt.Errorf("DelayOpen state=%v, want %s (query error %v)", last, want, lastErr)
}

func delayOpenFixtureRead(conn net.Conn, want byte) error {
	var header [message.HeaderLen]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return err
	}
	hdr, err := message.ParseHeader(header[:])
	if err != nil {
		return err
	}
	if err := hdr.ValidateLengthWithMax(false); err != nil {
		return err
	}
	if header[18] != want {
		return fmt.Errorf("BGP type=%d, want %d", header[18], want)
	}
	var body [message.MaxMsgLen]byte
	_, err = io.ReadFull(conn, body[:int(hdr.Length)-message.HeaderLen])
	return err
}
