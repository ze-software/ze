// Design: docs/research/l2tpv2-ze-integration.md -- injected kernel PPP interface operations

package kernel

import (
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/rtproto"
)

// InterfaceOperations is implemented by the active kernel interface backend.
// Injection keeps this shared library below both transports and the iface
// component; it never imports either transport or an edge backend plugin.
type InterfaceOperations interface {
	SetMTU(name string, mtu int) error
	SetAdminUp(name string) error
	AddAddressP2P(name, local, peer string) error
	RemoveAddress(name, address string) error
	AddRoute(name, destination, gateway string, metric int, protocol rtproto.Proto) error
	RemoveRoute(name, destination, gateway string, metric int, protocol rtproto.Proto) error
}

// SessionConfig transfers two owned descriptors to NewSession on EVERY outcome.
// After calling it, the caller MUST NOT close either descriptor, even on error.
// Every nonnegative descriptor, including zero, is owned. A negative descriptor
// denotes no ownership. Rejected aliases still transfer their one descriptor.
// InterfaceName is the actual created name; empty explicitly selects the kernel
// pppN name for UnitNum. ReadCounters MUST read raw backend counters, rather than
// operator-visible counters adjusted by a clear operation.
type SessionConfig struct {
	ChanFD          int
	UnitFD          int
	UnitNum         int
	InterfaceName   string
	CaptureProxyLCP bool
	Backend         InterfaceOperations
	ReadCounters    func(string) (ppp.SessionCounters, error)
}
