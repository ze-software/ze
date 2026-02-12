package bgp_llnh

import (
	"bytes"
	"strings"
	"testing"
)

// TestExtractLLNHCapabilities verifies config parsing for link-local-nexthop capability.
//
// VALIDATES: Peers with link-local-nexthop enable produce capability code 77.
// PREVENTS: Capability not declared when config has link-local-nexthop enabled.
func TestExtractLLNHCapabilities(t *testing.T) {
	jsonStr := `{
		"bgp": {
			"peer": {
				"10.0.0.1": {
					"capability": {
						"link-local-nexthop": "enable"
					}
				}
			}
		}
	}`

	caps := extractLLNHCapabilities(jsonStr)
	if len(caps) != 1 {
		t.Fatalf("expected 1 capability, got %d", len(caps))
	}

	cap := caps[0]
	if cap.Code != llnhCapCode {
		t.Errorf("expected code %d, got %d", llnhCapCode, cap.Code)
	}
	if cap.Payload != "" {
		t.Errorf("expected empty payload, got %q", cap.Payload)
	}
	if len(cap.Peers) != 1 || cap.Peers[0] != "10.0.0.1" {
		t.Errorf("expected peers [10.0.0.1], got %v", cap.Peers)
	}
}

// TestExtractLLNHCapabilitiesWrapped verifies parsing with bgp wrapper.
//
// VALIDATES: Both wrapped {"bgp": {...}} and bare {...} formats work.
// PREVENTS: Config delivery format mismatch between engine and plugin.
func TestExtractLLNHCapabilitiesWrapped(t *testing.T) {
	// Bare format (no "bgp" wrapper)
	jsonStr := `{
		"peer": {
			"10.0.0.1": {
				"capability": {
					"link-local-nexthop": "enable"
				}
			}
		}
	}`

	caps := extractLLNHCapabilities(jsonStr)
	if len(caps) != 1 {
		t.Fatalf("expected 1 capability from bare format, got %d", len(caps))
	}
	if caps[0].Peers[0] != "10.0.0.1" {
		t.Errorf("expected peer 10.0.0.1, got %v", caps[0].Peers)
	}
}

// TestExtractLLNHCapabilitiesNoCap verifies peers without capability produce nothing.
//
// VALIDATES: Peers without link-local-nexthop in config don't get capability 77.
// PREVENTS: Spurious capability declaration for unconfigured peers.
func TestExtractLLNHCapabilitiesNoCap(t *testing.T) {
	jsonStr := `{
		"bgp": {
			"peer": {
				"10.0.0.1": {
					"capability": {
						"route-refresh": "enable"
					}
				}
			}
		}
	}`

	caps := extractLLNHCapabilities(jsonStr)
	if len(caps) != 0 {
		t.Errorf("expected 0 capabilities, got %d", len(caps))
	}
}

// TestExtractLLNHCapabilitiesMultiplePeers verifies per-peer capability isolation.
//
// VALIDATES: Only peers with link-local-nexthop enabled get capability 77.
// PREVENTS: Capability leaking to peers that didn't configure it.
func TestExtractLLNHCapabilitiesMultiplePeers(t *testing.T) {
	jsonStr := `{
		"bgp": {
			"peer": {
				"10.0.0.1": {
					"capability": {
						"link-local-nexthop": "enable"
					}
				},
				"10.0.0.2": {
					"capability": {
						"route-refresh": "enable"
					}
				},
				"10.0.0.3": {
					"capability": {
						"link-local-nexthop": "enable"
					}
				}
			}
		}
	}`

	caps := extractLLNHCapabilities(jsonStr)
	if len(caps) != 2 {
		t.Fatalf("expected 2 capabilities, got %d", len(caps))
	}

	// Collect peer addresses
	peers := make(map[string]bool)
	for _, c := range caps {
		if c.Code != llnhCapCode {
			t.Errorf("expected code %d, got %d", llnhCapCode, c.Code)
		}
		for _, p := range c.Peers {
			peers[p] = true
		}
	}
	if !peers["10.0.0.1"] || !peers["10.0.0.3"] {
		t.Errorf("expected peers 10.0.0.1 and 10.0.0.3, got %v", peers)
	}
	if peers["10.0.0.2"] {
		t.Errorf("peer 10.0.0.2 should not have capability")
	}
}

// TestRunDecodeModeJSON verifies stdin decode protocol for capability 77.
//
// VALIDATES: "decode capability 77 <hex>" produces correct JSON response.
// PREVENTS: Decode protocol returning "unknown" for supported capability.
func TestRunDecodeModeJSON(t *testing.T) {
	// Capability 77 has empty payload, so hex is empty string
	input := "decode capability 77 \n"
	var stdout bytes.Buffer

	code := RunLLNHDecodeMode(strings.NewReader(input), &stdout)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	out := stdout.String()
	if !strings.Contains(out, "decoded json") {
		t.Errorf("expected 'decoded json' response, got: %s", out)
	}
	if !strings.Contains(out, "link-local-nexthop") {
		t.Errorf("expected 'link-local-nexthop' in JSON, got: %s", out)
	}
}

// TestRunDecodeModeText verifies text-format decode for capability 77.
//
// VALIDATES: "decode text capability 77 <hex>" produces human-readable output.
// PREVENTS: Text mode returning JSON instead of readable format.
func TestRunDecodeModeText(t *testing.T) {
	input := "decode text capability 77 \n"
	var stdout bytes.Buffer

	code := RunLLNHDecodeMode(strings.NewReader(input), &stdout)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	out := stdout.String()
	if !strings.Contains(out, "decoded text") {
		t.Errorf("expected 'decoded text' response, got: %s", out)
	}
	if !strings.Contains(out, "link-local-nexthop") {
		t.Errorf("expected 'link-local-nexthop' in text, got: %s", out)
	}
}

// TestRunDecodeModeUnknownCode verifies unknown capability codes are rejected.
//
// VALIDATES: Non-77 codes return "decoded unknown".
// PREVENTS: Plugin claiming to decode capabilities it doesn't handle.
func TestRunDecodeModeUnknownCode(t *testing.T) {
	input := "decode capability 73 AABB\n"
	var stdout bytes.Buffer

	RunLLNHDecodeMode(strings.NewReader(input), &stdout)

	out := stdout.String()
	if !strings.Contains(out, "decoded unknown") {
		t.Errorf("expected 'decoded unknown' for code 73, got: %s", out)
	}
}

// TestRunCLIDecode verifies CLI hex decode for capability 77.
//
// VALIDATES: Direct hex input produces correct JSON output.
// PREVENTS: CLI decode path broken while protocol decode works.
func TestRunCLIDecode(t *testing.T) {
	var stdout, stderr bytes.Buffer

	// Empty hex (capability 77 has no payload)
	code := RunLLNHCLIDecode("", false, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "link-local-nexthop") {
		t.Errorf("expected 'link-local-nexthop' in output, got: %s", out)
	}
}

// TestRunCLIDecodeText verifies CLI text output mode.
//
// VALIDATES: --text flag produces human-readable format.
// PREVENTS: Text output containing JSON.
func TestRunCLIDecodeText(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := RunLLNHCLIDecode("", true, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	out := stdout.String()
	if !strings.Contains(out, "link-local-nexthop") {
		t.Errorf("expected 'link-local-nexthop' in text output, got: %s", out)
	}
}

// TestLLNHPluginYANG verifies the YANG schema is embedded correctly.
//
// VALIDATES: GetYANG returns non-empty schema with correct module name.
// PREVENTS: Missing or corrupt YANG embed.
func TestLLNHPluginYANG(t *testing.T) {
	yang := GetLLNHYANG()
	if yang == "" {
		t.Fatal("GetYANG returned empty string")
	}
	if !strings.Contains(yang, "ze-link-local-nexthop") {
		t.Error("YANG schema missing module name")
	}
	if !strings.Contains(yang, "capability") {
		t.Error("YANG schema missing capability augmentation")
	}
}

// TestDecodableCapabilities verifies the plugin reports capability code 77.
//
// VALIDATES: DecodableCapabilities returns exactly [77].
// PREVENTS: Plugin registered for wrong capability codes.
func TestDecodableCapabilities(t *testing.T) {
	caps := LLNHDecodableCapabilities()
	if len(caps) != 1 || caps[0] != 77 {
		t.Errorf("expected [77], got %v", caps)
	}
}
