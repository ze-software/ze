// Design: docs/guide/command-reference.md -- ze support
// Related: internal/component/support/support.go -- collect and writeArchive,
// the producer whose archive members these drivers read back
//
// The support-archive fixtures: `ze support --json` prints the manifest and
// nothing else, so a test that asserts on a module's collected data reads that
// data out of the archive the same run wrote.

package fixture

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const supportCrashesArchiveName = "plugin/support-crashes-archive"

func init() {
	Register(supportCrashesArchiveName, supportCrashesArchive)
}

// supportCrashesArchive writes the crashes.json member of the support archive
// in the working directory to stdout.
//
// The manifest that `--json` prints carries one {collected, duration-ms} row
// per module and no collected value at all (SupportManifest,
// internal/component/support/support.go), so the crash reports and the
// readiness block are in the archive and only there. Putting the member's own
// bytes on stdout lets a .ci assert against what `ze support` produced rather
// than against a sentence this driver writes.
func supportCrashesArchive(_ context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("unexpected arguments: %v", args)
	}

	archivePath, err := supportArchivePath(".")
	if err != nil {
		return err
	}
	member, err := supportArchiveMember(archivePath, "crashes.json")
	if err != nil {
		return err
	}
	if _, err := os.Stdout.Write(member); err != nil {
		return fmt.Errorf("write crashes.json to stdout: %w", err)
	}
	return nil
}

// supportArchivePath names the one support archive in dir.
//
// `ze support` builds the file name from the hostname and the collection
// timestamp, so a caller cannot predict it. A test directory holds one run, so
// any other count is a broken scenario rather than a choice to make: two
// archives leave no way to tell which run is under test, and none means the
// command wrote nothing.
func supportArchivePath(dir string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "ze-support-*.tar.gz"))
	if err != nil {
		return "", fmt.Errorf("look for the support archive in %s: %w", dir, err)
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("want exactly one support archive in %s, found %d: %v", dir, len(matches), matches)
	}
	return matches[0], nil
}

// supportArchiveMember reads one named member out of a support archive.
//
// A name that appears twice resolves to the LAST entry, which is what an
// extraction does: `tar -x` overwrites the earlier file with the later one, so
// a reader that stopped at the first match would report content no operator
// would ever see.
func supportArchiveMember(archivePath, member string) ([]byte, error) {
	archiveFile, err := os.Open(archivePath) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return nil, fmt.Errorf("open support archive: %w", err)
	}
	defer archiveFile.Close() //nolint:errcheck // fixture teardown

	gzipReader, err := gzip.NewReader(archiveFile)
	if err != nil {
		return nil, fmt.Errorf("open support archive gzip stream: %w", err)
	}
	defer gzipReader.Close() //nolint:errcheck // fixture teardown

	tarReader := tar.NewReader(gzipReader)
	var content []byte
	found := false

	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read support archive: %w", err)
		}
		if header.Name != member {
			continue
		}

		found = header.Typeflag == tar.TypeReg
		content = nil
		if !found {
			continue
		}

		content, err = io.ReadAll(tarReader)
		if err != nil {
			return nil, fmt.Errorf("read %s from the support archive: %w", member, err)
		}
	}

	if !found {
		return nil, fmt.Errorf("support archive %s carries no %s", archivePath, member)
	}
	return content, nil
}
