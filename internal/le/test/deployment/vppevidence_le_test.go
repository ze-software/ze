package testdeployment

import (
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/le/linuxle"
)

// TestVPPEvidenceBuildsLinuxLe pins where VPP evidence writes the linux le the
// container runs, and the command that starts the harness peer from it.
//
// VALIDATES: AC-41 of spec-le-subject-first-command-tree. The file is named
// linuxle.Name under tmp/evidence/bin/linux-<arch>, and the container starts
// the peer as `<that file> test peer`.
// PREVENTS: a file name other than le, which cmd/ze reads as another
// personality, and a peer started through a harness binary that no longer
// exists.
func TestVPPEvidenceBuildsLinuxLe(t *testing.T) {
	for _, goarch := range []string{"amd64", "arm64"} {
		rel := vppLeRel(goarch)
		if filepath.Base(rel) != linuxle.Name {
			t.Errorf("%s: the container le is %q, want a file named %q", goarch, rel, linuxle.Name)
		}
		wantRel := filepath.Join("tmp", "evidence", "bin", "linux-"+goarch, linuxle.Name)
		if rel != wantRel {
			t.Errorf("%s: vppLeRel = %q, want %q", goarch, rel, wantRel)
		}
		wantPeer := "/src/" + filepath.ToSlash(wantRel) + " test peer"
		if got := vppPeerCommand(goarch); got != wantPeer {
			t.Errorf("%s: vppPeerCommand = %q, want %q", goarch, got, wantPeer)
		}
	}
}
