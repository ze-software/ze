// Design: docs/architecture/appliance/docker-host.md -- /perm on a darwin-built image
// Related: build.go -- the build that formats the image gok wrote

package dockerhost

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"

	"github.com/ze-software/ze/internal/appliance"
	"github.com/ze-software/ze/internal/core/gptdisk"
)

const (
	// ext4MagicOffset is the superblock's magic, 1024 bytes into the
	// partition plus 56 into the superblock.
	ext4MagicOffset = 1024 + 56
)

// linuxDataTypeGUID is the GPT type of a Linux file system partition, in the
// mixed-endian order it is stored on disk (0FC63DAF-8483-4772-8E79-3D69D8477DE4).
var linuxDataTypeGUID = []byte{0xaf, 0x3d, 0xc6, 0x0f, 0x83, 0x84, 0x72, 0x47, 0x8e, 0x79, 0x3d, 0x69, 0xd8, 0x47, 0x7d, 0xe4}

// partitionSpan is a partition's place in an image, in bytes.
type partitionSpan struct {
	offset int64
	size   int64
}

// formatPerm gives the image's /perm partition an ext4 file system when it
// carries none. gok formats /perm only where mkfs.ext4 is on its PATH, which
// a Mac does not have (spec Phase 1 constraint); the offset and size come
// from the image's own partition table, never a constant.
func formatPerm(image string) error {
	span, err := permPartition(image)
	if err != nil {
		return err
	}
	formatted, err := hasExt4(image, span)
	if err != nil {
		return err
	}
	if formatted {
		return nil
	}
	mkfs, err := findMkfs()
	if err != nil {
		return err
	}
	// The size argument is in KiB, which divides a partition gok aligns to MiB.
	output, err := exec.Command(mkfs, "-F", "-E", "offset="+strconv.Itoa(int(span.offset)), //nolint:gosec,noctx // a local formatter over the build's own image
		image, strconv.Itoa(int(span.size/1024))+"k").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s over /perm in %s failed: %w: %s", mkfs, image, err, bytes.TrimSpace(output))
	}
	return nil
}

// findMkfs answers the mkfs.ext4 to run, found where `ze appliance build`
// finds it: Homebrew keeps e2fsprogs keg-only, so it is not on PATH on a Mac.
func findMkfs() (string, error) {
	if path := appliance.ResolveE2FSTool("mkfs.ext4"); path != "" {
		return path, nil
	}
	return "", errors.New("mkfs.ext4 is not installed, and the image's /perm needs an ext4 file system. " +
		"Fix: brew install e2fsprogs (macOS) or install e2fsprogs (Linux)")
}

// permPartition reads the GPT through the shared parse and answers partition
// 4, refusing an image whose fourth entry is not a Linux file system
// partition: that rule is the lab image's own.
func permPartition(image string) (partitionSpan, error) {
	file, err := os.Open(image) //nolint:gosec // the image this build wrote
	if err != nil {
		return partitionSpan{}, fmt.Errorf("open %s: %w", image, err)
	}
	defer file.Close() //nolint:errcheck // read-only
	info, err := file.Stat()
	if err != nil {
		return partitionSpan{}, fmt.Errorf("stat %s: %w", image, err)
	}
	if info.Size() < 0 {
		return partitionSpan{}, fmt.Errorf("%s reports a negative size", image)
	}
	table, err := gptdisk.Parse(file, uint64(info.Size()))
	if err != nil {
		return partitionSpan{}, fmt.Errorf("%s has no partition table /perm can be found in: %w", image, err)
	}
	entry, ok := table.Partition(gptdisk.PersistentPartition)
	if !ok {
		return partitionSpan{}, fmt.Errorf("%s's GPT has no partition %d, gokrazy's /perm", image, gptdisk.PersistentPartition)
	}
	if !bytes.Equal(entry.Type[:], linuxDataTypeGUID) {
		return partitionSpan{}, fmt.Errorf("partition %d of %s is not a Linux file system partition, so it is not gokrazy's /perm", gptdisk.PersistentPartition, image)
	}
	return partitionSpan{offset: int64(entry.OffsetBytes()), size: int64(entry.SizeBytes())}, nil //nolint:gosec // bounded by the image's size, which fits int64
}

// hasExt4 answers whether the partition carries an ext4 superblock.
func hasExt4(image string, span partitionSpan) (bool, error) {
	file, err := os.Open(image) //nolint:gosec // the image this build wrote
	if err != nil {
		return false, fmt.Errorf("open %s: %w", image, err)
	}
	defer file.Close() //nolint:errcheck // read-only
	magic := make([]byte, 2)
	if _, err := file.ReadAt(magic, span.offset+ext4MagicOffset); err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("read /perm's superblock in %s: %w", image, err)
	}
	return magic[0] == 0x53 && magic[1] == 0xef, nil
}
