// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- config parsing for IKE engine
package engine

import (
	"encoding/json"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/pki"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// YANG config roots this engine claims. configRootVPN carries the IPsec peers
// and proposals; configRootPKI carries the certificates and keys they cite.
const (
	configRootVPN = "vpn"
	configRootPKI = "pki"
)

// parseIPsecSections finds the "vpn" config section and parses the IPsec config.
// Also loads the "pki" section into the global PKI store if present.
func parseIPsecSections(sections []sdk.ConfigSection) (*ipsec.IPsecConfig, error) {
	for _, s := range sections {
		if s.Root == configRootPKI {
			if err := loadPKIFromJSON(s.Data); err != nil {
				return nil, err
			}
		}
	}

	for _, s := range sections {
		if s.Root != configRootVPN {
			continue
		}
		return parseIPsecFromJSON(s.Data)
	}
	return &ipsec.IPsecConfig{
		ESPGroups: make(map[string]ipsec.ESPGroup),
		IKEGroups: make(map[string]ipsec.IKEGroup),
		Peers:     make(map[string]ipsec.SiteToSitePeer),
	}, nil
}

func loadPKIFromJSON(data string) error {
	cfg, err := pki.ParseJSON(data)
	if err != nil {
		return err
	}
	return pki.Load(cfg)
}

// parseVPNSections parses the "vpn" section only. Unlike parseIPsecSections it
// has no side effects, so it is safe on a verify path.
func parseVPNSections(sections []sdk.ConfigSection) (*ipsec.IPsecConfig, error) {
	for _, s := range sections {
		if s.Root != configRootVPN {
			continue
		}
		return parseIPsecFromJSON(s.Data)
	}
	return &ipsec.IPsecConfig{
		ESPGroups: make(map[string]ipsec.ESPGroup),
		IKEGroups: make(map[string]ipsec.IKEGroup),
		Peers:     make(map[string]ipsec.SiteToSitePeer),
	}, nil
}

// candidatePKI parses the "pki" section of a candidate delivery into a lookup
// set, without installing it. An absent section yields an empty set, so a
// certificate reference in a config that defines no PKI is correctly reported
// as unresolvable.
func candidatePKI(sections []sdk.ConfigSection) (
	hasCA, hasCert func(string) bool,
	certCN func(string) string,
	certChainLen func(string) int,
	err error,
) {
	cfg := &pki.PKIConfig{
		CACerts:      make(map[string]*pki.CACertEntry),
		Certificates: make(map[string]*pki.CertificateEntry),
	}
	for _, s := range sections {
		if s.Root != configRootPKI {
			continue
		}
		parsed, perr := pki.ParseJSON(s.Data)
		if perr != nil {
			return nil, nil, nil, nil, perr
		}
		cfg = parsed
		break
	}

	hasCA = func(name string) bool { return cfg.CACerts[name] != nil }
	hasCert = func(name string) bool { return cfg.Certificates[name] != nil }
	certCN = func(name string) string {
		entry := cfg.Certificates[name]
		if entry == nil || entry.Certificate == nil {
			return ""
		}
		return entry.Certificate.Subject.CommonName
	}
	// The count ze would put on the wire: the device certificate plus its intermediates,
	// which is exactly what localCertChain assembles. A name the candidate section does
	// not carry reports 0, and ValidatePKIRefs refuses that separately.
	certChainLen = func(name string) int {
		entry := cfg.Certificates[name]
		if entry == nil {
			return 0
		}
		return 1 + len(entry.RawIntermediates)
	}
	return hasCA, hasCert, certCN, certChainLen, nil
}

// validateIPsecSections parses the delivered config sections and runs the
// site-to-site cross-reference checks: group references, peer PKI references
// (including the RFC 5216 Section 5.3 trust-anchor requirement for EAP-TLS
// peers), and the remote-access pool and user credentials. It does NOT check
// the remote-access gateway's own certificate references -- that whole config
// surface is inert today and is owned by plan/spec-ipsec-remote-access.md.
//
// It deliberately does NOT check the interface binding either: interface
// existence is a HOST fact, so ValidateInterfaceRef is driven from the ike
// plugin's ze doctor check (engine/doctor.go) instead. Verifying it here would
// reject a config-first deployment that names an interface the same commit
// creates, and this plugin's ConfigRoots do not carry the interfaces section.
//
// This is the plugin's OnConfigVerify body. Before it existed, none of the
// IPsecConfig validators had a non-test caller anywhere in the repo, so a config
// naming a missing ike-group, a missing certificate, or an EAP-TLS peer with no
// CA was accepted and only failed later, at session setup, or not at all.
//
// It is SIDE-EFFECT FREE, which the InProcessConfigVerifier contract requires
// (internal/component/plugin/registry/registry.go) and which correctness requires
// independently: certificate names are resolved against the CANDIDATE pki
// section, parsed into a throwaway lookup set, never by installing it. Verifying
// against the live store would both judge the new config by the old PKI and, via
// pki.Load, leave a rejected config's certificates installed in a running daemon.
func validateIPsecSections(sections []sdk.ConfigSection) error {
	cfg, err := parseVPNSections(sections)
	if err != nil {
		return err
	}
	hasCA, hasCert, certCN, certChainLen, err := candidatePKI(sections)
	if err != nil {
		return err
	}
	if err := cfg.ValidateGroupRefs(); err != nil {
		return err
	}
	if err := cfg.ValidatePKIRefs(hasCA, hasCert, certCN); err != nil {
		return err
	}
	if err := cfg.ValidateCertificateChains(certChainLen); err != nil {
		return err
	}
	if err := cfg.ValidateIdentities(); err != nil {
		return err
	}
	if err := cfg.ValidatePolicyOrder(); err != nil {
		return err
	}
	// The operator's own SPD entries take the same reserved-band rule as a peer's
	// rank, plus the selector rules no negotiation can enforce for them: nothing
	// narrows an operator entry, so config verify is the only gate it passes
	// (spd_policy.go, ValidateSPDPolicies).
	if err := cfg.ValidateSPDPolicies(); err != nil {
		return err
	}
	// ai/rules/protocol.md: a traffic selector the dataplane cannot program
	// byte for byte is refused HERE, at ze config verify and ze config commit, never
	// approximated at negotiation time. The peer's own proposal never reaches this
	// function, so ts_narrow.go applies the same predicate to attacker-controlled
	// selectors.
	if err := cfg.ValidateTrafficSelectors(); err != nil {
		return err
	}
	return cfg.ValidateRemoteAccess()
}

// parseIPsecFromJSON parses the JSON config section data into IPsecConfig.
func parseIPsecFromJSON(data string) (*ipsec.IPsecConfig, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return nil, err
	}
	tree, err := config.TreeFromPluginMap(raw)
	if err != nil {
		return nil, err
	}
	return ipsec.ParseIPsecConfig(tree)
}
