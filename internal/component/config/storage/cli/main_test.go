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
	"path/filepath"
	"testing"

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
	path, rem := extractPathFlag([]string{"--path", "/var/db.zefs", "ls", "meta"})
	if path != "/var/db.zefs" {
		t.Errorf("path = %q, want /var/db.zefs", path)
	}
	if len(rem) != 2 || rem[0] != "ls" || rem[1] != "meta" {
		t.Errorf("remaining = %v, want [ls meta]", rem)
	}

	path, rem = extractPathFlag([]string{"cat", "--path=/x/y.zefs", "key"})
	if path != "/x/y.zefs" {
		t.Errorf("path= form: path = %q, want /x/y.zefs", path)
	}
	if len(rem) != 2 || rem[0] != "cat" || rem[1] != "key" {
		t.Errorf("path= form: remaining = %v, want [cat key]", rem)
	}
}

// VALIDATES: with no --path, the store defaults to {ze.config.dir}/database.zefs.
// PREVENTS: the regression where `ze data check` opened
// {binary-prefix}/etc/ze/database.zefs while `ze init` had written
// $ZE_CONFIG_DIR/database.zefs, so check reported ENOENT on a healthy store.
func TestExtractPathFlag_DefaultsToConfigDirEnv(t *testing.T) {
	pinned := t.TempDir()
	setConfigDirEnv(t, pinned)

	path, rem := extractPathFlag([]string{"check"})
	if want := filepath.Join(pinned, defaultBlobName); path != want {
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
