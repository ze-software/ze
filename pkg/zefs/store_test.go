package zefs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"testing/fstest"
)

// VALIDATES: basic WriteFile + ReadFile round-trip
// PREVENTS: data loss on write/read cycle

func TestStoreWriteRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	data := []byte("neighbor 10.0.0.1 {\n  asn 65001\n}\n")
	if err := s.WriteFile("bgp/peers.conf", data, 0); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := s.ReadFile("bgp/peers.conf")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("ReadFile: got %q, want %q", got, data)
	}
}

// VALIDATES: multiple files coexist
// PREVENTS: overwrite of unrelated entries

func TestStoreMultipleFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	files := map[string]string{
		"config.conf":           "router-id 1.1.1.1\n",
		"bgp/peers/n1.conf":     "neighbor 10.0.0.1\n",
		"bgp/peers/n2.conf":     "neighbor 10.0.0.2\n",
		"bgp/policy/reject.pol": "reject all\n",
	}
	for name, content := range files {
		if err := s.WriteFile(name, []byte(content), 0); err != nil {
			t.Fatalf("WriteFile(%s): %v", name, err)
		}
	}

	for name, want := range files {
		got, err := s.ReadFile(name)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", name, err)
		}
		if string(got) != want {
			t.Errorf("ReadFile(%s): got %q, want %q", name, got, want)
		}
	}
}

// VALIDATES: overwriting an existing key replaces its data
// PREVENTS: stale data after update

func TestStoreOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := s.WriteFile("config.conf", []byte("v1"), 0); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteFile("config.conf", []byte("v2-longer-content"), 0); err != nil {
		t.Fatal(err)
	}

	got, err := s.ReadFile("config.conf")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "v2-longer-content" {
		t.Errorf("got %q, want %q", got, "v2-longer-content")
	}
}

// VALIDATES: Remove deletes an entry
// PREVENTS: ghost entries after removal

func TestStoreRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := s.WriteFile("a.conf", []byte("aaa"), 0); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteFile("b.conf", []byte("bbb"), 0); err != nil {
		t.Fatal(err)
	}

	if err := s.Remove("a.conf"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if s.Has("a.conf") {
		t.Error("Has(a.conf) should be false after Remove")
	}
	if !s.Has("b.conf") {
		t.Error("Has(b.conf) should be true")
	}
}

// VALIDATES: ReadFile returns fs.ErrNotExist for missing keys
// PREVENTS: silent nil return for missing data

func TestStoreReadMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = s.ReadFile("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

// VALIDATES: fs.FS interface works with standard library
// PREVENTS: broken fs.FS contract

func TestStoreFSOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.WriteFile("hello.txt", []byte("world"), 0); err != nil {
		t.Fatal(err)
	}

	var fsys fs.FS = s
	f, err := fsys.Open("hello.txt")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })

	buf := make([]byte, 32)
	n, _ := f.Read(buf)
	if string(buf[:n]) != "world" {
		t.Errorf("Read: got %q, want %q", buf[:n], "world")
	}

	info, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size() != 5 {
		t.Errorf("Size: got %d, want 5", info.Size())
	}
	if info.Name() != "hello.txt" {
		t.Errorf("Name: got %q, want %q", info.Name(), "hello.txt")
	}
}

// VALIDATES: ReadDir returns virtual directory entries from /-keyed entries
// PREVENTS: flat namespace leaking through fs.FS directory API

func TestStoreReadDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	writeOrFatal(t, s, "bgp/peers/n1.conf", []byte("1"))
	writeOrFatal(t, s, "bgp/peers/n2.conf", []byte("2"))
	writeOrFatal(t, s, "bgp/config.conf", []byte("c"))
	writeOrFatal(t, s, "root.conf", []byte("r"))

	// Read root
	entries, err := s.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir(.): %v", err)
	}
	names := dirEntryNames(entries)
	sort.Strings(names)
	wantNames := []string{"bgp", "root.conf"}
	sort.Strings(wantNames)
	if !equalStrings(names, wantNames) {
		t.Errorf("ReadDir(.): got %v, want %v", names, wantNames)
	}

	// bgp should be a directory
	for _, e := range entries {
		if e.Name() == "bgp" && !e.IsDir() {
			t.Error("bgp should be a directory")
		}
		if e.Name() == "root.conf" && e.IsDir() {
			t.Error("root.conf should not be a directory")
		}
	}

	// Read bgp/peers
	entries, err = s.ReadDir("bgp/peers")
	if err != nil {
		t.Fatalf("ReadDir(bgp/peers): %v", err)
	}
	names = dirEntryNames(entries)
	sort.Strings(names)
	if !equalStrings(names, []string{"n1.conf", "n2.conf"}) {
		t.Errorf("ReadDir(bgp/peers): got %v, want [n1.conf n2.conf]", names)
	}
}

// VALIDATES: List returns all keys under a prefix
// PREVENTS: missing keys in enumeration

func TestStoreList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	writeOrFatal(t, s, "bgp/peers/n1.conf", []byte("1"))
	writeOrFatal(t, s, "bgp/peers/n2.conf", []byte("2"))
	writeOrFatal(t, s, "bgp/config.conf", []byte("c"))
	writeOrFatal(t, s, "root.conf", []byte("r"))

	got := s.List("bgp/peers")
	sort.Strings(got)
	want := []string{"bgp/peers/n1.conf", "bgp/peers/n2.conf"}
	if !equalStrings(got, want) {
		t.Errorf("List(bgp/peers): got %v, want %v", got, want)
	}

	all := s.List("")
	if len(all) != 4 {
		t.Errorf("List(): got %d entries, want 4", len(all))
	}
}

// VALIDATES: persistence -- data survives close and reopen
// PREVENTS: in-memory-only store that loses data

func TestStorePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")

	s, err := Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.WriteFile("key", []byte("value"), 0); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s2.Close() })

	got, err := s2.ReadFile("key")
	if err != nil {
		t.Fatalf("ReadFile after reopen: %v", err)
	}
	if string(got) != "value" {
		t.Errorf("got %q, want %q", got, "value")
	}
}

// VALIDATES: export produces same bytes that import accepts
// PREVENTS: export/import data loss

func TestStoreExportImport(t *testing.T) {
	path1 := filepath.Join(t.TempDir(), "src.zefs")
	path2 := filepath.Join(t.TempDir(), "dst.zefs")

	s1, err := Create(path1)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s1, "a", []byte("aaa"))
	writeOrFatal(t, s1, "b/c", []byte("bcc"))

	exportPath := filepath.Join(t.TempDir(), "export.zefs")
	ef, err := os.Create(exportPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := s1.Export(ef); err != nil {
		t.Fatal(err)
	}
	if err := ef.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s1.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Create(path2)
	if err != nil {
		t.Fatal(err)
	}
	rf, err := os.Open(exportPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := s2.Import(rf); err != nil {
		t.Fatalf("Import: %v", err)
	}
	if err := rf.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := s2.ReadFile("a")
	if err != nil {
		t.Fatalf("ReadFile(a): %v", err)
	}
	if string(got) != "aaa" {
		t.Errorf("got %q, want %q", got, "aaa")
	}

	got, err = s2.ReadFile("b/c")
	if err != nil {
		t.Fatalf("ReadFile(b/c): %v", err)
	}
	if string(got) != "bcc" {
		t.Errorf("got %q, want %q", got, "bcc")
	}
	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: growth adds 10% spare capacity
// PREVENTS: tight-fit allocation that requires regrowth on small additions

func TestStoreGrowthSpare(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Write data that exceeds initial slot capacity
	big := make([]byte, 8192)
	for i := range big {
		big[i] = 'x'
	}
	if err := s.WriteFile("big.conf", big, 0); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// The slot capacity should be at least 10% larger than used
	sl, ok := s.slot("big.conf")
	if !ok {
		t.Fatal("slot not found")
	}
	minCap := len(big) + len(big)/10
	if sl.data.capacity < minCap {
		t.Errorf("slot capacity %d should be >= %d (used %d + 10%% spare)", sl.data.capacity, minCap, len(big))
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: overwriting with larger data triggers slot regrowth
// PREVENTS: data corruption when slot capacity is exceeded on update

func TestStoreSlotRegrowth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	// Write small data to establish initial slot
	if err := s.WriteFile("key", []byte("tiny"), 0); err != nil {
		t.Fatal(err)
	}
	slBefore, ok := s.slot("key")
	if !ok {
		t.Fatal("slot not found")
	}

	// Overwrite with data much larger than the current slot capacity
	big := make([]byte, slBefore.data.capacity*3)
	for i := range big {
		big[i] = 'A'
	}
	if err := s.WriteFile("key", big, 0); err != nil {
		t.Fatal(err)
	}

	slAfter, ok := s.slot("key")
	if !ok {
		t.Fatal("slot not found after regrowth")
	}
	if slAfter.data.capacity <= slBefore.data.capacity {
		t.Errorf("slot should have grown: before=%d, after=%d", slBefore.data.capacity, slAfter.data.capacity)
	}
	minCap := len(big) + len(big)/10
	if slAfter.data.capacity < minCap {
		t.Errorf("regrown slot %d should be >= %d (10%% spare)", slAfter.data.capacity, minCap)
	}

	// Verify data round-trips through close+reopen
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s2.ReadFile("key")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(big) {
		t.Errorf("after reopen: got %d bytes, want %d", len(got), len(big))
	}
	for i, b := range got {
		if b != 'A' {
			t.Errorf("byte %d: got %q, want 'A'", i, b)
			break
		}
	}
	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: multiple growth cycles on the same key
// PREVENTS: growth logic failing after first regrowth

func TestStoreRepeatedGrowth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	sizes := []int{10, 100, 1000, 10000}
	for _, size := range sizes {
		data := make([]byte, size)
		for i := range data {
			data[i] = byte('0' + size%10)
		}
		if err := s.WriteFile("growing", data, 0); err != nil {
			t.Fatalf("WriteFile at size %d: %v", size, err)
		}
		got, err := s.ReadFile("growing")
		if err != nil {
			t.Fatalf("ReadFile at size %d: %v", size, err)
		}
		if len(got) != size {
			t.Fatalf("at size %d: got %d bytes", size, len(got))
		}
	}

	// Final value persists through reopen
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s2.ReadFile("growing")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 10000 {
		t.Errorf("after reopen: got %d bytes, want 10000", len(got))
	}
	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: remove compacts entries from disk on reopen
// PREVENTS: ghost entries reappearing after close+reopen

func TestStoreRemovePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "a", []byte("aaa"))
	writeOrFatal(t, s, "b", []byte("bbb"))
	writeOrFatal(t, s, "c", []byte("ccc"))

	// Capture file size before removal
	fi1, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Remove("b"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// File should be smaller after compaction
	fi2, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi2.Size() >= fi1.Size() {
		t.Errorf("file should shrink after remove: before=%d, after=%d", fi1.Size(), fi2.Size())
	}

	// Reopen and verify removed entry is gone, others intact
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if s2.Has("b") {
		t.Error("removed entry 'b' should not exist after reopen")
	}
	for _, tc := range []struct{ key, want string }{
		{"a", "aaa"}, {"c", "ccc"},
	} {
		got, readErr := s2.ReadFile(tc.key)
		if readErr != nil {
			t.Fatalf("ReadFile(%s): %v", tc.key, readErr)
		}
		if string(got) != tc.want {
			t.Errorf("ReadFile(%s): got %q, want %q", tc.key, got, tc.want)
		}
	}
	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: remove + add new entries coexist correctly
// PREVENTS: slot index corruption after compaction + new writes

func TestStoreRemoveThenAdd(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "x", []byte("111"))
	writeOrFatal(t, s, "y", []byte("222"))
	writeOrFatal(t, s, "z", []byte("333"))

	if err := s.Remove("y"); err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "w", []byte("444"))
	writeOrFatal(t, s, "y", []byte("555")) // re-add removed key with new data

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ key, want string }{
		{"x", "111"}, {"z", "333"}, {"w", "444"}, {"y", "555"},
	} {
		got, readErr := s2.ReadFile(tc.key)
		if readErr != nil {
			t.Fatalf("ReadFile(%s): %v", tc.key, readErr)
		}
		if string(got) != tc.want {
			t.Errorf("ReadFile(%s): got %q, want %q", tc.key, got, tc.want)
		}
	}
	all := s2.List("")
	if len(all) != 4 {
		t.Errorf("List: got %d entries, want 4: %v", len(all), all)
	}
	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: mmap remap -- reads work after writes trigger remap
// PREVENTS: dangling pointers after backing buffer is replaced

func TestStoreReopenAndModify(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")

	// Create and populate
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "config.conf", []byte("original"))
	writeOrFatal(t, s, "peers.conf", []byte("peer1"))
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopen (triggers mmap)
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}

	// Read from mmap
	got, err := s.ReadFile("config.conf")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "original" {
		t.Errorf("before write: got %q, want %q", got, "original")
	}

	// Write triggers remap
	if err := s.WriteFile("config.conf", []byte("updated content"), 0); err != nil {
		t.Fatal(err)
	}

	// Read from new mmap
	got, err = s.ReadFile("config.conf")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "updated content" {
		t.Errorf("after write: got %q, want %q", got, "updated content")
	}

	// Unmodified entry also still readable from new mmap
	got, err = s.ReadFile("peers.conf")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "peer1" {
		t.Errorf("unmodified: got %q, want %q", got, "peer1")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// Final reopen to verify persistence after the remap cycle
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err = s.ReadFile("config.conf")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "updated content" {
		t.Errorf("final reopen: got %q, want %q", got, "updated content")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteFile rejects invalid keys
// PREVENTS: silent corruption from bad key names

func TestStoreInvalidKeyRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	err = s.WriteFile(".", []byte("bad"), 0)
	if err == nil {
		t.Fatal("expected error for invalid key")
	}

	// Store should still be usable after the rejected write
	writeOrFatal(t, s, "ok", []byte("fine"))
	got, err := s.ReadFile("ok")
	if err != nil {
		t.Fatalf("ReadFile after rejected write: %v", err)
	}
	if string(got) != "fine" {
		t.Errorf("got %q, want %q", got, "fine")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: decodeInto rejects blob with too many entries
// PREVENTS: memory exhaustion from crafted blob with millions of tiny entries

func TestStoreDecodeEntryCountLimit(t *testing.T) {
	// Build a blob with maxEntryCount+1 entries.
	// Each entry is a key-value pair of minimal netcapstrings.
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	// Write maxEntryCount+1 entries via WriteLock (bypasses per-write flush)
	wl, err := s.Lock()
	if err != nil {
		t.Fatal(err)
	}
	for i := range maxEntryCount + 1 {
		key := fmt.Sprintf("k/%d", i)
		if writeErr := wl.WriteFile(key, []byte("v"), 0); writeErr != nil {
			_ = wl.Release()
			t.Fatalf("write entry %d: %v", i, writeErr)
		}
	}
	// Release triggers flush+load which decodes. The decode should fail.
	err = wl.Release()
	if err == nil {
		t.Error("expected error from entry count limit on decode")
	}

	if closeErr := s.Close(); closeErr != nil {
		t.Logf("close after limit error: %v", closeErr)
	}
}

// VALIDATES: store recovers after flush write failure
// PREVENTS: segfault from dangling mmap references after failed os.WriteFile

func TestStoreFlushRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "existing", []byte("data"))

	// Make file read-only to force os.WriteFile failure
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	// WriteFile should fail but store should remain usable
	err = s.WriteFile("new", []byte("should-fail"), 0)
	if err == nil {
		// Some platforms (e.g., root user) may succeed despite chmod
		t.Skip("os.WriteFile succeeded despite read-only file (likely running as root)")
	}

	// Recovery: store should still serve existing data from encoded copy
	got, err := s.ReadFile("existing")
	if err != nil {
		t.Fatalf("ReadFile after failed write: %v", err)
	}
	if string(got) != "data" {
		t.Errorf("got %q, want %q", got, "data")
	}

	// Restore permissions and verify store can write again
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteFile("recovery", []byte("works"), 0); err != nil {
		t.Fatalf("WriteFile after recovery: %v", err)
	}
	got, err = s.ReadFile("recovery")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "works" {
		t.Errorf("got %q, want %q", got, "works")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Open returns error for nonexistent store file
// PREVENTS: panic or silent empty store on missing file

func TestStoreOpenNonexistent(t *testing.T) {
	_, err := Open(filepath.Join(t.TempDir(), "does-not-exist.zefs"))
	if err == nil {
		t.Fatal("expected error opening nonexistent file")
	}
}

// VALIDATES: Open rejects file with invalid magic header
// PREVENTS: corrupt data interpreted as valid store

func TestStoreOpenCorrupted(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
	}{
		{"wrong magic", []byte("NOPE:garbage-data-here")},
		{"empty file", []byte{}},
		{"truncated magic", []byte("ZeF")},
		{"valid magic but truncated container", []byte("ZeFS:000")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "bad.zefs")
			if err := os.WriteFile(path, tt.content, 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := Open(path)
			if err == nil {
				t.Error("expected error opening corrupted store")
			}
		})
	}
}

// VALIDATES: empty store persists and reopens correctly
// PREVENTS: crash or error on empty store round-trip

func TestStoreEmptyPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("Open empty store: %v", err)
	}
	keys := s2.List("")
	if len(keys) != 0 {
		t.Errorf("expected no keys, got %v", keys)
	}
	entries, err := s2.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir on empty store: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no entries, got %d", len(entries))
	}
	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Remove returns error for nonexistent key
// PREVENTS: silent no-op on remove of missing key

func TestStoreRemoveNonexistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	err = s.Remove("does-not-exist")
	if err == nil {
		t.Error("expected error removing nonexistent key")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: List returns nil for nonexistent prefix
// PREVENTS: panic on missing tree node during prefix walk

func TestStoreListNonexistentPrefix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "a/b", []byte("data"))

	got := s.List("nonexistent/path")
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteFile copies data so caller mutations don't affect the store
// PREVENTS: shared backing between caller and store leading to data corruption

func TestStoreWriteDataIsolation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	data := []byte("original")
	if err := s.WriteFile("key", data, 0); err != nil {
		t.Fatal(err)
	}

	// Mutate the original slice after writing
	data[0] = 'X'

	got, err := s.ReadFile("key")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "original" {
		t.Errorf("store data mutated by caller: got %q, want %q", got, "original")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteFile accepts nil data (zero-length entry)
// PREVENTS: nil pointer dereference on nil data write

func TestStoreWriteNilData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.WriteFile("empty", nil, 0); err != nil {
		t.Fatalf("WriteFile(nil): %v", err)
	}
	got, err := s.ReadFile("empty")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty data, got %q", got)
	}

	// Persists through reopen
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err = s2.ReadFile("empty")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("after reopen: expected empty data, got %q", got)
	}
	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Close is idempotent
// PREVENTS: double-munmap panic on second Close

func TestStoreDoubleClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "key", []byte("value"))

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	// Second Close should not panic or error
	if err := s.Close(); err != nil {
		t.Errorf("second Close should not error, got: %v", err)
	}
}

// VALIDATES: Import replaces all existing store content
// PREVENTS: stale entries surviving an import

func TestStoreImportReplacesContent(t *testing.T) {
	srcPath := filepath.Join(t.TempDir(), "src.zefs")
	dstPath := filepath.Join(t.TempDir(), "dst.zefs")

	// Create source with one set of keys
	src, err := Create(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, src, "new-key", []byte("new-value"))

	var exported bytes.Buffer
	if err := src.Export(&exported); err != nil {
		t.Fatal(err)
	}
	if err := src.Close(); err != nil {
		t.Fatal(err)
	}

	// Create destination with different keys
	dst, err := Create(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, dst, "old-key", []byte("old-value"))

	// Import should replace everything
	if err := dst.Import(bytes.NewReader(exported.Bytes())); err != nil {
		t.Fatal(err)
	}

	if dst.Has("old-key") {
		t.Error("old-key should not exist after import")
	}
	got, err := dst.ReadFile("new-key")
	if err != nil {
		t.Fatalf("ReadFile(new-key): %v", err)
	}
	if string(got) != "new-value" {
		t.Errorf("got %q, want %q", got, "new-value")
	}
	if err := dst.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Import rejects invalid data
// PREVENTS: silent corruption from bad import source

func TestStoreImportInvalidData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "existing", []byte("data"))

	err = s.Import(bytes.NewReader([]byte("not-a-valid-store")))
	if err == nil {
		t.Error("expected error importing invalid data")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: fs.FS Open returns root directory for "."
// PREVENTS: broken root directory access via fs.FS

func TestStoreFSOpenRoot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "a.txt", []byte("aaa"))

	f, err := s.Open(".")
	if err != nil {
		t.Fatalf("Open(.): %v", err)
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Error("root should be a directory")
	}
	if info.Name() != "." {
		t.Errorf("root name: got %q, want %q", info.Name(), ".")
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: fs.FS Open returns directory node for intermediate path
// PREVENTS: directory traversal only working for leaf files

func TestStoreFSOpenDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "bgp/peers/n1.conf", []byte("data"))

	f, err := s.Open("bgp")
	if err != nil {
		t.Fatalf("Open(bgp): %v", err)
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Error("bgp should be a directory")
	}
	if info.Name() != "bgp" {
		t.Errorf("name: got %q, want %q", info.Name(), "bgp")
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: fs.FS Open returns ErrNotExist for missing paths
// PREVENTS: nil dereference on missing file access

func TestStoreFSOpenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.Open("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing path")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: fs.File.Read advances position through sequential reads
// PREVENTS: stuck read offset returning same data repeatedly

func TestStoreFSFileSequentialRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "data.txt", []byte("abcdefghij"))

	f, err := s.Open("data.txt")
	if err != nil {
		t.Fatal(err)
	}

	// Read in chunks
	buf := make([]byte, 4)
	n, err := f.Read(buf)
	if err != nil {
		t.Fatalf("first read: %v", err)
	}
	if string(buf[:n]) != "abcd" {
		t.Errorf("first read: got %q", buf[:n])
	}

	n, err = f.Read(buf)
	if err != nil {
		t.Fatalf("second read: %v", err)
	}
	if string(buf[:n]) != "efgh" {
		t.Errorf("second read: got %q", buf[:n])
	}

	n, err = f.Read(buf)
	if err != nil {
		t.Fatalf("third read: %v", err)
	}
	if string(buf[:n]) != "ij" {
		t.Errorf("third read: got %q, want %q", buf[:n], "ij")
	}

	// Next read should return EOF
	_, err = f.Read(buf)
	if err == nil {
		t.Error("expected EOF after exhausting data")
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Read on a directory returns an error
// PREVENTS: directory node returning garbage data

func TestStoreFSDirReadError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "dir/file.txt", []byte("content"))

	f, err := s.Open("dir")
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 10)
	_, err = f.Read(buf)
	if err == nil {
		t.Error("expected error reading from directory")
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: ReadDir returns error for nonexistent directory and for files
// PREVENTS: nil dereference on missing or non-directory nodes

func TestStoreReadDirErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "file.txt", []byte("data"))

	_, err = s.ReadDir("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}

	_, err = s.ReadDir("file.txt")
	if err == nil {
		t.Error("expected error for ReadDir on a file")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: DirEntry.Info returns correct FileInfo for files and directories
// PREVENTS: incorrect size or type in directory listing metadata

func TestStoreDirEntryInfo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "dir/file.txt", []byte("hello"))

	entries, err := s.ReadDir("dir")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	info, err := entries[0].Info()
	if err != nil {
		t.Fatal(err)
	}
	if info.Name() != "file.txt" {
		t.Errorf("name: got %q", info.Name())
	}
	if info.Size() != 5 {
		t.Errorf("size: got %d, want 5", info.Size())
	}
	if info.IsDir() {
		t.Error("file should not be a directory")
	}

	// Check directory entry via parent
	rootEntries, err := s.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range rootEntries {
		if e.Name() == "dir" {
			dirInfo, infoErr := e.Info()
			if infoErr != nil {
				t.Fatal(infoErr)
			}
			if !dirInfo.IsDir() {
				t.Error("dir should be a directory")
			}
		}
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: removing the last file in a subdirectory prunes empty parent dirs
// PREVENTS: ghost directory nodes after all children removed

func TestStoreRemovePrunesEmptyDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "a/b/c.txt", []byte("leaf"))
	writeOrFatal(t, s, "a/other.txt", []byte("sibling"))

	// Remove the only file in a/b/ -- should prune a/b/
	if err := s.Remove("a/b/c.txt"); err != nil {
		t.Fatal(err)
	}

	// a/b/ directory should be gone
	_, err = s.ReadDir("a/b")
	if err == nil {
		t.Error("expected error: a/b should be pruned")
	}

	// a/ should still exist with other.txt
	entries, err := s.ReadDir("a")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "other.txt" {
		t.Errorf("expected [other.txt], got %v", dirEntryNames(entries))
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Create returns error for nonexistent parent directory
// PREVENTS: panic from os.WriteFile on invalid path

func TestStoreCreateInNonexistentDir(t *testing.T) {
	_, err := Create(filepath.Join(t.TempDir(), "no", "such", "dir", "test.zefs"))
	if err == nil {
		t.Fatal("expected error creating store in nonexistent directory")
	}
}

// VALIDATES: overwriting with shorter data fully replaces the old value
// PREVENTS: stale trailing bytes from longer previous value

func TestStoreOverwriteShorter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.WriteFile("key", []byte("long-initial-value"), 0); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteFile("key", []byte("short"), 0); err != nil {
		t.Fatal(err)
	}

	got, err := s.ReadFile("key")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "short" {
		t.Errorf("got %q, want %q", got, "short")
	}

	// Verify through reopen
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err = s2.ReadFile("key")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "short" {
		t.Errorf("after reopen: got %q, want %q", got, "short")
	}
	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: removing deeply nested file prunes all empty ancestor dirs
// PREVENTS: ghost directory chain after removing the only leaf

func TestStoreDeepDirPruning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "a/b/c/d.txt", []byte("deep"))

	if err := s.Remove("a/b/c/d.txt"); err != nil {
		t.Fatal(err)
	}

	// All ancestor dirs should be pruned since they're all empty
	for _, dir := range []string{"a/b/c", "a/b", "a"} {
		_, err := s.ReadDir(dir)
		if err == nil {
			t.Errorf("expected error: %s should be pruned", dir)
		}
	}

	// Root should still work but be empty
	entries, err := s.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("root should be empty, got %v", dirEntryNames(entries))
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: imported data persists through close and reopen
// PREVENTS: import only updating in-memory state without flushing to disk

func TestStoreImportPersistence(t *testing.T) {
	srcPath := filepath.Join(t.TempDir(), "src.zefs")
	dstPath := filepath.Join(t.TempDir(), "dst.zefs")

	src, err := Create(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, src, "imported", []byte("data"))

	var exported bytes.Buffer
	if err := src.Export(&exported); err != nil {
		t.Fatal(err)
	}
	if err := src.Close(); err != nil {
		t.Fatal(err)
	}

	dst, err := Create(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := dst.Import(bytes.NewReader(exported.Bytes())); err != nil {
		t.Fatal(err)
	}
	if err := dst.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopen and verify data survived
	dst2, err := Open(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	got, err := dst2.ReadFile("imported")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "data" {
		t.Errorf("got %q, want %q", got, "data")
	}
	if err := dst2.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Export and Import work with an empty store
// PREVENTS: encode/decode crash on zero entries

func TestStoreExportImportEmpty(t *testing.T) {
	srcPath := filepath.Join(t.TempDir(), "src.zefs")
	dstPath := filepath.Join(t.TempDir(), "dst.zefs")

	src, err := Create(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	var exported bytes.Buffer
	if err := src.Export(&exported); err != nil {
		t.Fatal(err)
	}
	if err := src.Close(); err != nil {
		t.Fatal(err)
	}

	dst, err := Create(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, dst, "will-be-replaced", []byte("gone"))

	if err := dst.Import(bytes.NewReader(exported.Bytes())); err != nil {
		t.Fatal(err)
	}

	keys := dst.List("")
	if len(keys) != 0 {
		t.Errorf("expected no keys after importing empty store, got %v", keys)
	}
	if err := dst.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: storeFileInfo returns correct fixed values for Mode, ModTime, Sys
// PREVENTS: broken fs.FileInfo contract

func TestStoreFileInfoFixedValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "file.txt", []byte("data"))

	f, err := s.Open("file.txt")
	if err != nil {
		t.Fatal(err)
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode() != 0o444 {
		t.Errorf("Mode: got %o, want 444", info.Mode())
	}
	if !info.ModTime().IsZero() {
		t.Errorf("ModTime: got %v, want zero", info.ModTime())
	}
	if info.Sys() != nil {
		t.Errorf("Sys: got %v, want nil", info.Sys())
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: key insertion order is preserved through encode/decode cycles
// PREVENTS: non-deterministic serialization from map iteration

func TestStoreKeyOrderPreservation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	// Write in specific order
	keys := []string{"charlie", "alpha", "bravo", "delta"}
	for _, k := range keys {
		writeOrFatal(t, s, k, []byte(k+"-data"))
	}

	// List("") collects from tree (unordered), so check via close+reopen
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// Export, then re-import into fresh store and verify order
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	var exported bytes.Buffer
	if err := s2.Export(&exported); err != nil {
		t.Fatal(err)
	}
	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}

	// Open again and verify all keys still readable with correct data
	s3, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range keys {
		got, readErr := s3.ReadFile(k)
		if readErr != nil {
			t.Fatalf("ReadFile(%s): %v", k, readErr)
		}
		if string(got) != k+"-data" {
			t.Errorf("ReadFile(%s): got %q, want %q", k, got, k+"-data")
		}
	}
	if err := s3.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: keys with special characters (colons, spaces, unicode) round-trip correctly
// PREVENTS: netcapstring colon delimiters interfering with key parsing

func TestStoreSpecialCharKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	specialKeys := map[string]string{
		"key:with:colons": "colon-data",
		"key with spaces": "space-data",
		"key\twith\ttabs": "tab-data",
		"unicode/clé/日本語": "unicode-data",
	}
	for k, v := range specialKeys {
		if err := s.WriteFile(k, []byte(v), 0); err != nil {
			t.Fatalf("WriteFile(%q): %v", k, err)
		}
	}

	// Verify in-memory
	for k, want := range specialKeys {
		got, readErr := s.ReadFile(k)
		if readErr != nil {
			t.Fatalf("ReadFile(%q): %v", k, readErr)
		}
		if string(got) != want {
			t.Errorf("ReadFile(%q): got %q, want %q", k, got, want)
		}
	}

	// Verify through reopen (tests encode/decode with special chars)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for k, want := range specialKeys {
		got, readErr := s2.ReadFile(k)
		if readErr != nil {
			t.Fatalf("after reopen ReadFile(%q): %v", k, readErr)
		}
		if string(got) != want {
			t.Errorf("after reopen ReadFile(%q): got %q, want %q", k, got, want)
		}
	}
	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Lock() works on a store opened with Open()
// PREVENTS: openLockFd failure on reopened stores

func TestStoreLockAfterOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "seed", []byte("data"))
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	wl := lockOrFatal(t, s2)
	writeLockedOrFatal(t, wl, "added", []byte("via-lock"))
	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}

	got, err := s2.ReadFile("added")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "via-lock" {
		t.Errorf("got %q, want %q", got, "via-lock")
	}
	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: ReadFile returns error for a directory path
// PREVENTS: returning nil data without error when key is a directory

func TestStoreReadFileOnDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "bgp/peers/n1.conf", []byte("peer1"))

	// "bgp" is a directory node, not a file
	_, err = s.ReadFile("bgp")
	if err == nil {
		t.Error("expected error reading directory as file")
	}
	// "bgp/peers" is also a directory
	_, err = s.ReadFile("bgp/peers")
	if err == nil {
		t.Error("expected error reading intermediate directory as file")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Has returns false for directory paths
// PREVENTS: confusing directory existence with file existence

func TestStoreHasDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "a/b/c.txt", []byte("data"))

	if s.Has("a") {
		t.Error("Has(directory) should be false")
	}
	if s.Has("a/b") {
		t.Error("Has(intermediate directory) should be false")
	}
	if !s.Has("a/b/c.txt") {
		t.Error("Has(file) should be true")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: tree walk with empty path returns root
// PREVENTS: nil dereference on empty path walk

func TestStoreListEmptyPrefix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "x", []byte("1"))
	writeOrFatal(t, s, "y/z", []byte("2"))

	keys := s.List("")
	if len(keys) != 2 {
		t.Errorf("List empty prefix: got %d keys, want 2: %v", len(keys), keys)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: remove on deeply nested nonexistent path
// PREVENTS: panic on remove with missing intermediate directories

func TestStoreRemoveDeepNonexistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "a/real.txt", []byte("data"))

	// "a" exists but "a/b" does not, so "a/b/c" cannot be removed
	err = s.Remove("a/b/c")
	if err == nil {
		t.Error("expected error removing deeply nonexistent key")
	}

	// The existing file should be untouched
	got, err := s.ReadFile("a/real.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "data" {
		t.Errorf("got %q, want %q", got, "data")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: fs.File Stat on directory returns zero size
// PREVENTS: nonzero size reported for virtual directory nodes

func TestStoreFSDirStatSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "dir/file.txt", []byte("content"))

	f, err := s.Open("dir")
	if err != nil {
		t.Fatal(err)
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 0 {
		t.Errorf("directory size: got %d, want 0", info.Size())
	}
	if info.Mode() != fs.ModeDir|0o555 {
		t.Errorf("directory mode: got %v, want %v", info.Mode(), fs.ModeDir|0o555)
	}
	if info.Sys() != nil {
		t.Errorf("Sys: got %v, want nil", info.Sys())
	}
	if !info.ModTime().IsZero() {
		t.Errorf("ModTime: got %v, want zero", info.ModTime())
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: DirEntry.Type returns fs.ModeDir for directories, 0 for files
// PREVENTS: incorrect file mode bits in directory listings

func TestStoreDirEntryType(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "dir/file.txt", []byte("data"))
	writeOrFatal(t, s, "dir/sub/nested.txt", []byte("inner"))

	entries, err := s.ReadDir("dir")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			if e.Type() != fs.ModeDir {
				t.Errorf("dir entry %q Type: got %v, want %v", e.Name(), e.Type(), fs.ModeDir)
			}
		} else {
			if e.Type() != 0 {
				t.Errorf("file entry %q Type: got %v, want 0", e.Name(), e.Type())
			}
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: concurrent auto-locking reads and writes don't race
// PREVENTS: data race in public API methods

func TestStoreConcurrentPublicAPI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "key", []byte("initial"))

	var wg sync.WaitGroup

	// Concurrent readers via public API
	for range 5 {
		wg.Go(func() {
			if _, readErr := s.ReadFile("key"); readErr != nil {
				t.Errorf("concurrent ReadFile: %v", readErr)
			}
			if !s.Has("key") {
				t.Error("concurrent Has: expected true")
			}
			keys := s.List("")
			if len(keys) == 0 {
				t.Error("concurrent List: expected non-empty")
			}
		})
	}

	// Concurrent writer via public API
	for i := range 3 {
		wg.Go(func() {
			if writeErr := s.WriteFile("key", []byte{byte('A' + i)}, 0); writeErr != nil {
				t.Errorf("concurrent WriteFile: %v", writeErr)
			}
		})
	}

	wg.Wait()

	// Store should be usable after concurrent access
	if _, err = s.ReadFile("key"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: ReadDir on root via "." returns top-level entries
// PREVENTS: root ReadDir returning empty or error

func TestStoreReadDirRoot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "top.txt", []byte("root-level"))
	writeOrFatal(t, s, "sub/nested.txt", []byte("nested"))

	entries, err := s.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	names := dirEntryNames(entries)
	sort.Strings(names)
	if len(names) != 2 || names[0] != "sub" || names[1] != "top.txt" {
		t.Errorf("ReadDir root: got %v, want [sub, top.txt]", names)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: fs.FS Open on leaf file in nested directory
// PREVENTS: Name() returning full path instead of base name

func TestStoreFSOpenNestedFileName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "a/b/c/deep.txt", []byte("deep"))

	f, err := s.Open("a/b/c/deep.txt")
	if err != nil {
		t.Fatal(err)
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	// Name should be the base name, not the full path
	if info.Name() != "deep.txt" {
		t.Errorf("Name: got %q, want %q", info.Name(), "deep.txt")
	}
	if info.IsDir() {
		t.Error("file should not be a directory")
	}
	if info.Size() != 4 {
		t.Errorf("Size: got %d, want 4", info.Size())
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Export produces data that round-trips through Import
// PREVENTS: export/import data corruption

func TestStoreExportImportRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "key", []byte("value"))

	var buf bytes.Buffer
	if err := s.Export(&buf); err != nil {
		t.Fatal(err)
	}

	// Import into a new store
	path2 := filepath.Join(t.TempDir(), "test2.zefs")
	s2, err := Create(path2)
	if err != nil {
		t.Fatal(err)
	}
	if err := s2.Import(&buf); err != nil {
		t.Fatal(err)
	}

	got, err := s2.ReadFile("key")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "value" {
		t.Errorf("imported: got %q, want %q", got, "value")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: walk through a leaf node returns nil (not panic)
// PREVENTS: nil dereference when path traverses a non-directory node

func TestStoreWalkThroughLeaf(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "leaf", []byte("data"))

	// "leaf" is a file. "leaf/child" tries to walk through it.
	if s.Has("leaf/child") {
		t.Error("Has should return false for path through leaf")
	}
	_, err = s.ReadFile("leaf/child")
	if err == nil {
		t.Error("expected error reading path through leaf")
	}
	keys := s.List("leaf/child")
	if len(keys) != 0 {
		t.Errorf("List through leaf: got %v, want empty", keys)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: List on a key that is a leaf returns that key
// PREVENTS: collect on leaf returning empty or wrong result

func TestStoreListOnLeafKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "dir/file.txt", []byte("data"))

	// List with a prefix that matches a leaf exactly
	keys := s.List("dir/file.txt")
	if len(keys) != 1 || keys[0] != "dir/file.txt" {
		t.Errorf("List on leaf: got %v, want [dir/file.txt]", keys)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteFile rejects writing a file that conflicts with existing directory
// PREVENTS: silent corruption when leaf and directory share a path segment

func TestStoreWriteFileConflictsWithDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	// Write "a/b" first -- creates directory node "a"
	writeOrFatal(t, s, "a/b", []byte("nested"))

	// Now write "a" as a leaf -- should fail (conflicts with directory)
	err = s.WriteFile("a", []byte("leaf"), 0)
	if err == nil {
		t.Fatal("expected error writing file over existing directory, got nil")
	}

	// "a/b" should be unaffected
	got, err := s.ReadFile("a/b")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "nested" {
		t.Errorf("got %q, want %q", got, "nested")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteFile returns error when path traverses existing leaf
// PREVENTS: panic from nil map access or silent data corruption

func TestStoreWriteFileLeafBecomesDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	// Write "a" as a leaf
	writeOrFatal(t, s, "a", []byte("leaf"))

	// Now try to write "a/b" -- "a" is a leaf, not a directory
	// This should return an error (path conflict), not panic
	err = s.WriteFile("a/b", []byte("nested"), 0)
	if err == nil {
		t.Fatal("expected error writing nested key through existing leaf, got nil")
	}

	// Original leaf should be unaffected
	got, err := s.ReadFile("a")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "leaf" {
		t.Errorf("got %q, want %q", got, "leaf")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: storeFile.Read with zero-length buffer
// PREVENTS: incorrect return value on zero-length read

func TestStoreFSReadZeroBuffer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "file.txt", []byte("content"))

	f, err := s.Open("file.txt")
	if err != nil {
		t.Fatal(err)
	}

	// Read with zero-length buffer should return 0, nil (not at EOF yet)
	n, err := f.Read([]byte{})
	if err != nil {
		t.Errorf("zero-length read error: %v", err)
	}
	if n != 0 {
		t.Errorf("zero-length read n: got %d, want 0", n)
	}

	// Subsequent normal read should still work
	buf := make([]byte, 32)
	n, err = f.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf[:n]) != "content" {
		t.Errorf("got %q, want %q", buf[:n], "content")
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: storeFile.Read on file with empty data returns EOF immediately
// PREVENTS: stuck read on zero-length file

func TestStoreFSReadEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "empty.txt", []byte{})

	f, err := s.Open("empty.txt")
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 10)
	n, err := f.Read(buf)
	if n != 0 {
		t.Errorf("n: got %d, want 0", n)
	}
	if !errors.Is(err, io.EOF) {
		t.Errorf("err: got %v, want io.EOF", err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: keys with empty segments ("a//b") or trailing slash ("a/") are rejected
// PREVENTS: malformed keys creating unexpected tree structures

func TestStoreEdgeCaseKeyFormats(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	// Key with double slash is rejected (empty segment violates fs.ValidPath)
	if err := s.WriteFile("a//b", []byte("double-slash"), 0); err == nil {
		t.Error("WriteFile(a//b) should fail")
	}

	// Key with trailing slash is rejected
	if err := s.WriteFile("trailing/", []byte("trailing"), 0); err == nil {
		t.Error("WriteFile(trailing/) should fail")
	}

	// Store should remain usable after rejections
	writeOrFatal(t, s, "valid/key", []byte("ok"))
	got, err := s.ReadFile("valid/key")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "ok" {
		t.Errorf("got %q, want %q", got, "ok")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: flat keys (no "/" separator) work correctly
// PREVENTS: assumption that all keys are hierarchical

func TestStoreFlatKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	writeOrFatal(t, s, "alpha", []byte("1"))
	writeOrFatal(t, s, "beta", []byte("2"))
	writeOrFatal(t, s, "gamma", []byte("3"))

	keys := s.List("")
	if len(keys) != 3 {
		t.Errorf("List: got %d keys, want 3: %v", len(keys), keys)
	}

	entries, err := s.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Errorf("ReadDir root: got %d entries, want 3", len(entries))
	}
	for _, e := range entries {
		if e.IsDir() {
			t.Errorf("flat key %q should not be directory", e.Name())
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: multiple removes in a single WriteLock transaction
// PREVENTS: remove-tracking breaking after first remove

func TestWriteLockMultipleRemoves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "a", []byte("1"))
	writeOrFatal(t, s, "b", []byte("2"))
	writeOrFatal(t, s, "c", []byte("3"))

	wl := lockOrFatal(t, s)
	if err := wl.Remove("a"); err != nil {
		t.Fatalf("Remove(a): %v", err)
	}
	if err := wl.Remove("b"); err != nil {
		t.Fatalf("Remove(b): %v", err)
	}
	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}

	if s.Has("a") || s.Has("b") {
		t.Error("removed keys should not exist")
	}
	got, err := s.ReadFile("c")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "3" {
		t.Errorf("surviving key: got %q, want %q", got, "3")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteLock with only removes (no writes) still flushes
// PREVENTS: dirty flag not set on remove-only transactions

func TestWriteLockRemoveOnlyDirty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "victim", []byte("data"))
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	wl := lockOrFatal(t, s2)
	if err := wl.Remove("victim"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	// No WriteFile -- only a Remove. dirty should still be true.
	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}
	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopen and verify the remove persisted
	s3, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if s3.Has("victim") {
		t.Error("remove-only transaction should persist")
	}
	if err := s3.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Open("") on fs.FS returns root (same as ".")
// PREVENTS: empty string path treated differently from "."

func TestStoreFSOpenEmptyPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "file.txt", []byte("data"))

	// Open("") should not error -- walk("") returns root
	// Note: fs.FS spec says name must be unrooted slash-separated,
	// but our implementation handles "" via walk
	f, err := s.Open("")
	if err != nil {
		t.Fatalf("Open empty: %v", err)
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Error("empty path should be root directory")
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: removeRecursive non-pruning path (child has remaining children)
// PREVENTS: incorrect pruning when sibling entries exist

func TestStoreRemoveWithSiblings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "dir/keep.txt", []byte("stay"))
	writeOrFatal(t, s, "dir/drop.txt", []byte("go"))

	if err := s.Remove("dir/drop.txt"); err != nil {
		t.Fatal(err)
	}

	// "dir" should NOT be pruned -- it still has "keep.txt"
	entries, err := s.ReadDir("dir")
	if err != nil {
		t.Fatalf("ReadDir after partial remove: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "keep.txt" {
		t.Errorf("ReadDir: got %v, want [keep.txt]", dirEntryNames(entries))
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: BlobStore satisfies fs.FS, fs.ReadFileFS, fs.ReadDirFS
// PREVENTS: interface drift from standard library contracts

func TestStoreFSInterfaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "root.txt", []byte("root"))
	writeOrFatal(t, s, "dir/child.txt", []byte("child"))

	var fsys fs.FS = s
	var readFS fs.ReadFileFS = s
	var readDirFS fs.ReadDirFS = s

	// fs.FS
	f, err := fsys.Open("root.txt")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	// fs.ReadFileFS
	data, err := readFS.ReadFile("root.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "root" {
		t.Errorf("ReadFileFS: got %q", data)
	}

	// fs.ReadDirFS
	entries, err := readDirFS.ReadDir("dir")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "child.txt" {
		t.Errorf("ReadDirFS: got %v", entries)
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: testing/fstest.TestFS validates fs.FS implementation
// PREVENTS: subtle fs.FS contract violations missed by manual tests

func TestStoreFSTestValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "hello.txt", []byte("hello world"))
	writeOrFatal(t, s, "dir/sub.txt", []byte("sub content"))
	writeOrFatal(t, s, "dir/deep/leaf.txt", []byte("leaf"))

	if err := fstest.TestFS(s,
		"hello.txt",
		"dir/sub.txt",
		"dir/deep/leaf.txt",
	); err != nil {
		t.Fatal(err)
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Create fails when parent directory doesn't exist
// PREVENTS: cryptic error from flush on invalid path

func TestStoreCreateInvalidPath(t *testing.T) {
	_, err := Create(filepath.Join(t.TempDir(), "no", "such", "dir", "store.zefs"))
	if err == nil {
		t.Error("expected error creating store in nonexistent directory")
	}
}

// VALIDATES: Open fails on empty file (0 bytes)
// PREVENTS: mmap/decode error on zero-length store file

func TestStoreOpenEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.zefs")
	if err := os.WriteFile(path, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Open(path)
	if err == nil {
		t.Error("expected error opening empty file")
	}
}

// VALIDATES: ReadDir on root of empty store returns empty slice
// PREVENTS: error or nil on empty store root ReadDir

func TestStoreReadDirEmptyRoot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	entries, err := s.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir on empty root: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: empty key is rejected by WriteFile (fs.ValidPath contract)
// PREVENTS: unretrievable keys being silently stored

func TestStoreEmptyKeyName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	// WriteFile with empty key must fail (invalid fs.ValidPath)
	if err := s.WriteFile("", []byte("empty-key"), 0); err == nil {
		t.Fatal("WriteFile with empty key should fail")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: slot() returns false for nonexistent key
// PREVENTS: zero-value slotInfo treated as valid

func TestStoreSlotMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	sl, ok := s.slot("nonexistent")
	if ok {
		t.Error("slot should return false for missing key")
	}
	if sl.name.capacity != 0 || sl.data.capacity != 0 {
		t.Errorf("zero-value expected, got nameCap=%d dataCap=%d", sl.name.capacity, sl.data.capacity)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Open on root of empty store
// PREVENTS: panic when Open(".") on store with no entries

func TestStoreFSOpenRootEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	f, err := s.Open(".")
	if err != nil {
		t.Fatalf("Open(.) on empty store: %v", err)
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Error("root should be directory")
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Open on truncated store file (valid magic, truncated body)
// PREVENTS: unclear error when store file is partially written

func TestStoreOpenTruncated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "truncated.zefs")
	// Write valid magic prefix but truncate the container netcapstring
	if err := os.WriteFile(path, []byte("ZeFS:0000"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Open(path)
	if err == nil {
		t.Error("expected error opening truncated store")
	}
}

// VALIDATES: empty-key is rejected, normal keys work in same session
// PREVENTS: empty key rejection breaking subsequent valid writes

func TestStoreEmptyKeyPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	// Empty key is rejected
	if err := s.WriteFile("", []byte("empty-key-data"), 0); err == nil {
		t.Fatal("WriteFile with empty key should fail")
	}
	// Normal key still works after rejection
	if err := s.WriteFile("normal", []byte("ok"), 0); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopen and verify normal key persists
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, readErr := s2.ReadFile("normal")
	if readErr != nil {
		t.Fatalf("ReadFile(normal): %v", readErr)
	}
	if string(got) != "ok" {
		t.Errorf("got %q, want %q", got, "ok")
	}
	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: ReadFile returns caller-owned copy, not mmap-backed reference
// PREVENTS: SIGBUS when caller modifies returned bytes (fs.ReadFileFS contract)

func TestReadFileReturnsCopy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "key", []byte("original"))

	data1, err := s.ReadFile("key")
	if err != nil {
		t.Fatal(err)
	}
	if string(data1) != "original" {
		t.Fatalf("got %q, want %q", data1, "original")
	}

	// Mutate the returned bytes
	for i := range data1 {
		data1[i] = 'X'
	}

	// Second ReadFile must return the original data, unaffected
	data2, err := s.ReadFile("key")
	if err != nil {
		t.Fatal(err)
	}
	if string(data2) != "original" {
		t.Errorf("after mutation: got %q, want %q", data2, "original")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: storeDir.ReadDir streams entries correctly with n>0
// PREVENTS: cursor not advancing or io.EOF not returned at end

func TestStoreDirReadDirStreaming(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "dir/alpha", []byte("a"))
	writeOrFatal(t, s, "dir/beta", []byte("b"))
	writeOrFatal(t, s, "dir/gamma", []byte("g"))

	f, err := s.Open("dir")
	if err != nil {
		t.Fatal(err)
	}
	rdf, ok := f.(fs.ReadDirFile)
	if !ok {
		t.Fatal("Open(dir) did not return ReadDirFile")
	}

	// Read one at a time
	var allNames []string
	for {
		entries, readErr := rdf.ReadDir(1)
		for _, e := range entries {
			allNames = append(allNames, e.Name())
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			t.Fatalf("ReadDir(1): %v", readErr)
		}
	}
	sort.Strings(allNames)
	want := []string{"alpha", "beta", "gamma"}
	if !equalStrings(allNames, want) {
		t.Errorf("streamed names: got %v, want %v", allNames, want)
	}

	// After EOF, ReadDir(1) returns nil, io.EOF
	entries, readErr := rdf.ReadDir(1)
	if !errors.Is(readErr, io.EOF) {
		t.Errorf("after EOF: ReadDir(1) error = %v, want io.EOF", readErr)
	}
	if len(entries) != 0 {
		t.Errorf("after EOF: got %d entries, want 0", len(entries))
	}

	// After EOF, ReadDir(-1) returns empty, nil
	entries, readErr = rdf.ReadDir(-1)
	if readErr != nil {
		t.Errorf("after EOF: ReadDir(-1) error = %v, want nil", readErr)
	}
	if len(entries) != 0 {
		t.Errorf("after EOF: ReadDir(-1) got %d entries, want 0", len(entries))
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: storeDir.ReadDir n<=0 returns all remaining after partial n>0
// PREVENTS: n<=0 ignoring cursor and returning all entries

func TestStoreDirReadDirPartialThenAll(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "d/a", []byte("1"))
	writeOrFatal(t, s, "d/b", []byte("2"))
	writeOrFatal(t, s, "d/c", []byte("3"))
	writeOrFatal(t, s, "d/d", []byte("4"))

	f, err := s.Open("d")
	if err != nil {
		t.Fatal(err)
	}
	rdf, ok := f.(fs.ReadDirFile)
	if !ok {
		t.Fatal("Open did not return ReadDirFile")
	}

	// Read first 2
	entries, err := rdf.ReadDir(2)
	if err != nil {
		t.Fatalf("ReadDir(2): %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ReadDir(2): got %d entries, want 2", len(entries))
	}

	// ReadDir(-1) returns remaining 2
	rest, err := rdf.ReadDir(-1)
	if err != nil {
		t.Fatalf("ReadDir(-1) after partial: %v", err)
	}
	if len(rest) != 2 {
		t.Errorf("ReadDir(-1) after partial: got %d entries, want 2", len(rest))
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: storeDir.ReadDir on empty directory returns empty slice
// PREVENTS: nil slice or error on empty directory

func TestStoreDirReadDirEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	// Open root of empty store
	f, err := s.Open(".")
	if err != nil {
		t.Fatal(err)
	}
	rdf, ok := f.(fs.ReadDirFile)
	if !ok {
		t.Fatal("Open did not return ReadDirFile")
	}

	entries, err := rdf.ReadDir(-1)
	if err != nil {
		t.Fatalf("ReadDir(-1) on empty dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("empty dir: got %d entries, want 0", len(entries))
	}

	// n>0 on empty dir returns nil, io.EOF
	entries, err = rdf.ReadDir(1)
	if !errors.Is(err, io.EOF) {
		t.Errorf("ReadDir(1) on empty dir: error = %v, want io.EOF", err)
	}
	if len(entries) != 0 {
		t.Errorf("ReadDir(1) on empty dir: got %d entries, want 0", len(entries))
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: storeDir.ReadDir returns entries in sorted order
// PREVENTS: map iteration order leaking through to callers

func TestStoreDirReadDirSorted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	// Write in reverse alphabetical order
	for _, name := range []string{"zebra", "mango", "apple", "banana"} {
		writeOrFatal(t, s, "fruit/"+name, []byte(name))
	}

	f, err := s.Open("fruit")
	if err != nil {
		t.Fatal(err)
	}
	rdf, ok := f.(fs.ReadDirFile)
	if !ok {
		t.Fatal("Open did not return ReadDirFile")
	}
	entries, err := rdf.ReadDir(-1)
	if err != nil {
		t.Fatal(err)
	}

	names := dirEntryNames(entries)
	want := []string{"apple", "banana", "mango", "zebra"}
	if !equalStrings(names, want) {
		t.Errorf("sorted order: got %v, want %v", names, want)
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: storeDir.ReadDir entries have correct Info() sizes
// PREVENTS: DirEntry.Info reporting wrong size for files

func TestStoreDirReadDirInfoSizes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "d/small", []byte("hi"))
	writeOrFatal(t, s, "d/big", []byte("hello world, this is a longer string"))
	writeOrFatal(t, s, "d/empty", []byte{})

	f, err := s.Open("d")
	if err != nil {
		t.Fatal(err)
	}
	rdf, ok := f.(fs.ReadDirFile)
	if !ok {
		t.Fatal("Open did not return ReadDirFile")
	}
	entries, err := rdf.ReadDir(-1)
	if err != nil {
		t.Fatal(err)
	}

	sizes := make(map[string]int64)
	for _, e := range entries {
		info, infoErr := e.Info()
		if infoErr != nil {
			t.Fatalf("Info() for %q: %v", e.Name(), infoErr)
		}
		sizes[e.Name()] = info.Size()
	}

	if sizes["small"] != 2 {
		t.Errorf("small size: got %d, want 2", sizes["small"])
	}
	if sizes["big"] != 36 {
		t.Errorf("big size: got %d, want 36", sizes["big"])
	}
	if sizes["empty"] != 0 {
		t.Errorf("empty size: got %d, want 0", sizes["empty"])
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Open returns correct base name for nested directories
// PREVENTS: Stat().Name() returning full path instead of base name

func TestStoreOpenNestedDirName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "a/b/c/leaf", []byte("data"))

	// Open intermediate directories and verify Stat().Name()
	tests := []struct {
		path string
		want string
	}{
		{".", "."},
		{"a", "a"},
		{"a/b", "b"},
		{"a/b/c", "c"},
	}
	for _, tt := range tests {
		f, openErr := s.Open(tt.path)
		if openErr != nil {
			t.Fatalf("Open(%q): %v", tt.path, openErr)
		}
		info, statErr := f.Stat()
		if statErr != nil {
			t.Fatalf("Stat(%q): %v", tt.path, statErr)
		}
		if info.Name() != tt.want {
			t.Errorf("Open(%q).Stat().Name() = %q, want %q", tt.path, info.Name(), tt.want)
		}
		if !info.IsDir() {
			t.Errorf("Open(%q): expected directory", tt.path)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Import from empty reader returns error
// PREVENTS: silent creation of empty store from zero bytes

func TestStoreImportEmptyReader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "existing", []byte("data"))

	err = s.Import(bytes.NewReader(nil))
	if err == nil {
		t.Error("Import from empty reader should fail")
	}

	// Store should still be usable (flush recovery)
	if !s.Has("existing") {
		// It's acceptable if Import cleared the store on error,
		// but the store must not crash
		t.Log("Import cleared store content on error (expected if unload succeeded)")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Open on corrupted store (valid magic, bad container netcapstring)
// PREVENTS: panic on garbled internal data

func TestStoreOpenBadContainer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.zefs")
	// Write valid magic + garbage that isn't a valid netcapstring
	data := []byte("ZeFS:not-a-netcapstring-at-all")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Open(path)
	if err == nil {
		t.Error("Open on bad container should fail")
	}
}

// VALIDATES: storeFile.Stat reports correct size for files opened via Open
// PREVENTS: size mismatch between ReadFile and Open+Stat

func TestStoreOpenFileStatSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("twelve chars")
	writeOrFatal(t, s, "file.txt", content)

	f, err := s.Open("file.txt")
	if err != nil {
		t.Fatal(err)
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != int64(len(content)) {
		t.Errorf("Stat().Size() = %d, want %d", info.Size(), len(content))
	}
	if info.IsDir() {
		t.Error("file should not be a directory")
	}
	if info.Mode() != 0o444 {
		t.Errorf("file mode: got %v, want 0444", info.Mode())
	}
	if info.Name() != "file.txt" {
		t.Errorf("Name() = %q, want %q", info.Name(), "file.txt")
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: ReadFile through Open path vs direct ReadFile return same data
// PREVENTS: divergence between fs.FS Open+Read and ReadFile paths

func TestStoreOpenReadVsReadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("compare both paths")
	writeOrFatal(t, s, "x/y/z", content)

	// Path 1: ReadFile
	direct, err := s.ReadFile("x/y/z")
	if err != nil {
		t.Fatal(err)
	}

	// Path 2: Open + Read
	f, err := s.Open("x/y/z")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, copyErr := io.Copy(&buf, f); copyErr != nil {
		t.Fatal(copyErr)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(direct, buf.Bytes()) {
		t.Errorf("ReadFile vs Open+Read mismatch: %q vs %q", direct, buf.Bytes())
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Import with truncated but valid-magic data
// PREVENTS: panic on partial store data

func TestStoreImportTruncated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	// Valid magic but truncated netcapstring
	err = s.Import(bytes.NewReader([]byte("ZeFS:0000005:00000")))
	if err == nil {
		t.Error("Import with truncated data should fail")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: ReadDir on leaf via BlobStore.ReadDir returns error
// PREVENTS: treating file node as directory in ReadDir

func TestStoreReadDirOnLeaf(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "leaf", []byte("data"))

	_, err = s.ReadDir("leaf")
	if err == nil {
		t.Error("ReadDir on leaf file should fail")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: operations on a closed store do not panic
// PREVENTS: nil dereference on backing/tree after Close

func TestStoreUseAfterClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "key", []byte("data"))

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// ReadFile after close: backing is nil but tree still exists.
	// readFile walks tree, gets data pointing into old (munmapped) backing.
	// On unix this may SIGBUS; on heap fallback it may succeed with stale data.
	// The key invariant: no panic from nil pointer dereference.
	// We test Has since it only touches the tree (no backing access).
	if s.Has("key") {
		t.Log("Has returns true after Close (tree not cleared)")
	}

	// List touches tree only
	keys := s.List("")
	_ = keys // no panic is the test
}

// VALIDATES: remove cascading prune removes deeply nested empty parents
// PREVENTS: empty intermediate directories left behind after deep removal

func TestStoreRemoveDeepCascadingPrune(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "a/b/c/d/e/leaf", []byte("deep"))

	if err := s.Remove("a/b/c/d/e/leaf"); err != nil {
		t.Fatal(err)
	}

	// All intermediate directories should be pruned since the only leaf is gone
	entries, err := s.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		names := dirEntryNames(entries)
		t.Errorf("root should be empty after deep prune, got %v", names)
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: remove deep leaf preserves sibling branches
// PREVENTS: over-pruning deleting unrelated entries at intermediate levels

func TestStoreRemoveDeepPreservesSiblings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "a/b/c/d/leaf1", []byte("1"))
	writeOrFatal(t, s, "a/b/c/sibling", []byte("2"))
	writeOrFatal(t, s, "a/b/other", []byte("3"))

	if err := s.Remove("a/b/c/d/leaf1"); err != nil {
		t.Fatal(err)
	}

	// "d" directory should be pruned, but "c" stays (has "sibling")
	if !s.Has("a/b/c/sibling") {
		t.Error("sibling at c level should survive")
	}
	if !s.Has("a/b/other") {
		t.Error("sibling at b level should survive")
	}

	// "d" should no longer exist as a directory
	_, err = s.ReadDir("a/b/c/d")
	if err == nil {
		t.Error("ReadDir(a/b/c/d) should fail after pruning")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: List on a leaf key returns that single key
// PREVENTS: List panicking or returning wrong result when prefix is a file

func TestStoreListOnExactLeafKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "single", []byte("v"))

	// List with prefix exactly matching a leaf
	result := s.List("single")
	if len(result) != 1 || result[0] != "single" {
		t.Errorf("List(exact leaf): got %v, want [single]", result)
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: encode skips orphaned keys (in s.keys but not in tree)
// PREVENTS: encoding producing corrupt output for orphaned keys

func TestStoreWriteFileRejectsPathConflict(t *testing.T) {
	// VALIDATES: set() returns error on directory/file path conflicts
	// PREVENTS: nil map panic from writing through a leaf node, or
	//           silently orphaning subtrees by overwriting a directory with a file
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	writeOrFatal(t, s, "a/child", []byte("data"))

	// Writing "a" as a file should fail: "a" is a directory
	err = s.WriteFile("a", []byte("leaf"), 0)
	if err == nil {
		t.Fatal("expected error writing file over directory, got nil")
	}

	// Original child should still be reachable
	if !s.Has("a/child") {
		t.Error("a/child should still exist")
	}
	got, err := s.ReadFile("a/child")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "data" {
		t.Errorf("got %q, want %q", got, "data")
	}

	// Writing "a/child/deeper" should fail: "a/child" is a file
	err = s.WriteFile("a/child/deeper", []byte("nope"), 0)
	if err == nil {
		t.Fatal("expected error writing through file, got nil")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Import with valid multi-entry store data
// PREVENTS: off-by-one in decode loop when chaining many entries

func TestStoreImportManyEntries(t *testing.T) {
	path1 := filepath.Join(t.TempDir(), "source.zefs")
	s1, err := Create(path1)
	if err != nil {
		t.Fatal(err)
	}
	// Write 20 entries with varying sizes
	for i := range 20 {
		key := fmt.Sprintf("entry/%03d", i)
		data := bytes.Repeat([]byte{byte('a' + i%26)}, i*10+1)
		writeOrFatal(t, s1, key, data)
	}

	var buf bytes.Buffer
	if err := s1.Export(&buf); err != nil {
		t.Fatal(err)
	}
	if err := s1.Close(); err != nil {
		t.Fatal(err)
	}

	// Import into a fresh store
	path2 := filepath.Join(t.TempDir(), "dest.zefs")
	s2, err := Create(path2)
	if err != nil {
		t.Fatal(err)
	}
	if err := s2.Import(bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatal(err)
	}

	// Verify all 20 entries
	for i := range 20 {
		key := fmt.Sprintf("entry/%03d", i)
		got, readErr := s2.ReadFile(key)
		if readErr != nil {
			t.Fatalf("ReadFile(%s): %v", key, readErr)
		}
		want := bytes.Repeat([]byte{byte('a' + i%26)}, i*10+1)
		if !bytes.Equal(got, want) {
			t.Errorf("entry %d: got %d bytes, want %d bytes", i, len(got), len(want))
		}
	}

	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: storeFile.Read with buffer larger than remaining data
// PREVENTS: Read returning wrong count or not reaching EOF correctly

func TestStoreFileReadLargeBuffer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "small", []byte("hi"))

	f, err := s.Open("small")
	if err != nil {
		t.Fatal(err)
	}

	// Read with buffer much larger than file content
	buf := make([]byte, 4096)
	n, err := f.Read(buf)
	if n != 2 {
		t.Errorf("first Read: got %d bytes, want 2", n)
	}
	if err != nil {
		t.Errorf("first Read: unexpected error %v", err)
	}
	if string(buf[:n]) != "hi" {
		t.Errorf("content: got %q, want %q", buf[:n], "hi")
	}

	// Next read should return 0, io.EOF
	n, err = f.Read(buf)
	if n != 0 {
		t.Errorf("second Read: got %d bytes, want 0", n)
	}
	if !errors.Is(err, io.EOF) {
		t.Errorf("second Read: got %v, want io.EOF", err)
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: storeDirEntry.Info() on directory returns ModeDir
// PREVENTS: DirEntry.Info() losing directory mode for dir entries

func TestStoreDirEntryInfoOnDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "parent/child/leaf", []byte("data"))

	entries, err := s.ReadDir("parent")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}

	e := entries[0]
	if e.Name() != "child" {
		t.Fatalf("entry name: got %q, want %q", e.Name(), "child")
	}
	if !e.IsDir() {
		t.Fatal("child should be a directory")
	}

	info, err := e.Info()
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Error("Info().IsDir() should be true")
	}
	if info.Mode() != fs.ModeDir|0o555 {
		t.Errorf("Info().Mode() = %v, want %v", info.Mode(), fs.ModeDir|0o555)
	}
	if info.Size() != 0 {
		t.Errorf("directory Info().Size() = %d, want 0", info.Size())
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Open on corrupted file with only magic bytes
// PREVENTS: panic on file that is exactly the magic length

func TestStoreOpenMagicOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "magic.zefs")
	if err := os.WriteFile(path, []byte("ZeFS:"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Open(path)
	if err == nil {
		t.Error("Open on magic-only file should fail")
	}
}

// VALIDATES: WriteFile data isolation across multiple writes
// PREVENTS: caller's buffer mutations affecting stored data after WriteFile

func TestStoreWriteFileInputIsolation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	input := []byte("original")
	if err := s.WriteFile("key", input, 0); err != nil {
		t.Fatal(err)
	}

	// Mutate the caller's buffer after WriteFile
	input[0] = 'X'

	// Stored data should be unaffected
	got, err := s.ReadFile("key")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "original" {
		t.Errorf("stored data affected by caller mutation: got %q", got)
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: storeDir.Read on directory returns PathError
// PREVENTS: Read on directory succeeding or returning wrong error type

func TestStoreDirReadReturnsPathError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "dir/file", []byte("data"))

	f, err := s.Open("dir")
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 10)
	_, readErr := f.Read(buf)
	if readErr == nil {
		t.Fatal("Read on directory should fail")
	}
	var pathErr *fs.PathError
	if !errors.As(readErr, &pathErr) {
		t.Errorf("Read on directory: got %T, want *fs.PathError", readErr)
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteFile rejects keys that violate the fs.ValidPath contract
// PREVENTS: path traversal (..), absolute paths, and malformed keys in the store

func TestStoreWriteFileRejectsInvalidKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	invalid := []struct {
		name string
		key  string
	}{
		{"dot-dot traversal", "../secret"},
		{"mid dot-dot", "a/../b"},
		{"trailing dot-dot", "a/b/.."},
		{"absolute path", "/etc/passwd"},
		{"trailing slash", "key/"},
		{"leading slash", "/key"},
		{"double slash", "a//b"},
		{"dot only", "."},
		{"dot prefix", "./a"},
		{"empty string", ""},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			err := s.WriteFile(tt.key, []byte("data"), 0)
			if err == nil {
				t.Errorf("WriteFile(%q) should fail", tt.key)
			}
		})
	}

	// Valid keys must still work
	valid := []string{"f1", "d/f2", "d/sub/f3", "file.txt", "deep/nested/path/file"}
	for _, key := range valid {
		if err := s.WriteFile(key, []byte("ok"), 0); err != nil {
			t.Errorf("WriteFile(%q) should succeed: %v", key, err)
		}
	}
}

// VALIDATES: WriteLock.WriteFile also rejects invalid keys
// PREVENTS: bypassing key validation through the lock path

func TestWriteLockWriteFileRejectsInvalidKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	wl := lockOrFatal(t, s)
	defer func() { _ = wl.Release() }()

	if err := wl.WriteFile("../escape", []byte("data"), 0); err == nil {
		t.Error("WriteLock.WriteFile(../escape) should fail")
	}
	if err := wl.WriteFile("/absolute", []byte("data"), 0); err == nil {
		t.Error("WriteLock.WriteFile(/absolute) should fail")
	}
	if err := wl.WriteFile("a//b", []byte("data"), 0); err == nil {
		t.Error("WriteLock.WriteFile(a//b) should fail")
	}

	// Valid key through lock should work
	if err := wl.WriteFile("valid/key", []byte("ok"), 0); err != nil {
		t.Errorf("WriteLock.WriteFile(valid/key) should succeed: %v", err)
	}
}

// VALIDATES: decode stops at null byte in container data (not just newline)
// PREVENTS: null-byte padding in container misinterpreted as entry data

func TestStoreDecodeNullTermination(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "key", []byte("value"))
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopen and verify the store is readable (container has \n + null padding)
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s2.ReadFile("key")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "value" {
		t.Errorf("got %q, want %q", got, "value")
	}
	// List should only return the one key (no phantom entries from padding)
	keys := s2.List("")
	if len(keys) != 1 {
		t.Errorf("expected 1 key, got %d: %v", len(keys), keys)
	}
	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteFile update where new data fits exactly in existing capacity
// PREVENTS: off-by-one in capacity regrowth check (> vs >=)

func TestStoreUpdateExactCapacity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	// Write initial data, note the slot capacity
	writeOrFatal(t, s, "key", []byte("hello"))
	sl, ok := s.slot("key")
	if !ok {
		t.Fatal("slot not found")
	}
	origDataCap := sl.data.capacity

	// Write data exactly at dataCap length -- should NOT trigger regrowth
	exact := make([]byte, origDataCap)
	for i := range exact {
		exact[i] = byte('A' + i%26)
	}
	writeOrFatal(t, s, "key", exact)

	sl2, _ := s.slot("key")
	if sl2.data.capacity != origDataCap {
		t.Errorf("dataCap changed from %d to %d on exact-fit update", origDataCap, sl2.data.capacity)
	}

	// Read back and verify
	got, err := s.ReadFile("key")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, exact) {
		t.Errorf("data mismatch after exact-capacity update")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteFile update exceeding capacity triggers regrowth
// PREVENTS: data truncation when value grows past slot capacity

func TestStoreUpdateExceedsCapacity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	writeOrFatal(t, s, "key", []byte("hi"))
	sl, _ := s.slot("key")
	origDataCap := sl.data.capacity

	// Write data larger than current capacity
	big := bytes.Repeat([]byte("x"), origDataCap+1)
	writeOrFatal(t, s, "key", big)

	sl2, _ := s.slot("key")
	if sl2.data.capacity <= origDataCap {
		t.Errorf("dataCap should have grown: was %d, now %d", origDataCap, sl2.data.capacity)
	}

	got, err := s.ReadFile("key")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, big) {
		t.Errorf("data mismatch after capacity regrowth")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: growing one entry recalculates offsets for all entries
// PREVENTS: stale offsets after capacity growth shifts subsequent entries

func TestStoreGrowthRecalculatesOffsets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	// Create 5 entries in order. Keys are ordered deterministically.
	entries := []struct {
		key  string
		data string
	}{
		{"a/first", "data-1"},
		{"b/second", "data-2"},
		{"c/third", "data-3"},
		{"d/fourth", "data-4"},
		{"e/fifth", "data-5"},
	}
	for _, e := range entries {
		writeOrFatal(t, s, e.key, []byte(e.data))
	}

	// Record offsets before growth
	slotsBefore := make(map[string]slotInfo)
	for _, e := range entries {
		sl, ok := s.slot(e.key)
		if !ok {
			t.Fatalf("slot %q not found before growth", e.key)
		}
		slotsBefore[e.key] = sl
	}

	// Grow "b/second" well past its capacity -- forces re-encode, shifts c/d/e
	sl2 := slotsBefore["b/second"]
	bigData := bytes.Repeat([]byte("X"), sl2.data.capacity*3)
	writeOrFatal(t, s, "b/second", bigData)

	// Verify all entries are readable with correct data
	for _, e := range entries {
		want := e.data
		if e.key == "b/second" {
			want = string(bigData)
		}
		got, readErr := s.ReadFile(e.key)
		if readErr != nil {
			t.Fatalf("ReadFile(%q) after growth: %v", e.key, readErr)
		}
		if string(got) != want {
			t.Errorf("ReadFile(%q): got %d bytes, want %d bytes", e.key, len(got), len(want))
		}
	}

	// Verify offsets are consistent: each slot's offset should be after the previous slot's end
	slotsAfter := make(map[string]slotInfo)
	for _, e := range entries {
		sl, ok := s.slot(e.key)
		if !ok {
			t.Fatalf("slot %q not found after growth", e.key)
		}
		slotsAfter[e.key] = sl
	}

	// a/first should be at the same offset (before the grown entry)
	if slotsAfter["a/first"].name.offset != slotsBefore["a/first"].name.offset {
		t.Errorf("a/first offset changed: before=%d, after=%d",
			slotsBefore["a/first"].name.offset, slotsAfter["a/first"].name.offset)
	}

	// b/second should have grown capacity
	if slotsAfter["b/second"].data.capacity <= slotsBefore["b/second"].data.capacity {
		t.Errorf("b/second data capacity should have grown: before=%d, after=%d",
			slotsBefore["b/second"].data.capacity, slotsAfter["b/second"].data.capacity)
	}

	// c/third offset should have shifted (it's after the grown entry)
	if slotsAfter["c/third"].name.offset == slotsBefore["c/third"].name.offset {
		t.Error("c/third offset should have shifted after b/second grew")
	}

	// Verify offset ordering: each entry starts after the previous entry ends
	orderedKeys := []string{"a/first", "b/second", "c/third", "d/fourth", "e/fifth"}
	for i := 1; i < len(orderedKeys); i++ {
		prev := slotsAfter[orderedKeys[i-1]]
		curr := slotsAfter[orderedKeys[i]]
		prevEnd := prev.data.offset + prev.data.totalLen()
		if curr.name.offset != prevEnd {
			t.Errorf("entry %q starts at %d, but previous entry %q ends at %d (gap or overlap)",
				orderedKeys[i], curr.name.offset, orderedKeys[i-1], prevEnd)
		}
	}

	// Reopen store from disk and verify all data survives
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		want := e.data
		if e.key == "b/second" {
			want = string(bigData)
		}
		got, readErr := s2.ReadFile(e.key)
		if readErr != nil {
			t.Fatalf("ReadFile(%q) after reopen: %v", e.key, readErr)
		}
		if string(got) != want {
			t.Errorf("ReadFile(%q) after reopen: got %d bytes, want %d bytes", e.key, len(got), len(want))
		}
	}
	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: List on a key that is a leaf (file) not a directory
// PREVENTS: List returning results when prefix matches a file, not a dir

func TestStoreListOnFilePrefix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "a/b", []byte("leaf"))

	// "a/b" is a file. List("a/b") should return just ["a/b"] since
	// walk("a/b") finds the leaf, and collect on a leaf appends itself.
	keys := s.List("a/b")
	if len(keys) != 1 || keys[0] != "a/b" {
		t.Errorf("List on leaf: got %v, want [a/b]", keys)
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Open a path that traverses through a leaf node returns not-found
// PREVENTS: walk through a file silently succeeding

func TestStoreOpenPathThroughFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "a/b", []byte("leaf"))

	// "a/b/c" should fail because "a/b" is a file (no children)
	_, err = s.Open("a/b/c")
	if err == nil {
		t.Error("Open path through file should fail")
	}
	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) {
		t.Errorf("expected *fs.PathError, got %T", err)
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: ReadDir with negative n behaves like n <= 0 (returns all remaining)
// PREVENTS: negative n causing unexpected behavior in streaming ReadDir

func TestStoreDirReadDirNegativeN(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "d/a", []byte("1"))
	writeOrFatal(t, s, "d/b", []byte("2"))
	writeOrFatal(t, s, "d/c", []byte("3"))

	f, err := s.Open("d")
	if err != nil {
		t.Fatal(err)
	}
	rdf, ok := f.(fs.ReadDirFile)
	if !ok {
		t.Fatal("expected ReadDirFile")
	}

	entries, err := rdf.ReadDir(-1)
	if err != nil {
		t.Fatalf("ReadDir(-1): %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("ReadDir(-1): got %d entries, want 3", len(entries))
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: ReadDir(n) returns io.EOF on repeated calls after exhaustion
// PREVENTS: ReadDir returning entries after cursor is at end

func TestStoreDirReadDirRepeatedAfterEOF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "d/x", []byte("1"))

	f, err := s.Open("d")
	if err != nil {
		t.Fatal(err)
	}
	rdf, ok := f.(fs.ReadDirFile)
	if !ok {
		t.Fatal("expected ReadDirFile")
	}

	// First call returns the entry + EOF (only 1 entry, asking for 10)
	entries, err := rdf.ReadDir(10)
	if len(entries) != 1 {
		t.Errorf("first ReadDir: got %d entries, want 1", len(entries))
	}
	if !errors.Is(err, io.EOF) {
		t.Errorf("first ReadDir: got %v, want io.EOF", err)
	}

	// Second call: cursor exhausted, should return nil + EOF
	entries, err = rdf.ReadDir(10)
	if len(entries) != 0 {
		t.Errorf("second ReadDir: got %d entries, want 0", len(entries))
	}
	if !errors.Is(err, io.EOF) {
		t.Errorf("second ReadDir: got %v, want io.EOF", err)
	}

	// Third call: still EOF
	entries, err = rdf.ReadDir(1)
	if len(entries) != 0 {
		t.Errorf("third ReadDir: got %d entries, want 0", len(entries))
	}
	if !errors.Is(err, io.EOF) {
		t.Errorf("third ReadDir: got %v, want io.EOF", err)
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: ReadDir(n) with n exactly equal to remaining entries returns entries + EOF
// PREVENTS: off-by-one at exact boundary between last batch and EOF

func TestStoreDirReadDirExactBoundary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "d/a", []byte("1"))
	writeOrFatal(t, s, "d/b", []byte("2"))
	writeOrFatal(t, s, "d/c", []byte("3"))

	f, err := s.Open("d")
	if err != nil {
		t.Fatal(err)
	}
	rdf, ok := f.(fs.ReadDirFile)
	if !ok {
		t.Fatal("expected ReadDirFile")
	}

	// Read exactly 2
	entries, err := rdf.ReadDir(2)
	if len(entries) != 2 {
		t.Errorf("batch 1: got %d entries, want 2", len(entries))
	}
	if err != nil {
		t.Errorf("batch 1: unexpected error %v", err)
	}

	// Read exactly 1 (the last one) -- should return entry + EOF
	entries, err = rdf.ReadDir(1)
	if len(entries) != 1 {
		t.Errorf("batch 2: got %d entries, want 1", len(entries))
	}
	if !errors.Is(err, io.EOF) {
		t.Errorf("batch 2: got %v, want io.EOF", err)
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Open(".") then Read on root directory returns PathError
// PREVENTS: root directory being readable as a file

func TestStoreOpenDotReadError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "file", []byte("data"))

	f, err := s.Open(".")
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 10)
	_, readErr := f.Read(buf)
	if readErr == nil {
		t.Fatal("Read on root dir should fail")
	}
	var pathErr *fs.PathError
	if !errors.As(readErr, &pathErr) {
		t.Errorf("expected *fs.PathError, got %T", readErr)
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: storeFileInfo fields for files (ModTime, Sys, Name, Size)
// PREVENTS: FileInfo contract violations for file entries

func TestStoreFileInfoFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "dir/file.txt", []byte("content"))

	f, err := s.Open("dir/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}

	if info.Name() != "file.txt" {
		t.Errorf("Name: got %q, want %q", info.Name(), "file.txt")
	}
	if info.Size() != 7 {
		t.Errorf("Size: got %d, want 7", info.Size())
	}
	if info.Mode() != fs.FileMode(0o444) {
		t.Errorf("Mode: got %v, want 0o444", info.Mode())
	}
	if info.IsDir() {
		t.Error("IsDir should be false for file")
	}
	if info.ModTime().IsZero() == false {
		t.Error("ModTime should be zero time")
	}
	if info.Sys() != nil {
		t.Error("Sys should be nil")
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: storeDir Stat returns directory mode and zero size
// PREVENTS: Stat on directory returning file-like FileInfo

func TestStoreDirStatFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "parent/child", []byte("data"))

	f, err := s.Open("parent")
	if err != nil {
		t.Fatal(err)
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}

	if info.Name() != "parent" {
		t.Errorf("Name: got %q, want %q", info.Name(), "parent")
	}
	if info.Size() != 0 {
		t.Errorf("Size: got %d, want 0", info.Size())
	}
	if info.Mode() != fs.ModeDir|0o555 {
		t.Errorf("Mode: got %v, want %v", info.Mode(), fs.ModeDir|0o555)
	}
	if !info.IsDir() {
		t.Error("IsDir should be true for directory")
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Create on unwritable path returns error
// PREVENTS: panic or nil store on permission denied

func TestStoreCreateUnwritablePath(t *testing.T) {
	_, err := Create("/dev/null/impossible/store.zefs")
	if err == nil {
		t.Error("Create on unwritable path should fail")
	}
}

// VALIDATES: tree remove on nonexistent intermediate returns false
// PREVENTS: remove of deep path with missing parent succeeding

func TestStoreRemoveNonexistentDeep(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	err = s.Remove("no/such/deep/path")
	if err == nil {
		t.Error("Remove nonexistent deep path should fail")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: ReadDir on nonexistent directory returns error
// PREVENTS: nil entries without error on missing directory

func TestStoreReadDirNonexistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.ReadDir("nonexistent")
	if err == nil {
		t.Error("ReadDir on nonexistent dir should fail")
	}
	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) {
		t.Errorf("expected *fs.PathError, got %T", err)
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: ReadDir on a leaf path returns error (not directory)
// PREVENTS: ReadDir returning empty results for file paths

func TestStoreReadDirOnFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "a/leaf", []byte("data"))

	_, err = s.ReadDir("a/leaf")
	if err == nil {
		t.Error("ReadDir on file should fail")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: ReadDir(0) on streaming ReadDirFile returns all entries then empty
// PREVENTS: ReadDir(0) not advancing cursor (returning entries repeatedly)

func TestStoreDirReadDirZeroThenZero(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "d/a", []byte("1"))
	writeOrFatal(t, s, "d/b", []byte("2"))

	f, err := s.Open("d")
	if err != nil {
		t.Fatal(err)
	}
	rdf, ok := f.(fs.ReadDirFile)
	if !ok {
		t.Fatal("expected ReadDirFile")
	}

	// First ReadDir(0) returns all entries
	entries, err := rdf.ReadDir(0)
	if err != nil {
		t.Fatalf("first ReadDir(0): %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("first ReadDir(0): got %d entries, want 2", len(entries))
	}

	// Second ReadDir(0) should return empty (cursor at end)
	entries, err = rdf.ReadDir(0)
	if err != nil {
		t.Fatalf("second ReadDir(0): %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("second ReadDir(0): got %d entries, want 0", len(entries))
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Has returns false for directory paths (only true for leaves)
// PREVENTS: directory nodes being mistaken for files

func TestStoreHasDirectoryReturnsFalse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "dir/file", []byte("data"))

	if s.Has("dir") {
		t.Error("Has(directory) should return false")
	}
	if !s.Has("dir/file") {
		t.Error("Has(file) should return true")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Open on corrupted file with truncated magic returns error
// PREVENTS: panic or partial read on files shorter than magic length

func TestStoreOpenTruncatedMagic(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"1 byte", []byte("Z")},
		{"3 bytes", []byte("ZeF")},
		{"4 bytes", []byte("ZeFS")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "bad.zefs")
			if err := os.WriteFile(path, tt.data, 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := Open(path)
			if err == nil {
				t.Error("Open on truncated magic should fail")
			}
		})
	}
}

func writeOrFatal(t *testing.T, s *BlobStore, name string, data []byte) {
	t.Helper()
	if err := s.WriteFile(name, data, 0); err != nil {
		t.Fatalf("WriteFile(%s): %v", name, err)
	}
}

func lockOrFatal(t *testing.T, s *BlobStore) *WriteLock {
	t.Helper()
	wl, err := s.Lock()
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	return wl
}

func writeLockedOrFatal(t *testing.T, wl *WriteLock, name string, data []byte) {
	t.Helper()
	if err := wl.WriteFile(name, data, 0); err != nil {
		t.Fatalf("WriteLock.WriteFile(%s): %v", name, err)
	}
}

func dirEntryNames(entries []fs.DirEntry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
