package storage

import (
	"bytes"
	"crypto/rand"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// plantStages writes the folder entries the sweep tests judge, keyed by what
// each one is, and answers their names.
//
// Goal: one folder holds every shape a sweep could wrongly take. Method: a
// crash leftover of the live tree, then each neighbor the live owner MUST
// leave: a frame of a tree whose name extends
// `database`, a name with a non-base32 tail, an unrelated file, and a
// directory under a matching name. The legacy form, which no write produces
// any more, is the live owner's to remove as well.
func plantStages(t *testing.T, dir string) map[string]string {
	t.Helper()
	names := map[string]string{
		"leftover":      stagePrefix + treeName + "." + rand.Text(),
		"legacy":        stagePrefix + rand.Text(),
		"other tree":    stagePrefix + treeName + replacedInfix + "20260101T000000.000000000." + rand.Text(),
		"bad tail":      stagePrefix + treeName + ".not-base32",
		"unrelated":     "notes.txt",
		"matching dir":  stagePrefix + treeName + "." + rand.Text(),
		"empty tail":    stagePrefix + treeName + ".",
		"lowercase rnd": stagePrefix + treeName + "." + strings.ToLower(rand.Text()),
	}
	for shape, name := range names {
		if shape == "matching dir" {
			require.NoError(t, os.Mkdir(filepath.Join(dir, name), 0o700))
			continue
		}
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(shape), 0o600))
	}
	return names
}

func entryExists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false
	}
	require.NoError(t, err)
	return true
}

// TestOwnerOpenSweepsLeftoverStages proves the live owner removes its own
// crash leftovers, the legacy form included, and nothing else. Method: plant every shape, open as the
// owner through Open, and check each entry.
func TestOwnerOpenSweepsLeftoverStages(t *testing.T) {
	dir := t.TempDir()
	created, err := Create(dir)
	require.NoError(t, err)
	require.NoError(t, created.Close())
	names := plantStages(t, dir)

	s, err := Open(dir)
	require.NoError(t, err)
	defer s.Close() //nolint:errcheck // test cleanup.

	for shape, name := range names {
		want := shape != "leftover" && shape != "legacy"
		assert.Equal(t, want, entryExists(t, filepath.Join(dir, name)), "%s %s", shape, name)
	}
}

// TestReaderOpenSweepsNothing proves a read-only open beside the store removes
// no staged frame, because it holds no owner lock. Method: plant every shape,
// open with OpenReadOnly, and check that every entry is still there.
func TestReaderOpenSweepsNothing(t *testing.T) {
	dir := t.TempDir()
	created, err := Create(dir)
	require.NoError(t, err)
	require.NoError(t, created.Close())
	names := plantStages(t, dir)

	s, err := OpenReadOnly(dir)
	require.NoError(t, err)
	defer s.Close() //nolint:errcheck // test cleanup.

	for shape, name := range names {
		assert.True(t, entryExists(t, filepath.Join(dir, name)), "%s %s", shape, name)
	}
}

// TestOfflineOwnerSweepsOnlyItsTree proves a writable OpenTree of a retired
// tree removes the frames named for that tree and leaves the live tree's and
// the legacy form, which belong to the live owner of the same folder. Method:
// plant one of each, open the retired tree writable, check all three.
func TestOfflineOwnerSweepsOnlyItsTree(t *testing.T) {
	dir := t.TempDir()
	retired := treeName + replacedInfix + "20260101T000000.000000000"
	require.NoError(t, os.Mkdir(filepath.Join(dir, retired), 0o700))
	own := stagePrefix + retired + "." + rand.Text()
	live := stagePrefix + treeName + "." + rand.Text()
	legacy := stagePrefix + rand.Text()
	for _, name := range []string{own, live, legacy} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), nil, 0o600))
	}

	s, err := OpenTree(filepath.Join(dir, retired), true)
	require.NoError(t, err)
	defer s.Close() //nolint:errcheck // test cleanup.

	assert.False(t, entryExists(t, filepath.Join(dir, own)), "the retired tree's own leftover")
	assert.True(t, entryExists(t, filepath.Join(dir, live)), "the live tree's frame")
	assert.True(t, entryExists(t, filepath.Join(dir, legacy)), "the legacy frame")
}

// TestOfflineReaderSweepsNothing proves a read-only OpenTree of a retired tree
// leaves even the frames named for that tree. Method: plant one, open it
// read-only, check it.
func TestOfflineReaderSweepsNothing(t *testing.T) {
	dir := t.TempDir()
	retired := treeName + replacedInfix + "20260101T000000.000000000"
	require.NoError(t, os.Mkdir(filepath.Join(dir, retired), 0o700))
	own := filepath.Join(dir, stagePrefix+retired+"."+rand.Text())
	require.NoError(t, os.WriteFile(own, nil, 0o600))

	s, err := OpenTree(filepath.Join(dir, retired), false)
	require.NoError(t, err)
	defer s.Close() //nolint:errcheck // test cleanup.

	assert.True(t, entryExists(t, own))
}

// TestWriteStagesUnderItsLockName ties each producer to the sweep matcher: the
// name a real key write stages is one its own owner's sweep matches and the
// other tree's does not. Method: open the store each way a writer can, then
// capture the frame name at its sync barrier during WriteKey.
func TestWriteStagesUnderItsLockName(t *testing.T) {
	retired := treeName + replacedInfix + "20260101T000000.000000000"
	tests := []struct {
		name      string
		open      func(t *testing.T, dir string) Storage
		wantLock  string
		otherLock string
	}{
		{"Create", func(t *testing.T, dir string) Storage {
			s, err := Create(dir)
			require.NoError(t, err)
			return s
		}, treeName, retired},
		{"Open", func(t *testing.T, dir string) Storage {
			created, err := Create(dir)
			require.NoError(t, err)
			require.NoError(t, created.Close())
			s, err := Open(dir)
			require.NoError(t, err)
			return s
		}, treeName, retired},
		{"OpenTree retired", func(t *testing.T, dir string) Storage {
			require.NoError(t, os.Mkdir(filepath.Join(dir, retired), 0o700))
			s, err := OpenTree(filepath.Join(dir, retired), true)
			require.NoError(t, err)
			return s
		}, retired, treeName},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.open(t, t.TempDir())
			defer s.Close() //nolint:errcheck // test cleanup.
			concrete, ok := s.(*store)
			require.True(t, ok)
			var staged string
			concrete.tree.sync = func(file *os.File) error {
				if base := filepath.Base(file.Name()); strings.HasPrefix(base, stagePrefix) {
					staged = base
				}
				return file.Sync()
			}
			require.NoError(t, s.WriteKey("meta/test/key", []byte("value")))

			require.NotEmpty(t, staged)
			assert.True(t, stagedFor(staged, tt.wantLock), staged)
			assert.False(t, stagedFor(staged, tt.otherLock), staged)
			assert.False(t, stagedLegacy(staged), staged)
		})
	}
}

// TestSweepFailureDoesNotStopOpen proves the sweep is best-effort: a leftover
// that cannot be removed leaves Open succeeding and is reported with its path
// and the command that removes it. Method: make the folder read-only so the
// unlink fails, open the store through Open, then run the sweep itself over
// the same folder with a captured logger.
func TestSweepFailureDoesNotStopOpen(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root removes entries from a read-only folder")
	}
	dir := t.TempDir()
	created, err := Create(dir)
	require.NoError(t, err)
	require.NoError(t, created.Close())
	leftover := filepath.Join(dir, stagePrefix+treeName+"."+rand.Text())
	require.NoError(t, os.WriteFile(leftover, nil, 0o600))
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { require.NoError(t, os.Chmod(dir, 0o700)) })

	s, err := Open(dir)
	require.NoError(t, err, "a leftover the sweep cannot remove must not refuse the open")
	require.NoError(t, s.Close())
	assert.True(t, entryExists(t, leftover))

	folder, err := openFolder(dir)
	require.NoError(t, err)
	defer folder.Close() //nolint:errcheck // test cleanup.
	var out bytes.Buffer
	sweepStages(folder, liveLeftover, slog.New(slog.NewTextHandler(&out, nil)))
	logged := out.String()
	// The folder descriptor names the resolved directory, which the warning
	// reports (on darwin /var is a link to /private/var).
	reported := filepath.Join(folder.Name(), filepath.Base(leftover))
	assert.Contains(t, logged, "level=WARN")
	assert.Contains(t, logged, "could not remove a leftover staged write")
	assert.Contains(t, logged, "path="+reported)
	assert.Contains(t, logged, `fix="rm `+reported+`"`)
}

// TestWriteConfigFileNeedsTheOwnedStoreFolder proves a loose config is
// published only through the live owner of the folder that holds it, so the
// frame it stages is one that owner's sweep covers. Method: publish through
// the owner, then try a read-only store and a path outside the store folder,
// and check the refused files are untouched.
func TestWriteConfigFileNeedsTheOwnedStoreFolder(t *testing.T) {
	dir := t.TempDir()
	owner := newTreeStorage(t, dir)
	path := filepath.Join(dir, "router.conf")
	require.NoError(t, WriteConfigFile(owner, path, []byte("published")))
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "published", string(got))

	reader, err := OpenReadOnly(dir)
	require.NoError(t, err)
	defer reader.Close() //nolint:errcheck // test cleanup.
	err = WriteConfigFile(reader, path, []byte("refused"))
	require.ErrorContains(t, err, "read-only")

	elsewhere := filepath.Join(t.TempDir(), "router.conf")
	require.NoError(t, os.WriteFile(elsewhere, []byte("retain"), 0o600))
	err = WriteConfigFile(owner, elsewhere, []byte("refused"))
	require.ErrorContains(t, err, "not in the store folder")
	got, err = os.ReadFile(elsewhere)
	require.NoError(t, err)
	assert.Equal(t, "retain", string(got))
	got, err = os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "published", string(got))
}

// TestWriteConfigFileStagesUnderTheStoreLock proves the frame a loose config
// write stages is one the live owner's sweep matches, and that a memory store,
// which holds no lock, stages the reserved form no sweep matches. Method:
// capture the frame name at its sync barrier through writeConfigFile.
func TestWriteConfigFileStagesUnderTheStoreLock(t *testing.T) {
	capture := func(staged *string) func(*os.File) error {
		return func(file *os.File) error {
			if base := filepath.Base(file.Name()); strings.HasPrefix(base, stagePrefix) {
				*staged = base
			}
			return file.Sync()
		}
	}

	dir := t.TempDir()
	var live string
	require.NoError(t, writeConfigFile(newTreeStorage(t, dir), filepath.Join(dir, "router.conf"), []byte("live"), capture(&live)))
	assert.True(t, liveLeftover(live), live)
	assert.True(t, stagedFor(live, treeName), live)

	memoryDir := t.TempDir()
	var memory string
	path := filepath.Join(memoryDir, "router.conf")
	require.NoError(t, writeConfigFile(CreateMemory(), path, []byte("memory"), capture(&memory)))
	require.NotEmpty(t, memory)
	assert.False(t, liveLeftover(memory), memory)
	assert.False(t, stagedFor(memory, treeName+replacedInfix+"20260101T000000.000000000"), memory)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "memory", string(got))
}

// TestWriteConfigFileOnAMemoryStore proves the functional-test DUT's commit
// path: a memory store publishes its explicit config file. Method: publish
// through CreateMemory, read the file back.
func TestWriteConfigFileOnAMemoryStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "router.conf")
	require.NoError(t, WriteConfigFile(CreateMemory(), path, []byte("published")))
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "published", string(got))
}

// TestWriteConfigFileRefusesClosedAndBlobStores covers the two refusals the
// folder test does not: a closed store, and a store that is not a tree.
// Method: publish through each, check the error and that no file appears.
func TestWriteConfigFileRefusesClosedAndBlobStores(t *testing.T) {
	dir := t.TempDir()
	closed, err := Create(dir)
	require.NoError(t, err)
	require.NoError(t, closed.Close())
	path := filepath.Join(dir, "router.conf")
	require.ErrorIs(t, WriteConfigFile(closed, path, []byte("refused")), fs.ErrClosed)
	assert.False(t, entryExists(t, path))

	blobDir := t.TempDir()
	blob := newBlobStorageAt(t, blobDir)
	blobPath := filepath.Join(blobDir, "router.conf")
	require.ErrorContains(t, WriteConfigFile(blob, blobPath, []byte("refused")), "not a live tree")
	assert.False(t, entryExists(t, blobPath))
}
