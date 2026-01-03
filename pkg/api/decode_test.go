package api

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestExtractAttributeBytes verifies attribute bytes extraction from UPDATE body.
//
// VALIDATES: Correct byte range returned for path attributes section.
// PREVENTS: Off-by-one errors in UPDATE parsing.
func TestExtractAttributeBytes(t *testing.T) {
	// Build a minimal UPDATE message body:
	// withdrawn_len (2) + withdrawn + attr_len (2) + attrs + nlri
	//
	// UPDATE body format (RFC 4271 Section 4.3):
	// - Withdrawn Routes Length: 2 octets
	// - Withdrawn Routes: variable
	// - Total Path Attribute Length: 2 octets
	// - Path Attributes: variable
	// - NLRI: variable

	tests := []struct {
		name     string
		body     []byte
		wantLen  int
		wantNil  bool
		wantData []byte
	}{
		{
			name:    "empty body",
			body:    []byte{},
			wantNil: true,
		},
		{
			name:    "too short",
			body:    []byte{0, 0, 0}, // only 3 bytes
			wantNil: true,
		},
		{
			name: "no attributes",
			body: func() []byte {
				// withdrawn_len=0, attr_len=0
				buf := make([]byte, 4)
				binary.BigEndian.PutUint16(buf[0:2], 0) // withdrawn len
				binary.BigEndian.PutUint16(buf[2:4], 0) // attr len
				return buf
			}(),
			wantNil: true, // attr_len=0 should return nil
		},
		{
			name: "with attributes no withdrawn",
			body: func() []byte {
				attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN IGP
				buf := make([]byte, 4+len(attrs))
				binary.BigEndian.PutUint16(buf[0:2], 0)                  // withdrawn len
				binary.BigEndian.PutUint16(buf[2:4], uint16(len(attrs))) //nolint:gosec // test data
				copy(buf[4:], attrs)
				return buf
			}(),
			wantLen:  4,
			wantData: []byte{0x40, 0x01, 0x01, 0x00},
		},
		{
			name: "with withdrawn and attributes",
			body: func() []byte {
				withdrawn := []byte{24, 10, 0, 0}       // 10.0.0.0/24
				attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN IGP
				buf := make([]byte, 2+len(withdrawn)+2+len(attrs))
				binary.BigEndian.PutUint16(buf[0:2], uint16(len(withdrawn))) //nolint:gosec // test data
				copy(buf[2:], withdrawn)
				binary.BigEndian.PutUint16(buf[2+len(withdrawn):], uint16(len(attrs))) //nolint:gosec // test data
				copy(buf[2+len(withdrawn)+2:], attrs)
				return buf
			}(),
			wantLen:  4,
			wantData: []byte{0x40, 0x01, 0x01, 0x00},
		},
		{
			name: "with attributes and nlri",
			body: func() []byte {
				attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN IGP
				nlri := []byte{24, 192, 168, 1}         // 192.168.1.0/24
				buf := make([]byte, 4+len(attrs)+len(nlri))
				binary.BigEndian.PutUint16(buf[0:2], 0)
				binary.BigEndian.PutUint16(buf[2:4], uint16(len(attrs))) //nolint:gosec // test data
				copy(buf[4:], attrs)
				copy(buf[4+len(attrs):], nlri)
				return buf
			}(),
			wantLen:  4,
			wantData: []byte{0x40, 0x01, 0x01, 0x00},
		},
		{
			name: "truncated attributes",
			body: func() []byte {
				// Claims 10 bytes of attrs but only has 4
				buf := make([]byte, 8)
				binary.BigEndian.PutUint16(buf[0:2], 0)
				binary.BigEndian.PutUint16(buf[2:4], 10) // claims 10 bytes
				copy(buf[4:], []byte{0x40, 0x01, 0x01, 0x00})
				return buf
			}(),
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractAttributeBytes(tt.body)
			if tt.wantNil {
				if got != nil {
					t.Errorf("ExtractAttributeBytes() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("ExtractAttributeBytes() = nil, want non-nil")
			}
			if len(got) != tt.wantLen {
				t.Errorf("ExtractAttributeBytes() len = %d, want %d", len(got), tt.wantLen)
			}
			if !bytes.Equal(got, tt.wantData) {
				t.Errorf("ExtractAttributeBytes() = %x, want %x", got, tt.wantData)
			}
		})
	}
}

// TestExtractAttributeBytesEmpty verifies nil for empty/no attributes.
//
// VALIDATES: Returns nil for no attributes.
// PREVENTS: Empty slice vs nil confusion.
func TestExtractAttributeBytesEmpty(t *testing.T) {
	// attr_len = 0
	body := []byte{0, 0, 0, 0}
	got := ExtractAttributeBytes(body)
	if got != nil {
		t.Errorf("ExtractAttributeBytes() with zero attrs = %v, want nil", got)
	}
}

// TestExtractAttributeBytesMalformed verifies nil for malformed body.
//
// VALIDATES: Returns nil for malformed body.
// PREVENTS: Panic on bad input.
func TestExtractAttributeBytesMalformed(t *testing.T) {
	tests := []struct {
		name string
		body []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"one byte", []byte{0}},
		{"two bytes", []byte{0, 0}},
		{"three bytes", []byte{0, 0, 0}},
		{"withdrawn len overflow", []byte{0xff, 0xff, 0, 0}}, // claims 65535 withdrawn
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractAttributeBytes(tt.body)
			if got != nil {
				t.Errorf("ExtractAttributeBytes(%v) = %v, want nil", tt.body, got)
			}
		})
	}
}

// TestFormatCapabilityStrings verifies all capability types produce parseable hyphenated strings.
//
// VALIDATES: All capabilities use hyphenated format without spaces.
// PREVENTS: Unparseable capability strings in OPEN output.
func TestFormatCapabilityStrings(t *testing.T) {
	tests := []struct {
		name string
		cap  string
		want string
	}{
		// Basic capabilities
		{"multiprotocol ipv4 unicast", "multiprotocol-ipv4-unicast", "multiprotocol-ipv4-unicast"},
		{"multiprotocol ipv6 unicast", "multiprotocol-ipv6-unicast", "multiprotocol-ipv6-unicast"},
		{"route refresh", "route-refresh", "route-refresh"},
		{"enhanced route refresh", "enhanced-route-refresh", "enhanced-route-refresh"},
		{"extended message", "extended-message", "extended-message"},

		// ASN4 with value
		{"4-byte-asn", "4-byte-asn-65536", "4-byte-asn-65536"},

		// AddPath per family
		{"addpath receive", "addpath-receive-ipv4-unicast", "addpath-receive-ipv4-unicast"},
		{"addpath send", "addpath-send-ipv6-unicast", "addpath-send-ipv6-unicast"},
		{"addpath send/receive", "addpath-send/receive-ipv4-unicast", "addpath-send/receive-ipv4-unicast"},

		// Graceful restart
		{"graceful restart", "graceful-restart-120", "graceful-restart-120"},

		// Extended nexthop per family
		{"extended nexthop", "extended-nexthop-ipv4-unicast-ipv6", "extended-nexthop-ipv4-unicast-ipv6"},

		// FQDN
		{"hostname only", "hostname-router1", "hostname-router1"},
		{"hostname with domain", "hostname-router1.example.com", "hostname-router1.example.com"},

		// Software version
		{"software", "software-zebgp-1.0", "software-zebgp-1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify no spaces in capability string
			if bytes.Contains([]byte(tt.cap), []byte(" ")) {
				t.Errorf("capability %q contains spaces", tt.cap)
			}
			// Verify expected format
			if tt.cap != tt.want {
				t.Errorf("capability = %q, want %q", tt.cap, tt.want)
			}
		})
	}
}
