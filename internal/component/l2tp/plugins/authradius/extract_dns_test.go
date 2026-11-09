package l2tpauthradius

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/radius"
)

// TestExtractMSDNSServers checks that an Access-Accept's MS-Primary-DNS-Server
// and MS-Secondary-DNS-Server reach AuthMetadata, the pool's radius source.
// Method: build Access-Accepts with vendor 311 types 28 and 29, alone, both,
// malformed and under another vendor, and read the extracted metadata.
//
// RFC requirement: RFC2548-2.7.6-1 positive -- an Access-Accept's
// MS-Primary-DNS-Server (vendor 311, type 28, a four-octet address) is read
// as the subscriber's primary DNS server, alone or beside the secondary, the
// first of two such attributes winning (extract_dns.go extractMSDNSServers).
// RFC requirement: RFC2548-2.7.7-1 positive -- an Access-Accept's
// MS-Secondary-DNS-Server (vendor 311, type 29, a four-octet address) is read
// as the subscriber's secondary DNS server, alone or beside the primary
// (extract_dns.go extractMSDNSServers).
func TestExtractMSDNSServers(t *testing.T) {
	primary := netip.MustParseAddr("198.51.100.53")
	secondary := netip.MustParseAddr("198.51.100.54")
	cases := []struct {
		name          string
		attrs         []radius.Attr
		wantPrimary   netip.Addr
		wantSecondary netip.Addr
	}{
		{
			name: "both",
			attrs: []radius.Attr{
				buildVSAAttr(t, radius.VendorMicrosoft, radius.MSPrimaryDNSServer, primary.AsSlice()),
				buildVSAAttr(t, radius.VendorMicrosoft, radius.MSSecondaryDNSServer, secondary.AsSlice()),
			},
			wantPrimary:   primary,
			wantSecondary: secondary,
		},
		{
			name:        "primary alone",
			attrs:       []radius.Attr{buildVSAAttr(t, radius.VendorMicrosoft, radius.MSPrimaryDNSServer, primary.AsSlice())},
			wantPrimary: primary,
		},
		{
			name:          "secondary alone",
			attrs:         []radius.Attr{buildVSAAttr(t, radius.VendorMicrosoft, radius.MSSecondaryDNSServer, secondary.AsSlice())},
			wantSecondary: secondary,
		},
		{
			name: "first of each type wins",
			attrs: []radius.Attr{
				buildVSAAttr(t, radius.VendorMicrosoft, radius.MSPrimaryDNSServer, primary.AsSlice()),
				buildVSAAttr(t, radius.VendorMicrosoft, radius.MSPrimaryDNSServer, secondary.AsSlice()),
			},
			wantPrimary: primary,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			meta := extractAuthMetadata(&radius.Packet{Attrs: tc.attrs})
			if meta == nil {
				t.Fatal("expected metadata from an Access-Accept carrying MS DNS")
			}
			if meta.DNSPrimary != tc.wantPrimary {
				t.Errorf("DNSPrimary = %v, want %v", meta.DNSPrimary, tc.wantPrimary)
			}
			if meta.DNSSecondary != tc.wantSecondary {
				t.Errorf("DNSSecondary = %v, want %v", meta.DNSSecondary, tc.wantSecondary)
			}
		})
	}
}

// TestExtractMSDNSServersIgnoresMalformed checks that an attribute that is not
// a four-octet address, or that belongs to another vendor's type 28, carries
// no resolver. Method: each Access-Accept holds only such attributes, so no
// metadata is answered at all.
func TestExtractMSDNSServersIgnoresMalformed(t *testing.T) {
	cases := map[string]radius.Attr{
		"three octets":   buildVSAAttr(t, radius.VendorMicrosoft, radius.MSPrimaryDNSServer, []byte{198, 51, 100}),
		"sixteen octets": buildVSAAttr(t, radius.VendorMicrosoft, radius.MSSecondaryDNSServer, netip.MustParseAddr("2001:db8::53").AsSlice()),
		"other vendor":   buildVSAAttr(t, radius.VendorCisco, radius.MSPrimaryDNSServer, []byte{198, 51, 100, 53}),
	}
	for name, attr := range cases {
		t.Run(name, func(t *testing.T) {
			if meta := extractAuthMetadata(&radius.Packet{Attrs: []radius.Attr{attr}}); meta != nil {
				t.Errorf("metadata = %+v, want none from a malformed MS DNS attribute", meta)
			}
		})
	}
}
