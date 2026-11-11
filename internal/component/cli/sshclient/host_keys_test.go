// Design: docs/guide/operations.md -- verified remote SSH connections.

package sshclient

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/opererr"
)

func hostKeyTestEnvironment(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ze.ssh.insecure", "false")
	t.Setenv("ze.ssh.known.hosts", "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	return home
}

func hostKeyTestSigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func writeHostKeyTestFile(t *testing.T, filename, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteHostKeysUseDefaultKnownHosts(t *testing.T) {
	home := hostKeyTestEnvironment(t)
	signer := hostKeyTestSigner(t)
	address := "router.example:2222"
	writeHostKeyTestFile(t, filepath.Join(home, ".ssh", "known_hosts"), knownhosts.Line([]string{address}, signer.PublicKey())+"\n")
	callback, err := hostKeyCallback("router.example")
	if err != nil {
		t.Fatal(err)
	}
	if err := callback(address, &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 2222}, signer.PublicKey()); err != nil {
		t.Fatalf("trusted remote key refused: %v", err)
	}
}

func TestRemoteHostKeysUseConfiguredHashedIPv6AndCustomPort(t *testing.T) {
	home := hostKeyTestEnvironment(t)
	signer := hostKeyTestSigner(t)
	address := "[2001:db8::1]:2200"
	path := filepath.Join(home, "operator trust", "hosts")
	line := knownhosts.Line([]string{address}, signer.PublicKey())
	_, rest, _ := strings.Cut(line, " ")
	writeHostKeyTestFile(t, path, knownhosts.HashHostname(knownhosts.Normalize(address))+" "+rest+"\n")
	t.Setenv("ze.ssh.known.hosts", path)
	env.ResetCache()
	callback, err := hostKeyCallback("2001:db8::1")
	if err != nil {
		t.Fatal(err)
	}
	remote := &net.TCPAddr{IP: net.ParseIP("2001:db8::1"), Port: 2200}
	if err := callback(address, remote, signer.PublicKey()); err != nil {
		t.Fatalf("configured hashed address refused: %v", err)
	}
	if err := callback("[2001:db8::1]:2222", remote, signer.PublicKey()); err == nil {
		t.Fatal("trusted key accepted for a different port")
	}
}

func TestRemoteHostKeysRefuseUnknownChangedAndRevokedKeys(t *testing.T) {
	for _, mode := range []string{"unknown", "changed", "revoked"} {
		t.Run(mode, func(t *testing.T) {
			home := hostKeyTestEnvironment(t)
			known, offered := hostKeyTestSigner(t), hostKeyTestSigner(t)
			address := "router.example:2222"
			body := knownhosts.Line([]string{address}, known.PublicKey()) + "\n"
			if mode == "unknown" {
				body = "# no trusted hosts\n"
			}
			if mode == "revoked" {
				body = "@revoked " + knownhosts.Line([]string{address}, offered.PublicKey()) + "\n"
			}
			path := filepath.Join(home, ".ssh", "known_hosts")
			writeHostKeyTestFile(t, path, body)
			callback, err := hostKeyCallback("router.example")
			if err != nil {
				t.Fatal(err)
			}
			err = callback(address, &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 2222}, offered.PublicKey())
			if err == nil {
				t.Fatalf("%s key accepted", mode)
			}
			var refusal *opererr.Error
			if !errors.As(err, &refusal) {
				t.Fatalf("host-key failure has no operator repair: %v", err)
			}
			if !strings.Contains(refusal.Fix(), "ssh") || !strings.Contains(refusal.Error(), path) {
				t.Fatalf("host-key refusal omits trust file or next step: %v", err)
			}
			if mode == "revoked" {
				var revoked *knownhosts.RevokedError
				if !errors.As(err, &revoked) {
					t.Fatalf("revocation cause lost: %v", err)
				}
			} else {
				var mismatch *knownhosts.KeyError
				if !errors.As(err, &mismatch) {
					t.Fatalf("unknown/changed-key cause lost: %v", err)
				}
				if (len(mismatch.Want) > 0) != (mode == "changed") {
					t.Fatalf("%s cause changed: %+v", mode, mismatch)
				}
			}
			if got, err := os.ReadFile(path); err != nil || string(got) != body {
				t.Fatalf("refused key changed trust file: %q, %v", got, err)
			}
		})
	}
}

func TestRemoteHostKeysRefuseMissingOrMalformedTrustFile(t *testing.T) {
	for _, malformed := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "malformed"}[malformed], func(t *testing.T) {
			home := hostKeyTestEnvironment(t)
			path := filepath.Join(home, ".ssh", "known_hosts")
			if malformed {
				writeHostKeyTestFile(t, path, "not an OpenSSH known_hosts record\n")
			}
			_, err := hostKeyCallback("router.example")
			if err == nil {
				t.Fatal("unusable trust file accepted")
			}
			var refusal *opererr.Error
			if !errors.As(err, &refusal) || !strings.Contains(err.Error(), path) {
				t.Fatalf("unusable trust file has no specific repair: %v", err)
			}
		})
	}
}

func TestRemoteHostKeysPreserveExplicitInsecureAndLoopbackPolicy(t *testing.T) {
	hostKeyTestEnvironment(t)
	signer := hostKeyTestSigner(t)
	for _, host := range []string{"127.0.0.1", "::1", "localhost"} {
		callback, err := hostKeyCallback(host)
		if err != nil {
			t.Fatal(err)
		}
		if err := callback(net.JoinHostPort(host, "2222"), &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 2222}, signer.PublicKey()); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("ze.ssh.insecure", "true")
	env.ResetCache()
	callback, err := hostKeyCallback("router.example")
	if err != nil {
		t.Fatal(err)
	}
	if err := callback("router.example:2222", &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 2222}, signer.PublicKey()); err != nil {
		t.Fatal(err)
	}
}
