package flowspec

import (
	"testing"
)

// FuzzParseFlowSpec tests FlowSpec NLRI binary parsing robustness.
//
// VALIDATES: ParseFlowSpec handles arbitrary bytes without crashing.
// PREVENTS: Panic on truncated data, malformed operators, invalid component types.
// SECURITY: FlowSpec bytes come from untrusted BGP UPDATE NLRI.
func FuzzParseFlowSpec(f *testing.F) {
	// IPv4 FlowSpec: destination prefix 10.0.0.0/24
	// Length=5, Type=1 (dest), prefix-len=24, prefix bytes
	f.Add([]byte{0x05, 0x01, 0x18, 0x0A, 0x00, 0x00})
	// IPv4 FlowSpec: source prefix 192.168.0.0/16
	f.Add([]byte{0x04, 0x02, 0x10, 0xC0, 0xA8})
	// IPv4 FlowSpec: IP protocol = TCP (6)
	// Length=3, Type=3 (protocol), operator=0x81 (end+match), value=6
	f.Add([]byte{0x03, 0x03, 0x81, 0x06})
	// IPv4 FlowSpec: destination port = 80
	// Length=4, Type=5 (dest port), operator=0x91 (end+eq+1byte), value=80
	f.Add([]byte{0x04, 0x05, 0x91, 0x00, 0x50})
	// IPv4 FlowSpec: multi-component (dest prefix + protocol + port)
	f.Add([]byte{
		0x0C,                         // length = 12
		0x01, 0x18, 0x0A, 0x00, 0x00, // dest 10.0.0.0/24
		0x03, 0x81, 0x06, // protocol TCP
		0x05, 0x91, 0x00, 0x50, // dest port 80
	})
	// IPv4 FlowSpec: DSCP (type 11)
	f.Add([]byte{0x03, 0x0B, 0x81, 0x2E}) // DSCP 46
	// IPv4 FlowSpec: TCP flags (type 9)
	f.Add([]byte{0x03, 0x09, 0x81, 0x02}) // SYN flag
	// IPv4 FlowSpec: ICMP type (type 7)
	f.Add([]byte{0x03, 0x07, 0x81, 0x08})                         // ICMP echo
	f.Add([]byte{})                                               // Empty
	f.Add([]byte{0x00})                                           // Zero length
	f.Add([]byte{0x01})                                           // Length 1 but no data
	f.Add([]byte{0xFF, 0xFF})                                     // Extended length marker
	f.Add([]byte{0xF0, 0x01, 0x00, 0x01, 0x18, 0x0A, 0x00, 0x00}) // Extended length

	f.Fuzz(func(t *testing.T, data []byte) {
		fs, err := ParseFlowSpec(IPv4FlowSpec, data)
		if err != nil {
			return
		}
		// Exercise methods — MUST NOT panic
		_ = fs.String()
		_ = fs.Bytes()
		_ = fs.Len()
		buf := make([]byte, fs.Len()+10)
		_ = fs.WriteTo(buf, 0)
		_ = fs.Family()
	})
}

// FuzzParseFlowSpecIPv6 tests IPv6 FlowSpec NLRI parsing robustness.
//
// VALIDATES: ParseFlowSpec with IPv6 family handles arbitrary bytes without crashing.
// PREVENTS: Panic on IPv6-specific offset handling, extended prefix lengths.
// SECURITY: IPv6 FlowSpec bytes come from untrusted BGP UPDATE NLRI.
func FuzzParseFlowSpecIPv6(f *testing.F) {
	// IPv6 FlowSpec: destination prefix 2001:db8::/32
	// Type=1 (dest), prefix-len=32, offset=0, prefix bytes
	f.Add([]byte{0x07, 0x01, 0x20, 0x00, 0x20, 0x01, 0x0D, 0xB8})
	// IPv6 FlowSpec: protocol = TCP
	f.Add([]byte{0x03, 0x03, 0x81, 0x06})
	f.Add([]byte{})     // Empty
	f.Add([]byte{0x00}) // Zero length

	f.Fuzz(func(t *testing.T, data []byte) {
		fs, err := ParseFlowSpec(IPv6FlowSpec, data)
		if err != nil {
			return
		}
		_ = fs.String()
		_ = fs.Bytes()
		_ = fs.Len()
	})
}

// FuzzParseFlowSpecVPN tests FlowSpec VPN NLRI parsing robustness.
//
// VALIDATES: ParseFlowSpecVPN handles arbitrary bytes without crashing.
// PREVENTS: Panic on malformed RD + FlowSpec combination, length confusion.
// SECURITY: FlowSpec VPN bytes come from untrusted BGP UPDATE NLRI.
func FuzzParseFlowSpecVPN(f *testing.F) {
	// FlowSpec VPN: RD type 0 (65001:100) + dest prefix 10.0.0.0/24
	f.Add([]byte{
		0x0D,                                           // length = 13
		0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0x00, 0x64, // RD
		0x01, 0x18, 0x0A, 0x00, 0x00, // dest 10.0.0.0/24
	})
	f.Add([]byte{})                       // Empty
	f.Add([]byte{0x08})                   // Length 8 but no RD data
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF}) // Max values

	f.Fuzz(func(t *testing.T, data []byte) {
		fsv, err := ParseFlowSpecVPN(IPv4FlowSpecVPN, data)
		if err != nil {
			return
		}
		_ = fsv.String()
		_ = fsv.Bytes()
		_ = fsv.Len()
		_ = fsv.Family()
	})
}
