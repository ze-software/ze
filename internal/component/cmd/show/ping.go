// Design: plan/spec-diag-5-active-probes.md -- ICMP ping from the router

package show

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strconv"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

const (
	defaultPingCount   = 5
	maxPingCount       = 100
	defaultPingTimeout = 5 * time.Second
	maxPingTimeout     = 30 * time.Second
)

func handlePing(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	dest, count, timeout, err := parsePingArgs(args)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	results, pingErr := doPing(dest, count, timeout)
	if pingErr != nil {
		return &plugin.Response{Status: plugin.StatusError, Data: pingErr.Error()}, nil //nolint:nilerr // operational error in Response
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: results}, nil
}

func parsePingArgs(args []string) (netip.Addr, int, time.Duration, error) {
	var dest netip.Addr
	count := defaultPingCount
	timeout := defaultPingTimeout

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "count": //nolint:goconst // CLI arg, unrelated to map key uses in show.go
			if i+1 >= len(args) {
				return dest, 0, 0, fmt.Errorf("ping: count requires a value")
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 1 || n > maxPingCount {
				return dest, 0, 0, fmt.Errorf("ping: count must be 1-%d", maxPingCount)
			}
			count = n
			i++
		case "timeout":
			if i+1 >= len(args) {
				return dest, 0, 0, fmt.Errorf("ping: timeout requires a value (e.g. 5s)")
			}
			d, err := time.ParseDuration(args[i+1])
			if err != nil || d < time.Second || d > maxPingTimeout {
				return dest, 0, 0, fmt.Errorf("ping: timeout must be 1s-%s", maxPingTimeout)
			}
			timeout = d
			i++
		default:
			if !dest.IsValid() {
				addr, err := netip.ParseAddr(args[i])
				if err != nil {
					return dest, 0, 0, fmt.Errorf("ping: invalid destination %q: %w", args[i], err)
				}
				dest = addr
			}
		}
	}
	if !dest.IsValid() {
		return dest, 0, 0, fmt.Errorf("ping: missing destination address")
	}
	return dest, count, timeout, nil
}

func doPing(dest netip.Addr, count int, timeout time.Duration) (map[string]any, error) {
	network := "ip4:icmp"
	icmpEcho := byte(8)
	icmpEchoReply := byte(0)
	if dest.Is6() {
		network = "ip6:ipv6-icmp"
		icmpEcho = 128
		icmpEchoReply = 129
	}

	var lc net.ListenConfig
	conn, err := lc.ListenPacket(context.Background(), network, "")
	if err != nil {
		return nil, fmt.Errorf("ping: %w (requires CAP_NET_RAW)", err)
	}
	defer func() { _ = conn.Close() }()

	pid := uint16(os.Getpid() & 0xffff)
	var sent, received int
	var minRTT, maxRTT, totalRTT time.Duration
	replies := make([]map[string]any, 0, count)
	rb := make([]byte, 1500)

	for seq := range count {
		pkt := buildICMPEcho(icmpEcho, pid, uint16(seq), []byte("ze-ping"))

		start := time.Now()
		if deadlineErr := conn.SetDeadline(start.Add(timeout)); deadlineErr != nil {
			return nil, fmt.Errorf("ping: set deadline: %w", deadlineErr)
		}

		_, writeErr := conn.WriteTo(pkt, &net.IPAddr{IP: dest.AsSlice()})
		if writeErr != nil {
			return nil, fmt.Errorf("ping: write: %w", writeErr)
		}
		sent++
		matched := false
		for !matched {
			n, from, readErr := conn.ReadFrom(rb)
			if readErr != nil {
				replies = append(replies, map[string]any{
					"seq":    seq,
					"status": "timeout",
				})
				break
			}
			if n < 8 || rb[0] != icmpEchoReply {
				continue
			}
			replyID := binary.BigEndian.Uint16(rb[4:6])
			replySeq := binary.BigEndian.Uint16(rb[6:8])
			if replyID != pid || replySeq != uint16(seq) {
				continue
			}
			if from != nil {
				if ipAddr, ok := from.(*net.IPAddr); ok {
					fromAddr, _ := netip.AddrFromSlice(ipAddr.IP)
					if fromAddr.IsValid() && fromAddr != dest {
						continue
					}
				}
			}
			matched = true
		}
		if !matched {
			continue
		}

		rtt := time.Since(start)
		received++
		totalRTT += rtt
		if minRTT == 0 || rtt < minRTT {
			minRTT = rtt
		}
		if rtt > maxRTT {
			maxRTT = rtt
		}

		replies = append(replies, map[string]any{
			"seq":    seq,
			"rtt-ms": float64(rtt.Microseconds()) / 1000.0,
			"status": "ok",
		})
	}

	lossPercent := 0.0
	if sent > 0 {
		lossPercent = float64(sent-received) / float64(sent) * 100
	}

	result := map[string]any{
		"destination":  dest.String(),
		"sent":         sent,
		"received":     received,
		"loss-percent": lossPercent,
		"replies":      replies,
	}
	if received > 0 {
		avgRTT := totalRTT / time.Duration(received)
		result["min-rtt-ms"] = float64(minRTT.Microseconds()) / 1000.0
		result["avg-rtt-ms"] = float64(avgRTT.Microseconds()) / 1000.0
		result["max-rtt-ms"] = float64(maxRTT.Microseconds()) / 1000.0
	}
	return result, nil
}

func buildICMPEcho(typ byte, id, seq uint16, data []byte) []byte {
	b := make([]byte, 8+len(data))
	b[0] = typ
	b[1] = 0
	binary.BigEndian.PutUint16(b[4:], id)
	binary.BigEndian.PutUint16(b[6:], seq)
	copy(b[8:], data)
	binary.BigEndian.PutUint16(b[2:], icmpChecksum(b))
	return b
}

func icmpChecksum(b []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(b); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(b[i:]))
	}
	if len(b)%2 == 1 {
		sum += uint32(b[len(b)-1]) << 8
	}
	sum = (sum >> 16) + (sum & 0xffff)
	sum += sum >> 16
	return ^uint16(sum)
}
