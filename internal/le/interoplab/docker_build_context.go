// Design: docs/architecture/testing/interop.md -- the build context Ze writes itself.
// Related: docker.go -- Build streams this archive on the stdin of `docker build -`.
package interoplab

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/moby/patternmatcher"
	"github.com/moby/patternmatcher/ignorefile"
)

// Ze writes every lab image's build context as a tar stream itself, and
// `docker build` reads it from stdin, rather than handing docker a directory
// to walk. A macOS host shares the checkout with Ze's QEMU guest over 9p, and
// QEMU's darwin 9p backend answers llistxattr and lgetxattr on a symlink with
// ELOOP (it opens the link O_NOFOLLOW, and macOS has no O_PATH). BuildKit's
// directory walker tolerates only ENOTSUP from those calls, so a guest build
// whose context holds a symlink (the checkout's `cache`, the migrated `tmp/`
// children) failed with "too many levels of symbolic links", whatever the
// ignore file said. This walk reads each entry with lstat and readlink, writes
// a symlink as a link, and reads no extended attribute.

// dockerIgnoreName is the ignore file Docker reads at the context's root, and
// the suffix of the one it reads beside the Dockerfile first.
const dockerIgnoreName = ".dockerignore"

// buildContextDefaultDockerfile is the Dockerfile Docker reads when the build
// names none.
const buildContextDefaultDockerfile = "Dockerfile"

// errBuildContextUnread is the cause the archive writer meets when the build
// process returned before reading the whole context. The process's own exit
// status then judges the build, so this cause alone fails nothing.
var errBuildContextUnread = errors.New("the build process returned before reading the whole context")

// errBuildContextUnsent marks a build whose context walk failed part way.
// Docker may still have built from the part it read and moved the build's
// owned tag onto that image, so Build releases the tag when it meets this.
var errBuildContextUnsent = errors.New("the build context was not sent whole")

// buildContext is a build context checked and ready to stream: the directory,
// the Dockerfile's path inside it, and the ignore patterns that apply. Only
// prepareBuildContext builds one; its zero value names no directory and
// writeArchive refuses it.
type buildContext struct {
	// root is the context directory, absolute, with its symlinks resolved.
	root string
	// dockerfile is the Dockerfile's path relative to root, in slash form: the
	// `-f` argument of a build whose context arrives on stdin.
	dockerfile string
	// matcher holds the ignore file's patterns. Nil when no ignore file exists
	// or it holds no pattern, so every entry is sent.
	matcher *patternmatcher.PatternMatcher
}

// prepareBuildContext checks the context directory and reads its ignore file
// the way Docker does. A relative dockerfile is resolved against the working
// directory, as the docker CLI resolves `-f`, and an empty one names the
// context's own Dockerfile. A Dockerfile outside the context is refused: the
// archive carries the context alone.
func prepareBuildContext(context, dockerfile string) (buildContext, error) {
	if context == "" {
		return buildContext{}, errors.New("Docker build context is empty")
	}
	named, err := filepath.Abs(context)
	if err != nil {
		return buildContext{}, fmt.Errorf("resolve Docker build context %s: %w", context, err)
	}
	// WalkDir does not descend a root that is itself a symlink, so a context
	// reached through one (a migrated `tmp/` child) would be sent empty. The
	// docker CLI resolves the context the same way before it walks it
	// (ResolveAndValidateContextPath). The Dockerfile is still placed against
	// the path the caller named: resolving the root leaves every path below it
	// the same.
	root, err := filepath.EvalSymlinks(named)
	if err != nil {
		return buildContext{}, fmt.Errorf("read Docker build context: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return buildContext{}, fmt.Errorf("read Docker build context: %w", err)
	}
	if !info.IsDir() {
		return buildContext{}, errors.New("Docker build context " + root + " is not a directory")
	}
	relative, err := contextDockerfile(named, dockerfile)
	if err != nil {
		return buildContext{}, err
	}
	matcher, err := contextIgnoreMatcher(root, relative)
	if err != nil {
		return buildContext{}, err
	}
	return buildContext{root: root, dockerfile: relative, matcher: matcher}, nil
}

// contextDockerfile answers the Dockerfile's path inside root, in slash form.
func contextDockerfile(root, dockerfile string) (string, error) {
	if dockerfile == "" {
		return buildContextDefaultDockerfile, nil
	}
	absolute, err := filepath.Abs(dockerfile)
	if err != nil {
		return "", fmt.Errorf("resolve Dockerfile %s: %w", dockerfile, err)
	}
	relative, err := filepath.Rel(root, absolute)
	if err != nil {
		return "", fmt.Errorf("place Dockerfile %s in build context %s: %w", absolute, root, err)
	}
	outside := relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator))
	if outside {
		return "", errors.New("Dockerfile " + absolute + " is outside the build context " + root +
			": the context Ze sends holds only that directory, so move the Dockerfile under it or widen the image's Context")
	}
	return filepath.ToSlash(relative), nil
}

// contextIgnoreMatcher reads the ignore file Docker applies to this build:
// `<Dockerfile>.dockerignore` beside the Dockerfile wins over the context's
// `.dockerignore`. The Dockerfile and the ignore file are always sent, even
// when a pattern matches them, because the builder reads both from the
// context; the exclusions appended here are how the docker CLI keeps them
// (TrimBuildFilesFromExcludes).
func contextIgnoreMatcher(root, dockerfile string) (*patternmatcher.PatternMatcher, error) {
	ignore := dockerfile + dockerIgnoreName
	patterns, found, err := readIgnoreFile(filepath.Join(root, filepath.FromSlash(ignore)))
	if err != nil {
		return nil, err
	}
	if !found {
		ignore = dockerIgnoreName
		// An absent root ignore file leaves no pattern, which is the answer.
		patterns, _, err = readIgnoreFile(filepath.Join(root, ignore))
		if err != nil {
			return nil, err
		}
	}
	// A context with no ignore file sends every entry: a matcher over no
	// pattern matches nothing.
	for _, kept := range []string{dockerfile, ignore} {
		if len(patterns) == 0 {
			break
		}
		excluded, err := patternmatcher.MatchesOrParentMatches(kept, patterns)
		if err != nil {
			return nil, fmt.Errorf("match %s against the build's ignore file: %w", kept, err)
		}
		if excluded {
			patterns = append(patterns, "!"+kept)
		}
	}
	matcher, err := patternmatcher.New(patterns)
	if err != nil {
		return nil, fmt.Errorf("compile the build's ignore file %s: %w", ignore, err)
	}
	return matcher, nil
}

// readIgnoreFile answers the patterns of one ignore file, and whether the file
// exists. An absent file is not an error: Docker then reads the next one.
func readIgnoreFile(path string) ([]string, bool, error) {
	file, err := os.Open(path) //nolint:gosec // the ignore file of the build context the caller named
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("open Docker ignore file: %w", err)
	}
	defer file.Close() //nolint:errcheck // A read-only file; ReadAll's error is the one that matters.
	patterns, err := ignorefile.ReadAll(file)
	if err != nil {
		return nil, false, fmt.Errorf("read Docker ignore file %s: %w", path, err)
	}
	return patterns, true, nil
}

// streamBuildContext runs run with the context's archive on the reader it
// passes, and answers run's result. The archive is written by one goroutine
// whose lifetime is this call: it ends when the archive is complete, when the
// walk fails, or when run returns and the reader is closed, and this call
// waits for it before it answers. A walk failure fails the build with its
// cause, so a truncated context is never sent in silence. A process that
// returns before reading the whole archive ends the walk, and its own result
// judges the build.
func streamBuildContext(source buildContext, run func(stdin io.Reader) (ProcessResult, error)) (ProcessResult, error) {
	reader, writer := io.Pipe()
	written := make(chan error, 1)
	go writeBuildContext(source, writer, written)
	result, err := run(reader)
	reader.CloseWithError(errBuildContextUnread) //nolint:errcheck // CloseWithError always answers nil.
	walkErr := <-written
	if walkErr != nil && !errors.Is(walkErr, errBuildContextUnread) {
		err = errors.Join(err, fmt.Errorf("send Docker build context %s: %w: %w", source.root, errBuildContextUnsent, walkErr))
	}
	return result, err
}

// writeBuildContext is the archive goroutine of streamBuildContext. It closes
// writer with the walk's outcome, so the reader sees the end of the archive or
// the walk's error, and then reports that outcome on done.
func writeBuildContext(source buildContext, writer *io.PipeWriter, done chan<- error) {
	err := source.writeArchive(writer)
	writer.CloseWithError(err) //nolint:errcheck // CloseWithError always answers nil.
	done <- err
}

// writeArchive writes the context as one tar stream. Every entry is read with
// lstat: a symlink becomes a symlink entry carrying its link text and is never
// followed, and no extended attribute is read. Owners are root, as the docker
// CLI sends them (its ChownOpts of 0:0). A socket is skipped, as the docker
// CLI skips one; a device or a named pipe is refused, naming its path.
func (c buildContext) writeArchive(output io.Writer) error {
	if c.root == "" {
		return errors.New("BUG: build context archive written before prepareBuildContext")
	}
	archive := tar.NewWriter(output)
	err := filepath.WalkDir(c.root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		return c.writeEntry(archive, path, entry)
	})
	if err != nil {
		return err
	}
	return archive.Close()
}

// writeEntry writes one walked entry, or skips it, or skips a whole excluded
// directory.
func (c buildContext) writeEntry(archive *tar.Writer, path string, entry fs.DirEntry) error {
	relative, err := filepath.Rel(c.root, path)
	if err != nil {
		return err
	}
	if relative == "." {
		return nil
	}
	name := filepath.ToSlash(relative)
	excluded, err := c.excluded(name)
	if err != nil {
		return err
	}
	if excluded {
		return c.skipExcluded(name, entry)
	}
	info, err := entry.Info()
	if err != nil {
		return err
	}
	mode := info.Mode()
	link := ""
	switch {
	case mode.IsRegular(), mode.IsDir():
	case mode&fs.ModeSymlink != 0:
		link, err = os.Readlink(path)
		if err != nil {
			return err
		}
	case mode&fs.ModeSocket != 0:
		return nil
	default:
		return errors.New("build context entry " + path + " has file mode " + mode.String() +
			" (not a file, a directory or a symlink), which a Docker build context cannot carry: add it to the context's .dockerignore")
	}
	header, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return fmt.Errorf("describe %s: %w", path, err)
	}
	header.Name = name
	if mode.IsDir() {
		header.Name = name + "/"
	}
	header.Uid, header.Gid, header.Uname, header.Gname = 0, 0, "", ""
	if err := archive.WriteHeader(header); err != nil {
		return err
	}
	if !mode.IsRegular() {
		return nil
	}
	return copyContextFile(archive, path, header.Size)
}

// excluded answers whether the ignore file excludes name, a slash path inside
// the context.
func (c buildContext) excluded(name string) (bool, error) {
	excluded, err := c.matcher.MatchesOrParentMatches(name)
	if err != nil {
		return false, fmt.Errorf("match %s against the build's ignore file: %w", name, err)
	}
	return excluded, nil
}

// skipExcluded skips an excluded entry. An excluded directory is not descended
// unless an exclusion pattern (`!dir/file`) could match inside it; it is then
// walked without an entry of its own, as the docker CLI's archive walk does.
func (c buildContext) skipExcluded(name string, entry fs.DirEntry) error {
	if !entry.IsDir() {
		return nil
	}
	if !c.matcher.Exclusions() {
		return filepath.SkipDir
	}
	directory := name + "/"
	for _, pattern := range c.matcher.Patterns() {
		if !pattern.Exclusion() {
			continue
		}
		if strings.HasPrefix(filepath.ToSlash(pattern.String())+"/", directory) {
			return nil
		}
	}
	return filepath.SkipDir
}

// copyContextFile writes exactly size bytes of path into the archive. A file
// that changed size since its header was written fails the walk, rather than
// sending an entry whose length the header does not state.
func copyContextFile(archive *tar.Writer, path string, size int64) error {
	file, err := os.Open(path) //nolint:gosec // a file of the build context the caller named, sent to Docker as the context
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck // A read-only file; the copy's error is the one that matters.
	if _, err := io.CopyN(archive, file, size); err != nil {
		return fmt.Errorf("copy %s into the build context: %w", path, err)
	}
	return nil
}
