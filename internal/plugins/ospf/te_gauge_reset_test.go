// VALIDATES: fix E -- a GaugeVec label set that drains between refreshes is reset to 0 rather
// than left at its last value (metrics.GaugeVec has no Reset). Exercised on ze_ospf_te_database_links
// and ze_ospf_te_lsas by installing a TE link then withdrawing it.
// PREVENTS: a stale non-zero OSPF TE gauge for an area/scope whose population has emptied.
package ospf

import (
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
)

// recordingGaugeRegistry records the last value Set on each GaugeVec series ("name|labels"),
// leaving every other registry method a no-op.
type recordingGaugeRegistry struct {
	metrics.NopRegistry
	mu    sync.Mutex
	gauge map[string]float64
}

func (r *recordingGaugeRegistry) GaugeVec(name, _ string, _ []string) metrics.GaugeVec {
	return &recordingGaugeVec{reg: r, name: name}
}

type recordingGaugeVec struct {
	reg  *recordingGaugeRegistry
	name string
}

func (v *recordingGaugeVec) With(labelValues ...string) metrics.Gauge {
	return &recordingGauge{reg: v.reg, key: v.name + "|" + strings.Join(labelValues, ",")}
}
func (v *recordingGaugeVec) Delete(...string) bool { return false }

type recordingGauge struct {
	reg *recordingGaugeRegistry
	key string
}

func (g *recordingGauge) Set(f float64) {
	g.reg.mu.Lock()
	g.reg.gauge[g.key] = f
	g.reg.mu.Unlock()
}
func (g *recordingGauge) Inc()        {}
func (g *recordingGauge) Dec()        {}
func (g *recordingGauge) Add(float64) {}

func (r *recordingGaugeRegistry) value(key string) float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.gauge[key]
}

func TestTEGaugeResetOnEmptiedPopulation(t *testing.T) {
	eng, _ := newRedistEngine(t, teCfgJSON)
	reg := &recordingGaugeRegistry{gauge: map[string]float64{}}
	eng.setMetrics(reg)

	adv := mustRouterID(t, "2.2.2.2")
	// Install one TE link in the backbone area -> ze_ospf_te_database_links{area=0.0.0.0} = 1.
	eng.teOnReceive(teReceived(OpaqueScopeArea, packet.TEOpaqueType, 1, adv, teLinkBody(t), true, false))
	const dbKey = "ze_ospf_te_database_links|0.0.0.0"
	if got := reg.value(dbKey); got != 1 {
		t.Fatalf("%s = %v after install, want 1", dbKey, got)
	}

	// Withdraw it -> the TED empties -> the area label must be reset to 0, not left at 1.
	eng.teOnReceive(teReceived(OpaqueScopeArea, packet.TEOpaqueType, 1, adv, teLinkBody(t), true, true))
	if got := reg.value(dbKey); got != 0 {
		t.Fatalf("%s = %v after the population emptied, want 0 (gauge not reset)", dbKey, got)
	}
	// The per-scope/kind TE-LSA gauge must likewise drain to 0.
	if got := reg.value("ze_ospf_te_lsas|area,link"); got != 0 {
		t.Fatalf("ze_ospf_te_lsas{scope=area,kind=link} = %v after emptying, want 0", got)
	}
}
