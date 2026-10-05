package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/zefs"
)

// restoreDestination builds a live tree holding one old key and an unrelated
// canonical seed beside it, the two destinations a full restore retires.
func restoreDestination(t *testing.T) string {
	t.Helper()
	dir := storeSpelling(t, filepath.Join(t.TempDir(), "config"))
	require.NoError(t, os.Mkdir(dir, 0o700))
	old := newTreeStorage(t, dir)
	require.NoError(t, old.WriteKey("meta/old/key", []byte("old tree")))
	require.NoError(t, old.Close())
	// CreateBlob refuses the canonical name beside a live tree, so the seed is
	// written elsewhere and moved in, as an operator's copy would arrive.
	elsewhere := filepath.Join(t.TempDir(), "seed.zefs")
	seed, err := CreateBlob(elsewhere)
	require.NoError(t, err)
	require.NoError(t, seed.WriteKey("meta/seed/key", []byte("unrelated seed")))
	require.NoError(t, seed.Close())
	require.NoError(t, os.Rename(elsewhere, filepath.Join(dir, blobName)))
	return dir
}

// assertRetired proves exactly one node sits at name.replaced-* and that it
// holds key with want.
func assertRetired(t *testing.T, dir, name, key, want string) {
	t.Helper()
	names, err := filepath.Glob(filepath.Join(dir, name+replacedInfix+"*"))
	require.NoError(t, err)
	var retired []string
	for _, candidate := range names {
		if filepath.Ext(candidate) != ".lock" {
			retired = append(retired, candidate)
		}
	}
	require.Len(t, retired, 1, name)
	var s Storage
	if name == treeName {
		s, err = OpenTree(retired[0], false)
	} else {
		s, err = OpenBlob(retired[0], false)
	}
	require.NoError(t, err)
	defer s.Close() //nolint:errcheck // read-only fixture.
	got, err := s.ReadKey(key)
	require.NoError(t, err)
	assert.Equal(t, want, string(got))
}

// TestRestoreBlobReplacesTree proves AC-5: the tree holds exactly the source's
// keys, the old tree and the unrelated seed sit at their retirement names, the
// source and its lock stay at their names byte-unchanged, and no intent is left.
func TestRestoreBlobReplacesTree(t *testing.T) {
	source, values := importFixture(t)
	before, err := os.ReadFile(source)
	require.NoError(t, err)
	dir := restoreDestination(t)
	s, err := RestoreBlob(source, dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })
	assertImportValues(t, s, values)
	assertRetired(t, dir, treeName, "meta/old/key", "old tree")
	assertRetired(t, dir, blobName, "meta/seed/key", "unrelated seed")
	after, err := os.ReadFile(source)
	require.NoError(t, err)
	assert.Equal(t, before, after)
	require.FileExists(t, source+".lock")
	require.Empty(t, archivesOf(t, source))
	require.NoFileExists(t, filepath.Join(dir, importIntentName))
	require.NoError(t, s.Close())
	reopened, err := Open(dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reopened.Close()) })
	assertImportValues(t, reopened, values)
}

// TestRestoreBlobRefusesCanonicalSeed proves the storage half of AC-22: the
// canonical seed name is refused and nothing moves.
func TestRestoreBlobRefusesCanonicalSeed(t *testing.T) {
	dir := restoreDestination(t)
	_, err := RestoreBlob(filepath.Join(dir, blobName), dir)
	require.ErrorContains(t, err, "canonical seed")
	require.ErrorContains(t, err, "ze init --from")
	require.FileExists(t, filepath.Join(dir, blobName))
	require.NoFileExists(t, filepath.Join(dir, importIntentName))
	s, err := OpenTree(filepath.Join(dir, treeName), false)
	require.ErrorContains(t, err, "not a live store")
	if s != nil {
		require.NoError(t, s.Close())
	}
}

// interruptedRestore stops a keep-source restore at state after its intent is
// durable: "pre-move" (nothing moved), "tree-moved" (old tree retired, seed
// not), "absent" (both retired, stage unpublished), "published" (stage at
// database, intent not removed). It builds the stage and intent with the
// production helpers, so the intent is the one a crash would leave.
func interruptedRestore(t *testing.T, state string) (string, string, map[string][]byte, importIntent) {
	t.Helper()
	source, values := importFixture(t)
	dir := restoreDestination(t)
	folder, err := openFolder(dir)
	require.NoError(t, err)
	defer folder.Close() //nolint:errcheck // fixture handle.
	stamp := time.Now().Format("20060102T150405.000000000")
	intent := importIntent{Policy: policyKeepSource, Source: source}
	intent.Digest, err = blobDigest(source)
	require.NoError(t, err)
	intent.Tree, err = recordPrevious(folder, treeName, true, stamp)
	require.NoError(t, err)
	intent.Seed, err = recordPrevious(folder, blobName, false, stamp)
	require.NoError(t, err)
	intent.Stage, err = makeStage(folder, importStagePrefix)
	require.NoError(t, err)
	blob, err := zefs.Open(source)
	require.NoError(t, err)
	built, err := buildStage(folder, intent.Stage, blob)
	require.NoError(t, blob.Close())
	require.NoError(t, err)
	intent.Device, intent.Inode = built.Device, built.Inode
	encoded, err := json.Marshal(intent)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, importIntentName), encoded, 0o600))
	move := func(from, to string) { require.NoError(t, os.Rename(filepath.Join(dir, from), filepath.Join(dir, to))) }
	switch state {
	case "pre-move":
	case "tree-moved":
		move(treeName, intent.Tree.Retired)
	case "absent":
		move(treeName, intent.Tree.Retired)
		move(blobName, intent.Seed.Retired)
	case "published":
		move(treeName, intent.Tree.Retired)
		move(blobName, intent.Seed.Retired)
		move(intent.Stage, treeName)
	default:
		t.Fatalf("unknown state %s", state)
	}
	return source, dir, values, intent
}

// TestRestoreResumesInterrupted proves AC-23: every opener reports pending with
// the restore command, even beside the old tree, and that command alone
// finishes each state without inventing another stage or retirement name.
func TestRestoreResumesInterrupted(t *testing.T) {
	seed := func(dir string) (Storage, error) { return CreatePopulated(dir, func(Storage) error { return nil }) }
	openers := []func(string) (Storage, error){Open, OpenReadOnly, Create, seed}
	for _, state := range []string{"pre-move", "tree-moved", "absent", "published"} {
		t.Run(state, func(t *testing.T) {
			source, dir, values, intent := interruptedRestore(t, state)
			before, err := os.ReadFile(source)
			require.NoError(t, err)
			for _, open := range openers {
				s, err := open(dir)
				if s != nil {
					require.NoError(t, s.Close())
				}
				require.ErrorIs(t, err, ErrImportPending)
				assert.Contains(t, err.Error(), "ze data restore "+source+" full")
			}
			s, err := RestoreBlob(source, dir)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, s.Close()) })
			assertImportValues(t, s, values)
			require.DirExists(t, filepath.Join(dir, intent.Tree.Retired))
			require.FileExists(t, filepath.Join(dir, intent.Seed.Retired))
			assertRetired(t, dir, treeName, "meta/old/key", "old tree")
			assertRetired(t, dir, blobName, "meta/seed/key", "unrelated seed")
			require.NoDirExists(t, filepath.Join(dir, intent.Stage))
			require.NoFileExists(t, filepath.Join(dir, importIntentName))
			after, err := os.ReadFile(source)
			require.NoError(t, err)
			assert.Equal(t, before, after)
			require.FileExists(t, source+".lock")
			require.Empty(t, archivesOf(t, source))
		})
	}
}

// TestRestoreReplayRefusesBeforeMutation proves the AC-23 refusals: each one
// keeps the intent, the stage and every recorded node where they were.
func TestRestoreReplayRefusesBeforeMutation(t *testing.T) {
	for _, change := range []string{"unrelated tree", "missing policy", "unknown policy", "missing state", "unknown state", "changed source", "missing stage", "policy mismatch", "seed before tree"} {
		t.Run(change, func(t *testing.T) {
			state := "tree-moved"
			if change == "seed before tree" {
				state = "pre-move"
			}
			source, dir, _, intent := interruptedRestore(t, state)
			intentPath := filepath.Join(dir, importIntentName)
			rewrite := func(edit func(map[string]any)) {
				data, err := os.ReadFile(intentPath)
				require.NoError(t, err)
				var fields map[string]any
				require.NoError(t, json.Unmarshal(data, &fields))
				edit(fields)
				data, err = json.Marshal(fields)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(intentPath, data, 0o600))
			}
			switch change {
			case "unrelated tree":
				// Built by hand: every opener refuses beside the intent.
				require.NoError(t, os.Mkdir(filepath.Join(dir, treeName), 0o700))
				folder, err := openFolder(dir)
				require.NoError(t, err)
				unrelated, err := openTree(folder, treeName, nil, false)
				require.NoError(t, err)
				require.NoError(t, unrelated.WriteKey("meta/unrelated/key", []byte("keep")))
				require.NoError(t, unrelated.Close())
			case "missing policy":
				rewrite(func(fields map[string]any) { delete(fields, "policy") })
			case "unknown policy":
				rewrite(func(fields map[string]any) { fields["policy"] = "move-source" })
			case "missing state", "unknown state":
				rewrite(func(fields map[string]any) {
					tree, ok := fields["tree"].(map[string]any)
					require.True(t, ok)
					if change == "missing state" {
						delete(tree, "state")
					} else {
						tree["state"] = "future"
					}
				})
			case "changed source":
				s, err := OpenBlob(source, true)
				require.NoError(t, err)
				require.NoError(t, s.WriteKey("meta/changed/key", []byte("changed")))
				require.NoError(t, s.Close())
			case "missing stage":
				require.NoError(t, os.Rename(filepath.Join(dir, intent.Stage), filepath.Join(dir, "elsewhere")))
			case "seed before tree":
				require.NoError(t, os.Rename(filepath.Join(dir, blobName), filepath.Join(dir, intent.Seed.Retired)))
			}
			var err error
			if change == "policy mismatch" {
				_, err = ImportBlob(source, dir)
			} else {
				_, err = RestoreBlob(source, dir)
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), importIntentName)
			require.FileExists(t, intentPath)
			require.FileExists(t, source)
			require.Empty(t, archivesOf(t, source))
			if change != "missing stage" {
				require.DirExists(t, filepath.Join(dir, intent.Stage))
			}
			if change == "seed before tree" {
				require.DirExists(t, filepath.Join(dir, treeName))
				require.NoDirExists(t, filepath.Join(dir, intent.Tree.Retired))
				return
			}
			require.DirExists(t, filepath.Join(dir, intent.Tree.Retired))
			require.FileExists(t, filepath.Join(dir, blobName))
			if change == "unrelated tree" {
				folder, err := openFolder(dir)
				require.NoError(t, err)
				s, err := openTree(folder, treeName, nil, true)
				require.NoError(t, err)
				defer s.Close() //nolint:errcheck // read-only fixture.
				got, err := s.ReadKey("meta/unrelated/key")
				require.NoError(t, err)
				assert.Equal(t, "keep", string(got))
			}
		})
	}
}
