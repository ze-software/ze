//go:build linux

// Design: docs/architecture/l2tp/bng-5-pppoe.md -- the flood proof
// (spec-pppoe-padr-replay-allocates-unbounded-sessions, AC-6).
// Related: register_pppoe_padr_flood_linux.go -- registers tunnelPPPoEFloodReplay.
// Related: tunnel_fixture_pppoe_linux.go -- tunnelPPPoEOpen, tunnelPPPoEDiscover,
// the raw AF_PACKET client this personality reuses.
// RFC: rfc/short/rfc2516.md -- PPPoE discovery and PADR admission; this file
// carries no independent enforcement, only a client proving the AC's.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// tunnelPPPoEFloodReplayCount is how many times the fixture resends ONE
// already-admitted PADR from the same MAC, inside the same one-second window
// the AC-Cookie's own timestamp bucket covers (cookie.go). This is the exact
// shape of the original defect: one captured, valid PADR, replayed.
const tunnelPPPoEFloodReplayCount = 10000

// tunnelPPPoEFloodOneSessionFDs is the descriptor cost of the ONE session
// this flood is entitled to create: one AF_PPPOX socket (kernel_linux.go,
// pppoeCreate) plus two /dev/ppp opens, channel and unit (devppp_linux.go,
// DevPPPSetup), both held by the session until it ends (server.go, handlePADR
// storing sess.PppoxFD and handing ChanFD/UnitFD to the PPP driver). AC-6
// bounds growth to "one session's worth", not to zero, because the very
// first replay in the flood IS a legitimate admission.
const tunnelPPPoEFloodOneSessionFDs = 3

// tunnelPPPoEFloodReplay proves AC-6: replaying one valid, already-admitted
// PADR 10000 times from one MAC inside one second must not grow the AC's
// open file descriptor count beyond one session's worth, and the AC must
// still answer a fresh PADI afterward. Before this spec, every replay past
// the first re-entered the allocation path (a fresh SID, AF_PPPOX socket,
// /dev/ppp channel and unit each time) because the PADR dedup branch only
// covered a session still in StateDiscovery; matchLiveCookie (session.go)
// now answers a replay of the SAME cookie in every live state, so none of
// the 10000 replays after the first should allocate anything at all.
func tunnelPPPoEFloodReplay(ctx context.Context, args []string) error {
	metricsPort, err := tunnelArgPort(args, 0)
	if err != nil {
		return err
	}
	var urlBuf textbuf.Buffer
	metricsURL := urlBuf.Str("http://").HostPortN("127.0.0.1", uint16(metricsPort)).Str("/metrics").String()

	interfaceName := os.Getenv("TEST_IFACE")
	if interfaceName == "" {
		interfaceName = tunnelPPPoEDefaultInterface
	}
	wire, err := tunnelPPPoEOpen(interfaceName)
	if err != nil {
		return err
	}
	defer wire.close()

	baseline, err := tunnelPPPoEProcessOpenFDs(ctx, metricsURL)
	if err != nil {
		return fmt.Errorf("read baseline process_open_fds: %w", err)
	}

	server, tags, err := tunnelPPPoEDiscover(ctx, wire, []byte{0x70, 0x70})
	if err != nil {
		return fmt.Errorf("no PADO received: %w", err)
	}
	cookie := tags[tunnelPPPoEACCookie]
	if cookie == nil {
		return errors.New("PADO missing AC-Cookie")
	}
	packet := tunnelPPPoEPacket(tunnelPPPoEPADR, cookie, []byte{0x70, 0x70}, "")
	_, sid, _, err := wire.exchange(ctx, server, packet, tunnelPPPoEPADS, 6)
	if err != nil {
		return fmt.Errorf("no PADS received for the initial session: %w", err)
	}
	if sid == 0 {
		return errors.New("initial PADR was refused (PADS session id 0)")
	}
	var tb textbuf.Buffer
	tb.Str("OK: initial session admitted, session-id=").Uint(uint64(sid)).Byte('\n')
	if err := tb.StdOut(); err != nil {
		return err
	}

	deadline := time.Now().Add(time.Second)
	for range tunnelPPPoEFloodReplayCount {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := wire.send(server, packet); err != nil {
			return fmt.Errorf("send replay: %w", err)
		}
	}
	if remaining := time.Until(deadline); remaining > 0 {
		// Give the AC time to drain the socket backlog before scoring it;
		// AC-6 bounds descriptor growth, not delivery latency, and every
		// replay in this loop carries the SAME cookie as the one already
		// admitted above, so RFC 2516's one-second AC-Cookie window
		// (cookie.go) is not what this wait is protecting -- it only lets
		// the AC's discoveryReader goroutine (subsystem.go) catch up.
		time.Sleep(remaining)
	}

	// A fresh PADR carrying the same cookie proves the AC is still
	// answering this session coherently after the flood, and its own reply
	// drains one straggler PADS the send loop above did not wait to read.
	_, confirmSID, _, err := wire.exchange(ctx, server, packet, tunnelPPPoEPADS, 6)
	if err != nil {
		return fmt.Errorf("no PADS received after the flood: %w", err)
	}
	if confirmSID != sid {
		return fmt.Errorf("session id after the flood = %d, want the original %d", confirmSID, sid)
	}

	afterFlood, err := tunnelPPPoEProcessOpenFDs(ctx, metricsURL)
	if err != nil {
		return fmt.Errorf("read post-flood process_open_fds: %w", err)
	}
	if delta := afterFlood - baseline; delta > tunnelPPPoEFloodOneSessionFDs {
		return fmt.Errorf("process_open_fds grew by %.0f after %d PADR replays from one MAC, want at most %d (one session's worth)",
			delta, tunnelPPPoEFloodReplayCount, tunnelPPPoEFloodOneSessionFDs)
	}
	tb.Reset().Str("OK: process_open_fds grew by at most one session's worth after ").
		Uint(uint64(tunnelPPPoEFloodReplayCount)).Str(" PADR replays\n")
	if err := tb.StdOut(); err != nil {
		return err
	}

	// The per-MAC cap bounds THIS MAC's own replay; it says nothing about
	// whether the AC's discovery path as a whole is still healthy. A fresh,
	// standalone PADI proves the AC kept answering after the flood rather
	// than wedging or leaking its way into an unresponsive state.
	if _, _, err := tunnelPPPoEDiscover(ctx, wire, []byte{0x71, 0x71}); err != nil {
		return fmt.Errorf("no PADO received for a fresh PADI after the flood: %w", err)
	}
	tb.Reset().Str("OK: the AC answered a new PADI after the flood\n")
	return tb.StdOut()
}

// tunnelPPPoEProcessOpenFDs reads the standard client_golang process
// collector's process_open_fds gauge from the AC's own Prometheus endpoint
// (internal/core/metrics/prometheus.go, NewProcessCollector). It polls
// (Poll, fixture.go) rather than reading once: HTTP checks in the .ci file
// run only after every cmd= directive has finished (ci-format.md, "HTTP
// Checks"), so this fixture's own baseline read is the only thing that can
// run before the server has necessarily finished starting its listener.
func tunnelPPPoEProcessOpenFDs(ctx context.Context, metricsURL string) (float64, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	var value float64
	var lastErr error
	ok := Poll(ctx, 20, 250*time.Millisecond, func() bool {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, metricsURL, http.NoBody)
		if err != nil {
			lastErr = err
			return false
		}
		response, err := client.Do(request)
		if err != nil {
			lastErr = err
			return false
		}
		defer response.Body.Close() //nolint:errcheck // the body is read to completion below
		raw, err := io.ReadAll(response.Body)
		if err != nil {
			lastErr = err
			return false
		}
		for line := range strings.SplitSeq(string(raw), "\n") {
			if !strings.HasPrefix(line, "process_open_fds ") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			parsed, parseErr := strconv.ParseFloat(fields[1], 64)
			if parseErr != nil {
				lastErr = parseErr
				return false
			}
			value = parsed
			return true
		}
		lastErr = errors.New("process_open_fds not found in metrics output")
		return false
	})
	if !ok {
		if lastErr != nil {
			return 0, lastErr
		}
		return 0, errors.New("process_open_fds not available")
	}
	return value, nil
}
