//go:build linux

package vpp

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func startMockCLI(t *testing.T, response string) string {
	t.Helper()
	dir := t.TempDir()
	sock := filepath.Join(dir, "cli.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4096)
		conn.Read(buf)
		conn.Write([]byte(response))
	}()
	return sock
}

func setTestSocket(t *testing.T, path string) {
	t.Helper()
	t.Setenv("ZE_TEST_VPP_CLI_SOCKET", path)
}

func TestExecCLI(t *testing.T) {
	sock := startMockCLI(t, "mock output line 1\nmock output line 2\n")
	setTestSocket(t, sock)

	out, err := execCLI("show trace")
	if err != nil {
		t.Fatalf("execCLI error: %v", err)
	}
	if !strings.Contains(out, "mock output line 1") {
		t.Errorf("output missing expected content: %s", out)
	}
	if !strings.Contains(out, "mock output line 2") {
		t.Errorf("output missing expected content: %s", out)
	}
}

func TestExecCLISocketNotFound(t *testing.T) {
	setTestSocket(t, filepath.Join(t.TempDir(), "nonexistent.sock"))

	_, err := execCLI("show trace")
	if err == nil {
		t.Fatal("expected error when socket does not exist")
	}
	if !strings.Contains(err.Error(), "dial") {
		t.Errorf("expected dial error, got: %v", err)
	}
}

func TestExecCLIEmptyResponse(t *testing.T) {
	sock := startMockCLI(t, "")
	setTestSocket(t, sock)

	out, err := execCLI("clear trace")
	if err != nil {
		t.Fatalf("execCLI error: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty output, got: %q", out)
	}
}

func TestCliSocketPathDefault(t *testing.T) {
	os.Unsetenv("ZE_TEST_VPP_CLI_SOCKET")
	if got := cliSocketPath(); got != defaultCLISocket {
		t.Errorf("cliSocketPath() = %q, want %q", got, defaultCLISocket)
	}
}

func TestCliSocketPathOverride(t *testing.T) {
	t.Setenv("ZE_TEST_VPP_CLI_SOCKET", "/tmp/custom.sock")
	if got := cliSocketPath(); got != "/tmp/custom.sock" {
		t.Errorf("cliSocketPath() = %q, want /tmp/custom.sock", got)
	}
}
