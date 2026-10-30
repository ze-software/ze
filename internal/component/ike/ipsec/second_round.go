// Design: docs/architecture/ike/ipsec-3-data-model.md -- IPsec data model types
// RFC: rfc/short/rfc4739.md -- multiple authentication exchanges in IKEv2
// Related: config.go -- parseAuthConfig, which calls parseSecondRound
// Related: validate.go -- validateIdentityLeaves, the identity rules both rounds share

package ipsec

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/config"
)

// Config paths the second-round parser and validator name in their errors, so an
// operator reads the leaf to edit rather than a Go field name.
const (
	secondRoundLocalPath  = "authentication second-round local"
	secondRoundRemotePath = "authentication second-round remote"
)

// errSecondRoundModeMissing is returned for a round container with no mode leaf.
// The YANG makes mode mandatory inside the presence container, so only a tree that
// skipped the schema reaches it.
var errSecondRoundModeMissing = errors.New("mode is missing; set it to pre-shared-secret or x509")

// parseSecondRound reads the second-round container of a site-to-site peer's
// authentication into auth.Additions.SecondRound. It leaves it nil when neither
// local nor remote is configured, so the peer runs one round each way. The
// first-round leaves of auth MUST be parsed before it runs: each round takes the
// peer's certificate-count from them.
//
// RFC 4739 Section 2.1: "The IKE_AUTH phase is considered successful only if all
// the individual authentication exchanges complete successfully."
func parseSecondRound(peerName string, t *config.Tree, auth *AuthConfig) error {
	container := t.GetContainer("second-round")
	if container == nil {
		return nil
	}
	var rounds SecondRoundAuth
	if local := container.GetContainer("local"); local != nil {
		round, err := parseSecondRoundDirection(peerName, secondRoundLocalPath, local, auth)
		if err != nil {
			return err
		}
		if v, ok := local.Get("id"); ok {
			round.LocalID = v
		}
		rounds.Local = round
	}
	if remote := container.GetContainer("remote"); remote != nil {
		round, err := parseSecondRoundDirection(peerName, secondRoundRemotePath, remote, auth)
		if err != nil {
			return err
		}
		if v, ok := remote.Get("id"); ok {
			round.RemoteID = v
		}
		if v, ok := remote.Get("id-type"); ok && v != "" {
			idType, known := parseRemoteIDType(v)
			if !known {
				return fmt.Errorf(
					"ipsec peer %q %s id-type: unsupported value %q (valid: %v)",
					peerName, secondRoundRemotePath, v, RemoteIDTypeNames())
			}
			round.RemoteIDType = idType
		}
		rounds.Remote = round
	}
	// An empty second-round container configures no round, so nothing is added.
	if rounds.Local == nil && rounds.Remote == nil {
		return nil
	}
	if auth.Additions == nil {
		auth.Additions = &AuthAdditions{}
	}
	auth.Additions.SecondRound = &rounds
	return nil
}

// parseSecondRoundDirection reads the leaves local and remote share: mode, the
// pre-shared secret with its encoding, and the PKI name. Only pre-shared-secret
// and x509 are accepted, because an EAP method cannot run in a second round here.
func parseSecondRoundDirection(peerName, path string, t *config.Tree, round1 *AuthConfig) (*AuthConfig, error) {
	modeName, ok := t.Get("mode")
	if !ok {
		return nil, fmt.Errorf("ipsec peer %q %s: %w", peerName, path, errSecondRoundModeMissing)
	}
	mode, valid := ParseAuthMode(modeName)
	if !valid {
		return nil, fmt.Errorf(
			"ipsec peer %q %s mode: unsupported value %q (valid: pre-shared-secret, x509)",
			peerName, path, modeName)
	}
	if mode != AuthPreSharedSecret && mode != AuthX509 {
		return nil, fmt.Errorf(
			"ipsec peer %q %s mode: %s cannot run in a second round; set it to pre-shared-secret or x509",
			peerName, path, mode)
	}

	auth := &AuthConfig{Mode: mode, CertificateCount: round1.CertificateCount}
	if mode == AuthPreSharedSecret {
		psk, err := parsePreSharedSecret(peerName, t)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		auth.PSK = psk
	}
	if mode == AuthX509 {
		readCertificateRefs(t, auth)
	}
	return auth, nil
}

// ValidateSecondRounds refuses at commit a second authentication round the
// engine could not run as written.
//
// It refuses a second round on a peer whose first round is an EAP method, a round
// of mode pre-shared-secret with no secret, a local x509 round with no certificate
// and a remote x509 round with no ca-certificate. It applies the identity rules
// of local-id and remote-id to the round ids, and checks the round's PKI names
// against the store as ValidatePKIRefs checks the first round's.
func (c *IPsecConfig) ValidateSecondRounds(hasCA, hasCert func(string) bool, certCN func(string) string) error {
	for name := range c.Peers {
		auth := c.Peers[name].Auth
		rounds := auth.SecondRound()
		if rounds == nil {
			continue
		}
		if IsEAPMode(auth.Mode) {
			return fmt.Errorf(
				"ipsec peer %q: authentication second-round needs a first round of "+
					"pre-shared-secret or x509, and mode is %s; an EAP method cannot be "+
					"followed by a second round. Delete authentication second-round, or "+
					"set authentication mode to pre-shared-secret or x509",
				name, auth.Mode)
		}
		local, remote := rounds.Local, rounds.Remote
		if local != nil {
			if err := validateSecondRoundLocal(name, local, hasCert, certCN); err != nil {
				return err
			}
		}
		if remote != nil {
			if err := validateSecondRoundRemote(name, remote, hasCA); err != nil {
				return err
			}
		}
		if err := validateSecondRoundIdentities(name, local, remote); err != nil {
			return err
		}
	}
	return nil
}

// validateSecondRoundLocal checks the credential ze proves in its second round.
func validateSecondRoundLocal(name string, local *AuthConfig, hasCert func(string) bool, certCN func(string) string) error {
	if local.Mode == AuthPreSharedSecret && local.PSK == "" {
		return fmt.Errorf(
			"ipsec peer %q: %s mode is pre-shared-secret and no secret is set; "+
				"set %s pre-shared-secret", name, secondRoundLocalPath, secondRoundLocalPath)
	}
	// The round asserts an identity of its own. Falling back to the peer name, as the
	// first round does, would assert the same identity twice, and a peer that picks
	// the round's pre-shared key by identity (strongSwan's psk_authenticator) would
	// then look up the first round's key.
	if local.LocalID == "" {
		return fmt.Errorf(
			"ipsec peer %q: %s id is not set, and the second round must assert an identity "+
				"of its own; set %s id to the identity the peer expects for that round",
			name, secondRoundLocalPath, secondRoundLocalPath)
	}
	if local.Mode != AuthX509 {
		return nil
	}
	if local.Certificate == "" {
		return fmt.Errorf(
			"ipsec peer %q: %s mode is x509 and no certificate is set, so ze has nothing "+
				"to sign the round's AUTH with; set %s certificate to a PKI store name",
			name, secondRoundLocalPath, secondRoundLocalPath)
	}
	if !hasCert(local.Certificate) {
		return fmt.Errorf(
			"ipsec peer %q: %s certificate %q not found in PKI store; add it under pki "+
				"or name a certificate the store holds", name, secondRoundLocalPath, local.Certificate)
	}
	// The same binding ValidatePKIRefs holds the first x509 round to: the identity
	// ze asserts must be the one its certificate names.
	if cn := certCN(local.Certificate); cn != "" && local.LocalID != cn {
		return fmt.Errorf(
			"ipsec peer %q: %s id %q does not match certificate CN %q; set %s id to %q",
			name, secondRoundLocalPath, local.LocalID, cn, secondRoundLocalPath, cn)
	}
	return nil
}

// validateSecondRoundRemote checks the credential the peer must prove in its
// second round.
func validateSecondRoundRemote(name string, remote *AuthConfig, hasCA func(string) bool) error {
	if remote.Mode == AuthPreSharedSecret && remote.PSK == "" {
		return fmt.Errorf(
			"ipsec peer %q: %s mode is pre-shared-secret and no secret is set; "+
				"set %s pre-shared-secret", name, secondRoundRemotePath, secondRoundRemotePath)
	}
	if remote.Mode != AuthX509 {
		return nil
	}
	// Without a trust anchor any self-signed certificate would pass the round, the
	// reason ValidatePKIRefs demands ca-certificate of a first x509 round.
	if remote.CACertificate == "" {
		return fmt.Errorf(
			"ipsec peer %q: %s mode is x509 and no ca-certificate is set, so ze could not "+
				"validate the peer's certificate; set %s ca-certificate to a PKI store name",
			name, secondRoundRemotePath, secondRoundRemotePath)
	}
	if !hasCA(remote.CACertificate) {
		return fmt.Errorf(
			"ipsec peer %q: %s ca-certificate %q not found in PKI store; add it under pki "+
				"or name a CA the store holds", name, secondRoundRemotePath, remote.CACertificate)
	}
	return nil
}

// validateSecondRoundIdentities applies the local-id and remote-id rules to the
// round ids. A direction left unconfigured contributes an empty identity, which
// every rule accepts.
func validateSecondRoundIdentities(name string, local, remote *AuthConfig) error {
	localID := identityLeaf{leaf: secondRoundLocalPath + " id"}
	if local != nil {
		localID.value = local.LocalID
	}
	remoteID := identityLeaf{leaf: secondRoundRemotePath + " id"}
	var remoteIDType uint8
	if remote != nil {
		remoteID.value = remote.RemoteID
		remoteIDType = remote.RemoteIDType
	}
	return validateIdentityLeaves(name, localID, remoteID, remoteIDType)
}
