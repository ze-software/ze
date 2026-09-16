// Design: docs/architecture/core-design.md -- the IPsec inventory leaf
// Related: internal/component/ike/engine/inventory.go -- the one registrant
//
// The IPsec inventory is how a feature reads the live Child SAs without importing
// the IKE engine. The engine registers one snapshot function at init(); a reader asks
// Tunnels and gets the installed state as value types. The package imports the
// standard library only, so any owner registers from init() with no cycle, and a build
// that drops the IKE component still compiles every reader: that reader then gets
// ErrNotRegistered rather than an empty list (ai/rules/principles.md).

package ipsecinventory

import (
	"errors"
	"net/netip"
	"sync"
)

// Mode is the encapsulation mode a Child SA was installed with.
//
// RFC 4301 Section 4.1: "An SA is a simplex 'connection' that affords security
// services to the traffic carried by it ... in transport mode ... or tunnel mode".
// The zero value is a mode nobody set, so a Tunnel whose child is down cannot pass
// for a tunnel-mode SA.
type Mode uint8

const (
	ModeUnspecified Mode = iota
	ModeTransport
	ModeTunnel
)

// String is for display only; never compare with it.
func (m Mode) String() string {
	switch m {
	case ModeTransport:
		return "transport"
	case ModeTunnel:
		return "tunnel"
	default:
		return "unspecified"
	}
}

// EncryptionID is an IKEv2 Transform Type 1 (ENCR) identifier as IANA assigns it
// (RFC 7296 Section 3.3.2). It is the same number the SA payload carried on the wire,
// so a reader derives the cipher's IV, ICV and block sizes from it and the key length
// without parsing a name. The IKE crypto package declares the assigned values.
type EncryptionID uint16

// IntegrityID is an IKEv2 Transform Type 3 (INTEG) identifier as IANA assigns it
// (RFC 7296 Section 3.3.2). Zero is AUTH_NONE: the accepted proposal is AEAD and
// carries its own ICV.
type IntegrityID uint16

// Tunnel is one configured site-to-site peer and the Child SA it holds, as INSTALLED.
//
// It is a self-contained value type: it crosses a component boundary and carries no
// pointer into engine state (ai/rules/plugins.md). Every field below Up describes the
// Child SA and is meaningful only while Up is true; a peer whose tunnel is down still
// appears, so a reader can tell "down" from "absent".
type Tunnel struct {
	// Peer is the configured peer name.
	Peer string
	// ConfiguredRemote is the peer's configured remote address. It is invalid when
	// the configured value is not an address literal: a DNS hostname, or "any" on a
	// respond-only peer. InstalledRemote then holds the address the tunnel really uses.
	ConfiguredRemote netip.Addr
	// Up is true while a Child SA is installed for this peer.
	Up bool
	// InstalledRemote is the endpoint the Child SA was installed on. Behind a NAT it
	// differs from ConfiguredRemote, and it is the address ESP is sent to.
	InstalledRemote netip.Addr
	// InstalledLocal is the local endpoint the Child SA was installed on.
	InstalledLocal netip.Addr
	// IfID is the XFRM interface id the Child SA is bound to, zero when policy-based.
	IfID uint32
	// UDPEncap is true when the Child SA receives UDP-encapsulated ESP on port 4500,
	// which adds an 8-octet UDP header to every packet.
	UDPEncap bool
	// Mode is the installed encapsulation mode.
	Mode Mode
	// EncryptionName and IntegrityName are the negotiated transforms for display, in
	// the configuration vocabulary. IntegrityName is "none" for an AEAD cipher.
	EncryptionName string
	IntegrityName  string
	// Encryption, EncryptionKeyBits and Integrity are the same negotiated transforms
	// as typed identifiers, for a reader that derives from the algorithm.
	Encryption        EncryptionID
	EncryptionKeyBits uint16
	Integrity         IntegrityID
}

// Snapshot answers the live tunnels. It is called on the reader's goroutine, so the
// registrant MUST take its own locks and MUST return copies that share nothing with
// its state. Tunnels calls it under no lock of this package.
type Snapshot func() []Tunnel

// ErrNotRegistered is the answer when no IPsec engine registered a snapshot: the
// component is not in this build. It is distinct from a registered engine holding no
// tunnel, which answers an empty list and no error.
var ErrNotRegistered = errors.New("no ipsec inventory is registered: the ike component is not in this build")

var (
	mu       sync.RWMutex
	provider Snapshot
)

// Register installs the one snapshot function. It is called from an owner's init(),
// so a nil function or a second registrant is a programmer error and panics.
func Register(fn Snapshot) {
	if fn == nil {
		panic("BUG: ipsecinventory.Register called with a nil snapshot")
	}
	mu.Lock()
	defer mu.Unlock()
	if provider != nil {
		panic("BUG: ipsecinventory.Register called twice")
	}
	provider = fn
}

// Tunnels answers the registered snapshot, or ErrNotRegistered when no engine
// registered one. Safe for concurrent use.
func Tunnels() ([]Tunnel, error) {
	mu.RLock()
	fn := provider
	mu.RUnlock()
	if fn == nil {
		return nil, ErrNotRegistered
	}
	return fn(), nil
}

// ResetForTest clears the registration so a test can register its own provider.
func ResetForTest() {
	mu.Lock()
	provider = nil
	mu.Unlock()
}
