package dockerhost

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSourceDigestCoversEveryPathThatShapesTheImage builds a fake repository
// holding one source file and one test file under each path that shapes the
// image, then changes each file in turn and compares the digest.
//
// VALIDATES: review R1-3: a change to the guest program, the instance, this
// package (extra files, modes, static recipe), the instance preparation or
// the gok run gives a new image key; a test file or testdata does not.
// PREVENTS: `./le setup docker host start` reusing a cached image built
// from code that has since changed.
func TestSourceDigestCoversEveryPathThatShapesTheImage(t *testing.T) {
	shaping := []string{
		"cmd/ze-docker-host",
		"internal/appliance/dockerhost",
		"gokrazy/docker-host",
		"internal/le/setup/docker/host",
		"internal/appliance/instance",
		"internal/le/build/gokrazy",
	}
	repo := t.TempDir()
	write := func(relative, content string) {
		t.Helper()
		path := filepath.Join(repo, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range shaping {
		write(dir+"/source.go", "package x\n")
		write(dir+"/source_test.go", "package x\n")
		write(dir+"/testdata/fixture", "fixture\n")
	}
	write("cmd/ze-serial-shell/_gokrazy/extrafiles_arm64.tar", "busybox\n")

	base, err := sourceDigest(repo, "arm64", 8)
	if err != nil {
		t.Fatalf("sourceDigest: %v", err)
	}
	for _, dir := range shaping {
		write(dir+"/source.go", "package x // changed\n")
		changed, err := sourceDigest(repo, "arm64", 8)
		if err != nil {
			t.Fatalf("sourceDigest after changing %s: %v", dir, err)
		}
		if changed == base {
			t.Errorf("changing %s/source.go kept the digest", dir)
		}
		write(dir+"/source.go", "package x\n")

		write(dir+"/source_test.go", "package x // changed\n")
		write(dir+"/testdata/fixture", "changed\n")
		unchanged, err := sourceDigest(repo, "arm64", 8)
		if err != nil {
			t.Fatalf("sourceDigest after changing %s tests: %v", dir, err)
		}
		if unchanged != base {
			t.Errorf("changing %s's test file or testdata changed the digest", dir)
		}
		write(dir+"/source_test.go", "package x\n")
		write(dir+"/testdata/fixture", "fixture\n")
	}
}
