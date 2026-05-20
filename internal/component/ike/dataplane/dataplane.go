// Design: plan/spec-ipsec-8-ikev2-child-xfrm.md -- dataplane abstraction for SA/SP installation
// RFC: rfc/short/rfc7296.md -- Child SA keying material (Section 2.17)
// RFC: rfc/short/rfc4301.md -- SAD/SPD architecture

package dataplane

import (
	"errors"
	"fmt"
	"net"
	"sync"
)

var (
	ErrNotRegistered = errors.New("dataplane: no backend registered")
	ErrNotSupported  = errors.New("dataplane: operation not supported on this platform")
)

// SADir is the direction of a Security Association / Policy.
type SADir uint8

const (
	SADirIn  SADir = 1
	SADirOut SADir = 2
	SADirFwd SADir = 3
)

// SAParams describes an ESP Security Association to install in the kernel or VPP.
type SAParams struct {
	SPI       uint32
	Src       net.IP
	Dst       net.IP
	IfID      uint32
	Proto     uint8 // 50 = ESP
	Mode      uint8 // 1 = transport, 2 = tunnel
	ReqID     uint32
	ReplayWin uint8

	EncAlgo  string
	EncKey   []byte
	AuthAlgo string
	AuthKey  []byte //nolint:gosec // ESP integrity key material, not a credential

	IsAEAD bool

	// NAT-T UDP encapsulation (RFC 3948).
	UDPEncap      bool
	UDPEncapSPort uint16
	UDPEncapDPort uint16
}

// SPParams describes a Security Policy to install in the kernel or VPP.
type SPParams struct {
	Src   *net.IPNet
	Dst   *net.IPNet
	Dir   SADir
	Proto uint8 // ESP = 50
	Mode  uint8 // tunnel = 2
	IfID  uint32
	ReqID uint32
}

// SAInfo is a summary of an installed SA returned by ListSAs.
type SAInfo struct {
	SPI  uint32
	Src  net.IP
	Dst  net.IP
	IfID uint32
}

// Dataplane abstracts the ESP SA/SP installation backend.
// Implementations: XFRM (Linux netlink), VPP (binary API).
type Dataplane interface {
	InstallSA(p SAParams) error
	RemoveSA(spi uint32, dst net.IP, proto uint8) error
	InstallPolicy(p SPParams) error
	RemovePolicy(src, dst *net.IPNet, dir SADir) error
	ListSAs(ifID uint32) ([]SAInfo, error)
	Close() error
}

var (
	mu       sync.Mutex
	backends = make(map[string]func() (Dataplane, error))
	active   Dataplane
)

// Register registers a dataplane backend factory by name.
func Register(name string, factory func() (Dataplane, error)) error {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := backends[name]; ok {
		return fmt.Errorf("dataplane: backend %q already registered", name)
	}
	backends[name] = factory
	return nil
}

// Load instantiates the named backend and sets it as the active dataplane.
func Load(name string) error {
	mu.Lock()
	defer mu.Unlock()
	factory, ok := backends[name]
	if !ok {
		return fmt.Errorf("dataplane: backend %q not registered", name)
	}
	dp, err := factory()
	if err != nil {
		return fmt.Errorf("dataplane: load %q: %w", name, err)
	}
	active = dp
	return nil
}

// Get returns the active dataplane backend, or nil if none loaded.
func Get() Dataplane {
	mu.Lock()
	defer mu.Unlock()
	return active
}

// CloseBackend shuts down the active dataplane backend.
func CloseBackend() error {
	mu.Lock()
	defer mu.Unlock()
	if active == nil {
		return nil
	}
	err := active.Close()
	active = nil
	return err
}
