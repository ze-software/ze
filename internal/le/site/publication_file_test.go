// Design: docs/contributing/gh-pages.md -- traversal metadata cannot authorize a replaced artifact path.
package site

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestSitePublicationSnapshotRejectsReplacedAliases replaces a known regular
// path at the acquisition boundary, without a scheduling-dependent race loop.
// Both the leaf and an ancestor can alias an ordinary file inside the root.
func TestSitePublicationSnapshotRejectsReplacedAliases(t *testing.T) {
	for _, kind := range []string{"leaf symlink", "ancestor symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			output := t.TempDir()
			if err := os.Mkdir(filepath.Join(output, "a"), 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(output, "a", "file.txt")
			if err := os.WriteFile(path, []byte("reviewed bytes"), 0o600); err != nil {
				t.Fatal(err)
			}
			mutated := false
			acquire := func(root *os.Root, name string) (*os.File, error) {
				if name == "a/file.txt" {
					if mutated {
						t.Fatal("snapshot acquired the fixture path twice")
					}
					mutated = true
					if kind == "ancestor symlink" {
						if err := os.Rename(filepath.Join(output, "a"), filepath.Join(output, "original-directory")); err != nil {
							t.Fatal(err)
						}
						if err := os.Symlink("original-directory", filepath.Join(output, "a")); err != nil {
							t.Fatal(err)
						}
					} else {
						if err := os.Rename(path, filepath.Join(output, "original-file")); err != nil {
							t.Fatal(err)
						}
						if kind == "leaf symlink" {
							if err := os.Symlink("../original-file", path); err != nil {
								t.Fatal(err)
							}
						} else {
							if err := os.Mkdir(path, 0o700); err != nil {
								t.Fatal(err)
							}
						}
					}
				}
				return publicationOpenFile(root, name)
			}
			if _, _, err := publicationSnapshotWith(output, acquire); err == nil {
				t.Error("snapshot accepted a replaced nonregular path")
			}
			if !mutated {
				t.Error("test did not reach acquisition after regular traversal metadata")
			}
		})
	}
}

// TestSitePublicationSnapshotRejectsReplacedFIFO deterministically substitutes
// a FIFO after traversal. Its one reader is joined even when the counterfactual
// blocking acquisition needs a writer before it can finish.
func TestSitePublicationSnapshotRejectsReplacedFIFO(t *testing.T) {
	output := t.TempDir()
	path := filepath.Join(output, "file.txt")
	if err := os.WriteFile(path, []byte("reviewed bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	mutated := make(chan struct{})
	done := make(chan error, 1)
	joined := false
	acquire := func(root *os.Root, name string) (*os.File, error) {
		if name == "file.txt" {
			if err := os.Rename(path, filepath.Join(output, "original-file")); err != nil {
				return nil, err
			}
			if err := syscall.Mkfifo(path, 0o600); err != nil {
				return nil, err
			}
			close(mutated)
		}
		return publicationOpenFile(root, name)
	}
	// One owned snapshot-reader lifecycle. Cleanup releases the exact FIFO
	// and joins the reader before t.TempDir removes the fixture.
	go func() {
		_, _, err := publicationSnapshotWith(output, acquire)
		done <- err
	}()
	t.Cleanup(func() {
		if joined {
			return
		}
		release, err := os.OpenFile(path, os.O_RDWR|syscall.O_NONBLOCK, 0)
		if err != nil {
			t.Errorf("could not release the owned snapshot reader: %v", err)
			return
		}
		// Closing the writer lets a counterfactual io.Copy reader observe EOF.
		// Joining first would deadlock while this descriptor remains a writer.
		if err := release.Close(); err != nil {
			t.Error(err)
		}
		join := time.NewTimer(10 * time.Second)
		defer join.Stop()
		select {
		case <-done:
			joined = true
		case <-join.C:
			t.Error("owned snapshot reader did not finish after its FIFO writer closed")
		}
	})
	select {
	case <-mutated:
	case err := <-done:
		joined = true
		if err == nil {
			t.Error("snapshot accepted a replaced FIFO")
		}
		select {
		case <-mutated:
		default:
			t.Error("fixture did not replace the file after traversal")
		}
		return
	}
	bound := time.NewTimer(10 * time.Second)
	defer bound.Stop()
	select {
	case err := <-done:
		joined = true
		if err == nil {
			t.Error("snapshot accepted a replaced FIFO")
		}
		select {
		case <-mutated:
		default:
			t.Error("fixture did not replace the file after traversal")
		}
	case <-bound.C:
		t.Error("snapshot acquisition waited for a FIFO writer")
	}
}
