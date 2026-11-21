// Design: docs/research/l2tpv2-ze-integration.md -- original owner measurement conversion
// Related: ppp_counters_measurement_test.go -- preserved measurement questions
// Related: kernel_counter_transport_integration_linux_test.go -- genuine carrier

//go:build integration && linux

package l2tp

import (
	"errors"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
)

// kernelMeasurementBackend borrows the independently owned actual backend.
// Creation and the original reader remain real. The reply port supplies only
// measurement inputs after the original reader succeeds; it cannot create a
// file, native identity, namespace, birth or counter grant. Tests are serial.
type kernelMeasurementBackend struct {
	iface.Backend
	stats *iface.InterfaceStats
	err   error
	name  string
}

func (backend *kernelMeasurementBackend) CreateLinkCounterReader(request *nl.NetlinkRequest) (netlink.Link, func() (*netlink.LinkStatistics, error), bool, error) {
	source, ok := backend.Backend.(interface {
		CreateLinkCounterReader(*nl.NetlinkRequest) (netlink.Link, func() (*netlink.LinkStatistics, error), bool, error)
	})
	if !ok {
		return nil, nil, false, errors.New("actual measurement backend has no original creation capability")
	}
	link, original, attempted, err := source.CreateLinkCounterReader(request)
	if err != nil {
		return link, original, attempted, err
	}
	if link == nil {
		return nil, original, attempted, errors.New("native measurement creation returned no link")
	}
	if original == nil {
		return link, nil, attempted, errors.New("native measurement creation returned no original reader")
	}
	attrs := link.Attrs()
	if attrs == nil {
		return link, original, attempted, errors.New("native measurement creation returned no attributes")
	}
	backend.name = attrs.Name
	return link, func() (*netlink.LinkStatistics, error) {
		// Refuse a retired, moved, deleted or otherwise unreadable original
		// before supplying any controlled numeric reply.
		originalStats, err := original()
		if err != nil {
			return nil, err
		}
		if originalStats == nil {
			return nil, errors.New("actual original measurement reader returned no snapshot")
		}
		if backend.stats == nil {
			return nil, backend.err
		}
		stats := backend.stats
		return &netlink.LinkStatistics{
			RxBytes: stats.RxBytes, TxBytes: stats.TxBytes,
			RxPackets: stats.RxPackets, TxPackets: stats.TxPackets,
			RxErrors: stats.RxErrors, TxErrors: stats.TxErrors,
			RxDropped: stats.RxDropped, TxDropped: stats.TxDropped,
		}, backend.err
	}, attempted, nil
}

func (*kernelMeasurementBackend) GetStats(string) (*iface.InterfaceStats, error) {
	return nil, errors.New("name lookup cannot supply original measurement counters")
}

// newKernelMeasurementSession retains the old ppp73 input as an actual created
// unit name. The real constructor consumes both original file owners on every
// outcome. The backend and carrier outlive that sole session owner. Serial.
func newKernelMeasurementSession(t *testing.T, stats *iface.InterfaceStats) (*kernelMeasurementBackend, ppp.SessionHandle) {
	t.Helper()
	backend := &kernelMeasurementBackend{Backend: kernelCounterNativeBackend(t), stats: stats}
	creator, err := pppkernel.NewUnitCreator(backend)
	if err != nil {
		t.Fatal(err)
	}
	transport := kernelCounterTransport(t)
	channel, unit, err := pppkernel.DevPPPSetup(creator, transport, "ppp73")
	transferred := false
	t.Cleanup(func() {
		if transferred {
			return
		}
		if err := unit.Close(); err != nil {
			t.Error(err)
		}
		if err := channel.Close(); err != nil {
			t.Error(err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	config, err := unit.SessionConfig(channel)
	if err != nil {
		t.Fatal(err)
	}
	transferred = true
	_, handle, err := pppkernel.NewSession(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	})
	if handle.Identity().InterfaceName != "ppp73" {
		t.Fatalf("created measurement interface=%q, want ppp73", handle.Identity().InterfaceName)
	}
	return backend, handle
}
