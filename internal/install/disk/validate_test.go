// VALIDATES: AC-12 (malformed cmdline inputs rejected with shell-parity validation)
// PREVENTS: invalid input reaching dd, URLs, or filesystem paths

package disk

import (
	"strings"
	"testing"
)

func TestValidateIPv4(t *testing.T) {
	valid := []string{"192.168.1.1", "10.0.0.1", "255.255.255.255", "0.0.0.0", "1.2.3.4"}
	for _, ip := range valid {
		if err := validateIPv4(ip); err != nil {
			t.Errorf("validateIPv4(%q) = %v, want nil", ip, err)
		}
	}

	invalid := []string{
		"", "abc", "1.2.3", "1.2.3.4.5",
		"256.1.1.1", "1.256.1.1",
		"1.2.3.4a", "08.8.8.8", "1.2.3.00",
		"1.2.3.-1", " 1.2.3.4", "1.2.3.4 ",
	}
	for _, ip := range invalid {
		if err := validateIPv4(ip); err == nil {
			t.Errorf("validateIPv4(%q) = nil, want error", ip)
		}
	}
}

func TestValidatePort(t *testing.T) {
	valid := []string{"1", "80", "443", "8080", "65535"}
	for _, p := range valid {
		if err := validatePort(p); err != nil {
			t.Errorf("validatePort(%q) = %v, want nil", p, err)
		}
	}

	invalid := []string{"", "0", "65536", "-1", "abc", "80.5", "08"}
	for _, p := range invalid {
		if err := validatePort(p); err == nil {
			t.Errorf("validatePort(%q) = nil, want error", p)
		}
	}
}

func TestValidateImageName(t *testing.T) {
	valid := []string{"ze.img", "ze-2024.img", "test_image.gz", "A.B"}
	for _, n := range valid {
		if err := validateImageName(n); err != nil {
			t.Errorf("validateImageName(%q) = %v, want nil", n, err)
		}
	}

	tooLong := strings.Repeat("a", 256)
	if err := validateImageName(tooLong); err == nil {
		t.Errorf("validateImageName(256 chars) = nil, want error")
	}
	atLimit := strings.Repeat("a", 255)
	if err := validateImageName(atLimit); err != nil {
		t.Errorf("validateImageName(255 chars) = %v, want nil", err)
	}

	invalid := []string{"", ".", "..", "../etc/passwd", "foo bar", "img\nname", "img;rm", "img/sub"}
	for _, n := range invalid {
		if err := validateImageName(n); err == nil {
			t.Errorf("validateImageName(%q) = nil, want error", n)
		}
	}
}

func TestValidateMediaID(t *testing.T) {
	valid := "0123456789abcdef0123456789abcdef"
	if err := validateMediaID(valid); err != nil {
		t.Errorf("validateMediaID(%q) = %v, want nil", valid, err)
	}

	invalid := []string{
		"", "short",
		"0123456789abcdef0123456789abcde",   // 31 chars
		"0123456789abcdef0123456789abcdef0", // 33 chars
		"0123456789ABCDEF0123456789abcdef",  // uppercase
		"0123456789abcdef0123456789abcdeg",  // 'g'
	}
	for _, id := range invalid {
		if err := validateMediaID(id); err == nil {
			t.Errorf("validateMediaID(%q) = nil, want error", id)
		}
	}
}

func TestValidateSHA256(t *testing.T) {
	valid := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if err := validateSHA256(valid); err != nil {
		t.Errorf("validateSHA256(%q) = %v, want nil", valid, err)
	}

	tooShort := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b85"
	if err := validateSHA256(tooShort); err == nil {
		t.Errorf("validateSHA256(63 chars) = nil, want error")
	}

	tooLong := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b8555"
	if err := validateSHA256(tooLong); err == nil {
		t.Errorf("validateSHA256(65 chars) = nil, want error")
	}
}

func TestValidateTargetPath(t *testing.T) {
	valid := []string{
		"/dev/sda", "/dev/vda", "/dev/xvda", "/dev/hda",
		"/dev/nvme0n1", "/dev/nvme1n2",
		"/dev/mmcblk0",
	}
	for _, p := range valid {
		if err := validateTargetPath(p); err != nil {
			t.Errorf("validateTargetPath(%q) = %v, want nil", p, err)
		}
	}

	invalid := []string{
		"", "/tmp/sda", "sda",
		"/dev/sda1",   // partition, not whole disk
		"/dev/loop0",  // virtual
		"/dev/sr0",    // optical
		"/dev/../etc", // traversal
		"/dev/dm-0",
	}
	for _, p := range invalid {
		if err := validateTargetPath(p); err == nil {
			t.Errorf("validateTargetPath(%q) = nil, want error", p)
		}
	}
}

func TestValidateSource(t *testing.T) {
	if err := validateSource("http"); err != nil {
		t.Errorf("validateSource(http) = %v", err)
	}
	if err := validateSource("iso"); err != nil {
		t.Errorf("validateSource(iso) = %v", err)
	}
	if err := validateSource("ftp"); err == nil {
		t.Error("validateSource(ftp) = nil, want error")
	}
}

func TestParseCmdline(t *testing.T) {
	line := "console=ttyS0 ze.source=iso ze.server=10.0.0.1 ze.image=test.img ze.port=8080 ze.target=/dev/sda ze.wait=10 ze.media-id=0123456789abcdef0123456789abcdef"
	cfg := parseCmdlineString(line)

	if cfg.Source != "iso" {
		t.Errorf("Source = %q, want iso", cfg.Source)
	}
	if cfg.Server != "10.0.0.1" {
		t.Errorf("Server = %q, want 10.0.0.1", cfg.Server)
	}
	if cfg.Image != "test.img" {
		t.Errorf("Image = %q, want test.img", cfg.Image)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want 8080", cfg.Port)
	}
	if cfg.Target != "/dev/sda" {
		t.Errorf("Target = %q, want /dev/sda", cfg.Target)
	}
	if cfg.Wait != "10" {
		t.Errorf("Wait = %q, want 10", cfg.Wait)
	}
	if cfg.MediaID != "0123456789abcdef0123456789abcdef" {
		t.Errorf("MediaID = %q, want 0123456789abcdef0123456789abcdef", cfg.MediaID)
	}
}

func TestParseCmdlineDefaults(t *testing.T) {
	cfg := parseCmdlineString("console=ttyS0 root=/dev/sda1")

	if cfg.Source != "http" {
		t.Errorf("Source default = %q, want http", cfg.Source)
	}
	if cfg.Image != "ze.img" {
		t.Errorf("Image default = %q, want ze.img", cfg.Image)
	}
	if cfg.Port != "80" {
		t.Errorf("Port default = %q, want 80", cfg.Port)
	}
	if cfg.Wait != "30" {
		t.Errorf("Wait default = %q, want 30", cfg.Wait)
	}
}

func TestPortBoundary(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"1", true},
		{"65535", true},
		{"0", false},
		{"65536", false},
	}
	for _, tt := range tests {
		err := validatePort(tt.input)
		if tt.valid && err != nil {
			t.Errorf("validatePort(%q) = %v, want nil", tt.input, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("validatePort(%q) = nil, want error", tt.input)
		}
	}
}

func TestMediaIDBoundary(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{strings.Repeat("a", 32), true},
		{strings.Repeat("a", 31), false},
		{strings.Repeat("a", 33), false},
	}
	for _, tt := range tests {
		err := validateMediaID(tt.input)
		if tt.valid && err != nil {
			t.Errorf("validateMediaID(%d chars) = %v, want nil", len(tt.input), err)
		}
		if !tt.valid && err == nil {
			t.Errorf("validateMediaID(%d chars) = nil, want error", len(tt.input))
		}
	}
}

func TestSHA256Boundary(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{strings.Repeat("a", 64), true},
		{strings.Repeat("a", 63), false},
		{strings.Repeat("a", 65), false},
	}
	for _, tt := range tests {
		err := validateSHA256(tt.input)
		if tt.valid && err != nil {
			t.Errorf("validateSHA256(%d chars) = %v, want nil", len(tt.input), err)
		}
		if !tt.valid && err == nil {
			t.Errorf("validateSHA256(%d chars) = nil, want error", len(tt.input))
		}
	}
}

// validateShellAuth was deleted, not weakened. It checked a 64-char
// lowercase-hex sha256; the credential is now "<saltHex>:<digestHex>" behind
// argon2id. Shape coverage (including the uppercase rejection this test carried,
// and the legacy bare-sha256 form) lives in internal/core/rescueauth tests; this
// keeps the installer-side wrapper covered.
func TestValidateRescueAuthWrapsShapeErrors(t *testing.T) {
	valid := "aabbccddeeff00112233445566778899:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if err := validateRescueAuth(valid); err != nil {
		t.Errorf("validateRescueAuth(%q) = %v, want nil", valid, err)
	}
	for _, bad := range []string{
		"",
		strings.ToUpper(valid),
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"aabbccddeeff00112233445566778899:short",
	} {
		if err := validateRescueAuth(bad); err == nil {
			t.Errorf("validateRescueAuth(%q) = nil, want an error", bad)
		}
	}
}
