// Design: traceroute_parallel.go -- batch probe round (show probe-round RPC)
// Related: ping.go -- sibling probe command; ICMP helpers in internal/core/probe

package show

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"net"
	"net/netip"
	"time"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"

	"codeberg.org/thomas-mangin/ze/internal/core/probe"
)

// NewTracerouteSession starts a streaming probe round for the given target.
// Returns a channel of hop results, a cancel function, and an error.
func NewTracerouteSession(ctx context.Context, target string, maxHops int) (<-chan map[string]any, context.CancelFunc, error) {
	addr, err := probe.ResolveTarget(target)
	if err != nil {
		return nil, nil, err
	}

	ch := make(chan map[string]any, maxHops)
	roundCtx, cancel := context.WithCancel(ctx)

	go StreamProbeRound(roundCtx, addr, maxHops, time.Second, ch)

	return ch, cancel, nil
}

func randProbeID() uint16 {
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0xBEEF
	}
	return binary.BigEndian.Uint16(b[:])
}

// StreamProbeRound sends all TTL probes simultaneously and writes hop results
// to out as ICMP responses arrive. After deadline, unanswered hops are sent
// with addr="*" and rtt-ms=nil. The channel is closed when the round ends.
// Each message is a map with keys: "ttl" (int), "addr" (string), "rtt-ms" (float64 or nil).
func StreamProbeRound(ctx context.Context, dest netip.Addr, maxHops int, deadline time.Duration, out chan<- map[string]any) {
	defer close(out)

	network := probe.NetworkICMPv4
	icmpEcho := byte(8)
	icmpEchoReply := byte(0)
	icmpTimeExceeded := byte(11)
	icmpDestUnreach := byte(3)
	portUnreach := byte(icmpv4PortUnreach)
	isV6 := dest.Is6()
	if isV6 {
		network = probe.NetworkICMPv6
		icmpEcho = 128
		icmpEchoReply = 129
		icmpTimeExceeded = 3
		icmpDestUnreach = 1
		portUnreach = icmpv6PortUnreach
	}

	var lc net.ListenConfig
	rawConn, err := lc.ListenPacket(ctx, network, "")
	if err != nil {
		return
	}

	var conn ttlSetter
	if isV6 {
		conn = &ipv6TTLConn{raw: rawConn, pconn: ipv6.NewPacketConn(rawConn)}
	} else {
		conn = &ipv4TTLConn{raw: rawConn, pconn: ipv4.NewPacketConn(rawConn)}
	}
	defer func() { _ = conn.Close() }()

	pid := randProbeID()
	dst := &net.IPAddr{IP: dest.AsSlice()}
	sendTimes := make([]time.Time, maxHops)

	for ttl := 1; ttl <= maxHops; ttl++ {
		if setErr := conn.SetTTL(ttl); setErr != nil {
			continue
		}
		seq := uint16(ttl - 1)
		pkt := probe.BuildICMPEcho(icmpEcho, pid, seq, []byte("ze-probe"))
		sendTimes[ttl-1] = time.Now()
		if _, writeErr := conn.WriteTo(pkt, dst); writeErr != nil {
			continue
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
				continue
			}
			embID := binary.BigEndian.Uint16(rb[off+4 : off+6])
			embSeq := binary.BigEndian.Uint16(rb[off+6 : off+8])
			if embID != pid {
				continue
			}
			ttl := int(embSeq) + 1
			if ttl < 1 || ttl > maxHops || results[ttl-1].hasRTT {
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

		case icmpEchoReply:
			replyID := binary.BigEndian.Uint16(rb[4:6])
			replySeq := binary.BigEndian.Uint16(rb[6:8])
			if replyID != pid {
				continue
			}
			ttl := int(replySeq) + 1
			if ttl < 1 || ttl > maxHops || results[ttl-1].hasRTT {
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
		}
	}

	// Pad remaining time so rounds are consistently ~1s.
	if wait := time.Until(end); wait > 0 {
		time.Sleep(wait)
	}

	// Send results up to the destination (or all if destination not found).
	limit := maxHops
	if reachedTTL > 0 {
		limit = reachedTTL
	}
	for i := range limit {
		r := &results[i]
		hop := map[string]any{"ttl": i + 1}
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
