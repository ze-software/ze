// Design: docs/architecture/diagnostics/active-probes.md -- ICMP ping from the router
// Related: stream.go -- streaming ping session; ICMP helpers in internal/core/probe

package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/probe"
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
	// errPingNoProbesSent is returned when a count>0 batch put no probe on the
	// wire at all (e.g. the first WriteTo failed with ENETUNREACH/EPERM). The
	// serial engine surfaced the write error directly; runPingSession swallows it
	// (it only emits per-probe maps), so the batch reconstructs the failure from
	// the empty result rather than reporting a misleading sent=0/loss=0% success.
	errPingNoProbesSent = errors.New("ping: no probe could be sent (destination unreachable or permission denied)")
)

const (
	defaultPingCount   = 5
	maxPingCount       = 100
	defaultPingTimeout = 5 * time.Second
	maxPingTimeout     = 30 * time.Second
	// maxPingSize is the largest ICMP echo payload that still fits a 65535-byte
	// IP datagram after the 20-byte IPv4 and 8-byte ICMP headers.
	maxPingSize = 65507

	// defaultPingBatchInterval paces show ping's probe sends. show ping has no
	// operator-facing interval knob (unlike monitor ping); this internal cadence
	// exists so a large batch does not burst count packets onto the raw socket at
	// once -- count 100 size 65507 would queue ~6.5 MB instantly and risk a
	// WriteTo error. It is small enough that a healthy batch still returns
	// promptly: the pacing for the maximum count is maxPingCount * this, well
	// under a second. The bug this fixes lived in the coupling of send rate to
	// reply latency, not in the send rate itself, so a small fixed pace suffices.
	defaultPingBatchInterval = 10 * time.Millisecond

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

	// runPingBatch takes ownership of conn and closes it exactly once. It returns
	// an error when a count>0 batch put nothing on the wire (fail closed), which
	// doPingCtx propagates so the handler reports StatusError rather than a
	// misleading 0%-loss result.
	return runPingBatch(ctx, conn, clock.RealClock{}, dest, defaultPingBatchInterval, timeout, count, opts.size, icmpEcho, icmpEchoReply)
}

// runPingBatch runs a bounded ICMP echo batch over an already-open conn and an
// injected clock, returning the show-ping result map. It is the seam the unit
// tests exercise with a fake conn + fake clock (no CAP_NET_RAW, no wall-clock
// sleeps), the same reason the streaming session grew one.
//
// It reuses the streaming session's sender/receiver split (runPingSession):
// probe sends are paced by interval and each reply is matched to its probe by
// sequence number, so a lost probe no longer blocks the next send. The old
// serial loop sent probe seq+1 only after seq's own reply-or-deadline, so a
// black-holed target serialized to count*timeout -- a `show ping count 100
// timeout 30s` run against a black hole took ~50 minutes. The decoupled batch is
// bounded by ~(count*interval)+timeout instead. Sharing runPingSession (rather
// than a second seq-keyed receiver) is the R-4 mitigation in the spec: one
// matcher, no drift between the batch and streaming paths.
//
// conn is owned here: runPingSession closes it exactly once on teardown.
func runPingBatch(
	ctx context.Context,
	conn pingConn,
	clk clock.Clock,
	dest netip.Addr,
	interval, timeout time.Duration,
	count, size int,
	icmpEcho, icmpEchoReply byte,
) (map[string]any, error) {
	if count <= 0 {
		// A batch of no probes: nothing to send. Close the socket we were handed
		// and return the empty-but-well-formed result. Guarding here is
		// load-bearing: runPingSession treats count == 0 as "stream until the
		// context is canceled", so passing a non-positive count straight through
		// would turn a bounded batch into an unbounded hang (fail-closed).
		if closeErr := conn.Close(); closeErr != nil {
			// Nothing actionable on an empty batch; the close is still made so we
			// do not leak the socket we were handed.
			_ = closeErr
		}
		return emptyPingResult(dest), nil
	}

	// runPingSession streams one map per probe (seq/status[/rtt-ms] -- the exact
	// per-reply shape show ping returns) and closes out once the bounded run
	// ends. Buffer to count so it never blocks emitting while we aggregate.
	out := make(chan map[string]any, count)
	go runPingSession(ctx, conn, clk, dest, interval, timeout, count, size, icmpEcho, icmpEchoReply, out)

	replies := make([]map[string]any, 0, count)
	for r := range out {
		replies = append(replies, r)
	}

	// runPingSession emits in resolution order (a late or lost probe resolves
	// after later ones); sort by sequence so the batch output is deterministic
	// and keeps the old serial 0..N-1 order that printPingResults (offline.go)
	// and ping-show.ci render.
	sort.Slice(replies, func(i, j int) bool {
		si, _ := replies[i]["seq"].(int)
		sj, _ := replies[j]["seq"].(int)
		return si < sj
	})

	// Fail closed: a count>0 batch that collected no replies never put a probe on
	// the wire (every successful send resolves to exactly one reply-or-timeout
	// map). runPingSession swallows the WriteTo error, so an empty result means
	// either the first send failed (unreachable/permission) or the caller
	// canceled before any probe went out. Reporting that as sent=0/received=0/
	// loss-percent=0 would render a transport failure as a healthy 0%-loss answer
	// -- the fail-open pattern ai/rules/evidence.md forbids. Surface it
	// as an error instead, matching the serial engine's StatusError on write.
	if len(replies) == 0 {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("ping: canceled before any probe was sent: %w", ctxErr)
		}
		return nil, errPingNoProbesSent
	}

	return summarizePingReplies(dest, replies), nil
}

// summarizePingReplies builds the show-ping result map from the per-probe
// replies collected from the session, preserving the exact shape the serial
// engine produced. sent counts every probe that reached the wire (each collected
// reply is one successful WriteTo), received counts the ok replies, and
// min/avg/max are present only when at least one probe was answered.
func summarizePingReplies(dest netip.Addr, replies []map[string]any) map[string]any {
	sent := len(replies)
	received := 0
	var minMs, maxMs, sumMs float64
	for _, r := range replies {
		if status, _ := r["status"].(string); status != "ok" {
			continue
		}
		rtt, _ := r["rtt-ms"].(float64)
		if received == 0 || rtt < minMs {
			minMs = rtt
		}
		if rtt > maxMs {
			maxMs = rtt
		}
		sumMs += rtt
		received++
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
		result["min-rtt-ms"] = minMs
		result["avg-rtt-ms"] = sumMs / float64(received)
		result["max-rtt-ms"] = maxMs
	}
	return result
}

// emptyPingResult is the well-formed result map for a batch that sent nothing.
// replies is a non-nil []map[string]any so printPingResults' type assertion
// (offline.go) holds.
func emptyPingResult(dest netip.Addr) map[string]any {
	return map[string]any{
		"destination":  dest.String(),
		"sent":         0,
		"received":     0,
		"loss-percent": 0.0,
		"replies":      make([]map[string]any, 0),
	}
}
