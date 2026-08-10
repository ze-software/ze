package ipsec

import "testing"

func TestEncryptionAlgoString(t *testing.T) {
	tests := []struct {
		algo EncryptionAlgo
		want string
	}{
		{EncryptionAES128, "aes128"},
		{EncryptionAES256, "aes256"},
		{EncryptionAES128GCM, "aes128gcm"},
		{EncryptionAES256GCM, "aes256gcm"},
		{EncryptionChaCha20Poly, "chacha20poly1305"},
		{Encryption3DES, "3des"},
		{EncryptionUnknown, unknownEnum},
	}
	for _, tt := range tests {
		if got := tt.algo.String(); got != tt.want {
			t.Errorf("EncryptionAlgo(%d).String() = %q, want %q", tt.algo, got, tt.want)
		}
		if tt.algo == EncryptionUnknown {
			continue
		}
		parsed, ok := ParseEncryptionAlgo(tt.want)
		if !ok {
			t.Errorf("ParseEncryptionAlgo(%q) returned !ok", tt.want)
		}
		if parsed != tt.algo {
			t.Errorf("ParseEncryptionAlgo(%q) = %d, want %d", tt.want, parsed, tt.algo)
		}
	}
}

func TestEncryptionAlgoIsAEAD(t *testing.T) {
	aead := []EncryptionAlgo{EncryptionAES128GCM, EncryptionAES256GCM, EncryptionChaCha20Poly}
	nonAEAD := []EncryptionAlgo{EncryptionAES128, EncryptionAES256, Encryption3DES}

	for _, a := range aead {
		if !a.IsAEAD() {
			t.Errorf("%s.IsAEAD() = false, want true", a)
		}
	}
	for _, a := range nonAEAD {
		if a.IsAEAD() {
			t.Errorf("%s.IsAEAD() = true, want false", a)
		}
	}
}

func TestParseEncryptionAlgoInvalid(t *testing.T) {
	invalid := []string{"des", "aes512", "blowfish", ""}
	for _, s := range invalid {
		if _, ok := ParseEncryptionAlgo(s); ok {
			t.Errorf("ParseEncryptionAlgo(%q) returned ok, want !ok", s)
		}
	}
}

func TestHashAlgoString(t *testing.T) {
	tests := []struct {
		algo HashAlgo
		want string
	}{
		{HashSHA1, "sha1"},
		{HashSHA256, "sha256"},
		{HashSHA384, "sha384"},
		{HashSHA512, "sha512"},
		{HashUnknown, unknownEnum},
	}
	for _, tt := range tests {
		if got := tt.algo.String(); got != tt.want {
			t.Errorf("HashAlgo(%d).String() = %q, want %q", tt.algo, got, tt.want)
		}
		if tt.algo == HashUnknown {
			continue
		}
		parsed, ok := parseHashAlgo(tt.want)
		if !ok {
			t.Errorf("ParseHashAlgo(%q) returned !ok", tt.want)
		}
		if parsed != tt.algo {
			t.Errorf("ParseHashAlgo(%q) = %d, want %d", tt.want, parsed, tt.algo)
		}
	}
}

func TestDHGroupString(t *testing.T) {
	tests := []struct {
		group DHGroup
		valid bool
	}{
		{0, false},
		{1, true},
		{14, true},
		{31, true},
		{32, false},
		{99, false},
	}
	for _, tt := range tests {
		if got := ValidDHGroup(tt.group); got != tt.valid {
			t.Errorf("ValidDHGroup(%d) = %v, want %v", tt.group, got, tt.valid)
		}
	}
}

func TestPFSModeRoundTrip(t *testing.T) {
	for _, name := range []string{"enable", "disable"} {
		p, ok := parsePFSMode(name)
		if !ok {
			t.Fatalf("ParsePFSMode(%q) !ok", name)
		}
		if p.String() != name {
			t.Errorf("PFSMode round-trip: %q -> %d -> %q", name, p, p.String())
		}
	}
}

func TestAuthModeRoundTrip(t *testing.T) {
	for _, name := range []string{"pre-shared-secret", "x509", "eap-tls", "eap-mschapv2"} {
		m, ok := ParseAuthMode(name)
		if !ok {
			t.Fatalf("ParseAuthMode(%q) !ok", name)
		}
		if m.String() != name {
			t.Errorf("AuthMode round-trip: %q -> %d -> %q", name, m, m.String())
		}
	}
}

func TestConnectionTypeRoundTrip(t *testing.T) {
	for _, name := range []string{"initiate", "respond"} {
		c, ok := parseConnectionType(name)
		if !ok {
			t.Fatalf("ParseConnectionType(%q) !ok", name)
		}
		if c.String() != name {
			t.Errorf("ConnectionType round-trip: %q -> %d -> %q", name, c, c.String())
		}
	}
}

func TestCloseActionRoundTrip(t *testing.T) {
	for _, name := range []string{"none", "start", "restart"} {
		a, ok := parseCloseAction(name)
		if !ok {
			t.Fatalf("ParseCloseAction(%q) !ok", name)
		}
		if a.String() != name {
			t.Errorf("CloseAction round-trip: %q -> %d -> %q", name, a, a.String())
		}
	}
}

func TestDPDActionRoundTrip(t *testing.T) {
	for _, name := range []string{"restart", "hold", "clear"} {
		a, ok := parseDPDAction(name)
		if !ok {
			t.Fatalf("ParseDPDAction(%q) !ok", name)
		}
		if a.String() != name {
			t.Errorf("DPDAction round-trip: %q -> %d -> %q", name, a, a.String())
		}
	}
}

func TestKeyExchangeRoundTrip(t *testing.T) {
	for _, name := range []string{"ikev1", "ikev2"} {
		k, ok := parseKeyExchange(name)
		if !ok {
			t.Fatalf("ParseKeyExchange(%q) !ok", name)
		}
		if k.String() != name {
			t.Errorf("KeyExchange round-trip: %q -> %d -> %q", name, k, k.String())
		}
	}
}
