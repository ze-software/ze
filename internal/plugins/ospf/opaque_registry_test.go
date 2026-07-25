// VALIDATES: spec-ospf-ext-1 AC-1/AC-15/R-9 -- registerOpaqueConsumer stores a
// registration, rejects a duplicate Opaque Type and an invalid scope, and a consumer
// callback panic is recovered (the engine survives and counts
// ze_ospf_opaque_consumer_errors_total).
// PREVENTS: two consumers silently sharing one Opaque Type, and a single bad consumer
// crashing the OSPF engine or wedging the LSDB.
package ospf

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// opaqueMetricRegistry records CounterVec increments by "name|labels" so a test can assert a
// specific series fired.
type opaqueMetricRegistry struct {
	metrics.NopRegistry
	counts map[string]int
}

func (r *opaqueMetricRegistry) CounterVec(name, _ string, _ []string) metrics.CounterVec {
	return &opaqueMetricVec{reg: r, name: name}
}

type opaqueMetricVec struct {
	reg  *opaqueMetricRegistry
	name string
}

func (v *opaqueMetricVec) With(labelValues ...string) metrics.Counter {
	return opaqueMetricCounter{reg: v.reg, key: v.name + "|" + strings.Join(labelValues, ",")}
}
func (v *opaqueMetricVec) Delete(...string) bool { return false }

type opaqueMetricCounter struct {
	reg *opaqueMetricRegistry
	key string
}

func (c opaqueMetricCounter) Inc()        { c.reg.counts[c.key]++ }
func (c opaqueMetricCounter) Add(float64) { c.reg.counts[c.key]++ }

func TestOpaqueConsumerRegistered(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)

	if err := registerOpaqueConsumer(1, OpaqueScopeArea, nil, nil); err != nil {
		t.Fatalf("registerOpaqueConsumer: %v", err)
	}
	c, ok := lookupOpaqueConsumer(1)
	if !ok {
		t.Fatalf("registered consumer not found")
	}
	if c.opaqueType != 1 || c.scope != OpaqueScopeArea {
		t.Fatalf("stored consumer = %+v", c)
	}
	if len(opaqueConsumerSnapshot()) != 1 {
		t.Fatalf("snapshot = %d, want 1", len(opaqueConsumerSnapshot()))
	}
}

func TestOpaqueConsumerDuplicateRejected(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)

	if err := registerOpaqueConsumer(3, OpaqueScopeArea, nil, nil); err != nil {
		t.Fatalf("first registration: %v", err)
	}
	// Same Opaque Type -> rejected, even at a different scope.
	if err := registerOpaqueConsumer(3, OpaqueScopeAS, nil, nil); !errors.Is(err, ErrOpaqueTypeRegistered) {
		t.Fatalf("duplicate registration err = %v, want ErrOpaqueTypeRegistered", err)
	}
	// An invalid scope is rejected.
	if err := registerOpaqueConsumer(4, OpaqueScope(99), nil, nil); !errors.Is(err, ErrOpaqueScopeInvalid) {
		t.Fatalf("invalid-scope err = %v, want ErrOpaqueScopeInvalid", err)
	}
}

func TestOpaqueConsumerPanicIsolated(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)

	reg := &opaqueMetricRegistry{counts: map[string]int{}}
	eng := newEngine(transport.New(&fakeBackend{}))
	eng.setMetrics(reg)

	delivered := false
	if err := registerOpaqueConsumer(5, OpaqueScopeArea, nil, func(opaqueReceived) {
		delivered = true
		panic("consumer blew up")
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	// deliverOpaque must recover the panic (no crash) and count the consumer error.
	eng.deliverOpaque(ospflsdb.OpaqueDelivery{
		Scope:             types.LSTypeOpaqueArea,
		Area:              mustBackboneArea(t),
		AdvertisingRouter: mustRouterID(t, "2.2.2.2"),
		OpaqueType:        5,
		OpaqueID:          0x10,
		Body:              []byte{1, 2, 3, 4},
	})
	if !delivered {
		t.Fatalf("consumer OnReceive was not invoked")
	}
	if reg.counts["ze_ospf_opaque_consumer_errors_total|5"] == 0 {
		t.Fatalf("consumer panic did not increment ze_ospf_opaque_consumer_errors_total: %v", reg.counts)
	}
}
