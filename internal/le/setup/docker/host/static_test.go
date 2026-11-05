package dockerhost

import (
	"archive/tar"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// TestDockerHostFunctionalToolPayload builds the image through the recording
// builder and reads its actual extra-files archive. The functional harness
// needs full iproute2, nftables and capability tools in the own-system guest.
func TestDockerHostFunctionalToolPayload(t *testing.T) {
	repo := fakeRepo(t)
	recorder := newRecordingBuilder(t)
	_, err := recorder.build(context.Background(), buildRequest{
		repo: repo, arch: "arm64", pins: fullPins(t), sizeGiB: imageSizeGiBDefault,
	})
	if err != nil {
		t.Fatal(err)
	}
	members := readTar(t, filepath.Join(repo, "gokrazy", "docker-host", "extrafiles_arm64.tar"))
	for _, name := range []string{"ip", "nft", "setcap", "getcap"} {
		header, found := members[guestLibDir+"/"+name]
		if !found {
			t.Errorf("functional guest lacks %s", name)
			continue
		}
		if header.Typeflag != tar.TypeReg {
			t.Errorf("%s is not a regular executable payload", name)
		}
		if header.Mode != 0o755 {
			t.Errorf("%s mode = %o, want 755", name, header.Mode)
		}
		if header.Uid != 0 {
			t.Errorf("%s uid = %d, want root", name, header.Uid)
		}
		if header.Gid != 0 {
			t.Errorf("%s gid = %d, want root", name, header.Gid)
		}
	}
}

// TestBuilderContainerRefusesWithoutTheDockerCLI proves a host with no docker
// CLI gets the install fix before any container is attempted. Method: run the
// builder with an empty PATH and require the preflight refusal, whose text
// names the problem, the reason and the install command for both host kinds.
// Without the preflight the docker client's own lookup failure surfaces as a
// builder failure telling the operator to correct the build, and this fails.
//
// VALIDATES: review finding R2-1: `./le setup docker host build` on a host
// with no docker CLI names the missing tool and how to install it.
// PREVENTS: a missing CLI reported as a broken build recipe.
func TestBuilderContainerRefusesWithoutTheDockerCLI(t *testing.T) {
	t.Setenv("PATH", "")

	err := runBuilderContainer(context.Background(), containerRun{
		image:  "alpine@sha256:5291449c3df73caf6ed85e649dec1b9e818b39a5d8c871e97afc13e9cd5e8fa8",
		arch:   "arm64",
		inDir:  t.TempDir(),
		outDir: t.TempDir(),
		script: "true",
	})
	if !errors.Is(err, errDockerCLIMissing) {
		t.Fatalf("runBuilderContainer with no docker CLI = %v, want the docker CLI refusal", err)
	}
	for _, want := range []string{"not on PATH", "in a container", "brew install docker", "apt install docker.io"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not say %q", err, want)
		}
	}
}
