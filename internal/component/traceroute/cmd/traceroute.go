// Design: docs/architecture/diagnostics/active-probes.md -- ICMP traceroute from the router
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
	"syscall"
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

// The JSON keys a traceroute-shaped response answers with.
const (
	fieldHops = "hops"
	fieldTTL  = "ttl"
)

var (
	errTracerouteMissingTarget      = errors.New("traceroute: missing target address")
	errTracerouteMaxHopsRequiresVal = errors.New("traceroute: max-hops requires a value")
	errTracerouteTimeoutRequiresVal = errors.New("traceroute: timeout requires a value (e.g. 2s)")
	errTracerouteProbesRequiresVal  = errors.New("traceroute: probes requires a value")
	errTracerouteDFRequiresVal      = errors.New("traceroute: do-not-fragment requires a value (honor-cache or bypass-cache)")
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
	req, err := parseTracerouteArgs(args)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	hops, trErr := doTraceroute(req.target, req.maxHops, req.timeout, req.probes, req.opts)
	if trErr != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: trErr.Error()}, nil //nolint:nilerr // operational error in Response
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{
		fieldHops: hops,
	}}, nil
}

// tracerouteArgs is the parsed form of a `show traceroute` invocation, shared
// by show traceroute, show probe-round and the offline local handlers.
type tracerouteArgs struct {
	target  netip.Addr
	maxHops int
	timeout time.Duration
	probes  int
	opts    tracerouteOpts
}

// parseTracerouteArgs parses `show traceroute <dest> [max-hops <n>]
// [timeout <dur>] [probes <n>] [do-not-fragment honor-cache|bypass-cache]`.
// opts carries the DF mode, DFOff when the keyword is absent; show traceroute
// binds no source.
func parseTracerouteArgs(args []string) (tracerouteArgs, error) {
	req := tracerouteArgs{
		maxHops: defaultTracerouteMaxHops,
		timeout: defaultTracerouteTimeout,
		probes:  defaultTracerouteProbes,
		opts:    tracerouteOpts{df: probe.DFOff},
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "max-hops":
			if i+1 >= len(args) {
				return req, errTracerouteMaxHopsRequiresVal
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 1 || n > maxTracerouteMaxHops {
				return req, fmt.Errorf("traceroute: max-hops must be 1-%d", maxTracerouteMaxHops)
			}
			req.maxHops = n
			i++
		case argTimeout:
			if i+1 >= len(args) {
				return req, errTracerouteTimeoutRequiresVal
			}
			d, err := time.ParseDuration(args[i+1])
			if err != nil || d < time.Second || d > maxTracerouteTimeout {
				return req, fmt.Errorf("traceroute: timeout must be 1s-%s", maxTracerouteTimeout)
			}
			req.timeout = d
			i++
		case "probes":
			if i+1 >= len(args) {
				return req, errTracerouteProbesRequiresVal
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 1 || n > maxTracerouteProbes {
				return req, fmt.Errorf("traceroute: probes must be 1-%d", maxTracerouteProbes)
			}
			req.probes = n
			i++
		case probe.DFKeyword:
			if i+1 >= len(args) {
				return req, errTracerouteDFRequiresVal
			}
			mode, err := probe.DFModeOfValue(args[i+1])
			if err != nil {
				return req, fmt.Errorf("traceroute: %w", err)
			}
			req.opts.df = mode
			i++
		default:
			if !req.target.IsValid() {
				if err := validateResolveTarget(args[i]); err != nil {
					return req, fmt.Errorf("traceroute: invalid target %q: %w", args[i], err)
				}
				addr, err := probe.ResolveTarget(args[i], probe.FamilyAny)
				if err != nil {
					return req, fmt.Errorf("traceroute: invalid target %q: %w", args[i], err)
				}
				req.target = addr
			}
		}
	}
	if !req.target.IsValid() {
		return req, errTracerouteMissingTarget
	}
	return req, nil
}

// probeSocket is what the trace loops need from the *probe.Socket OpenICMP
// returns, so a test can stand a fake in through openProbeConn.
type probeSocket interface {
	Kind() probe.SocketKind
	Identifier() uint16
	PacketConn() net.PacketConn
	SetDeadline(t time.Time) error
	WriteTo(b []byte, dst net.Addr) (int, error)
	ReadFrom(b []byte) (int, net.Addr, error)
	Close() error
	DrainErrors(visit func(probe.QueuedError)) error
}

// errTracerouteNeedsRawSocket is what every traceroute surface answers on
// the unprivileged datagram socket: the kernel delivers Time Exceeded to a
// raw socket only, so a trace on the datagram kind would time out at every
// hop and report a path it never saw.
var errTracerouteNeedsRawSocket = errors.New("traceroute needs the raw ICMP socket (CAP_NET_RAW): the unprivileged datagram socket receives no Time Exceeded")

// openRawProbeConn opens the probe socket and refuses the datagram kind.
// It is the one opener the three trace loops call.
func openRawProbeConn(ctx context.Context, family probe.Family, bind netip.Addr, df probe.DFMode) (probeSocket, error) {
	sock, err := openProbeConn(ctx, family, bind, df)
	if err != nil {
		return nil, err
	}
	if sock.Kind() == probe.SocketDatagram {
		sock.Close() //nolint:errcheck // the socket is refused, not used
		return nil, errTracerouteNeedsRawSocket
	}
	return sock, nil
}

type ttlSetter interface {
	Identifier() uint16
	SetTTL(ttl int) error
	SetDeadline(t time.Time) error
	WriteTo(b []byte, dst net.Addr) (int, error)
	ReadFrom(b []byte) (int, net.Addr, error)
	Close() error
	// DrainErrors hands every entry queued on the socket's error queue to
	// visit, bounded by probe.ErrQueueDrainMax, and never blocks.
	DrainErrors(visit func(probe.QueuedError)) error
}

// newTTLConn wraps the raw probe socket in the per-family TTL setter. It is
// a variable so a test can stand a scripted ttlSetter in its place and drive
// the trace loop, including its refusal branch, with no CAP_NET_RAW: the
// x/net wrappers need a real descriptor for SetTTL.
var newTTLConn = func(raw probeSocket, isV6 bool) ttlSetter {
	if isV6 {
		return &ipv6TTLConn{raw: raw, pconn: ipv6.NewPacketConn(raw.PacketConn())}
	}
	return &ipv4TTLConn{raw: raw, pconn: ipv4.NewPacketConn(raw.PacketConn())}
}

type ipv4TTLConn struct {
	raw   probeSocket
	pconn *ipv4.PacketConn
}

func (c *ipv4TTLConn) Identifier() uint16                          { return c.raw.Identifier() }
func (c *ipv4TTLConn) SetTTL(ttl int) error                        { return c.pconn.SetTTL(ttl) }
func (c *ipv4TTLConn) SetDeadline(t time.Time) error               { return c.raw.SetDeadline(t) }
func (c *ipv4TTLConn) WriteTo(b []byte, dst net.Addr) (int, error) { return c.raw.WriteTo(b, dst) }
func (c *ipv4TTLConn) ReadFrom(b []byte) (int, net.Addr, error)    { return c.raw.ReadFrom(b) }
func (c *ipv4TTLConn) Close() error                                { return c.raw.Close() }
func (c *ipv4TTLConn) DrainErrors(visit func(probe.QueuedError)) error {
	return c.raw.DrainErrors(visit)
}

type ipv6TTLConn struct {
	raw   probeSocket
	pconn *ipv6.PacketConn
}

func (c *ipv6TTLConn) Identifier() uint16                          { return c.raw.Identifier() }
func (c *ipv6TTLConn) SetTTL(ttl int) error                        { return c.pconn.SetHopLimit(ttl) }
func (c *ipv6TTLConn) SetDeadline(t time.Time) error               { return c.raw.SetDeadline(t) }
func (c *ipv6TTLConn) WriteTo(b []byte, dst net.Addr) (int, error) { return c.raw.WriteTo(b, dst) }
func (c *ipv6TTLConn) ReadFrom(b []byte) (int, net.Addr, error)    { return c.raw.ReadFrom(b) }
func (c *ipv6TTLConn) Close() error                                { return c.raw.Close() }
func (c *ipv6TTLConn) DrainErrors(visit func(probe.QueuedError)) error {
	return c.raw.DrainErrors(visit)
}

// icmpPortUnreach is Code 3 for IPv4, Code 4 for IPv6. icmpv4FragNeeded is
// the Destination Unreachable code a router answers a DF datagram it cannot
// forward with; the IPv6 answer, Packet Too Big, is its own type (2) and
// falls through the trace loop's default arm.
const (
	icmpv4PortUnreach = 3
	icmpv6PortUnreach = 4
	icmpv4FragNeeded  = 4
)

// readErrorsMax bounds the trace loop's tolerance for consecutive read
// errors that are neither the deadline nor a refusal of the probe in hand.
// Under IP_RECVERR a queued error makes one ordinary read fail with its
// errno, once; a descriptor failing on every read would otherwise keep the
// loop spinning inside one probe's deadline.
const readErrorsMax = 32

// refusedHop is what the trace loop learned about a probe the path refused
// for its size: the router that answered and the value it reported. A
// probe the kernel refused at send names no router.
type refusedHop struct {
	addr  string
	entry *probe.QueuedError
}

// matchingRefusal drains the error queue and returns the entry that refused
// the probe (echoID, seq) for its size, or nil when no such entry was queued or
// the queue could not be read: either way the hop carries no reported value,
// which is what the loop wrote before the queue existed.
// The quoted identifier and sequence are matched before the value is
// believed, because the kernel hands an unconnected raw socket every ICMP
// error quoting an ICMP datagram from this host.
func matchingRefusal(conn ttlSetter, echoID, seq uint16) *probe.QueuedError {
	var match *probe.QueuedError
	drainErr := conn.DrainErrors(func(q probe.QueuedError) {
		if match != nil {
			return
		}
		if q.Errno != syscall.EMSGSIZE {
			return
		}
		if !q.Echo.Present {
			return
		}
		if q.Echo.ID != echoID {
			return
		}
		if q.Echo.Seq != seq {
			return
		}
		entry := q
		match = &entry
	})
	if drainErr != nil {
		return nil
	}
	return match
}

// localRefusal drains the error queue and returns the entry the kernel
// queued for a send it refused against its cached path MTU, or nil when
// none was queued. It is called right after WriteTo failed with EMSGSIZE,
// before anything else reads the queue.
func localRefusal(conn ttlSetter) *probe.QueuedError {
	var local *probe.QueuedError
	drainErr := conn.DrainErrors(func(q probe.QueuedError) {
		if local != nil {
			return
		}
		if !q.Local {
			return
		}
		entry := q
		local = &entry
	})
	if drainErr != nil {
		return nil
	}
	return local
}

// writeRefusedHop adds the two next-hop MTU keys to a hop the path or the
// cache refused. A refusal that carried no usable value writes reported=false
// and no MTU key: a zero is never written as a value.
func writeRefusedHop(hop map[string]any, entry *probe.QueuedError) {
	hop[probe.FieldNextHopMTUReported] = false
	if entry == nil {
		return
	}
	if entry.Outcome != probe.ErrQueueMTUReported {
		return
	}
	hop[probe.FieldNextHopMTUReported] = true
	hop[probe.FieldNextHopMTU] = int(entry.MTU)
}

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

// tracerouteOpts carries the optional arguments of one trace. source is the
// local address the socket binds to, and the zero Addr leaves it to the
// kernel. df is the Don't Fragment mode: every constructor names it, because
// the zero mode is refused by the probe layer rather than read as off.
type tracerouteOpts struct {
	source netip.Addr
	df     probe.DFMode
}

// openProbeConn is the one socket construction every prober calls, in a
// variable so a test can stand a recorder in its place and reach the
// constructor through the real command path without CAP_NET_RAW.
var openProbeConn = func(ctx context.Context, family probe.Family, bind netip.Addr, df probe.DFMode) (probeSocket, error) {
	sock, err := probe.OpenICMP(ctx, family, bind, df)
	if err != nil {
		return nil, err
	}
	return sock, nil
}

func doTraceroute(dest netip.Addr, maxHops int, timeout time.Duration, probes int, opts tracerouteOpts) ([]map[string]any, error) {
	return doTracerouteCtx(context.Background(), dest, maxHops, timeout, probes, opts)
}

func doTracerouteCtx(ctx context.Context, dest netip.Addr, maxHops int, timeout time.Duration, probes int, opts tracerouteOpts) ([]map[string]any, error) {
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

	rawConn, err := openRawProbeConn(ctx, probe.FamilyOf(dest), opts.source, opts.df)
	if err != nil {
		return nil, fmt.Errorf("traceroute: %w", err)
	}

	conn := newTTLConn(rawConn, isV6)
	defer func() { _ = conn.Close() }()

	echoID := conn.Identifier()
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
		var refused *refusedHop

		for p := range probes {
			seq := uint16((ttl-1)*probes + p)
			pkt := probe.BuildICMPEcho(icmpEcho, echoID, seq, []byte("ze-trace"))

			start := time.Now()
			if deadlineErr := conn.SetDeadline(start.Add(timeout)); deadlineErr != nil {
				return nil, fmt.Errorf("traceroute: set deadline: %w", deadlineErr)
			}

			_, writeErr := conn.WriteTo(pkt, dst)
			if writeErr != nil {
				if errors.Is(writeErr, syscall.EMSGSIZE) {
					// The kernel refused the send against its cached path
					// MTU, which honor-cache asks for. Nothing reached the
					// wire, so no hop can answer: the hop carries the cached
					// estimate and the trace ends here.
					refused = &refusedHop{addr: "*", entry: localRefusal(conn)}
					break
				}
				return nil, fmt.Errorf("traceroute: write: %w", writeErr)
			}

			readErrors := 0
			for {
				n, from, readErr := conn.ReadFrom(rb)
				if readErr != nil {
					if errors.Is(readErr, os.ErrDeadlineExceeded) {
						break
					}
					if errors.Is(readErr, net.ErrClosed) {
						break
					}
					// Under IP_RECVERR a queued refusal makes the read fail
					// once with its errno: the queue holds the answer.
					if entry := matchingRefusal(conn, echoID, seq); entry != nil {
						rtt := time.Since(start)
						r := float64(rtt.Microseconds()) / 1000.0
						bestRTT = &r
						addr := "*"
						if entry.Offender.IsValid() {
							addr = entry.Offender.String()
						}
						refused = &refusedHop{addr: addr, entry: entry}
						break
					}
					readErrors++
					if readErrors > readErrorsMax {
						break
					}
					continue
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
						if embID != echoID || embSeq != seq {
							continue
						}
					}
					if fragNeededUnderDF(msgType, rb[1], isV6, opts.df) {
						// The raw socket also receives the Fragmentation
						// Needed datagram itself. Under a DF mode the kernel
						// queues the same answer with the reported MTU, so
						// the loop keeps reading and lets the queue record
						// the hop rather than recording it here without one.
						continue
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
					if replyID != echoID || replySeq != seq {
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
			if refused != nil {
				break
			}
		}

		if refused != nil {
			bestAddr = refused.addr
		}
		hop := map[string]any{
			fieldTTL: ttl,
			"addr":   bestAddr,
		}
		if bestRTT != nil {
			hop["rtt-ms"] = *bestRTT
		} else {
			hop["rtt-ms"] = nil
		}
		if refused != nil {
			writeRefusedHop(hop, refused.entry)
		}
		hops = append(hops, hop)

		if reached {
			break
		}
		if refused != nil {
			// A probe the path refused for its size never passes that hop,
			// and a larger TTL is answered by the same router the same way.
			break
		}
	}
	return hops, nil
}

// fragNeededUnderDF says whether an ICMP error datagram the raw socket read
// is an IPv4 Fragmentation Needed answer to a probe sent with the DF bit.
// The IPv6 Packet Too Big is its own type and never reaches the arm that
// asks this.
func fragNeededUnderDF(msgType, code byte, isV6 bool, df probe.DFMode) bool {
	if isV6 {
		return false
	}
	if msgType != 3 {
		return false
	}
	if code != icmpv4FragNeeded {
		return false
	}
	return df != probe.DFOff
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
