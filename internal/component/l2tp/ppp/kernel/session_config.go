// Design: docs/research/l2tpv2-ze-integration.md -- injected kernel PPP interface operations

package kernel

import (
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

// SessionConfig transfers every newly supplied captured Channel and Unit owner
// to NewSession on EVERY outcome. Callers MUST NOT close them after that call.
// A copied/replayed owner supplies no already-admitted lease; the independent
// fresh sibling owner still transfers and is cleaned on refusal. Descriptor0
// is owned. ChanFD/UnitFD/UnitNum/InterfaceName are checked labels only: they
// never open, wrap, close, locate or establish custody of a native resource.
// Construct production input with actualUnit.SessionConfig(actualChannel).
// Only the original created unit's retained reader supplies measurements.
// All transports MUST supply both actual typed owners before activation.
type SessionConfig struct {
	Channel         sessionChannelSource
	Unit            sessionUnitSource
	ChanFD          int
	UnitFD          int
	UnitNum         int
	InterfaceName   string
	CaptureProxyLCP bool
	Backend         InterfaceOperations
}
