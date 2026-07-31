// Design: plan/learned/745-ipsec-10-cli-diag.md -- IPsec Prometheus metrics

package engine

import (
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
// It must not assert a non-zero absolute (ai/rules/fail-closed-guards.md).
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
func RegisterMetrics(reg metrics.Registry) *IPsecMetrics {
	return &IPsecMetrics{
		saCount:        reg.Gauge("ze_ipsec_sa_count", "Number of active IKE Security Associations"),
		tunnelUp:       reg.GaugeVec("ze_ipsec_tunnel_up", "Whether a peer tunnel is established and carries ESP (1=up, 0=down or degraded)", []string{"peer"}),
		tunnelDegraded: reg.GaugeVec("ze_ipsec_tunnel_degraded", "Whether a peer IKE SA is established but its Child SA carries no ESP (1=degraded)", []string{"peer"}),
		rekeyTotal:     reg.GaugeVec("ze_ipsec_rekey_total", "Cumulative child SA rekey count per peer", []string{"peer"}),
		errorNotifySent: reg.GaugeVec("ze_ipsec_error_notify_sent_total",
			"Cumulative error notifications sent, by notify type and whether the carrying message was encrypted",
			[]string{"type", "protected"}),
		errorNotifySuppressed: reg.GaugeVec("ze_ipsec_error_notify_suppressed_total",
			"Cumulative error notifications a guard stopped, by the name of the guard",
			[]string{"reason"}),
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

// espInstalled reports whether this peer's current Child SA is in the dataplane.
//
// It fails closed. An unknown peer, a session without a Child SA, and a Child SA
// whose install was refused all read false (ai/rules/fail-closed-guards.md).
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
	table := ActiveTable()
	if table == nil {
		m.saCount.Set(0)
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
}
