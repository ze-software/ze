// Design: docs/architecture/ike/ipsec-10-cli-diag.md -- IPsec metrics tests

package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/core/metrics"
)

func TestIPsecMetrics_NoEngine(t *testing.T) {
	activeTablePtr.Store(nil)
	setActivePeers(nil)

	reg := metrics.NopRegistry{}
	m := RegisterMetrics(reg)
	m.Update()
}

// recordingGauge keeps the last value set, so a test reads what an operator scrapes.
type recordingGauge struct {
	name   string
	values map[string]float64
}

func (g *recordingGauge) Set(v float64) { g.values[g.name] = v }
func (g *recordingGauge) Inc()          { g.values[g.name]++ }
func (g *recordingGauge) Dec()          { g.values[g.name]-- }
func (g *recordingGauge) Add(v float64) { g.values[g.name] += v }

type recordingGaugeVec struct {
	name   string
	values map[string]float64
}

func (v recordingGaugeVec) With(labelValues ...string) metrics.Gauge {
	return &recordingGauge{name: v.name + "{" + labelValues[0] + "}", values: v.values}
}
func (recordingGaugeVec) Delete(...string) bool { return true }

// recordingRegistry records gauge values and discards every other metric kind.
type recordingRegistry struct {
	metrics.NopRegistry
	values map[string]float64
}

func (r recordingRegistry) Gauge(name, _ string) metrics.Gauge {
	return &recordingGauge{name: name, values: r.values}
}

func (r recordingRegistry) GaugeVec(name, _ string, _ []string) metrics.GaugeVec {
	return recordingGaugeVec{name: name, values: r.values}
}

// VALIDATES: ze_ipsec_tunnel_up reads 1 only when the IKE SA is established and its
// Child SA is installed in the dataplane. An established SA with no ESP reads up 0
// and degraded 1.
// PREVENTS: the operator-visible half of the swallowed install failure, where a
// tunnel that carried no encrypted traffic reported itself as healthy.
func TestIPsecTunnelUpRequiresESP(t *testing.T) {
	tests := []struct {
		name         string
		child        *ChildSA
		wantUp       float64
		wantDegraded float64
	}{
		{"esp installed", &ChildSA{ESPInstalled: true}, 1, 0},
		{"esp refused", &ChildSA{ESPInstalled: false}, 0, 1},
		{"no child sa", nil, 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := NewSATable()
			sa := &SA{PeerName: "peer-a", State: StateEstablished}
			sa.InitiatorSPI = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
			table.Insert(sa)
			activeTablePtr.Store(table)
			setActivePeers(map[string]*PeerSession{
				"peer-a": {peerName: "peer-a", childSA: tt.child},
			})
			defer func() {
				activeTablePtr.Store(nil)
				setActivePeers(nil)
			}()

			reg := recordingRegistry{values: map[string]float64{}}
			RegisterMetrics(reg).Update()

			if got := reg.values["ze_ipsec_tunnel_up{peer-a}"]; got != tt.wantUp {
				t.Errorf("ze_ipsec_tunnel_up = %v, want %v", got, tt.wantUp)
			}
			if got := reg.values["ze_ipsec_tunnel_degraded{peer-a}"]; got != tt.wantDegraded {
				t.Errorf("ze_ipsec_tunnel_degraded = %v, want %v", got, tt.wantDegraded)
			}
		})
	}
}

// VALIDATES: a peer with no established IKE SA reads neither up nor degraded.
// PREVENTS: a degraded alert that fires for every configured but unstarted peer.
func TestIPsecTunnelDownIsNotDegraded(t *testing.T) {
	table := NewSATable()
	activeTablePtr.Store(table)
	setActivePeers(map[string]*PeerSession{
		"peer-a": {peerName: "peer-a"},
	})
	defer func() {
		activeTablePtr.Store(nil)
		setActivePeers(nil)
	}()

	reg := recordingRegistry{values: map[string]float64{}}
	RegisterMetrics(reg).Update()

	if got := reg.values["ze_ipsec_tunnel_up{peer-a}"]; got != 0 {
		t.Errorf("ze_ipsec_tunnel_up = %v, want 0", got)
	}
	if got := reg.values["ze_ipsec_tunnel_degraded{peer-a}"]; got != 0 {
		t.Errorf("ze_ipsec_tunnel_degraded = %v, want 0", got)
	}
}

func TestIPsecMetrics_WithSAs(t *testing.T) {
	table := NewSATable()
	sa := &SA{PeerName: "peer-a", State: StateEstablished}
	sa.InitiatorSPI = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	table.Insert(sa)
	activeTablePtr.Store(table)
	peers := map[string]*PeerSession{
		"peer-a": {peerName: "peer-a"},
	}
	setActivePeers(peers)

	reg := metrics.NopRegistry{}
	m := RegisterMetrics(reg)
	assert.NotNil(t, m)
	m.Update()

	activeTablePtr.Store(nil)
	setActivePeers(nil)
}
