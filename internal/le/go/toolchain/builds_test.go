// VALIDATES: spec-le-built-from-head AC-14. ParseBuildName reads the
// bin/builds name grammar the launcher writes, and RunnableBuild answers which
// build directories hold an le the launcher would exec. Which build is the
// newest is the script's choice, held to these two by
// TestLauncherAgreesWithTheResolver in cmd/ze.
// PREVENTS: a staging, rolled-back or malformed entry being read as a build, a
// guest's build read as the checkout's own, and a symbolic link or an empty
// directory counted as a build to run.

package gotoolchain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseBuildName is the name grammar's contract.
// Method: one table of names, each with the sha and platform it must answer,
// or none when the name is not a build.
func TestParseBuildName(t *testing.T) {
	sha40 := strings.Repeat("0123456789", 4)
	tests := []struct {
		name     string
		ok       bool
		sha      string
		platform string
	}{
		{name: "20261025T090000Z-bbbbbbb", ok: true, sha: "bbbbbbb"},
		{name: "20261025T100000Z-" + sha40, ok: true, sha: sha40},
		{name: "20261026T000000Z-ddddddd-linux-arm64", ok: true, sha: "ddddddd", platform: "linux-arm64"},
		{name: ".failed-20261027T000000Z-eeeeeee"},
		{name: ".building"},
		{name: ".staging-123"},
		{name: "20261027T000000Z-eeeeeee.new.123"},
		{name: "2026-10-27-eeeeeee"},
		{name: "20261027T000000Z-EEEEEEE"},
		{name: "20261027T000000Z-eeee"},
		{name: "20261027T000000Z-" + sha40 + "a"},
		{name: "20261327T000000Z-eeeeeee"},
		{name: "20261027T000000Z-eeeeeee-linux"},
		{name: "20261027T000000Z-eeeeeee-Linux-arm64"},
		{name: "20261027T000000Z-eeeeeee-linux-arm64-extra"},
		{name: "20261027T000000Z_eeeeeee"},
		{name: "20261027T000000Z"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseBuildName(tc.name)
			if ok != tc.ok {
				t.Fatalf("ParseBuildName(%q) ok = %v, want %v", tc.name, ok, tc.ok)
			}
			if got.Sha() != tc.sha {
				t.Errorf("sha = %q, want %q", got.Sha(), tc.sha)
			}
			if got.Platform() != tc.platform {
				t.Errorf("platform = %q, want %q", got.Platform(), tc.platform)
			}
		})
	}
}

// TestRunnableBuild proves only a directory holding a regular le is a build
// to run: a directory with no le, and an le that is a symbolic link, are not.
func TestRunnableBuild(t *testing.T) {
	root := t.TempDir()
	regular := filepath.Join(root, "regular")
	bare := filepath.Join(root, "bare")
	linked := filepath.Join(root, "linked")
	for _, dir := range []string{regular, bare, linked} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(regular, "le"), []byte("build"), 0o700); err != nil { //nolint:gosec // a stand-in executable
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(regular, "le"), filepath.Join(linked, "le")); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		dir  string
		want bool
	}{
		{regular, true},
		{bare, false},
		{linked, false},
	} {
		got, err := RunnableBuild(tc.dir)
		if err != nil {
			t.Fatalf("RunnableBuild(%s): %v", tc.dir, err)
		}
		if got != tc.want {
			t.Errorf("RunnableBuild(%s) = %v, want %v", filepath.Base(tc.dir), got, tc.want)
		}
	}
}

// TestBuildEntriesMatchTheShellScript pins the `le` script's copies of the
// entries it writes beside the builds, which the store trim judges by these
// constants: a rename on one side only would leave the trim removing nothing,
// or something the launcher still needs.
func TestBuildEntriesMatchTheShellScript(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	script, err := os.ReadFile(filepath.Join(root, "le"))
	if err != nil {
		t.Fatalf("read the le bootstrap script: %v", err)
	}
	for _, assignment := range []string{
		"aside=$builds/" + BuildFailedPrefix + "${1##*/}",
		"staging=$builds/" + BuildStagingPrefix + "$(cat \"$lock/pid\")",
		"lock=$builds/" + BuildLockEntry,
		"legacy=$builds/" + BuildLegacyEntry + "/le",
		"\"$builds/" + BuildFailedPrefix + "${head_commit:-$(git -C \"$root\" rev-parse HEAD)}.log\"",
		": >\"$builds/" + BuildFailedPrefix + "$head_full.reported\"",
	} {
		if !strings.Contains(string(script), assignment) {
			t.Errorf("the le script does not hold %q, so the store trim judges an entry the launcher does not write", assignment)
		}
	}
}
