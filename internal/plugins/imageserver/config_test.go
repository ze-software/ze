// Design: docs/architecture/provisioning/image-server.md -- image server config tests

package imageserver

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func testBcryptHash(t *testing.T) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(hash)
}

func TestParseImageConfig(t *testing.T) {
	t.Parallel()

	data := `{
		"service": {
			"image-server": {
				"enabled": "true",
				"listen-interface": ["eth0"],
				"listen-port": "8080",
				"image-directory": "/var/lib/images",
				"boot-directory": "/var/lib/boot",
				"ssh-username": "admin",
				"ssh-password-hash": "$2a$10$abcdefghijklmnopqrstuv",
				"rescue-auth": "aabbccddeeff00112233445566778899:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
			}
		}
	}`

	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if !cfg.Enabled {
		t.Error("expected enabled=true")
	}
	if len(cfg.ListenInterfaces) != 1 || cfg.ListenInterfaces[0] != "eth0" {
		t.Errorf("listen-interfaces = %v", cfg.ListenInterfaces)
	}
	if cfg.ListenPort != 8080 {
		t.Errorf("listen-port = %d, want 8080", cfg.ListenPort)
	}
	if cfg.ImageDirectory != "/var/lib/images" {
		t.Errorf("image-directory = %q", cfg.ImageDirectory)
	}
	if cfg.BootDirectory != "/var/lib/boot" {
		t.Errorf("boot-directory = %q", cfg.BootDirectory)
	}
	if cfg.SSHUsername != "admin" {
		t.Errorf("ssh-username = %q", cfg.SSHUsername)
	}
	if cfg.SSHPasswordHash != "$2a$10$abcdefghijklmnopqrstuv" {
		t.Errorf("ssh-password-hash = %q", cfg.SSHPasswordHash)
	}
	if cfg.RescueAuth != "aabbccddeeff00112233445566778899:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" {
		t.Errorf("rescue-auth = %q", cfg.RescueAuth)
	}
}

func TestParseImageConfigDefaults(t *testing.T) {
	t.Parallel()

	data := `{"service": {"image-server": {"enabled": "true", "image-directory": "/tmp/img"}}}`
	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.ListenPort != defaultListenPort {
		t.Errorf("listen-port = %d, want default %d", cfg.ListenPort, defaultListenPort)
	}
}

func TestParseImageConfigDisabled(t *testing.T) {
	t.Parallel()

	data := `{"service": {"image-server": {}}}`
	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.Enabled {
		t.Error("expected disabled")
	}
}

func TestVerifyImageConfig(t *testing.T) {
	t.Parallel()

	hash := testBcryptHash(t)
	tests := []struct {
		name    string
		cfg     imageConfig
		wantErr bool
	}{
		{"disabled", imageConfig{Enabled: false}, false},
		{"enabled with image-dir", imageConfig{Enabled: true, ImageDirectory: "/tmp"}, false},
		{"enabled with boot-dir", imageConfig{Enabled: true, BootDirectory: "/tmp"}, false},
		{"enabled no dirs", imageConfig{Enabled: true}, true},
		{"both ssh creds", imageConfig{Enabled: true, ImageDirectory: "/tmp", SSHUsername: "admin", SSHPasswordHash: hash}, false},
		{"ssh username only", imageConfig{Enabled: true, ImageDirectory: "/tmp", SSHUsername: "admin"}, true},
		{"ssh hash only", imageConfig{Enabled: true, ImageDirectory: "/tmp", SSHPasswordHash: hash}, true},
		{"plaintext ssh hash", imageConfig{Enabled: true, ImageDirectory: "/tmp", SSHUsername: "admin", SSHPasswordHash: "secret"}, true},
		{"malformed ssh hash", imageConfig{Enabled: true, ImageDirectory: "/tmp", SSHUsername: "admin", SSHPasswordHash: "$2y$.."}, true},
		{"valid rescue-auth", imageConfig{Enabled: true, ImageDirectory: "/tmp", RescueAuth: "aabbccddeeff00112233445566778899:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}, false},
		{"rescue-auth too short", imageConfig{Enabled: true, ImageDirectory: "/tmp", RescueAuth: "abc123"}, true},
		{"rescue-auth legacy bare sha256", imageConfig{Enabled: true, ImageDirectory: "/tmp", RescueAuth: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}, true},
		{"rescue-auth non-hex", imageConfig{Enabled: true, ImageDirectory: "/tmp", RescueAuth: "aabbccddeeff00112233445566778899:z123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde"}, true},
		{"rescue-auth uppercase", imageConfig{Enabled: true, ImageDirectory: "/tmp", RescueAuth: "AABBCCDDEEFF00112233445566778899:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := verifyConfig(tc.cfg)
			if tc.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseImageConfigInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data string
	}{
		{"port below min", `{"service":{"image-server":{"listen-port":"0"}}}`},
		{"port above max", `{"service":{"image-server":{"listen-port":"65536"}}}`},
		{"port not number", `{"service":{"image-server":{"listen-port":"abc"}}}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseConfig(tc.data)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}
