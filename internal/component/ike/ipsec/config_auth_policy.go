// Design: docs/architecture/ike/ipsec-3-data-model.md -- IPsec data model types
// RFC: rfc/short/rfc7296.md -- certificate payloads (Section 3.6), conformance set (Section 4)
// Related: config.go -- parseAuthConfig, which calls both parsers here
// Related: identity.go -- the remote-id-type name-to-number mapping these read

package ipsec

import (
	"fmt"
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/component/config"
)

// parseIdentityPolicy reads the accept-side identity policy: remote-id-type.
//
// Every value the config framework delivers is a STRING, whatever the YANG type says
// (ai/rules/config.md). The enum is therefore looked up by name rather
// than type-asserted.
func parseIdentityPolicy(peerName string, t *config.Tree, auth *AuthConfig) error {
	v, ok := t.Get("remote-id-type")
	if !ok || v == "" {
		return nil
	}
	idType, known := parseRemoteIDType(v)
	if !known {
		return fmt.Errorf(
			"ipsec peer %q remote-id-type: unsupported value %q (valid: %v)",
			peerName, v, RemoteIDTypeNames())
	}
	auth.RemoteIDType = idType
	return nil
}

// parseCertificatePolicy reads the X.509 chain and Hash-and-URL policy:
// certificate-count, hash-and-url, certificate-url and certificate-url-allow.
//
// Each malformed value is REFUSED rather than defaulted. A silently defaulted bound is
// the shape ai/rules/protocol.md forbids: the operator asked for something the
// daemon then did not do, and nothing said so.
func parseCertificatePolicy(peerName string, t *config.Tree, auth *AuthConfig) error {
	if v, ok := t.Get("certificate-count"); ok && v != "" {
		n, err := strconv.ParseUint(v, 10, 8)
		if err != nil {
			return fmt.Errorf(
				"ipsec peer %q certificate-count: %q is not a number (%w); "+
					"give a value from 1 to %d", peerName, v, err, DefaultCertificateCount)
		}
		if n < 1 || n > uint64(DefaultCertificateCount) {
			return fmt.Errorf(
				"ipsec peer %q certificate-count: %d is out of range 1..%d. RFC 7296 "+
					"Section 3.6 sets the figure ze must be configurable to reach at %d",
				peerName, n, DefaultCertificateCount, DefaultCertificateCount)
		}
		auth.CertificateCount = uint8(n)
	}

	if v, ok := t.Get("hash-and-url"); ok && v != "" {
		enabled, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf(
				"ipsec peer %q hash-and-url: %q is not a boolean (%w); give true or false",
				peerName, v, err)
		}
		auth.HashAndURL = enabled
	}

	if v, ok := t.Get("certificate-url"); ok && v != "" {
		auth.CertificateURL = v
	}

	for _, raw := range t.GetSlice("certificate-url-allow") {
		if raw == "" {
			continue
		}
		p, err := netip.ParsePrefix(raw)
		if err != nil {
			return fmt.Errorf(
				"ipsec peer %q certificate-url-allow: %q is not a CIDR prefix (%w)",
				peerName, raw, err)
		}
		auth.CertificateURLAllow = append(auth.CertificateURLAllow, p.Masked())
	}

	// hash-and-url without a certificate-url is a half-configured peer. Ze would
	// advertise HTTP_CERT_LOOKUP_SUPPORTED and then have no URL to publish. It would
	// fall back to sending the certificate inline, and the operator would never learn
	// the leaf did nothing. Refuse it at commit instead (ai/rules/protocol.md).
	if auth.HashAndURL && auth.CertificateURL == "" {
		return fmt.Errorf(
			"ipsec peer %q: hash-and-url is true and certificate-url is empty, so ze has no "+
				"URL to publish its certificate at. Set certificate-url to an http URL, or set "+
				"hash-and-url to false", peerName)
	}
	if !auth.HashAndURL && len(auth.CertificateURLAllow) > 0 {
		return fmt.Errorf(
			"ipsec peer %q: certificate-url-allow names a destination and hash-and-url is "+
				"false, so ze performs no lookup and the allowance has no effect. Set "+
				"hash-and-url to true, or remove certificate-url-allow", peerName)
	}
	return nil
}
