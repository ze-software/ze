// Design: plan/learned/734-ipsec-3-data-model.md -- IPsec cross-reference validation

package ipsec

import (
	"fmt"
	"net/netip"
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

// ValidatePKIRefs checks that X.509 auth references (ca-certificate,
// certificate) exist in the PKI store, and that local-id matches the
// certificate's subject CN.
func (c *IPsecConfig) ValidatePKIRefs(hasCA, hasCert func(string) bool, certCN func(string) string) error {
	for name := range c.Peers {
		peer := c.Peers[name]
		if peer.Auth.Mode != AuthX509 && peer.Auth.Mode != AuthEAPTLS {
			continue
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
		if ca := peer.Auth.CACertificate; ca != "" && !hasCA(ca) {
			return fmt.Errorf("ipsec peer %q: ca-certificate %q not found in PKI store", name, ca)
		}
		cert := peer.Auth.Certificate
		if cert != "" {
			if !hasCert(cert) {
				return fmt.Errorf("ipsec peer %q: certificate %q not found in PKI store", name, cert)
			}
			if peer.Auth.LocalID != "" {
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
