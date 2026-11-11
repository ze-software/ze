//go:build linux

// Design: docs/guide/operations.md -- the offered trust repair selects one file.

package sshclient

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestRemoteHostKeyRepairSelectsOneOpenSSHTrustFile(t *testing.T) {
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skipf("OpenSSH configuration parser unavailable: %v", err)
	}
	for _, name := range []string{"operator  trust/hosts", "operator's/hosts", `operator"trust/hosts`, `operator\trust/hosts`} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			repair := knownHostsConnectCommand("router.example:2200", path)
			// Run the exact repair through its shell, adding only configuration
			// dump mode and an empty config. -G never connects to the network.
			// Repeated spaces distinguish one preserved filename from multiple
			// filenames: OpenSSH's -G joins array entries with single spaces.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			command := "ssh -G -F /dev/null " + strings.TrimPrefix(repair, "ssh ")
			output, err := exec.CommandContext(ctx, "sh", "-c", command).CombinedOutput()
			if err != nil {
				t.Fatalf("OpenSSH could not parse trust repair: %v\n%s", err, output)
			}
			want := "userknownhostsfile " + path
			found := 0
			for _, line := range strings.Split(string(output), "\n") {
				if strings.HasPrefix(line, "userknownhostsfile ") {
					found++
					if line != want {
						t.Fatalf("OpenSSH selected different trust files:\n got %q\nwant %q", line, want)
					}
				}
			}
			if found != 1 {
				t.Fatalf("OpenSSH trust option count = %d, want one: %s", found, output)
			}
			if !strings.Contains(string(output), "\nport 2200\n") || !strings.Contains(string(output), "\nstricthostkeychecking ask\n") {
				t.Fatalf("OpenSSH repair lost target port or confirmation: %s", output)
			}
		})
	}
}

func TestRemoteHostKeyRepairAuthenticatesWithSelectedOpenSSHTrustFile(t *testing.T) {
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skipf("OpenSSH client unavailable: %v", err)
	}
	for _, name := range []string{"operator trust/hosts", "operator's/hosts", `operator"trust/hosts`, `operator\trust/hosts`} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			signer := hostKeyTestSigner(t)
			listener, err := net.Listen("tcp4", "127.0.0.2:0")
			if err != nil {
				t.Fatal(err)
			}
			address := listener.Addr().String()
			body := knownhosts.Line([]string{address}, signer.PublicKey()) + "\n"
			writeHostKeyTestFile(t, path, body)
			done := make(chan error, 1)
			go func() { done <- serveKnownHostCommandConfig(listener, signer, &ssh.ServerConfig{NoClientAuth: true}) }()
			t.Cleanup(func() {
				_ = listener.Close()
				if err := <-done; err != nil {
					t.Errorf("OpenSSH proof server: %v", err)
				}
			})
			repair := knownHostsConnectCommand(address, path)
			// Only the selected file contains this key. Neither the user's SSH
			// config nor a system trust file can satisfy host verification.
			// Closed stdin and BatchMode prohibit accepting an unknown key.
			command := "ssh -T -F /dev/null -o GlobalKnownHostsFile=/dev/null -o BatchMode=yes " + strings.TrimPrefix(repair, "ssh ") + " 'show version'"
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			output, err := exec.CommandContext(ctx, "sh", "-c", command).CombinedOutput()
			if err != nil || strings.TrimSpace(string(output)) != "verified-host-key-command" {
				t.Fatalf("selected trust file did not authenticate OpenSSH command: %v\n%s", err, output)
			}
			if got, err := os.ReadFile(path); err != nil || string(got) != body {
				t.Fatalf("OpenSSH changed the verified trust file: %q, %v", got, err)
			}
		})
	}
}
