// Design: docs/architecture/api/commands.md -- streaming traceroute session
// Overview: register.go -- RPC + local-handler registration for this module
//
// stream.go provides the continuous traceroute session used by `monitor traceroute`.
// The CLI/hub streaming factories call NewTracerouteSession; ICMP echo packets and
// target resolution come from internal/core/probe.

package cmd

import (
	"context"
	"encoding/binary"
	"net"
	"net/netip"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/probe"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var _ = env.MustRegister(env.EnvEntry{
	Key:         "ze.trace.probe",
	Type:        "bool",
	Default:     "false",
	Description: "Trace ICMP probe send/receive in StreamProbeRound (stderr diagnostic)",
})

func probeTraceEnabled() bool {
	return env.GetBool("ze.trace.probe", false)
}

func identifierHex(echoID uint16) []byte {
	return []byte{byte(echoID >> 8), byte(echoID)}
}

// NewTracerouteSession starts a streaming probe round for the given target.
// Returns a channel of hop results, a cancel function, and an error.
func NewTracerouteSession(ctx context.Context, target string, maxHops int) (<-chan map[string]any, context.CancelFunc, error) {
	addr, err := probe.ResolveTarget(target, probe.FamilyAny)
	if err != nil {
		return nil, nil, err
	}

	ch := make(chan map[string]any, maxHops)
	roundCtx, cancel := context.WithCancel(ctx)

	go StreamProbeRound(roundCtx, addr, maxHops, time.Second, ch)

	return ch, cancel, nil
}

// StreamProbeRound sends all TTL probes simultaneously and writes hop results
// to out as ICMP responses arrive. After deadline, unanswered hops are sent
// with addr="*" and rtt-ms=nil. The channel is closed when the round ends.
// Each message is a map with keys: "ttl" (int), "addr" (string), "rtt-ms" (float64 or nil).
//
// Set ze.trace.probe=true to emit per-packet diagnostics to stderr.
func StreamProbeRound(ctx context.Context, dest netip.Addr, maxHops int, deadline time.Duration, out chan<- map[string]any) {
	defer close(out)

	trace := probeTraceEnabled()

	icmpEcho := byte(8)
	icmpEchoReply := byte(0)
	icmpTimeExceeded := byte(11)
	icmpDestUnreach := byte(3)
	portUnreach := byte(icmpv4PortUnreach)
	isV6 := dest.Is6()
	if isV6 {
		icmpEcho = 128
		icmpEchoReply = 129
		icmpTimeExceeded = 3
		icmpDestUnreach = 1
		portUnreach = icmpv6PortUnreach
	}

	// This surface carries no DF keyword yet, so the mode is named here as off
	// rather than left to the zero value the probe layer refuses.
	rawConn, err := openRawProbeConn(ctx, probe.FamilyOf(dest), netip.Addr{}, probe.DFOff)
	if err != nil {
		return
	}

	conn := newTTLConn(rawConn, isV6)
	defer func() { _ = conn.Close() }()

	// The identifier is the socket's own, so concurrent rounds on one host
	// answer to different identifiers.
	echoID := conn.Identifier()
	identifierBytes := identifierHex(echoID)
	dst := &net.IPAddr{IP: dest.AsSlice()}
	sendTimes := make([]time.Time, maxHops)

	if trace {
		var tb textbuf.Buffer
		tb.Str("[probe] round start dest=").Str(dest.String())
		tb.Str(" echoID=0x").Hex(identifierBytes)
		tb.Str(" maxHops=").Int(int64(maxHops)).Byte('\n')
		tb.StdErr() //nolint:errcheck // trace
	}

	for ttl := 1; ttl <= maxHops; ttl++ {
		if setErr := conn.SetTTL(ttl); setErr != nil {
			if trace {
				var tb textbuf.Buffer
				tb.Str("[probe] SetTTL(").Int(int64(ttl)).Str(") failed: ").Err(setErr).Byte('\n')
				tb.StdErr() //nolint:errcheck // trace
			}
			continue
		}
		seq := uint16(ttl - 1)
		pkt := probe.BuildICMPEcho(icmpEcho, echoID, seq, []byte("ze-probe"))
		sendTimes[ttl-1] = time.Now()
		if _, writeErr := conn.WriteTo(pkt, dst); writeErr != nil {
			if trace {
				var tb textbuf.Buffer
				tb.Str("[probe] send ttl=").Int(int64(ttl)).Str(" failed: ").Err(writeErr).Byte('\n')
				tb.StdErr() //nolint:errcheck // trace
			}
			continue
		}
		if trace {
			var tb textbuf.Buffer
			tb.Str("[probe] send ttl=").Int(int64(ttl))
			tb.Str(" seq=").Int(int64(seq))
			tb.Str(" echoID=0x").Hex(identifierBytes).Byte('\n')
			tb.StdErr() //nolint:errcheck // trace
		}
	}

	type hopResult struct {
		addr    string
		rttMS   float64
		hasRTT  bool
		reached bool
	}

	results := make([]hopResult, maxHops)
	reachedTTL := 0
	end := time.Now().Add(deadline)
	rb := make([]byte, 1500)
	total := 0

	for total < maxHops {
		if ctx.Err() != nil {
			return
		}
		remaining := time.Until(end)
		if remaining <= 0 {
			break
		}
		if setErr := conn.SetDeadline(time.Now().Add(remaining)); setErr != nil {
			break
		}

		n, from, readErr := conn.ReadFrom(rb)
		if readErr != nil {
			break
		}
		if n < 8 {
			continue
		}

		addr := addrFromNetAddr(from)
		msgType := rb[0]

		switch msgType {
		case icmpTimeExceeded, icmpDestUnreach:
			off := embeddedICMPOffset(rb, n, isV6)
			if off < 0 {
				if trace {
					var tb textbuf.Buffer
					tb.Str("[probe] recv type=").Int(int64(msgType))
					tb.Str(" from=").Str(addr)
					tb.Str(" n=").Int(int64(n))
					tb.Str(" bad-offset hex=").Hex(rb[:min(n, 64)]).Byte('\n')
					tb.StdErr() //nolint:errcheck // trace
				}
				continue
			}
			embID := binary.BigEndian.Uint16(rb[off+4 : off+6])
			embSeq := binary.BigEndian.Uint16(rb[off+6 : off+8])
			if embID != echoID {
				if trace {
					var tb textbuf.Buffer
					tb.Str("[probe] recv type=").Int(int64(msgType))
					tb.Str(" from=").Str(addr)
					tb.Str(" embID=0x").Hex(identifierHex(embID))
					tb.Str(" want=0x").Hex(identifierBytes)
					tb.Str(" FILTERED\n")
					tb.StdErr() //nolint:errcheck // trace
				}
				continue
			}
			ttl := int(embSeq) + 1
			if ttl < 1 || ttl > maxHops || results[ttl-1].hasRTT {
				if trace {
					var tb textbuf.Buffer
					tb.Str("[probe] recv type=").Int(int64(msgType))
					tb.Str(" from=").Str(addr)
					tb.Str(" embSeq=").Int(int64(embSeq))
					tb.Str(" ttl=").Int(int64(ttl))
					if ttl >= 1 && ttl <= maxHops && results[ttl-1].hasRTT {
						tb.Str(" DUPLICATE")
					} else {
						tb.Str(" OUT-OF-RANGE")
					}
					tb.Byte('\n')
					tb.StdErr() //nolint:errcheck // trace
				}
				continue
			}
			r := &results[ttl-1]
			r.addr = addr
			r.rttMS = float64(time.Since(sendTimes[ttl-1]).Microseconds()) / 1000.0
			r.hasRTT = true
			total++
			if msgType == icmpDestUnreach && rb[1] == portUnreach {
				r.reached = true
				if reachedTTL == 0 || ttl < reachedTTL {
					reachedTTL = ttl
				}
			}

			if trace {
				var tb textbuf.Buffer
				tb.Str("[probe] recv type=").Int(int64(msgType))
				tb.Str(" from=").Str(addr)
				tb.Str(" ttl=").Int(int64(ttl))
				tb.Str(" rtt=").Str(strconv.FormatFloat(r.rttMS, 'f', 3, 64)).Str("ms")
				if r.reached {
					tb.Str(" REACHED")
				}
				tb.Byte('\n')
				tb.StdErr() //nolint:errcheck // trace
			}

		case icmpEchoReply:
			replyID := binary.BigEndian.Uint16(rb[4:6])
			replySeq := binary.BigEndian.Uint16(rb[6:8])
			if replyID != echoID {
				if trace {
					var tb textbuf.Buffer
					tb.Str("[probe] recv echo-reply from=").Str(addr)
					tb.Str(" replyID=0x").Hex(identifierHex(replyID))
					tb.Str(" want=0x").Hex(identifierBytes)
					tb.Str(" FILTERED\n")
					tb.StdErr() //nolint:errcheck // trace
				}
				continue
			}
			ttl := int(replySeq) + 1
			if ttl < 1 || ttl > maxHops || results[ttl-1].hasRTT {
				if trace {
					var tb textbuf.Buffer
					tb.Str("[probe] recv echo-reply from=").Str(addr)
					tb.Str(" seq=").Int(int64(replySeq))
					tb.Str(" ttl=").Int(int64(ttl))
					if ttl >= 1 && ttl <= maxHops && results[ttl-1].hasRTT {
						tb.Str(" DUPLICATE")
					} else {
						tb.Str(" OUT-OF-RANGE")
					}
					tb.Byte('\n')
					tb.StdErr() //nolint:errcheck // trace
				}
				continue
			}
			r := &results[ttl-1]
			r.addr = addr
			r.rttMS = float64(time.Since(sendTimes[ttl-1]).Microseconds()) / 1000.0
			r.hasRTT = true
			r.reached = true
			total++
			if reachedTTL == 0 || ttl < reachedTTL {
				reachedTTL = ttl
			}

			if trace {
				var tb textbuf.Buffer
				tb.Str("[probe] recv echo-reply from=").Str(addr)
				tb.Str(" ttl=").Int(int64(ttl))
				tb.Str(" rtt=").Str(strconv.FormatFloat(r.rttMS, 'f', 3, 64)).Str("ms REACHED\n")
				tb.StdErr() //nolint:errcheck // trace
			}

		default:
			if trace {
				var tb textbuf.Buffer
				tb.Str("[probe] recv unknown type=").Int(int64(msgType))
				tb.Str(" from=").Str(addr)
				tb.Str(" n=").Int(int64(n))
				tb.Str(" hex=").Hex(rb[:min(n, 32)]).Byte('\n')
				tb.StdErr() //nolint:errcheck // trace
			}
		}
	}

	if wait := time.Until(end); wait > 0 {
		time.Sleep(wait)
	}

	limit := maxHops
	if reachedTTL > 0 {
		limit = reachedTTL
	}

	if trace {
		var tb textbuf.Buffer
		tb.Str("[probe] round done: ").Int(int64(total)).Str("/").Int(int64(maxHops)).Str(" answered")
		tb.Str(" limit=").Int(int64(limit)).Byte('\n')
		tb.StdErr() //nolint:errcheck // trace
	}

	for i := range limit {
		r := &results[i]
		hop := map[string]any{fieldTTL: i + 1}
		if r.hasRTT {
			hop["addr"] = r.addr
			hop["rtt-ms"] = r.rttMS
		} else {
			hop["addr"] = "*"
			hop["rtt-ms"] = nil
		}
		select {
		case out <- hop:
		case <-ctx.Done():
			return
		}
	}
}
