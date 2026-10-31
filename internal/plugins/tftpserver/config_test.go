// Design: docs/architecture/provisioning/tftp-server.md -- TFTP server config tests

package tftpserver

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	tftpyang "github.com/ze-software/ze/internal/plugins/tftpserver/yang"
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

// TestParseConfigWindowAndTimeout reads max-windowsize and timeout: their
// defaults when the leaves are absent, the RFC bounds, and the refusal of a
// value outside them with the leaf and the fix in the error.
// VALIDATES: AC-21, AC-22 and AC-23 at the plugin's own parse, the second check
// of the pair beside the YANG range.
// PREVENTS: an out-of-range value reaching the server as a zero window or a
// zero timeout.
func TestParseConfigWindowAndTimeout(t *testing.T) {
	t.Parallel()

	block := func(leaves string) string {
		return `{"service":{"tftp-server":{"enabled":"true","root-directory":"/tmp"` + leaves + `}}}`
	}

	cfg, err := parseConfig(block(""))
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.MaxWindowsize != defaultMaxWindowsize {
		t.Errorf("max-windowsize = %d, want the default %d", cfg.MaxWindowsize, defaultMaxWindowsize)
	}
	if cfg.Timeout != defaultTimeoutSeconds {
		t.Errorf("timeout = %d, want the default %d", cfg.Timeout, defaultTimeoutSeconds)
	}

	for _, tc := range []struct {
		leaves          string
		window, timeout int
	}{
		{`,"max-windowsize":"1","timeout":"1"`, 1, 1},
		{`,"max-windowsize":"65535","timeout":"255"`, 65535, 255},
		{`,"max-windowsize":"8","timeout":"2"`, 8, 2},
	} {
		cfg, err := parseConfig(block(tc.leaves))
		if err != nil {
			t.Fatalf("parseConfig(%s): %v", tc.leaves, err)
		}
		if cfg.MaxWindowsize != tc.window || cfg.Timeout != tc.timeout {
			t.Errorf("parseConfig(%s) = window %d timeout %d, want %d and %d",
				tc.leaves, cfg.MaxWindowsize, cfg.Timeout, tc.window, tc.timeout)
		}
	}

	for _, tc := range []struct{ leaves, leaf string }{
		{`,"max-windowsize":"0"`, "max-windowsize"},
		{`,"max-windowsize":"65536"`, "max-windowsize"},
		{`,"max-windowsize":"many"`, "max-windowsize"},
		{`,"timeout":"0"`, "timeout"},
		{`,"timeout":"256"`, "timeout"},
	} {
		_, err := parseConfig(block(tc.leaves))
		if err == nil {
			t.Errorf("parseConfig(%s) accepted the value", tc.leaves)
			continue
		}
		if !strings.Contains(err.Error(), "Fix: set service tftp-server "+tc.leaf+" to a number from") {
			t.Errorf("parseConfig(%s) error names no fix: %v", tc.leaves, err)
		}
	}
}

// TestTFTPConfigDefaultsMatchYANG pins the Go defaults parseConfig and
// ServeConn use to the defaults the YANG module declares.
// VALIDATES: the defaults are declared once, in YANG, and the Go copy agrees.
// PREVENTS: a YANG default changed without the server following it.
func TestTFTPConfigDefaultsMatchYANG(t *testing.T) {
	t.Parallel()

	for leaf, want := range map[string]int{
		"max-transfers":  defaultMaxTransfers,
		"max-windowsize": defaultMaxWindowsize,
		"timeout":        defaultTimeoutSeconds,
	} {
		match := regexp.MustCompile(`leaf ` + leaf + ` \{[^}]*\}[^}]*?default (\d+);`).FindStringSubmatch(tftpyang.ZeTFTPServerConfYANG)
		if match == nil {
			t.Fatalf("leaf %s carries no default in the YANG module", leaf)
		}
		if match[1] != strconv.Itoa(want) {
			t.Errorf("leaf %s: YANG default %s, Go default %d", leaf, match[1], want)
		}
	}
}
