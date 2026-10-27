package dnsquery

import (
	"bytes"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// recorder is a loopback DNS server that keeps the first query it read, with
// the address it came from, and answers every query with every header bit
// set, one A record and an OPT.
type recorder struct {
	addr string
	last chan recorded
}

// recorded is one query the recorder read and its source address.
type recorded struct {
	msg    *dns.Msg
	source netip.Addr
}

func startRecorder(t *testing.T, network, addr string) *recorder {
	t.Helper()
	rec := &recorder{last: make(chan recorded, 1)}
	handler := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		source, perr := netip.ParseAddrPort(w.RemoteAddr().String())
		if perr != nil {
			t.Errorf("remote address: %v", perr)
			return
		}
		// A retried query finds the slot taken; the test reads the first.
		select {
		case rec.last <- recorded{msg: r, source: source.Addr()}:
		default:
		}
		m := new(dns.Msg)
		m.SetReply(r)
		m.Authoritative = true
		m.Truncated = true
		m.RecursionAvailable = true
		m.AuthenticatedData = true
		m.CheckingDisabled = true
		m.Rcode = dns.RcodeServerFailure
		rr, err := dns.NewRR(r.Question[0].Name + " 300 IN A 192.0.2.7")
		if err != nil {
			t.Errorf("record: %v", err)
			return
		}
		m.Answer = append(m.Answer, rr)
		m.SetEdns0(1400, true)
		_ = w.WriteMsg(m) //nolint:errcheck // the client side reports a missing reply
	})
	started := make(chan struct{})
	srv := &dns.Server{Handler: handler, NotifyStartedFunc: func() { close(started) }}
	switch network {
	case "udp":
		pc, err := new(net.ListenConfig).ListenPacket(t.Context(), "udp", addr)
		if err != nil {
			t.Fatalf("listen udp: %v", err)
		}
		srv.PacketConn = pc
		rec.addr = pc.LocalAddr().String()
	case "tcp":
		ln, err := new(net.ListenConfig).Listen(t.Context(), "tcp", addr)
		if err != nil {
			t.Fatalf("listen tcp: %v", err)
		}
		srv.Listener = ln
		rec.addr = ln.Addr().String()
	}
	go func() { _ = srv.ActivateAndServe() }() //nolint:errcheck // Shutdown below ends it
	<-started
	t.Cleanup(func() { _ = srv.Shutdown() })
	return rec
}

// VALIDATES: le dns query sets each header bit, the OPT size and DO, the
// opcode and the transport its flags name, and prints rcode, every reply
// flag, the size, the OPT and each section's records.
// METHOD: run the command against a loopback server that records the query
// and answers with every bit set; compare the recorded query and the output.
// PREVENTS: a .ci asserting RA, AA or TC on output the tool never derived
// from the reply, or a flag that silently leaves its bit clear.
func TestQueryToolFlags(t *testing.T) {
	for _, network := range []string{"udp", "tcp"} {
		rec := startRecorder(t, network, loopbackAny)
		args := []string{"--server", rec.addr, "--name", "www.example.test", "--type", "AAAA",
			"--rd", "--cd", "--ad", "--do", "--edns", "1300"}
		if network == "tcp" {
			args = append(args, "--tcp")
		}
		var out, errOut bytes.Buffer
		if code := run(args, &out, &errOut); code != 0 {
			t.Fatalf("%s: exit %d, stderr %q", network, code, errOut.String())
		}
		q := rec.received(t)
		if !q.RecursionDesired || !q.CheckingDisabled || !q.AuthenticatedData {
			t.Errorf("%s: query RD=%v CD=%v AD=%v, want all set", network, q.RecursionDesired, q.CheckingDisabled, q.AuthenticatedData)
		}
		opt := q.IsEdns0()
		if opt == nil {
			t.Fatalf("%s: query carries no OPT", network)
		}
		if opt.UDPSize() != 1300 || !opt.Do() {
			t.Errorf("%s: query OPT size %d DO %v, want 1300 and DO set", network, opt.UDPSize(), opt.Do())
		}
		if q.Question[0].Name != "www.example.test." || q.Question[0].Qtype != dns.TypeAAAA {
			t.Errorf("%s: question %v, want www.example.test. AAAA", network, q.Question[0])
		}
		for _, want := range []string{
			"rcode SERVFAIL\n",
			"flags qr aa tc rd ra ad cd\n",
			"size ",
			"edns size 1400 do 1\n",
			"question www.example.test. IN AAAA\n",
			"answer www.example.test. 300 IN A 192.0.2.7\n",
		} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("%s: output lacks %q:\n%s", network, want, out.String())
			}
		}
	}

	rec := startRecorder(t, "udp", loopbackAny)
	var out, errOut bytes.Buffer
	if code := run([]string{"--server", rec.addr, "--name", "plain.example.test", "--no-edns", "--opcode", "4"}, &out, &errOut); code != 0 {
		t.Fatalf("plain: exit %d, stderr %q", code, errOut.String())
	}
	q := rec.received(t)
	if q.RecursionDesired {
		t.Error("query without --rd has RD=1, want 0")
	}
	if q.IsEdns0() != nil {
		t.Error("query with --no-edns carries an OPT")
	}
	if q.Opcode != dns.OpcodeNotify {
		t.Errorf("opcode %d, want 4 (NOTIFY), the one non-QUERY opcode miekg hands a handler", q.Opcode)
	}
}

// VALIDATES: the command refuses a bad command line naming the fix, and
// exits 1 with the reason when no reply arrives.
// METHOD: bad flags, then a query to a bound socket that never answers.
// PREVENTS: a .ci reading exit 0 as an answer the server never gave.
func TestQueryToolRefusals(t *testing.T) {
	for _, args := range [][]string{
		{"--name", "x.test"},
		{"--server", "127.0.0.1"},
		{"--server", "nonsense", "--name", "x.test"},
		{"--server", "127.0.0.1", "--name", "x.test", "--type", "NOPE"},
		{"--server", "127.0.0.1", "--name", "x.test", "--no-edns", "--do"},
		{"--server", "127.0.0.1", "--name", "x.test", "--opcode", "16"},
	} {
		var out, errOut bytes.Buffer
		if code := run(args, &out, &errOut); code != 1 {
			t.Errorf("%v: exit %d, want 1", args, code)
		}
		if !strings.Contains(errOut.String(), "error: ") {
			t.Errorf("%v: stderr %q names no reason", args, errOut.String())
		}
	}

	silent, err := new(net.ListenConfig).ListenPacket(t.Context(), "udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = silent.Close() }()
	var out, errOut bytes.Buffer
	if code := run([]string{"--server", silent.LocalAddr().String(), "--name", "x.test"}, &out, &errOut); code != 1 {
		t.Errorf("silent server: exit %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "no reply from") {
		t.Errorf("silent server: stderr %q, want the no-reply reason", errOut.String())
	}
}

// loopbackAny is a loopback listen address on a port the system picks.
const loopbackAny = "127.0.0.1:0"

// VALIDATES: --source sends the query from the address it names, over UDP
// and over TCP, which is what an ACL test relies on.
// METHOD: query a recorder on 127.0.0.1 from 127.0.0.2 (Linux answers the
// whole of 127.0.0.0/8 on lo) and compare the source the recorder saw.
// PREVENTS: an ACL test passing because every query came from 127.0.0.1.
func TestQueryToolSource(t *testing.T) {
	for _, network := range []string{"udp", "tcp"} {
		rec := startRecorder(t, network, loopbackAny)
		args := []string{"--server", rec.addr, "--name", "src.example.test", "--source", "127.0.0.2"}
		if network == "tcp" {
			args = append(args, "--tcp")
		}
		var out, errOut bytes.Buffer
		if code := run(args, &out, &errOut); code != 0 {
			t.Fatalf("%s: exit %d, stderr %q", network, code, errOut.String())
		}
		got := rec.receivedFrom(t)
		if got != netip.MustParseAddr("127.0.0.2") {
			t.Errorf("%s: query came from %s, want 127.0.0.2", network, got)
		}
	}
}

// VALIDATES: --wait keeps retrying until a server that was not listening
// yet answers, and the answer is printed as on a first-attempt reply.
// METHOD: reserve a UDP port, release it, start the query with --wait 10s,
// and only then bind a recorder on that port.
// PREVENTS: a .ci step that queries a daemon still starting reading the
// first refused attempt as the result.
func TestQueryToolWait(t *testing.T) {
	reserved, err := new(net.ListenConfig).ListenPacket(t.Context(), "udp", loopbackAny)
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := reserved.LocalAddr().String()
	if cerr := reserved.Close(); cerr != nil {
		t.Fatalf("release port: %v", cerr)
	}

	type result struct {
		code int
		out  string
		err  string
	}
	done := make(chan result, 1)
	go func() {
		var out, errOut bytes.Buffer
		code := run([]string{"--server", addr, "--name", "late.example.test", "--wait", "10s"}, &out, &errOut)
		done <- result{code: code, out: out.String(), err: errOut.String()}
	}()
	time.Sleep(500 * time.Millisecond)
	startRecorder(t, "udp", addr)

	select {
	case got := <-done:
		if got.code != 0 {
			t.Fatalf("exit %d, stderr %q, want a reply once the server bound", got.code, got.err)
		}
		if !strings.Contains(got.out, "answer late.example.test. 300 IN A 192.0.2.7\n") {
			t.Errorf("output lacks the late server's answer:\n%s", got.out)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("le dns query --wait 10s did not return")
	}
}

// received returns the query the recorder read, failing the test rather than
// hanging when none arrived.
func (r *recorder) received(t *testing.T) *dns.Msg {
	t.Helper()
	return r.next(t).msg
}

// receivedFrom returns the source address of the query the recorder read.
func (r *recorder) receivedFrom(t *testing.T) netip.Addr {
	t.Helper()
	return r.next(t).source
}

func (r *recorder) next(t *testing.T) recorded {
	t.Helper()
	select {
	case q := <-r.last:
		return q
	case <-time.After(5 * time.Second):
		t.Fatal("the server read no query")
		return recorded{}
	}
}
