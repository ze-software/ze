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
			err := ValidateIfaceName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateIfaceName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
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

func TestSetAdminUpInvalidName(t *testing.T) {
	// VALIDATES: SetAdminUp rejects invalid interface names.
	// PREVENTS: Invalid names passed to netlink.
	if err := SetAdminUp(""); err == nil {
		t.Error("expected error for empty name, got nil")
	}
	if err := SetAdminUp("abcdefghijklmnop"); err == nil {
		t.Error("expected error for too-long name, got nil")
	}
}

func TestSetAdminDownInvalidName(t *testing.T) {
	// VALIDATES: SetAdminDown rejects invalid interface names.
	// PREVENTS: Invalid names passed to netlink.
	if err := SetAdminDown(""); err == nil {
		t.Error("expected error for empty name, got nil")
	}
	if err := SetAdminDown("abcdefghijklmnop"); err == nil {
		t.Error("expected error for too-long name, got nil")
	}
}

func TestSetMACAddressValidation(t *testing.T) {
	// VALIDATES: SetMACAddress rejects invalid names and bad MAC formats.
	// PREVENTS: Invalid MAC addresses reaching netlink.
	tests := []struct {
		name    string
		iface   string
		mac     string
		wantErr bool
	}{
		{"empty iface", "", "00:11:22:33:44:55", true},
		{"invalid mac", "eth0", "not-a-mac", true},
		{"too short mac", "eth0", "00:11:22", true},
		{"valid but iface not found", "eth0", "00:11:22:33:44:55", true}, // no such device
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SetMACAddress(tt.iface, tt.mac)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetMACAddress(%q, %q) error = %v, wantErr %v",
					tt.iface, tt.mac, err, tt.wantErr)
			}
		})
	}
}

func TestGetMACAddressInvalidName(t *testing.T) {
	// VALIDATES: GetMACAddress rejects invalid interface names.
	// PREVENTS: Invalid names passed to netlink.
	if _, err := GetMACAddress(""); err == nil {
		t.Error("expected error for empty name, got nil")
	}
}

func TestGetStatsInvalidName(t *testing.T) {
	// VALIDATES: GetStats rejects invalid interface names.
	// PREVENTS: Invalid names passed to netlink.
	if _, err := GetStats(""); err == nil {
		t.Error("expected error for empty name, got nil")
	}
}

func TestBridgeAddPortInvalidNames(t *testing.T) {
	// VALIDATES: BridgeAddPort rejects invalid bridge and port names.
	// PREVENTS: Invalid names reaching netlink.
	if err := BridgeAddPort("", "eth0"); err == nil {
		t.Error("expected error for empty bridge name, got nil")
	}
	if err := BridgeAddPort("br0", ""); err == nil {
		t.Error("expected error for empty port name, got nil")
	}
}

func TestBridgeDelPortInvalidName(t *testing.T) {
	// VALIDATES: BridgeDelPort rejects invalid port names.
	// PREVENTS: Invalid names reaching netlink.
	if err := BridgeDelPort(""); err == nil {
		t.Error("expected error for empty name, got nil")
	}
}

func TestBridgeSetSTPInvalidName(t *testing.T) {
	// VALIDATES: BridgeSetSTP rejects invalid bridge names.
	// PREVENTS: Invalid names reaching sysfs writes.
	if err := BridgeSetSTP("", true); err == nil {
		t.Error("expected error for empty name, got nil")
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
