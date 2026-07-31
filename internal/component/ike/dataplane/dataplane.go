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
//
// These values are Ze's own vocabulary. They are 1-based on purpose, so the zero
// value of an unset Mode field is never a valid mode. They are NOT the kernel XFRM
// mode numbers. Use kernelXFRMMode to convert.
const (
	ModeTransport uint8 = 1 // RFC 4301 transport mode (protects the payload, keeps the outer IP header)
	ModeTunnel    uint8 = 2 // RFC 4301 tunnel mode (encapsulates the whole packet)
)

// Kernel XFRM mode numbers from uapi/linux/xfrm.h. They start at 0, so they are
// offset by one from the ModeTransport / ModeTunnel constants above.
const (
	kernelModeTransport uint8 = 0 // XFRM_MODE_TRANSPORT
	kernelModeTunnel    uint8 = 1 // XFRM_MODE_TUNNEL
)

// kernelXFRMMode converts a Ze dataplane mode to the kernel XFRM mode number.
// The second result is false for an unknown mode. The caller MUST then reject
// the install. The function never defaults. A wrong mode number is silent. The
// kernel accepts it and protects the traffic in the wrong mode.
//
// The two enumerations do not share a numbering. Ze counts from 1, so an unset
// field is invalid. The kernel counts from 0. A direct numeric pass-through
// shifted every mode by one.
//
// ModeTunnel (2) reached the kernel as XFRM_MODE_ROUTEOPTIMIZATION. The array
// xfrm4_mode_map in net/xfrm/xfrm_state.c does not define that index.
// __xfrm_init_state failed the outer-mode lookup and returned EPROTONOSUPPORT.
//
// ModeTransport (1) was worse. It reached the kernel as XFRM_MODE_TUNNEL, which
// is valid. The kernel installed an SA in the wrong mode and reported no error.
//
// This mirrors the direction conversion the same backend already does for SADir.
func kernelXFRMMode(mode uint8) (uint8, bool) {
	switch mode {
	case ModeTransport:
		return kernelModeTransport, true
	case ModeTunnel:
		return kernelModeTunnel, true
	default:
		return 0, false
	}
}

// validTunnelEndpoint reports whether an address can serve as a tunnel endpoint.
// A nil, a malformed, and an unspecified address cannot. The unspecified test is
// the load-bearing one. 0.0.0.0 is the exact value an absent endpoint produced.
func validTunnelEndpoint(ip net.IP) bool {
	return len(ip) > 0 && ip.To16() != nil && !ip.IsUnspecified()
}

// tunnelEndpoints checks the tunnel endpoints of a policy template against the mode
// and returns the pair to write into the template. Transport mode returns no pair.
//
// The guard fails closed. A tunnel-mode policy with no endpoints reaches the kernel
// as tmpl src 0.0.0.0 dst 0.0.0.0. Netlink derives the template family from a nil
// address and writes zeros. The kernel then resolves the policy to no state, and
// the tunnel forwards nothing. No error reports the fault, so a zero address must
// never be a valid answer here.
//
// RFC 4301 Section 4.4.1.2 leaves a transport-mode template's addresses unused.
// Endpoints in a transport-mode request are a caller mistake. This rejects them
// rather than discards them, because a silent discard hides the same confusion.
func tunnelEndpoints(p SPParams) (net.IP, net.IP, error) {
	if p.Mode != ModeTunnel {
		if len(p.TunnelSrc) > 0 || len(p.TunnelDst) > 0 {
			return nil, nil, fmt.Errorf(
				"transport mode must carry no tunnel endpoints, got src=%v dst=%v: RFC 4301 Section 4.4.1.2 leaves them unused",
				p.TunnelSrc, p.TunnelDst)
		}
		return nil, nil, nil
	}
	if !validTunnelEndpoint(p.TunnelSrc) || !validTunnelEndpoint(p.TunnelDst) {
		return nil, nil, fmt.Errorf(
			"tunnel mode needs both tunnel endpoints, got src=%v dst=%v: an absent endpoint reaches the kernel as 0.0.0.0 and matches no state",
			p.TunnelSrc, p.TunnelDst)
	}
	if (p.TunnelSrc.To4() != nil) != (p.TunnelDst.To4() != nil) {
		return nil, nil, fmt.Errorf(
			"tunnel endpoints must share an address family, got src=%v dst=%v",
			p.TunnelSrc, p.TunnelDst)
	}
	return p.TunnelSrc, p.TunnelDst, nil
}

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

	EncAlgo string

	// EncKey is the encryption key material for one direction. Its layout depends on
	// IsAEAD, and a backend that ignores that distinction keys the SA wrongly.
	//
	// IsAEAD false: the cipher key alone.
	//
	// IsAEAD true: the cipher key followed by that cipher's salt, in one slice. RFC 4106
	// Section 8.1 makes AES-GCM KEYMAT four octets longer than the AES key, so AES-GCM-256
	// gives 36 octets: 32 of key then 4 of salt. crypto.encKeyMaterialLen derives the
	// length per algorithm, so a future AEAD whose salt is not four octets changes it.
	//
	// A backend whose API takes the key and the salt in separate fields MUST split the
	// slice. The salt is the last len(EncKey)-keyBytes octets. The Linux XFRM backend
	// takes the whole slice unsplit, because rfc4106(gcm(aes)) expects this layout. The
	// VPP backend does not split it (plan/spec-fixit-vpp-ipsec-inoperable.md).
	EncKey []byte

	AuthAlgo string
	AuthKey  []byte //nolint:gosec // ESP integrity key material, not a credential

	// IsAEAD selects the EncKey layout above, and it selects a single combined-mode
	// transform over the separate encryption and integrity pair.
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

	// TunnelSrc and TunnelDst are the tunnel endpoints of the policy template. They
	// are the outer IP header addresses of the encapsulated packet (RFC 4301 Section
	// 4.4.1.2, "the tunnel header IP source and destination addresses").
	//
	// They are NOT the selector. Src and Dst above are the selector, the inner
	// traffic the policy matches, and they are prefixes. The tunnel endpoints are
	// single addresses, so a mix-up of the two pairs is a compile error.
	//
	// Tunnel mode needs both. The kernel resolves a policy to a state through the
	// template addresses. An absent pair leaves the template at 0.0.0.0, no state
	// matches it, and the tunnel forwards nothing.
	//
	// Transport mode needs neither. RFC 4301 Section 4.4.1.2 leaves a transport-mode
	// template's addresses unused. tunnelEndpoints rejects both mistakes.
	TunnelSrc net.IP
	TunnelDst net.IP
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
