// Design: docs/architecture/diagnostics/path-mtu.md -- the ESP overhead, derived per transform
// RFC: rfc/short/rfc4303.md -- the ESP packet layout these octets add up
// Related: arith.go -- the ceiling, the recommended value and the MSS built on this overhead
// Related: internal/component/ike/crypto/transform.go -- the transform ids and the ICV lengths

package cmd

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/core/ipsecinventory"
)

// The fixed header sizes the ESP arithmetic adds up, in octets.
const (
	ipv4HeaderOctets = 20
	ipv6HeaderOctets = 40
	udpHeaderOctets  = 8
	tcpHeaderOctets  = 20

	// espHeaderOctets is the SPI and the Sequence Number. RFC 4303 Section 2:
	// "The packet begins with two 4-byte fields (Security Parameters Index
	// (SPI) and Sequence Number)".
	espHeaderOctets = 8

	// espTrailerOctets is the Pad Length and the Next Header octet that end
	// every ESP plaintext. RFC 4303 Section 2: "The (transmitted) ESP trailer
	// consists of the Padding, Pad Length, and Next Header fields." The
	// padding is what the alignment below accounts for; these two are fixed.
	espTrailerOctets = 2
)

// errOverheadRefused is the answer when the overhead cannot be derived for a
// tunnel. The caller reports the tunnel as not advisable and computes nothing
// from it: a default overhead would size a tunnel from a figure nobody
// derived (ai/rules/principles.md).
var errOverheadRefused = errors.New("mtu: the ESP overhead cannot be derived")

// espOverhead is what ESP adds to one packet a Child SA carries, split the way
// the ceiling arithmetic needs it (arith.go).
//
// In tunnel mode the packet handed to the SA is a whole IP packet, and ESP
// wraps it in a NEW outer header: the expansion is outerHeader plus fixed plus
// the padding to block plus the trailer. In transport mode the packet keeps
// its own IP header and ESP is inserted behind it (RFC 4303 Section 3.1.1:
// "In transport mode, ESP is inserted after the IP header and before a next
// layer protocol"), so outerHeader is the packet's own header rather than an
// addition, and only the part after it is padded. The mode is carried so the
// ceiling never applies tunnel-mode arithmetic to a transport-mode SA (AC-16).
type espOverhead struct {
	mode ipsecinventory.Mode
	// outerHeader is the IP header ESP is carried in: 20 for an IPv4
	// endpoint, 40 for IPv6. The endpoint family is the installed one.
	outerHeader uint16
	// fixed is the UDP header when encapsulated, the ESP header, the IV and
	// the ICV: every octet the SA adds that does not depend on the payload.
	fixed uint16
	// block is the alignment the plaintext, trailer included, is padded to.
	block uint16
}

// octets is the tunnel-mode expansion before padding and trailer, the figure
// the ported tool called the overhead: AES-GCM-128 over IPv4 without UDP
// encapsulation is 20 + 8 + 8 + 16 = 52 (AC-2), and 60 encapsulated (AC-3).
func (o espOverhead) octets() uint16 {
	return o.outerHeader + o.fixed
}

// deriveESPOverhead derives the overhead of one installed Child SA from the
// transform it negotiated, its mode and its encapsulation. It refuses, rather
// than defaults, a tunnel whose child is not installed, whose mode is not one
// ESP defines, whose transforms the IKE engine could not resolve, or whose
// cipher registry declares no installable ESP framing.
func deriveESPOverhead(t *ipsecinventory.Tunnel) (espOverhead, error) {
	if !t.Up {
		return espOverhead{}, fmt.Errorf("%w: peer %s has no child SA installed", errOverheadRefused, t.Peer)
	}
	// The transform fields are unset beside TransformErr, so reading them would size
	// the packet for ENCR 0.
	if t.TransformErr != nil {
		return espOverhead{}, fmt.Errorf("%w: peer %s has unknown transforms: %w", errOverheadRefused, t.Peer, t.TransformErr)
	}
	if !t.InstalledRemote.IsValid() {
		return espOverhead{}, fmt.Errorf("%w: peer %s has no installed endpoint", errOverheadRefused, t.Peer)
	}
	var o espOverhead
	switch t.Mode {
	case ipsecinventory.ModeTunnel, ipsecinventory.ModeTransport:
		o.mode = t.Mode
	case ipsecinventory.ModeUnspecified:
		return espOverhead{}, fmt.Errorf("%w: peer %s has an unspecified mode", errOverheadRefused, t.Peer)
	default:
		// The inventory mode set is open; refuse values this arithmetic does not know.
		return espOverhead{}, fmt.Errorf("%w: peer %s has an unspecified mode", errOverheadRefused, t.Peer)
	}
	o.outerHeader = ipv4HeaderOctets
	if t.InstalledRemote.Is6() {
		o.outerHeader = ipv6HeaderOctets
	}
	cipherID := crypto.EncryptionID(t.Encryption)
	// RFC 7634 Section 2, RFC 2451 Section 2.4 and RFC 4303 Section 2.4.
	ivOctets, blockOctets, err := cipherID.ESPWireSizes()
	if err != nil {
		return espOverhead{}, fmt.Errorf("%w: peer %s negotiated ENCR transform %d (%s), which this arithmetic does not know", errOverheadRefused, t.Peer, cipherID, cipherID)
	}
	icv, err := espICVOctets(cipherID, crypto.IntegrityID(t.Integrity))
	if err != nil {
		return espOverhead{}, fmt.Errorf("%w: peer %s: %w", errOverheadRefused, t.Peer, err)
	}
	o.block = blockOctets
	o.fixed = espHeaderOctets + ivOctets + icv
	if t.UDPEncap {
		o.fixed += udpHeaderOctets
	}
	return o, nil
}

// espICVOctets answers the ICV one Child SA appends. An AEAD cipher carries
// its own, and the crypto package declares it per transform id; a CBC cipher
// carries its integrity transform's truncated MAC, which the same package
// declares per integrity transform. AUTH_NONE beside a non-AEAD cipher (CBC or
// the null cipher) is refused: RFC 4303 Section 2.8 makes integrity optional
// for ESP, but no live Child SA carries that pair. crypto.espProposalComplete
// refuses ENCR_NULL beside AUTH_NONE (RFC 4303 Section 3.2), and a CBC cipher
// without a hash never matches, because crypto.matchESP requires the
// peer's integrity id to equal local policy's and an esp-group proposal with
// a non-AEAD cipher and no hash is refused at commit (ipsec.parseESPProposal).
func espICVOctets(cipherID crypto.EncryptionID, integrityID crypto.IntegrityID) (uint16, error) {
	if cipherID.IsAEAD() {
		icv, err := crypto.AEADICVOctets(cipherID)
		if err != nil {
			return 0, err
		}
		return uint16(icv), nil //nolint:gosec // an ICV is at most 32 octets
	}
	if integrityID == crypto.AUTH_NONE {
		return 0, fmt.Errorf("%s carries no integrity transform", cipherID)
	}
	// RFC 2404 Sections 2 and 3; ESP integrity includes SHA-1 without an IKE PRF.
	transform, err := crypto.LookupESPIntegrity(integrityID.String())
	if err != nil {
		return 0, fmt.Errorf("INTEG transform %d (%s) is not one this build implements", integrityID, integrityID)
	}
	return transform.TruncatedLength, nil
}
