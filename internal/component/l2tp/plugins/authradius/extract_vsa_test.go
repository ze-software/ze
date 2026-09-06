package l2tpauthradius

import (
	"testing"

	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/component/traffic"
)

func buildVSAAttr(t *testing.T, vendorID uint32, vendorType uint8, value []byte) radius.Attr {
	t.Helper()
	encoded, err := radius.EncodeVSA(vendorID, vendorType, value)
	if err != nil {
		t.Fatal(err)
	}
	return radius.Attr{Type: radius.AttrVendorSpecific, Value: encoded[2:]}
}

// VALIDATES: AC-1 -- Cisco-AVPair "subscriber:sub-qos-policy-in=gold".
func TestExtractCiscoAVPairCoS(t *testing.T) {
	tests := []struct {
		name string
		avp  string
		want string
	}{
		{"policy-in", "subscriber:sub-qos-policy-in=gold", "gold"},
		{"policy-out", "subscriber:sub-qos-policy-out=silver", "silver"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkt := &radius.Packet{Attrs: []radius.Attr{
				buildVSAAttr(t, radius.VendorCisco, radius.CiscoAVPair, []byte(tt.avp)),
			}}
			got := extractVSACoSProfile(pkt)
			if got != tt.want {
				t.Errorf("extractVSACoSProfile = %q, want %q", got, tt.want)
			}
		})
	}
}

// VALIDATES: AC-11, AC-14 -- malformed/equals-in-value Cisco-AVPair.
func TestExtractCiscoAVPairMalformed(t *testing.T) {
	tests := []struct {
		name string
		avp  string
		want string
	}{
		{"no-equals", "subscriber:sub-qos-policy-in", ""},
		{"empty-value", "subscriber:sub-qos-policy-in=", ""},
		{"equals-in-value", "subscriber:sub-qos-policy-in=name=with=equals", "name=with=equals"},
		{"unrecognized-key", "ip:vrf-id=red", ""},
		{"random-string", "random-no-equals-at-all", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkt := &radius.Packet{Attrs: []radius.Attr{
				buildVSAAttr(t, radius.VendorCisco, radius.CiscoAVPair, []byte(tt.avp)),
			}}
			got := extractVSACoSProfile(pkt)
			if got != tt.want {
				t.Errorf("extractVSACoSProfile = %q, want %q", got, tt.want)
			}
		})
	}
}

// VALIDATES: AC-3, AC-4 -- Juniper ERX ingress/egress policy.
func TestExtractJuniperCoS(t *testing.T) {
	tests := []struct {
		name       string
		vendorType uint8
		value      string
		want       string
	}{
		{"ingress", radius.ERXIngressPolicyName, "residential", "residential"},
		{"egress", radius.ERXEgressPolicyName, "business", "business"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkt := &radius.Packet{Attrs: []radius.Attr{
				buildVSAAttr(t, radius.VendorJuniper, tt.vendorType, []byte(tt.value)),
			}}
			got := extractVSACoSProfile(pkt)
			if got != tt.want {
				t.Errorf("extractVSACoSProfile = %q, want %q", got, tt.want)
			}
		})
	}
}

// VALIDATES: AC-5 -- Nokia Alc-Subscriber-QoS-Override.
func TestExtractNokiaCoS(t *testing.T) {
	pkt := &radius.Packet{Attrs: []radius.Attr{
		buildVSAAttr(t, radius.VendorNokia, radius.AlcSubscriberQoSOverride, []byte("premium")),
	}}
	got := extractVSACoSProfile(pkt)
	if got != "premium" {
		t.Errorf("extractVSACoSProfile = %q, want %q", got, "premium")
	}
}

// VALIDATES: AC-6 -- Huawei HW-Subscriber-QoS-Profile.
func TestExtractHuaweiCoS(t *testing.T) {
	pkt := &radius.Packet{Attrs: []radius.Attr{
		buildVSAAttr(t, radius.VendorHuawei, radius.HWSubscriberQoSProfile, []byte("enterprise")),
	}}
	got := extractVSACoSProfile(pkt)
	if got != "enterprise" {
		t.Errorf("extractVSACoSProfile = %q, want %q", got, "enterprise")
	}
}

// VALIDATES: AC-7, AC-13 -- MikroTik rate extraction.
func TestExtractMikrotikRate(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		wantRate uint64
	}{
		{"rx-and-tx", "10M/5M", 10_000_000},
		{"rx-only", "10M", 10_000_000},
		{"kilo", "100k/50k", 100_000},
		{"giga", "1G/500M", 1_000_000_000},
		{"plain-bps", "10000000/5000000", 10_000_000},
		{"with-burst", "10M/5M 20M/10M 8M/4M 10/10", 10_000_000},
		{"empty", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkt := &radius.Packet{Attrs: []radius.Attr{
				buildVSAAttr(t, radius.VendorMikrotik, radius.MikrotikRateLimit, []byte(tt.value)),
			}}
			got, _ := extractVSARates(pkt)
			if got != tt.wantRate {
				t.Errorf("extractVSARates = %d, want %d", got, tt.wantRate)
			}
		})
	}
}

// VALIDATES: AC-8 -- Ze "cos:" prefix wins over vendor VSA.
func TestZePrefixPriority(t *testing.T) {
	pkt := &radius.Packet{Attrs: []radius.Attr{
		{Type: radius.AttrFilterID, Value: []byte("cos:gold")},
		buildVSAAttr(t, radius.VendorCisco, radius.CiscoAVPair, []byte("subscriber:sub-qos-policy-in=silver")),
	}}
	meta := extractAuthMetadata(pkt)
	if meta == nil {
		t.Fatal("expected metadata")
	}
	if meta.CoSProfile != "gold" {
		t.Errorf("CoSProfile = %q, want %q (Ze prefix should win)", meta.CoSProfile, "gold")
	}
}

// VALIDATES: AC-9 -- no vendor VSA, no "cos:" Filter-Id.
func TestNoVendorVSA(t *testing.T) {
	pkt := &radius.Packet{Attrs: []radius.Attr{
		{Type: radius.AttrFilterID, Value: []byte("10mbit")},
	}}
	meta := extractAuthMetadata(pkt)
	if meta == nil {
		t.Fatal("expected metadata")
	}
	if meta.CoSProfile != "" {
		t.Errorf("CoSProfile = %q, want empty", meta.CoSProfile)
	}
	if meta.FilterID != "10mbit" {
		t.Errorf("FilterID = %q, want %q", meta.FilterID, "10mbit")
	}
}

// VALIDATES: AC-10 -- unknown vendor ID silently ignored.
func TestUnknownVendorIgnored(t *testing.T) {
	pkt := &radius.Packet{Attrs: []radius.Attr{
		buildVSAAttr(t, 99999, 1, []byte("anything")),
	}}
	got := extractVSACoSProfile(pkt)
	if got != "" {
		t.Errorf("extractVSACoSProfile = %q, want empty for unknown vendor", got)
	}
}

// VALIDATES: AC-15 -- empty VSA value.
func TestEmptyVSAValue(t *testing.T) {
	pkt := &radius.Packet{Attrs: []radius.Attr{
		buildVSAAttr(t, radius.VendorCisco, radius.CiscoAVPair, []byte{}),
	}}
	got := extractVSACoSProfile(pkt)
	if got != "" {
		t.Errorf("extractVSACoSProfile = %q, want empty for empty value", got)
	}
}

// VALIDATES: AC-1 -- vendor VSA wired into extractAuthMetadata.
func TestExtractAuthMetadataVendorCoS(t *testing.T) {
	tests := []struct {
		name     string
		vendorID uint32
		vtype    uint8
		value    string
		wantCoS  string
	}{
		{"cisco", radius.VendorCisco, radius.CiscoAVPair, "subscriber:sub-qos-policy-in=gold", "gold"},
		{"juniper", radius.VendorJuniper, radius.ERXIngressPolicyName, "residential", "residential"},
		{"nokia", radius.VendorNokia, radius.AlcSubscriberQoSOverride, "premium", "premium"},
		{"huawei", radius.VendorHuawei, radius.HWSubscriberQoSProfile, "enterprise", "enterprise"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkt := &radius.Packet{Attrs: []radius.Attr{
				buildVSAAttr(t, tt.vendorID, tt.vtype, []byte(tt.value)),
			}}
			meta := extractAuthMetadata(pkt)
			if meta == nil {
				t.Fatal("expected metadata")
			}
			if meta.CoSProfile != tt.wantCoS {
				t.Errorf("CoSProfile = %q, want %q", meta.CoSProfile, tt.wantCoS)
			}
		})
	}
}

// VALIDATES: AC-7 -- MikroTik rate wired into extractAuthMetadata.
//
// The expected value carries BOTH halves. It read "10000000bit" until
// 2026-09-06, which pinned the loss of the subscriber's upload rate: the
// MikroTik attribute says 10M down and 5M up, and the rewrite kept the first
// number only. Nothing enforced upload then, so the loss was invisible; the
// shaper now installs an ingress policer from this value.
func TestExtractAuthMetadataMikrotikRate(t *testing.T) {
	pkt := &radius.Packet{Attrs: []radius.Attr{
		buildVSAAttr(t, radius.VendorMikrotik, radius.MikrotikRateLimit, []byte("10M/5M")),
	}}
	meta := extractAuthMetadata(pkt)
	if meta == nil {
		t.Fatal("expected metadata")
	}
	if meta.FilterID != "10000000bit/5000000bit" {
		t.Errorf("FilterID = %q, want %q", meta.FilterID, "10000000bit/5000000bit")
	}
}

func TestParseMikrotikRate(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		wantDownload uint64
		wantUpload   uint64
	}{
		{"rx-tx", "10M/5M", 10_000_000, 5_000_000},
		{"rx-only", "10M", 10_000_000, 10_000_000},
		{"kilo", "100k/50k", 100_000, 50_000},
		{"giga", "1G/500M", 1_000_000_000, 500_000_000},
		{"plain-bps", "10000000/5000000", 10_000_000, 5_000_000},
		{"with-burst-fields", "10M/5M 20M/10M", 10_000_000, 5_000_000},
		{"empty", "", 0, 0},
		{"zero", "0/0", 0, 0},
		{"bad-format", "notanumber", 0, 0},
		{"overflow-G", "18446744074G", 0, 0},
		{"overflow-M", "18446744073710M", 0, 0},
		{"max-valid", "1000G/500G", 1_000_000_000_000, 500_000_000_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dl, ul := parseMikrotikRate([]byte(tt.value))
			if dl != tt.wantDownload {
				t.Errorf("download = %d, want %d", dl, tt.wantDownload)
			}
			if ul != tt.wantUpload {
				t.Errorf("upload = %d, want %d", ul, tt.wantUpload)
			}
		})
	}
}

func TestParseCiscoAVPairCoS(t *testing.T) {
	tests := []struct {
		name   string
		avp    string
		want   string
		wantOK bool
	}{
		{"policy-in", "subscriber:sub-qos-policy-in=gold", "gold", true},
		{"policy-out", "subscriber:sub-qos-policy-out=silver", "silver", true},
		{"equals-in-value", "subscriber:sub-qos-policy-in=name=with=equals", "name=with=equals", true},
		{"empty-value", "subscriber:sub-qos-policy-in=", "", false},
		{"no-equals", "subscriber:sub-qos-policy-in", "", false},
		{"other-key", "ip:vrf-id=red", "", false},
		{"no-key-value", "plaintext", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseCiscoAVPairCoS([]byte(tt.avp))
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("name = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMikrotikRateToFilterID(t *testing.T) {
	tests := []struct {
		download uint64
		upload   uint64
		want     string
	}{
		{0, 0, ""},
		{10_000_000, 10_000_000, "10000000bit"},
		{1_000_000_000, 0, "1000000000bit"},
		// An asymmetric pair keeps both halves, in the form the shaper reads.
		{20_000_000, 5_000_000, "20000000bit/5000000bit"},
	}
	for _, tt := range tests {
		got := mikrotikRateToFilterID(tt.download, tt.upload)
		if got != tt.want {
			t.Errorf("mikrotikRateToFilterID(%d, %d) = %q, want %q", tt.download, tt.upload, got, tt.want)
		}
	}
}

// TestAccessAcceptKeepsMikrotikUploadHalf checks the Access-Accept path. A
// MikroTik Rate-Limit of "10M/5M" is rewritten into the Filter-Id the shaper
// reads, and that rewrite must carry both directions: writing only the download
// rate loses the subscriber's upload limit before the shaper ever sees it.
func TestAccessAcceptKeepsMikrotikUploadHalf(t *testing.T) {
	encoded, err := radius.EncodeVSA(radius.VendorMikrotik, radius.MikrotikRateLimit, []byte("10M/5M"))
	if err != nil {
		t.Fatal(err)
	}
	pkt := &radius.Packet{
		Attrs: []radius.Attr{{Type: radius.AttrVendorSpecific, Value: encoded[2:]}},
	}
	meta := extractAuthMetadata(pkt)
	if meta == nil {
		t.Fatal("no metadata extracted from a packet carrying a MikroTik rate")
	}
	down, up, ok := traffic.ParseFilterIDRate(meta.FilterID)
	if !ok {
		t.Fatalf("the rewritten Filter-Id %q does not parse as a rate", meta.FilterID)
	}
	if down != 10_000_000 || up != 5_000_000 {
		t.Fatalf("Filter-Id %q parses as %d/%d, want 10000000/5000000", meta.FilterID, down, up)
	}
}
