// Design: docs/architecture/ike/ipsec-10-cli-diag.md -- IPsec Prometheus metrics
// RFC: rfc/short/rfc7296.md -- error notification and COOKIE counters (Sections 2.6, 2.21.4)

package engine

import (
	"strconv"
	"sync"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/metrics"
)

// IPsecMetrics holds Prometheus metric handles for the IPsec subsystem.
type IPsecMetrics struct {
	saCount               metrics.Gauge
	tunnelUp              metrics.GaugeVec
	tunnelDegraded        metrics.GaugeVec
	rekeyTotal            metrics.GaugeVec
	errorNotifySent       metrics.GaugeVec
	errorNotifySuppressed metrics.GaugeVec
	cookieChallenges      metrics.GaugeVec
	cookieVerifyFailures  metrics.GaugeVec
	saInitRetries         metrics.GaugeVec
	eapTLSResumptionHits  metrics.GaugeVec
	eapTLSResumptionMiss  metrics.GaugeVec

	// dataplaneSACount and dataplaneDrift report what the KERNEL holds, beside
	// the belief gauges above. Both are a GaugeVec even where one label value
	// would do, because a dataplane nobody could read publishes NO series and a
	// plain Gauge cannot be deleted.
	dataplaneSACount metrics.GaugeVec
	dataplaneDrift   metrics.GaugeVec

	// dataplaneMu guards the label sets the two dataplane gauges last published.
	// Update runs on two goroutines: the 5 second metrics ticker, and the SDK
	// dispatch goroutine that applies a configuration (register.go).
	dataplaneMu      sync.Mutex
	dataplaneIfIDs   map[string]bool
	dataplaneDrifted map[string]bool
}

// eapTLSResumptionStats counts how each completed EAP-TLS authentication was
// reached: a resumed TLS session, or a full handshake (RFC 9190 Section 2.1.3).
//
// It records here for the reason errorNotifyStats does: the two call sites are
// free functions on the dispatch and owner paths (handleEAPResponse in fsm.go
// for the peer role, handleEAPRound in responder_eap.go for the authenticator)
// and hold no registry handle. Update publishes the totals.
//
// The pair is what an operator reads to know resumption is working at all. A
// hit count stuck at zero with a rising miss count is the signal that the
// session-resumption leaf is off, that the peer never offers a ticket, or that
// the authenticator refuses every one it is shown.
var eapTLSResumptionStats = struct {
	mu     sync.Mutex
	hits   map[string]uint64
	misses map[string]uint64
}{
	hits:   map[string]uint64{},
	misses: map[string]uint64{},
}

// countEAPTLSAuthentication records one completed EAP-TLS authentication under
// the peer it authenticated, and under whether it resumed.
func countEAPTLSAuthentication(peer string, resumed bool) {
	eapTLSResumptionStats.mu.Lock()
	defer eapTLSResumptionStats.mu.Unlock()
	if resumed {
		eapTLSResumptionStats.hits[peer]++
		return
	}
	eapTLSResumptionStats.misses[peer]++
}

// eapTLSResumptionCounts reports one peer's resumed and full-handshake totals.
//
// It fails closed for the READER, not for a guard: an unrecorded peer reads two
// zeros, which are the true counts. A test that asserts a resumption happened
// must therefore assert a RISE (ai/rules/evidence.md).
func eapTLSResumptionCounts(peer string) (hits, misses uint64) {
	eapTLSResumptionStats.mu.Lock()
	defer eapTLSResumptionStats.mu.Unlock()
	return eapTLSResumptionStats.hits[peer], eapTLSResumptionStats.misses[peer]
}

// errorNotifyStats counts the error notifications this node emits, and the ones a
// guard stops. The emitters (notify_error.go) are free functions on the dispatch and
// owner paths, and they hold no registry handle.
// They therefore record here, and Update publishes the totals.
// RFC 7296 Section 2.21.4 makes the suppression counts operationally interesting.
// A rising rate-limited count is the signature of a forged-packet flood.
var errorNotifyStats = struct {
	mu         sync.Mutex
	sent       map[errorNotifyKey]uint64
	suppressed map[string]uint64
}{
	sent:       map[errorNotifyKey]uint64{},
	suppressed: map[string]uint64{},
}

// errorNotifyKey labels one sent notification by its RFC type and by whether the
// message that carried it was cryptographically protected.
type errorNotifyKey struct {
	notifyType uint16
	protected  bool
}

// cookieStats counts the COOKIE challenges this node issues, the cookies that failed to
// verify, and the IKE_SA_INIT retries it sends.
//
// They record here for the reason errorNotifyStats does: the emitters are free functions
// on the dispatch goroutine (cookie.go, sa_init_retry.go, responder.go) and hold no
// registry handle. Update publishes the totals.
//
// The retry count is the operator's signal for the forged-notify flood RFC 7296
// Section 2.6 describes, where each forged reply costs this node two packets.
var cookieStats = struct {
	mu             sync.Mutex
	challenges     map[string]uint64
	verifyFailures map[string]uint64
	retries        map[saInitRetryKey]uint64
}{
	challenges:     map[string]uint64{},
	verifyFailures: map[string]uint64{},
	retries:        map[saInitRetryKey]uint64{},
}

// saInitRetryKey labels one IKE_SA_INIT retry by peer and by what caused it.
type saInitRetryKey struct {
	peer  string
	cause retryCause
}

// countCookieChallenge records one COOKIE challenge put on the wire.
func countCookieChallenge(peer string) {
	cookieStats.mu.Lock()
	defer cookieStats.mu.Unlock()
	cookieStats.challenges[peer]++
}

// countCookieVerifyFailure records one inbound cookie that did not verify. A rising rate
// is either an attacker probing the half-open slot or a secret rotation catching an
// in-flight challenge.
func countCookieVerifyFailure(peer string) {
	cookieStats.mu.Lock()
	defer cookieStats.mu.Unlock()
	cookieStats.verifyFailures[peer]++
}

// countSAInitRetry records one IKE_SA_INIT retry, by the cause that drove it.
func countSAInitRetry(peer string, cause retryCause) {
	cookieStats.mu.Lock()
	defer cookieStats.mu.Unlock()
	cookieStats.retries[saInitRetryKey{peer: peer, cause: cause}]++
}

// cookieChallengeCount reports how many challenges have gone out to one peer.
//
// It fails closed for the READER, not for a guard: an unrecorded peer reads zero, which
// is the true count. A test that asserts a challenge fired must therefore assert a RISE
// (ai/rules/evidence.md).
func cookieChallengeCount(peer string) uint64 {
	cookieStats.mu.Lock()
	defer cookieStats.mu.Unlock()
	return cookieStats.challenges[peer]
}

// saInitRetryCount reports how many retries of one cause have gone out to one peer.
func saInitRetryCount(peer string, cause retryCause) uint64 {
	cookieStats.mu.Lock()
	defer cookieStats.mu.Unlock()
	return cookieStats.retries[saInitRetryKey{peer: peer, cause: cause}]
}

// countErrorNotifySent records one error notification put on the wire.
func countErrorNotifySent(notifyType uint16, protected bool) {
	errorNotifyStats.mu.Lock()
	defer errorNotifyStats.mu.Unlock()
	errorNotifyStats.sent[errorNotifyKey{notifyType: notifyType, protected: protected}]++
}

// countErrorNotifySuppressed records one error notification a guard stopped, under
// the name of the guard that stopped it.
func countErrorNotifySuppressed(reason string) {
	errorNotifyStats.mu.Lock()
	defer errorNotifyStats.mu.Unlock()
	errorNotifyStats.suppressed[reason]++
}

// errorNotifySuppressedCount reports how many emissions one named guard has stopped.
//
// It fails closed for the reader, not for a guard.
// An unrecorded reason reads zero, which is the true count.
// A test that asserts a guard fired must therefore assert a RISE.
// It must not assert a non-zero absolute (ai/rules/evidence.md).
func errorNotifySuppressedCount(reason string) uint64 {
	errorNotifyStats.mu.Lock()
	defer errorNotifyStats.mu.Unlock()
	return errorNotifyStats.suppressed[reason]
}

// errorNotifySentCount reports how many notifications of one type and protection
// level have gone out.
func errorNotifySentCount(notifyType uint16, protected bool) uint64 {
	errorNotifyStats.mu.Lock()
	defer errorNotifyStats.mu.Unlock()
	return errorNotifyStats.sent[errorNotifyKey{notifyType: notifyType, protected: protected}]
}

// RegisterMetrics creates IPsec metrics on the given registry.
//
// A tunnel counts as up only when its IKE SA is established AND its Child SA is
// installed in the dataplane. An established SA whose ESP install was refused reads
// up 0 and degraded 1. The two gauges therefore separate "no session" from "session
// but no encrypted traffic", which the up gauge alone reported as healthy.
// metricLabelPeer is the Prometheus label whose value is a peer name. It names
// the column, not the peer.
const metricLabelPeer = "peer"

// metricLabelIfID names the XFRM if_id column. The underscore is not a style
// choice: a Prometheus label name matches [a-zA-Z_][a-zA-Z0-9_]*, so the
// kebab-case "if-id" this repository uses for a JSON key would be refused at
// registration and take the daemon down at startup.
const metricLabelIfID = "if_id"

func RegisterMetrics(reg metrics.Registry) *IPsecMetrics {
	return &IPsecMetrics{
		saCount:        reg.Gauge("ze_ipsec_sa_count", "Number of active IKE Security Associations"),
		tunnelUp:       reg.GaugeVec("ze_ipsec_tunnel_up", "Whether a peer tunnel is established and carries ESP (1=up, 0=down or degraded)", []string{metricLabelPeer}),
		tunnelDegraded: reg.GaugeVec("ze_ipsec_tunnel_degraded", "Whether a peer IKE SA is established but its Child SA carries no ESP (1=degraded)", []string{metricLabelPeer}),
		rekeyTotal:     reg.GaugeVec("ze_ipsec_rekey_total", "Cumulative child SA rekey count per peer", []string{metricLabelPeer}),
		errorNotifySent: reg.GaugeVec("ze_ipsec_error_notify_sent_total",
			"Cumulative error notifications sent, by notify type and whether the carrying message was encrypted",
			[]string{"type", "protected"}),
		errorNotifySuppressed: reg.GaugeVec("ze_ipsec_error_notify_suppressed_total",
			"Cumulative error notifications a guard stopped, by the name of the guard",
			[]string{"reason"}),
		cookieChallenges: reg.GaugeVec("ze_ipsec_cookie_challenges_total",
			"Cumulative COOKIE challenges sent to an inbound initiation, by peer",
			[]string{metricLabelPeer}),
		cookieVerifyFailures: reg.GaugeVec("ze_ipsec_cookie_verify_failures_total",
			"Cumulative inbound cookies that did not verify, by peer",
			[]string{metricLabelPeer}),
		saInitRetries: reg.GaugeVec("ze_ipsec_sa_init_retries_total",
			"Cumulative IKE_SA_INIT retries sent, by peer and by the notify that caused them",
			[]string{metricLabelPeer, "cause"}),
		eapTLSResumptionHits: reg.GaugeVec("ze_ipsec_eap_tls_resumption_hits_total",
			"Cumulative EAP-TLS authentications that resumed an earlier TLS session, by peer",
			[]string{metricLabelPeer}),
		eapTLSResumptionMiss: reg.GaugeVec("ze_ipsec_eap_tls_resumption_misses_total",
			"Cumulative EAP-TLS authentications that ran a full TLS handshake, by peer",
			[]string{metricLabelPeer}),
		dataplaneSACount: reg.GaugeVec("ze_ipsec_dataplane_sa_count",
			"Number of IPsec SAs the kernel SAD holds, by XFRM if_id. No series is published when the dataplane cannot be read",
			[]string{metricLabelIfID}),
		dataplaneDrift: reg.GaugeVec("ze_ipsec_dataplane_drift",
			"Whether the kernel SAD is missing a Child SA the IKE engine counts as installed (1=drifting), by peer. No series is published when the dataplane cannot be read",
			[]string{metricLabelPeer}),
		dataplaneIfIDs:   map[string]bool{},
		dataplaneDrifted: map[string]bool{},
	}
}

// publishDataplaneGauges reports what the kernel SAD holds, and where it
// disagrees with what the engine believes it installed.
//
// It takes ONE dump for the pass and feeds both gauges from it. Two dumps would
// be two answers to one question, and they disagree across a rekey, when the old
// and the new Child SA are both alive (RFC 7296 Section 2.8).
//
// An unreadable SAD deletes every series and publishes none. A drift of 0 there
// would say "no drift" on the strength of a question nobody asked, which is the
// false green the dataplane read surface exists to remove (ai/rules/evidence.md).
// Prometheus spells "unknown" as the absence of a series.
//
// Safe for concurrent use.
func (m *IPsecMetrics) publishDataplaneGauges() {
	observation, err := ObserveDataplane()
	if err != nil {
		m.clearDataplaneGauges()
		return
	}
	sas := observation.SAs
	infos := observation.Peers

	counts := make(map[string]float64, len(sas))
	for i := range sas {
		counts[strconv.FormatUint(uint64(sas[i].IfID), 10)]++
	}

	// A peer with a Child SA reads 0 or 1. A peer with none has no belief to
	// contradict, so it gets no series rather than a 0 that reads as agreement.
	// Both the peer set and the comparison come from the validated observation.
	drift := make(map[string]float64, len(infos))
	for name := range infos {
		if infos[name].HasChild {
			drift[name] = 0
		}
	}
	for _, name := range driftingPeersFrom(observation) {
		if _, known := drift[name]; known {
			drift[name] = 1
		}
	}

	m.dataplaneMu.Lock()
	defer m.dataplaneMu.Unlock()
	setGaugeSeries(m.dataplaneSACount, m.dataplaneIfIDs, counts)
	setGaugeSeries(m.dataplaneDrift, m.dataplaneDrifted, drift)
}

// clearDataplaneGauges removes every series the two dataplane gauges published
// and sets none.
//
// Safe for concurrent use.
func (m *IPsecMetrics) clearDataplaneGauges() {
	m.dataplaneMu.Lock()
	defer m.dataplaneMu.Unlock()
	setGaugeSeries(m.dataplaneSACount, m.dataplaneIfIDs, nil)
	setGaugeSeries(m.dataplaneDrift, m.dataplaneDrifted, nil)
}

// setGaugeSeries publishes one pass of a gauge vector and deletes every series
// the pass before it published that this one does not carry. The caller owns
// published and MUST hold the lock that guards it.
//
// A series that is merely left behind is indistinguishable at a scrape from a
// live value, so a label that stops appearing is removed rather than held.
func setGaugeSeries(vec metrics.GaugeVec, published map[string]bool, values map[string]float64) {
	for label := range published {
		if _, ok := values[label]; ok {
			continue
		}
		vec.Delete(label)
		delete(published, label)
	}
	for label, value := range values {
		vec.With(label).Set(value)
		published[label] = true
	}
}

// publishEAPTLSResumptionCounts copies the resumption counters into their gauges.
func (m *IPsecMetrics) publishEAPTLSResumptionCounts() {
	eapTLSResumptionStats.mu.Lock()
	defer eapTLSResumptionStats.mu.Unlock()
	for peer, count := range eapTLSResumptionStats.hits {
		m.eapTLSResumptionHits.With(peer).Set(float64(count))
	}
	for peer, count := range eapTLSResumptionStats.misses {
		m.eapTLSResumptionMiss.With(peer).Set(float64(count))
	}
}

// publishErrorNotifyCounts copies the emitter counters into the two gauges.
func (m *IPsecMetrics) publishErrorNotifyCounts() {
	errorNotifyStats.mu.Lock()
	defer errorNotifyStats.mu.Unlock()
	for key, count := range errorNotifyStats.sent {
		protected := "false"
		if key.protected {
			protected = "true"
		}
		m.errorNotifySent.With(wire.NotifyTypeName(key.notifyType), protected).Set(float64(count))
	}
	for reason, count := range errorNotifyStats.suppressed {
		m.errorNotifySuppressed.With(reason).Set(float64(count))
	}
}

// publishCookieCounts copies the COOKIE and retry counters into their gauges.
func (m *IPsecMetrics) publishCookieCounts() {
	cookieStats.mu.Lock()
	defer cookieStats.mu.Unlock()
	for peer, count := range cookieStats.challenges {
		m.cookieChallenges.With(peer).Set(float64(count))
	}
	for peer, count := range cookieStats.verifyFailures {
		m.cookieVerifyFailures.With(peer).Set(float64(count))
	}
	for key, count := range cookieStats.retries {
		m.saInitRetries.With(key.peer, key.cause.String()).Set(float64(count))
	}
}

// espInstalled reports whether this peer's current Child SA is in the dataplane.
//
// It fails closed. An unknown peer, a session without a Child SA, and a Child SA
// whose install was refused all read false (ai/rules/evidence.md).
func espInstalled(peers map[string]*PeerSession, name string) bool {
	ps, ok := peers[name]
	if !ok || ps == nil {
		return false
	}
	child := ps.getChildSA()
	return child != nil && child.ESPInstalled
}

// Update reads the current SA table and peer session state to refresh all metrics.
func (m *IPsecMetrics) Update() {
	m.publishErrorNotifyCounts()
	m.publishCookieCounts()
	m.publishEAPTLSResumptionCounts()
	// RFC 7296 Section 2.23 needs the kernel to decapsulate ESP that arrives inside
	// UDP. A failure to arm that is invisible on the wire: the tunnel establishes and
	// carries nothing. The count rises on every listener rebuild that fails, so a
	// reader sees a persistent condition rather than one startup line.
	m.errorNotifySuppressed.With("udp-encap-unavailable").Set(float64(udpEncapFailureCount()))
	table := ActiveTable()
	if table == nil {
		m.saCount.Set(0)
		// A daemon that runs no IKE engine has no belief to contradict, and it
		// MUST NOT issue a netlink dump every 5 seconds to learn that. Series an
		// earlier pass published are deleted rather than left to go stale.
		m.clearDataplaneGauges()
		return
	}

	sas := table.All()
	m.saCount.Set(float64(len(sas)))

	infos := PeerInfoMap()
	peers := ActivePeers()
	for name := range infos {
		info := infos[name]
		established := false
		for _, sa := range sas {
			if sa.PeerName == name && sa.State == StateEstablished {
				established = true
				break
			}
		}

		// An established IKE SA whose Child SA is not in the dataplane forwards no
		// encrypted traffic. It is degraded, and it is not up.
		carriesESP := established && espInstalled(peers, name)
		up, degraded := float64(0), float64(0)
		if carriesESP {
			up = 1
		} else if established {
			degraded = 1
		}

		m.tunnelUp.With(name).Set(up)
		m.tunnelDegraded.With(name).Set(degraded)
		m.rekeyTotal.With(name).Set(float64(info.RekeyCount))
	}

	m.publishDataplaneGauges()
}
