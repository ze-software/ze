// RFC 7011 conformance tests for the collector's UDP payload bound and the
// UDP checksum Ze's socket produces. Both are config and socket surfaces of
// this package, reached by every encoder through Sender.
//
// VALIDATES: max-datagram-size is configurable, the default reserves
// IPv6 and UDP headers within 512 octets, and invalid bounds are refused.
// PREVENTS: an unknown-PMTU default larger than RFC 7011 recommends.

package flowexport

import (
	"strings"
	"testing"
)

// RFC 7011 Section 10.3.3 recommends a 512-octet packet when PMTU is unknown.
// Ze's UDP sockets add no IP options or IPv6 extension headers.
const (
	unknownPMTUPacketSize = 512
	ipv6UDPHeaders        = 40 + 8
)

// RFC requirement: RFC7011-10.3.3-1 positive -- an unset bound plus IPv6
// and UDP headers stays within 512 octets, and an explicit 600-octet payload
// bound reaches the sender.
func TestRFC7011MaxDatagramSizeConfigured(t *testing.T) {
	cfg, err := ParseConfig(`{"flow-export":{"collector":[{"name":"c1","address":"127.0.0.1","protocol":"ipfix"}]}}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Collectors[0].MaxDatagramSize; got != DatagramSizeDefault {
		t.Fatalf("default max-datagram-size = %d, want %d", got, DatagramSizeDefault)
	}
	if DatagramSizeDefault+ipv6UDPHeaders > unknownPMTUPacketSize {
		t.Fatalf("default %d + %d header octets exceeds %d", DatagramSizeDefault, ipv6UDPHeaders, unknownPMTUPacketSize)
	}

	cfg, err = ParseConfig(`{"flow-export":{"collector":[{"name":"c1","address":"127.0.0.1","protocol":"ipfix","max-datagram-size":600}]}}`)
	if err != nil {
		t.Fatal(err)
	}
	c := cfg.Collectors[0]
	if c.MaxDatagramSize != 600 {
		t.Fatalf("max-datagram-size = %d, want 600", c.MaxDatagramSize)
	}
	s, err := NewSender(c.Address, c.Port, "", c.MaxDatagramSize)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if s.MaxDatagram() != 600 {
		t.Fatalf("sender bound = %d, want the configured 600", s.MaxDatagram())
	}
}

// RFC requirement: RFC7011-10.3.3-1 negative -- a max-datagram-size above
// the 1400-octet buffer or below the 464-octet floor is refused by Validate,
// naming the leaf, so no sender is ever built with a bound the encoders
// cannot honor.
func TestRFC7011MaxDatagramSizeOutOfRangeRefused(t *testing.T) {
	for _, size := range []int{0, DatagramSizeMin - 1, MaxDatagramSize + 1, 9000} {
		cfg := &Config{Collectors: []CollectorConfig{{
			Name: "c1", Address: "10.0.0.1", Port: 4739, Protocol: "ipfix",
			PollingInterval: 20, TemplateRefresh: 600, MaxDatagramSize: size,
		}}}
		err := cfg.Validate()
		if err == nil {
			t.Fatalf("max-datagram-size %d accepted, want refused", size)
		}
		if !strings.Contains(err.Error(), "max-datagram-size") {
			t.Fatalf("max-datagram-size %d: error %q does not name the leaf", size, err)
		}
	}
	for _, size := range []int{DatagramSizeMin, MaxDatagramSize} {
		cfg := &Config{Collectors: []CollectorConfig{{
			Name: "c1", Address: "10.0.0.1", Port: 4739, Protocol: "ipfix",
			PollingInterval: 20, TemplateRefresh: 600, MaxDatagramSize: size,
		}}}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("max-datagram-size %d refused: %v", size, err)
		}
	}
}

// TestMaxDatagramSizeMalformedRefused rejects explicit malformed values
// rather than exporting with an unrelated default.
func TestMaxDatagramSizeMalformedRefused(t *testing.T) {
	for _, value := range []string{`"invalid"`, `null`, `true`, `600.5`, `[]`} {
		cfg, err := ParseConfig(`{"flow-export":{"collector":[{"name":"c1","address":"127.0.0.1","protocol":"ipfix","max-datagram-size":` + value + `}]}}`)
		if err != nil {
			t.Fatal(err)
		}
		if err := cfg.Validate(); err == nil {
			t.Fatalf("malformed bound %s accepted", value)
		}
	}
}
