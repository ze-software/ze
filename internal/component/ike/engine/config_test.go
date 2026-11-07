package engine

import "testing"

func TestParseIPsecFromJSON(t *testing.T) {
	jsonData := `{
		"vpn": {
			"ipsec": {
				"interface": "eth0",
				"ike-group": {
					"IKE-1": {
						"key-exchange": "ikev2",
						"lifetime": "28800",
						"proposal": {
							"1": {
								"encryption": "aes256",
								"hash": "sha256",
								"dh-group": "14"
							}
						}
					}
				},
				"esp-group": {
					"ESP-1": {
						"lifetime": "3600",
						"proposal": {
							"1": {
								"encryption": "aes256",
								"hash": "sha256"
							}
						}
					}
				},
				"site-to-site": {
					"peer": {
						"vpn-peer": {
							"ike-group": "IKE-1",
							"esp-group": "ESP-1",
							"connection-type": "initiate",
							"local-address": "10.0.0.1",
							"remote-address": "10.0.0.2",
							"authentication": {
								"mode": "pre-shared-secret",
								"pre-shared-secret": "secret123"
							}
						}
					}
				}
			}
		}
	}`

	cfg, err := parseIPsecFromJSON(jsonData)
	if err != nil {
		t.Fatalf("parseIPsecFromJSON: %v", err)
	}

	if cfg.Interface != "eth0" {
		t.Fatalf("expected interface eth0, got %q", cfg.Interface)
	}
	if len(cfg.IKEGroups) != 1 {
		t.Fatalf("expected 1 ike-group, got %d", len(cfg.IKEGroups))
	}
	ikeGroup, ok := cfg.IKEGroups["IKE-1"]
	if !ok {
		t.Fatal("expected ike-group IKE-1")
	}
	if len(ikeGroup.Proposals) != 1 {
		t.Fatalf("expected 1 ike proposal, got %d", len(ikeGroup.Proposals))
	}
	if len(cfg.ESPGroups) != 1 {
		t.Fatalf("expected 1 esp-group, got %d", len(cfg.ESPGroups))
	}
	if len(cfg.Peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(cfg.Peers))
	}
	peer, ok := cfg.Peers["vpn-peer"]
	if !ok {
		t.Fatal("expected peer vpn-peer")
	}
	if peer.RemoteAddress != "10.0.0.2" {
		t.Fatalf("expected remote 10.0.0.2, got %q", peer.RemoteAddress)
	}
	if peer.Auth.PSK != "secret123" {
		t.Fatalf("expected PSK secret123, got %q", peer.Auth.PSK)
	}
}

func TestParseIPsecSectionsEmpty(t *testing.T) {
	cfg, err := parseIPsecSections(nil)
	if err != nil {
		t.Fatalf("parseIPsecSections nil: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if len(cfg.Peers) != 0 {
		t.Fatalf("expected 0 peers, got %d", len(cfg.Peers))
	}
}

// TestDualFamilySelectorListValidates verifies the home CPE's management peer, one
// IPv4 0.0.0.0/0 to 0.0.0.0/0 entry and one IPv6 ::/0 to ::/0 entry on a VTI-bound
// tunnel, through validateIPsecSections, the plugin's OnConfigVerify body.
//
// VALIDATES: AC-1, validation half: the list commits. validateSelectorCombinations
// passes it, because each local entry already meets its own family's remote entry and
// the cross-family combinations match no packet (RFC 7296 Section 3.13).
// PREVENTS: the dual-stack form the guide recommends being refused at commit.
func TestDualFamilySelectorListValidates(t *testing.T) {
	if err := validateIPsecSections(vpnSection(dualFamilyCPEConfig)); err != nil {
		t.Fatalf("the dual-family VTI peer failed config verify: %v", err)
	}
	_, cfg := dualFamilyPeer(t)
	if err := validateSelectorCombinations(cfg); err != nil {
		t.Errorf("validateSelectorCombinations refused one IPv4 and one IPv6 entry: %v", err)
	}
}
