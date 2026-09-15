// Design: docs/architecture/diagnostics/active-probes.md -- batch probe round (show probe-round RPC)
// Related: traceroute.go -- shared ICMP helpers (ttlSetter, embeddedICMPOffset, addrFromNetAddr)

package cmd

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/probe"
)

const defaultProbeMaxHops = 16

// HandleProbeRound runs a single parallel traceroute probe round.
func HandleProbeRound(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	req, err := parseTracerouteArgs(args)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	maxHops := req.maxHops
	if maxHops == defaultTracerouteMaxHops {
		maxHops = defaultProbeMaxHops
	}
	hops, probeErr := doProbeRound(req.target, maxHops, time.Second)
	if probeErr != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: probeErr.Error()}, nil //nolint:nilerr // operational error in Response
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{
		fieldHops: hops,
	}}, nil
}

type probeResult struct {
	ttl     int
	addr    string
	rttMS   float64
	hasRTT  bool
	reached bool
}

func doProbeRound(dest netip.Addr, maxHops int, deadline time.Duration) ([]map[string]any, error) {
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
	rawConn, err := openRawProbeConn(context.Background(), probe.FamilyOf(dest), netip.Addr{}, probe.DFOff)
	if err != nil {
		return nil, fmt.Errorf("probe: %w", err)
	}

	conn := newTTLConn(rawConn, isV6)
	defer func() { _ = conn.Close() }()

	echoID := conn.Identifier()
	dst := &net.IPAddr{IP: dest.AsSlice()}
	sendTimes := make([]time.Time, maxHops)

	for ttl := 1; ttl <= maxHops; ttl++ {
		if setErr := conn.SetTTL(ttl); setErr != nil {
			return nil, fmt.Errorf("probe: set TTL %d: %w", ttl, setErr)
		}
		seq := uint16(ttl - 1)
		pkt := probe.BuildICMPEcho(icmpEcho, echoID, seq, []byte("ze-probe"))
		sendTimes[ttl-1] = time.Now()
		if _, writeErr := conn.WriteTo(pkt, dst); writeErr != nil {
			return nil, fmt.Errorf("probe: write TTL %d: %w", ttl, writeErr)
		}
	}

	results := make([]probeResult, maxHops)
	for i := range results {
		results[i].ttl = i + 1
		results[i].addr = "*"
	}

	probeStart := time.Now()
	end := probeStart.Add(deadline)
	rb := make([]byte, 1500)
	answered := 0

	for answered < maxHops {
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
			if embID != echoID {
				continue
			}
			ttl := int(embSeq) + 1
			if ttl < 1 || ttl > maxHops {
				continue
			}
			r := &results[ttl-1]
			if r.hasRTT {
				continue
			}
			r.addr = addr
			r.rttMS = float64(time.Since(sendTimes[ttl-1]).Microseconds()) / 1000.0
			r.hasRTT = true
			answered++
			if msgType == icmpDestUnreach && rb[1] == portUnreach {
				r.reached = true
			}

		case icmpEchoReply:
			replyID := binary.BigEndian.Uint16(rb[4:6])
			replySeq := binary.BigEndian.Uint16(rb[6:8])
			if replyID != echoID {
				continue
			}
			ttl := int(replySeq) + 1
			if ttl < 1 || ttl > maxHops {
				continue
			}
			r := &results[ttl-1]
			if r.hasRTT {
				continue
			}
			r.addr = addr
			r.rttMS = float64(time.Since(sendTimes[ttl-1]).Microseconds()) / 1000.0
			r.hasRTT = true
			r.reached = true
			answered++
		}
	}

	if wait := time.Until(end); wait > 0 {
		time.Sleep(wait)
	}

	// Find the lowest TTL that reached the destination and trim beyond it.
	limit := maxHops
	for i := range results {
		if results[i].reached {
			limit = i + 1
			break
		}
	}

	hops := make([]map[string]any, limit)
	for i := range hops {
		hop := map[string]any{
			fieldTTL: results[i].ttl,
			"addr":   results[i].addr,
		}
		if results[i].hasRTT {
			hop["rtt-ms"] = results[i].rttMS
		} else {
			hop["rtt-ms"] = nil
		}
		hops[i] = hop
	}
	return hops, nil
}
