// Design: website/AI.md -- the artifact checkout keeps its own ignore list
// Related: build.go -- Build writes the list after removeSourceOnly
// Related: actions.go -- runCheck reports published source through publishedSourceOnly
package site

import (
	"bytes"
	"os"
	"path/filepath"
)

// The artifact checkout's own ignore list. It keeps what sessions leave under
// tmp/ in that checkout (commit scripts, their messages, logs) out of the
// published branch.
//
// website/.gitignore is source and never ships, so isSourceOnly names
// .gitignore and removeSourceOnly deletes any staged copy. The build then
// writes this list in its place, which makes the root .gitignore of a built
// artifact always this content and nothing the source tree carried.
const (
	artifactIgnoreFile    = ".gitignore"
	artifactIgnoreContent = "tmp/\n.DS_Store\n"
)

// writeArtifactIgnore lays the artifact's ignore list at its root. It MUST run
// after removeSourceOnly, which deletes every .gitignore as source.
func writeArtifactIgnore(output string) error {
	return writeNamedArtifact(output, artifactIgnoreFile, artifactIgnoreContent)
}

// publishedSourceOnly answers every path in the artifact that is source and
// must not be deployed, in walk order.
//
// The root .gitignore is the one source-only name the artifact may carry, and
// only while it holds exactly artifactIgnoreContent, which is what the build
// writes. Any other content there is the source file leaking through, so it is
// reported; a file that cannot be read is reported too.
func publishedSourceOnly(output string) ([]string, error) {
	var found []string
	err := filepath.WalkDir(output, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == output {
			return nil
		}
		relative, err := filepath.Rel(output, path)
		if err != nil {
			return err
		}
		if !isSourceOnly(relative) {
			return nil
		}
		if relative == artifactIgnoreFile && holdsArtifactIgnore(path) {
			return nil
		}
		found = append(found, filepath.ToSlash(relative))
		if entry.IsDir() {
			return filepath.SkipDir
		}
		return nil
	})
	return found, err
}

// holdsArtifactIgnore reports whether the file at path is the artifact's own
// ignore list, byte for byte. A read failure answers false, so the caller
// reports the file rather than passing it unread.
func holdsArtifactIgnore(path string) bool {
	content, err := os.ReadFile(path) //nolint:gosec // path is the artifact root's .gitignore, joined from the configured output
	if err != nil {
		return false
	}
	return bytes.Equal(content, []byte(artifactIgnoreContent))
}
