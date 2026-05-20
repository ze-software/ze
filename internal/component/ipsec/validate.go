// Design: plan/spec-ipsec-3-data-model.md -- IPsec cross-reference validation

package ipsec

import "fmt"

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
		if peer.Auth.Mode != AuthX509 {
			continue
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
