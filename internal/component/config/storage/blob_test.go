package storage

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNewBlobSelfHealsCorruptStore verifies a 0-byte (unreadable) blob does not
// wedge storage: NewBlob moves it aside to .replaced-<date> and recreates a
// usable store instead of returning an error.
//
// VALIDATES: NewBlob recovers from a corrupt store instead of failing forever.
// PREVENTS: regression of the zefs "mmap: empty file" wedge, where NewBlob chose
// Open vs Create by file existence (os.Stat) rather than validity, so a 0-byte
// database.zefs left by an interrupted/concurrent write broke every startup.
func TestNewBlobSelfHealsCorruptStore(t *testing.T) {
	dir := t.TempDir()
	blobPath := filepath.Join(dir, "database.zefs")

	// Simulate the corruption: a 0-byte store left by an interrupted write.
	if err := os.WriteFile(blobPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	st, err := NewBlob(blobPath, dir)
	if err != nil {
		t.Fatalf("NewBlob on 0-byte store returned error, want self-heal: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	// The corrupt file is preserved aside for post-mortem, not destroyed.
	aside, _ := filepath.Glob(blobPath + ".replaced-*")
	if len(aside) != 1 {
		t.Fatalf("want exactly one .replaced-<date> backup, got %d", len(aside))
	}

	// A fresh, non-empty, usable store now exists at the original path.
	info, statErr := os.Stat(blobPath)
	if statErr != nil {
		t.Fatalf("recreated blob missing: %v", statErr)
	}
	if info.Size() == 0 {
		t.Fatal("recreated blob is still 0 bytes")
	}
	name := filepath.Join(dir, "probe.conf")
	if err := st.WriteFile(name, []byte("hello"), 0); err != nil {
		t.Fatalf("recreated store not writable: %v", err)
	}
	got, err := st.ReadFile(name)
	if err != nil || string(got) != "hello" {
		t.Fatalf("recreated store read-back = %q, %v; want \"hello\"", got, err)
	}
}

// TestNewBlobOpensValidStoreInPlace verifies a valid existing store is opened as
// is: no .replaced backup is made and previously written data survives a
// close/reopen. This is the guard that the self-heal path does not fire on
// healthy stores.
func TestNewBlobOpensValidStoreInPlace(t *testing.T) {
	dir := t.TempDir()
	blobPath := filepath.Join(dir, "database.zefs")
	name := filepath.Join(dir, "keep.conf")

	first, err := NewBlob(blobPath, dir)
	if err != nil {
		t.Fatalf("initial NewBlob: %v", err)
	}
	if err := first.WriteFile(name, []byte("persisted"), 0); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	second, err := NewBlob(blobPath, dir)
	if err != nil {
		t.Fatalf("reopen NewBlob: %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })

	if aside, _ := filepath.Glob(blobPath + ".replaced-*"); len(aside) != 0 {
		t.Errorf("valid store must not be moved aside, found %d backups", len(aside))
	}
	got, err := second.ReadFile(name)
	if err != nil || string(got) != "persisted" {
		t.Fatalf("reopened store read-back = %q, %v; want \"persisted\"", got, err)
	}
}

// VALIDATES: List accepts a filesystem directory prefix and returns the config
// files written from that directory.
// PREVENTS: the pending-change review UI rendering an empty body under blob
// storage. Editor.listChangeFiles lists filepath.Dir(originalPath), so a
// directory prefix that resolves to nothing hides every structural operation an
// operator is about to commit.
func TestBlobStorageListFilesystemDirectory(t *testing.T) {
	dir := t.TempDir()
	st, err := NewBlob(filepath.Join(dir, "database.zefs"), dir)
	if err != nil {
		t.Fatalf("NewBlob: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	for _, name := range []string{
		"/etc/ze/router.conf",
		"/etc/ze/router.conf.change.alice",
		"/etc/ze/router.conf.change.bob",
	} {
		if err := st.WriteFile(name, []byte("x"), 0); err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}
	}

	got, err := st.List("/etc/ze")
	if err != nil {
		t.Fatalf("List(\"/etc/ze\"): %v", err)
	}
	want := []string{
		"file/active/router.conf",
		"file/active/router.conf.change.alice",
		"file/active/router.conf.change.bob",
	}
	if len(got) != len(want) {
		t.Fatalf("List(\"/etc/ze\"): got %d entries %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("List(\"/etc/ze\")[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// VALIDATES: every List prefix shape resolves to a blob directory key.
// PREVENTS: a List prefix resolving to a FILE key, which ReadDir answers with
// fs.ErrNotExist, so the caller reads an empty directory as "no files".
func TestResolveDirKey(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"absolute directory", "/etc/ze", "file/active"},
		{"bare filename", "router.conf", "file/active"},
		{"absolute file path", "/etc/ze/router.conf", "file/active"},
		{"namespaced active dir", "file/active", "file/active"},
		{"namespaced draft dir", "file/draft", "file/draft"},
		{"namespaced meta dir", "meta/ssh", "meta/ssh"},
		{"leading slash namespaced", "/file/active", "file/active"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveDirKey(tt.input); got != tt.want {
				t.Errorf("resolveDirKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
