package fixture

import (
	"context"
	"crypto/md5" //nolint:gosec // RFC 2661 Section 4.4.3 mandates the CHAP-style MD5 response.
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/teardown-tunnel-peer", plugin16L2TPTunnelPeer)
	Register("plugin/teardown-session-peer", plugin16L2TPSessionPeer)
	Register("plugin/teardown-tunnel", func(ctx context.Context, _ []string) error {
		return Observe(ctx, "teardown-tunnel-test", sdk.Registration{}, plugin16TeardownTunnel)
	})
	Register("plugin/teardown-tunnel-all", func(ctx context.Context, _ []string) error {
		return Observe(ctx, "teardown-tunnel-all-test", sdk.Registration{}, plugin16TeardownTunnelAll)
	})
	Register("plugin/teardown-session", func(ctx context.Context, _ []string) error {
		return Observe(ctx, "teardown-session-test", sdk.Registration{}, plugin16TeardownSession)
	})
	Register("plugin/teardown-session-all", func(ctx context.Context, _ []string) error {
		return Observe(ctx, "teardown-session-all-test", sdk.Registration{}, plugin16TeardownSessionAll)
	})
}

func plugin16L2TPArgs(args []string, withSession bool) (*net.UDPAddr, uint16, uint16, error) {
	if len(args) < 1 {
		return nil, 0, 0, fmt.Errorf("usage: fixture <port> [tunnel-id] [session-id]")
	}
	port, err := strconv.Atoi(args[0])
	if err != nil || port < 1 || port > 65535 {
		return nil, 0, 0, fmt.Errorf("invalid L2TP port %q", args[0])
	}
	tid := uint64(0x0123)
	if len(args) > 1 {
		tid, err = strconv.ParseUint(args[1], 0, 16)
		if err != nil || tid == 0 {
			return nil, 0, 0, fmt.Errorf("invalid tunnel id %q", args[1])
		}
	}
	sid := uint64(500)
	if withSession && len(args) > 2 {
		sid, err = strconv.ParseUint(args[2], 0, 16)
		if err != nil || sid == 0 {
			return nil, 0, 0, fmt.Errorf("invalid session id %q", args[2])
		}
	}
	return &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: port}, uint16(tid), uint16(sid), nil
}

func plugin16L2TPAVP(mandatory bool, attribute uint16, value []byte) []byte {
	length := 6 + len(value)
	word := uint16(length)
	if mandatory {
		word |= 0x8000
	}
	out := make([]byte, length)
	binary.BigEndian.PutUint16(out[0:2], word)
	binary.BigEndian.PutUint16(out[4:6], attribute)
	copy(out[6:], value)
	return out
}

func plugin16L2TPU16(value uint16) []byte {
	out := make([]byte, 2)
	binary.BigEndian.PutUint16(out, value)
	return out
}

func plugin16L2TPU32(value uint32) []byte {
	out := make([]byte, 4)
	binary.BigEndian.PutUint32(out, value)
	return out
}

func plugin16L2TPCtrl(tid, sid, ns, nr uint16, body []byte) []byte {
	out := make([]byte, 12+len(body))
	binary.BigEndian.PutUint16(out[0:2], 0xc802)
	binary.BigEndian.PutUint16(out[2:4], uint16(len(out)))
	binary.BigEndian.PutUint16(out[4:6], tid)
	binary.BigEndian.PutUint16(out[6:8], sid)
	binary.BigEndian.PutUint16(out[8:10], ns)
	binary.BigEndian.PutUint16(out[10:12], nr)
	copy(out[12:], body)
	return out
}

func plugin16L2TPParse(packet []byte) map[uint16][]byte {
	out := make(map[uint16][]byte)
	if len(packet) < 12 {
		return out
	}
	length := int(binary.BigEndian.Uint16(packet[2:4]))
	if length > len(packet) {
		length = len(packet)
	}
	for offset := 12; offset+6 <= length; {
		word := binary.BigEndian.Uint16(packet[offset : offset+2])
		avpLength := int(word & 0x03ff)
		if avpLength < 6 || offset+avpLength > length {
			break
		}
		vendor := binary.BigEndian.Uint16(packet[offset+2 : offset+4])
		attribute := binary.BigEndian.Uint16(packet[offset+4 : offset+6])
		if vendor == 0 {
			out[attribute] = append([]byte(nil), packet[offset+6:offset+avpLength]...)
		}
		offset += avpLength
	}
	return out
}

func plugin16L2TPRead(conn *net.UDPConn, timeout time.Duration) (map[uint16][]byte, error) {
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}
	buffer := make([]byte, 1500)
	n, _, err := conn.ReadFromUDP(buffer)
	if err != nil {
		return nil, err
	}
	return plugin16L2TPParse(buffer[:n]), nil
}

func plugin16L2TPSCCRQ(tid uint16, hostname string, secret []byte) []byte {
	body := make([]byte, 0, 128)
	body = append(body, plugin16L2TPAVP(true, 0, plugin16L2TPU16(1))...)
	body = append(body, plugin16L2TPAVP(true, 2, []byte{1, 0})...)
	body = append(body, plugin16L2TPAVP(true, 3, plugin16L2TPU32(3))...)
	body = append(body, plugin16L2TPAVP(true, 4, plugin16L2TPU32(0))...)
	body = append(body, plugin16L2TPAVP(true, 7, []byte(hostname))...)
	body = append(body, plugin16L2TPAVP(true, 9, plugin16L2TPU16(tid))...)
	body = append(body, plugin16L2TPAVP(true, 10, plugin16L2TPU16(8))...)
	if len(secret) != 0 {
		body = append(body, plugin16L2TPAVP(true, 11, []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff})...)
	}
	return plugin16L2TPCtrl(0, 0, 0, 0, body)
}

func plugin16L2TPSCCCN(tid uint16, secret, challenge []byte) []byte {
	body := plugin16L2TPAVP(true, 0, plugin16L2TPU16(3))
	if len(secret) != 0 {
		digestInput := append([]byte{3}, secret...)
		digestInput = append(digestInput, challenge...)
		digest := md5.Sum(digestInput) //nolint:gosec // RFC 2661 Section 4.4.3 fixes the wire algorithm.
		body = append(body, plugin16L2TPAVP(true, 13, digest[:])...)
	}
	return plugin16L2TPCtrl(tid, 0, 1, 1, body)
}

func plugin16L2TPHandshake(ctx context.Context, target *net.UDPAddr, tid uint16, hostname string) (*net.UDPConn, uint16, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		return nil, 0, err
	}
	secret := []byte(os.Getenv("TEST_SECRET"))
	var response map[uint16][]byte
	for range 40 {
		if _, err = conn.WriteToUDP(plugin16L2TPSCCRQ(tid, hostname, secret), target); err == nil {
			response, err = plugin16L2TPRead(conn, 250*time.Millisecond)
			if err == nil && len(response[9]) >= 2 {
				break
			}
		}
		select {
		case <-ctx.Done():
			conn.Close()
			return nil, 0, ctx.Err()
		default:
		}
	}
	if len(response[9]) < 2 {
		conn.Close()
		return nil, 0, fmt.Errorf("no SCCRP for tunnel %d", tid)
	}
	remoteTID := binary.BigEndian.Uint16(response[9])
	if len(secret) != 0 && len(response[11]) == 0 {
		conn.Close()
		return nil, 0, fmt.Errorf("SCCRP missing Challenge")
	}
	if _, err = conn.WriteToUDP(plugin16L2TPSCCCN(remoteTID, secret, response[11]), target); err != nil {
		conn.Close()
		return nil, 0, err
	}
	_, _ = plugin16L2TPRead(conn, time.Second)
	return conn, remoteTID, nil
}

func plugin16L2TPIdle(ctx context.Context, conn *net.UDPConn, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	buffer := make([]byte, 1500)
	for time.Now().Before(deadline) {
		if _, err := os.Stat("peer.exit"); err == nil {
			return nil
		}
		if err := conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
			return err
		}
		_, _, _ = conn.ReadFromUDP(buffer)
		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
	return nil
}

// Mass-teardown peers use 0x0111 and 0x0222 and retain the predecessors' longer lifetime.
func plugin16L2TPIdleTimeout(tid uint16) time.Duration {
	if tid == 0x0111 || tid == 0x0222 {
		return 20 * time.Second
	}
	return 15 * time.Second
}

func plugin16L2TPTunnelPeer(ctx context.Context, args []string) error {
	if err := os.Remove("peer.exit"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale peer exit marker: %w", err)
	}
	target, tid, _, err := plugin16L2TPArgs(args, false)
	if err != nil {
		return err
	}
	conn, remoteTID, err := plugin16L2TPHandshake(ctx, target, tid, "py-peer")
	if err != nil {
		return err
	}
	defer conn.Close()
	fmt.Printf("OK: tunnel established local_tid=%d remote_tid=%d\n", tid, remoteTID)
	return plugin16L2TPIdle(ctx, conn, plugin16L2TPIdleTimeout(tid))
}

func plugin16L2TPSessionPeer(ctx context.Context, args []string) error {
	if err := os.Remove("peer.exit"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale peer exit marker: %w", err)
	}
	target, tid, sid, err := plugin16L2TPArgs(args, true)
	if err != nil {
		return err
	}
	conn, remoteTID, err := plugin16L2TPHandshake(ctx, target, tid, fmt.Sprintf("py-peer-%d", tid))
	if err != nil {
		return err
	}
	defer conn.Close()
	body := plugin16L2TPAVP(true, 0, plugin16L2TPU16(10))
	body = append(body, plugin16L2TPAVP(true, 14, plugin16L2TPU16(sid))...)
	body = append(body, plugin16L2TPAVP(true, 15, plugin16L2TPU32(42))...)
	icrq := plugin16L2TPCtrl(remoteTID, 0, 2, 1, body)
	var response map[uint16][]byte
	for range 40 {
		if _, err = conn.WriteToUDP(icrq, target); err == nil {
			response, err = plugin16L2TPRead(conn, 250*time.Millisecond)
			if err == nil && len(response[14]) >= 2 {
				break
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	if len(response[14]) < 2 {
		return fmt.Errorf("no ICRP for tunnel %d", tid)
	}
	remoteSID := binary.BigEndian.Uint16(response[14])
	_, _ = plugin16L2TPRead(conn, 250*time.Millisecond)
	body = plugin16L2TPAVP(true, 0, plugin16L2TPU16(12))
	body = append(body, plugin16L2TPAVP(true, 24, plugin16L2TPU32(10_000_000))...)
	body = append(body, plugin16L2TPAVP(true, 19, plugin16L2TPU32(2))...)
	if _, err = conn.WriteToUDP(plugin16L2TPCtrl(remoteTID, remoteSID, 3, 2, body), target); err != nil {
		return err
	}
	_, _ = plugin16L2TPRead(conn, 250*time.Millisecond)
	fmt.Printf("OK[%d]: session established local_sid=%d remote_sid=%d\n", tid, sid, remoteSID)
	return plugin16L2TPIdle(ctx, conn, plugin16L2TPIdleTimeout(tid))
}

func plugin16L2TPList(ctx context.Context, p *sdk.Plugin, command string) ([]map[string]any, bool) {
	r := plugin16Dispatch(ctx, p, command)
	if r.status != "done" || r.err != nil {
		return nil, false
	}
	values, ok := r.data.([]any)
	if !ok {
		return nil, false
	}
	rows := make([]map[string]any, 0, len(values))
	for _, value := range values {
		row, ok := value.(map[string]any)
		if !ok {
			return nil, false
		}
		rows = append(rows, row)
	}
	return rows, true
}

func plugin16L2TPEstablished(rows []map[string]any) int {
	count := 0
	for _, row := range rows {
		if row["state"] == "established" {
			count++
		}
	}
	return count
}

func plugin16L2TPWriteExit() error {
	return os.WriteFile("peer.exit", []byte("ok"), 0o600)
}

func plugin16TeardownTunnel(ctx context.Context, p *sdk.Plugin) error {
	var tunnels []map[string]any
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		var ok bool
		tunnels, ok = plugin16L2TPList(ctx, p, "show l2tp tunnels")
		return ok && len(tunnels) >= 1
	}) {
		return fmt.Errorf("tunnel never established")
	}
	localTID, ok := plugin16Number(tunnels[0]["local-tid"])
	if !ok || localTID == 0 {
		return fmt.Errorf("tunnel never established")
	}
	r := plugin16Dispatch(ctx, p, fmt.Sprintf("clear l2tp tunnel id %d", localTID))
	if r.err != nil || r.status != "done" {
		return fmt.Errorf("teardown %d: status=%s data=%s", localTID, r.status, r.text())
	}
	payload, ok := r.data.(map[string]any)
	if !ok || payload["status"] != "sent" {
		return fmt.Errorf("teardown payload status=%v want sent", payload["status"])
	}
	payloadTID, ok := plugin16Number(payload["tunnel-id"])
	if !ok || payloadTID != localTID {
		return fmt.Errorf("teardown payload tunnel-id=%v want %d", payload["tunnel-id"], localTID)
	}
	if !Poll(ctx, 50, 100*time.Millisecond, func() bool {
		rows, ok := plugin16L2TPList(ctx, p, "show l2tp tunnels")
		return ok && plugin16L2TPEstablished(rows) == 0
	}) {
		return fmt.Errorf("tunnel remained in established state after teardown")
	}
	unknown := plugin16Dispatch(ctx, p, "clear l2tp tunnel id 65535")
	if unknown.status != "error" || !strings.Contains(unknown.text(), "65535") {
		return fmt.Errorf("teardown unknown: status=%s error=%q", unknown.status, unknown.text())
	}
	if err := plugin16L2TPWriteExit(); err != nil {
		return err
	}
	if !plugin16WaitEOR(ctx, p, "show bgp", 60) {
		return fmt.Errorf("ze did not send its initial-sync end-of-rib before shutdown")
	}
	return nil
}

func plugin16TeardownTunnelAll(ctx context.Context, p *sdk.Plugin) error {
	if !Poll(ctx, 150, 100*time.Millisecond, func() bool {
		rows, ok := plugin16L2TPList(ctx, p, "show l2tp tunnels")
		return ok && plugin16L2TPEstablished(rows) >= 2
	}) {
		return fmt.Errorf("both tunnels never appeared (peers did not finish handshake)")
	}
	r := plugin16Dispatch(ctx, p, "clear l2tp tunnel all")
	if r.err != nil || r.status != "done" {
		return fmt.Errorf("teardown-all: status=%s data=%s", r.status, r.text())
	}
	payload, ok := r.data.(map[string]any)
	cleared, numberOK := plugin16Number(payload["tunnels-cleared"])
	if !ok || !numberOK || cleared < 2 {
		return fmt.Errorf("teardown-all tunnels-cleared=%v want >= 2", payload["tunnels-cleared"])
	}
	if !Poll(ctx, 50, 100*time.Millisecond, func() bool {
		rows, ok := plugin16L2TPList(ctx, p, "show l2tp tunnels")
		return ok && plugin16L2TPEstablished(rows) == 0
	}) {
		return fmt.Errorf("tunnels remained established after teardown-all")
	}
	if err := plugin16L2TPWriteExit(); err != nil {
		return err
	}
	if !plugin16WaitEOR(ctx, p, "show bgp peer peer1 detail", 60) {
		return fmt.Errorf("peer1 initial-sync EOR never reached the wire")
	}
	return nil
}

func plugin16TeardownSession(ctx context.Context, p *sdk.Plugin) error {
	if !plugin16WaitEOR(ctx, p, "show bgp peer peer1 detail", 60) {
		return fmt.Errorf("peer1 never sent its initial-sync End-of-RIB")
	}
	var sessions []map[string]any
	if !Poll(ctx, 150, 100*time.Millisecond, func() bool {
		var ok bool
		sessions, ok = plugin16L2TPList(ctx, p, "show l2tp sessions")
		return ok && len(sessions) >= 1
	}) {
		return fmt.Errorf("session never established")
	}
	localSID, ok := plugin16Number(sessions[0]["local-sid"])
	if !ok || localSID == 0 {
		return fmt.Errorf("session never established")
	}
	r := plugin16Dispatch(ctx, p, fmt.Sprintf("clear l2tp session id %d", localSID))
	if r.err != nil || r.status != "done" {
		return fmt.Errorf("session teardown %d: status=%s data=%s", localSID, r.status, r.text())
	}
	payload, ok := r.data.(map[string]any)
	if !ok || payload["status"] != "sent" {
		return fmt.Errorf("teardown payload status=%v want sent", payload["status"])
	}
	payloadSID, ok := plugin16Number(payload["session-id"])
	if !ok || payloadSID != localSID {
		return fmt.Errorf("teardown payload session-id=%v want %d", payload["session-id"], localSID)
	}
	if !Poll(ctx, 50, 100*time.Millisecond, func() bool {
		rows, ok := plugin16L2TPList(ctx, p, "show l2tp sessions")
		if !ok {
			return false
		}
		for _, row := range rows {
			sid, _ := plugin16Number(row["local-sid"])
			if sid == localSID && row["state"] == "established" {
				return false
			}
		}
		return true
	}) {
		return fmt.Errorf("session remained established after teardown")
	}
	unknown := plugin16Dispatch(ctx, p, "clear l2tp session id 65534")
	if unknown.status != "error" || !strings.Contains(unknown.text(), "65534") {
		return fmt.Errorf("teardown unknown: status=%s error=%q", unknown.status, unknown.text())
	}
	return plugin16L2TPWriteExit()
}

func plugin16TeardownSessionAll(ctx context.Context, p *sdk.Plugin) error {
	if !plugin16WaitEOR(ctx, p, "show bgp peer peer1 detail", 60) {
		return fmt.Errorf("peer1 never sent its initial-sync End-of-RIB")
	}
	var sessions []map[string]any
	Poll(ctx, 200, 100*time.Millisecond, func() bool {
		var ok bool
		sessions, ok = plugin16L2TPList(ctx, p, "show l2tp sessions")
		return ok && len(sessions) >= 2
	})
	if len(sessions) < 1 {
		return fmt.Errorf("no session ever appeared")
	}
	r := plugin16Dispatch(ctx, p, "clear l2tp session all")
	if r.err != nil || r.status != "done" {
		return fmt.Errorf("teardown-all: status=%s data=%s", r.status, r.text())
	}
	payload, ok := r.data.(map[string]any)
	cleared, numberOK := plugin16Number(payload["sessions-cleared"])
	if !ok || !numberOK || cleared < 0 {
		return fmt.Errorf("teardown-all sessions-cleared=%v must be non-negative integer", payload["sessions-cleared"])
	}
	if !Poll(ctx, 50, 100*time.Millisecond, func() bool {
		rows, ok := plugin16L2TPList(ctx, p, "show l2tp sessions")
		return ok && plugin16L2TPEstablished(rows) == 0
	}) {
		return fmt.Errorf("sessions remained established after teardown-all")
	}
	return plugin16L2TPWriteExit()
}
