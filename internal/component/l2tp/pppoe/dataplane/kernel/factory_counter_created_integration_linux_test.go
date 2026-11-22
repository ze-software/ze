// Design: docs/research/l2tpv2-ze-integration.md -- original factory measurement conversion
// Related: counter_measurement_test.go -- every preserved numeric/error/zero assertion

//go:build integration && linux

package kernel

import (
	"errors"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/test/userns"
)

// factoryCounterBackend delegates actual original creation to the independently
// owned real backend. Its one private reply seam changes numeric measurement
// inputs only, never files, identity, namespace, socket custody or native birth.
// Embedding iface.Backend hides the real backend's optional capabilities, so
// the original-unit mutation binding (iface.NativePPPBackend) is forwarded
// unchanged to the same real backend, the way original creation is.
// These tests prove conversion/refusal, not actual traffic measurement. Serial.
type factoryCounterBackend struct {
	iface.Backend
	stats     *iface.InterfaceStats
	statsErr  error
	statsName string
}

func (backend *factoryCounterBackend) CreateLinkCounterReader(request *nl.NetlinkRequest) (netlink.Link, func() (*netlink.LinkStatistics, error), bool, error) {
	source, ok := backend.Backend.(interface {
		CreateLinkCounterReader(*nl.NetlinkRequest) (netlink.Link, func() (*netlink.LinkStatistics, error), bool, error)
	})
	if !ok {
		return nil, nil, false, errors.New("actual fixture backend has no original creation capability")
	}
	link, original, attempted, err := source.CreateLinkCounterReader(request)
	if err != nil {
		return link, original, attempted, err
	}
	if link == nil {
		return nil, original, attempted, errors.New("native fixture creation returned no link")
	}
	if original == nil {
		return link, nil, attempted, errors.New("native fixture creation returned no original reader")
	}
	backend.statsName = link.Attrs().Name
	return link, func() (*netlink.LinkStatistics, error) {
		// Read the actual original lease first. A substituted measurement never
		// turns a retired/moved/deleted native owner into a productive reader.
		if _, err := original(); err != nil {
			return nil, err
		}
		if backend.statsErr != nil {
			return nil, backend.statsErr
		}
		if backend.stats == nil {
			return nil, nil
		}
		stats := backend.stats
		return &netlink.LinkStatistics{
			RxBytes: stats.RxBytes, TxBytes: stats.TxBytes,
			RxPackets: stats.RxPackets, TxPackets: stats.TxPackets,
			RxErrors: stats.RxErrors, TxErrors: stats.TxErrors,
			RxDropped: stats.RxDropped, TxDropped: stats.TxDropped,
		}, nil
	}, attempted, nil
}

// BindNativePPPUnit forwards the original-unit mutation binding to the real
// backend that created the unit. A real backend without it stays refused.
func (backend *factoryCounterBackend) BindNativePPPUnit(owner iface.NativePPPUnitOwner, index int, unit int32) (iface.NativePPPWriter, error) {
	source, ok := backend.Backend.(iface.NativePPPBackend)
	if !ok {
		return nil, errors.Join(iface.ErrNativePPPUnavailable, errors.New("actual fixture backend has no original-unit mutation binding"))
	}
	return source.BindNativePPPUnit(owner, index, unit)
}

func (*factoryCounterBackend) GetStats(string) (*iface.InterfaceStats, error) {
	return nil, errors.New("name lookup cannot supply original factory counters")
}

func newFactoryCounterSession(t *testing.T, stats *iface.InterfaceStats) (*factoryCounterBackend, dataplane.OpenedSession) {
	t.Helper()
	prepared := newPreparedNativeAccess(t)
	backend := &factoryCounterBackend{Backend: prepared.backend, stats: stats}
	creator, err := pppkernel.NewUnitCreator(backend)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := openWithOps(t.Context(), prepared.request(81), preparedFactoryOps(backend, creator))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := errors.Join(opened.Dataplane().Close(), opened.CloseTransport()); err != nil {
			t.Error(err)
		}
	})
	return backend, opened
}

// TestFactoryCountersUseRawBackend retains the original raw totals, provider
// error and absent-snapshot questions through a genuine typed factory session.
// The old callback name is replaced by the actual original creation identity;
// a GetStats trap proves names grant no measurement authority.
func TestFactoryCountersUseRawBackend(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	backend, opened := newFactoryCounterSession(t, &iface.InterfaceStats{RxBytes: 11, TxBytes: 13, RxPackets: 17, TxPackets: 19})
	counters, err := opened.Dataplane().Counters()
	if err != nil || counters != (ppp.SessionCounters{
		RxBytes: 11, TxBytes: 13, RxPackets: 17, TxPackets: 19,
		RxErrors: ppp.MeasuredCounter(0), TxErrors: ppp.MeasuredCounter(0),
		RxDropped: ppp.MeasuredCounter(0), TxDropped: ppp.MeasuredCounter(0),
	}) {
		t.Fatalf("raw counters=%+v error=%v", counters, err)
	}
	if backend.statsName != opened.Dataplane().Identity().InterfaceName {
		t.Fatalf("counter creation device=%q", backend.statsName)
	}
	want := errors.New("counter read")
	backend.statsErr = want
	if _, err := opened.Dataplane().Counters(); !errors.Is(err, want) {
		t.Fatalf("counter failure lost: %v", err)
	}
	backend.statsErr = nil
	backend.stats = nil
	if _, err := opened.Dataplane().Counters(); err == nil {
		t.Fatal("missing counter snapshot reported success")
	}
}
