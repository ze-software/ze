// Design: plan/learned/664-diag-5-active-probes.md -- ICMP ping from the router
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
	argCount    = "count"
	argInterval = "interval"
	argSize     = "size"
	argTimeout  = "timeout"
)

var (
	errPingCountRequiresAValue       = errors.New("ping: count requires a value")
	errPingSizeRequiresAValue        = errors.New("ping: size requires a value (bytes)")
	errPingTimeoutRequiresAValueE    = errors.New("ping: timeout requires a value (e.g. 5s)")
	errPingIntervalRequiresAValue    = errors.New("monitor ping: interval requires a value (e.g. 500ms)")
	errPingMissingDestinationAddress = errors.New("ping: missing destination address")
)

const (
	defaultPingCount   = 5
	maxPingCount       = 100
	defaultPingTimeout = 5 * time.Second
	maxPingTimeout     = 30 * time.Second
	// maxPingSize is the largest ICMP echo payload that still fits a 65535-byte
	// IP datagram after the 20-byte IPv4 and 8-byte ICMP headers.
	maxPingSize = 65507

	// Monitor-only bounds. These match the interactive CLI's own monitor parser
	// (internal/component/cli/model_ping.go parsePingMonitorArgs) so `monitor
	// ping` accepts the same arguments whether or not a daemon is running.
	defaultPingMonitorInterval = time.Second
	minPingMonitorInterval     = 100 * time.Millisecond
	maxPingMonitorInterval     = 30 * time.Second
)

// monitorPingArgs is the parsed form of a `monitor ping` invocation.
// Count 0 streams until interrupted; Size 0 sends the default payload.
type monitorPingArgs struct {
	Dest     netip.Addr
	Interval time.Duration
	Timeout  time.Duration
	Count    int
	Size     int
}

// parseMonitorPingArgs parses
// `monitor ping <dest> [interval <dur>] [timeout <dur>] [count <n>] [size <bytes>]`.
//
// Deliberately NOT parsePingArgs. That parser has no interval case, so `monitor
// ping <dest> interval 500ms` -- which the docs advertise -- fell through to the
// destination branch and streamed at a hardcoded 1s. Bounds here match the
// interactive CLI's own monitor parser (internal/component/cli/model_ping.go
// parsePingMonitorArgs) so the command behaves the same with and without a
// daemon.
func parseMonitorPingArgs(args []string) (monitorPingArgs, error) {
	out := monitorPingArgs{
		Interval: defaultPingMonitorInterval,
		Timeout:  defaultPingTimeout,
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case argInterval:
			if i+1 >= len(args) {
				return out, errPingIntervalRequiresAValue
			}
			d, err := time.ParseDuration(args[i+1])
			if err != nil || d < minPingMonitorInterval || d > maxPingMonitorInterval {
				return out, fmt.Errorf("monitor ping: interval must be %s-%s", minPingMonitorInterval, maxPingMonitorInterval)
			}
			out.Interval = d
			i++
		case argTimeout:
			if i+1 >= len(args) {
				return out, errPingTimeoutRequiresAValueE
			}
			d, err := time.ParseDuration(args[i+1])
			if err != nil || d < time.Second || d > maxPingTimeout {
				return out, fmt.Errorf("monitor ping: timeout must be 1s-%s", maxPingTimeout)
			}
			out.Timeout = d
			i++
		case argCount:
			if i+1 >= len(args) {
				return out, errPingCountRequiresAValue
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 1 || n > maxPingCount {
				return out, fmt.Errorf("monitor ping: count must be 1-%d", maxPingCount)
			}
			out.Count = n
			i++
		case argSize:
			if i+1 >= len(args) {
				return out, errPingSizeRequiresAValue
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 1 || n > maxPingSize {
				return out, fmt.Errorf("monitor ping: size must be 1-%d", maxPingSize)
			}
			out.Size = n
			i++
		default:
			if !out.Dest.IsValid() {
				if err := validateResolveTarget(args[i]); err != nil {
					return out, fmt.Errorf("monitor ping: invalid destination %q: %w", args[i], err)
				}
				addr, err := probe.ResolveTarget(args[i])
				if err != nil {
					return out, fmt.Errorf("monitor ping: invalid destination %q: %w", args[i], err)
				}
				out.Dest = addr
				continue
			}
			return out, fmt.Errorf("monitor ping: unexpected argument %q", args[i])
		}
	}
	if !out.Dest.IsValid() {
		return out, errPingMissingDestinationAddress
	}
	return out, nil
}

// handleShowPing is the RPC handler for `show ping` (ze-show:ping): a bounded
// batch of ICMP echo requests sent from the router, returning per-reply RTT
// and an aggregate summary.
func handleShowPing(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	dest, count, timeout, opts, err := parsePingArgs(args)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	results, pingErr := doPing(dest, count, timeout, opts)
	if pingErr != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: pingErr.Error()}, nil //nolint:nilerr // operational error in Response
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(results)}, nil
}

// parsePingArgs parses `show ping <dest> [count <n>] [size <bytes>]
// [timeout <dur>]`. The returned pingOpts carries the optional ICMP payload
// size; its zero value means the engine picks its default.
func parsePingArgs(args []string) (netip.Addr, int, time.Duration, pingOpts, error) {
	var dest netip.Addr
	var opts pingOpts
	count := defaultPingCount
	timeout := defaultPingTimeout

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case argCount:
			if i+1 >= len(args) {
				return dest, 0, 0, opts, errPingCountRequiresAValue
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 1 || n > maxPingCount {
				return dest, 0, 0, opts, fmt.Errorf("ping: count must be 1-%d", maxPingCount)
			}
			count = n
			i++
		case argSize:
			if i+1 >= len(args) {
				return dest, 0, 0, opts, errPingSizeRequiresAValue
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 1 || n > maxPingSize {
				return dest, 0, 0, opts, fmt.Errorf("ping: size must be 1-%d", maxPingSize)
			}
			opts.size = n
			i++
		case argTimeout:
			if i+1 >= len(args) {
				return dest, 0, 0, opts, errPingTimeoutRequiresAValueE
			}
			d, err := time.ParseDuration(args[i+1])
			if err != nil || d < time.Second || d > maxPingTimeout {
				return dest, 0, 0, opts, fmt.Errorf("ping: timeout must be 1s-%s", maxPingTimeout)
			}
			timeout = d
			i++
		default:
			if !dest.IsValid() {
				if err := validateResolveTarget(args[i]); err != nil {
					return dest, 0, 0, opts, fmt.Errorf("ping: invalid destination %q: %w", args[i], err)
				}
				addr, err := probe.ResolveTarget(args[i])
				if err != nil {
					return dest, 0, 0, opts, fmt.Errorf("ping: invalid destination %q: %w", args[i], err)
				}
				dest = addr
			}
		}
	}
	if !dest.IsValid() {
		return dest, 0, 0, opts, errPingMissingDestinationAddress
	}
	return dest, count, timeout, opts, nil
}

type pingOpts struct {
	source netip.Addr
	size   int
}

// pingPayload returns the ICMP echo payload for a requested size. size <= 0
// yields the small default marker; otherwise the payload is exactly size bytes
// with the marker copied into the front, so a capture still identifies the
// sender. Shared by the batch engine (doPing) and the streaming session
// (streamPing) so `show ping ... size N` and `monitor ping ... size N` put the
// same bytes on the wire.
func pingPayload(size int) []byte {
	if size <= 0 {
		return []byte("ze-ping")
	}
	b := make([]byte, size)
	copy(b, "ze-ping")
	return b
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

	payload := pingPayload(opts.size)

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
