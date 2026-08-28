package fixture

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInstallISOFixtures(t *testing.T) {
	dir := t.TempDir()
	kernel := filepath.Join(dir, "Image")
	image := filepath.Join(dir, "appliance.img")
	if err := os.WriteFile(image, []byte("image-bytes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := isoAMD64Prepare(context.Background(), []string{kernel, image}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(kernel)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0x206 || binary.LittleEndian.Uint32(data[0x202:]) != 0x53726448 {
		t.Fatalf("amd64 kernel image has length %d and magic %#x", len(data), binary.LittleEndian.Uint32(data[0x202:]))
	}
	sum := sha256.Sum256([]byte("image-bytes\n"))
	wantSidecar := fmt.Sprintf("%x  appliance.img\n", sum)
	gotSidecar, err := os.ReadFile(image + ".sha256")
	if err != nil {
		t.Fatal(err)
	}
	if string(gotSidecar) != wantSidecar {
		t.Fatalf("checksum sidecar = %q, want %q", gotSidecar, wantSidecar)
	}

	archive := filepath.Join(dir, "appliance.img.gz")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	writer := gzip.NewWriter(file)
	if _, err := writer.Write([]byte("image-bytes\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := isoCheckImage(context.Background(), []string{archive}); err != nil {
		t.Fatal(err)
	}
}

func TestInstallNativeKernelFixtures(t *testing.T) {
	ctx := context.Background()
	if err := kernelAutoBuilderFixture(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if err := kernelQEMUFixture(ctx, []string{"../../.."}); err != nil {
		t.Fatal(err)
	}
	if err := kernelBuilderNativeFixture(ctx, []string{"../../.."}); err != nil {
		t.Fatal(err)
	}
	if err := kernelBuilderPackagesFixture(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if err := kernelBuilderSingleDriverFixture(ctx, []string{"../../.."}); err != nil {
		t.Fatal(err)
	}
	if err := kernelQEMUArchAliasFixture(ctx, nil); err != nil {
		t.Fatal(err)
	}
	repo, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	// "true" is looked up on PATH rather than hardcoded at /bin/true: recent
	// macOS releases ship it at /usr/bin/true instead, and /bin/true no
	// longer exists there.
	noop, err := exec.LookPath("true")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(noop, filepath.Join(bin, "docker")); err != nil {
		t.Fatal(err)
	}
	restore := setFixtureEnv("PATH", bin)
	defer restore()
	if err := kernelBuildOwnershipFixture(ctx, []string{repo, filepath.Join(t.TempDir(), "ownership")}); err != nil {
		t.Fatal(err)
	}
	provenance := filepath.Join(t.TempDir(), "provenance")
	if err := kernelVersionProvenanceFixture(ctx, []string{repo, provenance, "7.1.1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(provenance, "kernel.version")); err != nil {
		t.Fatal(err)
	}
	if err := kernelVersionProvenanceFixture(ctx, []string{repo, provenance, "6.12.9"}); err == nil {
		t.Fatal("pre-7 kernel version was accepted")
	}
}
