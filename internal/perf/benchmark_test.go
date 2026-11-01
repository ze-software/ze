package perf_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/perf"
)

// VALIDATES: AC-6 "le perf send with -sender-first, and without it: the result
// JSON carries "pass": "rib-propagation", and "pass": "live-forwarding",
// respectively".
// METHOD: run RunBenchmark against the in-process test forwarder once per
// pass, write each result as the NDJSON line `le perf send --output` writes,
// and read the "pass" key back out of the raw JSON object.
// PREVENTS: a sender-first result that is indistinguishable from a
// live-forwarding one, which let the comparison show the propagation pass
// under the bare DUT name.
func TestRunResultRecordsPass(t *testing.T) {
	cases := []struct {
		name        string
		senderFirst bool
		want        perf.Pass
		wantJSON    string
	}{
		{"live forwarding", false, perf.PassLiveForwarding, "live-forwarding"},
		{"sender first", true, perf.PassRIBPropagation, "rib-propagation"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()

			fwd := perf.NewTestForwarder(t)
			go fwd.Run(ctx)

			host, portText, err := net.SplitHostPort(fwd.Addr())
			if err != nil {
				t.Fatalf("splitting forwarder address %q: %v", fwd.Addr(), err)
			}
			port, err := strconv.Atoi(portText)
			if err != nil {
				t.Fatalf("parsing port %q: %v", portText, err)
			}

			cfg := perf.BenchmarkConfig{
				DUTAddr:        host,
				DUTPort:        port,
				DUTASN:         65000,
				DUTName:        "test-forwarder",
				SenderAddr:     "127.0.0.1",
				SenderASN:      65001,
				ReceiverAddr:   "127.0.0.1",
				ReceiverASN:    65002,
				Routes:         10,
				Family:         "ipv4/unicast",
				Seed:           42,
				Warmup:         100 * time.Millisecond,
				ConnectTimeout: 5 * time.Second,
				Duration:       10 * time.Second,
				Repeat:         1,
				SenderFirst:    tc.senderFirst,
			}

			result, err := perf.RunBenchmark(ctx, cfg, io.Discard)
			if err != nil {
				t.Fatalf("RunBenchmark: %v", err)
			}
			if result.Pass != tc.want {
				t.Errorf("Result.Pass = %v, want %v", result.Pass, tc.want)
			}

			var line bytes.Buffer
			if err := perf.WriteNDJSON(&line, result); err != nil {
				t.Fatalf("WriteNDJSON: %v", err)
			}
			var object map[string]any
			if err := json.Unmarshal(line.Bytes(), &object); err != nil {
				t.Fatalf("the result line does not parse: %v\n%s", err, line.String())
			}
			if got := object["pass"]; got != tc.wantJSON {
				t.Errorf(`result JSON "pass" = %v, want %q: %s`, got, tc.wantJSON, strings.TrimSpace(line.String()))
			}

			cancel()
		})
	}
}

// VALIDATES: the security review row "Result JSON with an unknown pass value
// is refused with the value and the accepted values"; a missing field reads
// as PassUnrecorded and a recorded value reads back as itself.
// METHOD: unmarshal result objects carrying each shape of the "pass" key.
// PREVENTS: a mistyped or future pass value read silently as a known pass or
// as no pass.
func TestResultPassJSON(t *testing.T) {
	var missing perf.Result
	if err := json.Unmarshal([]byte(`{"dut-name":"ze"}`), &missing); err != nil {
		t.Fatalf("a result with no pass field: %v", err)
	}
	if missing.Pass != perf.PassUnrecorded {
		t.Errorf("missing pass field read as %v, want %v", missing.Pass, perf.PassUnrecorded)
	}
	if got, want := missing.Pass.String(), "not recorded"; got != want {
		t.Errorf("PassUnrecorded.String() = %q, want %q", got, want)
	}

	var recorded perf.Result
	if err := json.Unmarshal([]byte(`{"dut-name":"ze","pass":"rib-propagation"}`), &recorded); err != nil {
		t.Fatalf("a result with a known pass: %v", err)
	}
	if recorded.Pass != perf.PassRIBPropagation {
		t.Errorf("pass read as %v, want %v", recorded.Pass, perf.PassRIBPropagation)
	}

	for _, value := range []string{`"sender-first"`, `""`} {
		var unknown perf.Result
		err := json.Unmarshal([]byte(`{"dut-name":"ze","pass":`+value+`}`), &unknown)
		if err == nil {
			t.Fatalf("pass %s was accepted, want it refused", value)
		}
		for _, want := range []string{value, "live-forwarding", "rib-propagation"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("refusal of pass %s = %q, lacks %q", value, err.Error(), want)
			}
		}
	}

	data, err := json.Marshal(perf.Result{DUTName: "ze"})
	if err != nil {
		t.Fatalf("marshal a result with no pass: %v", err)
	}
	if strings.Contains(string(data), `"pass"`) {
		t.Errorf("a result with no pass wrote a pass key: %s", data)
	}
}

// VALIDATES: only a named pass has a result file name.
// METHOD: ask each pass value, named and unnamed, for its file name.
// PREVENTS: PassUnrecorded or a stray value silently given "<dut>.json", the
// live-forwarding name.
func TestPassResultFileRefusesAnUnnamedPass(t *testing.T) {
	for _, pass := range []perf.Pass{perf.PassUnrecorded, perf.Pass(200)} {
		name, err := pass.ResultFile("ze")
		if err == nil {
			t.Errorf("%v.ResultFile(ze) = %q, want a refusal", pass, name)
			continue
		}
		for _, want := range []string{"live-forwarding", "rib-propagation"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("refusal %q lacks %q", err.Error(), want)
			}
		}
	}
	for pass, want := range map[perf.Pass]string{perf.PassLiveForwarding: "ze.json", perf.PassRIBPropagation: "ze-propagation.json"} {
		if got, err := pass.ResultFile("ze"); err != nil || got != want {
			t.Errorf("%v.ResultFile(ze) = %q, %v, want %q", pass, got, err, want)
		}
	}
}

// VALIDATES: le perf send --passive-listen completes the BGP handshake on the
// receiver session the DUT opened, in both passes, and measures every route.
// METHOD: a fake DUT that behaves as rustbgpd does: it dials both le perf
// listeners, sends its OPEN at once, and resets a session whose OPEN does not
// arrive within its hold time with NOTIFICATION 5/0. It forwards UPDATEs from
// one session to the other only once both are established. Two iterations
// run, so the DUT reconnects once.
// PREVENTS: the passive-listen receiver never sent an OPEN (since 1427dc96cb),
// so the DUT reset the session and the warmup failed with "receiving updates:
// receive: reading header: EOF" in the live-forwarding pass, and the
// rib-propagation pass read from a receiver that was never set up.
func TestRunPassiveListenHandshakesReceiver(t *testing.T) {
	cases := []struct {
		name        string
		senderFirst bool
		want        perf.Pass
	}{
		{"receiver first", false, perf.PassLiveForwarding},
		{"sender first", true, perf.PassRIBPropagation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()

			senderPort := freeLoopbackPort(t)
			receiverPort := freeLoopbackPort(t)

			dut := &dialingDUT{
				t:            t,
				senderAddr:   net.JoinHostPort("127.0.0.1", strconv.Itoa(senderPort)),
				receiverAddr: net.JoinHostPort("127.0.0.1", strconv.Itoa(receiverPort)),
				openWait:     3 * time.Second,
			}

			var dutWg sync.WaitGroup
			dutWg.Go(func() { dut.run(ctx) })
			defer dutWg.Wait()
			defer cancel()

			cfg := perf.WithPassiveListenPorts(perf.BenchmarkConfig{
				DUTAddr:        "127.0.0.1",
				DUTPort:        179,
				DUTASN:         65000,
				DUTName:        "passive-dut",
				SenderAddr:     "127.0.0.1",
				SenderASN:      65001,
				ReceiverAddr:   "127.0.0.1",
				ReceiverASN:    65002,
				Routes:         10,
				Family:         "ipv4/unicast",
				Seed:           42,
				Warmup:         100 * time.Millisecond,
				ConnectTimeout: 5 * time.Second,
				Duration:       10 * time.Second,
				Repeat:         1,
				WarmupRuns:     1,
				PassiveListen:  true,
				SenderFirst:    tc.senderFirst,
			}, senderPort, receiverPort)

			result, err := perf.RunBenchmark(ctx, cfg, io.Discard)
			if err != nil {
				t.Fatalf("RunBenchmark --passive-listen: %v", err)
			}
			if result.RoutesReceived != cfg.Routes {
				t.Errorf("routes received = %d, want %d (lost %d)", result.RoutesReceived, cfg.Routes, result.RoutesLost)
			}
			if result.Pass != tc.want {
				t.Errorf("Result.Pass = %v, want %v", result.Pass, tc.want)
			}
		})
	}
}

// freeLoopbackPort returns a TCP port on 127.0.0.1 that was free a moment ago.
func freeLoopbackPort(t *testing.T) int {
	t.Helper()

	var lc net.ListenConfig

	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for a free port: %v", err)
	}

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address %v is not TCP", ln.Addr())
	}

	if err := ln.Close(); err != nil {
		t.Fatalf("closing the free-port listener: %v", err)
	}

	return addr.Port
}

// dialingDUT is a fake DUT that opens both BGP sessions itself, as a DUT
// configured to connect to its peers does when le perf runs with
// --passive-listen. Each session sends its OPEN as soon as TCP is up and waits
// openWait for the peer's OPEN; a session that gets none is reset with
// NOTIFICATION 5/0 (Finite State Machine Error), as rustbgpd does. UPDATEs
// read on one session are written to the other once that one is established,
// so a sender-first run finds every route held for the receiver.
//
// run redials after both sessions close, once per benchmark iteration, until
// ctx is canceled. Not safe for concurrent use: one run per value.
type dialingDUT struct {
	t            *testing.T
	senderAddr   string
	receiverAddr string
	openWait     time.Duration
}

// dutOpenConfig is the OPEN the fake DUT sends on both sessions.
var dutOpenConfig = perf.SessionConfig{
	ASN:      65000,
	RouterID: netip.MustParseAddr("10.0.0.1"),
	HoldTime: 90,
	Family:   "ipv4/unicast",
}

// run serves one pair of sessions per iteration until ctx is canceled.
func (d *dialingDUT) run(ctx context.Context) {
	for ctx.Err() == nil {
		d.runPair(ctx)
	}
}

// runPair dials both listeners, handshakes each session, forwards UPDATEs
// between them, and returns once both sessions have closed.
func (d *dialingDUT) runPair(ctx context.Context) {
	var conns [2]net.Conn
	var dialWg sync.WaitGroup
	for i, addr := range []string{d.senderAddr, d.receiverAddr} {
		dialWg.Go(func() { conns[i] = dialUntilAccepted(ctx, addr) })
	}
	dialWg.Wait()

	for _, conn := range conns {
		if conn == nil {
			for _, c := range conns {
				if c != nil {
					_ = c.Close()
				}
			}
			return
		}
	}

	pairCtx, pairCancel := context.WithCancel(ctx)
	defer pairCancel()

	established := [2]chan struct{}{make(chan struct{}), make(chan struct{})}

	var sessionWg sync.WaitGroup
	for i := range conns {
		sessionWg.Go(func() {
			defer conns[i].Close() //nolint:errcheck // session end
			other := 1 - i
			if err := d.handshake(conns[i]); err != nil {
				if ctx.Err() == nil {
					d.t.Logf("fake DUT session %s: %v", conns[i].LocalAddr(), err)
				}
				// The peer never answered: no route can reach it, so the
				// other session ends too, as a DUT tearing down the pair.
				// pairCancel releases the other session's forwarder if it is
				// holding an UPDATE for this one.
				pairCancel()
				_ = conns[other].Close()
				return
			}
			close(established[i])
			d.forward(pairCtx, conns[i], conns[other], established[other])
		})
	}
	sessionWg.Wait()
}

// dialUntilAccepted dials addr every 20ms until a listener accepts, or returns
// nil once ctx is canceled.
func dialUntilAccepted(ctx context.Context, addr string) net.Conn {
	var dialer net.Dialer
	for ctx.Err() == nil {
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err == nil {
			return conn
		}
		select {
		case <-ctx.Done():
		case <-time.After(20 * time.Millisecond):
		}
	}
	return nil
}

// handshake sends the DUT's OPEN at once, then waits openWait for the peer's
// OPEN and resets the session with NOTIFICATION 5/0 when it does not come.
func (d *dialingDUT) handshake(conn net.Conn) error {
	open, err := perf.BuildOpen(dutOpenConfig)
	if err != nil {
		return fmt.Errorf("building OPEN: %w", err)
	}
	if err := perf.WriteMessage(conn, open); err != nil {
		return fmt.Errorf("sending OPEN: %w", err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(d.openWait)); err != nil {
		return fmt.Errorf("setting the OPEN wait: %w", err)
	}

	msgType, _, err := perf.ReadMessage(conn)
	if ne, ok := errors.AsType[net.Error](err); ok && ne.Timeout() {
		_ = perf.WriteMessage(conn, fsmErrorNotification)
		return fmt.Errorf("no OPEN within %v, sent NOTIFICATION 5/0", d.openWait)
	}
	if err != nil {
		return fmt.Errorf("reading peer OPEN: %w", err)
	}
	if msgType != msgtype.TypeOPEN {
		return fmt.Errorf("expected OPEN, got type %d", msgType)
	}

	if err := perf.WriteMessage(conn, keepaliveMessage); err != nil {
		return fmt.Errorf("sending KEEPALIVE: %w", err)
	}

	msgType, _, err = perf.ReadMessage(conn)
	if err != nil {
		return fmt.Errorf("reading peer KEEPALIVE: %w", err)
	}
	if msgType != msgtype.TypeKEEPALIVE {
		return fmt.Errorf("expected KEEPALIVE, got type %d", msgType)
	}

	return conn.SetReadDeadline(time.Time{})
}

// forward copies each UPDATE read from source to target, waiting until the
// target session is established, and returns when source closes.
func (d *dialingDUT) forward(ctx context.Context, source, target net.Conn, targetUp <-chan struct{}) {
	for {
		msgType, msg, err := perf.ReadMessage(source)
		if err != nil {
			return
		}
		if msgType != msgtype.TypeUPDATE {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-targetUp:
		}
		if err := perf.WriteMessage(target, msg); err != nil {
			return
		}
	}
}

// keepaliveMessage is a BGP KEEPALIVE: the 19-octet header alone (RFC 4271
// Section 4.4).
var keepaliveMessage = bgpMessage(msgtype.TypeKEEPALIVE)

// fsmErrorNotification is a NOTIFICATION with error code 5 (Finite State
// Machine Error) and subcode 0 (RFC 4271 Section 4.5, RFC 6608 Section 3).
var fsmErrorNotification = bgpMessage(msgtype.TypeNOTIFICATION, 5, 0)

// bgpMessage builds a BGP message of the given type around body: a marker of
// 16 octets of ones, the total length, then the type (RFC 4271 Section 4.1).
func bgpMessage(msgType msgtype.MessageType, body ...byte) []byte {
	msg := make([]byte, 19, 19+len(body))
	for i := range 16 {
		msg[i] = 0xff
	}
	msg[16] = byte((19 + len(body)) >> 8)
	msg[17] = byte(19 + len(body))
	msg[18] = byte(msgType)
	return append(msg, body...)
}
