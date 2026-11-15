// Design: docs/guide/pppoe.md -- server settings reach the live PPP session
// Integration: the live PPP driver fixture opens, polls and reads Linux sockets.
//go:build integration && linux

package pppoe

import (
	"log/slog"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestPPPoEStartSessionCarriesLCPRestartConfig admits a subscriber through the
// production PADR handler. Its unanswered LCP requests must use the operator's
// 150 ms / three-transmission budget, so losing either copy into StartSession
// fails the wire observation instead of merely comparing configuration structs.
func TestPPPoEStartSessionCarriesLCPRestartConfig(t *testing.T) {
	p := extractFromConfigText(t, "set pppoe interface eth0\nset pppoe lcp restart-timer 150\nset pppoe lcp max-configure 3\n")
	driver := ppp.NewProductionDriver(slog.Default(), mruCeilingBackend{})
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	channel, unit := socketPair(t), socketPair(t)
	key := CookieKey{}
	hardware, peer := [EthALen]byte{2, 0, 0, 0, 0, 1}, [EthALen]byte{2, 0, 0, 0, 0, 2}
	server := &InterfaceServer{
		ifName: "eth0", hwAddr: hardware, sessions: newSessionTable("eth0", 8), cookieKey: key, logger: slog.Default(), pppDriver: driver,
		sendFrameFn: func([]byte) {},
		pppoeCreateFn: func(string, uint16, [EthALen]byte) (int, error) {
			return unix.Open("/dev/null", unix.O_RDWR|unix.O_CLOEXEC, 0)
		},
		devPPPSetupFn: func(int) (int, int, int, error) { return channel[0], unit[0], 1, nil },
	}
	applyInterfaceConfig(server, &p.Interfaces[0], &p)
	server.handlePADR(&Packet{Code: CodePADR, SrcMAC: peer, Tags: []Tag{
		{Type: TagACCookie, Value: GenerateCookie(key, hardware[:], peer[:], nil)}, {Type: TagServiceName},
	}})
	deadline := time.Now().Add(2 * time.Second)
	requests := 0
	var first, previous time.Time
	var buf [2048]byte
	for time.Now().Before(deadline) {
		poll := []unix.PollFd{{Fd: int32(channel[1]), Events: unix.POLLIN}}
		n, err := unix.Poll(poll, 50)
		if err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			n, err := unix.Read(channel[1], buf[:])
			if err != nil {
				t.Fatal(err)
			}
			if n > 0 {
				proto, payload, _, err := ppp.ParseFrame(buf[:n])
				if err != nil {
					t.Fatal(err)
				}
				if proto != ppp.ProtoLCP {
					t.Fatalf("protocol %#x during LCP", proto)
				}
				packet, err := ppp.ParseLCPPacket(payload)
				if err != nil {
					t.Fatal(err)
				}
				if packet.Code != ppp.LCPConfigureRequest {
					t.Fatalf("LCP code = %d", packet.Code)
				}
				now := time.Now()
				if requests == 0 {
					first = now
				} else {
					if gap := now.Sub(previous); gap < 100*time.Millisecond {
						t.Fatalf("request gap = %s, want configured 150ms", gap)
					}
				}
				previous = now
				requests++
			}
		}
		select {
		case event := <-driver.EventsOut():
			if _, ok := event.(ppp.EventSessionDown); !ok {
				t.Fatalf("event = %T before silent-peer timeout", event)
			}
			if requests != 3 {
				t.Fatalf("Configure-Requests = %d, want exactly 3", requests)
			}
			if elapsed := time.Since(first); elapsed > time.Second {
				t.Fatalf("silent-peer budget = %s, want 450ms", elapsed)
			}
			return
		default:
			// Keep reading until the bounded silent-peer budget ends the session.
		}
	}
	t.Fatalf("configured budget did not end the session within two seconds; requests=%d", requests)
}
