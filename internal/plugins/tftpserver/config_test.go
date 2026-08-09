// Design: docs/architecture/provisioning/tftp-server.md -- TFTP server config tests

package tftpserver

import (
	"testing"
)

func TestTFTPConfigParse(t *testing.T) {
	t.Parallel()

	data := `{
		"service": {
			"tftp-server": {
				"enabled": "true",
				"listen-interface": ["eth0"],
				"root-directory": "/var/lib/tftp",
				"max-transfers": "20"
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
	if cfg.RootDirectory != "/var/lib/tftp" {
		t.Errorf("root-directory = %q", cfg.RootDirectory)
	}
	if cfg.MaxTransfers != 20 {
		t.Errorf("max-transfers = %d, want 20", cfg.MaxTransfers)
	}
}

func TestTFTPConfigParseDefaults(t *testing.T) {
	t.Parallel()

	data := `{"service": {"tftp-server": {"enabled": "true", "root-directory": "/tmp/tftp"}}}`
	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.MaxTransfers != defaultMaxTransfers {
		t.Errorf("max-transfers = %d, want default %d", cfg.MaxTransfers, defaultMaxTransfers)
	}
}

func TestTFTPConfigParseMissing(t *testing.T) {
	t.Parallel()

	data := `{"service": {}}`
	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.Enabled {
		t.Error("expected disabled when no tftp-server block")
	}
}

func TestTFTPConfigVerify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     tftpConfig
		wantErr bool
	}{
		{"disabled no root", tftpConfig{Enabled: false}, false},
		{"enabled with root", tftpConfig{Enabled: true, RootDirectory: "/tmp"}, false},
		{"enabled no root", tftpConfig{Enabled: true}, true},
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

func TestTFTPConfigParseInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data string
	}{
		{
			"max-transfers below minimum",
			`{"service":{"tftp-server":{"max-transfers":"0"}}}`,
		},
		{
			"max-transfers above maximum",
			`{"service":{"tftp-server":{"max-transfers":"1001"}}}`,
		},
		{
			"max-transfers not a number",
			`{"service":{"tftp-server":{"max-transfers":"abc"}}}`,
		},
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
