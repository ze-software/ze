package appliance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigDefaults(t *testing.T) {
	cfg := DefaultConfig("lab")

	if cfg.Identity.Name != "lab" {
		t.Errorf("name = %q, want lab", cfg.Identity.Name)
	}
	if cfg.Identity.Hostname != "lab" {
		t.Errorf("hostname = %q, want lab", cfg.Identity.Hostname)
	}
	if cfg.Credentials.Username != "admin" {
		t.Errorf("username = %q, want admin", cfg.Credentials.Username)
	}
	if !cfg.Credentials.AdminEnabled {
		t.Error("admin-enabled should default to true")
	}
	if cfg.SSH.Host != "0.0.0.0" {
		t.Errorf("ssh.host = %q, want 0.0.0.0", cfg.SSH.Host)
	}
	if cfg.SSH.Port != "22" {
		t.Errorf("ssh.port = %q, want 22", cfg.SSH.Port)
	}
	if !cfg.Web.Enabled {
		t.Error("web.enabled should default to true")
	}
	if cfg.Web.Port != "8080" {
		t.Errorf("web.port = %q, want 8080", cfg.Web.Port)
	}
	if cfg.TLS.ValidityYears != 10 {
		t.Errorf("tls.validity-years = %d, want 10", cfg.TLS.ValidityYears)
	}
	if cfg.Image.Arch != "amd64" {
		t.Errorf("image.arch = %q, want amd64", cfg.Image.Arch)
	}
	if cfg.Image.SizeBytes != 2*1024*1024*1024 {
		t.Errorf("image.size-bytes = %d, want 2G", cfg.Image.SizeBytes)
	}
	if cfg.Managed {
		t.Error("managed should default to false")
	}
	if cfg.Device.UpdatePort != 443 {
		t.Errorf("device.update-port = %d, want 443", cfg.Device.UpdatePort)
	}
	if cfg.QEMU.SSHPort != 2222 {
		t.Errorf("qemu.ssh-port = %d, want 2222", cfg.QEMU.SSHPort)
	}
}

func TestConfigMarshalRoundtrip(t *testing.T) {
	original := DefaultConfig("edge-01")
	original.Managed = true
	original.ConfigBase = "../_shared/ze.conf"
	original.Credentials.SSHAuthorizedKeys = []string{"ssh-ed25519 AAAA... test@host"}
	original.Device.Address = "10.0.100.1"

	data, err := json.Marshal(&original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored applianceConfig
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if restored.Identity.Name != original.Identity.Name {
		t.Errorf("name roundtrip: got %q, want %q", restored.Identity.Name, original.Identity.Name)
	}
	if restored.Managed != original.Managed {
		t.Errorf("managed roundtrip: got %v, want %v", restored.Managed, original.Managed)
	}
	if restored.ConfigBase != original.ConfigBase {
		t.Errorf("config-base roundtrip: got %q, want %q", restored.ConfigBase, original.ConfigBase)
	}
	if len(restored.Credentials.SSHAuthorizedKeys) != 1 {
		t.Fatalf("ssh-authorized-keys length: got %d, want 1", len(restored.Credentials.SSHAuthorizedKeys))
	}
	if restored.Credentials.SSHAuthorizedKeys[0] != original.Credentials.SSHAuthorizedKeys[0] {
		t.Errorf("ssh-authorized-keys[0] roundtrip mismatch")
	}
	if restored.Device.Address != "10.0.100.1" {
		t.Errorf("device.address roundtrip: got %q, want 10.0.100.1", restored.Device.Address)
	}
	if restored.TLS.ValidityYears != 10 {
		t.Errorf("tls.validity-years roundtrip: got %d, want 10", restored.TLS.ValidityYears)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*applianceConfig)
		wantErr string
	}{
		{
			name:   "valid defaults",
			modify: func(_ *applianceConfig) {},
		},
		{
			name:    "empty name",
			modify:  func(c *applianceConfig) { c.Identity.Name = "" },
			wantErr: "identity.name is required",
		},
		{
			name:    "name with path traversal",
			modify:  func(c *applianceConfig) { c.Identity.Name = "../evil" },
			wantErr: "identity.name",
		},
		{
			name:    "name with slash",
			modify:  func(c *applianceConfig) { c.Identity.Name = "foo/bar" },
			wantErr: "identity.name",
		},
		{
			name:    "port zero",
			modify:  func(c *applianceConfig) { c.SSH.Port = "0" },
			wantErr: "ssh.port 0: must be 1-65535",
		},
		{
			name:    "port too high",
			modify:  func(c *applianceConfig) { c.SSH.Port = "65536" },
			wantErr: "ssh.port 65536: must be 1-65535",
		},
		{
			name:    "port last valid",
			modify:  func(c *applianceConfig) { c.SSH.Port = "65535" },
			wantErr: "",
		},
		{
			name:    "port not a number",
			modify:  func(c *applianceConfig) { c.SSH.Port = "abc" },
			wantErr: "not a valid port number",
		},
		{
			name:    "image too small",
			modify:  func(c *applianceConfig) { c.Image.SizeBytes = 536870911 },
			wantErr: "minimum is",
		},
		{
			name:    "image at minimum",
			modify:  func(c *applianceConfig) { c.Image.SizeBytes = 512 * 1024 * 1024 },
			wantErr: "",
		},
		{
			name:    "validity zero",
			modify:  func(c *applianceConfig) { c.TLS.ValidityYears = 0 },
			wantErr: "minimum is 1",
		},
		{
			name:    "validity too high",
			modify:  func(c *applianceConfig) { c.TLS.ValidityYears = 26 },
			wantErr: "maximum is 25",
		},
		{
			name:    "bad arch",
			modify:  func(c *applianceConfig) { c.Image.Arch = "mips" },
			wantErr: "must be amd64 or arm64",
		},
		{
			name:   "arm64 valid",
			modify: func(c *applianceConfig) { c.Image.Arch = "arm64" },
		},
		{
			name:   "kernel profile qemu valid",
			modify: func(c *applianceConfig) { c.Image.KernelProfile = defaultKernelProfile },
		},
		{
			name:   "kernel profile hardware valid",
			modify: func(c *applianceConfig) { c.Image.KernelProfile = "hardware" },
		},
		{
			name:   "kernel profile hardware-kms valid",
			modify: func(c *applianceConfig) { c.Image.KernelProfile = hardwareKMSProfile },
		},
		{
			name:   "kernel profile empty valid",
			modify: func(c *applianceConfig) { c.Image.KernelProfile = "" },
		},
		{
			name:   "kernel profile custom token valid",
			modify: func(c *applianceConfig) { c.Image.KernelProfile = "bare-metal" },
		},
		{
			name:    "kernel profile invalid",
			modify:  func(c *applianceConfig) { c.Image.KernelProfile = "../bad" },
			wantErr: "must match",
		},
		{
			name:    "qemu port below 1024",
			modify:  func(c *applianceConfig) { c.QEMU.SSHPort = 1023 },
			wantErr: "must be 1024-65535",
		},
		{
			name:   "name at max length",
			modify: func(c *applianceConfig) { c.Identity.Name = strings.Repeat("a", maxNameLen) },
		},
		{
			name:    "name exceeds max length",
			modify:  func(c *applianceConfig) { c.Identity.Name = strings.Repeat("a", maxNameLen+1) },
			wantErr: "maximum length",
		},
		// config-base path validation deferred to assemble time (needs resolved appliance dir)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig("test")
			tt.modify(&cfg)
			err := cfg.Validate()

			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if got := err.Error(); !contains(got, tt.wantErr) {
				t.Errorf("error = %q, want containing %q", got, tt.wantErr)
			}
		})
	}
}

// TestParseByteSize verifies the byte-size string parser: unit multipliers
// (1024-based), case-insensitivity, required unit suffix, and rejection of bad
// input.
func TestParseByteSize(t *testing.T) {
	ok := []struct {
		in   string
		want int64
	}{
		{"10b", 10},
		{"1kb", 1024},
		{"2mb", 2 * 1024 * 1024},
		{"1gb", 1024 * 1024 * 1024},
		{"1tb", 1024 * 1024 * 1024 * 1024},
		{"8gb", 8 * 1024 * 1024 * 1024},
		{"1GB", 1024 * 1024 * 1024}, // case-insensitive
		{"  512mb  ", 512 * 1024 * 1024},
		{"0b", 0},
	}
	for _, tc := range ok {
		got, err := parseByteSize(tc.in)
		if err != nil {
			t.Errorf("parseByteSize(%q): unexpected error %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseByteSize(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}

	bad := []string{"10", "8g", "abc", "-5mb", "", "mb", "1.5gb"}
	for _, in := range bad {
		if _, err := parseByteSize(in); err == nil {
			t.Errorf("parseByteSize(%q): expected an error", in)
		}
	}
}

// TestHugepageConfigValidate covers AC-4/AC-7: image.hugepages ({size,page-size}
// byte-size strings) and image.memory (byte-size string) bounds, including the
// 512 GiB ceiling and the 50%-of-memory cap.
func TestHugepageConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*applianceConfig)
		wantErr string
	}{
		{
			name:   "no hugepages (default) valid",
			modify: func(_ *applianceConfig) {},
		},
		{
			name:   "1gb of 2mb pages valid",
			modify: func(c *applianceConfig) { c.Image.Hugepages = &Hugepages{Size: "1gb", PageSize: "2mb"} },
		},
		{
			name:   "one 2mb page valid",
			modify: func(c *applianceConfig) { c.Image.Hugepages = &Hugepages{Size: "2mb", PageSize: "2mb"} },
		},
		{
			name:    "size below one page rejected",
			modify:  func(c *applianceConfig) { c.Image.Hugepages = &Hugepages{Size: "1mb", PageSize: "2mb"} },
			wantErr: "at least one",
		},
		{
			name:    "size not a whole multiple of page-size rejected",
			modify:  func(c *applianceConfig) { c.Image.Hugepages = &Hugepages{Size: "3mb", PageSize: "2mb"} },
			wantErr: "whole multiple",
		},
		{
			name:    "bad page-size rejected",
			modify:  func(c *applianceConfig) { c.Image.Hugepages = &Hugepages{Size: "1gb", PageSize: "4mb"} },
			wantErr: "must be 2mb or 1gb",
		},
		{
			name:    "size missing unit rejected",
			modify:  func(c *applianceConfig) { c.Image.Hugepages = &Hugepages{Size: "1g", PageSize: "2mb"} },
			wantErr: "must end in b, kb, mb, gb, or tb",
		},
		{
			name:   "2mb pages at the 512 GiB ceiling valid",
			modify: func(c *applianceConfig) { c.Image.Hugepages = &Hugepages{Size: "512gb", PageSize: "2mb"} },
		},
		{
			name:    "1gb pages one past the ceiling rejected",
			modify:  func(c *applianceConfig) { c.Image.Hugepages = &Hugepages{Size: "513gb", PageSize: "1gb"} },
			wantErr: "512 GiB",
		},
		{
			name: "reservation at exactly 50% of memory valid",
			modify: func(c *applianceConfig) {
				c.Image.Memory = "8gb"
				c.Image.Hugepages = &Hugepages{Size: "4gb", PageSize: "2mb"} // 4gb == 50%
			},
		},
		{
			name: "reservation over 50% of memory rejected",
			modify: func(c *applianceConfig) {
				c.Image.Memory = "8gb"
				c.Image.Hugepages = &Hugepages{Size: "6gb", PageSize: "2mb"} // > 4gb
			},
			wantErr: "50% of image.memory",
		},
		{
			name:    "memory below minimum rejected",
			modify:  func(c *applianceConfig) { c.Image.Memory = "1mb" },
			wantErr: "minimum is 256mb",
		},
		{
			name:   "memory at minimum valid",
			modify: func(c *applianceConfig) { c.Image.Memory = "256mb" },
		},
		{
			name:   "memory at maximum valid",
			modify: func(c *applianceConfig) { c.Image.Memory = "1tb" },
		},
		{
			name:    "memory above maximum rejected",
			modify:  func(c *applianceConfig) { c.Image.Memory = "2tb" },
			wantErr: "maximum is 1tb",
		},
		{
			name:    "memory missing unit rejected",
			modify:  func(c *applianceConfig) { c.Image.Memory = "8g" },
			wantErr: "must end in b, kb, mb, gb, or tb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig("test")
			tt.modify(&cfg)
			err := cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if got := err.Error(); !contains(got, tt.wantErr) {
				t.Errorf("error = %q, want containing %q", got, tt.wantErr)
			}
		})
	}
}

func TestLoadSaveRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "appliance.json")

	original := DefaultConfig("roundtrip")
	original.Managed = true

	if err := saveConfig(path, &original); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if loaded.Identity.Name != "roundtrip" {
		t.Errorf("name = %q, want roundtrip", loaded.Identity.Name)
	}
	if !loaded.Managed {
		t.Error("managed should be true after roundtrip")
	}
}

func TestLoadConfigMissing(t *testing.T) {
	_, err := LoadConfig("/nonexistent/appliance.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadConfigInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{invalid"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadConfigRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "appliance.json")
	data := []byte(`{"identity":{"name":"test","hostname":"test"},"kernel-profile":"hardware"}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for unknown top-level field kernel-profile")
	}
	if !strings.Contains(err.Error(), "kernel-profile") {
		t.Errorf("error should mention the unknown field, got: %v", err)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
