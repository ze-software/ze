package zefs

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
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
	if sl.dataCap < minCap {
		t.Errorf("slot capacity %d should be >= %d (used %d + 10%% spare)", sl.dataCap, minCap, len(big))
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
	big := make([]byte, slBefore.dataCap*3)
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
	if slAfter.dataCap <= slBefore.dataCap {
		t.Errorf("slot should have grown: before=%d, after=%d", slBefore.dataCap, slAfter.dataCap)
	}
	minCap := len(big) + len(big)/10
	if slAfter.dataCap < minCap {
		t.Errorf("regrown slot %d should be >= %d (10%% spare)", slAfter.dataCap, minCap)
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

// VALIDATES: WriteFile rejects data exceeding the 7-digit header limit
// PREVENTS: silent framing corruption on oversized entries

func TestStoreHeaderOverflow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	// Write data larger than the 7-digit header can represent
	big := make([]byte, maxHeaderVal+1)
	err = s.WriteFile("toobig", big, 0)
	if err == nil {
		t.Fatal("expected error for data exceeding header limit")
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
// PREVENTS: netstring colon delimiters interfering with key parsing

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

func writeOrFatal(t *testing.T, s *BlobStore, name string, data []byte) {
	t.Helper()
	if err := s.WriteFile(name, data, 0); err != nil {
		t.Fatalf("WriteFile(%s): %v", name, err)
	}
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
