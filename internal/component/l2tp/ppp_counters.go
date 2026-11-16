// Design: docs/research/l2tpv2-ze-integration.md -- raw counters of a kernel PPP session

package l2tp

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
)

// kernelPPPCounters binds the admitted interface owner, rather than rereading
// the active backend or operator counters whose baseline can be cleared.
func kernelPPPCounters(backend pppkernel.InterfaceOperations) func(string) (ppp.SessionCounters, error) {
	reader, supported := backend.(interface {
		GetStats(string) (*iface.InterfaceStats, error)
	})
	return func(name string) (ppp.SessionCounters, error) {
		if !supported {
			return ppp.SessionCounters{}, fmt.Errorf("l2tp: interface owner cannot read raw counters for %s", name)
		}
		stats, err := reader.GetStats(name)
		if err != nil {
			return ppp.SessionCounters{}, err
		}
		if stats == nil {
			return ppp.SessionCounters{}, fmt.Errorf("l2tp: raw counters for %s returned no snapshot", name)
		}
		return ppp.SessionCounters{
			RxBytes:   stats.RxBytes,
			TxBytes:   stats.TxBytes,
			RxPackets: stats.RxPackets,
			TxPackets: stats.TxPackets,
		}, nil
	}
}
