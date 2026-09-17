// VALIDATES: the ze-data CLI dispatch and pure arg helpers — extractPathFlag
// parses --path/--path= and returns the remaining args, defaults the store to
// the resolved config dir when --path is absent, Run returns the right exit
// codes for empty/help/unknown subcommands, and filePathToKey maps a file path
// to its active-config storage key by basename.
// PREVENTS: a --path flag leaking into the subcommand args, an unknown subcommand
// being dispatched, a full path (not the basename) becoming the storage key, or
// `ze data` resolving a store that `ze init` never wrote.

package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/zefs"
)

// setConfigDirEnv pins ze.config.dir for the duration of the test.
func setConfigDirEnv(t *testing.T, value string) {
	t.Helper()
	orig := env.Get("ze.config.dir")
	t.Cleanup(func() { _ = env.Set("ze.config.dir", orig) })
	if err := env.Set("ze.config.dir", value); err != nil {
		t.Fatalf("env.Set ze.config.dir: %v", err)
	}
}

func TestExtractPathFlag(t *testing.T) {
	path, rem := extractPathFlag([]string{"--path", "/var/db.zefs", "list", "meta"})
	if path != "/var/db.zefs" {
		t.Errorf("path = %q, want /var/db.zefs", path)
	}
	if len(rem) != 2 || rem[0] != "list" || rem[1] != "meta" {
		t.Errorf("remaining = %v, want [list meta]", rem)
	}

	path, rem = extractPathFlag([]string{"cat", "--path=/x/y.zefs", "key"})
	if path != "/x/y.zefs" {
		t.Errorf("path= form: path = %q, want /x/y.zefs", path)
	}
	if len(rem) != 2 || rem[0] != "cat" || rem[1] != "key" {
		t.Errorf("path= form: remaining = %v, want [cat key]", rem)
	}
}

// The data CLI and init must resolve the same configured store directory.
func TestExtractPathFlag_DefaultsToConfigDirEnv(t *testing.T) {
	pinned := t.TempDir()
	setConfigDirEnv(t, pinned)

	path, rem := extractPathFlag([]string{"check"})
	if want := filepath.Join(pinned, defaultStoreName); path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	if len(rem) != 1 || rem[0] != "check" {
		t.Errorf("remaining = %v, want [check]", rem)
	}
}

// VALIDATES: an explicit --path still wins over ze.config.dir.
// PREVENTS: the env override hijacking a store the operator named explicitly.
func TestExtractPathFlag_ExplicitPathBeatsEnv(t *testing.T) {
	setConfigDirEnv(t, t.TempDir())

	explicit := filepath.Join(t.TempDir(), "explicit.zefs")
	path, _ := extractPathFlag([]string{"check", "--path", explicit})
	if path != explicit {
		t.Errorf("path = %q, want %q (--path must win over env)", path, explicit)
	}
}

func TestRunDispatchCodes(t *testing.T) {
	if rc := Run(nil); rc != 1 {
		t.Errorf("Run(nil) = %d, want 1 (usage)", rc)
	}
	if rc := Run([]string{"help"}); rc != 0 {
		t.Errorf("Run(help) = %d, want 0", rc)
	}
	if rc := Run([]string{"definitely-not-a-subcommand"}); rc != 1 {
		t.Errorf("Run(unknown) = %d, want 1", rc)
	}
}

func TestFilePathToKey(t *testing.T) {
	got := filePathToKey("/etc/ze/ze.conf")
	want := zefs.KeyFileActive.Key("ze.conf")
	if got != want {
		t.Errorf("filePathToKey = %q, want %q (basename-keyed)", got, want)
	}
}

// Raw namespaces and recursive listing must survive the CLI, on both encodings.
func TestDataRawKeyParity(t *testing.T) {
	for _, blob := range []bool{false, true} {
		name := "tree"
		if blob {
			name = "blob"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "database")
			var owner storage.Storage
			var err error
			if blob {
				path = filepath.Join(dir, "artifact.zefs")
				owner, err = storage.CreateBlob(path)
			} else {
				owner, err = storage.Create(dir)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(t.TempDir(), "input.conf")
			value := []byte("raw-value\n")
			if err := os.WriteFile(source, value, 0o600); err != nil {
				t.Fatal(err)
			}
			want := []string{"custom/nested/value", "file/active/input.conf", "meta/example"}
			for _, key := range []string{want[0], want[2]} {
				if code := cmdWrite(path, []string{key, source}); code != 0 {
					t.Fatalf("write %s returned %d", key, code)
				}
			}
			if code := cmdImport(path, []string{source}); code != 0 {
				t.Fatalf("import returned %d", code)
			}
			reader, err := openStore(path, false)
			if err != nil {
				t.Fatal(err)
			}
			keys, err := reader.ListKeys("")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(keys, want) {
				t.Fatalf("keys = %v, want %v", keys, want)
			}
			for _, key := range want {
				data, err := reader.ReadKey(key)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(data, value) {
					t.Fatalf("%s = %q", key, data)
				}
			}
			if err := reader.Close(); err != nil {
				t.Fatal(err)
			}
			answer, code := dataList([]string{"--path", path, "custom/"})
			if code != 0 {
				t.Fatalf("list returned %d", code)
			}
			fields, ok := answer.(map[string]any)
			if !ok {
				t.Fatalf("list answer = %T", answer)
			}
			rows, ok := fields["keys"].([]map[string]any)
			if !ok {
				t.Fatalf("list keys = %T", fields["keys"])
			}
			if len(rows) != 1 {
				t.Fatalf("recursive list = %v", rows)
			}
			if rows[0]["key"] != want[0] {
				t.Fatalf("recursive list = %v", rows)
			}
			if code := cmdRm(path, []string{want[0]}); code != 0 {
				t.Fatalf("remove returned %d", code)
			}
			reader, err = openStore(path, false)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = reader.Close() }()
			if _, err := reader.ReadKey(want[0]); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("removed key read: %v", err)
			}
		})
	}
}

// Offline mutations must refuse a live owner while read-only inspection works.
func TestDataRefusesOwnedStoreMutation(t *testing.T) {
	dir := t.TempDir()
	owner, err := storage.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = owner.Close() }()
	if err := owner.WriteKey("meta/held", []byte("keep")); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "database")
	if code := cmdRm(path, []string{"meta/held"}); code != 2 {
		t.Fatalf("owned-store remove returned %d", code)
	}
	if _, code := dataList([]string{"--path", path}); code != 0 {
		t.Fatalf("read-only inspection returned %d", code)
	}
	data, err := owner.ReadKey("meta/held")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "keep" {
		t.Fatalf("held key = %q", data)
	}
}
