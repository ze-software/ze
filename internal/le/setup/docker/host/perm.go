// Design: docs/architecture/appliance/docker-host.md -- /perm on a darwin-built image
// Related: build.go -- the build that formats the image gok wrote

package dockerhost

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"

	"github.com/ze-software/ze/internal/appliance"
)

const (
	// sectorBytes is the logical sector gok writes the GPT with.
	sectorBytes = 512
	// permPartitionNumber is /perm's place in gokrazy's partition table:
	// boot, root A, root B, perm.
	permPartitionNumber = 4
	// gptEntriesMax bounds the partition entries read from the header.
	gptEntriesMax = 128
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

// permPartition reads the GPT and answers partition 4, refusing an image
// whose fourth entry is not a Linux file system partition.
func permPartition(image string) (partitionSpan, error) {
	file, err := os.Open(image) //nolint:gosec // the image this build wrote
	if err != nil {
		return partitionSpan{}, fmt.Errorf("open %s: %w", image, err)
	}
	defer file.Close() //nolint:errcheck // read-only

	header := make([]byte, 92)
	if _, err := file.ReadAt(header, sectorBytes); err != nil {
		return partitionSpan{}, fmt.Errorf("read the GPT header of %s: %w", image, err)
	}
	if string(header[:8]) != "EFI PART" {
		return partitionSpan{}, fmt.Errorf("%s has no GPT header at sector 1, so /perm cannot be found", image)
	}
	entriesLBA := binary.LittleEndian.Uint64(header[72:80])
	entries := binary.LittleEndian.Uint32(header[80:84])
	entryBytes := binary.LittleEndian.Uint32(header[84:88])
	if entries < permPartitionNumber {
		return partitionSpan{}, fmt.Errorf("%s's GPT holds %d entries, fewer than gokrazy's four", image, entries)
	}
	if entries > gptEntriesMax {
		return partitionSpan{}, fmt.Errorf("%s's GPT claims %d entries, above %d", image, entries, gptEntriesMax)
	}
	if entryBytes < 128 {
		return partitionSpan{}, fmt.Errorf("%s's GPT entries are %d bytes, below the 128 the format fixes", image, entryBytes)
	}
	entry := make([]byte, 128)
	at := int64(entriesLBA)*sectorBytes + int64(permPartitionNumber-1)*int64(entryBytes) //nolint:gosec // bounded by the checks above
	if _, err := file.ReadAt(entry, at); err != nil {
		return partitionSpan{}, fmt.Errorf("read GPT entry %d of %s: %w", permPartitionNumber, image, err)
	}
	if !bytes.Equal(entry[:16], linuxDataTypeGUID) {
		return partitionSpan{}, fmt.Errorf("partition %d of %s is not a Linux file system partition, so it is not gokrazy's /perm", permPartitionNumber, image)
	}
	first := int64(binary.LittleEndian.Uint64(entry[32:40])) //nolint:gosec // an LBA in an image this build wrote
	last := int64(binary.LittleEndian.Uint64(entry[40:48]))  //nolint:gosec // an LBA in an image this build wrote
	if last < first {
		return partitionSpan{}, fmt.Errorf("partition %d of %s ends before it starts", permPartitionNumber, image)
	}
	return partitionSpan{offset: first * sectorBytes, size: (last - first + 1) * sectorBytes}, nil
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
