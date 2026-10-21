// Design: docs/architecture/appliance/docker-host.md -- the extra files the image carries
// Related: build.go -- the build that writes them before gok runs

package dockerhost

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// guestLibDir is where the image keeps every third-party binary; the
	// guest program links /bin, /usr/bin and /usr/sbin names to it at boot.
	guestLibDir = "usr/lib/ze-docker-host"
	// busyboxMember is busybox's path inside ze-serial-shell's extra files tar.
	busyboxMember = "usr/local/bin/ze-recovery-shell"
	// dockerMemberPrefix is the directory Docker's static tarball unpacks into.
	dockerMemberPrefix = "docker/"
	// extraFileBytesMax bounds one member copied into the tar, against a
	// corrupt input claiming an absurd size.
	extraFileBytesMax = 512 << 20
)

// extraFileDirs are the directories the tar creates, every one 0755 root:
// gokrazy's root has none of them, and the guest program mounts or links
// into each.
var extraFileDirs = []string{
	"bin", "sbin", "usr", "usr/bin", "usr/sbin", "usr/lib", guestLibDir, guestLibDir + "/docker",
	"usr/local", "usr/local/bin", "home", "mnt", "mnt/lima-cidata",
}

// extraFileInputs are the files the tar is assembled from.
type extraFileInputs struct {
	// busyboxTar is ze-serial-shell's extra files tar for the arch.
	busyboxTar string
	// staticDir holds the builder container's static binaries.
	staticDir string
	// dockerTgz is Docker's static release tarball for the arch.
	dockerTgz string
}

// writeExtraFiles writes the instance's extrafiles_<arch>.tar at target:
// busybox, the static binaries (sudo setuid root, spec A-9) and Docker's
// binaries under guestLibDir, every member owned by root with a fixed
// modification time so the same inputs give the same bytes. It writes a
// partial file and renames it, so gok never reads half a tar.
func writeExtraFiles(target string, in extraFileInputs) error {
	partial := target + ".partial-" + strconv.Itoa(os.Getpid())
	file, err := os.Create(partial) //nolint:gosec // the instance's extra files tar
	if err != nil {
		return fmt.Errorf("create %s: %w", partial, err)
	}
	defer os.Remove(partial) //nolint:errcheck // gone already once renamed into place

	archive := tar.NewWriter(file)
	err = writeExtraFileMembers(archive, in)
	if err == nil {
		err = archive.Close()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write %s: %w", partial, err)
	}
	if err := os.Rename(partial, target); err != nil {
		return fmt.Errorf("move the extra files into %s: %w", target, err)
	}
	return nil
}

// writeExtraFileMembers writes every member in a fixed order.
func writeExtraFileMembers(archive *tar.Writer, in extraFileInputs) error {
	for _, dir := range extraFileDirs {
		if err := archive.WriteHeader(rootHeader(dir+"/", tar.TypeDir, 0o755, 0)); err != nil {
			return err
		}
	}
	if err := copyBusybox(archive, in.busyboxTar); err != nil {
		return err
	}
	for _, name := range staticBinaryNames {
		mode := int64(0o755)
		if name == sudoBinary {
			// sudo elevates only as a setuid-root file; the mode survives
			// into the squashfs (spec A-9).
			mode = 0o4755
		}
		if err := copyFileMember(archive, filepath.Join(in.staticDir, name), guestLibDir+"/"+name, mode); err != nil {
			return err
		}
	}
	return copyDocker(archive, in.dockerTgz)
}

// copyBusybox copies busybox out of ze-serial-shell's tar.
func copyBusybox(archive *tar.Writer, source string) error {
	file, err := os.Open(source) //nolint:gosec // the committed ze-serial-shell tar
	if err != nil {
		return fmt.Errorf("open busybox's tar %s: %w", source, err)
	}
	defer file.Close() //nolint:errcheck // read-only
	reader := tar.NewReader(file)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("%s holds no %s, the busybox the image runs as its shell. Fix: restore it with "+
				"`git checkout` of cmd/ze-serial-shell/_gokrazy", source, busyboxMember)
		}
		if err != nil {
			return fmt.Errorf("read %s: %w", source, err)
		}
		if strings.TrimPrefix(header.Name, "./") != busyboxMember {
			continue
		}
		return copyMember(archive, reader, header.Size, guestLibDir+"/busybox", 0o755)
	}
}

// copyDocker copies every regular file under docker/ in Docker's tarball.
func copyDocker(archive *tar.Writer, source string) error {
	file, err := os.Open(source) //nolint:gosec // the sum-checked Docker tarball
	if err != nil {
		return fmt.Errorf("open %s: %w", source, err)
	}
	defer file.Close() //nolint:errcheck // read-only
	unzipped, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("read %s: %w", source, err)
	}
	reader := tar.NewReader(unzipped)
	copied := 0
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read %s: %w", source, err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		name := path.Clean(header.Name)
		if !strings.HasPrefix(name, dockerMemberPrefix) {
			continue
		}
		if strings.Contains(strings.TrimPrefix(name, dockerMemberPrefix), "/") {
			continue
		}
		if err := copyMember(archive, reader, header.Size, guestLibDir+"/"+name, 0o755); err != nil {
			return err
		}
		copied++
	}
	if copied == 0 {
		return fmt.Errorf("%s holds no file under %s, so it is not Docker's static tarball. Fix: check the docker url in %s", source, dockerMemberPrefix, PinFile)
	}
	return nil
}

// copyFileMember copies one host file into the tar.
func copyFileMember(archive *tar.Writer, source, name string, mode int64) error {
	file, err := os.Open(source) //nolint:gosec // a static binary in the docker-host cache
	if err != nil {
		return fmt.Errorf("open %s: %w", source, err)
	}
	defer file.Close() //nolint:errcheck // read-only
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("inspect %s: %w", source, err)
	}
	return copyMember(archive, file, info.Size(), name, mode)
}

// copyMember writes one regular-file member of size bytes read from reader.
func copyMember(archive *tar.Writer, reader io.Reader, size int64, name string, mode int64) error {
	if size > extraFileBytesMax {
		return fmt.Errorf("%s is %d bytes, above the %d a binary of the image may be", name, size, extraFileBytesMax)
	}
	header := rootHeader(name, tar.TypeReg, mode, size)
	if err := archive.WriteHeader(header); err != nil {
		return err
	}
	if _, err := io.CopyN(archive, reader, size); err != nil {
		return fmt.Errorf("copy %s: %w", name, err)
	}
	return nil
}

// rootHeader is a member header owned by root with a fixed time.
func rootHeader(name string, kind byte, mode, size int64) *tar.Header {
	return &tar.Header{
		Typeflag: kind,
		Name:     name,
		Mode:     mode,
		Size:     size,
		Uid:      0,
		Gid:      0,
		Uname:    "root",
		Gname:    "root",
		ModTime:  time.Unix(0, 0),
		Format:   tar.FormatUSTAR,
	}
}
