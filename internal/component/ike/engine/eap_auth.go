// Design: plan/learned/744-ipsec-9-ikev2-eap-nat.md -- AUTH payload from EAP MSK
// RFC: rfc/short/rfc7296.md -- Section 2.16: AUTH = prf(prf(MSK, "Key Pad for IKEv2"), signed_octets)

package engine

import (
	"fmt"
	"net"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/eap"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

var keyPadForIKEv2 = []byte("Key Pad for IKEv2")

// ComputeAuthFromMSK computes the IKEv2 AUTH payload from an EAP Master Session Key.
// RFC 7296 Section 2.16: AUTH = prf(prf(MSK, "Key Pad for IKEv2"), <signed octets>).
func ComputeAuthFromMSK(prfID crypto.PRFID, msk [64]byte, signedOctets []byte) ([]byte, error) {
	sk, err := crypto.PRF(prfID, msk[:], keyPadForIKEv2)
	if err != nil {
		return nil, err
	}
	auth, err := crypto.PRF(prfID, sk, signedOctets)
	clear(sk)
	if err != nil {
		return nil, err
	}
	return auth, nil
}

// VerifyAuthFromMSK verifies an AUTH payload against the expected MSK-derived value.
func VerifyAuthFromMSK(prfID crypto.PRFID, msk [64]byte, signedOctets, receivedAuth []byte) error {
	expected, err := ComputeAuthFromMSK(prfID, msk, signedOctets)
	if err != nil {
		return err
	}
	if !constantTimeEqualAuth(expected, receivedAuth) {
		return errAuthFailed
	}
	return nil
}

func constantTimeEqualAuth(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

// natHashEqual compares NAT detection hashes.
// RFC 7296 Section 2.23: SHA-1 hashes are 20 bytes.
func natHashEqual(a, b []byte) bool {
	return constantTimeEqualAuth(a, b)
}

// sendWithNATT sends an IKE message, prepending the non-ESP marker if NAT is detected.
// RFC 3948 Section 2.2.
func sendWithNATT(sa *SA, data []byte, tr *transport.UDPTransport, remote *net.UDPAddr) error {
	if sa.NATDetected {
		data = transport.AddNonESPMarker(data)
		remote = &net.UDPAddr{IP: remote.IP, Port: transport.NATTPort}
	}
	return tr.Send(data, remote)
}

// NewEAPSession creates an EAP session for a peer configured with EAP authentication.
func NewEAPSession(authMode ipsec.AuthMode, config eap.MethodConfig) (*eap.Session, error) {
	var methodType uint8
	switch authMode {
	case ipsec.AuthEAPMSCHAPv2:
		methodType = eap.TypeMSCHAPv2
	case ipsec.AuthEAPTLS:
		methodType = eap.TypeTLS
	default:
		return nil, fmt.Errorf("ike: auth mode %s is not an EAP method", authMode)
	}
	return eap.NewSession(methodType, config)
}

// computeEAPAuth computes AUTH from the EAP MSK stored on the SA.
// RFC 7296 Section 2.16: after EAP success, AUTH = prf(prf(MSK, "Key Pad for IKEv2"), signed_octets).
func computeEAPAuth(sa *SA) (*wire.PayloadAUTH, error) {
	signedOctets, err := computeSignedOctets(sa, sa.IsInitiator)
	if err != nil {
		return nil, err
	}
	authData, err := ComputeAuthFromMSK(sa.Proposal.PRF.ID, sa.EAPMSK, signedOctets)
	if err != nil {
		return nil, err
	}
	return &wire.PayloadAUTH{
		AuthMethod: wire.AuthMethodPSK,
		AuthData:   authData,
	}, nil
}
