// Design: docs/architecture/behavior/fsm.md — strict Event 20 on real daemon sockets.
package fixture

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/bgp-bfd-delay-open", bgpBFDDelayOpen)
}

// bgpBFDDelayOpen keeps BFD Down through the OPEN exchange, checks the public
// pending substate and absence of KEEPALIVE, then sends a real BFD Init packet.
// No test-only BGP/FSM hook supplies the release: the BFD engine and subscriber
// must turn that datagram into BfdUp before the session can establish.
func bgpBFDDelayOpen(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("bgp-bfd-delay-open requires the runner-assigned BGP listener address")
	}
	return Observe(ctx, "bgp-bfd-delay-open", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		detail, err := bfdSessionDetail(ctx, plugin, "127.0.0.2")
		if err != nil {
			return err
		}
		discriminator, ok := detail["local-discriminator"].(float64)
		if !ok || discriminator <= 0 || discriminator > 4294967295 || detail["state"] != "down" {
			return fmt.Errorf("initial BFD session is not Down with a local discriminator: %v", detail)
		}
		dialer := net.Dialer{LocalAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.2")}}
		conn, err := dialer.DialContext(ctx, "tcp4", args[0])
		if err != nil {
			return err
		}
		defer conn.Close() //nolint:errcheck // assertion errors are authoritative
		if err := conn.SetDeadline(time.Now().Add(12 * time.Second)); err != nil {
			return err
		}
		// Remote AS 65001, hold zero, identifier 2.2.2.2, capability 74/0.
		// The peer's zero selects BfdHoldTimer rather than the ordinary hold.
		body := []byte{4, 0xfd, 0xe9, 0, 0, 2, 2, 2, 2, 4, 2, 2, 74, 0}
		var frame [33]byte
		for i := range 16 {
			frame[i] = 0xff
		}
		frame[17], frame[18] = 33, 1
		copy(frame[19:], body)
		if _, err := conn.Write(frame[:]); err != nil {
			return err
		}
		if err := delayOpenFixtureRead(conn, 1); err != nil {
			return err
		}
		if err := bfdDelayOpenFixtureState(ctx, plugin, "ACTIVE", "ACTIVE-DELAYOPEN-BFD-UP-PENDING"); err != nil {
			return err
		}
		// A completed pending-state observation is the readiness barrier. This
		// bounded read measures forbidden output, never waits for readiness.
		if err := conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
			return err
		}
		var octet [1]byte
		n, readErr := conn.Read(octet[:])
		var timeout net.Error
		if n != 0 || !errors.As(readErr, &timeout) || !timeout.Timeout() {
			return fmt.Errorf("strict Event 20 sent before BFD Up: n=%d error=%v", n, readErr)
		}
		if err := conn.SetReadDeadline(time.Now().Add(12 * time.Second)); err != nil {
			return err
		}
		if err := bfdDelayOpenSendInit(ctx, uint32(discriminator)); err != nil {
			return err
		}
		if err := delayOpenFixtureRead(conn, 4); err != nil {
			return err
		}
		if err := bfdDelayOpenFixtureState(ctx, plugin, "OPENCONFIRM", ""); err != nil {
			return err
		}
		if _, err := conn.Write(message.PackTo(&message.Keepalive{}, nil)); err != nil {
			return err
		}
		if err := bfdDelayOpenFixtureState(ctx, plugin, "ESTABLISHED", ""); err != nil {
			return err
		}
		detail, err = bfdSessionDetail(ctx, plugin, "127.0.0.2")
		if err != nil {
			return err
		}
		if detail["state"] != "up" || detail["remote-discriminator"] != float64(0xda1a0001) {
			return fmt.Errorf("BFD wire release was not consumed: %v", detail)
		}
		var out textbuf.Buffer
		return out.Str("OK: strict DelayOpen withheld KEEPALIVE until real BFD Up and established\n").StdErr()
	})
}

func bfdDelayOpenFixtureState(ctx context.Context, plugin *sdk.Plugin, state, substate string) error {
	var last any
	if Poll(ctx, 60, 50*time.Millisecond, func() bool {
		rows, err := peerRows03(ctx, plugin, "delay")
		if err != nil {
			last = err
			return false
		}
		for _, row := range rows {
			last = row
			gotSubstate, _ := row["bfd-sub-state"].(string)
			if row["fsm-state"] == state && gotSubstate == substate {
				return true
			}
		}
		return false
	}) {
		return nil
	}
	return fmt.Errorf("strict DelayOpen wanted %s/%s, got %v", state, substate, last)
}

func bfdDelayOpenSendInit(ctx context.Context, discriminator uint32) error {
	// RFC 5881 fixes the destination, not a single source port: ask the OS
	// for an ephemeral port and retain only one in its required 49152..65535
	// source range. No literal source port can contend with a sibling runner.
	for range 256 {
		if err := ctx.Err(); err != nil {
			return err
		}
		conn, err := listenBFDUDP03("127.0.0.2:0")
		if err != nil {
			return err
		}
		if conn.LocalAddr().(*net.UDPAddr).Port < 49152 {
			if err := conn.Close(); err != nil {
				return err
			}
			continue
		}
		defer conn.Close() //nolint:errcheck // the datagram's write determines success
		// RFC 5880 Section 4.1: Init, no flags, 24 octets. A Down local
		// session receiving Init moves Up. Detection multiplier 255 with a
		// one-second interval keeps the real session alive throughout the
		// observer; no artificial timer or disabled detector is involved.
		var packet [24]byte
		packet[0], packet[1], packet[2], packet[3] = 1<<5, 2<<6, 255, 24
		binary.BigEndian.PutUint32(packet[4:8], 0xda1a0001)
		binary.BigEndian.PutUint32(packet[8:12], discriminator)
		binary.BigEndian.PutUint32(packet[12:16], 1_000_000)
		binary.BigEndian.PutUint32(packet[16:20], 1_000_000)
		_, err = conn.WriteToUDP(packet[:], &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 3784})
		return err
	}
	return errors.New("could not acquire an ephemeral BFD source port in 49152..65535")
}
