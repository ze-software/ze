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

	// espAlignOctets is the alignment every ESP plaintext ends on, whatever
	// the cipher. RFC 4303 Section 2.4: "Padding also may be required,
	// irrespective of encryption algorithm requirements, to ensure that the
	// resulting ciphertext terminates on a 4-byte boundary." An AEAD cipher
	// imposes no block of its own, so this is its whole alignment.
	espAlignOctets = 4
)

// errOverheadRefused is the answer when the overhead cannot be derived for a
// tunnel. The caller reports the tunnel as not advisable and computes nothing
// from it: a default overhead would size a tunnel from a figure nobody
// derived (ai/rules/principles.md).
var errOverheadRefused = errors.New("mtu: the ESP overhead cannot be derived")

// espCipherWire is what one ESP cipher puts on the wire beyond the ESP header:
// the IV it transmits before the ciphertext, and the block the plaintext is
// padded to.
type espCipherWire struct {
	ivOctets    uint16
	blockOctets uint16
}

// espCipherWires is the one place these two ESP facts are written down for a
// wire Transform ID. The ICV is NOT here: an AEAD cipher's ICV is the crypto
// package's AEADICVOctets, and a CBC cipher's ICV is its integrity transform's
// TruncatedLength, so the length a packet really carries has one declaration.
//
// The keys are the ENCR ids an ESP proposal can carry into a Child SA, which
// is ipsec.SupportedESPEncryptionNames: the AES CCM ids are absent on purpose,
// because no dataplane installs one (ipsec.EncryptionImplementedESP), so an
// inventory row naming one is a defect and is refused rather than sized.
// TestESPOverheadPerTransform derives its population from that same registry,
// so a transform added there reddens the test until its row exists here.
var espCipherWires = map[crypto.EncryptionID]espCipherWire{
	// RFC 4106 Section 3.1: "The AES-GCM-ESP IV field MUST be eight octets."
	// GCM is a stream construction, so the plaintext is padded only to the
	// 4-octet ESP boundary (RFC 4303 Section 2.4, espAlignOctets).
	crypto.ENCR_AES_GCM_16: {ivOctets: 8, blockOctets: espAlignOctets},
	// RFC 3602 Section 3: "The IV field MUST be the same size as the block
	// size of the cipher algorithm being used." RFC 3602 Section 2.4: "The AES
	// uses a block size of sixteen octets (128 bits). Padding is required by
	// the AES to maintain a 16-octet (128-bit) blocksize."
	crypto.ENCR_AES_CBC: {ivOctets: 16, blockOctets: 16},
}

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
// ESP defines, or whose transform the table above does not hold.
func deriveESPOverhead(t *ipsecinventory.Tunnel) (espOverhead, error) {
	if !t.Up {
		return espOverhead{}, fmt.Errorf("%w: peer %s has no child SA installed", errOverheadRefused, t.Peer)
	}
	if !t.InstalledRemote.IsValid() {
		return espOverhead{}, fmt.Errorf("%w: peer %s has no installed endpoint", errOverheadRefused, t.Peer)
	}
	var o espOverhead
	switch t.Mode {
	case ipsecinventory.ModeTunnel, ipsecinventory.ModeTransport:
		o.mode = t.Mode
	default:
		return espOverhead{}, fmt.Errorf("%w: peer %s has an unspecified mode", errOverheadRefused, t.Peer)
	}
	o.outerHeader = ipv4HeaderOctets
	if t.InstalledRemote.Is6() {
		o.outerHeader = ipv6HeaderOctets
	}
	cipherID := crypto.EncryptionID(t.Encryption)
	wire, ok := espCipherWires[cipherID]
	if !ok {
		return espOverhead{}, fmt.Errorf("%w: peer %s negotiated ENCR transform %d (%s), which this arithmetic does not know", errOverheadRefused, t.Peer, cipherID, cipherID)
	}
	icv, err := espICVOctets(cipherID, crypto.IntegrityID(t.Integrity))
	if err != nil {
		return espOverhead{}, fmt.Errorf("%w: peer %s: %w", errOverheadRefused, t.Peer, err)
	}
	o.block = wire.blockOctets
	o.fixed = espHeaderOctets + wire.ivOctets + icv
	if t.UDPEncap {
		o.fixed += udpHeaderOctets
	}
	return o, nil
}

// espICVOctets answers the ICV one Child SA appends. An AEAD cipher carries
// its own, and the crypto package declares it per transform id; a CBC cipher
// carries its integrity transform's truncated MAC, which the same package
// declares per integrity transform. AUTH_NONE beside a CBC cipher is refused:
// RFC 4303 Section 2.8 makes integrity optional for ESP, but Ze negotiates no
// ESP proposal without it (crypto.espProposalComplete), so such a row cannot
// come from a live SA.
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
	transform, ok := integrityTransformByID(integrityID)
	if !ok {
		return 0, fmt.Errorf("INTEG transform %d (%s) is not one this build implements", integrityID, integrityID)
	}
	return transform.TruncatedLength, nil
}

// integrityTransformByID finds an integrity transform by its wire id through
// the crypto registry's name index, which is the only index it exports. The
// registry holds three entries, so the scan is bounded by that count.
func integrityTransformByID(id crypto.IntegrityID) (crypto.IntegrityTransform, bool) {
	for _, name := range crypto.SupportedIntegrityNames() {
		transform, err := crypto.LookupIntegrity(name)
		if err != nil {
			continue
		}
		if transform.ID == id {
			return transform, true
		}
	}
	return crypto.IntegrityTransform{}, false
}
