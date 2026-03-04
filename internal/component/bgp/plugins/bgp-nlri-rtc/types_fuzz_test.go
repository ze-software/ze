package bgp_nlri_rtc

import (
	"testing"
)

// FuzzParseRTC tests RTC NLRI binary parsing robustness.
//
// VALIDATES: ParseRTC handles arbitrary bytes without crashing.
// PREVENTS: Panic on truncated data, malformed prefix length, RT parsing.
// SECURITY: RTC bytes come from untrusted BGP UPDATE NLRI.
func FuzzParseRTC(f *testing.F) {
	// Default RTC: prefix-length 0
	f.Add([]byte{0x00})
	// RTC with 2-byte ASN route target: prefixLen=96, origin-AS, RT type + value
	f.Add([]byte{
		0x60,                   // 96 bits
		0x00, 0x00, 0xFD, 0xE9, // origin AS 65001
		0x00, 0x02, // RT type 0x0002
		0xFD, 0xE9, 0x00, 0x00, 0x00, 0x64, // RT value
	})
	// RTC with 4-byte ASN route target
	f.Add([]byte{
		0x60,
		0x00, 0x00, 0xFD, 0xEA, // origin AS 65002
		0x02, 0x00, // RT type 0x0200
		0x00, 0x00, 0xFD, 0xE9, 0x00, 0xC8, // RT value
	})
	f.Add([]byte{})                                                     // Empty
	f.Add([]byte{0x60})                                                 // Prefix-len but no data
	f.Add([]byte{0xFF})                                                 // Invalid prefix length
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}) // Max values
	f.Add([]byte{0x60, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}) // All zeros

	f.Fuzz(func(t *testing.T, data []byte) {
		rtc, _, err := ParseRTC(data)
		if err != nil {
			return
		}
		// Exercise methods — MUST NOT panic
		_ = rtc.String()
		_ = rtc.Bytes()
		_ = rtc.Len()
		buf := make([]byte, rtc.Len()+10)
		_ = rtc.WriteTo(buf, 0)
		_ = rtc.OriginAS()
		_ = rtc.IsDefault()
		_ = rtc.RouteTargetValue()
	})
}
