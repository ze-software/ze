package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDiscoverExaBGPSuiteAssignsNumericIDsInDisplayOrder(t *testing.T) {
	base := t.TempDir()
	writeExaBGPFixture(t, base, "zeta", "conf-zeta.conf", 1)
	writeExaBGPFixture(t, base, "alpha", "conf-alpha.conf", 3)

	suite, err := discoverExaBGPSuite(base)
	if err != nil {
		t.Fatalf("discover ExaBGP suite: %v", err)
	}

	registered := suite.tests.Registered()
	if len(registered) != 2 {
		t.Fatalf("registered tests = %d, want 2", len(registered))
	}
	if registered[0].Nick != "1" || registered[0].Name != "alpha" {
		t.Fatalf("first test = %s %s, want 1 alpha", registered[0].Nick, registered[0].Name)
	}
	if registered[1].Nick != "2" || registered[1].Name != "zeta" {
		t.Fatalf("second test = %s %s, want 2 zeta", registered[1].Nick, registered[1].Name)
	}

	alpha := suite.byNick["1"]
	if alpha == nil {
		t.Fatal("missing metadata for nick 1")
	}
	if alpha.tcpConnections != 3 {
		t.Fatalf("tcpConnections = %d, want 3", alpha.tcpConnections)
	}
	wantConfig := filepath.Join(base, "test", "exabgp-compat", "etc", "conf-alpha.conf")
	if len(alpha.configs) != 1 || alpha.configs[0] != wantConfig {
		t.Fatalf("configs = %#v, want %s", alpha.configs, wantConfig)
	}
}

func TestDiscoverExaBGPSuiteParsesSerialOption(t *testing.T) {
	base := t.TempDir()
	writeExaBGPFixture(t, base, "watchdog", "conf-watchdog.conf", 1, "option=serial")

	suite, err := discoverExaBGPSuite(base)
	if err != nil {
		t.Fatalf("discover ExaBGP suite: %v", err)
	}

	test := suite.byNick["1"]
	if test == nil {
		t.Fatal("missing metadata for nick 1")
	}
	if !test.serial {
		t.Fatal("serial option was not parsed")
	}
}

func TestParseExaBGPCIRejectsMissingConfig(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "test", "exabgp-compat")
	if err := os.MkdirAll(filepath.Join(root, "encoding"), 0o755); err != nil {
		t.Fatal(err)
	}
	ciFile := filepath.Join(root, "encoding", "bad.ci")
	if err := os.WriteFile(ciFile, []byte("1:raw:FFFF\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	suite, err := discoverExaBGPSuite(base)
	if err == nil {
		t.Fatalf("discover suite unexpectedly succeeded: %#v", suite)
	}
}

func writeExaBGPFixture(t *testing.T, base, name, config string, tcpConnections int, options ...string) {
	t.Helper()
	root := filepath.Join(base, "test", "exabgp-compat")
	if err := os.MkdirAll(filepath.Join(root, "encoding"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "etc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "etc", config), []byte("neighbor 127.0.0.1 { }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	content := "option=file:" + config + "\noption=tcp_connections:" + strconv.Itoa(tcpConnections) + "\n" + strings.Join(options, "\n")
	if err := os.WriteFile(filepath.Join(root, "encoding", name+".ci"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
