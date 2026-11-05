// Design: docs/architecture/appliance/docker-host.md -- the extra files for functional test mounts.

package dockerhost

import (
	"archive/tar"
	"path/filepath"
	"testing"
)

// TestExtraFilesCarriesTheFunctionalWorkspace checks the produced archive, not
// its directory list: a read-only image needs the mountpoint before a guest can
// bind the shared checkout there for test qemu all.
func TestExtraFilesCarriesTheFunctionalWorkspace(t *testing.T) {
	repo := fakeRepo(t)
	static := t.TempDir()
	for _, name := range staticBinaryNames {
		writeFile(t, filepath.Join(static, name), []byte("static binary fixture"))
	}
	docker := filepath.Join(t.TempDir(), "docker.tgz")
	writeFile(t, docker, dockerTgz(t))
	archive := filepath.Join(t.TempDir(), "extra.tar")
	if err := writeExtraFiles(archive, extraFileInputs{
		busyboxTar: filepath.Join(repo, "cmd", "ze-serial-shell", "_gokrazy", "extrafiles_arm64.tar"),
		staticDir:  static,
		dockerTgz:  docker,
	}); err != nil {
		t.Fatal(err)
	}
	workspace := readTar(t, archive)["workspace/"]
	if workspace == nil {
		t.Fatal("the read-only image has no /workspace mountpoint")
	}
	if workspace.Typeflag != tar.TypeDir {
		t.Fatalf("/workspace type = %d, want directory", workspace.Typeflag)
	}
	if workspace.Mode != 0o755 {
		t.Fatalf("/workspace mode = %o, want 755", workspace.Mode)
	}
	if workspace.Uid != 0 {
		t.Fatalf("/workspace uid = %d, want root", workspace.Uid)
	}
	if workspace.Gid != 0 {
		t.Fatalf("/workspace gid = %d, want root", workspace.Gid)
	}
}
