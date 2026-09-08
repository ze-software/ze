// Design: docs/architecture/ike/ipsec-10-cli-diag.md -- IPsec metrics tests

package engine

import (
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
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
	return &recordingGauge{name: v.key(labelValues), values: v.values}
}

// Delete removes the series, so a test can tell a deleted series from one set to
// zero. That difference is what AC-15 turns on.
func (v recordingGaugeVec) Delete(labelValues ...string) bool {
	key := v.key(labelValues)
	_, ok := v.values[key]
	delete(v.values, key)
	return ok
}

func (v recordingGaugeVec) key(labelValues []string) string {
	return v.name + "{" + labelValues[0] + "}"
}

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

// useCountingDriftSAD scripts the kernel half of the dataplane gauges and counts
// how often Update asks for it. The count is the assertion that one pass takes
// ONE dump: two dumps are two answers to one question, and they disagree across
// a rekey.
func useCountingDriftSAD(t *testing.T, sas []dataplane.SAInfo, err error) *int {
	t.Helper()
	calls := 0
	old := driftSAD
	driftSAD = func() ([]dataplane.SAInfo, error) {
		calls++
		return sas, err
	}
	t.Cleanup(func() { driftSAD = old })
	return &calls
}

// establishedPeer builds the engine belief one peer contributes: an established
// IKE SA in the table, and a Child SA holding the two SPIs.
func establishedPeer(t *testing.T, name string, inSPI, outSPI uint32) {
	t.Helper()
	table := ActiveTable()
	if table == nil {
		table = NewSATable()
		activeTablePtr.Store(table)
		t.Cleanup(func() { activeTablePtr.Store(nil) })
	}
	sa := &SA{PeerName: name, State: StateEstablished}
	sa.InitiatorSPI = [8]byte{1, 2, 3, 4, 5, 6, 7, byte(inSPI)}
	table.Insert(sa)

	next := map[string]*PeerSession{}
	maps.Copy(next, ActivePeers())
	ps := &PeerSession{peerName: name}
	ps.childSA = &ChildSA{InboundSPI: inSPI, OutboundSPI: outSPI, ESPInstalled: true}
	next[name] = ps
	setActivePeers(next)
	t.Cleanup(func() { setActivePeers(nil) })
}

// VALIDATES: one SAD read feeds both dataplane gauges. The count gauge carries one
// series per if_id the kernel holds, and the drift gauge carries 1 for a peer whose
// believed SPI is absent and 0 for a peer whose SPIs are present (AC-14).
// PREVENTS: two netlink dumps for one metrics pass, which can disagree across a
// rekey, and a drift gauge that never reports the clean case.
func TestDataplaneGaugesPublishFromOneDump(t *testing.T) {
	establishedPeer(t, "peer-a", 100, 200)
	establishedPeer(t, "peer-b", 300, 400)

	calls := useCountingDriftSAD(t, []dataplane.SAInfo{
		{SPI: 100, IfID: 0},
		{SPI: 200, IfID: 0},
		{SPI: 300, IfID: 7},
	}, nil)

	reg := recordingRegistry{values: map[string]float64{}}
	RegisterMetrics(reg).Update()

	if *calls != 1 {
		t.Errorf("driftSAD called %d times in one pass, want 1", *calls)
	}
	want := map[string]float64{
		"ze_ipsec_dataplane_sa_count{0}":   2,
		"ze_ipsec_dataplane_sa_count{7}":   1,
		"ze_ipsec_dataplane_drift{peer-a}": 0,
		"ze_ipsec_dataplane_drift{peer-b}": 1,
	}
	for name, value := range want {
		got, ok := reg.values[name]
		if !ok {
			t.Errorf("%s absent, want %v", name, value)
			continue
		}
		if got != value {
			t.Errorf("%s = %v, want %v", name, got, value)
		}
	}
}

// VALIDATES: an unreadable SAD publishes NO series and deletes every series an
// earlier pass published (AC-15).
// PREVENTS: the false green this spec exists to remove. A drift of 0 on a kernel
// nobody could read says "no drift" on the strength of a question nobody asked.
func TestDataplaneGaugesAbsentWhenKernelUnreadable(t *testing.T) {
	establishedPeer(t, "peer-a", 100, 200)

	reg := recordingRegistry{values: map[string]float64{}}
	m := RegisterMetrics(reg)

	useCountingDriftSAD(t, []dataplane.SAInfo{{SPI: 100, IfID: 0}, {SPI: 200, IfID: 0}}, nil)
	m.Update()
	if _, ok := reg.values["ze_ipsec_dataplane_drift{peer-a}"]; !ok {
		t.Fatal("the readable pass published no drift series, so the deletion proves nothing")
	}

	useCountingDriftSAD(t, nil, errors.New("operation not permitted"))
	m.Update()

	for name := range reg.values {
		if strings.HasPrefix(name, "ze_ipsec_dataplane_") {
			t.Errorf("%s still published after the SAD read failed", name)
		}
	}
}

// VALIDATES: a peer that leaves PeerInfoMap loses its drift series.
// PREVENTS: a scrape reporting a value for a peer that no longer exists, which an
// alert rule reads as a live drifting tunnel.
func TestDataplaneGaugesDeleteAPeerThatWentAway(t *testing.T) {
	establishedPeer(t, "peer-a", 100, 200)
	useCountingDriftSAD(t, []dataplane.SAInfo{{SPI: 100, IfID: 0}}, nil)

	reg := recordingRegistry{values: map[string]float64{}}
	m := RegisterMetrics(reg)
	m.Update()
	if _, ok := reg.values["ze_ipsec_dataplane_drift{peer-a}"]; !ok {
		t.Fatal("the first pass published no drift series for peer-a")
	}

	setActivePeers(map[string]*PeerSession{})
	m.Update()

	if _, ok := reg.values["ze_ipsec_dataplane_drift{peer-a}"]; ok {
		t.Error("ze_ipsec_dataplane_drift{peer-a} survived the peer leaving PeerInfoMap")
	}
}

// VALIDATES: every IPsec metric registers on a real Prometheus registry.
// PREVENTS: a label name Prometheus refuses. A hyphen is legal in a Ze JSON key and
// illegal in a Prometheus label, and MustRegister panics on one, which would take
// the daemon down at startup rather than at a scrape.
func TestIPsecMetricsRegisterOnPrometheus(t *testing.T) {
	RegisterMetrics(metrics.NewPrometheusRegistry())
}
