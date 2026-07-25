package as112

import (
	"net"
	"net/netip"
	"sync"
	"testing"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/metrics"
)

// recordingRegistry is a metrics.Registry that sums operations per metric
// name, so a test can assert that a metric was actually updated. Mirrors
// internal/plugins/geodns/metrics_record_test.go's recordingRegistry.
type recordingRegistry struct {
	mu  sync.Mutex
	val map[string]float64
}

func newRecordingRegistry() *recordingRegistry { return &recordingRegistry{val: map[string]float64{}} }

func (r *recordingRegistry) add(name string, v float64) {
	r.mu.Lock()
	r.val[name] += v
	r.mu.Unlock()
}

func (r *recordingRegistry) get(name string) float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.val[name]
}

func (r *recordingRegistry) Counter(name, _ string) metrics.Counter { return &recMetric{r, name} }
func (r *recordingRegistry) Gauge(name, _ string) metrics.Gauge     { return &recMetric{r, name} }
func (r *recordingRegistry) Histogram(name, _ string, _ []float64) metrics.Histogram {
	return &recMetric{r, name}
}
func (r *recordingRegistry) CounterVec(name, _ string, _ []string) metrics.CounterVec {
	return &recCounterVec{r, name}
}
func (r *recordingRegistry) GaugeVec(name, _ string, _ []string) metrics.GaugeVec {
	return &recGaugeVec{r, name}
}
func (r *recordingRegistry) HistogramVec(name, _ string, _ []float64, _ []string) metrics.HistogramVec {
	return &recHistogramVec{r, name}
}

type recMetric struct {
	r    *recordingRegistry
	name string
}

func (m *recMetric) Inc()            { m.r.add(m.name, 1) }
func (m *recMetric) Add(v float64)   { m.r.add(m.name, v) }
func (m *recMetric) Dec()            { m.r.add(m.name, -1) }
func (m *recMetric) Set(v float64)   { m.r.val[m.name] = v } //nolint:unused // Gauge interface
func (m *recMetric) Observe(float64) { m.r.add(m.name, 1) }

type recCounterVec struct {
	r    *recordingRegistry
	name string
}

func (v *recCounterVec) With(...string) metrics.Counter { return &recMetric{v.r, v.name} }
func (v *recCounterVec) Delete(...string) bool          { return false }

type recGaugeVec struct {
	r    *recordingRegistry
	name string
}

func (v *recGaugeVec) With(...string) metrics.Gauge { return &recMetric{v.r, v.name} }
func (v *recGaugeVec) Delete(...string) bool        { return false }

type recHistogramVec struct {
	r    *recordingRegistry
	name string
}

func (v *recHistogramVec) With(...string) metrics.Histogram { return &recMetric{v.r, v.name} }
func (v *recHistogramVec) Delete(...string) bool            { return false }

// fakePeer is a minimal dnsserver.Peer for tests.
type fakePeer struct{ addr net.Addr }

func (p fakePeer) RemoteAddr() net.Addr { return p.addr }

func udpPeer(ip string) fakePeer {
	return fakePeer{addr: &net.UDPAddr{IP: net.ParseIP(ip), Port: 53000}}
}

// VALIDATES: AC-6 / R-3 -- every response has RecursionAvailable=false, and
// answerQuery never issues an upstream query (no recursion capability exists
// in this plugin's dependency graph -- structurally enforced by the harness,
// this test asserts the observable header bit).
func TestAS112NeverRecurses(t *testing.T) {
	resetAS112State(t)
	storeState(buildState(as112Config{Enabled: true}, 1))

	r := new(dns.Msg)
	r.SetQuestion("1.0.10.in-addr.arpa.", dns.TypePTR)
	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.RecursionAvailable = false // harness sets this; asserted here as the invariant under test

	send := answerQuery(msg, r, udpPeer("203.0.113.1"))
	if !send {
		t.Fatal("answerQuery returned send=false, want true")
	}
	if msg.RecursionAvailable {
		t.Fatal("RecursionAvailable = true, want false")
	}
}

// VALIDATES: AC-14 -- empty/unset allow-from answers every source.
func TestAllowFrom_EmptyAnswersAll(t *testing.T) {
	resetAS112State(t)
	storeState(buildState(as112Config{Enabled: true}, 1))

	r := new(dns.Msg)
	r.SetQuestion("1.0.10.in-addr.arpa.", dns.TypePTR)
	msg := new(dns.Msg)
	msg.SetReply(r)

	send := answerQuery(msg, r, udpPeer("203.0.113.1"))
	if !send {
		t.Fatal("send = false, want true (empty allow-from answers every source)")
	}
	if len(msg.Ns) == 0 {
		t.Fatal("expected SOA in Authority, got none")
	}
}

// VALIDATES: AC-15 -- allow-from set: in-range answered, out-of-range dropped
// (no WriteMsg -- send=false) and the denied counter increments.
func TestAllowFrom_DropsOutOfRange(t *testing.T) {
	resetAS112State(t)
	rec := newRecordingRegistry()
	setMetricsRegistry(rec)
	t.Cleanup(func() { setMetricsRegistry(metrics.NopRegistry{}) })

	cfg := as112Config{Enabled: true, AllowFrom: mustPrefixes(t, "10.0.0.0/8")}
	storeState(buildState(cfg, 1))

	r := new(dns.Msg)
	r.SetQuestion("1.0.10.in-addr.arpa.", dns.TypePTR)

	msgIn := new(dns.Msg)
	msgIn.SetReply(r)
	if send := answerQuery(msgIn, r, udpPeer("10.1.2.3")); !send {
		t.Fatal("in-range source dropped, want answered")
	}

	before := rec.get("ze_as112_dns_denied_total")
	msgOut := new(dns.Msg)
	msgOut.SetReply(r)
	if send := answerQuery(msgOut, r, udpPeer("203.0.113.5")); send {
		t.Fatal("out-of-range source answered, want dropped (send=false)")
	}
	if after := rec.get("ze_as112_dns_denied_total"); after != before+1 {
		t.Fatalf("denied counter = %v, want %v (incremented by 1)", after, before+1)
	}
}

// VALIDATES: ze_as112_dns_request_total counts every received request,
// matching its own registered description ("DNS requests received, by zone
// and query type") -- including one allow-from drops, not just answered
// ones. A request that never reaches requestTotal is invisible in "requests
// received" while visible in deniedTotal, which understates traffic seen by
// a locked-down node (e.g. a scan/attack from an out-of-range source).
func TestRequestTotal_CountsAllowFromDenials(t *testing.T) {
	resetAS112State(t)
	rec := newRecordingRegistry()
	setMetricsRegistry(rec)
	t.Cleanup(func() { setMetricsRegistry(metrics.NopRegistry{}) })

	cfg := as112Config{Enabled: true, AllowFrom: mustPrefixes(t, "10.0.0.0/8")}
	storeState(buildState(cfg, 1))

	r := new(dns.Msg)
	r.SetQuestion("1.0.10.in-addr.arpa.", dns.TypePTR)

	before := rec.get("ze_as112_dns_request_total")
	msg := new(dns.Msg)
	msg.SetReply(r)
	if send := answerQuery(msg, r, udpPeer("203.0.113.5")); send {
		t.Fatal("out-of-range source answered, want dropped (send=false)")
	}
	if after := rec.get("ze_as112_dns_request_total"); after != before+1 {
		t.Fatalf("request counter after a denied query = %v, want %v (denied queries are still received requests)", after, before+1)
	}
}

// VALIDATES: ze_as112_dns_request_total and ze_as112_dns_response_total both
// increment for a normal answered query (the counters' primary, documented
// purpose), and the latency histogram records an observation.
func TestMetrics_IncrementOnAnsweredQuery(t *testing.T) {
	resetAS112State(t)
	rec := newRecordingRegistry()
	setMetricsRegistry(rec)
	t.Cleanup(func() { setMetricsRegistry(metrics.NopRegistry{}) })
	storeState(buildState(as112Config{Enabled: true}, 1))

	r := new(dns.Msg)
	r.SetQuestion("1.0.10.in-addr.arpa.", dns.TypePTR)
	msg := new(dns.Msg)
	msg.SetReply(r)

	if send := answerQuery(msg, r, udpPeer("203.0.113.1")); !send {
		t.Fatal("answerQuery returned send=false, want true")
	}
	if got := rec.get("ze_as112_dns_request_total"); got != 1 {
		t.Fatalf("request counter = %v, want 1", got)
	}
	if got := rec.get("ze_as112_dns_response_total"); got != 1 {
		t.Fatalf("response counter = %v, want 1", got)
	}
	if got := rec.get("ze_as112_dns_request_latency_milliseconds"); got != 1 {
		t.Fatalf("latency histogram observations = %v, want 1", got)
	}
}

// VALIDATES: onListenerChange (server.go) publishes 1/0 to the listenerUp
// gauge on bind/unbind, the harness's OnListenerChange contract.
func TestOnListenerChange_SetsListenerUpGauge(t *testing.T) {
	rec := newRecordingRegistry()
	setMetricsRegistry(rec)
	t.Cleanup(func() { setMetricsRegistry(metrics.NopRegistry{}) })

	onListenerChange("udp", "192.175.48.1", true)
	if got := rec.get("ze_as112_listener_up"); got != 1 {
		t.Fatalf("listenerUp after bind = %v, want 1", got)
	}

	onListenerChange("udp", "192.175.48.1", false)
	if got := rec.get("ze_as112_listener_up"); got != 0 {
		t.Fatalf("listenerUp after unbind = %v, want 0", got)
	}
}

// VALIDATES: AC-16 -- loopback/on-box source is always permitted even when
// allow-from does not include it (H1/M4 carve-out for the healthcheck probe).
func TestAllowFrom_LoopbackAlwaysPermitted(t *testing.T) {
	resetAS112State(t)
	cfg := as112Config{Enabled: true, AllowFrom: mustPrefixes(t, "203.0.113.0/24")}
	storeState(buildState(cfg, 1))

	r := new(dns.Msg)
	r.SetQuestion("1.0.10.in-addr.arpa.", dns.TypePTR)
	msg := new(dns.Msg)
	msg.SetReply(r)

	if send := answerQuery(msg, r, udpPeer("127.0.0.1")); !send {
		t.Fatal("loopback source dropped despite not being in allow-from, want always permitted")
	}
}

// VALIDATES: H1/M4's on-box carve-out also covers the healthcheck probe
// querying a real anycast service address (not just loopback) from the same
// box. The probe (finding H1) is explicitly designed to target an anycast
// address rather than loopback, since a loopback-only probe would report UP
// even when the anycast path itself is unreachable. Whether the kernel
// presents 127.0.0.1 or the destination anycast address as the query's
// source is architecture/routing-dependent and unverified here -- isOnBox
// must recognize EITHER so the carve-out holds regardless.
func TestAllowFrom_AnycastAddressSelfQueryAlwaysPermitted(t *testing.T) {
	resetAS112State(t)
	cfg := as112Config{Enabled: true, AllowFrom: mustPrefixes(t, "203.0.113.0/24")}
	storeState(buildState(cfg, 1))

	r := new(dns.Msg)
	r.SetQuestion("1.0.10.in-addr.arpa.", dns.TypePTR)

	for _, addr := range []string{
		anycastV4DirectDelegationAddr,
		anycastV4DNAMERedirectionAddr,
		anycastV6DirectDelegationAddr,
		anycastV6DNAMERedirectionAddr,
	} {
		t.Run(addr, func(t *testing.T) {
			msg := new(dns.Msg)
			msg.SetReply(r)
			if send := answerQuery(msg, r, udpPeer(addr)); !send {
				t.Fatalf("self-query from own anycast address %s dropped despite not being in allow-from, want always permitted (H1/M4)", addr)
			}
		})
	}
}

func mustPrefixes(t *testing.T, cidrs ...string) []netip.Prefix {
	t.Helper()
	out := make([]netip.Prefix, 0, len(cidrs))
	for _, c := range cidrs {
		p, err := netip.ParsePrefix(c)
		if err != nil {
			t.Fatalf("parse prefix %q: %v", c, err)
		}
		out = append(out, p)
	}
	return out
}

func resetAS112State(t *testing.T) {
	t.Helper()
	storeState(nil)
	t.Cleanup(func() { storeState(nil) })
}
