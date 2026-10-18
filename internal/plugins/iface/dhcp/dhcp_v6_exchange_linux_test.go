// Design: docs/features/interfaces.md -- DHCPv6 client lifecycle
// Related: dhcp_v6_lease_linux_test.go -- the rig these tests drive

//go:build linux

package ifacedhcp

import (
	"context"
	"encoding/binary"
	"log/slog"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/iana"

	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/test/sim"
)

// TestRFC9915Section15RetransmissionTimeout checks the RT formulas over the
// whole RAND range.
//
// VALIDATES: AC-5 -- the first RT is within IRT +-10%, each next RT within
// +-10% of twice the previous, and RT past MRT is MRT +-10%.
func TestRFC9915Section15RetransmissionTimeout(t *testing.T) {
	p := v6Retransmission{irt: v6RenTimeout, mrt: v6RenMaxRT}
	for _, unit := range []float64{0, 0.25, 0.5, 0.75, 0.999} {
		rt := v6FirstRT(p, unit)
		if rt < 9*time.Second || rt > 11*time.Second {
			t.Errorf("unit %v: first RT %v outside 10s +-10%%", unit, rt)
		}
		for range 12 {
			rnd := (unit*2 - 1) * 0.1
			next := v6NextRT(rt, p.mrt, rnd)
			low, high := v6Scale(rt, 1.9), v6Scale(rt, 2.1)
			capped := next >= v6Scale(p.mrt, 0.9) && next <= v6Scale(p.mrt, 1.1)
			if !capped && (next < low || next > high) {
				t.Errorf("unit %v: RT %v after %v, want 2*RTprev +-10%% or MRT +-10%%", unit, next, rt)
			}
			rt = next
		}
		if rt < v6Scale(p.mrt, 0.9) || rt > v6Scale(p.mrt, 1.1) {
			t.Errorf("unit %v: RT settled at %v, want MRT 600s +-10%%", unit, rt)
		}
	}
}

// TestRFC9915Section15ElapsedTimeUpdated retransmits a Solicit no server
// answers.
//
// VALIDATES: AC-5 -- the xid is unchanged within the exchange, the Elapsed
// Time is the hundredths of a second since the first transmission, and it
// saturates at 0xffff.
// PREVENTS: D-5, identical bytes retransmitted with Elapsed Time 0.
func TestRFC9915Section15ElapsedTimeUpdated(t *testing.T) {
	r := newV6Rig(t, dHCPConfig{})
	r.startWorker()
	r.runUntil(seconds(1000))
	solicits := r.sentOf(dhcpv6.MessageTypeSolicit)
	if len(solicits) < 5 {
		t.Fatalf("%d SOLICIT in 1000s, want retransmissions", len(solicits))
	}
	for _, s := range solicits {
		if s.msg.TransactionID != solicits[0].msg.TransactionID {
			t.Errorf("SOLICIT at %v has xid %v, want %v", s.at, s.msg.TransactionID, solicits[0].msg.TransactionID)
		}
		want := v6ElapsedTime(s.at - solicits[0].at)
		if got := s.msg.Options.ElapsedTime(); got != want {
			t.Errorf("SOLICIT at %v: Elapsed Time %v, want %v", s.at, got, want)
		}
	}
	if got := v6ElapsedTime(700 * time.Second); got != 655350*time.Millisecond {
		t.Errorf("Elapsed Time for 700s = %v, want the 0xffff ceiling 655.35s", got)
	}
}

// TestRFC9915Section1821FirstSolicitRTAboveIRT pins RAND at the middle of
// its range.
//
// VALIDATES: AC-6 -- the first Solicit is delayed by RAND*SOL_MAX_DELAY, its
// RT is above IRT, and the Option Request option carries SOL_MAX_RT (82).
// PREVENTS: D-11.
func TestRFC9915Section1821FirstSolicitRTAboveIRT(t *testing.T) {
	r := newV6Rig(t, dHCPConfig{})
	r.client.v6Random = func() float64 { return 0.5 }
	r.startWorker()
	r.runUntil(seconds(3))
	solicits := r.sentOf(dhcpv6.MessageTypeSolicit)
	if len(solicits) < 2 {
		t.Fatalf("SOLICIT at %v, want two", sentTimes(solicits))
	}
	if solicits[0].at != seconds(0.5) {
		t.Errorf("first SOLICIT at %v, want the 0.5s delay", solicits[0].at)
	}
	if rt := solicits[1].at - solicits[0].at; rt <= v6SolTimeout {
		t.Errorf("first RT %v, want strictly above 1s", rt)
	}
	if oro := solicits[0].msg.Options.RequestedOptions(); !slices.Contains(oro, dhcpv6.OptionSolMaxRT) {
		t.Errorf("Solicit ORO %v lacks SOL_MAX_RT", oro)
	}
}

// TestDHCPv6SolicitBacksOffToSolMaxRT runs five hours with no server, then
// lets one answer.
//
// VALIDATES: AC-6 -- RT grows to SOL_MAX_RT and stays there, never stopping.
// AC-8 -- exactly one Warn for the outage, one Info once the lease is held.
// PREVENTS: D-4, a Warn every 45 s forever.
func TestDHCPv6SolicitBacksOffToSolMaxRT(t *testing.T) {
	r := newV6Rig(t, dHCPConfig{})
	r.startWorker()
	r.runUntil(5 * time.Hour)
	times := sentTimes(r.sentOf(dhcpv6.MessageTypeSolicit))
	if len(times) < 10 {
		t.Fatalf("%d SOLICIT in 5h", len(times))
	}
	last := times[len(times)-1] - times[len(times)-2]
	if last < v6Scale(v6SolMaxRT, 0.9) || last > v6Scale(v6SolMaxRT, 1.1) {
		t.Errorf("RT settled at %v, want SOL_MAX_RT 3600s +-10%%", last)
	}
	if got := r.logs.count(slog.LevelWarn, ""); got != 1 {
		t.Errorf("%d Warn lines over the outage, want 1", got)
	}
	r.srv.mu.Lock()
	r.srv.serve = leaseServer(lease10x16x30, true, false)
	r.srv.mu.Unlock()
	r.runUntil(5*time.Hour + v6Scale(v6SolMaxRT, 1.2))
	if got := r.logs.count(slog.LevelInfo, "lease obtained"); got != 1 {
		t.Errorf("%d 'lease obtained' Info lines, want 1", got)
	}
	if got := r.logs.count(slog.LevelWarn, ""); got != 1 {
		t.Errorf("%d Warn lines after recovery, want 1", got)
	}
}

func solMaxRTOption(seconds uint32) dhcpv6.Option {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, seconds)
	return &dhcpv6.OptionGeneric{OptionCode: dhcpv6.OptionSolMaxRT, OptionData: data}
}

func testWorker(t *testing.T, cfg dHCPConfig) *v6Worker {
	t.Helper()
	c, err := newDHCPClient("eth0", "0", &recordingBus{}, false, true, cfg, sim.NewFakeClock(time.Unix(0, 0)))
	if err != nil {
		t.Fatalf("newDHCPClient: %v", err)
	}
	return &v6Worker{
		c:        c,
		ctx:      context.Background(),
		duid:     &dhcpv6.DUIDLL{HWType: iana.HWTypeEthernet, LinkLayerAddr: v6TestMAC},
		iaidNA:   v6ClientIAID,
		iaidPD:   v6ClientPDIAI,
		solMaxRT: v6SolMaxRT,
	}
}

func testWorkerAnswer(w *v6Worker, kind dhcpv6.MessageType, opts ...dhcpv6.Option) *dhcpv6.Message {
	req := &dhcpv6.Message{MessageType: dhcpv6.MessageTypeRenew}
	req.AddOption(dhcpv6.OptClientID(w.duid))
	return testV6Answer(req, kind, v6ServerA, opts...)
}

// TestRFC9915Section2124SolMaxRTRange offers SOL_MAX_RT values at and past
// both bounds.
//
// VALIDATES: AC-7 -- 60 and 86400 replace SOL_MAX_RT, 59 and 86401 are
// ignored, and a Reply with a failure status is still read for it.
func TestRFC9915Section2124SolMaxRTRange(t *testing.T) {
	for _, tc := range []struct {
		value uint32
		want  time.Duration
	}{{60, 60 * time.Second}, {86400, 86400 * time.Second}, {59, v6SolMaxRT}, {86401, v6SolMaxRT}} {
		w := testWorker(t, dHCPConfig{})
		failure := testWorkerAnswer(w, dhcpv6.MessageTypeReply, solMaxRTOption(tc.value),
			&dhcpv6.OptStatusCode{StatusCode: iana.StatusUnspecFail})
		if w.replyMatcher()(failure) {
			t.Errorf("SOL_MAX_RT %d: a failure Reply was accepted as the answer", tc.value)
		}
		if w.solMaxRT != tc.want {
			t.Errorf("SOL_MAX_RT %d: internal value %v, want %v", tc.value, w.solMaxRT, tc.want)
		}
	}
}

// TestRFC9915Section1610ReplyValidation offers the four kinds of Reply.
//
// VALIDATES: AC-18 -- a Reply without Server ID and one with a foreign
// Client ID are discarded; one with an unknown option is taken; an unknown
// message type is discarded.
// PREVENTS: D-10, any message with the right xid taken.
func TestRFC9915Section1610ReplyValidation(t *testing.T) {
	w := testWorker(t, dHCPConfig{})
	accept := w.replyMatcher()

	noServer := &dhcpv6.Message{MessageType: dhcpv6.MessageTypeReply}
	noServer.AddOption(dhcpv6.OptClientID(w.duid))
	if accept(noServer) {
		t.Error("Reply without Server ID accepted")
	}
	foreign := &dhcpv6.Message{MessageType: dhcpv6.MessageTypeReply}
	foreign.AddOption(dhcpv6.OptServerID(v6ServerA))
	foreign.AddOption(dhcpv6.OptClientID(v6ServerB))
	if accept(foreign) {
		t.Error("Reply with a foreign Client ID accepted")
	}
	unknown := testWorkerAnswer(w, dhcpv6.MessageTypeReply, &dhcpv6.OptionGeneric{OptionCode: 65000, OptionData: []byte{1}})
	if !accept(unknown) {
		t.Error("Reply with an unknown option discarded")
	}
	if accept(testWorkerAnswer(w, dhcpv6.MessageType(250))) {
		t.Error("unknown message type accepted")
	}
}

// TestRFC9915Section141PerInterfaceRateLimit sends 20 messages at one
// instant and asks for a 21st.
//
// VALIDATES: AC-19 -- the 21st waits until the first leaves the 20 s window.
func TestRFC9915Section141PerInterfaceRateLimit(t *testing.T) {
	w := testWorker(t, dHCPConfig{})
	clk, ok := w.c.clock.(*sim.FakeClock)
	if !ok {
		t.Fatal("test worker has no fake clock")
	}
	start := clk.Now()
	for i := range v6RateMessages {
		if !w.admitTransmission(context.Background()) {
			t.Fatalf("transmission %d refused", i)
		}
	}
	admitted := make(chan time.Time, 1)
	go w.admitAsync(admitted)
	if !clk.AwaitTimers(1, 2*time.Second) {
		t.Fatal("the 21st transmission did not wait")
	}
	next, _ := clk.NextTimer()
	if next != start.Add(v6RateWindow) {
		t.Errorf("21st transmission waits until %v, want %v", next.Sub(start), v6RateWindow)
	}
	clk.Set(next)
	if at := <-admitted; at.Sub(start) != v6RateWindow {
		t.Errorf("21st transmission at %v, want %v", at.Sub(start), v6RateWindow)
	}
}

func (w *v6Worker) admitAsync(admitted chan<- time.Time) {
	w.admitTransmission(context.Background())
	admitted <- w.c.clock.Now()
}

// TestRFC9915Section1821AdvertiseSelection feeds the Advertise collector.
//
// VALIDATES: AC-22 -- of preferences 10 and 200 the 200 is chosen when the
// first RT ends; a 255 is taken at once; a Reply without Rapid Commit is
// discarded.
func TestRFC9915Section1821AdvertiseSelection(t *testing.T) {
	w := testWorker(t, dHCPConfig{})
	pref := func(v byte) dhcpv6.Option {
		return &dhcpv6.OptionGeneric{OptionCode: dhcpv6.OptionPreference, OptionData: []byte{v}}
	}
	lease := lease10x16x30()[0]
	a := &v6AdvertiseCollector{w: w}
	low := testWorkerAnswer(w, dhcpv6.MessageTypeAdvertise, lease, pref(10))
	high := testWorkerAnswer(w, dhcpv6.MessageTypeAdvertise, lease, pref(200))
	if a.accept(low) || a.accept(high) {
		t.Fatal("an Advertise below 255 ended the first RT")
	}
	if got := a.windowClosed(); got != high {
		t.Errorf("chosen Advertise preference %d, want 200", v6Preference(got))
	}
	b := &v6AdvertiseCollector{w: w}
	if !b.accept(testWorkerAnswer(w, dhcpv6.MessageTypeAdvertise, lease, pref(255))) {
		t.Error("preference 255 not taken at once")
	}
	if b.accept(testWorkerAnswer(w, dhcpv6.MessageTypeReply, lease)) {
		t.Error("Reply without Rapid Commit accepted")
	}
}

// TestRFC9915Section1822RequestGivesUpAfterMRC answers Solicit with an
// Advertise and never answers Request.
//
// VALIDATES: AC-23 -- REQ_MAX_RC (10) Requests, then a new Solicit.
func TestRFC9915Section1822RequestGivesUpAfterMRC(t *testing.T) {
	r := newV6Rig(t, dHCPConfig{})
	r.srv.serve = func(req *dhcpv6.Message) []*dhcpv6.Message {
		if req.MessageType != dhcpv6.MessageTypeSolicit {
			return nil
		}
		return []*dhcpv6.Message{testV6Answer(req, dhcpv6.MessageTypeAdvertise, v6ServerA, lease10x16x30()...)}
	}
	r.startWorker()
	r.runUntil(seconds(200))
	sent := r.sent()
	requests := 0
	for _, s := range sent {
		if s.msg.MessageType == dhcpv6.MessageTypeSolicit && requests > 0 {
			break
		}
		if s.msg.MessageType == dhcpv6.MessageTypeRequest {
			requests++
		}
	}
	if requests != v6ReqMaxRC {
		t.Errorf("%d Requests before the next Solicit, want %d; sent %s", requests, v6ReqMaxRC, r.describeSent())
	}
	if got := len(r.sentOf(dhcpv6.MessageTypeSolicit)); got < 2 {
		t.Errorf("%d Solicit, want a new exchange after the Request failed", got)
	}
}

// TestDHCPv6PDLengthRequestsDelegatedPrefix builds every message with and
// without the pd container.
//
// VALIDATES: AC-25 -- with pd length 56 each Solicit, Request, Renew and
// Rebind carries an IA_PD whose IAID differs from the IA_NA's and an IA
// Prefix hint ::/56; pd with no length sends an IA_PD with no hint; no pd
// sends no IA_PD.
// PREVENTS: D-13, the pd leaf reaching no message.
func TestDHCPv6PDLengthRequestsDelegatedPrefix(t *testing.T) {
	build := func(w *v6Worker) []*dhcpv6.Message {
		t.Helper()
		solicit, err := w.buildSolicit()
		if err != nil {
			t.Fatal(err)
		}
		advertise := testWorkerAnswer(w, dhcpv6.MessageTypeAdvertise, lease10x16x30()...)
		request, err := w.buildRequest(advertise)
		if err != nil {
			t.Fatal(err)
		}
		lease := &v6Lease{server: v6ServerA}
		lease.apply(advertise, time.Unix(0, 0), w.iaidNA, w.iaidPD)
		out := []*dhcpv6.Message{solicit, request}
		for _, kind := range []dhcpv6.MessageType{dhcpv6.MessageTypeRenew, dhcpv6.MessageTypeRebind} {
			m, err := w.buildFromLease(kind, lease, time.Unix(1, 0))
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, m)
		}
		return out
	}
	for _, m := range build(testWorker(t, dHCPConfig{PDRequested: true, PDLength: 56})) {
		pd := m.Options.OneIAPD()
		if pd == nil {
			t.Errorf("%s carries no IA_PD", m.MessageType)
			continue
		}
		if na := m.Options.OneIANA(); na == nil || na.IaId == pd.IaId {
			t.Errorf("%s IA_PD IAID %v not distinct from the IA_NA's", m.MessageType, pd.IaId)
		}
		prefixes := pd.Options.Prefixes()
		if len(prefixes) != 1 {
			t.Errorf("%s IA_PD prefixes %v, want one hint", m.MessageType, prefixes)
			continue
		}
		ones, _ := prefixes[0].Prefix.Mask.Size()
		if ones != 56 || !prefixes[0].Prefix.IP.Equal(net.IPv6unspecified) {
			t.Errorf("%s hint %v, want ::/56", m.MessageType, prefixes[0].Prefix)
		}
	}
	for _, m := range build(testWorker(t, dHCPConfig{PDRequested: true})) {
		pd := m.Options.OneIAPD()
		if pd == nil || len(pd.Options.Prefixes()) != 0 {
			t.Errorf("%s: pd with no length sends IA_PD %v, want one with no hint", m.MessageType, pd)
		}
	}
	for _, m := range build(testWorker(t, dHCPConfig{})) {
		if pd := m.Options.OneIAPD(); pd != nil {
			t.Errorf("%s carries IA_PD %v with no pd configured", m.MessageType, pd)
		}
	}
}

var _ clock.Clock = (*sim.FakeClock)(nil)
