// Design: plan/spec-diag-5-active-probes.md -- ICMP ping from the router
// Related: stream.go -- streaming ping session; ICMP helpers in internal/core/probe

package cmd

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strconv"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/probe"
)

// CLI argument keywords shared across the ping handlers in this package.
const (
	argCount   = "count"
	argTimeout = "timeout"
)

var (
	errPingCountRequiresAValue       = errors.New("ping: count requires a value")
	errPingTimeoutRequiresAValueE    = errors.New("ping: timeout requires a value (e.g. 5s)")
	errPingMissingDestinationAddress = errors.New("ping: missing destination address")
)

const (
	defaultPingCount   = 5
	maxPingCount       = 100
	defaultPingTimeout = 5 * time.Second
	maxPingTimeout     = 30 * time.Second
)

// handleShowPing is the RPC handler for `show ping` (ze-show:ping): a bounded
// batch of ICMP echo requests sent from the router, returning per-reply RTT
// and an aggregate summary.
func handleShowPing(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	dest, count, timeout, err := parsePingArgs(args)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	results, pingErr := doPing(dest, count, timeout, pingOpts{})
	if pingErr != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: pingErr.Error()}, nil //nolint:nilerr // operational error in Response
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(results)}, nil
}

func parsePingArgs(args []string) (netip.Addr, int, time.Duration, error) {
	var dest netip.Addr
	count := defaultPingCount
	timeout := defaultPingTimeout

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case argCount:
			if i+1 >= len(args) {
				return dest, 0, 0, errPingCountRequiresAValue
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 1 || n > maxPingCount {
				return dest, 0, 0, fmt.Errorf("ping: count must be 1-%d", maxPingCount)
			}
			count = n
			i++
		case argTimeout:
			if i+1 >= len(args) {
				return dest, 0, 0, errPingTimeoutRequiresAValueE
			}
			d, err := time.ParseDuration(args[i+1])
			if err != nil || d < time.Second || d > maxPingTimeout {
				return dest, 0, 0, fmt.Errorf("ping: timeout must be 1s-%s", maxPingTimeout)
			}
			timeout = d
			i++
		default:
			if !dest.IsValid() {
				addr, err := probe.ResolveTarget(args[i])
				if err != nil {
					return dest, 0, 0, fmt.Errorf("ping: invalid destination %q: %w", args[i], err)
				}
				dest = addr
			}
		}
	}
	if !dest.IsValid() {
		return dest, 0, 0, errPingMissingDestinationAddress
	}
	return dest, count, timeout, nil
}

type pingOpts struct {
	source netip.Addr
	size   int
}

func doPing(dest netip.Addr, count int, timeout time.Duration, opts pingOpts) (map[string]any, error) {
	return doPingCtx(context.Background(), dest, count, timeout, opts)
}

func doPingCtx(ctx context.Context, dest netip.Addr, count int, timeout time.Duration, opts pingOpts) (map[string]any, error) {
	network := probe.NetworkICMPv4
	icmpEcho := byte(8)
	icmpEchoReply := byte(0)
	if dest.Is6() {
		network = probe.NetworkICMPv6
		icmpEcho = 128
		icmpEchoReply = 129
	}

	bindAddr := ""
	if opts.source.IsValid() {
		bindAddr = opts.source.String()
	}

	var lc net.ListenConfig
	conn, err := lc.ListenPacket(ctx, network, bindAddr)
	if err != nil {
		return nil, fmt.Errorf("ping: %w (requires CAP_NET_RAW)", err)
	}
	defer func() { _ = conn.Close() }()

	payload := []byte("ze-ping")
	if opts.size > 0 {
		payload = make([]byte, opts.size)
		copy(payload, "ze-ping")
	}

	rbSize := max(1500, opts.size+8)

	pid := uint16(os.Getpid() & 0xffff)
	var sent, received int
	var minRTT, maxRTT, totalRTT time.Duration
	replies := make([]map[string]any, 0, count)
	rb := make([]byte, rbSize)

	for seq := range count {
		pkt := probe.BuildICMPEcho(icmpEcho, pid, uint16(seq), payload)

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
