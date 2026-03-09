package secret

import (
	"strings"
	"testing"
)

func TestEncodeHasPrefix(t *testing.T) {
	encoded, err := Encode("abc123")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !strings.HasPrefix(encoded, Prefix) {
		t.Errorf("encoded %q missing prefix %q", encoded, Prefix)
	}
}

func TestDecodeRecoversPlaintext(t *testing.T) {
	tests := []string{
		"abc123",
		"hello",
		"p@ssw0rd!",
		"a",
		"",
		"longer password with spaces and $pecial chars!@#",
		"\x00\x01\xff", // binary bytes
	}
	for _, plain := range tests {
		encoded, err := Encode(plain)
		if err != nil {
			t.Fatalf("Encode(%q): %v", plain, err)
		}
		decoded, err := Decode(encoded)
		if err != nil {
			t.Fatalf("Decode(%q): %v", encoded, err)
		}
		if decoded != plain {
			t.Errorf("round-trip failed: input=%q encoded=%q decoded=%q", plain, encoded, decoded)
		}
	}
}

func TestEncodeRandomSalt(t *testing.T) {
	// Two encodes of the same value should produce different output
	// (random salt). Run enough times to be confident.
	encoded1, err := Encode("test")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	different := false
	for range 20 {
		encoded2, err := Encode("test")
		if err != nil {
			t.Fatalf("Encode: %v", err)
		}
		if encoded1 != encoded2 {
			different = true
			break
		}
	}
	if !different {
		t.Error("20 encodes of same value produced identical output — salt not random")
	}
}

func TestDecodeInvalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"no prefix", "abc123"},
		{"prefix only", "$9$"},
		// "a" is family 2 (extra=1), so $9$a has salt but not enough extra chars
		{"too short for extra", "$9$a"},
		// "Q" is family 0 (extra=3), so $9$Qab has salt+2 extra but needs 3
		{"too short for extra 2", "$9$Qab"},
		{"invalid chars", "$9$!!!!!!"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decode(tt.input)
			if err == nil {
				t.Errorf("Decode(%q) should have returned error", tt.input)
			}
		})
	}
}

func TestIsEncoded(t *testing.T) {
	if IsEncoded("plaintext") {
		t.Error("plaintext should not be detected as encoded")
	}
	if !IsEncoded("$9$abcdef") {
		t.Error("$9$ prefixed string should be detected as encoded")
	}
	if IsEncoded("$9") {
		t.Error("incomplete prefix should not be detected")
	}
}

func TestDecodeKnownValue(t *testing.T) {
	// Encode a known value, then verify decode works
	plain := "secret"
	encoded, err := Encode(plain)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded != plain {
		t.Errorf("got %q, want %q", decoded, plain)
	}
}

func FuzzEncodeDecode(f *testing.F) {
	f.Add("abc123")
	f.Add("")
	f.Add("p@ssw0rd!")
	f.Add("\x00\xff")
	f.Add("a]b[c{d}e;f")

	f.Fuzz(func(t *testing.T, plain string) {
		encoded, err := Encode(plain)
		if err != nil {
			t.Fatalf("Encode(%q): %v", plain, err)
		}
		if !strings.HasPrefix(encoded, Prefix) {
			t.Fatalf("encoded missing prefix: %q", encoded)
		}
		decoded, err := Decode(encoded)
		if err != nil {
			t.Fatalf("Decode(%q): %v (original: %q)", encoded, err, plain)
		}
		if decoded != plain {
			t.Errorf("round-trip: %q → %q → %q", plain, encoded, decoded)
		}
	})
}
