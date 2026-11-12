// Design: docs/contributing/preview-artifacts.md -- portable daemon archives.
package buildpreview

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"
)

const archiveReadme = `Ze preview: an identified evaluation build, not a signed stable release.

Verify SHA256SUMS using a trusted copy before extracting this archive.
The enclosing manifest records architecture, features and exact source identity.

On the matching Linux architecture:
  ./ze version
  sudo ./ze install
  sudo ze uninstall

Review the initial configuration and provision authentication before exposing
management access. Native installation uses systemd and preserves configuration
and persistent state on ordinary uninstall. See docs/guide/ubuntu-build-install.md and
docs/guide/docker.md in the corresponding source checkout.

This archive carries no credentials. Publication requires complete corresponding
source and the project's release signing and publication gates.
`

func fileSum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	return readerSum(file)
}

func readerSum(reader io.Reader) (string, error) {
	sum := sha256.New()
	if _, err := io.Copy(sum, reader); err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

func writeArchive(root, binary, output string, source Source) (err error) {
	license, err := os.ReadFile(root + "/LICENSE")
	if err != nil {
		return err
	}
	stamp, err := time.Parse(time.RFC3339, source.BuildDate)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}()
	zip := gzip.NewWriter(file)
	tarball := tar.NewWriter(zip)
	for _, entry := range []struct {
		name string
		body []byte
		mode int64
	}{{"LICENSE", license, 0o644}, {"README", []byte(archiveReadme), 0o644}, {"ze", nil, 0o755}} {
		var reader io.Reader
		size := int64(len(entry.body))
		var executable *os.File
		if entry.name == "ze" {
			executable, err = os.Open(binary)
			if err != nil {
				return err
			}
			info, statErr := executable.Stat()
			if statErr != nil {
				executable.Close()
				return statErr
			}
			size, reader = info.Size(), executable
		}
		if err = tarball.WriteHeader(&tar.Header{Name: entry.name, Mode: entry.mode, Size: size, ModTime: stamp, Typeflag: tar.TypeReg, Format: tar.FormatUSTAR}); err != nil {
			if executable != nil {
				executable.Close()
			}
			return err
		}
		if executable != nil {
			_, err = io.Copy(tarball, reader)
			closeErr := executable.Close()
			if err == nil {
				err = closeErr
			}
		} else {
			_, err = tarball.Write(entry.body)
		}
		if err != nil {
			return fmt.Errorf("archive %s: %w", entry.name, err)
		}
	}
	if err := tarball.Close(); err != nil {
		return err
	}
	return zip.Close()
}
