// Design: plan/learned/740-ipsec-7-ikev2-engine.md -- config parsing for IKE engine
package engine

import (
	"encoding/json"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/ipsec"
	"codeberg.org/thomas-mangin/ze/internal/component/pki"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// parseIPsecSections finds the "vpn" config section and parses the IPsec config.
// Also loads the "pki" section into the global PKI store if present.
func parseIPsecSections(sections []sdk.ConfigSection) (*ipsec.IPsecConfig, error) {
	for _, s := range sections {
		if s.Root == "pki" {
			if err := loadPKIFromJSON(s.Data); err != nil {
				return nil, err
			}
		}
	}

	for _, s := range sections {
		if s.Root != "vpn" {
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
	var raw map[string]any
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return err
	}
	wrapper := map[string]any{"pki": raw}
	tree := treeFromMap(wrapper)
	cfg, err := pki.ParseConfig(tree)
	if err != nil {
		return err
	}
	return pki.Load(cfg)
}

// ValidateIPsecSections parses the delivered config sections and runs every
// cross-reference check the IPsec data model defines: group references, PKI
// references (including the RFC 5216 Section 5.3 trust-anchor requirement for
// EAP-TLS peers), and the remote-access pool and credentials.
//
// This is the plugin's OnConfigVerify body. Before it existed the three
// IPsecConfig validators had no non-test caller anywhere in the repo, so a
// config naming a missing ike-group, a missing certificate, or an EAP-TLS peer
// with no CA was accepted at commit and only failed later, at session setup, or
// not at all.
//
// The certificate lookups read the process-wide PKI store, which
// parseIPsecSections has just loaded from the same config delivery, so a
// reference to a certificate defined in the very config being verified resolves.
func ValidateIPsecSections(sections []sdk.ConfigSection) error {
	cfg, err := parseIPsecSections(sections)
	if err != nil {
		return err
	}
	if err := cfg.ValidateGroupRefs(); err != nil {
		return err
	}
	if err := cfg.ValidatePKIRefs(hasPKICA, hasPKICertificate, pkiCertificateCN); err != nil {
		return err
	}
	return cfg.ValidateRemoteAccess()
}

func hasPKICA(name string) bool { return pki.GetCA(name) != nil }

func hasPKICertificate(name string) bool { return pki.GetCertificate(name) != nil }

// pkiCertificateCN returns the subject CN of a stored certificate, or "" when
// the certificate is absent. Returning "" is safe: ValidatePKIRefs skips the
// local-id comparison for an empty CN, and a missing certificate is already
// reported by the hasCert check.
func pkiCertificateCN(name string) string {
	entry := pki.GetCertificate(name)
	if entry == nil || entry.Certificate == nil {
		return ""
	}
	return entry.Certificate.Subject.CommonName
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
