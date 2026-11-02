// Design: docs/architecture/testing/interop.md -- the native speaker's readiness fence.
package bgp

import (
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestParsedEmptyMPReadyOwnsItsFile proves the readiness writer creates its own
// marker and refuses existing files and symlinks without changing their contents.
// Every path belongs to this test; the lab's fixed /tmp paths are never touched.
func TestParsedEmptyMPReadyOwnsItsFile(t *testing.T) {
	t.Run("new marker", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "ready")
		if err := writeParsedEmptyMPReady(path); err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != "established\n" {
			t.Fatalf("marker = %q, want establishment fence", content)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("marker permissions = %v, want 0600", info.Mode().Perm())
		}
	})
	for _, symlink := range []bool{false, true} {
		name := "existing file"
		if symlink {
			name = "symlink"
		}
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			target := filepath.Join(directory, "target")
			const original = "owned by another writer\n"
			if err := os.WriteFile(target, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			path := target
			if symlink {
				path = filepath.Join(directory, "ready")
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			}
			if err := writeParsedEmptyMPReady(path); !errors.Is(err, os.ErrExist) {
				t.Errorf("existing marker error = %v, want file-exists refusal", err)
			}
			content, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != original {
				t.Errorf("existing target overwritten: got %q, want %q", content, original)
			}
		})
	}
}

// TestParsedEmptyMPSourcePropagatesMarkerRefusal drives the registered speaker
// entry through OPEN and KEEPALIVE over TCP. A preexisting marker must reach the
// caller as a file-exists error, without overwriting its file or symlink target.
// --result keeps both capture and marker under this test's private directory.
func TestParsedEmptyMPSourcePropagatesMarkerRefusal(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		name := "existing file"
		if symlink {
			name = "symlink"
		}
		t.Run(name, func(t *testing.T) {
			capture := filepath.Join(t.TempDir(), "capture.jsonl")
			marker := capture + ".ready"
			target := marker
			if symlink {
				target = filepath.Join(filepath.Dir(capture), "target")
			}
			const original = "owned by another writer\n"
			if err := os.WriteFile(target, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			if symlink {
				if err := os.Symlink(target, marker); err != nil {
					t.Fatal(err)
				}
			}
			listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { listener.Close() }) //nolint:errcheck // Cleanup closes only this test's listener.
			deadline := time.Now().Add(5 * time.Second)
			if err := listener.SetDeadline(deadline); err != nil {
				t.Fatal(err)
			}
			result := make(chan error, 1)
			stopped := make(chan struct{})
			// This goroutine owns one bounded speaker lifecycle. Cleanup MUST
			// join it; the accepted transport closes first to interrupt its reads.
			go func() {
				defer close(stopped)
				result <- runSpeakerHelper([]string{
					"--connect", listener.Addr().String(), "--asn", "65004",
					"--router-id", "192.0.2.4", "--test", parsedEmptyMPSource,
					"--duration", "5", "--result", capture,
				}, io.Discard)
			}()
			t.Cleanup(func() {
				select {
				case <-stopped:
				case <-time.After(6 * time.Second):
					t.Error("speaker did not stop within its bounded lifetime")
				}
			})
			peer, err := listener.AcceptTCP()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { peer.Close() }) //nolint:errcheck // The speaker may already have closed its connection.
			if err := peer.SetDeadline(deadline); err != nil {
				t.Fatal(err)
			}
			readLLGRSourceTestMessage(t, peer, bgpOpen)
			// RFC 5492 Section 4, RFC 6793 Section 3: advertise both unicast
			// families and ASN4 so the real source accepts the peer's OPEN.
			open, err := speakerOpen(65001, 90, net.ParseIP("192.0.2.1"), []bgpFamily{{afi: 1, safi: 1}, {afi: 2, safi: 1}}, false, false)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := peer.Write(open); err != nil {
				t.Fatal(err)
			}
			readLLGRSourceTestMessage(t, peer, bgpKeepalive)
			if _, err := peer.Write(speakerKeepalive()); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-result:
				if !errors.Is(err, os.ErrExist) {
					t.Errorf("speaker result = %v, want marker file-exists refusal", err)
				}
				if pathErr, ok := errors.AsType[*os.PathError](err); ok {
					if pathErr.Path != marker {
						t.Errorf("refused path = %q, want readiness marker %q", pathErr.Path, marker)
					}
				} else {
					t.Errorf("speaker result has no path error: %v", err)
				}
			case <-time.After(6 * time.Second):
				t.Fatal("speaker did not propagate the marker refusal")
			}
			content, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != original {
				t.Errorf("marker target overwritten: got %q, want %q", content, original)
			}
		})
	}
}
