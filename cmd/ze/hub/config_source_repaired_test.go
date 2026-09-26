// Design: docs/architecture/storage-backends.md -- content-addressed history, repaired-state startup
// Related: config_source.go -- initializeConfigSource logs the rebuild

package hub

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/zefs"
)

// repairedName is the config name of the repaired-output scenario. It is the
// name bare `ze start` selects when meta/instance/name is absent.
const repairedName = "ze.conf"

// repairedStore is what the repaired-output scenario leaves on disk: the
// three stamps the pointers name and the bytes rollback must still read.
type repairedStore struct {
	dir      string // config directory holding the repaired `database` tree
	rollback string // stamp R, whose entry and object survive repair
	active   string // stamp S, whose entry repair dropped
	pending  string // stamp C, the stale candidate whose entry repair dropped
	bytesR   []byte
}

// buildRepairedStore runs steps 1-3 of the spec's repaired-output scenario:
// history R, S and C with rollback=R, active=S, candidate=C; the objects of S
// and C removed by raw-key mutation; the production repair producers
// (zefs.RepairPath then storage.RepairHistory, as `ze data repair` runs them)
// into a fresh config directory. It asserts the repair report and the pointers
// the output keeps.
func buildRepairedStore(t *testing.T) repairedStore {
	t.Helper()
	source := t.TempDir()
	store, err := storage.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	bytesR := []byte("set environment log level info\n")
	bytesS := []byte("set environment log level error\n")
	bytesC := []byte("set environment log level debug\n")
	base := time.Date(2026, 9, 26, 10, 0, 0, 0, time.Local)
	stampR := writeAndPromote(t, store, bytesR, base, true)
	stampS := writeAndPromote(t, store, bytesS, base.Add(time.Hour), true)
	stampC := writeAndPromote(t, store, bytesC, base.Add(2*time.Hour), false)
	mirror, err := store.ReadFile(repairedName)
	if err != nil || !bytes.Equal(mirror, bytesS) {
		t.Fatalf("active mirror = %q, %v; want S", mirror, err)
	}
	for _, lost := range [][]byte{bytesS, bytesC} {
		if err := store.RemoveKey(zefs.KeyObject.Key(digestHex(lost))); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	output := t.TempDir()
	if _, err := zefs.RepairPath(filepath.Join(source, storage.TreeName), filepath.Join(output, storage.TreeName)); err != nil {
		t.Fatal(err)
	}
	repaired, err := storage.OpenTree(filepath.Join(output, storage.TreeName), true)
	if err != nil {
		t.Fatal(err)
	}
	defer repaired.Close() //nolint:errcheck // test cleanup
	findings, err := storage.RepairHistory(repaired)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"file/" + stampS + "/" + repairedName:     storage.HistoryDanglingEntry,
		"file/" + stampC + "/" + repairedName:     storage.HistoryDanglingEntry,
		zefs.KeyConfigActive.Key(repairedName):    storage.HistoryDanglingPointer,
		zefs.KeyConfigCandidate.Key(repairedName): storage.HistoryDanglingPointer,
	}
	for _, finding := range findings {
		if kind, ok := want[finding.Key]; ok && kind == finding.Kind {
			delete(want, finding.Key)
		}
	}
	if len(want) != 0 {
		t.Fatalf("repair report %+v misses %v", findings, want)
	}
	requirePointers(t, repaired, stampS, stampC, stampR)
	for _, dropped := range []string{stampS, stampC} {
		if _, err := repaired.ReadVersion(repairedName, dropped); err == nil {
			t.Fatalf("repair kept the dangling entry %s", dropped)
		}
	}
	if content, err := repaired.ReadVersion(repairedName, stampR); err != nil || !bytes.Equal(content, bytesR) {
		t.Fatalf("rollback R = %q, %v", content, err)
	}
	return repairedStore{dir: output, rollback: stampR, active: stampS, pending: stampC, bytesR: bytesR}
}

// writeAndPromote stages data as candidate and, when promote is set, promotes
// it, so the pointer sequence is the one commits produce.
func writeAndPromote(t *testing.T, store storage.Storage, data []byte, at time.Time, promote bool) string {
	t.Helper()
	stamp, err := storage.WriteCandidateVersion(store, repairedName, data, at)
	if err != nil {
		t.Fatal(err)
	}
	if !promote {
		return stamp
	}
	if err := storage.PromoteCandidate(store, repairedName); err != nil {
		t.Fatal(err)
	}
	return stamp
}

func digestHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// requirePointers asserts the raw active, candidate and rollback pointers.
// An empty want asserts the pointer is absent.
func requirePointers(t *testing.T, store storage.Storage, active, candidate, rollback string) {
	t.Helper()
	pointers := []struct{ key, want string }{
		{zefs.KeyConfigActive.Key(repairedName), active},
		{zefs.KeyConfigCandidate.Key(repairedName), candidate},
		{zefs.KeyConfigRollback.Key(repairedName), rollback},
	}
	for _, pointer := range pointers {
		value, err := store.ReadKey(pointer.key)
		if pointer.want == "" {
			if err == nil {
				t.Fatalf("%s = %q, want absent", pointer.key, value)
			}
			continue
		}
		if err != nil || strings.TrimSpace(string(value)) != pointer.want {
			t.Fatalf("%s = %q, %v; want %s", pointer.key, value, err, pointer.want)
		}
	}
}

// runCapturingStderr enters the daemon's run with os.Stderr on a file, so the
// refusal and the slog lines run writes are readable after it returns.
func runCapturingStderr(t *testing.T, store storage.Storage, configPath string) (int, string) {
	t.Helper()
	capture, err := os.Create(filepath.Join(t.TempDir(), "stderr"))
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stderr
	os.Stderr = capture
	code := run(store, configPath, nil, 0, 0, false, "", false, "", "", false, nil)
	os.Stderr = saved
	if err := capture.Close(); err != nil {
		t.Fatal(err)
	}
	logged, err := os.ReadFile(capture.Name())
	if err != nil {
		t.Fatal(err)
	}
	return code, string(logged)
}

// AC-15 and R-10, stored source: `ze start` without a file reads the active
// pointer, whose version repair dropped. The daemon refuses with exit 1
// naming the pointer and the stamp, serves nothing, and leaves every pointer
// where repair left it.
func TestRunRepairedStoreStoredSource(t *testing.T) {
	scenario := buildRepairedStore(t)
	store, err := storage.Open(scenario.dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close() //nolint:errcheck // test cleanup
	bound := storage.BindConfigSource(store, repairedName, storage.ConfigSourceStored)

	code, logged := runCapturingStderr(t, bound, repairedName)
	if code != 1 {
		t.Fatalf("run = %d, want 1\n%s", code, logged)
	}
	for _, want := range []string{"error: read config", zefs.KeyConfigActive.Key(repairedName), scenario.active} {
		if !strings.Contains(logged, want) {
			t.Fatalf("refusal does not name %q:\n%s", want, logged)
		}
	}
	requirePointers(t, store, scenario.active, scenario.pending, scenario.rollback)
}

// AC-15, AC-17, R-10 and R-11, explicit source: the stale candidate C is
// cleared at boot, the file F is promoted over the unresolvable active S, the
// rebuild is logged naming S and the active pointer, and rollback R still
// names the version it named and still reads its bytes.
//
// run stops after initializeConfigSource on an unparsable looking-glass
// listen address, so the test proves the startup path without serving.
func TestRunRepairedStoreExplicitSource(t *testing.T) {
	scenario := buildRepairedStore(t)
	path := filepath.Join(scenario.dir, repairedName)
	fileF := []byte("set environment log level warn\n")
	if err := os.WriteFile(path, fileF, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(scenario.dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close() //nolint:errcheck // test cleanup
	bound := storage.BindConfigSource(store, path, storage.ConfigSourceFile)
	if _, _, present, err := storage.ReadCandidateConfig(bound, path); present || err == nil {
		t.Fatalf("candidate C must still be named at entry to run: present=%v err=%v", present, err)
	}

	const listenKey = "ze.looking-glass.listen"
	original := env.Get(listenKey)
	t.Cleanup(func() { _ = env.Set(listenKey, original) })
	if err := env.Set(listenKey, "not-an-endpoint"); err != nil {
		t.Fatal(err)
	}

	code, logged := runCapturingStderr(t, bound, path)
	if code != 1 || !strings.Contains(logged, listenKey) {
		t.Fatalf("run = %d, want the injected stop after initialization:\n%s", code, logged)
	}
	for _, want := range []string{"rebuilt active config from explicit file", zefs.KeyConfigActive.Key(repairedName), scenario.active, path} {
		if !strings.Contains(logged, want) {
			t.Fatalf("recovery log does not name %q:\n%s", want, logged)
		}
	}
	active, err := storage.ReadActiveConfig(bound, path)
	if err != nil || !bytes.Equal(active, fileF) {
		t.Fatalf("active = %q, %v; want F", active, err)
	}
	if _, _, present, err := storage.ReadCandidateConfig(bound, path); present || err != nil {
		t.Fatalf("candidate after startup: present=%v err=%v", present, err)
	}
	rollback, err := bound.ReadKey(zefs.KeyConfigRollback.Key(repairedName))
	if err != nil || strings.TrimSpace(string(rollback)) != scenario.rollback {
		t.Fatalf("rollback = %q, %v; want %s", rollback, err, scenario.rollback)
	}
	if content, err := bound.ReadVersion(path, scenario.rollback); err != nil || !bytes.Equal(content, scenario.bytesR) {
		t.Fatalf("rollback R = %q, %v", content, err)
	}
	disk, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(disk, fileF) {
		t.Fatalf("explicit file = %q, %v", disk, err)
	}
}
