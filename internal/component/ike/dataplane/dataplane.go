// Design: plan/learned/742-ipsec-8-ikev2-child-xfrm.md -- dataplane abstraction for SA/SP installation
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

// IPsec protocol numbers used in SAParams.Proto / SPParams tmpl proto.
const (
	ProtoESP uint8 = 50 // RFC 4303 Encapsulating Security Payload
	ProtoAH  uint8 = 51 // RFC 4302 Authentication Header (integrity only, no encryption)
)

// SA / policy modes used in SAParams.Mode and SPParams.Mode.
const (
	ModeTransport uint8 = 1 // RFC 4301 transport mode (protects the payload, keeps the outer IP header)
	ModeTunnel    uint8 = 2 // RFC 4301 tunnel mode (encapsulates the whole packet)
)

// xfrmAlgoPlan describes which XfrmState algorithm slots an SA install must set.
// It isolates the AEAD-vs-crypt+auth-vs-auth-only decision from the netlink-only
// backend so the choice is unit-testable on any platform.
type xfrmAlgoPlan struct {
	AEAD  bool // single combined-mode (AEAD) transform, e.g. AES-GCM
	Crypt bool // separate encryption transform (ESP confidentiality)
	Auth  bool // integrity transform
}

// planStateAlgos decides which algorithm slots an SA install sets. RFC 4302: AH
// carries integrity only and MUST NOT set an encryption transform; RFC 4303: ESP
// sets encryption + integrity, or a single AEAD transform when the algorithm is
// combined-mode.
func planStateAlgos(p SAParams) xfrmAlgoPlan {
	switch {
	case p.IsAEAD:
		return xfrmAlgoPlan{AEAD: true}
	case p.Proto == ProtoAH:
		return xfrmAlgoPlan{Auth: true}
	default:
		return xfrmAlgoPlan{Crypt: true, Auth: true}
	}
}

// SASelector narrows which flows a Security Association matches during kernel state
// resolution (RFC 4301 SPD selector projected onto the SA, i.e. the XFRM x->sel). It
// lets one SA cover many flows: the kernel resolves the SA for any flow matching
// Src/Dst/UpperProto rather than only flows whose daddr equals SAParams.Dst.
//
// RFC 4552 OSPFv3 needs this because one manually-keyed SA per direction must cover
// every OSPFv3 destination (ff02::5 AllSPFRouters, ff02::6 AllDRouters, and neighbor
// link-local unicast for DBD/LSU-retransmit), and the outbound neighbor unicast daddr
// is not known at install time. Src/Dst are prefixes (::/0 = any); UpperProto is the
// upper-layer protocol (0 = any, 89 = OSPF).
type SASelector struct {
	Src        *net.IPNet
	Dst        *net.IPNet
	UpperProto uint8
}

// SAParams describes an ESP Security Association to install in the kernel or VPP.
type SAParams struct {
	SPI       uint32
	Src       net.IP
	Dst       net.IP
	IfID      uint32
	Proto     uint8 // ProtoESP (50) or ProtoAH (51)
	Mode      uint8 // ModeTransport (1) or ModeTunnel (2)
	ReqID     uint32
	ReplayWin uint8

	EncAlgo  string
	EncKey   []byte
	AuthAlgo string
	AuthKey  []byte //nolint:gosec // ESP integrity key material, not a credential

	IsAEAD bool

	// Sel, when non-nil, installs an explicit XFRM state selector (x->sel) so the
	// kernel resolves this SA for any flow matching the selector, not only for flows
	// whose daddr equals Dst. IKE child SAs leave Sel nil, so their state selector
	// stays the zero value (byte-identical to before this field existed).
	Sel *SASelector

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
	Proto uint8 // IPsec transform proto for the policy template: ESP = 50, AH = 51
	Mode  uint8 // ModeTransport = 1, ModeTunnel = 2
	IfID  uint32
	ReqID uint32
	// UpperProto is the upper-layer protocol selector for the policy (0 = any,
	// the historical IKE default; 89 restricts the policy to OSPF traffic per
	// RFC 4552 §5/§6). It threads onto the XfrmPolicy selector, not the template.
	UpperProto uint8
	// IfIndex is the kernel interface index of the policy selector (XFRM sel.ifindex,
	// the outbound oif / inbound iif). RFC 4552 §6 interface-based selectors: a
	// non-zero value scopes the policy to a single interface so a plain non-IPsec
	// OSPFv3 interface on the same node keeps passing OSPF unprotected. It is
	// distinct from IfID (the XFRM if_id / XFRMA_IF_ID used by IKE to bind an SA to
	// an xfrm interface device). IKE leaves IfIndex 0 (node-wide), byte-identical.
	IfIndex int
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
	// RemovePolicyParams removes a policy by its full selector (Src, Dst, Dir,
	// UpperProto, IfID). RemovePolicy only carries src/dst/dir, so it cannot
	// delete a policy installed with an upper-layer-protocol selector (e.g. the
	// OSPF proto-89 policies): the kernel identifies a policy by its whole
	// selector, so the proto must match on delete.
	RemovePolicyParams(p SPParams) error
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
