// Design: docs/architecture/appliance/ota-push.md -- validated update payloads.

package appliance

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ze-software/ze/internal/core/gptdisk"
)

const (
	pushRootHeader   = 96
	pushMBRCodeBytes = 440
)

// pushImage holds readers admitted by readPushImage. Readers remain bounded to
// the validated payloads; the source file MUST stay open until streaming ends.
// Its zero value is uninitialized and MUST NOT be passed to the updater.
type pushImage struct {
	boot *io.SectionReader
	mbr  *io.SectionReader
	root *io.SectionReader
}

type pushPartition struct {
	offset int64
	bytes  int64
}

func readPushImage(file *os.File) (pushImage, error) {
	parts, err := readPushPartitions(file)
	if err != nil {
		return pushImage{}, err
	}
	boot, err := readPushBoot(file, parts[0])
	if err != nil {
		return pushImage{}, err
	}
	root, err := readPushRoot(file, parts[1])
	if err != nil {
		return pushImage{}, err
	}
	return pushImage{boot: boot, root: root, mbr: io.NewSectionReader(file, 0, pushMBRCodeBytes)}, nil
}

// readPushPartitions validates the image file's table through the shared
// parse and answers the four appliance partitions. Its own rules are the
// push's: entries 1 to 4 are all present, in disk order. Parse already
// refuses overlapping entries.
func readPushPartitions(file *os.File) ([4]pushPartition, error) {
	var parts [4]pushPartition
	info, err := file.Stat()
	if err != nil {
		return parts, err
	}
	if info.Size() < 0 {
		return parts, errors.New("image reports a negative size")
	}
	table, err := gptdisk.Parse(file, uint64(info.Size()))
	if err != nil {
		return parts, fmt.Errorf("image partition table: %w", err)
	}
	var previous gptdisk.Entry
	for index := range parts {
		entry, ok := table.Partition(index + 1)
		if !ok {
			return parts, errors.New("image is missing an appliance partition")
		}
		if index > 0 && entry.FirstLBA <= previous.LastLBA {
			return parts, errors.New("image appliance partitions are out of order")
		}
		parts[index] = pushPartition{offset: int64(entry.OffsetBytes()), bytes: int64(entry.SizeBytes())} //nolint:gosec // bounded by the image's size, which fits int64
		previous = entry
	}
	return parts, nil
}

func readPushBoot(file *os.File, part pushPartition) (*io.SectionReader, error) {
	var header [gptSectorSize]byte
	if _, err := file.ReadAt(header[:], part.offset); err != nil {
		return nil, fmt.Errorf("read boot filesystem: %w", err)
	}
	if !bytes.Equal(header[510:], []byte{0x55, 0xaa}) {
		return nil, errors.New("image boot partition has no FAT signature")
	}
	sectorBytes := binary.LittleEndian.Uint16(header[11:13])
	if sectorBytes != gptSectorSize {
		return nil, errors.New("image boot filesystem has an unsupported sector size")
	}
	sectors := uint32(binary.LittleEndian.Uint16(header[19:21]))
	if sectors == 0 {
		sectors = binary.LittleEndian.Uint32(header[32:36])
	}
	size := int64(sectors) * int64(sectorBytes)
	if size < gptSectorSize {
		return nil, errors.New("image boot filesystem is empty")
	}
	if size > part.bytes {
		return nil, errors.New("image boot filesystem exceeds its partition")
	}
	return io.NewSectionReader(file, part.offset, size), nil
}

func readPushRoot(file *os.File, part pushPartition) (*io.SectionReader, error) {
	var header [pushRootHeader]byte
	if _, err := file.ReadAt(header[:], part.offset); err != nil {
		return nil, fmt.Errorf("read root filesystem: %w", err)
	}
	if string(header[:4]) != "hsqs" {
		return nil, errors.New("image root partition is not SquashFS")
	}
	if binary.LittleEndian.Uint16(header[28:30]) != 4 {
		return nil, errors.New("image SquashFS version is not supported")
	}
	size := binary.LittleEndian.Uint64(header[40:48])
	if size < pushRootHeader {
		return nil, errors.New("image root filesystem is too short")
	}
	if size > uint64(part.bytes) {
		return nil, errors.New("image root filesystem exceeds its partition")
	}
	return io.NewSectionReader(file, part.offset, int64(size)), nil
}
