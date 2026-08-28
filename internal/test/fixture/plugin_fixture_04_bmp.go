package fixture

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func bmpHeader04(kind byte, payload []byte) []byte {
	out := make([]byte, 6+len(payload))
	out[0] = 3
	binary.BigEndian.PutUint32(out[1:5], uint32(len(out)))
	out[5] = kind
	copy(out[6:], payload)
	return out
}

func bmpInitiation04(withDescription bool) []byte {
	name := []byte("test-router")
	payload := make([]byte, 4+len(name))
	binary.BigEndian.PutUint16(payload[0:2], 2)
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(name)))
	copy(payload[4:], name)
	if withDescription {
		desc := []byte("ze ci test")
		tlv := make([]byte, 4+len(desc))
		binary.BigEndian.PutUint16(tlv[0:2], 1)
		binary.BigEndian.PutUint16(tlv[2:4], uint16(len(desc)))
		copy(tlv[4:], desc)
		payload = append(payload, tlv...)
	}
	return bmpHeader04(4, payload)
}

func bgpOpen04(asn uint16) []byte {
	out := make([]byte, 29)
	for i := range 16 {
		out[i] = 0xff
	}
	binary.BigEndian.PutUint16(out[16:18], uint16(len(out)))
	out[18] = 1
	out[19] = 4
	binary.BigEndian.PutUint16(out[20:22], asn)
	binary.BigEndian.PutUint16(out[22:24], 90)
	binary.BigEndian.PutUint32(out[24:28], 0x01020304)
	return out
}

func bmpPeerHeader04() []byte {
	out := make([]byte, 42)
	copy(out[20:26], []byte{0xff, 0xff, 10, 0, 0, 1})
	binary.BigEndian.PutUint32(out[26:30], 65001)
	binary.BigEndian.PutUint32(out[30:34], 0x01020304)
	binary.BigEndian.PutUint32(out[34:38], uint32(time.Now().Unix()))
	return out
}

func bmpPeerUp04() []byte {
	payload := bmpPeerHeader04()
	local := make([]byte, 16)
	copy(local[10:], []byte{0xff, 0xff, 10, 0, 0, 100})
	payload = append(payload, local...)
	ports := make([]byte, 4)
	binary.BigEndian.PutUint16(ports[0:2], 179)
	binary.BigEndian.PutUint16(ports[2:4], 54321)
	payload = append(payload, ports...)
	payload = append(payload, bgpOpen04(65000)...)
	payload = append(payload, bgpOpen04(65001)...)
	return bmpHeader04(3, payload)
}

func bgpUpdate04() []byte {
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 3, 4, 10, 0, 0, 100}
	body := []byte{0, 0, 0, byte(len(attrs))}
	body = append(body, attrs...)
	body = append(body, 24, 10, 0, 0)
	out := bytes.Repeat([]byte{0xff}, 16)
	length := 19 + len(body)
	out = append(out, byte(length>>8), byte(length), 2)
	return append(out, body...)
}

func bmpRouteMonitoring04() []byte {
	payload := append(bmpPeerHeader04(), bgpUpdate04()...)
	return bmpHeader04(0, payload)
}

func bmpPeerDown04() []byte {
	return bmpHeader04(2, append(bmpPeerHeader04(), 5))
}

func connectBMP04(args []string) (net.Conn, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("usage: fixture requires BMP port")
	}
	return net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", args[0]), 5*time.Second)
}

func withBMP04(fn func(context.Context, *sdk.Plugin, net.Conn) error) Driver {
	return func(ctx context.Context, args []string) error {
		return Observe(ctx, "fixture-bmp-04", sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) error {
			conn, err := connectBMP04(args)
			if err != nil {
				return err
			}
			defer conn.Close()
			return fn(ctx, p, conn)
		})
	}
}

func writeBMP04(conn net.Conn, messages ...[]byte) error {
	for _, message := range messages {
		if _, err := conn.Write(message); err != nil {
			return err
		}
	}
	return nil
}

func containsPrefix04(value any, prefix string) bool {
	encoded, _ := json.Marshal(value)
	return strings.Contains(string(encoded), prefix)
}

func hasRoute04(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		if prefix, ok := v["prefix"].(string); ok && prefix != "" {
			return true
		}
		for _, child := range v {
			if hasRoute04(child) {
				return true
			}
		}
	case []any:
		for _, child := range v {
			if hasRoute04(child) {
				return true
			}
		}
	}
	return false
}

var bmpIngest04 = withBMP04(func(ctx context.Context, p *sdk.Plugin, conn net.Conn) error {
	if err := writeBMP04(conn, bmpInitiation04(false), bmpPeerUp04(), bmpRouteMonitoring04()); err != nil {
		return err
	}
	_, value, err := pollCommand04(ctx, p, 100, "show bmp rib", func(status string, value any) bool {
		return status == "done" && containsPrefix04(value, "10.0.0.0")
	})
	if err != nil {
		return err
	}
	if !containsPrefix04(value, "10.0.0.0") {
		return fmt.Errorf("route 10.0.0.0/24 not found in show bmp rib: %v", value)
	}
	fmt.Fprintln(os.Stderr, "PASS: BMP route 10.0.0.0/24 found in show bmp rib")
	return nil
})

var bmpBestpath04 = withBMP04(func(ctx context.Context, p *sdk.Plugin, conn net.Conn) error {
	if err := writeBMP04(conn, bmpInitiation04(false), bmpPeerUp04(), bmpRouteMonitoring04()); err != nil {
		return err
	}
	_, value, err := pollCommand04(ctx, p, 100, "show bmp rib", func(status string, value any) bool {
		return status == "done" && hasRoute04(value)
	})
	if err != nil {
		return err
	}
	if !hasRoute04(value) {
		return errors.New("no BMP routes found")
	}
	_, bgpRIB, err := command04(ctx, p, "show bgp rib")
	if err != nil {
		return err
	}
	if containsPrefix04(bgpRIB, "10.0.0.0") {
		return errors.New("BMP route leaked into bgp rib show")
	}
	_, best, err := command04(ctx, p, "show bgp rib best")
	if err != nil {
		return err
	}
	if containsPrefix04(best, "10.0.0.0") {
		return errors.New("BMP route appeared in best-path")
	}
	fmt.Fprintln(os.Stderr, "PASS: BMP routes isolated from best-path selection")
	return nil
})

var bmpDisconnect04 = withBMP04(func(ctx context.Context, p *sdk.Plugin, conn net.Conn) error {
	if err := writeBMP04(conn, bmpInitiation04(false), bmpPeerUp04(), bmpRouteMonitoring04()); err != nil {
		return err
	}
	_, value, err := pollCommand04(ctx, p, 100, "show bmp rib", func(status string, value any) bool {
		return status == "done" && hasRoute04(value)
	})
	if err != nil || !hasRoute04(value) {
		return errors.Join(err, errors.New("no routes before disconnect"))
	}
	if err := conn.Close(); err != nil {
		return err
	}
	_, value, err = pollCommand04(ctx, p, 100, "show bmp rib", func(status string, value any) bool {
		return status == "done" && !hasRoute04(value)
	})
	if err != nil {
		return err
	}
	if hasRoute04(value) {
		return fmt.Errorf("routes still present after disconnect: %v", value)
	}
	fmt.Fprintln(os.Stderr, "PASS: BMP routes withdrawn after session disconnect")
	return nil
})

var bmpMessages04 = withBMP04(func(ctx context.Context, p *sdk.Plugin, conn net.Conn) error {
	if err := writeBMP04(conn, bmpInitiation04(true), bmpPeerUp04(), bmpPeerDown04()); err != nil {
		return err
	}
	status, value, err := pollCommand04(ctx, p, 100, "show bmp peers", func(status string, value any) bool {
		data, _ := value.(map[string]any)
		for _, peer := range mapSlice04(data["peers"]) {
			up, _ := peer["up"].(bool)
			if number04(peer["peer-as"]) == 65001 && !up {
				return status == "done"
			}
		}
		return false
	})
	if err != nil {
		return err
	}
	data, _ := value.(map[string]any)
	down := false
	for _, peer := range mapSlice04(data["peers"]) {
		up, _ := peer["up"].(bool)
		down = down || (number04(peer["peer-as"]) == 65001 && !up)
	}
	if status != "done" || !down {
		return errors.New("bmp peer-down never reflected in show bmp peers")
	}
	if err := conn.Close(); err != nil {
		return err
	}
	_, _, _ = pollCommand04(ctx, p, 100, "show bmp sessions", func(status string, value any) bool {
		data, _ := value.(map[string]any)
		return status == "done" && len(mapSlice04(data["sessions"])) == 0
	})
	return nil
})

func bmpReceiverSessionDriver04(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: receiver-session <port>")
	}
	port := args[0]
	return Observe(ctx, "fixture-bmp-receiver-session-04", sdk.Registration{}, func(ctx context.Context, _ *sdk.Plugin) error {
		if !Poll(ctx, 100, 200*time.Millisecond, func() bool {
			conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", port), time.Second)
			if err != nil {
				return false
			}
			conn.Close()
			return true
		}) {
			return fmt.Errorf("BMP receiver never listened on port %s", port)
		}
		return nil
	})
}

var bmpSessions04 = withBMP04(func(ctx context.Context, p *sdk.Plugin, conn net.Conn) error {
	if err := writeBMP04(conn, bmpInitiation04(false), bmpPeerUp04()); err != nil {
		return err
	}
	status, value, err := pollCommand04(ctx, p, 40, "show bmp sessions", func(status string, value any) bool {
		data, _ := value.(map[string]any)
		return status == "done" && len(mapSlice04(data["sessions"])) > 0
	})
	if err != nil {
		return err
	}
	data, _ := value.(map[string]any)
	sessions := mapSlice04(data["sessions"])
	if status != "done" || len(sessions) == 0 {
		return fmt.Errorf("expected non-empty session list: %v", value)
	}
	if _, a := sessions[0]["sys-name"]; !a {
		if _, b := sessions[0]["sys_name"]; !b {
			if _, c := sessions[0]["sysName"]; !c {
				return fmt.Errorf("session missing sys-name field: %v", sessions[0])
			}
		}
	}
	status, _, err = command04(ctx, p, "show bmp collectors")
	if err != nil || status != "done" {
		return fmt.Errorf("show bmp collectors status=%s: %w", status, err)
	}
	fmt.Fprintf(os.Stderr, "PASS: show bmp sessions returned %d session(s)\n", len(sessions))
	fmt.Fprintln(os.Stderr, "PASS: show bmp collectors returned valid response")
	return nil
})

func bmpMarkerPath04(port string) string {
	return filepath.Join(os.TempDir(), "ze-fixture-bmp-"+port+".done")
}

func configuredBMPMarker04(ctx context.Context, p *sdk.Plugin) (string, error) {
	_, value, err := pollCommand04(ctx, p, 100, "show bmp collectors", func(status string, value any) bool {
		data, _ := value.(map[string]any)
		collectors := mapSlice04(data["collectors"])
		return status == "done" && len(collectors) > 0 && number04(collectors[0]["port"]) > 0
	})
	if err != nil {
		return "", err
	}
	data, _ := value.(map[string]any)
	collectors := mapSlice04(data["collectors"])
	return bmpMarkerPath04(fmt.Sprintf("%.0f", number04(collectors[0]["port"]))), nil
}

func markerObserver04(_ bool, attempts int, delay time.Duration) ObserverScenario {
	return func(ctx context.Context, p *sdk.Plugin) error {
		marker, err := configuredBMPMarker04(ctx, p)
		if err != nil {
			return err
		}
		defer os.Remove(marker) //nolint:errcheck
		if !Poll(ctx, attempts, delay, func() bool {
			_, err := os.Stat(marker)
			return err == nil
		}) {
			return errors.New("bmp collector never wrote its completion marker")
		}
		if err := waitPeerEOR04(ctx, p); err != nil {
			return fmt.Errorf("test peer never sent its initial-sync End-of-RIB: %w", err)
		}
		return nil
	}
}

func bmpCollector04(mode string) Driver {
	return func(ctx context.Context, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("usage: collector <port>")
		}
		marker := bmpMarkerPath04(args[0])
		if err := os.Remove(marker); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove stale collector marker: %w", err)
		}
		listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", args[0]))
		if err != nil {
			return err
		}
		defer listener.Close()
		fmt.Fprintf(os.Stderr, "BMP-COLLECTOR: listening on %s\n", args[0])
		if tcp, ok := listener.(*net.TCPListener); ok {
			tcp.SetDeadline(time.Now().Add(15 * time.Second))
		}
		conn, err := listener.Accept()
		if err != nil {
			fmt.Fprintln(os.Stderr, "BMP-COLLECTOR: invalid-pdu: no connection accepted before timeout")
			return err
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
		valid := false
		for {
			header := make([]byte, 6)
			if _, err := io.ReadFull(conn, header); err != nil {
				break
			}
			length := binary.BigEndian.Uint32(header[1:5])
			if header[0] != 3 || length < 6 || length > 65535 {
				fmt.Fprintf(os.Stderr, "BMP-COLLECTOR: invalid-pdu: BMP version=%d length=%d\n", header[0], length)
				break
			}
			payload := make([]byte, int(length)-6)
			if _, err := io.ReadFull(conn, payload); err != nil {
				break
			}
			switch mode {
			case "monitoring":
				if header[5] == 0 {
					valid = validateMonitoring04(payload, false)
				}
			case "locrib":
				if len(payload) > 0 && payload[0] == 3 && header[5] == 3 {
					fmt.Fprintln(os.Stderr, "BMP-COLLECTOR: locrib-peer-up")
				}
				if len(payload) > 0 && payload[0] == 3 && header[5] == 0 {
					valid = validateMonitoring04(payload, true)
				}
			case "mirroring":
				if header[5] == 6 {
					valid = validateMirroring04(payload)
				}
			case "peer-up":
				if header[5] == 3 {
					valid = validatePeerUp04(payload)
				}
			}
			if valid {
				break
			}
		}
		fmt.Fprintln(os.Stderr, "BMP-COLLECTOR: done")
		if err := os.WriteFile(marker, []byte("1\n"), 0o600); err != nil {
			return fmt.Errorf("write collector marker: %w", err)
		}
		if !valid {
			return fmt.Errorf("no valid %s PDU received", mode)
		}
		return nil
	}
}

func validateBGP04(pdu []byte, kind byte, minimum int) error {
	if len(pdu) < minimum {
		return fmt.Errorf("BGP PDU too short (%d bytes)", len(pdu))
	}
	if !bytes.Equal(pdu[:16], bytes.Repeat([]byte{0xff}, 16)) {
		return errors.New("BGP marker mismatch")
	}
	length := int(binary.BigEndian.Uint16(pdu[16:18]))
	if length != len(pdu) || length > 4096 {
		return fmt.Errorf("BGP length=%d but PDU has %d bytes", length, len(pdu))
	}
	if pdu[18] != kind {
		return fmt.Errorf("BGP type=%d, want %d", pdu[18], kind)
	}
	return nil
}

func validateMonitoring04(payload []byte, locrib bool) bool {
	if len(payload) < 42+19 {
		fmt.Fprintf(os.Stderr, "BMP-COLLECTOR: invalid-pdu: payload too short (%d bytes)\n", len(payload))
		return false
	}
	if err := validateBGP04(payload[42:], 2, 19); err != nil {
		fmt.Fprintf(os.Stderr, "BMP-COLLECTOR: invalid-pdu: %v\n", err)
		return false
	}
	if locrib {
		fmt.Fprintln(os.Stderr, "BMP-COLLECTOR: valid-locrib-route-monitoring")
	} else {
		fmt.Fprintln(os.Stderr, "BMP-COLLECTOR: valid-route-monitoring-pdu")
	}
	return true
}

func validateMirroring04(payload []byte) bool {
	if len(payload) < 46 {
		fmt.Fprintln(os.Stderr, "BMP-COLLECTOR: invalid-pdu: mirroring payload too short")
		return false
	}
	kind := binary.BigEndian.Uint16(payload[42:44])
	length := int(binary.BigEndian.Uint16(payload[44:46]))
	if kind != 0 || length < 19 || len(payload) < 46+length {
		fmt.Fprintln(os.Stderr, "BMP-COLLECTOR: invalid-pdu: invalid route-mirroring TLV")
		return false
	}
	pdu := payload[46 : 46+length]
	if err := validateBGP04(pdu, pdu[18], 19); err != nil {
		fmt.Fprintf(os.Stderr, "BMP-COLLECTOR: invalid-pdu: %v\n", err)
		return false
	}
	fmt.Fprintf(os.Stderr, "BMP-COLLECTOR: valid-route-mirroring: type=%d len=%d\n", pdu[18], len(pdu))
	return true
}

func validatePeerUp04(payload []byte) bool {
	const fixed = 42 + 16 + 2 + 2
	if len(payload) < fixed+29 {
		fmt.Fprintln(os.Stderr, "BMP-COLLECTOR: invalid-pdu: peer up too short")
		return false
	}
	off := fixed
	if len(payload) < off+19 {
		return false
	}
	sentLen := int(binary.BigEndian.Uint16(payload[off+16 : off+18]))
	if len(payload) < off+sentLen {
		return false
	}
	sent := payload[off : off+sentLen]
	off += sentLen
	if len(payload) < off+19 {
		return false
	}
	recvLen := int(binary.BigEndian.Uint16(payload[off+16 : off+18]))
	if len(payload) < off+recvLen {
		return false
	}
	received := payload[off : off+recvLen]
	if err := validateBGP04(sent, 1, 30); err != nil {
		fmt.Fprintf(os.Stderr, "BMP-COLLECTOR: invalid-pdu: sent-open %v\n", err)
		return false
	}
	if err := validateBGP04(received, 1, 30); err != nil {
		fmt.Fprintf(os.Stderr, "BMP-COLLECTOR: invalid-pdu: recv-open %v\n", err)
		return false
	}
	fmt.Fprintf(os.Stderr, "BMP-COLLECTOR: valid-peer-up-open: sent=%dB recv=%dB\n", len(sent), len(received))
	return true
}
