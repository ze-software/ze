// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- config parsing for IKE engine
package engine

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/internal/core/configorder"
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
	cfg, err := parsePKIFromJSON(data)
	if err != nil {
		return err
	}
	return pki.Load(cfg)
}

// parsePKIFromJSON parses a "pki" config section WITHOUT touching the global
// store. Verification must be able to resolve certificate names in the config
// it is judging without adopting that config: pki.Load swaps the process-wide
// store outright (internal/component/pki/store.go Load) and raises expiry
// warnings, so calling it from a verify path would leave a REJECTED config's
// PKI installed in a running daemon.
func parsePKIFromJSON(data string) (*pki.PKIConfig, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return nil, err
	}
	wrapper := map[string]any{configRootPKI: raw}
	return pki.ParseConfig(treeFromMap(wrapper))
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
		parsed, perr := parsePKIFromJSON(s.Data)
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
	tree := treeFromMap(raw)
	return ipsec.ParseIPsecConfig(tree)
}

// treeFromMap recursively converts a map[string]any (from JSON) to a config.Tree.
// Every nested map is stored as a container (for GetContainer). If all children
// of a nested map are themselves maps, each child is also stored as a list entry
// (for GetListOrdered). This dual storage handles the ambiguity between YANG
// containers and keyed lists in untyped JSON.
func treeFromMap(m map[string]any) *config.Tree {
	t := config.NewTree()
	for k, v := range m {
		// A reserved order key is not config: it is how ToPluginMap carries a
		// list's entry order beside the list. Rebuilding it would put a
		// multi-value leaf in this tree that no YANG module declares.
		if strings.HasPrefix(k, configorder.KeyPrefix) {
			continue
		}
		switch val := v.(type) {
		case string:
			t.Set(k, val)
		case float64:
			t.Set(k, strconv.FormatFloat(val, 'f', -1, 64))
		case bool:
			if val {
				t.Set(k, "true")
			} else {
				t.Set(k, "false")
			}
		case map[string]any:
			t.SetContainer(k, treeFromMap(val))
			if allMaps(val) {
				for entryKey, entryVal := range val {
					if em, ok := entryVal.(map[string]any); ok {
						t.AddListEntry(k, entryKey, treeFromMap(em))
					}
				}
			}
		case []any:
			for _, item := range val {
				if s, ok := item.(string); ok {
					t.AppendValue(k, s)
				}
			}
		}
	}
	return t
}

// allMaps returns true if all values in the map are maps (keyed list pattern).
func allMaps(m map[string]any) bool {
	if len(m) == 0 {
		return false
	}
	for _, v := range m {
		if _, ok := v.(map[string]any); !ok {
			return false
		}
	}
	return true
}
