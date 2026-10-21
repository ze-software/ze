package dockerhost

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakeRepo writes the files the build digests and reads: the guest program,
// the instance, and ze-serial-shell's busybox tar. It answers the repo root.
func fakeRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	for _, root := range digestRoots {
		writeFile(t, filepath.Join(repo, root, "file.go"), []byte("package x\n"))
	}
	var busybox bytes.Buffer
	archive := tar.NewWriter(&busybox)
	body := []byte("busybox binary")
	if err := archive.WriteHeader(&tar.Header{Name: busyboxMember, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, "cmd", "ze-serial-shell", "_gokrazy", "extrafiles_arm64.tar"), busybox.Bytes())
	return repo
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// dockerTgz is a minimal Docker static tarball holding docker/dockerd.
func dockerTgz(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	zipped := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(zipped)
	body := []byte("dockerd binary")
	if err := archive.WriteHeader(&tar.Header{Name: "docker/dockerd", Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zipped.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// fullPins writes a pin file with the arm64 docker download and the three
// sources the static build needs.
func fullPins(t *testing.T) *Pins {
	t.Helper()
	sum := strings.Repeat("a", 64)
	body := `{"builder": "alpine@sha256:` + strings.Repeat("b", 64) + `",
"downloads": [{"name": "docker", "version": "29.9.2", "arch": "arm64", "url": "https://example.invalid/docker-29.9.2.tgz", "sha256": "` + sum + `"}],
"sources": [
 {"name": "openssh", "version": "9.9p2", "url": "https://example.invalid/openssh-9.9p2.tar.gz", "sha256": "` + sum + `"},
 {"name": "sudo", "version": "1.9.16p2", "url": "https://example.invalid/sudo-1.9.16p2.tar.gz", "sha256": "` + sum + `"},
 {"name": "iptables", "version": "1.8.13", "url": "https://example.invalid/iptables-1.8.13.tar.xz", "sha256": "` + sum + `"}]}`
	path := filepath.Join(t.TempDir(), "binaries.json")
	writeFile(t, path, []byte(body))
	pins, err := LoadPins(path)
	if err != nil {
		t.Fatalf("LoadPins: %v", err)
	}
	return pins
}

// recordingBuilder is a builder whose steps record their calls.
type recordingBuilder struct {
	builder
	gokRuns       [][]string
	containerRuns []containerRun
	formatted     []string
}

func newRecordingBuilder(t *testing.T) *recordingBuilder {
	t.Helper()
	cache := t.TempDir()
	downloads := filepath.Join(cache, "downloads")
	docker := dockerTgz(t)
	recorder := &recordingBuilder{}
	recorder.builder = builder{
		cacheDir: cache,
		kernelTree: func(string) (string, error) {
			return "/cache/7.2.9-runtime-arm64-runtime-docker-387d6b76-34cba457", nil
		},
		fetch: func(_ context.Context, _ *Pins, pin Pin) (string, error) {
			path := filepath.Join(downloads, fileName(pin))
			body := []byte("source " + pin.Name)
			if pin.Name == "docker" {
				body = docker
			}
			writeFile(t, path, body)
			return path, nil
		},
		runContainer: func(_ context.Context, run containerRun) error {
			recorder.containerRuns = append(recorder.containerRuns, run)
			for _, name := range staticBinaryNames {
				writeFile(t, filepath.Join(run.outDir, name), []byte(name+" binary"))
			}
			return nil
		},
		gok: func(_ string, args []string) error {
			recorder.gokRuns = append(recorder.gokRuns, args)
			image := args[slices.Index(args, "--full")+1]
			writeFile(t, image, []byte("raw image"))
			return nil
		},
		formatPerm: func(image string) error {
			recorder.formatted = append(recorder.formatted, image)
			return nil
		},
		progress: io.Discard,
	}
	return recorder
}

// VALIDATES: wiring row 1 and spec AC-5 runs one and two: `build` fetches the
// pins, runs the builder container once, writes the instance's extra files
// tar (sudo setuid root, busybox, Docker's binaries, all root-owned), runs gok
// on the docker-host instance, formats /perm, and a second build with the
// same inputs answers the same path without running gok.
// PREVENTS: an image built from unpinned files, a sudo that cannot elevate,
// and a rebuild of an image the cache already holds.
func TestDockerHostBuildActionBuildsTheInstance(t *testing.T) {
	repo := fakeRepo(t)
	recorder := newRecordingBuilder(t)
	req := buildRequest{repo: repo, arch: "arm64", pins: fullPins(t), sizeGiB: imageSizeGiBDefault}

	first, err := recorder.build(context.Background(), req)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !first.Built {
		t.Error("the first build reported the cache held the image")
	}
	if want := filepath.Join(recorder.cacheDir, "arm64", first.Key, imageFileName); first.Path != want {
		t.Errorf("image at %s, want %s", first.Path, want)
	}
	if _, err := os.Stat(first.Path); err != nil {
		t.Errorf("no image at the answered path: %v", err)
	}
	if len(recorder.gokRuns) != 1 {
		t.Fatalf("gok ran %d times, want 1", len(recorder.gokRuns))
	}
	args := strings.Join(recorder.gokRuns[0], " ")
	for _, want := range []string{"-i docker-host", "overwrite", "--full ", "--target_storage_bytes 17179869184", filepath.Join(repo, "gokrazy")} {
		if !strings.Contains(args, want) {
			t.Errorf("gok args %q lack %q", args, want)
		}
	}
	if len(recorder.formatted) != 1 {
		t.Errorf("/perm formatted %d times, want 1", len(recorder.formatted))
	}
	if len(recorder.containerRuns) != 1 {
		t.Fatalf("builder container ran %d times, want 1", len(recorder.containerRuns))
	}
	run := recorder.containerRuns[0]
	if run.image != req.pins.Builder() || run.arch != "arm64" {
		t.Errorf("container ran %s for %s, want the pinned builder for arm64", run.image, run.arch)
	}
	for _, want := range []string{"openssh_tar='openssh-9.9p2.tar.gz'", "sudo_tar='sudo-1.9.16p2.tar.gz'", "iptables_tar='iptables-1.8.13.tar.xz'"} {
		if !strings.Contains(run.script, want) {
			t.Errorf("recipe lacks %s", want)
		}
	}

	members := readTar(t, filepath.Join(repo, "gokrazy", "docker-host", "extrafiles_arm64.tar"))
	for name, mode := range map[string]int64{
		guestLibDir + "/sudo":           0o4755,
		guestLibDir + "/sshd":           0o755,
		guestLibDir + "/busybox":        0o755,
		guestLibDir + "/docker/dockerd": 0o755,
		"mnt/lima-cidata/":              0o755,
	} {
		header, found := members[name]
		if !found {
			t.Errorf("extra files lack %s", name)
			continue
		}
		if header.Mode != mode || header.Uid != 0 || header.Gid != 0 {
			t.Errorf("%s mode %o uid %d gid %d, want %o root", name, header.Mode, header.Uid, header.Gid, mode)
		}
	}

	second, err := recorder.build(context.Background(), req)
	if err != nil {
		t.Fatalf("second build: %v", err)
	}
	if second.Built || second.Path != first.Path || len(recorder.gokRuns) != 1 || len(recorder.containerRuns) != 1 {
		t.Errorf("second build: built %v path %s, gok runs %d, container runs %d; want the cached image and no run",
			second.Built, second.Path, len(recorder.gokRuns), len(recorder.containerRuns))
	}
}

// VALIDATES: spec AC-5 run three: a fetch refusal stops the build before any
// image or extra files tar is written.
// PREVENTS: a partly built image from bytes that are not the pinned ones.
func TestDockerHostBuildStopsAtAFetchRefusal(t *testing.T) {
	repo := fakeRepo(t)
	recorder := newRecordingBuilder(t)
	refusal := errors.New("sha256 is x, but pins y")
	recorder.fetch = func(context.Context, *Pins, Pin) (string, error) { return "", refusal }
	_, err := recorder.build(context.Background(), buildRequest{repo: repo, arch: "arm64", pins: fullPins(t), sizeGiB: 2})
	if !errors.Is(err, refusal) {
		t.Fatalf("build answered %v, want the fetch refusal", err)
	}
	if len(recorder.gokRuns) != 0 || len(recorder.containerRuns) != 0 {
		t.Errorf("gok ran %d times and the container %d after the refusal", len(recorder.gokRuns), len(recorder.containerRuns))
	}
	if _, err := os.Stat(filepath.Join(recorder.cacheDir, "arm64")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the refusal left a cache entry: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "gokrazy", "docker-host", "extrafiles_arm64.tar")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the refusal wrote the extra files tar: %v", err)
	}
}

// VALIDATES: the size keyword's bounds (2 to 1024 GiB) and that the size is
// part of the image key.
// PREVENTS: an image too small for /perm, and a resized image reusing the old key.
func TestDockerHostBuildSizeBounds(t *testing.T) {
	cases := []struct {
		size string
		ok   bool
	}{{"1", false}, {"2", true}, {"1024", true}, {"1025", false}, {"x", false}}
	for _, tc := range cases {
		_, err := parseBuildRequest("arm64", tc.size, filepath.Join("..", "..", "..", "..", PinFile))
		if (err == nil) != tc.ok {
			t.Errorf("size %s: err %v, want ok %v", tc.size, err, tc.ok)
		}
	}
	repo := fakeRepo(t)
	small, err := sourceDigest(repo, "arm64", 2)
	if err != nil {
		t.Fatal(err)
	}
	large, err := sourceDigest(repo, "arm64", 3)
	if err != nil {
		t.Fatal(err)
	}
	if small == large {
		t.Error("the image size does not change the source digest")
	}
}

func readTar(t *testing.T, path string) map[string]*tar.Header {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close() //nolint:errcheck // read-only test file
	members := map[string]*tar.Header{}
	reader := tar.NewReader(file)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return members
		}
		if err != nil {
			t.Fatal(err)
		}
		members[header.Name] = header
	}
}
