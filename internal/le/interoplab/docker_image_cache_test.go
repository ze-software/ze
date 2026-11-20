// Design: docs/architecture/testing/interop.md -- the persistent image cache.
package interoplab

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// cacheBuild is one `docker build` the fake daemon ran: its argv and the
// Dockerfile it read from the streamed context.
type cacheBuild struct {
	arguments  []string
	dockerfile string
}

// cacheDaemon is a fake docker CLI for the image cache: it builds, tags,
// saves, loads and removes, keeping each tag's image ID, and writes a saved
// archive as the text `image <id>`, which its load reads back.
type cacheDaemon struct {
	t        *testing.T
	commands [][]string
	builds   []cacheBuild
	tags     map[string]string
}

func newCacheDaemon(t *testing.T) *cacheDaemon {
	return &cacheDaemon{t: t, tags: make(map[string]string)}
}

// cacheImageID answers the nth image ID the fake daemon hands out.
func cacheImageID(n int) string {
	return "sha256:" + strings.Repeat(string("0123456789abcdef"[n%16]), 64)
}

func (f *cacheDaemon) docker(cacheDir string) *Docker {
	docker := newDocker(&recordingRunner{run: f.run})
	docker.imageCacheDir = func() (string, error) { return cacheDir, nil }
	return docker
}

func (f *cacheDaemon) run(command ProcessCommand) (ProcessResult, error) {
	args := command.Arguments
	f.commands = append(f.commands, args)
	switch args[1] {
	case "version":
		return ProcessResult{Stdout: runtime.GOARCH + "\n"}, nil
	case "context":
		return ProcessResult{Stdout: "unix:///var/run/docker.sock\n"}, nil
	case "build":
		return f.build(command)
	case "tag":
		f.tags[args[3]] = args[2]
		return ProcessResult{}, nil
	case "image":
		return f.image(args)
	}
	f.t.Fatalf("unexpected docker command %v", args)
	return ProcessResult{}, nil
}

func (f *cacheDaemon) build(command ProcessCommand) (ProcessResult, error) {
	args := command.Arguments
	archive, err := io.ReadAll(command.Stdin)
	if err != nil {
		f.t.Fatalf("read build context: %v", err)
	}
	name := args[slices.Index(args, "-f")+1]
	reader := tar.NewReader(bytes.NewReader(archive))
	dockerfile := ""
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			f.t.Fatalf("build context is not a tar: %v", err)
		}
		if header.Name == name {
			body, err := io.ReadAll(reader)
			if err != nil {
				f.t.Fatalf("read %s from the build context: %v", name, err)
			}
			dockerfile = string(body)
		}
	}
	f.builds = append(f.builds, cacheBuild{arguments: args, dockerfile: dockerfile})
	id := cacheImageID(len(f.builds))
	for index, arg := range args {
		if arg == "-t" {
			f.tags[args[index+1]] = id
		}
	}
	return ProcessResult{Stdout: id + "\n"}, nil
}

func (f *cacheDaemon) image(args []string) (ProcessResult, error) {
	switch args[2] {
	case "inspect":
		return ProcessResult{Stdout: f.tags[args[len(args)-1]] + "\n"}, nil
	case "save":
		if err := os.WriteFile(args[4], []byte("image "+args[5]), 0o600); err != nil {
			return cliFailed(err)
		}
		return ProcessResult{}, nil
	case "load":
		data, err := os.ReadFile(args[5])
		if err != nil {
			return cliFailed(err)
		}
		id, valid := strings.CutPrefix(string(data), "image ")
		if !valid {
			return ProcessResult{ExitCode: 1, Stderr: "archive/tar: invalid tar header"}, nil
		}
		return ProcessResult{Stdout: loadedImagePrefix + id + "\n"}, nil
	case "rm":
		delete(f.tags, args[len(args)-1])
		return ProcessResult{}, nil
	}
	f.t.Fatalf("unexpected docker image command %v", args)
	return ProcessResult{}, nil
}

// countVerb answers how many commands the fake daemon ran whose argv opens
// with words after `docker`.
func (f *cacheDaemon) countVerb(words ...string) int {
	count := 0
	for _, args := range f.commands {
		if len(args) > len(words) && slices.Equal(args[1:1+len(words)], words) {
			count++
		}
	}
	return count
}

// cacheContext writes a build context: a Dockerfile, a payload it copies, and
// an ignored file the ignore file excludes.
func cacheContext(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeContextFile(t, filepath.Join(dir, "Dockerfile"), "FROM scratch\nCOPY payload /\n", 0o644)
	writeContextFile(t, filepath.Join(dir, "payload"), "one", 0o644)
	writeContextFile(t, filepath.Join(dir, "ignored"), "first", 0o644)
	writeContextFile(t, filepath.Join(dir, ".dockerignore"), "ignored\n", 0o644)
	return dir
}

func cachedBuild(context string, progress io.Writer) ImageBuild {
	return ImageBuild{Name: "vpp", Tag: "ze-vpp:shared", Dockerfile: filepath.Join(context, "Dockerfile"), Context: context,
		BuildArgs: []string{"VARIANT=baseline"}, Cache: true, HostCPUSpecific: true, Progress: progress}
}

// VALIDATES: the key changes with the Dockerfile, a context file, a build
// argument and the CPU identity, and stays put for a rerun, an ignored file
// and a new modification time. Method: derive keys before and after each
// change on one fake daemon.
// PREVENTS: a cached VPP image reused after its recipe or CPU changed, or a
// rebuild for a change Docker never sees.
func TestImageCacheKeyFollowsTheInputs(t *testing.T) {
	context := cacheContext(t)
	daemon := newCacheDaemon(t)
	docker := daemon.docker(t.TempDir())
	build := cachedBuild(context, nil)
	key := func() string {
		t.Helper()
		value, err := docker.imageCacheKey(t.Context(), build)
		if err != nil {
			t.Fatalf("imageCacheKey: %v", err)
		}
		return value
	}
	base := key()
	if again := key(); again != base {
		t.Fatalf("the same inputs gave two keys: %s and %s", base, again)
	}
	writeContextFile(t, filepath.Join(context, "ignored"), "second", 0o644)
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(context, "payload"), later, later); err != nil {
		t.Fatal(err)
	}
	if unchanged := key(); unchanged != base {
		t.Fatal("an ignored file or a modification time changed the key")
	}

	changes := []struct {
		name   string
		change func()
		undo   func()
	}{
		{"Dockerfile", func() {
			writeContextFile(t, filepath.Join(context, "Dockerfile"), "FROM scratch\nCOPY payload /p\n", 0o644)
		}, func() {
			writeContextFile(t, filepath.Join(context, "Dockerfile"), "FROM scratch\nCOPY payload /\n", 0o644)
		}},
		{"context file", func() {
			writeContextFile(t, filepath.Join(context, "payload"), "two", 0o644)
		}, func() {
			writeContextFile(t, filepath.Join(context, "payload"), "one", 0o644)
		}},
		{"build argument", func() { build.BuildArgs = []string{"VARIANT=metadata"} },
			func() { build.BuildArgs = []string{"VARIANT=baseline"} }},
		{"CPU identity", func() { docker.hostCPU = func() (string, error) { return "other-cpu", nil } },
			func() { docker.hostCPU = fixtureHostCPU }},
	}
	for _, change := range changes {
		change.change()
		if changed := key(); changed == base {
			t.Errorf("a changed %s left the key %s", change.name, base)
		}
		change.undo()
		if restored := key(); restored != base {
			t.Errorf("restoring the %s did not restore the key", change.name)
		}
	}

	build.HostCPUSpecific = false
	portable := key()
	docker.hostCPU = func() (string, error) { return "other-cpu", nil }
	if other := key(); other != portable {
		t.Error("the CPU identity changed the key of an image that is not CPU-specific")
	}
}

// VALIDATES: a miss builds once, saves the built image ID to a partial file in
// the cache directory and renames it to the key's entry, leaving no partial.
// PREVENTS: a killed save leaving a truncated entry that later loads as an
// image, or a build that is never saved.
func TestImageCacheMissBuildsThenSavesAtomically(t *testing.T) {
	cacheDir := t.TempDir()
	daemon := newCacheDaemon(t)
	docker := daemon.docker(cacheDir)
	var progress bytes.Buffer
	build := cachedBuild(cacheContext(t), &progress)
	key, err := docker.imageCacheKey(t.Context(), build)
	if err != nil {
		t.Fatal(err)
	}
	// A killed run's partial save, which the next run of the key removes.
	stale := filepath.Join(cacheDir, "."+key+"-KILLED"+imageCachePartialSuffix)
	if err := os.WriteFile(stale, []byte("half an archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	image, err := docker.Build(t.Context(), build)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !image.Built || image.Cached {
		t.Fatalf("a miss answered built=%v cached=%v, want built and not cached", image.Built, image.Cached)
	}
	if len(daemon.builds) != 1 || !strings.Contains(daemon.builds[0].dockerfile, "COPY payload") {
		t.Fatalf("a miss ran %d builds, want the recipe's one", len(daemon.builds))
	}
	if _, err := os.Stat(stale); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a killed run's partial save stayed: %v", err)
	}
	entry := filepath.Join(cacheDir, key+imageCacheEntrySuffix)
	var saved []string
	for _, args := range daemon.commands {
		if len(args) > 2 && args[1] == "image" && args[2] == "save" {
			saved = args
		}
	}
	if saved == nil {
		t.Fatal("a miss saved nothing to the cache")
	}
	if saved[5] != image.Reference {
		t.Fatalf("saved %s, want the built image ID %s", saved[5], image.Reference)
	}
	if filepath.Dir(saved[4]) != cacheDir || !strings.HasSuffix(saved[4], imageCachePartialSuffix) {
		t.Fatalf("saved to %s, want a partial file in %s that a rename moves into place", saved[4], cacheDir)
	}
	data, err := os.ReadFile(entry)
	if err != nil {
		t.Fatalf("the entry is not in place: %v", err)
	}
	if string(data) != "image "+image.Reference {
		t.Fatalf("the entry holds %q", data)
	}
	if _, err := os.Stat(saved[4]); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the partial file stayed behind: %v", err)
	}
	if !strings.Contains(progress.String(), "saved to the image cache, key "+key) {
		t.Fatalf("no progress line names the saved key:\n%s", progress.String())
	}
}

// VALIDATES: a hit loads the entry, builds only the relabel Dockerfile FROM the
// loaded image, carries the owner labels, the shared tag and a run-owned tag,
// and answers cached. Method: a miss fills the cache, a second Build hits it.
// PREVENTS: a hit that compiles the recipe again, or one whose tags or labels
// differ from a build's, which the orphan sweep would then remove.
func TestImageCacheHitLoadsAndTagsWithoutBuilding(t *testing.T) {
	cacheDir := t.TempDir()
	daemon := newCacheDaemon(t)
	docker := daemon.docker(cacheDir)
	var progress bytes.Buffer
	build := cachedBuild(cacheContext(t), &progress)
	first, err := docker.Build(t.Context(), build)
	if err != nil {
		t.Fatalf("first Build: %v", err)
	}
	image, err := docker.Build(t.Context(), build)
	if err != nil {
		t.Fatalf("second Build: %v", err)
	}
	if !image.Cached || !image.Built {
		t.Fatalf("a hit answered built=%v cached=%v", image.Built, image.Cached)
	}
	if daemon.countVerb("image", "load") != 1 {
		t.Fatalf("a hit ran %d loads, want 1", daemon.countVerb("image", "load"))
	}
	if len(daemon.builds) != 2 {
		t.Fatalf("ran %d builds, want the miss's and the hit's relabel", len(daemon.builds))
	}
	relabel := daemon.builds[1]
	if !strings.HasPrefix(relabel.dockerfile, "FROM "+imageCacheLoadRepository+":") || strings.Contains(relabel.dockerfile, "COPY") {
		t.Fatalf("the hit built %q, want only a FROM of the loaded image", relabel.dockerfile)
	}
	if !slices.Contains(relabel.arguments, build.Tag) {
		t.Fatalf("the hit did not apply the shared tag: %v", relabel.arguments)
	}
	if !strings.HasPrefix(image.OwnedTag(), retainedTagRepository+":") || image.OwnedTag() == first.OwnedTag() ||
		!slices.Contains(relabel.arguments, image.OwnedTag()) {
		t.Fatalf("the hit's run-owned tag %q is not its own", image.OwnedTag())
	}
	if !slices.Contains(relabel.arguments, "--label") {
		t.Fatalf("the hit carries no owner labels: %v", relabel.arguments)
	}
	if daemon.tags[image.OwnedTag()] != image.Reference || daemon.tags[build.Tag] != image.Reference {
		t.Fatalf("the tags name %s and %s, want the answered %s", daemon.tags[build.Tag], daemon.tags[image.OwnedTag()], image.Reference)
	}
	for tag := range daemon.tags {
		if strings.HasPrefix(tag, imageCacheLoadRepository+":") {
			t.Fatalf("the temporary load tag %s stayed", tag)
		}
	}
	if daemon.countVerb("image", "save") != 1 {
		t.Fatal("a hit saved the image again")
	}
	if !strings.Contains(progress.String(), "loaded from the image cache, key ") {
		t.Fatalf("no progress line says the image came from the cache:\n%s", progress.String())
	}
}

// VALIDATES: an entry Docker refuses to load is removed, the recipe is built,
// and the good build replaces the entry.
// PREVENTS: a corrupt entry failing every run, or being loaded forever.
func TestImageCacheCorruptEntryIsRemovedAndRebuilt(t *testing.T) {
	cacheDir := t.TempDir()
	daemon := newCacheDaemon(t)
	docker := daemon.docker(cacheDir)
	var progress bytes.Buffer
	build := cachedBuild(cacheContext(t), &progress)
	key, err := docker.imageCacheKey(t.Context(), build)
	if err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(cacheDir, key+imageCacheEntrySuffix)
	if err := os.WriteFile(entry, []byte("truncated"), 0o600); err != nil {
		t.Fatal(err)
	}
	image, err := docker.Build(t.Context(), build)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if image.Cached {
		t.Fatal("a corrupt entry answered cached")
	}
	if len(daemon.builds) != 1 || !strings.Contains(daemon.builds[0].dockerfile, "COPY payload") {
		t.Fatal("a corrupt entry was not rebuilt from the recipe")
	}
	if !strings.Contains(progress.String(), "so it was removed") {
		t.Fatalf("no progress line says the entry was removed:\n%s", progress.String())
	}
	data, err := os.ReadFile(entry)
	if err != nil || string(data) != "image "+image.Reference {
		t.Fatalf("the entry was not replaced by the rebuild: %q, %v", data, err)
	}
}

// VALIDATES: a CPU identity that cannot be read builds without the cache and
// says so, saving nothing.
// PREVENTS: a CPU-specific image keyed without its CPU, then loaded on another.
func TestImageCacheUnreadableCPUBuildsUncached(t *testing.T) {
	cacheDir := t.TempDir()
	daemon := newCacheDaemon(t)
	docker := daemon.docker(cacheDir)
	docker.hostCPU = func() (string, error) { return "", errors.New("no cpuinfo") }
	var progress bytes.Buffer
	if _, err := docker.Build(t.Context(), cachedBuild(cacheContext(t), &progress)); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if daemon.countVerb("image", "save") != 0 || daemon.countVerb("image", "load") != 0 {
		t.Fatal("the cache was used without a CPU identity")
	}
	if !strings.Contains(progress.String(), "CPU identity cannot be read") {
		t.Fatalf("no progress line says why the cache was skipped:\n%s", progress.String())
	}
}

// VALIDATES: a build that does not opt in never reaches the cache.
// PREVENTS: every lab image paying a context hash and a multi-gigabyte save.
func TestImageCacheUnusedWithoutOptIn(t *testing.T) {
	daemon := newCacheDaemon(t)
	docker := daemon.docker(t.TempDir())
	docker.imageCacheDir = func() (string, error) {
		t.Fatal("a build that did not opt in asked for the image cache")
		return "", nil
	}
	docker.hostCPU = func() (string, error) {
		t.Fatal("a build that did not opt in read the CPU identity")
		return "", nil
	}
	build := cachedBuild(cacheContext(t), nil)
	build.Cache = false
	if _, err := docker.Build(t.Context(), build); err != nil {
		t.Fatalf("Build: %v", err)
	}
	for _, words := range [][]string{{"version"}, {"context"}, {"image", "save"}, {"image", "load"}} {
		if daemon.countVerb(words...) != 0 {
			t.Fatalf("a build that did not opt in ran docker %v", words)
		}
	}
}

// VALIDATES: the CPU identity keeps the fields that decide native code from
// the first processor, and refuses a text with no model or no flags.
// PREVENTS: an identity that matches every CPU, or one that changes with the
// clock speed.
func TestCPUInfoIdentity(t *testing.T) {
	x86 := "processor\t: 0\nvendor_id\t: GenuineIntel\ncpu family\t: 6\nmodel\t\t: 85\n" +
		"model name\t: Intel(R) Xeon(R)\ncpu MHz\t\t: 2000.000\nflags\t\t: fpu sse avx512f\n\n" +
		"processor\t: 1\nmodel name\t: Other\nflags\t\t: fpu\n"
	identity, err := cpuinfoIdentity(x86)
	if err != nil {
		t.Fatal(err)
	}
	want := "vendor_id=GenuineIntel\ncpu family=6\nmodel=85\nmodel name=Intel(R) Xeon(R)\nflags=fpu sse avx512f\n"
	if identity != want {
		t.Fatalf("identity = %q, want %q", identity, want)
	}
	arm := "processor\t: 0\nBogoMIPS\t: 48.00\nFeatures\t: fp asimd sve\nCPU implementer\t: 0x41\nCPU part\t: 0xd40\n"
	if _, err := cpuinfoIdentity(arm); err != nil {
		t.Fatalf("an arm64 text was refused: %v", err)
	}
	for _, text := range []string{"", "processor\t: 0\nflags\t: fpu\n", "processor\t: 0\nmodel name\t: X\n"} {
		if _, err := cpuinfoIdentity(text); err == nil {
			t.Errorf("accepted %q", text)
		}
	}
}

// VALIDATES: only a local socket or a loopback address counts as this
// machine's Docker host.
// PREVENTS: a remote daemon keyed by this machine's CPU.
func TestLocalDockerEndpoint(t *testing.T) {
	for endpoint, want := range map[string]bool{
		"unix:///var/run/docker.sock": true,
		"tcp://127.0.0.1:2375":        true,
		"tcp://[::1]:2375":            true,
		"tcp://localhost:2375":        true,
		"tcp://192.0.2.7:2375":        false,
		"ssh://builder@192.0.2.7":     false,
	} {
		if got := localDockerEndpoint(endpoint); got != want {
			t.Errorf("localDockerEndpoint(%q) = %v, want %v", endpoint, got, want)
		}
	}
}

// cliFailed answers what the docker CLI reports for a failed file operation:
// a nonzero exit with the cause on stderr, which Docker treats as a command
// failure rather than an error starting the command.
func cliFailed(cause error) (ProcessResult, error) {
	return ProcessResult{ExitCode: 1, Stderr: cause.Error()}, nil
}
