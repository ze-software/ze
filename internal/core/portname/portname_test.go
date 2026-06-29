package portname

import "testing"

// VALIDATES: known ports resolve to service names, unknown ports to numeric strings (AC-5/AC-12).
// Uses (port, proto) pairs from the generated /etc/services table.
func TestPortnameLookup(t *testing.T) {
	tests := []struct {
		port  uint16
		proto uint8
		name  string
	}{
		{22, 6, "ssh"},
		{53, 17, "domain"},
		{53, 6, "domain"},
		{80, 6, "http"},
		{443, 6, "https"},
		{179, 6, "bgp"},
		{0, 0, "0"},
		{65535, 0, "65535"},
	}
	for _, tt := range tests {
		info := Lookup(tt.port, tt.proto)
		if info.Name != tt.name {
			t.Errorf("Lookup(%d, %d).Name = %q, want %q", tt.port, tt.proto, info.Name, tt.name)
		}
	}
}

// VALIDATES: proto=0 falls back to port-only resolution.
func TestPortnameLookupProtoZero(t *testing.T) {
	info := Lookup(22, 0)
	if info.Name != "ssh" {
		t.Errorf("Lookup(22, 0).Name = %q, want ssh", info.Name)
	}
}

// VALIDATES: (port, proto) pairs with different names resolve correctly.
func TestPortnameLookupProtoDiffers(t *testing.T) {
	tcp := Lookup(512, 6)
	udp := Lookup(512, 17)
	if tcp.Name == udp.Name {
		t.Fatalf("port 512 tcp=%q udp=%q; expected different names", tcp.Name, udp.Name)
	}
}

// VALIDATES: reflection-port amplification labels are derived from service names (AC-5).
func TestPortnameAmplificationOverlay(t *testing.T) {
	amplified := []struct {
		port uint16
		amp  string
	}{
		{53, "domain-amplification"},
		{123, "ntp-amplification"},
		{1900, "ssdp-amplification"},
		{389, "ldap-amplification"},
		{161, "snmp-amplification"},
		{19, "chargen-amplification"},
	}
	for _, tt := range amplified {
		info := Lookup(tt.port, 17)
		if info.Amplification != tt.amp {
			t.Errorf("Lookup(%d, 17).Amplification = %q, want %q", tt.port, info.Amplification, tt.amp)
		}
	}

	noAmp := []uint16{22, 80, 443, 179, 0, 65535}
	for _, port := range noAmp {
		info := Lookup(port, 0)
		if info.Amplification != "" {
			t.Errorf("Lookup(%d, 0).Amplification = %q, want empty", port, info.Amplification)
		}
	}
}
