package iface

import (
	"strings"
	"testing"
)

// VALIDATES: EmitConfig produces correct Ze config syntax
// PREVENTS: malformed config from interface discovery breaking config parser

func TestEmitConfig(t *testing.T) {
	tests := []struct {
		name       string
		discovered []DiscoveredInterface
		wantEmpty  bool
		contains   []string
		excludes   []string
	}{
		{
			name:       "empty input",
			discovered: nil,
			wantEmpty:  true,
		},
		{
			name: "single ethernet with MAC",
			discovered: []DiscoveredInterface{
				{Name: "eth0", Type: "ethernet", MAC: "aa:bb:cc:dd:ee:ff"},
			},
			contains: []string{
				"interface {",
				"ethernet eth0 {",
				"mac-address aa:bb:cc:dd:ee:ff;",
				"os-name eth0;",
			},
		},
		{
			name: "single ethernet without MAC",
			discovered: []DiscoveredInterface{
				{Name: "eth0", Type: "ethernet"},
			},
			contains: []string{
				"ethernet eth0 {",
				"os-name eth0;",
			},
			excludes: []string{
				"mac-address",
			},
		},
		{
			name: "loopback only",
			discovered: []DiscoveredInterface{
				{Name: "lo", Type: "loopback"},
			},
			contains: []string{
				"interface {",
				"loopback {",
			},
			excludes: []string{
				"ethernet",
				"os-name",
			},
		},
		{
			name: "mixed types",
			discovered: []DiscoveredInterface{
				{Name: "br0", Type: "bridge", MAC: "11:22:33:44:55:66"},
				{Name: "dummy0", Type: "dummy"},
				{Name: "eth0", Type: "ethernet", MAC: "aa:bb:cc:dd:ee:ff"},
				{Name: "lo", Type: "loopback"},
			},
			contains: []string{
				"bridge br0 {",
				"dummy dummy0 {",
				"ethernet eth0 {",
				"loopback {",
			},
		},
		{
			name: "invalid name with brace is skipped",
			discovered: []DiscoveredInterface{
				{Name: "bad{name", Type: "ethernet", MAC: "aa:bb:cc:dd:ee:ff"},
				{Name: "eth0", Type: "ethernet", MAC: "11:22:33:44:55:66"},
			},
			contains: []string{
				"ethernet eth0 {",
			},
			excludes: []string{
				"bad{name",
			},
		},
		{
			name: "invalid name with semicolon is skipped",
			discovered: []DiscoveredInterface{
				{Name: "bad;name", Type: "ethernet"},
			},
			contains: []string{
				"interface {",
			},
			excludes: []string{
				"bad;name",
				"ethernet",
			},
		},
		{
			name: "invalid name with newline is skipped",
			discovered: []DiscoveredInterface{
				{Name: "bad\nname", Type: "ethernet"},
			},
			excludes: []string{
				"ethernet",
			},
		},
		{
			name: "invalid name with space is skipped",
			discovered: []DiscoveredInterface{
				{Name: "bad name", Type: "ethernet"},
			},
			excludes: []string{
				"bad name",
			},
		},
		{
			name: "os-name populated in config",
			discovered: []DiscoveredInterface{
				{Name: "enp3s0", Type: "ethernet", MAC: "aa:bb:cc:dd:ee:ff"},
			},
			contains: []string{
				"os-name enp3s0;",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EmitConfig(tt.discovered)

			if tt.wantEmpty {
				if got != "" {
					t.Fatalf("expected empty string, got %q", got)
				}
				return
			}

			for _, s := range tt.contains {
				if !strings.Contains(got, s) {
					t.Errorf("output missing %q\ngot:\n%s", s, got)
				}
			}

			for _, s := range tt.excludes {
				if strings.Contains(got, s) {
					t.Errorf("output should not contain %q\ngot:\n%s", s, got)
				}
			}
		})
	}
}

// TestEmitConfigWireguardSkeleton verifies that a wireguard entry with no
// backend-populated spec still emits a valid config block containing the
// interface name and os-name leaf. Operators fill in the rest by hand.
//
// VALIDATES: EmitConfig emits a skeleton wireguard block even when
// GetWireguardDevice fails.
// PREVENTS: discovery silently dropping wireguard interfaces because of
// a backend error.
func TestEmitConfigWireguardSkeleton(t *testing.T) {
	discovered := []DiscoveredInterface{
		{Name: "wg0", Type: "wireguard"},
	}
	out := EmitConfig(discovered)
	if !strings.Contains(out, "wireguard wg0 {") {
		t.Errorf("missing wireguard block: %q", out)
	}
	if !strings.Contains(out, "os-name wg0;") {
		t.Errorf("missing os-name leaf: %q", out)
	}
	for _, leaf := range []string{"private-key", "listen-port", "fwmark", "peer "} {
		if strings.Contains(out, leaf) {
			t.Errorf("skeleton should omit %q leaf: %q", leaf, out)
		}
	}
}

// TestEmitConfigWireguardFullSpec verifies that when the backend returned
// a full WireguardSpec, every field is emitted and the sensitive leaves
// (private-key, preshared-key) are $9$-encoded.
//
// VALIDATES: EmitConfig captures a running wireguard netdev into config with
// correctly encoded secrets. Public-keys stay plaintext; private and
// preshared keys pass through secret.Encode.
// PREVENTS: plaintext private keys leaking into ze.conf at init time.
func TestEmitConfigWireguardFullSpec(t *testing.T) {
	var priv, pub, psk WireguardKey
	for i := range priv {
		priv[i] = 0x11
		pub[i] = 0x22
		psk[i] = 0x33
	}

	spec := &WireguardSpec{
		Name:          "wg0",
		PrivateKey:    priv,
		ListenPort:    51820,
		ListenPortSet: true,
		FirewallMark:  0x1234,
		Peers: []WireguardPeerSpec{{
			Name:                "site2",
			PublicKey:           pub,
			PresharedKey:        psk,
			HasPresharedKey:     true,
			EndpointIP:          "198.51.100.2",
			EndpointPort:        51820,
			AllowedIPs:          []string{"10.0.0.2/32", "192.168.10.0/24"},
			PersistentKeepalive: 25,
		}},
	}

	discovered := []DiscoveredInterface{
		{Name: "wg0", Type: "wireguard", Wireguard: spec},
	}
	out := EmitConfig(discovered)

	mustContain := []string{
		"wireguard wg0 {",
		"listen-port 51820;",
		"fwmark 4660;",
		`private-key "$9$`,
		"peer peer0 {",
		`public-key "`,
		`preshared-key "$9$`,
		"endpoint {",
		"ip 198.51.100.2;",
		"port 51820;",
		"allowed-ips [ 10.0.0.2/32 192.168.10.0/24 ];",
		"persistent-keepalive 25;",
	}
	for _, s := range mustContain {
		if !strings.Contains(out, s) {
			t.Errorf("output missing %q:\n%s", s, out)
		}
	}

	plaintextPriv := priv.String()
	if strings.Contains(out, plaintextPriv) {
		t.Errorf("plaintext private-key leaked into config:\n%s", out)
	}
	plaintextPSK := psk.String()
	if strings.Contains(out, plaintextPSK) {
		t.Errorf("plaintext preshared-key leaked into config:\n%s", out)
	}
}

// VALIDATES: EmitConfig output is structurally valid config syntax
// PREVENTS: generated config breaking the config parser due to unbalanced braces,
// missing terminators, or malformed block structure

func TestEmitConfigStructure(t *testing.T) {
	tests := []struct {
		name       string
		discovered []DiscoveredInterface
	}{
		{
			name: "single ethernet",
			discovered: []DiscoveredInterface{
				{Name: "eth0", Type: "ethernet", MAC: "aa:bb:cc:dd:ee:ff"},
			},
		},
		{
			name: "ethernet without MAC",
			discovered: []DiscoveredInterface{
				{Name: "eth0", Type: "ethernet"},
			},
		},
		{
			name: "loopback only",
			discovered: []DiscoveredInterface{
				{Name: "lo", Type: "loopback"},
			},
		},
		{
			name: "all types",
			discovered: []DiscoveredInterface{
				{Name: "br0", Type: "bridge", MAC: "11:22:33:44:55:66"},
				{Name: "dummy0", Type: "dummy"},
				{Name: "eth0", Type: "ethernet", MAC: "aa:bb:cc:dd:ee:ff"},
				{Name: "eth1", Type: "ethernet", MAC: "ff:ee:dd:cc:bb:aa"},
				{Name: "lo", Type: "loopback"},
				{Name: "veth0", Type: "veth", MAC: "00:11:22:33:44:55"},
			},
		},
		{
			name: "multiple ethernet",
			discovered: []DiscoveredInterface{
				{Name: "enp3s0", Type: "ethernet", MAC: "aa:bb:cc:dd:ee:ff"},
				{Name: "enp4s0", Type: "ethernet", MAC: "11:22:33:44:55:66"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EmitConfig(tt.discovered)
			if got == "" {
				t.Fatal("expected non-empty output")
			}

			if !strings.HasPrefix(got, "interface {\n") {
				t.Errorf("output must start with 'interface {\\n', got prefix: %q",
					got[:min(len(got), 30)])
			}
			if !strings.HasSuffix(got, "}\n") {
				t.Errorf("output must end with '}\\n', got suffix: %q",
					got[max(0, len(got)-20):])
			}

			opens := strings.Count(got, "{")
			closes := strings.Count(got, "}")
			if opens != closes {
				t.Errorf("unbalanced braces: %d opens, %d closes", opens, closes)
			}

			for _, di := range tt.discovered {
				if !safeEmitName(di.Name) {
					continue
				}
				switch di.Type {
				case "ethernet", "bridge", "veth", "dummy":
					blockHeader := di.Type + " " + di.Name + " {"
					if !strings.Contains(got, blockHeader) {
						t.Errorf("missing block header %q", blockHeader)
					}
					osLine := "os-name " + di.Name + ";"
					if !strings.Contains(got, osLine) {
						t.Errorf("missing os-name line %q for %s", osLine, di.Name)
					}
					if di.MAC != "" {
						macLine := "mac-address " + di.MAC + ";"
						if !strings.Contains(got, macLine) {
							t.Errorf("missing mac-address line %q for %s", macLine, di.Name)
						}
					}
				case "loopback":
					if strings.Contains(got, "loopback "+di.Name) {
						t.Errorf("loopback should not have a name key")
					}
				}
			}

			hasLoopback := false
			for _, di := range tt.discovered {
				if di.Type == "loopback" {
					hasLoopback = true
					break
				}
			}
			if hasLoopback {
				if !strings.Contains(got, "loopback {") {
					t.Error("expected 'loopback {' block")
				}
				if _, after, found := strings.Cut(got, "loopback {"); found {
					if body, _, ok := strings.Cut(after, "}"); ok {
						loBody := strings.TrimSpace(body)
						if loBody != "" {
							t.Errorf("loopback block should be empty, got: %q", loBody)
						}
					}
				}
			}

			for i, line := range strings.Split(got, "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasSuffix(trimmed, ";") {
					if !strings.HasPrefix(line, "    ") {
						t.Errorf("line %d: semicolon-terminated line not indented: %q", i+1, line)
					}
				}
			}
		})
	}
}

// VALIDATES: EmitConfig round-trip: known inputs produce parseable output
// PREVENTS: config generation creating syntax that the config tokenizer rejects

func TestEmitConfigTokenizable(t *testing.T) {
	discovered := []DiscoveredInterface{
		{Name: "eth0", Type: "ethernet", MAC: "aa:bb:cc:dd:ee:ff"},
		{Name: "br0", Type: "bridge", MAC: "11:22:33:44:55:66"},
		{Name: "lo", Type: "loopback"},
	}

	got := EmitConfig(discovered)

	depth := 0
	for i, line := range strings.Split(got, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		switch {
		case strings.HasSuffix(trimmed, "{"):
			depth++
		case trimmed == "}":
			depth--
			if depth < 0 {
				t.Fatalf("line %d: brace depth went negative", i+1)
			}
		case !strings.HasSuffix(trimmed, ";"):
			t.Errorf("line %d: expected ';' or '{' or '}', got: %q", i+1, trimmed)
		}
	}
	if depth != 0 {
		t.Errorf("final brace depth is %d, expected 0", depth)
	}
}

// VALIDATES: safeEmitName rejects config-breaking characters
// PREVENTS: interface names with special characters breaking config syntax

func TestSafeEmitName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"valid simple", "eth0", true},
		{"valid with dash", "enp3s0", true},
		{"valid with dot", "veth0.1", true},
		{"empty", "", false},
		{"contains brace open", "bad{", false},
		{"contains brace close", "bad}", false},
		{"contains semicolon", "bad;", false},
		{"contains newline", "bad\n", false},
		{"contains carriage return", "bad\r", false},
		{"contains tab", "bad\t", false},
		{"contains space", "bad name", false},
		{"contains null", "bad\x00", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safeEmitName(tt.in)
			if got != tt.want {
				t.Errorf("safeEmitName(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
