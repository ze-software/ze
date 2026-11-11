// Design: docs/architecture/testing/interop.md -- the build context Ze writes itself.
package interoplab

import (
	"archive/tar"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// contextEntry is one entry of the archive a build received on its stdin.
type contextEntry struct {
	header tar.Header
	body   string
}

// buildContextRunner answers `docker build` with an image id after reading
// the whole of its stdin, and records the archive it read.
type buildContextRunner struct {
	arguments []string
	archive   []byte
	readErr   error
}

func (r *buildContextRunner) runner() *recordingRunner {
	return &recordingRunner{run: func(command ProcessCommand) (ProcessResult, error) {
		if command.Arguments[1] != "build" {
			return ProcessResult{}, nil
		}
		r.arguments = command.Arguments
		if command.Stdin == nil {
			r.readErr = errors.New("docker build received no stdin")
			return ProcessResult{Stdout: "sha256:context\n"}, nil
		}
		r.archive, r.readErr = io.ReadAll(command.Stdin)
		return ProcessResult{Stdout: "sha256:context\n"}, nil
	}}
}

// entries parses the archive the build read, by entry name.
func (r *buildContextRunner) entries(t *testing.T) map[string]contextEntry {
	t.Helper()
	if r.readErr != nil {
		t.Fatalf("reading the build's stdin: %v", r.readErr)
	}
	entries := make(map[string]contextEntry)
	reader := tar.NewReader(strings.NewReader(string(r.archive)))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return entries
		}
		if err != nil {
			t.Fatalf("the build's stdin is not a tar archive: %v", err)
		}
		body, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("read %s from the archive: %v", header.Name, err)
		}
		entries[header.Name] = contextEntry{header: *header, body: string(body)}
	}
}

func writeContextFile(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Build sends the main context as a tar Ze wrote on the stdin of
// `docker build -f <Dockerfile inside the context> -`: a symlink is a link
// entry carrying its link text and is never followed, the context's
// .dockerignore excludes what it names (an excluded directory and a symlink
// to a directory outside the context included) while a `!` pattern keeps its
// file, and a regular file keeps its mode bits and its bytes.
// METHOD: a temporary context shaped like the checkout (a `cache` symlink out
// of the tree, an ignored `tmp/`), built through a runner that reads stdin as
// docker does, then the archive it read is parsed.
// PREVENTS: docker walking the directory itself, which on a macOS host's 9p
// share reads the xattrs of the `cache` symlink and fails with ELOOP.
func TestBuildSendsContextAsTarWithSymlinksAsLinks(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeContextFile(t, filepath.Join(outside, "secret"), "outside the context\n", 0o600)
	dockerfile := filepath.Join(root, "docker", "Dockerfile.lab")
	writeContextFile(t, dockerfile, "FROM scratch\n", 0o644)
	writeContextFile(t, filepath.Join(root, ".dockerignore"), "cache/\ntmp/\n*.md\n!keep.md\n", 0o644)
	writeContextFile(t, filepath.Join(root, "file.txt"), "regular bytes\n", 0o751)
	writeContextFile(t, filepath.Join(root, "keep.md"), "kept\n", 0o644)
	writeContextFile(t, filepath.Join(root, "notes.md"), "ignored\n", 0o644)
	writeContextFile(t, filepath.Join(root, "tmp", "session", "state"), "ignored\n", 0o644)
	if err := os.Symlink(outside, filepath.Join(root, "cache")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("file.txt", filepath.Join(root, "link-in")); err != nil {
		t.Fatal(err)
	}

	recorded := &buildContextRunner{}
	image, err := newDocker(recorded.runner()).Build(t.Context(), ImageBuild{Name: "lab", Tag: "lab:latest",
		Dockerfile: dockerfile, Context: root})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !image.Built || image.Reference != "sha256:context" {
		t.Fatalf("Build answered %+v, want the built image sha256:context", image)
	}
	argv := recorded.arguments
	if len(argv) < 3 || argv[len(argv)-1] != "-" {
		t.Fatalf("docker build argv %v does not end in -, the context on stdin", argv)
	}
	if argv[len(argv)-3] != "-f" || argv[len(argv)-2] != "docker/Dockerfile.lab" {
		t.Errorf("docker build argv %v lacks -f docker/Dockerfile.lab before -", argv)
	}
	for _, argument := range argv {
		if argument == root {
			t.Errorf("docker build argv %v still names the context directory", argv)
		}
	}

	entries := recorded.entries(t)
	link, ok := entries["link-in"]
	if !ok {
		t.Fatalf("the archive lacks link-in: %v", entryNames(entries))
	}
	if link.header.Typeflag != tar.TypeSymlink || link.header.Linkname != "file.txt" {
		t.Errorf("link-in is type %q linking %q, want a symlink to file.txt", link.header.Typeflag, link.header.Linkname)
	}
	for name := range entries {
		if name == "cache" || strings.HasPrefix(name, "cache/") {
			t.Errorf("the archive carries %s, which .dockerignore excludes", name)
		}
		if name == "tmp/" || strings.HasPrefix(name, "tmp/") {
			t.Errorf("the archive carries %s, which .dockerignore excludes", name)
		}
		if strings.Contains(name, "secret") {
			t.Errorf("the archive carries %s: the cache symlink was followed", name)
		}
	}
	if _, ok := entries["notes.md"]; ok {
		t.Error("the archive carries notes.md, which *.md excludes")
	}
	for _, name := range []string{"keep.md", "docker/", "docker/Dockerfile.lab", ".dockerignore"} {
		if _, ok := entries[name]; !ok {
			t.Errorf("the archive lacks %s: %v", name, entryNames(entries))
		}
	}
	file := entries["file.txt"]
	if file.header.Typeflag != tar.TypeReg || file.body != "regular bytes\n" {
		t.Errorf("file.txt is type %q holding %q, want a regular file holding its bytes", file.header.Typeflag, file.body)
	}
	if file.header.Mode&0o7777 != 0o751 {
		t.Errorf("file.txt mode = %o, want 751", file.header.Mode&0o7777)
	}
	if file.header.Uid != 0 || file.header.Gid != 0 {
		t.Errorf("file.txt owner = %d:%d, want 0:0 as the docker CLI sends", file.header.Uid, file.header.Gid)
	}
}

// VALIDATES: the ignore file beside the Dockerfile (`<Dockerfile>.dockerignore`)
// wins over the context's .dockerignore, and the Dockerfile and that ignore
// file are sent even though its own patterns exclude their directory.
// METHOD: the root ignore file excludes file.txt, the Dockerfile's own one
// excludes other.txt and the whole docker/ directory; the archive is parsed.
// PREVENTS: the root file applied to a build whose Dockerfile carries its own,
// or a Dockerfile excluded from the context the builder reads it from.
func TestBuildPrefersTheDockerfilesOwnIgnoreFile(t *testing.T) {
	root := t.TempDir()
	dockerfile := filepath.Join(root, "docker", "Dockerfile.lab")
	writeContextFile(t, dockerfile, "FROM scratch\n", 0o644)
	writeContextFile(t, dockerfile+".dockerignore", "other.txt\ndocker\n", 0o644)
	writeContextFile(t, filepath.Join(root, ".dockerignore"), "file.txt\n", 0o644)
	writeContextFile(t, filepath.Join(root, "file.txt"), "sent\n", 0o644)
	writeContextFile(t, filepath.Join(root, "other.txt"), "ignored\n", 0o644)
	writeContextFile(t, filepath.Join(root, "docker", "unrelated"), "ignored\n", 0o644)

	recorded := &buildContextRunner{}
	if _, err := newDocker(recorded.runner()).Build(t.Context(), ImageBuild{Name: "lab", Tag: "lab:latest",
		Dockerfile: dockerfile, Context: root}); err != nil {
		t.Fatalf("Build: %v", err)
	}
	entries := recorded.entries(t)
	for _, name := range []string{"file.txt", "docker/Dockerfile.lab", "docker/Dockerfile.lab.dockerignore"} {
		if _, ok := entries[name]; !ok {
			t.Errorf("the archive lacks %s: %v", name, entryNames(entries))
		}
	}
	for _, name := range []string{"other.txt", "docker/unrelated"} {
		if _, ok := entries[name]; ok {
			t.Errorf("the archive carries %s, which the Dockerfile's own ignore file excludes", name)
		}
	}
}

// VALIDATES: an entry a build context cannot carry (a named pipe) fails the
// build with its path, even when docker answers an image id.
// METHOD: a context holding a fifo, built through a runner that reads stdin
// to its end and then reports success.
// PREVENTS: a walk that stops part way handing docker a truncated context
// that builds in silence.
func TestBuildFailsWhenTheContextCannotBeSentWhole(t *testing.T) {
	root := t.TempDir()
	writeContextFile(t, filepath.Join(root, "Dockerfile"), "FROM scratch\n", 0o644)
	pipe := filepath.Join(root, "pipe")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Fatal(err)
	}
	recorded := &buildContextRunner{}
	image, err := newDocker(recorded.runner()).Build(t.Context(), ImageBuild{Name: "lab", Tag: "lab:latest",
		Dockerfile: filepath.Join(root, "Dockerfile"), Context: root})
	if err == nil {
		t.Fatal("Build succeeded over a context it could not send whole")
	}
	if !strings.Contains(err.Error(), pipe) {
		t.Errorf("the failure does not name %s: %v", pipe, err)
	}
	if image.Built {
		t.Errorf("Build answered a built image over a truncated context: %+v", image)
	}
}

// VALIDATES: a Dockerfile outside the context is refused before docker runs.
// PREVENTS: a build whose `-f` names a file the archive does not carry.
func TestBuildRefusesADockerfileOutsideTheContext(t *testing.T) {
	root := t.TempDir()
	dockerfile := filepath.Join(t.TempDir(), "Dockerfile")
	writeContextFile(t, dockerfile, "FROM scratch\n", 0o644)
	runner := &recordingRunner{}
	_, err := newDocker(runner).Build(t.Context(), ImageBuild{Name: "lab", Tag: "lab:latest", Dockerfile: dockerfile, Context: root})
	if err == nil || !strings.Contains(err.Error(), "outside the build context") {
		t.Fatalf("Build answered %v, want the outside-the-context refusal", err)
	}
	if len(runner.commands) != 0 {
		t.Errorf("docker ran %v for a refused build", runner.commands)
	}
}

func entryNames(entries map[string]contextEntry) []string {
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	return names
}
