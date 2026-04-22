// Design: plan/spec-l2tp-10-metrics.md -- Prometheus metrics exposure
// Related: observer.go -- CQM ring buffers that feed histogram/gauge metrics
// Related: subsystem.go -- Start/Stop wires metrics registration and poller

package l2tp

import (
	"context"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

// l2tpMetrics holds registered Prometheus metrics for the L2TP subsystem.
// Populated once by bindL2TPMetrics; subsequent reads go through l2tpMetricsPtr.
type l2tpMetrics struct {
	sessionsActive    metrics.Gauge
	sessionsStarting  metrics.Gauge
	sessionsFinishing metrics.Gauge
	tunnelsActive     metrics.Gauge

	sessionState        metrics.GaugeVec
	sessionUptimeSecs   metrics.GaugeVec
	sessionRxBytesTotal metrics.CounterVec
	sessionTxBytesTotal metrics.CounterVec
	sessionRxPktsTotal  metrics.CounterVec
	sessionTxPktsTotal  metrics.CounterVec

	lcpEchoRTTSecs   metrics.HistogramVec
	lcpEchoLossRatio metrics.GaugeVec
	bucketState      metrics.GaugeVec
}

var sessionLabels = []string{"sid", "ifname", "username", "ip", "caller_id"}

var echoRTTBuckets = []float64{
	0.001, 0.002, 0.005,
	0.01, 0.02, 0.05,
	0.1, 0.2, 0.5,
	1.0, 2.0, 5.0,
}

var l2tpMetricsPtr atomic.Pointer[l2tpMetrics]

func bindL2TPMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	m := &l2tpMetrics{
		sessionsActive:    reg.Gauge("ze_l2tp_sessions_active", "Number of L2TP sessions in established state."),
		sessionsStarting:  reg.Gauge("ze_l2tp_sessions_starting", "Number of L2TP sessions in negotiation."),
		sessionsFinishing: reg.Gauge("ze_l2tp_sessions_finishing", "Number of L2TP sessions being torn down."),
		tunnelsActive:     reg.Gauge("ze_l2tp_tunnels_active", "Number of active L2TP tunnels."),

		sessionState:        reg.GaugeVec("ze_l2tp_session_state", "Session FSM state as integer (idle=0, wait-tunnel=1, wait-reply=2, wait-connect=3, wait-cs-answer=4, established=5).", sessionLabels),
		sessionUptimeSecs:   reg.GaugeVec("ze_l2tp_session_uptime_seconds", "Seconds since session creation.", sessionLabels),
		sessionRxBytesTotal: reg.CounterVec("ze_l2tp_session_rx_bytes_total", "Total bytes received on session pppN interface.", sessionLabels),
		sessionTxBytesTotal: reg.CounterVec("ze_l2tp_session_tx_bytes_total", "Total bytes transmitted on session pppN interface.", sessionLabels),
		sessionRxPktsTotal:  reg.CounterVec("ze_l2tp_session_rx_packets_total", "Total packets received on session pppN interface.", sessionLabels),
		sessionTxPktsTotal:  reg.CounterVec("ze_l2tp_session_tx_packets_total", "Total packets transmitted on session pppN interface.", sessionLabels),

		lcpEchoRTTSecs:   reg.HistogramVec("ze_l2tp_lcp_echo_rtt_seconds", "LCP echo round-trip time in seconds.", echoRTTBuckets, []string{"username"}),
		lcpEchoLossRatio: reg.GaugeVec("ze_l2tp_lcp_echo_loss_ratio", "Current 100s bucket echo loss ratio per login.", []string{"username"}),
		bucketState:      reg.GaugeVec("ze_l2tp_bucket_state", "Current CQM bucket state per login (established=0, negotiating=1, down=2).", []string{"username"}),
	}
	l2tpMetricsPtr.Store(m)
}

// observeCQMBucket records CQM metrics from a finalized bucket.
// Called from Observer.maybeCloseBucket.
func observeCQMBucket(username string, bucket CQMBucket, expectedEchoes uint16) {
	m := l2tpMetricsPtr.Load()
	if m == nil {
		return
	}
	if bucket.EchoCount > 0 {
		m.lcpEchoRTTSecs.With(username).Observe(bucket.AvgRTT().Seconds())
	}
	if expectedEchoes > 0 {
		var lost uint16
		if bucket.EchoCount < expectedEchoes {
			lost = expectedEchoes - bucket.EchoCount
		}
		m.lcpEchoLossRatio.With(username).Set(float64(lost) / float64(expectedEchoes))
	}
	m.bucketState.With(username).Set(float64(bucket.State))
}

// deleteLoginMetrics removes all per-login CQM series.
// Called from Observer.evictLRU.
func deleteLoginMetrics(username string) {
	m := l2tpMetricsPtr.Load()
	if m == nil {
		return
	}
	m.lcpEchoRTTSecs.Delete(username)
	m.lcpEchoLossRatio.Delete(username)
	m.bucketState.Delete(username)
}

// -- Stats poller --

// l2tpStatsPoller periodically reads pppN interface counters and updates
// Prometheus metrics. Owns per-session series lifecycle: creates on first
// poll, deletes when the session disappears from snapshots.
type l2tpStatsPoller struct {
	reactors []*L2TPReactor
	interval time.Duration
	cancel   context.CancelFunc
	done     chan struct{}

	prev     map[string]ifaceSnapshot
	prevKeys map[string][5]string // pppN -> label set, for deletion on disappearance
}

type ifaceSnapshot struct {
	rxBytes   uint64
	rxPackets uint64
	txBytes   uint64
	txPackets uint64
}

var getStatsFn = iface.GetStats

func newL2TPStatsPoller(reactors []*L2TPReactor, interval time.Duration) *l2tpStatsPoller {
	return &l2tpStatsPoller{
		reactors: reactors,
		interval: interval,
		prev:     make(map[string]ifaceSnapshot),
		prevKeys: make(map[string][5]string),
	}
}

func (p *l2tpStatsPoller) start() {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.done = make(chan struct{})
	go p.run(ctx)
}

func (p *l2tpStatsPoller) stop() {
	if p.cancel != nil {
		p.cancel()
		<-p.done
	}
}

func (p *l2tpStatsPoller) run(ctx context.Context) {
	defer close(p.done)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		p.poll()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (p *l2tpStatsPoller) poll() {
	m := l2tpMetricsPtr.Load()
	if m == nil {
		return
	}

	seen := make(map[string]struct{})
	now := time.Now()

	var tunnelCount, sessionEstablished, sessionNegotiating, sessionFinishing int

	for _, reactor := range p.reactors {
		snap := reactor.Snapshot()
		tunnelCount += snap.TunnelCount
		for ti := range snap.Tunnels {
			t := &snap.Tunnels[ti]
			for si := range t.Sessions {
				s := &t.Sessions[si]
				switch L2TPSessionState(s.StateNum) {
				case L2TPSessionEstablished:
					sessionEstablished++
				case L2TPSessionIdle:
					sessionFinishing++
				default:
					sessionNegotiating++
				}

				if s.PppInterface == "" {
					continue
				}

				sid := formatSID(s.LocalSID)
				ip := ""
				if s.AssignedAddr.IsValid() {
					ip = s.AssignedAddr.String()
				}
				labels := [5]string{sid, s.PppInterface, s.Username, ip, ""}

				m.sessionState.With(labels[0], labels[1], labels[2], labels[3], labels[4]).Set(float64(s.StateNum))
				m.sessionUptimeSecs.With(labels[0], labels[1], labels[2], labels[3], labels[4]).Set(now.Sub(s.CreatedAt).Seconds())

				seen[s.PppInterface] = struct{}{}
				p.prevKeys[s.PppInterface] = labels

				stats, err := getStatsFn(s.PppInterface)
				if err != nil {
					continue
				}

				prev := p.prev[s.PppInterface]
				addCounterDelta(m.sessionRxBytesTotal, labels, prev.rxBytes, stats.RxBytes)
				addCounterDelta(m.sessionTxBytesTotal, labels, prev.txBytes, stats.TxBytes)
				addCounterDelta(m.sessionRxPktsTotal, labels, prev.rxPackets, stats.RxPackets)
				addCounterDelta(m.sessionTxPktsTotal, labels, prev.txPackets, stats.TxPackets)

				p.prev[s.PppInterface] = ifaceSnapshot{
					rxBytes:   stats.RxBytes,
					rxPackets: stats.RxPackets,
					txBytes:   stats.TxBytes,
					txPackets: stats.TxPackets,
				}
			}
		}
	}

	m.tunnelsActive.Set(float64(tunnelCount))
	m.sessionsActive.Set(float64(sessionEstablished))
	m.sessionsStarting.Set(float64(sessionNegotiating))
	m.sessionsFinishing.Set(float64(sessionFinishing))

	for name, labels := range p.prevKeys {
		if _, ok := seen[name]; !ok {
			deleteSessionSeries(m, labels)
			delete(p.prev, name)
			delete(p.prevKeys, name)
		}
	}
}

func deleteSessionSeries(m *l2tpMetrics, labels [5]string) {
	m.sessionState.Delete(labels[0], labels[1], labels[2], labels[3], labels[4])
	m.sessionUptimeSecs.Delete(labels[0], labels[1], labels[2], labels[3], labels[4])
	m.sessionRxBytesTotal.Delete(labels[0], labels[1], labels[2], labels[3], labels[4])
	m.sessionTxBytesTotal.Delete(labels[0], labels[1], labels[2], labels[3], labels[4])
	m.sessionRxPktsTotal.Delete(labels[0], labels[1], labels[2], labels[3], labels[4])
	m.sessionTxPktsTotal.Delete(labels[0], labels[1], labels[2], labels[3], labels[4])
}

func addCounterDelta(cv metrics.CounterVec, labels [5]string, prev, curr uint64) {
	var delta uint64
	if curr >= prev {
		delta = curr - prev
	} else {
		delta = curr
	}
	if delta > 0 {
		cv.With(labels[0], labels[1], labels[2], labels[3], labels[4]).Add(float64(delta))
	}
}

func formatSID(sid uint16) string {
	buf := make([]byte, 0, 5)
	return string(appendUint16(buf, sid))
}

func appendUint16(buf []byte, v uint16) []byte {
	if v == 0 {
		return append(buf, '0')
	}
	var digits [5]byte
	pos := len(digits)
	for v > 0 {
		pos--
		digits[pos] = byte('0' + v%10)
		v /= 10
	}
	return append(buf, digits[pos:]...)
}

func parsePollInterval() time.Duration {
	const defaultInterval = 30 * time.Second
	raw := env.Get("ze.l2tp.metrics.poll-interval")
	if raw == "" {
		return defaultInterval
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return defaultInterval
	}
	return d
}
