package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestBlockedFailsAnAnsweredAttempt: blocked passes only an attempt that got
// no answer, so a port that opens and a port the stack refuses both fail it:
// a published port that works, and a port nothing publishes, cannot pass for
// one the firewall policy drops.
//
// Method: one open loopback listener and one closed loopback port. The
// passing case needs a packet dropped on its way, which only the guest test
// container-port-blocked produces.
func TestBlockedFailsAnAnsweredAttempt(t *testing.T) {
	open, openPort := listenLoopback(t)
	go serve(open)
	if blocked("127.0.0.1", openPort) {
		t.Fatal("blocked accepted a port that opened")
	}
	closed, closedPort := listenLoopback(t)
	if err := closed.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if blocked("127.0.0.1", closedPort) {
		t.Fatal("blocked accepted a port that was refused")
	}
}

// TestAdoptFailsWithoutItsReplacement: adopt fails, and leaves the daemon's
// file as it is, when the replacement cannot be read: it never asks for a
// reload of a file it did not write.
//
// Method: a config file in a temporary folder and a replacement path that
// names no file; the config must hold its bytes afterwards.
func TestAdoptFailsWithoutItsReplacement(t *testing.T) {
	folder := t.TempDir()
	config := filepath.Join(folder, "running.conf")
	if err := os.WriteFile(config, []byte("kept\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if adopt(config, filepath.Join(folder, "absent.conf")) {
		t.Fatal("adopt passed with no replacement to read")
	}
	held, err := os.ReadFile(config) //nolint:gosec // the test's own file
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(held) != "kept\n" {
		t.Fatalf("adopt changed the config to %q with no replacement to read", held)
	}
}
