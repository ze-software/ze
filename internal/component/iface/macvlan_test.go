// Design: docs/features/interfaces.md -- naming + spec-validation boundary tests

package iface

import (
	"strings"
	"testing"
)

// TestComposeOwnedDeviceName_Boundaries exercises the name-budget math and the
// reject-not-truncate rule.
//
// VALIDATES: AC-6 (a composed name over 15 chars is rejected naming the limit,
// NO truncation) plus the Boundary Tests table (last-valid 15, ifindex 0
// reject, negative id reject).
// PREVENTS: a silently-truncated device name colliding with another device, or
// an over-budget name reaching the kernel.
func TestComposeOwnedDeviceName_Boundaries(t *testing.T) {
	tests := []struct {
		name          string
		prefix        string
		parentIfindex int
		id            int
		want          string
		wantErr       bool
		errContains   string
	}{
		{name: "typical", prefix: "zv4", parentIfindex: 42, id: 10, want: "zv4-42-10"},
		{name: "last-valid-15", prefix: "zv4", parentIfindex: 9999999, id: 255, want: "zv4-9999999-255"},
		{name: "over-budget-16-rejects", prefix: "zv4", parentIfindex: 9999999, id: 2550, wantErr: true, errContains: "15-char limit"},
		{name: "over-budget-names-candidate", prefix: "zv6", parentIfindex: 10000000, id: 255, wantErr: true, errContains: "zv6-10000000-255"},
		{name: "empty-prefix-rejects", prefix: "", parentIfindex: 42, id: 10, wantErr: true, errContains: "prefix is empty"},
		{name: "ifindex-zero-rejects", prefix: "zv4", parentIfindex: 0, id: 10, wantErr: true, errContains: "must be positive"},
		{name: "ifindex-negative-rejects", prefix: "zv4", parentIfindex: -1, id: 10, wantErr: true, errContains: "must be positive"},
		{name: "id-negative-rejects", prefix: "zv4", parentIfindex: 42, id: -1, wantErr: true, errContains: "must be non-negative"},
		{name: "id-max-in-budget", prefix: "zv4", parentIfindex: 42, id: 999, want: "zv4-42-999"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ComposeOwnedDeviceName(tt.prefix, tt.parentIfindex, tt.id)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ComposeOwnedDeviceName(%q,%d,%d) = %q, want error", tt.prefix, tt.parentIfindex, tt.id, got)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				if got != "" {
					t.Errorf("on error, name should be empty, got %q (no truncation)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ComposeOwnedDeviceName(%q,%d,%d) unexpected error: %v", tt.prefix, tt.parentIfindex, tt.id, err)
			}
			if got != tt.want {
				t.Errorf("ComposeOwnedDeviceName(%q,%d,%d) = %q, want %q", tt.prefix, tt.parentIfindex, tt.id, got, tt.want)
			}
			if len(got) > maxIfaceNameLen {
				t.Errorf("composed name %q exceeds %d chars", got, maxIfaceNameLen)
			}
		})
	}
}

// TestMacvlanSpecValidate exercises MacvlanSpec.validate: name via
// ValidateIfaceName, non-empty parent, and a MAC that parses to a non-zero
// unicast address.
//
// VALIDATES: the Boundary Tests MAC rows (valid unicast, all-zero reject,
// multicast reject) plus name/parent validation.
// PREVENTS: an invalid device reaching the kernel (bad name, missing parent,
// broadcast/multicast/zero MAC).
func TestMacvlanSpecValidate(t *testing.T) {
	tests := []struct {
		name        string
		spec        MacvlanSpec
		wantErr     bool
		errContains string
	}{
		{name: "valid", spec: MacvlanSpec{Name: "zv4-42-10", Parent: "eth0", MAC: "00:00:5e:00:01:ff"}},
		{name: "valid-ipv6-vmac", spec: MacvlanSpec{Name: "zv6-42-10", Parent: "eth0", MAC: "00:00:5e:00:02:0a"}},
		{name: "empty-name", spec: MacvlanSpec{Name: "", Parent: "eth0", MAC: "00:00:5e:00:01:ff"}, wantErr: true},
		{name: "name-too-long", spec: MacvlanSpec{Name: "abcdefghijklmnop", Parent: "eth0", MAC: "00:00:5e:00:01:ff"}, wantErr: true},
		{name: "empty-parent", spec: MacvlanSpec{Name: "zv4-42-10", Parent: "", MAC: "00:00:5e:00:01:ff"}, wantErr: true, errContains: "parent is empty"},
		{name: "zero-mac", spec: MacvlanSpec{Name: "zv4-42-10", Parent: "eth0", MAC: "00:00:00:00:00:00"}, wantErr: true, errContains: "all-zero"},
		{name: "multicast-mac", spec: MacvlanSpec{Name: "zv4-42-10", Parent: "eth0", MAC: "01:00:5e:00:01:ff"}, wantErr: true, errContains: "multicast"},
		{name: "unparseable-mac", spec: MacvlanSpec{Name: "zv4-42-10", Parent: "eth0", MAC: "not-a-mac"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.validate()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("validate(%+v) = nil, want error", tt.spec)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("validate(%+v) unexpected error: %v", tt.spec, err)
			}
		})
	}
}

// TestMacEqual confirms MAC comparison tolerates format differences and
// distinguishes distinct addresses (used by the drift check).
func TestMacEqual(t *testing.T) {
	if !macEqual("00:00:5e:00:01:0a", "00:00:5E:00:01:0A") {
		t.Error("macEqual should ignore case")
	}
	if macEqual("00:00:5e:00:01:0a", "00:00:5e:00:01:0b") {
		t.Error("macEqual should distinguish different MACs")
	}
}
