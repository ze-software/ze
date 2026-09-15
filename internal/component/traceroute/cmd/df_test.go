// Design: docs/architecture/diagnostics/active-probes.md -- the DF keyword reaches the probe socket

package cmd

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/probe"
)

// deadProbeConn is a net.PacketConn that answers nothing: the traceroute loop
// ends at its first SetTTL, which the ipv4 wrapper cannot perform on a fake.
// The test needs only what the constructor was asked for, which is recorded
// before the loop starts. It is also a net.Conn because ipv4.NewPacketConn
// asserts that on its argument.
type deadProbeConn struct{}

func (deadProbeConn) ReadFrom(_ []byte) (int, net.Addr, error)  { return 0, nil, net.ErrClosed }
func (deadProbeConn) WriteTo(p []byte, _ net.Addr) (int, error) { return len(p), nil }
func (deadProbeConn) Close() error                              { return nil }
func (deadProbeConn) Read(_ []byte) (int, error)                { return 0, net.ErrClosed }
func (deadProbeConn) Write(p []byte) (int, error)               { return len(p), nil }
func (deadProbeConn) RemoteAddr() net.Addr                      { return nil }
func (deadProbeConn) LocalAddr() net.Addr                       { return nil }
func (deadProbeConn) SetDeadline(_ time.Time) error             { return nil }
func (deadProbeConn) SetReadDeadline(_ time.Time) error         { return nil }
func (deadProbeConn) SetWriteDeadline(_ time.Time) error        { return nil }
func (d deadProbeConn) Kind() probe.SocketKind                  { return probe.SocketRaw }
func (deadProbeConn) Identifier() uint16                        { return tracePID() }
func (d deadProbeConn) PacketConn() net.PacketConn              { return d }
func (deadProbeConn) DrainErrors(func(probe.QueuedError)) error { return nil }

// recordingOpener replaces openProbeConn for one test and records the DF mode
// the handler asked the socket constructor for.
func recordingOpener(t *testing.T) *probe.DFMode {
	t.Helper()
	got := new(probe.DFMode)
	realOpener := openProbeConn
	openProbeConn = func(_ context.Context, _ probe.Family, _ netip.Addr, df probe.DFMode) (probeSocket, error) {
		*got = df
		return deadProbeConn{}, nil
	}
	t.Cleanup(func() { openProbeConn = realOpener })
	return got
}

// TestTracerouteDoNotFragmentReachesTheSocketOption is the wiring test for
// `show traceroute <dest> do-not-fragment honor-cache`: the value word
// resolves to probe.DFHonorCache and that mode is what the socket constructor
// receives, through the same constructor ping calls. Before the wiring, the
// keyword was swallowed by the target branch and the constructor saw DFOff.
func TestTracerouteDoNotFragmentReachesTheSocketOption(t *testing.T) {
	got := recordingOpener(t)
	if _, err := handleTraceroute(nil, []string{"192.0.2.1", "max-hops", "1", "probes", "1", "timeout", "1s", probe.DFKeyword, "honor-cache"}); err != nil {
		t.Fatalf("handleTraceroute: %v", err)
	}
	if *got != probe.DFHonorCache {
		t.Fatalf("socket constructor received DF mode %v, want %v", *got, probe.DFHonorCache)
	}
}

// TestTracerouteDoNotFragmentWithoutValueIsRefused proves the handler agrees
// with the RPC layer: the bare keyword, trailing or before another keyword,
// and a word that is not one of the two values are each refused by name
// before any socket opens.
func TestTracerouteDoNotFragmentWithoutValueIsRefused(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"trailing", []string{"192.0.2.1", "max-hops", "1", probe.DFKeyword}, "requires a value"},
		{"before-keyword", []string{"192.0.2.1", probe.DFKeyword, "max-hops", "1"}, "honor-cache or bypass-cache"},
		{"unknown-value", []string{"192.0.2.1", probe.DFKeyword, "always"}, "honor-cache or bypass-cache"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := recordingOpener(t)
			resp, err := handleTraceroute(nil, tc.args)
			if err != nil {
				t.Fatalf("handleTraceroute: %v", err)
			}
			if resp.Status != plugin.StatusError || !strings.Contains(resp.Error, tc.want) {
				t.Fatalf("show traceroute = %q (%s), want an error naming %q", resp.Status, resp.Error, tc.want)
			}
			if *got != probe.DFUnspecified {
				t.Fatalf("socket constructor was reached with DF mode %v, want no socket", *got)
			}
		})
	}
}

// TestTracerouteDoNotFragmentBypassCacheReachesTheSocketOption proves the
// explicit value word after the keyword selects the bypass mode.
func TestTracerouteDoNotFragmentBypassCacheReachesTheSocketOption(t *testing.T) {
	got := recordingOpener(t)
	if _, err := handleTraceroute(nil, []string{"192.0.2.1", "max-hops", "1", "probes", "1", "timeout", "1s", probe.DFKeyword, "bypass-cache"}); err != nil {
		t.Fatalf("handleTraceroute: %v", err)
	}
	if *got != probe.DFBypassCache {
		t.Fatalf("socket constructor received DF mode %v, want %v", *got, probe.DFBypassCache)
	}
}

// TestTracerouteWithoutDoNotFragmentOpensWithDFOff proves AC-1 at the
// constructor: with no keyword the handler asks for DFOff, never the zero mode.
func TestTracerouteWithoutDoNotFragmentOpensWithDFOff(t *testing.T) {
	got := recordingOpener(t)
	if _, err := handleTraceroute(nil, []string{"192.0.2.1", "max-hops", "1", "probes", "1", "timeout", "1s"}); err != nil {
		t.Fatalf("handleTraceroute: %v", err)
	}
	if *got != probe.DFOff {
		t.Fatalf("socket constructor received DF mode %v, want %v", *got, probe.DFOff)
	}
}

// datagramProbeConn is a deadProbeConn the opener reports as the
// unprivileged datagram kind, which no trace loop can run on.
type datagramProbeConn struct {
	deadProbeConn
	closed bool
}

func (d *datagramProbeConn) Kind() probe.SocketKind { return probe.SocketDatagram }
func (d *datagramProbeConn) Close() error           { d.closed = true; return nil }

// TestTracerouteRefusesTheDatagramSocket: the trace loop names CAP_NET_RAW
// and closes the socket rather than running a trace whose every hop would
// time out, because Time Exceeded never reaches the datagram kind.
func TestTracerouteRefusesTheDatagramSocket(t *testing.T) {
	sock := &datagramProbeConn{}
	realOpener := openProbeConn
	openProbeConn = func(context.Context, probe.Family, netip.Addr, probe.DFMode) (probeSocket, error) {
		return sock, nil
	}
	t.Cleanup(func() { openProbeConn = realOpener })

	_, err := doTracerouteCtx(context.Background(), netip.MustParseAddr("192.0.2.1"), 3, time.Second, 1, tracerouteOpts{df: probe.DFOff})
	if !errors.Is(err, errTracerouteNeedsRawSocket) {
		t.Fatalf("error %v, want errTracerouteNeedsRawSocket", err)
	}
	if !sock.closed {
		t.Error("the refused socket was left open")
	}
	if _, roundErr := doProbeRound(netip.MustParseAddr("192.0.2.1"), 3, time.Second); !errors.Is(roundErr, errTracerouteNeedsRawSocket) {
		t.Errorf("show probe-round error %v, want errTracerouteNeedsRawSocket", roundErr)
	}
}
