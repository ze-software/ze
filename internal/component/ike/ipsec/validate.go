// Design: plan/learned/734-ipsec-3-data-model.md -- IPsec cross-reference validation
// RFC: rfc/short/rfc7296.md -- Section 2.16, EAP responder public-key authentication
// Related: identity.go -- the remote-id-type name-to-number mapping these checks read

package ipsec

import (
	"fmt"
	"net/netip"
	"strings"
)

// ValidateGroupRefs checks that every peer's ike-group and esp-group
// references point to defined groups within the config.
func (c *IPsecConfig) ValidateGroupRefs() error {
	for name := range c.Peers {
		peer := c.Peers[name]
		if peer.IKEGroup != "" {
			if _, ok := c.IKEGroups[peer.IKEGroup]; !ok {
				return fmt.Errorf("ipsec peer %q: ike-group %q not defined", name, peer.IKEGroup)
			}
		}
		if peer.ESPGroup != "" {
			if _, ok := c.ESPGroups[peer.ESPGroup]; !ok {
				return fmt.Errorf("ipsec peer %q: esp-group %q not defined", name, peer.ESPGroup)
			}
		}
	}
	return nil
}

// IsEAPMode reports whether an auth mode runs the EAP exchange. RFC 7296
// Section 2.16 puts an extra obligation on these modes, so several layers need
// to ask the question.
func IsEAPMode(mode AuthMode) bool {
	return mode == AuthEAPMSCHAPv2 || mode == AuthEAPTLS
}

// rdnAttributes are the X.500 attribute names an operator writes at the head of a
// distinguished name. RFC 4514 Section 3 gives the short forms, and OpenSSL and
// strongSwan add a few more.
var rdnAttributes = map[string]bool{
	"c": true, "cn": true, "dc": true, "e": true, "emailaddress": true,
	"g": true, "gn": true, "l": true, "o": true, "ou": true,
	"postalcode": true, "serialnumber": true, "sn": true, "st": true,
	"street": true, "t": true, "uid": true,
}

// IsDistinguishedName reports whether an identity value is written as an X.500
// distinguished name. The test is the FIRST relative distinguished name: an attribute
// name from rdnAttributes, then an equals sign. A value such as "gw=1" is not one, so an
// opaque key id that happens to hold an equals sign still passes.
//
// It is exported because the engine's identity classifier must make the SAME reading as
// the commit-time validator. Two independent notions of "this value is a DN" would let a
// config commit on one reading and then be compared under the other
// (ai/rules/derive-not-hardcode.md).
func IsDistinguishedName(value string) bool {
	return distinguishedName(value)
}

func distinguishedName(value string) bool {
	head, _, found := strings.Cut(value, "=")
	if !found {
		return false
	}
	return rdnAttributes[strings.ToLower(strings.TrimSpace(head))]
}

// ValidateIdentities refuses a local-id that no IKE_AUTH can ever satisfy.
//
// A distinguished-name LOCAL-id is still refused. encodeIKEID derives the type ze sends
// from the shape of the value, and it has no ID_DER_ASN1_DN branch. It would therefore
// send the literal text as ID_FQDN, and a peer expecting a distinguished name refuses
// that. Ze cannot deliver what the config asks for, so the config does not commit
// (ai/rules/exact-or-reject.md).
//
// A distinguished-name REMOTE-id is now accepted. The refusal that used to cover it was
// lifted in the same change that gave the engine the capability. RFC 7296 Section 4
// requires it be possible to configure ze to accept a PKIX certificate "where the ID
// passed is any of ID_KEY_ID, ID_FQDN, ID_RFC822_ADDR, or ID_DER_ASN1_DN". assertedIdentity
// now renders the asserted DER in RFC 4514 form for the policy comparison.
// certificateCarriesIdentity then binds it against the certificate's own RawSubject with
// bytes.Equal, which is exact and needs no canonical text form at all. The objection this
// function used to record was correct about comparing a DN with operator TEXT and does not
// reach the certificate comparison.
func (c *IPsecConfig) ValidateIdentities() error {
	for name := range c.Peers {
		auth := c.Peers[name].Auth
		// RFC 7296 Section 3.5 MUST NOT: "The ID_FQDN and ID_RFC822_ADDR strings MUST
		// NOT contain any terminators (e.g., NULL, CR, etc.)."
		//
		// THE CHECK IS PER ID TYPE, because the prohibition is. Section 3.5 states it
		// for ID_FQDN and repeats it for ID_RFC822_ADDR, and those two alone. ID_KEY_ID
		// is "an opaque octet stream", and ID_DER_ASN1_DN is "the binary Distinguished
		// Encoding Rules (DER) encoding" -- binary forms in which a control octet is
		// ordinary content, not a terminator.
		//
		// Applying the string rule to every type refused at COMMIT a key-id the wire
		// path accepts on receipt (refuseIDTerminators, engine/remote_id.go, gates on
		// the same two types). Two gates over one obligation disagreeing means one of
		// them is wrong, and Section 3.5 says which: the config gate was.
		//
		// local-id needs no type test. encodeIKEID (engine/auth.go) derives the type ze
		// SENDS from the value's shape, and it has exactly two branches: an IP address,
		// or ID_FQDN. So a local-id is always one of the two types the prohibition
		// covers, or an address in which a terminator cannot appear.
		//
		// Both refusals land at COMMIT rather than at encode time. encodeIKEID would put
		// local-id on the wire verbatim, and a silent repair there would send an identity
		// the operator never wrote (ai/rules/exact-or-reject.md). remote-id is checked
		// for the same reason from the other direction: a conformant peer can never
		// assert an ID_FQDN or ID_RFC822_ADDR holding a terminator, so such a remote-id
		// matches nothing and the tunnel would fail authentication with no stated cause.
		checked := []struct{ leaf, value string }{{"local-id", auth.LocalID}}
		if remoteIDIsTerminatorFree(auth.RemoteIDType) {
			checked = append(checked, struct{ leaf, value string }{"remote-id", auth.RemoteID})
		}
		for _, id := range checked {
			if c, found := IDTerminator(id.value); found {
				return fmt.Errorf(
					"ipsec peer %q: %s %q contains the terminator octet %#02x, and RFC 7296 "+
						"Section 3.5 forbids a terminator in an ID_FQDN or ID_RFC822_ADDR "+
						"string. Remove the control character",
					name, id.leaf, id.value, c)
			}
		}
		if distinguishedName(auth.LocalID) {
			return fmt.Errorf(
				"ipsec peer %q: local-id %q is a distinguished name, and ze sends the identity "+
					"type derived from the value's shape, which has no ID_DER_ASN1_DN form. "+
					"Set it to an address, an FQDN, a mail address, or a key id",
				name, auth.LocalID)
		}
	}
	return nil
}

// ValidatePKIRefs checks that X.509 and EAP auth references (ca-certificate,
// certificate) exist in the PKI store, and that local-id matches the
// certificate's subject CN.
func (c *IPsecConfig) ValidatePKIRefs(hasCA, hasCert func(string) bool, certCN func(string) string) error {
	for name := range c.Peers {
		peer := c.Peers[name]
		if peer.Auth.Mode != AuthX509 && !IsEAPMode(peer.Auth.Mode) {
			continue
		}
		// RFC 7296 Section 2.16 covers EAP. It says such methods "MUST be used in
		// conjunction with a public-key-signature-based authentication of the
		// responder to the initiator". The responder signs its AUTH with the
		// certificate named here. An EAP peer without one cannot meet that
		// obligation. Refuse the config rather than let the daemon decide at
		// session setup (ai/rules/exact-or-reject.md).
		//
		// Before this check, eap-mschapv2 with no certificate was accepted. The
		// responder then signed its AUTH with a pre-shared key, which is not a
		// public-key signature. The runtime refuses it too, in
		// engine.computeServerAuth.
		if IsEAPMode(peer.Auth.Mode) && peer.Auth.Certificate == "" {
			return fmt.Errorf(
				"ipsec peer %q: %s requires a certificate so the responder can sign its AUTH, "+
					"set authentication certificate to a PKI store name (RFC 7296 Section 2.16)",
				name, peer.Auth.Mode)
		}
		// RFC 5216 Section 5.3: "Both sides MUST perform certificate path
		// validation." An EAP-TLS peer validates the authenticator against its
		// configured trust anchor and has no other way to do so, because EAP
		// carries no server hostname to check. Without one the session
		// authenticates nothing, so the config is refused here rather than at
		// session setup (ai/rules/exact-or-reject.md). The runtime refuses it too,
		// in eap.PeerSession.startTLSClient and engine.buildPeerTLSConfig.
		if peer.Auth.Mode == AuthEAPTLS && peer.Auth.CACertificate == "" {
			return fmt.Errorf(
				"ipsec peer %q: eap-tls requires a ca-certificate to validate the authenticator (RFC 5216 Section 5.3)",
				name)
		}
		// The certificate above is only half of the obligation. RFC 7296
		// Section 2.16 wants a public-key signature the INITIATOR can attribute to
		// the responder, and a signature it cannot chain to a trust anchor
		// attributes nothing. ca-certificate is the only anchor Ze holds, so
		// without it engine.getRemoteCert had no chain to build and any
		// self-signed certificate carrying a valid signature authenticated. For
		// eap-mschapv2 Ze then sent the user challenge and response to whoever
		// answered. The same reasoning covers x509, which authenticates the peer
		// by that certificate alone. The runtime refuses both too, in
		// engine.getRemoteCert.
		if IsEAPMode(peer.Auth.Mode) || peer.Auth.Mode == AuthX509 {
			if peer.Auth.CACertificate == "" {
				return fmt.Errorf(
					"ipsec peer %q: %s requires a ca-certificate to validate the remote certificate, "+
						"set authentication ca-certificate to a PKI store name (RFC 7296 Section 2.16)",
					name, peer.Auth.Mode)
			}
		}
		if ca := peer.Auth.CACertificate; ca != "" && !hasCA(ca) {
			return fmt.Errorf("ipsec peer %q: ca-certificate %q not found in PKI store", name, ca)
		}
		cert := peer.Auth.Certificate
		if cert != "" {
			if !hasCert(cert) {
				return fmt.Errorf("ipsec peer %q: certificate %q not found in PKI store", name, cert)
			}
			// X.509 only. For EAP-TLS the IKE AUTH is derived from the EAP MSK,
			// not signed by this certificate, and local-id is the EAP identity
			// (an NAI); nothing in RFC 5216 or RFC 7296 binds it to the
			// certificate subject. Requiring equality there would reject the
			// ordinary deployment of a user identity with a device certificate.
			if peer.Auth.Mode == AuthX509 && peer.Auth.LocalID != "" {
				cn := certCN(cert)
				if cn != "" && peer.Auth.LocalID != cn {
					return fmt.Errorf(
						"ipsec peer %q: local-id %q does not match certificate CN %q",
						name, peer.Auth.LocalID, cn)
				}
			}
		}
	}
	return nil
}

// ValidateCertificateChains refuses a peer whose certificate-count is smaller than the
// certificate chain its PKI entry actually holds.
//
// certChainLen reports the number of certificates a PKI entry would put on the wire: the
// device certificate plus its intermediates. It returns 0 for a name the candidate PKI
// section does not carry, and ValidatePKIRefs has already refused that case.
//
// Without this check the send path had two honest options and both are wrong. Truncating
// the chain silently sends a path the peer cannot complete. The operator then sees a peer
// that refuses ze's certificate, with nothing in ze's own logs to explain it. Ignoring the
// bound makes certificate-count a decoration on the send side. Refusing at commit is the
// third option, and it is the one ai/rules/exact-or-reject.md requires. Ze cannot deliver
// exactly what the config asks for, so the config does not commit.
func (c *IPsecConfig) ValidateCertificateChains(certChainLen func(string) int) error {
	for name := range c.Peers {
		peer := c.Peers[name]
		cert := peer.Auth.Certificate
		if cert == "" {
			continue
		}
		held := certChainLen(cert)
		limit := int(peer.Auth.CertificateChainLimit())
		if held > limit {
			return fmt.Errorf(
				"ipsec peer %q: certificate %q holds a chain of %d and certificate-count allows %d, "+
					"so ze would have to send a path the peer cannot complete. Raise "+
					"certificate-count to %d, or name fewer intermediate certificates",
				name, cert, held, limit, held)
		}
	}
	return nil
}

// ValidateInterfaceRef checks that the interface binding leaf references
// a known interface.
func (c *IPsecConfig) ValidateInterfaceRef(exists func(string) bool) error {
	if c.Interface == "" {
		return nil
	}
	if !exists(c.Interface) {
		return fmt.Errorf("ipsec: interface %q not found", c.Interface)
	}
	return nil
}

// ValidateRemoteAccess checks remote-access pool ranges and EAP user credentials.
func (c *IPsecConfig) ValidateRemoteAccess() error {
	if c.RemoteAccess == nil {
		return nil
	}
	ra := c.RemoteAccess

	if ra.Pool.Range != "" {
		if err := validatePoolPrefix(ra.Pool.Range, 8, 30); err != nil {
			return fmt.Errorf("ipsec remote-access pool %q range: %w", ra.Pool.Name, err)
		}
	}
	if ra.Pool.Range6 != "" {
		if err := validatePoolPrefix(ra.Pool.Range6, 48, 126); err != nil {
			return fmt.Errorf("ipsec remote-access pool %q range6: %w", ra.Pool.Name, err)
		}
	}

	if ra.IKEGroup != "" {
		if _, ok := c.IKEGroups[ra.IKEGroup]; !ok {
			return fmt.Errorf("ipsec remote-access: ike-group %q not defined", ra.IKEGroup)
		}
	}
	if ra.ESPGroup != "" {
		if _, ok := c.ESPGroups[ra.ESPGroup]; !ok {
			return fmt.Errorf("ipsec remote-access: esp-group %q not defined", ra.ESPGroup)
		}
	}

	for name := range ra.Users {
		user := ra.Users[name]
		if ra.Auth.Mode == AuthEAPMSCHAPv2 && user.Password == "" {
			return fmt.Errorf("ipsec eap-user %q: password is required for eap-mschapv2", name)
		}
		if ra.Auth.Mode == AuthEAPTLS && user.Certificate == "" {
			return fmt.Errorf("ipsec eap-user %q: certificate is required for eap-tls", name)
		}
	}

	return nil
}

func validatePoolPrefix(cidr string, minBits, maxBits int) error {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR %q: %w", cidr, err)
	}
	bits := prefix.Bits()
	if bits < minBits || bits > maxBits {
		return fmt.Errorf("prefix length /%d out of range /%d-/%d", bits, minBits, maxBits)
	}
	return nil
}
