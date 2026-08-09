// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- dataplane abstraction for SA/SP installation
// RFC: rfc/short/rfc7296.md -- Child SA keying material (Section 2.17)
// RFC: rfc/short/rfc4301.md -- SAD/SPD architecture

package dataplane

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
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

// SPAction is what a Security Policy DOES with the traffic its selector matches.
//
// The zero value is SPActionProtect on purpose, and that choice is the fail-closed
// one (ai/rules/evidence.md). This enumeration is the exception to the
// 1-based convention Mode and SADir follow above, and the reason is the direction
// each mistake fails in. An unset Mode made the kernel protect traffic in the WRONG
// mode, silently, so zero had to be invalid there. An unset Action gives the
// RESTRICTIVE behavior: the traffic is handed to IPsec. A caller who forgets this
// field gets a policy that protects, never one that lets traffic out in the clear.
type SPAction uint8

const (
	// SPActionProtect hands matching traffic to the IPsec transform named by the
	// policy template. This is XFRM_POLICY_ALLOW WITH a template.
	SPActionProtect SPAction = 0
	// SPActionBypass lets matching traffic pass unprotected. This is
	// XFRM_POLICY_ALLOW with NO template, the SPD "BYPASS" disposition of RFC 4301
	// Section 4.4.1. A bypass policy carries no template, so it needs no mode, no
	// tunnel endpoints and no reqid.
	SPActionBypass SPAction = 1
)

// Security Policy priorities.
//
// LOWER VALUE MEANS HIGHER PRECEDENCE. The kernel keeps each policy chain sorted by
// priority ascending and takes the first match (net/xfrm/xfrm_policy.c,
// xfrm_policy_insert and xfrm_policy_lookup_bytype). At EQUAL priority the ordering
// falls back to insertion order, which is why neither of these is left at zero: a
// tie between the IKE bypass and a Child SA policy would be resolved by whichever
// was installed last.
//
// Every policy this package installs for IKE carries one of these two. Before they
// existed both landed at priority 0, so no bypass could outrank a Child SA policy
// whose selector had already captured the IKE endpoints.
//
// The gap between them is deliberate room. Nothing occupies it today.
const (
	// PriorityIKEBypass ranks the IKE control-plane bypass ahead of every negotiated
	// Child SA policy. IKE is what BUILDS the Child SA, so a Child SA policy that
	// captured IKE would prevent its own renegotiation, its own rekey and its own
	// teardown.
	PriorityIKEBypass = 100
	// PriorityChildSA ranks a negotiated Child SA policy. It is deliberately worse
	// than PriorityIKEBypass and deliberately non-zero.
	PriorityChildSA = 2000
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

	// AcceptBothESPForms asks the backend to receive BOTH ESP wire forms on this state,
	// rather than only the one UDPEncap selects. It is meaningful on an INBOUND state
	// alone, because it describes what the device accepts and never what it sends.
	//
	// RFC 7296 Section 2.23: "all devices MUST be able to receive and process both
	// UDP-encapsulated ESP and non-UDP-encapsulated ESP packets at any time"
	// (rfc/full/rfc7296.txt:3544-3548). One Linux XFRM state binds exactly one form, so
	// a backend serves the second form beside the kernel rather than through it.
	//
	// A backend that cannot receive both forms MUST reject the install rather than
	// report success (ai/rules/protocol.md). An SA installed for one form only
	// silently drops the other, which is the quietest failure this subsystem has: the
	// tunnel establishes and carries no traffic.
	AcceptBothESPForms bool
}

// PortMatch is the port half of a policy selector, as the kernel expresses it: a value
// plus a mask.
//
// A zero Mask matches EVERY port and ignores Port, which is the historical behavior and
// the byte-identical default for a caller that sets neither field. A Mask of 0xffff
// matches exactly Port.
//
// The two-field shape is the kernel's, not a Ze invention: XfrmSelector carries Sport,
// SportMask, Dport and DportMask. A caller that needs an inclusive port RANGE cannot
// express it here, and must narrow to a single port before installing (RFC 7296 Section
// 2.9 permits narrowing, and rounding outward to "any port" would widen the policy).
type PortMatch struct {
	Port uint16
	Mask uint16
}

// AnyPortMatch matches every port. It is the zero value, named so a caller can say so.
func AnyPortMatch() PortMatch { return PortMatch{} }

// ExactPortMatch matches one port.
func ExactPortMatch(port uint16) PortMatch { return PortMatch{Port: port, Mask: 0xffff} }

// IsAny reports whether this match constrains nothing.
func (p PortMatch) IsAny() bool { return p.Mask == 0 }

// SPParams describes a Security Policy to install in the kernel or VPP.
type SPParams struct {
	Src   *net.IPNet
	Dst   *net.IPNet
	Dir   SADir
	Proto uint8 // IPsec transform proto for the policy template: ESP = 50, AH = 51
	Mode  uint8 // ModeTransport = 1, ModeTunnel = 2
	IfID  uint32
	ReqID uint32

	// Action is what the policy does with matching traffic. The zero value protects
	// it (see SPAction). SPActionBypass installs a template-free policy, and a
	// backend that cannot express one MUST reject the install rather than fall back
	// to protecting (ai/rules/protocol.md): a bypass silently downgraded to a
	// protect policy black-holes the traffic it was meant to let through.
	Action SPAction

	// Owner names who this policy's selector belongs to, so a backend can tell one
	// installer's re-install from a DIFFERENT installer's takeover.
	//
	// The kernel cannot make that distinction. A policy's whole identity there is its
	// selector, its direction, its mark and its if_id, and IKE leaves mark unset and
	// if_id 0 unless an XFRM interface is configured. Two site-to-site peers that both
	// negotiate 0.0.0.0/0 therefore describe the SAME kernel policy, and the backend
	// upserts (see xfrmBackend.InstallPolicy), so the second peer to establish would
	// silently capture the first peer's traffic into its own tunnel and either peer's
	// teardown would blackhole the survivor.
	//
	// A per-peer kernel MARK does not fix it. The kernel's packet-matching predicate
	// is (flowi_mark & pol->mark.m) == pol->mark.v, so a value with no mask matches
	// nothing at all and a value with a mask matches only packets something else has
	// already marked. Nothing in ze marks them, so a marked policy forwards nothing.
	// Two identical 0.0.0.0/0 selectors are also ambiguous by construction: even if
	// they coexisted, the kernel could not tell which tunnel a packet belongs to.
	//
	// So the second install is REFUSED instead, which is what the kernel did through
	// EEXIST before the upsert landed. Ownership is what separates the refusal from
	// the rekey: a rekey re-installs an IDENTICAL selector under the SAME owner and
	// must still upsert (ai/rules/protocol.md).
	//
	// An empty Owner is the historical, unowned policy. It collides only with another
	// empty one, so a caller that never sets it installs exactly what it installed
	// before this field existed.
	Owner string

	// Priority ranks this policy against every other policy whose selector also
	// matches. LOWER VALUE MEANS HIGHER PRECEDENCE. Use PriorityIKEBypass or
	// PriorityChildSA; a bare 0 ties with every other unset policy and the winner is
	// then decided by installation order.
	Priority int

	// SrcPort and DstPort narrow the policy selector to a port. The zero value of each
	// matches every port, so a caller that never sets them installs the same policy it
	// installed before these fields existed.
	//
	// RFC 7296 Section 3.13.1 negotiates a port RANGE, and this pair carries only ANY or
	// one exact port, so the IKE engine narrows the negotiated range to a form that fits
	// before it reaches here (engine/ts_narrow.go). A backend that cannot honor a
	// non-any value MUST reject the install rather than widen it.
	SrcPort PortMatch
	DstPort PortMatch
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

// SAInfo is one Security Association as the DATAPLANE holds it, returned by
// ListSAs.
//
// It reports the kernel's SAD, never the IKE engine's belief about it. The two
// disagree whenever the kernel expires an SA, an operator flushes it, or a rekey
// strands one, and reporting that disagreement is why this type exists.
//
// NO KEY MATERIAL IS CARRIED. The kernel returns the encryption and integrity
// keys on every dump (netlink.XfrmState carries Auth, Crypt and Aead, each with
// a Key), and this type keeps the algorithm NAME and the key LENGTH instead. A
// key that reaches this struct reaches a terminal, a log, and a `| json` pipe.
type SAInfo struct {
	SPI  uint32
	Src  net.IP
	Dst  net.IP
	IfID uint32

	// Proto is the IPsec transform: ESP = 50, AH = 51.
	Proto uint8
	// Mode is ModeTransport or ModeTunnel.
	Mode  uint8
	ReqID uint32

	// Encryption names the cipher. For an AEAD cipher it names the combined
	// transform and Integrity is empty, because RFC 7296 Section 3.3.2 lets an
	// AEAD transform carry integrity itself rather than negotiating a separate
	// one. An empty Integrity beside a non-empty Encryption is therefore a fact
	// about the negotiation, not a missing read.
	Encryption        string
	EncryptionKeyBits int
	Integrity         string
	IntegrityKeyBits  int

	ReplayWindow uint32

	// BytesCurrent and PacketsCurrent are what the SA has carried. They come
	// from the kernel because the IKE engine never sees ESP payload: counting in
	// userspace would report zero forever.
	BytesCurrent   uint64
	PacketsCurrent uint64
	// BytesHard and PacketsHard are the lifetime ceilings. Zero means no limit.
	BytesHard   uint64
	PacketsHard uint64

	// AddedAt is when the kernel accepted the SA. UsedAt is when it last carried
	// a packet, and is the zero time when it has carried none.
	AddedAt time.Time
	UsedAt  time.Time
}

// PolicyInfo is one Security Policy as the DATAPLANE holds it, returned by
// ListPolicies.
//
// RFC 4301 Section 4.4 defines the SPD and the SAD as two databases, and this
// type is deliberately not merged into SAInfo: a policy with no matching state
// is the failure the read surface exists to show, and a merged view hides it.
type PolicyInfo struct {
	// Src and Dst are the SELECTOR, the inner traffic the policy matches. A nil
	// prefix is the wildcard.
	Src     *net.IPNet
	Dst     *net.IPNet
	SrcPort PortMatch
	DstPort PortMatch
	Dir     SADir
	// UpperProto is the upper-layer protocol the selector matches. Zero is any.
	UpperProto uint8
	Priority   int
	IfIndex    int
	IfID       uint32
	Action     SPAction

	// TunnelSrc and TunnelDst are the TEMPLATE endpoints, single addresses, not
	// prefixes. They are how the kernel resolves this policy to a state. Both are
	// nil for a bypass policy, which carries no template at all.
	TunnelSrc net.IP
	TunnelDst net.IP
	Mode      uint8
	ReqID     uint32

	// Owner names the peer that installed this policy, and OwnerKnown says
	// whether ze installed it at all.
	//
	// A false OwnerKnown is a legitimate and common answer, not an error: the
	// kernel SPD holds every policy on the node, including ones another daemon
	// or the operator installed. A renderer MUST say so rather than leaving the
	// field blank, because a blank owner reads as "unowned" when the truth is
	// "not ours" (ai/rules/evidence.md).
	Owner      string
	OwnerKnown bool
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

	// ListSAs dumps the SAD. A zero ifID means every if_id; any other value
	// filters on it.
	//
	// A backend that cannot enumerate MUST return ErrNotSupported and a nil
	// slice, never an empty slice and a nil error. The two are indistinguishable
	// to a caller, and the second one renders as "no SAs are installed", which
	// answers the operator's question with a confident lie
	// (ai/rules/evidence.md).
	ListSAs(ifID uint32) ([]SAInfo, error)

	// ListPolicies dumps the SPD, every policy the dataplane holds. The same
	// ErrNotSupported obligation as ListSAs applies.
	ListPolicies() ([]PolicyInfo, error)

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
