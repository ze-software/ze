// VALIDATES: the ze-data CLI dispatch and pure arg helpers — extractPathFlag
// parses --path/--path= and returns the remaining args, Run returns the right
// exit codes for empty/help/unknown subcommands, and filePathToKey maps a file
// path to its active-config storage key by basename.
// PREVENTS: a --path flag leaking into the subcommand args, an unknown subcommand
// being dispatched, or a full path (not the basename) becoming the storage key.

package cli

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

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
