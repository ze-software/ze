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
				{Name: "eth0", Type: "ethernet", MAC: "aa:bb:cc:dd:ee:ff", PermanentMAC: "aa:bb:cc:dd:ee:ff"},
			},
			contains: []string{
				"interface {",
				"ethernet eth0 {",
				"mac {",
				"match aa:bb:cc:dd:ee:ff;",
			},
			excludes: []string{
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
				"mac {",
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
		{
			name: "ethernet reporting a factory address",
			discovered: []DiscoveredInterface{
				{Name: "enp3s0", Type: "ethernet", MAC: "aa:bb:cc:dd:ee:ff", PermanentMAC: "aa:bb:cc:dd:ee:ff"},
				{Name: "enp4s0", Type: "ethernet", MAC: "02:00:00:00:00:01", PermanentMAC: "11:22:33:44:55:66"},
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
					// Every entry carries exactly one selector: mac/match for a
					// NIC that reports a factory address, os-name for the rest.
					if match := matchMACFor(&di, uniquePermanentMACs(tt.discovered)); match != "" {
						matchLine := "match " + match + ";"
						if !strings.Contains(got, matchLine) {
							t.Errorf("missing mac match line %q for %s", matchLine, di.Name)
						}
						continue
					}
					osLine := "os-name " + di.Name + ";"
					if !strings.Contains(got, osLine) {
						t.Errorf("missing os-name line %q for %s", osLine, di.Name)
					}
					if di.Type != "ethernet" && di.MAC != "" {
						macLine := "address " + di.MAC + ";"
						if !strings.Contains(got, macLine) {
							t.Errorf("missing mac address line %q for %s", macLine, di.Name)
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

// VALIDATES: EmitBootstrapConfig produces DHCP-on-ethernet + SSH config
// PREVENTS: bootstrap mode generating invalid or incomplete config

func TestEmitBootstrapConfig(t *testing.T) {
	discovered := []DiscoveredInterface{
		{Name: "eth0", Type: "ethernet", MAC: "aa:bb:cc:dd:ee:ff", PermanentMAC: "aa:bb:cc:dd:ee:ff"},
	}
	got := EmitBootstrapConfig(discovered)

	mustContain := []string{
		"interface {",
		"ethernet eth0 {",
		"mac {",
		"match aa:bb:cc:dd:ee:ff;",
		"unit default {",
		"ipv4 {",
		"dhcp {",
		"enabled true;",
		"environment {",
		"ssh {",
		"enabled true;",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("output missing %q\ngot:\n%s", s, got)
		}
	}
}

func TestEmitBootstrapConfigEmpty(t *testing.T) {
	got := EmitBootstrapConfig(nil)
	if got != "" {
		t.Fatalf("expected empty string for nil input, got %q", got)
	}
	got = EmitBootstrapConfig([]DiscoveredInterface{})
	if got != "" {
		t.Fatalf("expected empty string for empty input, got %q", got)
	}
}

func TestEmitBootstrapConfigEthernetOnly(t *testing.T) {
	discovered := []DiscoveredInterface{
		{Name: "br0", Type: "bridge", MAC: "11:22:33:44:55:66"},
		{Name: "dummy0", Type: "dummy"},
		{Name: "eth0", Type: "ethernet", MAC: "aa:bb:cc:dd:ee:ff"},
		{Name: "lo", Type: "loopback"},
		{Name: "veth0", Type: "veth", MAC: "00:11:22:33:44:55"},
		{Name: "wg0", Type: "wireguard"},
		{Name: "xfrm0", Type: "xfrm"},
	}
	got := EmitBootstrapConfig(discovered)

	if !strings.Contains(got, "ethernet eth0 {") {
		t.Errorf("missing ethernet eth0 block:\n%s", got)
	}
	if !strings.Contains(got, "dhcp {") {
		t.Errorf("missing dhcp block:\n%s", got)
	}

	for _, excluded := range []string{"bridge", "dummy", "loopback", "veth", "wireguard", "xfrm"} {
		if strings.Contains(got, excluded+" ") {
			t.Errorf("bootstrap config should not contain %q type:\n%s", excluded, got)
		}
	}
}

func TestEmitBootstrapConfigMultipleEthernet(t *testing.T) {
	discovered := []DiscoveredInterface{
		{Name: "eth0", Type: "ethernet", MAC: "aa:bb:cc:dd:ee:ff"},
		{Name: "eth1", Type: "ethernet", MAC: "11:22:33:44:55:66"},
		{Name: "enp3s0", Type: "ethernet", MAC: "ff:ee:dd:cc:bb:aa"},
	}
	got := EmitBootstrapConfig(discovered)

	for _, name := range []string{"eth0", "eth1", "enp3s0"} {
		block := "ethernet " + name + " {"
		if !strings.Contains(got, block) {
			t.Errorf("missing block %q:\n%s", block, got)
		}
	}

	dhcpCount := strings.Count(got, "enabled true;")
	// 3 DHCP enabled + 1 SSH enabled = 4 total
	if dhcpCount != 4 {
		t.Errorf("expected 4 'enabled true;' (3 DHCP + 1 SSH), got %d:\n%s", dhcpCount, got)
	}
}

func TestEmitBootstrapConfigStructure(t *testing.T) {
	discovered := []DiscoveredInterface{
		{Name: "eth0", Type: "ethernet", MAC: "aa:bb:cc:dd:ee:ff"},
		{Name: "eth1", Type: "ethernet"},
	}
	got := EmitBootstrapConfig(discovered)

	opens := strings.Count(got, "{")
	closes := strings.Count(got, "}")
	if opens != closes {
		t.Errorf("unbalanced braces: %d opens, %d closes:\n%s", opens, closes, got)
	}

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

func TestEmitBootstrapConfigNoRegression(t *testing.T) {
	discovered := []DiscoveredInterface{
		{Name: "eth0", Type: "ethernet", MAC: "aa:bb:cc:dd:ee:ff"},
		{Name: "br0", Type: "bridge", MAC: "11:22:33:44:55:66"},
		{Name: "lo", Type: "loopback"},
	}
	got := EmitConfig(discovered)

	if strings.Contains(got, "dhcp") {
		t.Errorf("EmitConfig should not contain DHCP blocks:\n%s", got)
	}
	if strings.Contains(got, "environment") {
		t.Errorf("EmitConfig should not contain environment block:\n%s", got)
	}
	if strings.Contains(got, "ssh") {
		t.Errorf("EmitConfig should not contain SSH block:\n%s", got)
	}
}

func TestEmitBootstrapConfigNoEthernetFallback(t *testing.T) {
	discovered := []DiscoveredInterface{
		{Name: "br0", Type: "bridge"},
		{Name: "lo", Type: "loopback"},
		{Name: "wg0", Type: "wireguard"},
	}
	got := EmitBootstrapConfig(discovered)
	if got != "" {
		t.Errorf("expected empty string when no ethernet interfaces, got:\n%s", got)
	}
}

func TestEmitBootstrapConfigUnsafeName(t *testing.T) {
	discovered := []DiscoveredInterface{
		{Name: "bad{name", Type: "ethernet"},
		{Name: "eth0", Type: "ethernet", MAC: "aa:bb:cc:dd:ee:ff"},
	}
	got := EmitBootstrapConfig(discovered)

	if strings.Contains(got, "bad{name") {
		t.Errorf("unsafe name should be skipped:\n%s", got)
	}
	if !strings.Contains(got, "ethernet eth0 {") {
		t.Errorf("safe interface should still appear:\n%s", got)
	}
}

// An unsafe MAC (config-breaking characters) must not be interpolated into the
// config. The interface block is still emitted, just without the mac block,
// and the output stays well-formed.
func TestEmitBootstrapConfigUnsafeMAC(t *testing.T) {
	discovered := []DiscoveredInterface{
		{Name: "eth0", Type: "ethernet", MAC: "aa;bb { injected"},
	}
	got := EmitBootstrapConfig(discovered)

	if strings.Contains(got, "aa;bb") || strings.Contains(got, "injected") {
		t.Errorf("unsafe MAC was interpolated into config:\n%s", got)
	}
	if strings.Contains(got, "mac {") {
		t.Errorf("mac block should be skipped for an unsafe MAC:\n%s", got)
	}
	if !strings.Contains(got, "ethernet eth0 {") {
		t.Errorf("interface block should still be emitted:\n%s", got)
	}
	if opens, closes := strings.Count(got, "{"), strings.Count(got, "}"); opens != closes {
		t.Errorf("unbalanced braces (%d open, %d close):\n%s", opens, closes, got)
	}
}

func TestEmitConfigXFRMFull(t *testing.T) {
	dis := []DiscoveredInterface{{
		Name: "xfrm0",
		Type: zeTypeXFRM,
		XFRM: &XFRMInfo{
			IfID:      42,
			ParentDev: "eth0",
			Addresses: []string{"10.0.0.1/30", "fd00::1/64"},
		},
	}}
	out := EmitConfig(dis)
	for _, want := range []string{
		"xfrm xfrm0 {",
		"if-id 42;",
		"dev eth0;",
		"ipv4 {\n                address 10.0.0.1/30;",
		"ipv6 {\n                address fd00::1/64;",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("EmitConfig missing %q in:\n%s", want, out)
		}
	}
}

func TestEmitConfigXFRMSkeleton(t *testing.T) {
	dis := []DiscoveredInterface{{
		Name: "xfrm1",
		Type: zeTypeXFRM,
	}}
	out := EmitConfig(dis)
	if !strings.Contains(out, "xfrm xfrm1 {") {
		t.Errorf("EmitConfig missing xfrm block in:\n%s", out)
	}
	if strings.Contains(out, "if-id") {
		t.Errorf("EmitConfig skeleton should not have if-id in:\n%s", out)
	}
}

func TestEmitConfigXFRMNoDev(t *testing.T) {
	dis := []DiscoveredInterface{{
		Name: "xfrm0",
		Type: zeTypeXFRM,
		XFRM: &XFRMInfo{
			IfID:      99,
			Addresses: []string{"192.168.1.1/24"},
		},
	}}
	out := EmitConfig(dis)
	if !strings.Contains(out, "if-id 99;") {
		t.Errorf("EmitConfig missing if-id in:\n%s", out)
	}
	if strings.Contains(out, "dev ") {
		t.Errorf("EmitConfig should not have dev in:\n%s", out)
	}
}

func TestEmitSetConfigXFRM(t *testing.T) {
	dis := []DiscoveredInterface{{
		Name: "xfrm0",
		Type: zeTypeXFRM,
		XFRM: &XFRMInfo{
			IfID:      42,
			ParentDev: "eth0",
			Addresses: []string{"10.0.0.1/30"},
		},
	}}
	out := emitSetConfig(dis, false)
	for _, want := range []string{
		"set interface xfrm xfrm0 if-id 42",
		"set interface xfrm xfrm0 dev eth0",
		"set interface xfrm xfrm0 unit default ipv4 address 10.0.0.1/30",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("emitSetConfig missing %q in:\n%s", want, out)
		}
	}
}

func TestEmitSetConfigWithDHCPAllEthernet(t *testing.T) {
	dis := []DiscoveredInterface{
		{Name: "br0", Type: zeTypeBridge, MAC: "11:22:33:44:55:66"},
		{Name: "eth0", Type: zeTypeEthernet, MAC: "aa:bb:cc:dd:ee:ff"},
		{Name: "eth1", Type: zeTypeEthernet, MAC: "bb:cc:dd:ee:ff:00"},
	}
	out := EmitSetConfigWithDHCP(dis)

	for _, name := range []string{"eth0", "eth1"} {
		want := "set interface ethernet " + name + " unit default ipv4 dhcp enabled true"
		if !strings.Contains(out, want) {
			t.Errorf("missing DHCP line for %s:\n%s", name, out)
		}
	}

	if strings.Contains(out, "br0 unit default ipv4 dhcp") {
		t.Errorf("DHCP should not be on bridge:\n%s", out)
	}
}

func TestEmitSetConfigWithDHCPEmpty(t *testing.T) {
	out := EmitSetConfigWithDHCP(nil)
	if out != "" {
		t.Errorf("expected empty for nil input, got: %q", out)
	}
}

func TestEmitSetConfigWithDHCPNoEthernet(t *testing.T) {
	dis := []DiscoveredInterface{
		{Name: "br0", Type: zeTypeBridge},
		{Name: "dummy0", Type: zeTypeDummy},
	}
	out := EmitSetConfigWithDHCP(dis)

	if strings.Contains(out, "dhcp") {
		t.Errorf("no ethernet means no DHCP line:\n%s", out)
	}
}

func TestEmitSetConfigWithoutDHCPFlag(t *testing.T) {
	dis := []DiscoveredInterface{
		{Name: "eth0", Type: zeTypeEthernet, MAC: "aa:bb:cc:dd:ee:ff"},
	}
	out := emitSetConfig(dis, false)

	if strings.Contains(out, "dhcp") {
		t.Errorf("emitSetConfig (without DHCP) should not emit DHCP:\n%s", out)
	}
}

// TestEmitSetConfigCreatedKindMAC verifies the set-form MAC emission uses the
// two-token "mac address" path (container mac { leaf address }), not the old
// single "mac-address" leaf. It runs on a bridge because that is where the
// address override survives: a discovered ethernet binds by its factory address
// instead (TestEmitSetConfigEthernetMatchesPermanentMAC).
//
// PREVENTS: set-form emit drifting back to the renamed leaf and producing
// config that no longer parses against the schema.
func TestEmitSetConfigCreatedKindMAC(t *testing.T) {
	dis := []DiscoveredInterface{{
		Name: "br0",
		Type: zeTypeBridge,
		MAC:  "aa:bb:cc:dd:ee:ff",
	}}
	out := emitSetConfig(dis, false)
	if want := "set interface bridge br0 mac address aa:bb:cc:dd:ee:ff"; !strings.Contains(out, want) {
		t.Errorf("emitSetConfig missing %q in:\n%s", want, out)
	}
	if strings.Contains(out, "mac-address") {
		t.Errorf("emitSetConfig must not emit the old mac-address leaf:\n%s", out)
	}
}

// TestEmitConfigEthernetMatchesPermanentMAC drives EmitConfig over a discovered
// NIC that reports a factory address. The goal is the SELECTOR: the entry must
// bind by mac/match, so the config follows the NIC, and must carry no mac
// address override, which would impose one NIC's address on whatever device
// held its kernel name at the next boot.
func TestEmitConfigEthernetMatchesPermanentMAC(t *testing.T) {
	got := EmitConfig([]DiscoveredInterface{
		{Name: "eth0", Type: "ethernet", MAC: "aa:bb:cc:dd:ee:ff", PermanentMAC: "aa:bb:cc:dd:ee:ff"},
	})

	for _, want := range []string{"ethernet eth0 {", "mac {", "match aa:bb:cc:dd:ee:ff;"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"address aa:bb:cc:dd:ee:ff;", "os-name eth0;"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("emitted %q; an ethernet binds by its factory address alone:\n%s", unwanted, got)
		}
	}
}

// TestEmitConfigEthernetWithoutPermanentMAC proves the fallback. A device with
// no factory address (a virtual NIC classified as ethernet) binds by name, and
// still gets no address override: the override is what makes a stale name
// dangerous.
func TestEmitConfigEthernetWithoutPermanentMAC(t *testing.T) {
	got := EmitConfig([]DiscoveredInterface{
		{Name: "eth0", Type: "ethernet", MAC: "aa:bb:cc:dd:ee:ff"},
	})

	if !strings.Contains(got, "os-name eth0;") {
		t.Errorf("missing os-name selector:\n%s", got)
	}
	if strings.Contains(got, "mac {") {
		t.Errorf("emitted a mac block for an ethernet with no factory address:\n%s", got)
	}
}

// TestEmitConfigCreatedKindKeepsMACAddress proves the address override stays
// where it is an instruction rather than a record. Ze creates a dummy, a veth
// and a bridge, so writing the address back pins the kernel's random choice
// across a recreate. Only the physical kind loses it.
func TestEmitConfigCreatedKindKeepsMACAddress(t *testing.T) {
	got := EmitConfig([]DiscoveredInterface{
		{Name: "br0", Type: "bridge", MAC: "aa:bb:cc:dd:ee:01"},
		{Name: "dum0", Type: "dummy", MAC: "aa:bb:cc:dd:ee:02"},
	})

	for _, want := range []string{"address aa:bb:cc:dd:ee:01;", "os-name br0;", "address aa:bb:cc:dd:ee:02;", "os-name dum0;"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "match ") {
		t.Errorf("emitted a mac/match selector for a Ze-created kind:\n%s", got)
	}
}

// TestEmitSetConfigEthernetMatchesPermanentMAC is the set-command counterpart of
// TestEmitConfigEthernetMatchesPermanentMAC. The first-boot bootstrap path emits
// this form, so it carries the same selector.
func TestEmitSetConfigEthernetMatchesPermanentMAC(t *testing.T) {
	got := EmitSetConfigWithDHCP([]DiscoveredInterface{
		{Name: "eth0", Type: "ethernet", MAC: "aa:bb:cc:dd:ee:ff", PermanentMAC: "aa:bb:cc:dd:ee:ff"},
	})

	if want := "set interface ethernet eth0 mac match aa:bb:cc:dd:ee:ff"; !strings.Contains(got, want) {
		t.Errorf("missing %q:\n%s", want, got)
	}
	if strings.Contains(got, "mac address") {
		t.Errorf("emitted a mac address override for a discovered NIC:\n%s", got)
	}
}

// TestEmitBootstrapConfigEthernetMatchesPermanentMAC covers the third emitter.
// It runs on a first boot with no template, so a wrong binding here reaches a
// box nobody is watching.
func TestEmitBootstrapConfigEthernetMatchesPermanentMAC(t *testing.T) {
	got := EmitBootstrapConfig([]DiscoveredInterface{
		{Name: "eth0", Type: "ethernet", MAC: "aa:bb:cc:dd:ee:ff", PermanentMAC: "aa:bb:cc:dd:ee:ff"},
	})

	if !strings.Contains(got, "match aa:bb:cc:dd:ee:ff;") {
		t.Errorf("missing mac/match selector:\n%s", got)
	}
	if strings.Contains(got, "address aa:bb:cc:dd:ee:ff;") {
		t.Errorf("emitted a mac address override:\n%s", got)
	}
}

// TestEmitConfigSharedPermanentMACFallsBack covers the case that would cost an
// appliance its whole config. A factory address two NICs report is not an
// identity: validateSelectors (config_apply.go) REFUSES a commit whose selector
// names more than one device, so emitting it on a first boot would leave the box
// with no config at all. Both entries fall back to the os-name selector.
func TestEmitConfigSharedPermanentMACFallsBack(t *testing.T) {
	const shared = "aa:bb:cc:dd:ee:ff"
	got := EmitConfig([]DiscoveredInterface{
		{Name: "eth0", Type: "ethernet", MAC: shared, PermanentMAC: shared},
		{Name: "eth1", Type: "ethernet", MAC: shared, PermanentMAC: shared},
	})

	if strings.Contains(got, "match ") {
		t.Errorf("emitted a selector two devices answer to:\n%s", got)
	}
	for _, want := range []string{"os-name eth0;", "os-name eth1;"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing fallback selector %q:\n%s", want, got)
		}
	}
}

// TestEmitConfigZeroPermanentMACFallsBack proves an all-zero factory address is
// read as "the driver reported nothing", not as an address. Binding to it would
// name no device, and the entry would stay deferred and unconfigured.
func TestEmitConfigZeroPermanentMACFallsBack(t *testing.T) {
	got := EmitConfig([]DiscoveredInterface{
		{Name: "eth0", Type: "ethernet", MAC: "aa:bb:cc:dd:ee:ff", PermanentMAC: "00:00:00:00:00:00"},
	})

	if strings.Contains(got, "match ") {
		t.Errorf("emitted a selector for an all-zero factory address:\n%s", got)
	}
	if !strings.Contains(got, "os-name eth0;") {
		t.Errorf("missing fallback selector:\n%s", got)
	}
}
