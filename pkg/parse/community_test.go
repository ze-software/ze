package parse

import "testing"

// TestCommunity tests single community parsing per RFC 1997.
func TestCommunity(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint32
		wantErr bool
	}{
		// Valid ASN:value format
		{"simple", "2914:666", (2914 << 16) | 666, false},
		{"zero values", "0:0", 0, false},
		{"max values", "65535:65535", 0xFFFFFFFF, false},

		// Well-known communities per RFC 1997
		{"no-export hyphen", "no-export", CommunityNoExport, false},
		{"no_export underscore", "no_export", CommunityNoExport, false},
		{"no-advertise", "no-advertise", CommunityNoAdvertise, false},
		{"no-export-subconfed", "no-export-subconfed", CommunityNoExportSubconfed, false},
		{"nopeer", "nopeer", CommunityNoPeer, false},
		{"no-peer", "no-peer", CommunityNoPeer, false},
		{"no_peer", "no_peer", CommunityNoPeer, false},
		{"blackhole RFC 7999", "blackhole", CommunityBlackhole, false},

		// Case insensitivity
		{"NO-EXPORT uppercase", "NO-EXPORT", CommunityNoExport, false},
		{"No-Advertise mixed", "No-Advertise", CommunityNoAdvertise, false},
		{"BLACKHOLE uppercase", "BLACKHOLE", CommunityBlackhole, false},

		// Bare integers (ExaBGP compatible)
		{"bare integer zero", "0", 0, false},
		{"bare integer", "2914666", 2914666, false},
		{"bare integer max", "4294967295", 0xFFFFFFFF, false},

		// Hex format (ExaBGP compatible)
		{"hex format", "0x12345678", 0x12345678, false},
		{"hex format lowercase", "0xabcdef00", 0xABCDEF00, false},
		{"hex format uppercase", "0xABCDEF00", 0xABCDEF00, false},

		// Invalid
		{"too many colons", "2914:666:1", 0, true},
		{"invalid ASN", "abc:666", 0, true},
		{"invalid value", "2914:abc", 0, true},
		{"ASN too large", "65536:1", 0, true},
		{"value too large", "1:65536", 0, true},
		{"empty string", "", 0, true},
		{"unknown name", "unknown", 0, true},
		{"invalid hex", "0xGGGG", 0, true},
		{"bare integer overflow", "4294967296", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Community(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Community(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Community(%q) = 0x%08X, want 0x%08X", tt.input, got, tt.want)
			}
		})
	}
}

// TestLargeCommunity tests single large community parsing per RFC 8092.
func TestLargeCommunity(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    [3]uint32
		wantErr bool
	}{
		// Valid
		{"simple", "2914:100:200", [3]uint32{2914, 100, 200}, false},
		{"zeros", "0:0:0", [3]uint32{0, 0, 0}, false},
		{"max values", "4294967295:4294967295:4294967295", [3]uint32{0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF}, false},

		// Invalid
		{"missing parts", "2914:100", [3]uint32{}, true},
		{"too many parts", "2914:100:200:300", [3]uint32{}, true},
		{"invalid global admin", "abc:100:200", [3]uint32{}, true},
		{"invalid local data 1", "2914:abc:200", [3]uint32{}, true},
		{"invalid local data 2", "2914:100:abc", [3]uint32{}, true},
		{"empty string", "", [3]uint32{}, true},
		{"overflow global admin", "4294967296:0:0", [3]uint32{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LargeCommunity(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("LargeCommunity(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("LargeCommunity(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
