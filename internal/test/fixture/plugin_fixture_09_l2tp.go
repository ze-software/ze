package fixture

import (
	"context"
	"crypto/md5" //nolint:gosec // L2TP challenge response requires MD5
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func l2tpAVP09(mandatory bool, attribute uint16, value []byte) []byte {
	out := make([]byte, 6+len(value))
	word := uint16(len(out))
	if mandatory {
		word |= 0x8000
	}
	binary.BigEndian.PutUint16(out, word)
	binary.BigEndian.PutUint16(out[4:], attribute)
	copy(out[6:], value)
	return out
}

func l2tpUint1609(value uint16) []byte {
	out := make([]byte, 2)
	binary.BigEndian.PutUint16(out, value)
	return out
}

func l2tpUint3209(value uint32) []byte {
	out := make([]byte, 4)
	binary.BigEndian.PutUint32(out, value)
	return out
}

func l2tpControl09(tid, sid, ns, nr uint16, avps ...[]byte) []byte {
	length := 12
	for _, avp := range avps {
		length += len(avp)
	}
	out := make([]byte, length)
	binary.BigEndian.PutUint16(out, 0xc802)
	binary.BigEndian.PutUint16(out[2:], uint16(length))
	binary.BigEndian.PutUint16(out[4:], tid)
	binary.BigEndian.PutUint16(out[6:], sid)
	binary.BigEndian.PutUint16(out[8:], ns)
	binary.BigEndian.PutUint16(out[10:], nr)
	offset := 12
	for _, avp := range avps {
		copy(out[offset:], avp)
		offset += len(avp)
	}
	return out
}

func l2tpParse09(packet []byte) map[uint16][]byte {
	out := make(map[uint16][]byte)
	if len(packet) < 12 {
		return out
	}
	length := int(binary.BigEndian.Uint16(packet[2:4]))
	if length > len(packet) {
		length = len(packet)
	}
	for offset := 12; offset+6 <= length; {
		word := binary.BigEndian.Uint16(packet[offset:])
		span := int(word & 0x03ff)
		if span < 6 || offset+span > length {
			break
		}
		if binary.BigEndian.Uint16(packet[offset+2:]) == 0 {
			attribute := binary.BigEndian.Uint16(packet[offset+4:])
			out[attribute] = append([]byte(nil), packet[offset+6:offset+span]...)
		}
		offset += span
	}
	return out
}

func l2tpPeer09(session bool) Driver {
	return func(ctx context.Context, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("L2TP peer requires <port> [tunnel-id] [session-id]")
		}
		port, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("L2TP port %q: %w", args[0], err)
		}
		peerTID, err := uint16Arg09(args, 1, 0x0123)
		if err != nil {
			return fmt.Errorf("L2TP tunnel id: %w", err)
		}
		peerSID, err := uint16Arg09(args, 2, 500)
		if err != nil {
			return fmt.Errorf("L2TP session id: %w", err)
		}
		target := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: port}
		conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
		if err != nil {
			return err
		}
		defer conn.Close()

		hostname := "py-peer"
		if session {
			hostname = fmt.Sprintf("py-peer-%d", peerTID)
		}
		secret := []byte(os.Getenv("TEST_SECRET"))
		exitPath := l2tpExitPath09()
		if err := os.Remove(exitPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove stale L2TP peer exit signal: %w", err)
		}
		defer os.Remove(exitPath) //nolint:errcheck // stale removal above handles interrupted runs
		sccrqAVPs := [][]byte{
			l2tpAVP09(true, 0, l2tpUint1609(1)),
			l2tpAVP09(true, 2, []byte{1, 0}),
			l2tpAVP09(true, 3, l2tpUint3209(3)),
			l2tpAVP09(true, 4, l2tpUint3209(0)),
			l2tpAVP09(true, 7, []byte(hostname)),
			l2tpAVP09(true, 9, l2tpUint1609(peerTID)),
			l2tpAVP09(true, 10, l2tpUint1609(8)),
		}
		if len(secret) != 0 {
			sccrqAVPs = append(sccrqAVPs, l2tpAVP09(true, 11, []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}))
		}
		sccrq := l2tpControl09(0, 0, 0, 0, sccrqAVPs...)
		buffer := make([]byte, 1500)
		var sccrp map[uint16][]byte
		for range 40 {
			if err := ctx.Err(); err != nil {
				return err
			}
			_ = conn.SetDeadline(time.Now().Add(250 * time.Millisecond))
			if _, err := conn.WriteToUDP(sccrq, target); err != nil {
				continue
			}
			n, _, err := conn.ReadFromUDP(buffer)
			if err == nil {
				sccrp = l2tpParse09(buffer[:n])
				if len(sccrp) != 0 {
					break
				}
			}
		}
		tidBytes := sccrp[9]
		if len(tidBytes) != 2 {
			return fmt.Errorf("L2TP SCCRP missing AssignedTID")
		}
		localTID := binary.BigEndian.Uint16(tidBytes)
		scccnAVPs := [][]byte{l2tpAVP09(true, 0, l2tpUint1609(3))}
		if len(secret) != 0 {
			challenge := sccrp[11]
			if len(challenge) == 0 {
				return fmt.Errorf("L2TP SCCRP missing Challenge")
			}
			sum := md5.Sum(append(append([]byte{3}, secret...), challenge...))
			scccnAVPs = append(scccnAVPs, l2tpAVP09(true, 13, sum[:]))
		}
		_ = conn.SetDeadline(time.Now().Add(time.Second))
		if _, err := conn.WriteToUDP(l2tpControl09(localTID, 0, 1, 1, scccnAVPs...), target); err != nil {
			return err
		}
		_, _, _ = conn.ReadFromUDP(buffer)

		if session {
			icrq := l2tpControl09(localTID, 0, 2, 1,
				l2tpAVP09(true, 0, l2tpUint1609(10)),
				l2tpAVP09(true, 14, l2tpUint1609(peerSID)),
				l2tpAVP09(true, 15, l2tpUint3209(42)))
			var icrp map[uint16][]byte
			for range 40 {
				_ = conn.SetDeadline(time.Now().Add(250 * time.Millisecond))
				_, _ = conn.WriteToUDP(icrq, target)
				n, _, err := conn.ReadFromUDP(buffer)
				if err == nil {
					candidate := l2tpParse09(buffer[:n])
					if len(candidate[14]) == 2 {
						icrp = candidate
						break
					}
				}
			}
			if len(icrp[14]) != 2 {
				return fmt.Errorf("L2TP ICRP missing AssignedSID")
			}
			zeSID := binary.BigEndian.Uint16(icrp[14])
			_ = conn.SetDeadline(time.Now().Add(250 * time.Millisecond))
			_, _, _ = conn.ReadFromUDP(buffer)
			_ = conn.SetDeadline(time.Now().Add(time.Second))
			iccn := l2tpControl09(localTID, zeSID, 3, 2,
				l2tpAVP09(true, 0, l2tpUint1609(12)),
				l2tpAVP09(true, 24, l2tpUint3209(10_000_000)),
				l2tpAVP09(true, 19, l2tpUint3209(2)))
			_, err = conn.WriteToUDP(iccn, target)
			if err != nil {
				return err
			}
			_ = conn.SetDeadline(time.Now().Add(time.Second))
			_, _, _ = conn.ReadFromUDP(buffer)
			fmt.Fprintf(os.Stdout, "OK[%d]: session established local_sid=%d remote_sid=%d\n", peerTID, peerSID, zeSID)
		} else {
			fmt.Fprintf(os.Stdout, "OK: tunnel established local_tid=%d remote_tid=%d\n", peerTID, localTID)
		}

		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(exitPath); err == nil {
				return nil
			}
			if ctx.Err() != nil {
				return nil
			}
			_ = conn.SetDeadline(time.Now().Add(200 * time.Millisecond))
			_, _, _ = conn.ReadFromUDP(buffer)
		}
		return nil
	}
}

func l2tpExitPath09() string {
	return fmt.Sprintf("peer.exit-%x", sha256.Sum256([]byte(os.Getenv("TEST_SECRET"))))
}

func l2tpWriteExit09() error {
	return os.WriteFile(l2tpExitPath09(), []byte("ok"), 0o600)
}

func pollCommand09(ctx context.Context, p *sdk.Plugin, command string, attempts int, predicate func(commandResult09) bool) commandResult09 {
	var result commandResult09
	Poll(ctx, attempts, 100*time.Millisecond, func() bool {
		result = dispatch09(ctx, p, command)
		return predicate(result)
	})
	return result
}

func waitL2TPBGPEOR09(ctx context.Context, p *sdk.Plugin) error {
	if !Poll(ctx, 60, 250*time.Millisecond, func() bool {
		return bgpEORSent09(dispatch09(ctx, p, "show bgp peer peer1 detail"), 1)
	}) {
		return fmt.Errorf("peer1 never sent its initial-sync End-of-Rib")
	}
	return nil
}

func l2tpConfigShow09(ctx context.Context, _ []string) error {
	return observe09(ctx, "l2tp-config-show-test", func(ctx context.Context, p *sdk.Plugin) error {
		if err := waitL2TPBGPEOR09(ctx, p); err != nil {
			return err
		}
		r := dispatch09(ctx, p, "show l2tp config")
		cfg := map09(r.data)
		if !done09(r) {
			return fmt.Errorf("show l2tp config: status=%s data=%s", r.status, r.text)
		}
		checks := map[string]any{"enabled": true, "max-tunnels": 50, "max-sessions": 75, "hello-interval": 45, "shared-secret": "<set>"}
		for key, want := range checks {
			got := cfg[key]
			if fmt.Sprint(got) != fmt.Sprint(want) {
				return fmt.Errorf("show l2tp config: %s=%v want %v", key, got, want)
			}
		}
		listeners := list09(cfg["listeners"])
		if len(listeners) != 1 || !strings.Contains(fmt.Sprint(listeners[0]), "127.0.0.1:") {
			return fmt.Errorf("show l2tp config listeners unexpected: %v", listeners)
		}
		return nil
	})
}

func l2tpEmptyShow09(ctx context.Context, _ []string) error {
	return observe09(ctx, "l2tp-empty-show-test", func(ctx context.Context, p *sdk.Plugin) error {
		r := dispatch09(ctx, p, "show l2tp")
		summary := map09(r.data)
		if !done09(r) || number09(summary["tunnel-count"]) != 0 || number09(summary["session-count"]) != 0 || number09(summary["listener-count"]) != 1 {
			return fmt.Errorf("show l2tp unexpected: status=%s data=%s", r.status, r.text)
		}
		listeners := dispatch09(ctx, p, "show l2tp listeners")
		rows := list09(listeners.data)
		if !done09(listeners) || len(rows) != 1 || map09(rows[0])["address"] != "127.0.0.1" {
			return fmt.Errorf("show l2tp listeners unexpected: %s", listeners.text)
		}
		config := dispatch09(ctx, p, "show l2tp config")
		secret := map09(config.data)["shared-secret"]
		if !done09(config) || (secret != "<set>" && secret != "<unset>") {
			return fmt.Errorf("show l2tp config shared-secret leaked: %v", secret)
		}
		missing := dispatch09(ctx, p, "show l2tp tunnel id 999")
		if missing.status != "error" || !strings.Contains(missing.text, "999") {
			return fmt.Errorf("show l2tp tunnel id 999: expected named error, status=%s data=%s", missing.status, missing.text)
		}
		return nil
	})
}

func l2tpHealthShow09(ctx context.Context, _ []string) error {
	return observe09(ctx, "l2tp-health-show-test", func(ctx context.Context, p *sdk.Plugin) error {
		if err := waitL2TPBGPEOR09(ctx, p); err != nil {
			return err
		}
		r := dispatch09(ctx, p, "show l2tp health")
		if done09(r) {
			data := map09(r.data)
			if _, logins := data["logins"]; !logins {
				return fmt.Errorf("l2tp-health: done but missing logins: %s", r.text)
			}
			if _, count := data["count"]; !count {
				return fmt.Errorf("l2tp-health: done but missing count: %s", r.text)
			}
			fmt.Fprintf(os.Stderr, "OK: l2tp health done count=%v\n", data["count"])
			return nil
		}
		if r.status == "error" && (strings.Contains(r.text, "observer not enabled") || strings.Contains(r.text, "CQM") || strings.Contains(r.text, "l2tp subsystem")) {
			fmt.Fprintln(os.Stderr, "OK: l2tp health reached handler (CQM/observer off in harness)")
			return nil
		}
		return fmt.Errorf("l2tp-health: did not reach handler: %s", r.text)
	})
}

func l2tpTunnelRows09(ctx context.Context, p *sdk.Plugin, command string, attempts int) ([]any, error) {
	r := pollCommand09(ctx, p, command, attempts, func(r commandResult09) bool { return done09(r) && len(list09(r.data)) > 0 })
	rows := list09(r.data)
	if len(rows) == 0 {
		return nil, fmt.Errorf("entry never appeared in %q", command)
	}
	return rows, nil
}

func l2tpTunnelsShow09(ctx context.Context, _ []string) error {
	return observe09(ctx, "l2tp-tunnels-show-test", func(ctx context.Context, p *sdk.Plugin) error {
		rows, err := l2tpTunnelRows09(ctx, p, "show l2tp tunnels", 100)
		if err != nil {
			return err
		}
		t := map09(rows[0])
		if t["peer-hostname"] != "py-peer" || t["state"] != "established" || number09(t["local-tid"]) == 0 || number09(t["remote-tid"]) != 0x0123 {
			return fmt.Errorf("unexpected tunnel: %v", t)
		}
		if err := l2tpWriteExit09(); err != nil {
			return fmt.Errorf("signal L2TP peer exit: %w", err)
		}
		return nil
	})
}

func l2tpTunnelDetailShow09(ctx context.Context, _ []string) error {
	return observe09(ctx, "l2tp-tunnel-detail-show-test", func(ctx context.Context, p *sdk.Plugin) error {
		rows, err := l2tpTunnelRows09(ctx, p, "show l2tp tunnels", 100)
		if err != nil {
			return err
		}
		localTID := number09(map09(rows[0])["local-tid"])
		r := dispatch09(ctx, p, fmt.Sprintf("show l2tp tunnel id %d", localTID))
		t := map09(r.data)
		if !done09(r) {
			return fmt.Errorf("show l2tp tunnel %d: %s", localTID, r.text)
		}
		for _, key := range []string{"peer-framing", "peer-bearer", "peer-recv-window", "sessions"} {
			if _, ok := t[key]; !ok {
				return fmt.Errorf("tunnel detail missing %q: %v", key, t)
			}
		}
		if _, ok := t["sessions"].([]any); !ok || t["peer-hostname"] != "py-peer" || number09(t["remote-tid"]) != 0x0123 {
			return fmt.Errorf("unexpected tunnel detail: %v", t)
		}
		if err := l2tpWriteExit09(); err != nil {
			return fmt.Errorf("signal L2TP peer exit: %w", err)
		}
		if !Poll(ctx, 20, 250*time.Millisecond, func() bool { return bgpEORSent09(dispatch09(ctx, p, "show bgp"), 1) }) {
			return fmt.Errorf("ze never sent its BGP End-of-Rib before shutdown")
		}
		return nil
	})
}

func l2tpSessionsShow09(ctx context.Context, _ []string) error {
	return observe09(ctx, "l2tp-sessions-show-test", func(ctx context.Context, p *sdk.Plugin) error {
		if !Poll(ctx, 60, 250*time.Millisecond, func() bool { return bgpEORSent09(dispatch09(ctx, p, "show bgp peer peer1 detail"), 1) }) {
			return fmt.Errorf("peer1 never sent its initial-sync End-of-Rib")
		}
		rows, err := l2tpTunnelRows09(ctx, p, "show l2tp sessions", 150)
		if err != nil {
			return err
		}
		s := map09(rows[0])
		for _, key := range []string{"local-sid", "remote-sid", "tunnel-local-tid", "state"} {
			if _, ok := s[key]; !ok {
				return fmt.Errorf("session missing %q: %v", key, s)
			}
		}
		if number09(s["remote-sid"]) != 500 {
			return fmt.Errorf("session remote-sid=%v want 500", s["remote-sid"])
		}
		if err := l2tpWriteExit09(); err != nil {
			return fmt.Errorf("signal L2TP peer exit: %w", err)
		}
		return nil
	})
}

func l2tpSessionDetailShow09(ctx context.Context, _ []string) error {
	return observe09(ctx, "l2tp-session-detail-show-test", func(ctx context.Context, p *sdk.Plugin) error {
		rows, err := l2tpTunnelRows09(ctx, p, "show l2tp sessions", 150)
		if err != nil {
			return err
		}
		localSID := number09(map09(rows[0])["local-sid"])
		r := dispatch09(ctx, p, fmt.Sprintf("show l2tp session id %d", localSID))
		s := map09(r.data)
		if !done09(r) {
			return fmt.Errorf("show l2tp session %d: %s", localSID, r.text)
		}
		for _, key := range []string{"tx-connect-speed", "rx-connect-speed", "framing-type", "sequencing-required", "lns-mode", "kernel-setup-needed"} {
			if _, ok := s[key]; !ok {
				return fmt.Errorf("session detail missing %q: %v", key, s)
			}
		}
		tx := number09(s["tx-connect-speed"])
		if number09(s["remote-sid"]) != 500 || (tx != 0 && tx != 10_000_000) {
			return fmt.Errorf("unexpected session detail: %v", s)
		}
		if err := l2tpWriteExit09(); err != nil {
			return fmt.Errorf("signal L2TP peer exit: %w", err)
		}
		return nil
	})
}

func l2tpStatisticsShow09(ctx context.Context, _ []string) error {
	return observe09(ctx, "l2tp-statistics-show-test", func(ctx context.Context, p *sdk.Plugin) error {
		r := pollCommand09(ctx, p, "show l2tp statistics", 100, func(r commandResult09) bool { return done09(r) && number09(map09(r.data)["tunnels-active"]) >= 1 })
		stats := map09(r.data)
		if number09(stats["tunnels-active"]) != 1 {
			return fmt.Errorf("statistics tunnels-active=%v want 1", stats["tunnels-active"])
		}
		if _, ok := stats["sessions-active"]; !ok {
			return fmt.Errorf("statistics missing sessions-active: %v", stats)
		}
		if err := l2tpWriteExit09(); err != nil {
			return fmt.Errorf("signal L2TP peer exit: %w", err)
		}
		return nil
	})
}

func l2tpHistoryShow09(ctx context.Context, _ []string) error {
	return observe09(ctx, "l2tp-history-test", func(ctx context.Context, p *sdk.Plugin) error {
		if !Poll(ctx, 60, 250*time.Millisecond, func() bool { return bgpEORSent09(dispatch09(ctx, p, "show bgp peer peer1 detail"), 1) }) {
			return fmt.Errorf("peer1 never sent its initial-sync End-of-Rib")
		}
		sessions, err := l2tpTunnelRows09(ctx, p, "show l2tp sessions", 150)
		if err != nil {
			return err
		}
		tunnels, err := l2tpTunnelRows09(ctx, p, "show l2tp tunnels", 1)
		if err != nil {
			return err
		}
		localSID := number09(map09(sessions[0])["local-sid"])
		localTID := number09(map09(tunnels[0])["local-tid"])
		sh := dispatch09(ctx, p, fmt.Sprintf("show l2tp session history %d", localSID))
		if !done09(sh) {
			return fmt.Errorf("session history: %s", sh.text)
		}
		transitions, ok := map09(sh.data)["transitions"].([]any)
		if !ok {
			return fmt.Errorf("session history missing transitions: %s", sh.text)
		}
		traffic := dispatch09(ctx, p, fmt.Sprintf("show l2tp session traffic %d", localSID))
		if !done09(traffic) && !strings.Contains(traffic.text, "PPP interface") {
			return fmt.Errorf("session traffic did not reach handler: %s", traffic.text)
		}
		th := dispatch09(ctx, p, fmt.Sprintf("show l2tp tunnel history %d", localTID))
		if !done09(th) {
			return fmt.Errorf("tunnel history: %s", th.text)
		}
		tunnelTransitions, ok := map09(th.data)["transitions"].([]any)
		if !ok {
			return fmt.Errorf("tunnel history missing transitions: %s", th.text)
		}
		fmt.Fprintf(os.Stderr, "OK: session history(%d), session traffic, tunnel history(%d) dispatched\n", len(transitions), len(tunnelTransitions))
		if err := l2tpWriteExit09(); err != nil {
			return fmt.Errorf("signal L2TP peer exit: %w", err)
		}
		return nil
	})
}
