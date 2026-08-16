package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/test/runner"
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

// the three TestResolveZeDaemonBinary* cases here tested a private
// resolver this package should never have had. Their subject (honor ZE_BIN,
// find this session's binary, fail closed when there is none) is buildZe, which
// already carries that coverage in build_test.go -- TestBuildZeNoBuild,
// TestBuildZeNoBuildEnvOverride, TestBuildZeNoBuildRelativeOverride -- and is
// enforced package-wide by TestSuiteRunnersResolveDUTThroughBuildZe. The
// duplicate resolver was deleted, so the coverage moves rather than shrinks.
//
// VALIDATES: cmd_exabgp.go names buildZe, so that structural guard cannot be
// satisfied by dropping the DUT lookup altogether.
// PREVENTS: the ExaBGP wrapper being left to guess again. It has no way to know
// where this session's bin/ is or which build tags a binary carries, and the
// version that guessed reached a stale root-level ./ze with no ze_exabgp
// command -- all 42 encoding tests red on "unknown command: exabgp"
// (ai/rules/evidence.md).
func TestExaBGPSuiteCallsBuildZe(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(".", "cmd_exabgp.go"))
	if err != nil {
		t.Fatalf("read cmd_exabgp.go: %v", err)
	}
	if !strings.Contains(string(src), "buildZe(ctx, baseDir)") {
		t.Error("cmd_exabgp.go must resolve the ze binary with buildZe(ctx, baseDir)")
	}
}

// VALIDATES: the resolved path reaches the wrapper process as ZE_BIN, last, so
// it beats any value inherited from the parent shell.
// PREVENTS: the wrapper guessing again because the runner resolved a binary and
// then never told it (ai/rules/evidence.md, "make the miss explicit at
// the producer").
func TestExaBGPClientEnvExportsResolvedZeBin(t *testing.T) {
	t.Setenv("ZE_BIN", "/inherited/ze")
	test := &exabgpTestEntry{record: &runner.Record{Nick: "1"}, tcpConnections: 1}

	got := exaBGPClientEnv(test, 17900, "/resolved/ze")

	last := ""
	for _, entry := range got {
		if strings.HasPrefix(entry, "ZE_BIN=") {
			last = entry
		}
	}
	if last != "ZE_BIN=/resolved/ze" {
		t.Fatalf("effective ZE_BIN = %q, want the resolved path", last)
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
