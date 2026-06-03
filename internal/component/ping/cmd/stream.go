// Design: docs/architecture/api/commands.md -- streaming ping session
// Overview: register.go -- RPC + offline-root registration for this module
//
// stream.go provides the continuous ping session used by `monitor ping`. The
// CLI/hub streaming factories call NewPingSession; ICMP echo packets and target
// resolution come from internal/core/probe.

package cmd

import (
	"context"
	"encoding/binary"
	"net"
	"net/netip"
	"os"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/probe"
)

// NewPingSession starts a continuous ping stream to the given target.
// Each reply is sent as a map on the returned channel. The channel is
// closed when the context is canceled. Cancel stops the session.
func NewPingSession(ctx context.Context, target string, interval, timeout time.Duration) (<-chan map[string]any, context.CancelFunc, error) {
	addr, err := probe.ResolveTarget(target)
	if err != nil {
		return nil, nil, err
	}

	ch := make(chan map[string]any, 64)
	pingCtx, cancel := context.WithCancel(ctx)

	go streamPing(pingCtx, addr, interval, timeout, ch)

	return ch, cancel, nil
}

func streamPing(ctx context.Context, dest netip.Addr, interval, timeout time.Duration, out chan<- map[string]any) {
	defer close(out)

	network := probe.NetworkICMPv4
	icmpEcho := byte(8)
	icmpEchoReply := byte(0)
	if dest.Is6() {
		network = probe.NetworkICMPv6
		icmpEcho = 128
		icmpEchoReply = 129
	}

	var lc net.ListenConfig
	conn, err := lc.ListenPacket(ctx, network, "")
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	pid := uint16(os.Getpid() & 0xffff)
	rb := make([]byte, 1500)

	for seq := 0; ; seq++ {
		if ctx.Err() != nil {
			return
		}

		pkt := probe.BuildICMPEcho(icmpEcho, pid, uint16(seq&0xffff), []byte("ze-ping"))
		start := time.Now()

		if deadlineErr := conn.SetDeadline(start.Add(timeout)); deadlineErr != nil {
			return
		}

		_, writeErr := conn.WriteTo(pkt, &net.IPAddr{IP: dest.AsSlice()})
		if writeErr != nil {
			return
		}

		result := map[string]any{
			"seq":    seq,
			"status": "timeout",
		}

		for {
			n, from, readErr := conn.ReadFrom(rb)
			if readErr != nil {
				break
			}
			if n < 8 || rb[0] != icmpEchoReply {
				continue
			}
			replyID := binary.BigEndian.Uint16(rb[4:6])
			replySeq := binary.BigEndian.Uint16(rb[6:8])
			if replyID != pid || replySeq != uint16(seq&0xffff) {
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
			rtt := time.Since(start)
			result["status"] = "ok"
			result["rtt-ms"] = float64(rtt.Microseconds()) / 1000.0
			break
		}

		select {
		case out <- result:
		case <-ctx.Done():
			return
		}

		elapsed := time.Since(start)
		if wait := interval - elapsed; wait > 0 {
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return
			}
		}
	}
}
