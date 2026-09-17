// Design: docs/architecture/zefs-format.md -- framed-tree integrity boundaries.

//go:build linux || darwin

package zefs

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func writeFrameFixture(t *testing.T, root, key string, frame []byte) {
	t.Helper()
	path := filepath.Join(root, key)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, frame, 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestCheckPathDirectory requires a complete single frame and diagnoses the
// logical key, including corruption that a valid first frame would hide.
func TestCheckPathDirectory(t *testing.T) {
	frame, err := EncodeNetcapstring([]byte("secret"), 8)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := bytes.Clone(frame)
	corrupt[bytes.IndexByte(corrupt, '\n')+1] ^= 1
	cases := []struct {
		name   string
		frame  []byte
		status string
	}{
		{"clean", frame, "ok"},
		{"crc", corrupt, "crc-mismatch"},
		{"truncated", frame[:len(frame)-1], "truncated"},
		{"appended", append(bytes.Clone(frame), 'x'), entryStatusParseError},
		{"concatenated", append(bytes.Clone(frame), frame...), entryStatusParseError},
		{"max-int", []byte("19:9223372036854775807:0000000000000000000:00000000\n\n"), "truncated"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "source")
			writeFrameFixture(t, root, "meta/test/key", tt.frame)
			report, err := CheckPath(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(report.Entries) != 1 {
				t.Fatalf("entries %v", report.Entries)
			}
			entry := report.Entries[0]
			if entry.Key != "meta/test/key" {
				t.Fatalf("key %q", entry.Key)
			}
			if entry.Status != tt.status {
				t.Fatalf("status %q, want %q: %s", entry.Status, tt.status, entry.Error)
			}
			wantCorrupt := 1
			if tt.status == "ok" {
				wantCorrupt = 0
			}
			if report.CorruptEntries != wantCorrupt {
				t.Fatalf("corrupt count %d, want %d", report.CorruptEntries, wantCorrupt)
			}
		})
	}
}

// TestRepairDirectory preserves every good value and the original evidence,
// excludes corrupt keys, and produces modes accepted by the same secure reader.
func TestRepairDirectory(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "source")
	good, err := EncodeNetcapstring([]byte("value"), 12)
	if err != nil {
		t.Fatal(err)
	}
	empty, err := EncodeNetcapstring(nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	bad := append(bytes.Clone(good), 'x')
	writeFrameFixture(t, source, "meta/good", good)
	writeFrameFixture(t, source, "file/active/empty", empty)
	writeFrameFixture(t, source, "meta/bad", bad)
	destination := filepath.Join(parent, "repaired")
	report, err := RepairPath(source, destination)
	if err != nil {
		t.Fatal(err)
	}
	if report.RecoveredCount != 2 {
		t.Fatalf("recovered %v", report)
	}
	if report.SkippedCount != 1 {
		t.Fatalf("skipped %v", report)
	}
	if report.Skipped[0].Key != "meta/bad" {
		t.Fatalf("skipped key %q", report.Skipped[0].Key)
	}
	for key, want := range map[string][]byte{"meta/good": good, "file/active/empty": empty} {
		got, err := os.ReadFile(filepath.Join(destination, key))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("recovered %s differs", key)
		}
	}
	if _, err := os.Stat(filepath.Join(destination, "meta/bad")); !os.IsNotExist(err) {
		t.Fatalf("corrupt key copied: %v", err)
	}
	original, err := os.ReadFile(filepath.Join(source, "meta/bad"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, bad) {
		t.Fatal("source evidence changed")
	}
	check, err := CheckPath(destination)
	if err != nil {
		t.Fatalf("reopen repaired tree: %v", err)
	}
	if check.TotalEntries != 2 {
		t.Fatalf("repaired keys %v", check)
	}
	if check.CorruptEntries != 0 {
		t.Fatalf("corrupt repaired tree %v", check)
	}
	if _, err := RepairPath(source, destination); err == nil {
		t.Fatal("existing destination overwritten")
	}
	inside := filepath.Join(source, "nested-repair")
	if _, err := RepairPath(source, inside); err == nil {
		t.Fatal("source-descendant destination accepted")
	}
	if _, err := os.Stat(inside); !os.IsNotExist(err) {
		t.Fatalf("repair changed source: %v", err)
	}
}

// TestTreeIntegrityRefusesUnsafeNodes verifies both entry points reject special
// nodes and permissions before reading, including a FIFO with no writer.
func TestTreeIntegrityRefusesUnsafeNodes(t *testing.T) {
	for _, kind := range []string{"root-symlink", "directory-symlink", "file-symlink", "fifo", "root-mode", "directory-mode", "file-mode"} {
		t.Run(kind, func(t *testing.T) {
			parent := t.TempDir()
			root := filepath.Join(parent, "source")
			frame, err := EncodeNetcapstring([]byte("value"), 5)
			if err != nil {
				t.Fatal(err)
			}
			writeFrameFixture(t, root, "meta/key", frame)
			badPath := root
			switch kind {
			case "root-symlink":
				badPath = filepath.Join(parent, "link")
				err = os.Symlink(root, badPath)
				root = badPath
			case "directory-symlink":
				badPath = filepath.Join(root, "link")
				err = os.Symlink(filepath.Join(root, "meta"), badPath)
			case "file-symlink":
				badPath = filepath.Join(root, "link")
				err = os.Symlink(filepath.Join(root, "meta/key"), badPath)
			case "fifo":
				badPath = filepath.Join(root, "pipe")
				err = unix.Mkfifo(badPath, 0o600)
			case "root-mode":
				err = os.Chmod(root, 0o750)
			case "directory-mode":
				badPath = filepath.Join(root, "meta")
				err = os.Chmod(badPath, 0o750)
			case "file-mode":
				badPath = filepath.Join(root, "meta/key")
				err = os.Chmod(badPath, 0o640)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := CheckPath(root); err == nil {
				t.Fatal("unsafe check accepted")
			} else if !strings.Contains(err.Error(), badPath) {
				t.Fatalf("error omits path %s: %v", badPath, err)
			}
			dst := filepath.Join(parent, "repaired")
			if _, err := RepairPath(root, dst); err == nil {
				t.Fatal("unsafe repair accepted")
			}
			if _, err := os.Stat(dst); !os.IsNotExist(err) {
				t.Fatalf("unsafe repair published destination: %v", err)
			}
		})
	}
}

// TestTreeIntegrityRequiresOwner verifies root has no bypass for another uid.
func TestTreeIntegrityRequiresOwner(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("changing file ownership requires root")
	}
	for _, key := range []string{"", "meta", "meta/key"} {
		t.Run(key, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "source")
			frame, err := EncodeNetcapstring(nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			writeFrameFixture(t, root, "meta/key", frame)
			path := filepath.Join(root, key)
			if err := os.Chown(path, 65534, -1); err != nil {
				t.Fatal(err)
			}
			if _, err := CheckPath(root); err == nil {
				t.Fatal("root accepted another owner's tree")
			} else if !strings.Contains(err.Error(), path) {
				t.Fatalf("error omits owned path: %v", err)
			}
		})
	}
}

// TestTreeIntegrityPrivateContainment accepts writable non-store directories
// only below a private owner-controlled ancestor, for both reading and repair.
func TestTreeIntegrityPrivateContainment(t *testing.T) {
	frame, err := EncodeNetcapstring([]byte("private value"), 13)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []bool{true, false} {
		name := "public-parent"
		if private {
			name = "private-parent"
		}
		t.Run(name, func(t *testing.T) {
			// Use the public temporary directory directly: t.TempDir's private
			// ancestor would otherwise protect the public-parent refusal case.
			base, err := os.MkdirTemp("/tmp", "zefs-containment-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.RemoveAll(base); err != nil {
					t.Error(err)
				}
			})
			mode := os.FileMode(0o755)
			if private {
				mode = 0o700
			}
			if err := os.Chmod(base, mode); err != nil {
				t.Fatal(err)
			}
			shared := filepath.Join(base, "shared")
			if err := os.Mkdir(shared, 0o775); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(shared, 0o775); err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(shared, "source")
			writeFrameFixture(t, source, "meta/key", frame)
			check, err := CheckPath(source)
			if private {
				if err != nil {
					t.Fatal(err)
				}
				if check.TotalEntries != 1 || check.CorruptEntries != 0 || check.Entries[0].Key != "meta/key" {
					t.Fatalf("private tree check: %+v", check)
				}
			} else if err == nil {
				t.Fatal("check accepted writable ancestry before private tree root")
			} else if !strings.Contains(err.Error(), shared) {
				t.Fatalf("check omitted unsafe ancestor: %v", err)
			}

			// A separately safe source isolates the destination-parent check.
			safe := filepath.Join(base, "safe")
			writeFrameFixture(t, safe, "meta/key", frame)
			destination := filepath.Join(shared, "repaired")
			repair, err := RepairPath(safe, destination)
			if private {
				if err != nil {
					t.Fatal(err)
				}
				if repair.RecoveredCount != 1 || repair.SkippedCount != 0 {
					t.Fatalf("private tree repair: %+v", repair)
				}
				got, err := os.ReadFile(filepath.Join(destination, "meta/key"))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, frame) {
					t.Fatal("repair changed recovered frame")
				}
			} else {
				if err == nil {
					t.Fatal("repair accepted unprotected writable destination parent")
				}
				if !strings.Contains(err.Error(), shared) {
					t.Fatalf("repair omitted unsafe ancestor: %v", err)
				}
				if _, err := os.Lstat(destination); !os.IsNotExist(err) {
					t.Fatalf("unsafe repair published output: %v", err)
				}
				// Leaving a private directory cannot protect its public sibling.
				escaped := safe + "/../shared/source"
				if _, err := CheckPath(escaped); err == nil {
					t.Fatal("check retained private containment after leaving it")
				}
			}
			info, err := os.Stat(shared)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0o775 {
				t.Fatalf("changed non-store directory mode to %04o", info.Mode().Perm())
			}
		})
	}
}

// TestTreeIntegrityIntermediateSymlinks rejects links before the tree root and
// before the repair destination, including links hidden by lexical ".." cleanup.
func TestTreeIntegrityIntermediateSymlinks(t *testing.T) {
	parent := t.TempDir()
	actual := filepath.Join(parent, "actual")
	source := filepath.Join(actual, "source")
	frame, err := EncodeNetcapstring([]byte("evidence"), 8)
	if err != nil {
		t.Fatal(err)
	}
	writeFrameFixture(t, source, "meta/key", frame)
	link := filepath.Join(parent, "link")
	if err := os.Symlink(actual, link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		link + "/source",
		link + "/../actual/source",
	} {
		if _, err := CheckPath(path); err == nil {
			t.Fatalf("check followed intermediate link: %s", path)
		}
		dst := filepath.Join(parent, "repaired")
		if _, err := RepairPath(path, dst); err == nil {
			t.Fatalf("repair followed intermediate source link: %s", path)
		}
		if _, err := os.Lstat(dst); !os.IsNotExist(err) {
			t.Fatalf("unsafe source published output: %v", err)
		}
	}
	for _, dst := range []string{
		link + "/repaired",
		link + "/../actual/repaired",
	} {
		if _, err := RepairPath(source, dst); err == nil {
			t.Fatalf("repair followed intermediate destination link: %s", dst)
		}
		if _, err := os.Lstat(filepath.Join(actual, "repaired")); !os.IsNotExist(err) {
			t.Fatalf("unsafe parent received output: %v", err)
		}
	}
	before, err := os.ReadFile(filepath.Join(source, "meta/key"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, frame) {
		t.Fatal("source evidence changed")
	}
	entries, err := os.ReadDir(actual)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("refused repair created staging under linked parent: %v", entries)
	}
}

// TestRepairTreePublication exercises the platform's no-replace primitive
// against an existing empty directory, which ordinary rename can overwrite.
// The same case runs against Linux Renameat2 and Darwin RenameatxNp.
func TestRepairTreePublication(t *testing.T) {
	path := t.TempDir()
	parent, err := openFrameDirectory(path)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close() //nolint:errcheck // Test cleanup.
	for _, name := range []string{"stage", "existing"} {
		if err := os.Mkdir(filepath.Join(path, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	frame, err := EncodeNetcapstring([]byte("keep"), 4)
	if err != nil {
		t.Fatal(err)
	}
	writeFrameFixture(t, filepath.Join(path, "stage"), "key", frame)
	before, err := os.Stat(filepath.Join(path, "existing"))
	if err != nil {
		t.Fatal(err)
	}
	if err := renameFrameTree(parent, "stage", "existing"); err == nil {
		t.Fatal("publication replaced an existing empty directory")
	}
	after, err := os.Stat(filepath.Join(path, "existing"))
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("publication replaced destination inode")
	}
	if err := renameFrameTree(parent, "stage", "published"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(path, "published/key"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, frame) {
		t.Fatalf("published frame differs: %q", got)
	}
}

// TestRepairPublicationPinnedParent replaces a parent's pathname after opening
// it and checks that publication still targets the retained directory.
func TestRepairPublicationPinnedParent(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "parent")
	if err := os.MkdirAll(filepath.Join(path, "stage"), 0o700); err != nil {
		t.Fatal(err)
	}
	parent, err := openFrameDirectory(path)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close() //nolint:errcheck // Test cleanup.
	moved := filepath.Join(base, "moved")
	if err := os.Rename(path, moved); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(base, "other")
	if err := os.Mkdir(other, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, path); err != nil {
		t.Fatal(err)
	}
	if err := renameFrameTree(parent, "stage", "published"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(moved, "published")); err != nil {
		t.Fatalf("retained parent did not receive publication: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(other, "published")); !os.IsNotExist(err) {
		t.Fatalf("publication followed replacement symlink: %v", err)
	}
}

// TestTreeIntegrityNativeDepth accepts a valid key deeper than the removed
// integrity-only limit and preserves it through repair.
func TestTreeIntegrityNativeDepth(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "source")
	key := strings.Repeat("d/", 80) + "value"
	frame, err := EncodeNetcapstring([]byte("deep value"), 10)
	if err != nil {
		t.Fatal(err)
	}
	writeFrameFixture(t, source, key, frame)
	check, err := CheckPath(source)
	if err != nil {
		t.Fatal(err)
	}
	if check.TotalEntries != 1 {
		t.Fatalf("deep key not checked: %v", check)
	}
	if check.Entries[0].Key != key {
		t.Fatalf("checked key %q, want %q", check.Entries[0].Key, key)
	}
	destination := filepath.Join(parent, "repaired")
	repair, err := RepairPath(source, destination)
	if err != nil {
		t.Fatal(err)
	}
	if repair.RecoveredCount != 1 {
		t.Fatalf("deep key not recovered: %v", repair)
	}
	got, err := os.ReadFile(filepath.Join(destination, key))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, frame) {
		t.Fatal("deep frame changed during repair")
	}
}
