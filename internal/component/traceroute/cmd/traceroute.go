// Design: plan/learned/729-diag-traceroute.md -- ICMP traceroute from the router
// Related: register.go -- registers ze-show:traceroute and the rest of the surface

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

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/probe"
)

// argTimeout is the keyword for the per-probe timeout argument. The traceroute
// feature module owns its own copy so it does not depend on a central verb
// package (the central show package keeps its own for tcp-check).
const argTimeout = "timeout"

var (
	errTracerouteMissingTarget      = errors.New("traceroute: missing target address")
	errTracerouteMaxHopsRequiresVal = errors.New("traceroute: max-hops requires a value")
	errTracerouteTimeoutRequiresVal = errors.New("traceroute: timeout requires a value (e.g. 2s)")
	errTracerouteProbesRequiresVal  = errors.New("traceroute: probes requires a value")
)

const (
	defaultTracerouteMaxHops = 30
	maxTracerouteMaxHops     = 64
	defaultTracerouteTimeout = 3 * time.Second
	maxTracerouteTimeout     = 30 * time.Second
	defaultTracerouteProbes  = 3
	maxTracerouteProbes      = 10
)

func handleTraceroute(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	target, maxHops, timeout, probes, err := parseTracerouteArgs(args)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	hops, trErr := doTraceroute(target, maxHops, timeout, probes, tracerouteOpts{})
	if trErr != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: trErr.Error()}, nil //nolint:nilerr // operational error in Response
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{
		"hops": hops,
	}}, nil
}

func parseTracerouteArgs(args []string) (netip.Addr, int, time.Duration, int, error) {
	var target netip.Addr
	maxHops := defaultTracerouteMaxHops
	timeout := defaultTracerouteTimeout
	probes := defaultTracerouteProbes

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "max-hops":
			if i+1 >= len(args) {
				return target, 0, 0, 0, errTracerouteMaxHopsRequiresVal
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 1 || n > maxTracerouteMaxHops {
				return target, 0, 0, 0, fmt.Errorf("traceroute: max-hops must be 1-%d", maxTracerouteMaxHops)
			}
			maxHops = n
			i++
		case argTimeout:
			if i+1 >= len(args) {
				return target, 0, 0, 0, errTracerouteTimeoutRequiresVal
			}
			d, err := time.ParseDuration(args[i+1])
			if err != nil || d < time.Second || d > maxTracerouteTimeout {
				return target, 0, 0, 0, fmt.Errorf("traceroute: timeout must be 1s-%s", maxTracerouteTimeout)
			}
			timeout = d
			i++
		case "probes":
			if i+1 >= len(args) {
				return target, 0, 0, 0, errTracerouteProbesRequiresVal
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 1 || n > maxTracerouteProbes {
				return target, 0, 0, 0, fmt.Errorf("traceroute: probes must be 1-%d", maxTracerouteProbes)
			}
			probes = n
			i++
		default:
			if !target.IsValid() {
				if err := validateResolveTarget(args[i]); err != nil {
					return target, 0, 0, 0, fmt.Errorf("traceroute: invalid target %q: %w", args[i], err)
				}
				addr, err := probe.ResolveTarget(args[i])
				if err != nil {
					return target, 0, 0, 0, fmt.Errorf("traceroute: invalid target %q: %w", args[i], err)
				}
				target = addr
			}
		}
	}
	if !target.IsValid() {
		return target, 0, 0, 0, errTracerouteMissingTarget
	}
	return target, maxHops, timeout, probes, nil
}

type ttlSetter interface {
	SetTTL(ttl int) error
	SetDeadline(t time.Time) error
	WriteTo(b []byte, dst net.Addr) (int, error)
	ReadFrom(b []byte) (int, net.Addr, error)
	Close() error
}

type ipv4TTLConn struct {
	raw   net.PacketConn
	pconn *ipv4.PacketConn
}

func (c *ipv4TTLConn) SetTTL(ttl int) error                        { return c.pconn.SetTTL(ttl) }
func (c *ipv4TTLConn) SetDeadline(t time.Time) error               { return c.raw.SetDeadline(t) }
func (c *ipv4TTLConn) WriteTo(b []byte, dst net.Addr) (int, error) { return c.raw.WriteTo(b, dst) }
func (c *ipv4TTLConn) ReadFrom(b []byte) (int, net.Addr, error)    { return c.raw.ReadFrom(b) }
func (c *ipv4TTLConn) Close() error                                { return c.raw.Close() }

type ipv6TTLConn struct {
	raw   net.PacketConn
	pconn *ipv6.PacketConn
}

func (c *ipv6TTLConn) SetTTL(ttl int) error                        { return c.pconn.SetHopLimit(ttl) }
func (c *ipv6TTLConn) SetDeadline(t time.Time) error               { return c.raw.SetDeadline(t) }
func (c *ipv6TTLConn) WriteTo(b []byte, dst net.Addr) (int, error) { return c.raw.WriteTo(b, dst) }
func (c *ipv6TTLConn) ReadFrom(b []byte) (int, net.Addr, error)    { return c.raw.ReadFrom(b) }
func (c *ipv6TTLConn) Close() error                                { return c.raw.Close() }

// icmpPortUnreach is Code 3 for IPv4, Code 4 for IPv6.
const (
	icmpv4PortUnreach = 3
	icmpv6PortUnreach = 4
)

// embeddedICMPOffset returns the byte offset within an ICMP error
// message (TimeExceeded / DestUnreach) where the original ICMP header
// starts. Returns -1 if the packet is too short to contain the
// embedded header. Layout: [8 bytes ICMP error header] [original IP
// header] [original ICMP header ...].
func embeddedICMPOffset(rb []byte, n int, isV6 bool) int {
	if isV6 {
		// ICMPv6 error: 8-byte header + 40-byte IPv6 header + 8 bytes payload
		if n < 56 {
			return -1
		}
		return 48
	}
	// ICMPv4 error: 8-byte header + variable IP header + 8 bytes payload
	if n < 36 {
		return -1
	}
	ihl := int(rb[8]&0x0f) * 4
	if ihl < 20 || 8+ihl+8 > n {
		return -1
	}
	return 8 + ihl
}

type tracerouteOpts struct {
	source netip.Addr
}

func doTraceroute(dest netip.Addr, maxHops int, timeout time.Duration, probes int, opts tracerouteOpts) ([]map[string]any, error) {
	return doTracerouteCtx(context.Background(), dest, maxHops, timeout, probes, opts)
}

func doTracerouteCtx(ctx context.Context, dest netip.Addr, maxHops int, timeout time.Duration, probes int, opts tracerouteOpts) ([]map[string]any, error) {
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

	bindAddr := ""
	if opts.source.IsValid() {
		bindAddr = opts.source.String()
	}

	var lc net.ListenConfig
	rawConn, err := lc.ListenPacket(ctx, network, bindAddr)
	if err != nil {
		return nil, fmt.Errorf("traceroute: %w (requires CAP_NET_RAW)", err)
	}

	var conn ttlSetter
	if isV6 {
		conn = &ipv6TTLConn{raw: rawConn, pconn: ipv6.NewPacketConn(rawConn)}
	} else {
		conn = &ipv4TTLConn{raw: rawConn, pconn: ipv4.NewPacketConn(rawConn)}
	}
	defer func() { _ = conn.Close() }()

	pid := uint16(os.Getpid() & 0xffff)
	rb := make([]byte, 1500)
	hops := make([]map[string]any, 0, maxHops)
	dst := &net.IPAddr{IP: dest.AsSlice()}

	for ttl := 1; ttl <= maxHops; ttl++ {
		if setErr := conn.SetTTL(ttl); setErr != nil {
			return nil, fmt.Errorf("traceroute: set TTL %d: %w", ttl, setErr)
		}

		bestAddr := "*"
		var bestRTT *float64
		reached := false

		for p := range probes {
			seq := uint16((ttl-1)*probes + p)
			pkt := probe.BuildICMPEcho(icmpEcho, pid, seq, []byte("ze-trace"))

			start := time.Now()
			if deadlineErr := conn.SetDeadline(start.Add(timeout)); deadlineErr != nil {
				return nil, fmt.Errorf("traceroute: set deadline: %w", deadlineErr)
			}

			_, writeErr := conn.WriteTo(pkt, dst)
			if writeErr != nil {
				return nil, fmt.Errorf("traceroute: write: %w", writeErr)
			}

			for {
				n, from, readErr := conn.ReadFrom(rb)
				if readErr != nil {
					break
				}
				if n < 8 {
					continue
				}
				msgType := rb[0]
				switch msgType {
				case icmpTimeExceeded, icmpDestUnreach:
					if off := embeddedICMPOffset(rb, n, isV6); off >= 0 {
						embID := binary.BigEndian.Uint16(rb[off+4 : off+6])
						embSeq := binary.BigEndian.Uint16(rb[off+6 : off+8])
						if embID != pid || embSeq != seq {
							continue
						}
					}
					rtt := time.Since(start)
					addr := addrFromNetAddr(from)
					if bestAddr == "*" {
						bestAddr = addr
					}
					r := float64(rtt.Microseconds()) / 1000.0
					bestRTT = &r
					if msgType == icmpDestUnreach && rb[1] == portUnreach {
						reached = true
					}
				case icmpEchoReply:
					replyID := binary.BigEndian.Uint16(rb[4:6])
					replySeq := binary.BigEndian.Uint16(rb[6:8])
					if replyID != pid || replySeq != seq {
						continue
					}
					rtt := time.Since(start)
					addr := addrFromNetAddr(from)
					if bestAddr == "*" {
						bestAddr = addr
					}
					r := float64(rtt.Microseconds()) / 1000.0
					bestRTT = &r
					reached = true
				default:
					continue
				}
				break
			}
		}

		hop := map[string]any{
			"ttl":  ttl,
			"addr": bestAddr,
		}
		if bestRTT != nil {
			hop["rtt-ms"] = *bestRTT
		} else {
			hop["rtt-ms"] = nil
		}
		hops = append(hops, hop)

		if reached {
			break
		}
	}
	return hops, nil
}

func addrFromNetAddr(from net.Addr) string {
	if from == nil {
		return "*"
	}
	if ipAddr, ok := from.(*net.IPAddr); ok {
		addr, ok := netip.AddrFromSlice(ipAddr.IP)
		if ok {
			return addr.Unmap().String()
		}
	}
	return from.String()
}
