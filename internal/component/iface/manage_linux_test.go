package iface

import (
	"testing"
)

func TestValidateInterfaceName(t *testing.T) {
	// VALIDATES: Interface name validation rejects empty, too-long, and accepts valid names.
	// PREVENTS: Invalid names passed to netlink causing kernel errors.
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty", "", true},
		{"one char", "a", false},
		{"max length 15", "abcdefghijklmno", false},
		{"too long 16", "abcdefghijklmnop", true},
		{"way too long", "this-name-is-way-too-long-for-linux", true},
		{"slash", "eth/0", true},
		{"null byte", "eth\x000", true},
		{"space", "eth 0", true},
		{"tab", "eth\t0", true},
		{"path traversal", "../../etc/a", true},
		{"dot-dot mid", "a..b", true},
		{"valid with dot", "eth0.100", false},
		{"valid with dash", "veth-a", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIfaceName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateIfaceName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateVLANID(t *testing.T) {
	// VALIDATES: VLAN ID validation enforces 802.1Q range [1, 4094].
	// PREVENTS: Reserved VLAN IDs 0 and 4095 reaching netlink.
	tests := []struct {
		name    string
		id      int
		wantErr bool
	}{
		{"zero invalid", 0, true},
		{"min valid", 1, false},
		{"mid range", 2000, false},
		{"max valid", 4094, false},
		{"4095 invalid", 4095, true},
		{"negative", -1, true},
		{"large", 10000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVLANID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateVLANID(%d) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

func TestValidateMTU(t *testing.T) {
	// VALIDATES: MTU validation enforces range [68, 16000].
	// PREVENTS: Kernel rejection of out-of-range MTU values.
	tests := []struct {
		name    string
		mtu     int
		wantErr bool
	}{
		{"below min", 67, true},
		{"min valid", 68, false},
		{"standard", 1500, false},
		{"jumbo", 9000, false},
		{"max valid", 16000, false},
		{"above max", 16001, true},
		{"zero", 0, true},
		{"negative", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMTU(tt.mtu)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateMTU(%d) error = %v, wantErr %v", tt.mtu, err, tt.wantErr)
			}
		})
	}
}
