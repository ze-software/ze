// Design: docs/architecture/appliance/ota-push.md -- validated update payloads.

package appliance

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
)

const (
	pushGPTHeaderMin = 92
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

func readPushPartitions(file *os.File) ([4]pushPartition, error) {
	var parts [4]pushPartition
	info, err := file.Stat()
	if err != nil {
		return parts, err
	}
	if info.Size()%gptSectorSize != 0 {
		return parts, errors.New("image is not a complete sector-aligned disk")
	}
	var mbr [gptSectorSize]byte
	if _, err := file.ReadAt(mbr[:], 0); err != nil {
		return parts, fmt.Errorf("read image MBR: %w", err)
	}
	if !bytes.Equal(mbr[510:], []byte{0x55, 0xaa}) {
		return parts, errors.New("image has no MBR signature")
	}
	var header [gptSectorSize]byte
	if _, err := file.ReadAt(header[:], gptSectorSize); err != nil {
		return parts, fmt.Errorf("read image GPT: %w", err)
	}
	if string(header[:8]) != "EFI PART" {
		return parts, errors.New("image has no primary GPT header")
	}
	headerBytes := binary.LittleEndian.Uint32(header[12:16])
	if headerBytes < pushGPTHeaderMin {
		return parts, errors.New("image GPT header is too short")
	}
	if headerBytes > gptSectorSize {
		return parts, errors.New("image GPT header exceeds its sector")
	}
	wantCRC := binary.LittleEndian.Uint32(header[16:20])
	clear(header[16:20])
	if crc32.ChecksumIEEE(header[:headerBytes]) != wantCRC {
		return parts, errors.New("image GPT header checksum does not match")
	}
	sectors := uint64(info.Size() / gptSectorSize)
	firstUsable := binary.LittleEndian.Uint64(header[40:48])
	lastUsable := binary.LittleEndian.Uint64(header[48:56])
	if lastUsable >= sectors {
		return parts, errors.New("image GPT extends beyond the disk")
	}
	if firstUsable > lastUsable {
		return parts, errors.New("image GPT usable sectors are reversed")
	}
	entryLBA := binary.LittleEndian.Uint64(header[72:80])
	count := binary.LittleEndian.Uint32(header[80:84])
	entryBytes := binary.LittleEndian.Uint32(header[84:88])
	if count < uint32(len(parts)) {
		return parts, errors.New("image GPT does not hold the four appliance partitions")
	}
	if count > gptMaxEntries {
		return parts, errors.New("image GPT exceeds the appliance partition limit")
	}
	if entryBytes != gptEntrySize {
		return parts, errors.New("image GPT entry size is not supported")
	}
	if entryLBA < 2 {
		return parts, errors.New("image GPT entries overlap its header")
	}
	if entryLBA >= firstUsable {
		return parts, errors.New("image GPT entries overlap its partitions")
	}
	arrayBytes := uint64(count) * uint64(entryBytes)
	if arrayBytes > (firstUsable-entryLBA)*gptSectorSize {
		return parts, errors.New("image GPT entry array exceeds its reserved sectors")
	}
	entries := make([]byte, arrayBytes)
	if _, err := file.ReadAt(entries, int64(entryLBA)*gptSectorSize); err != nil {
		return parts, fmt.Errorf("read image GPT entries: %w", err)
	}
	if crc32.ChecksumIEEE(entries) != binary.LittleEndian.Uint32(header[88:92]) {
		return parts, errors.New("image GPT entry checksum does not match")
	}
	var emptyType [16]byte
	for index := range parts {
		entry := entries[index*gptEntrySize : (index+1)*gptEntrySize]
		if bytes.Equal(entry[:16], emptyType[:]) {
			return parts, errors.New("image is missing an appliance partition")
		}
		first := binary.LittleEndian.Uint64(entry[32:40])
		last := binary.LittleEndian.Uint64(entry[40:48])
		if first < firstUsable {
			return parts, errors.New("image partition overlaps disk metadata")
		}
		if last > lastUsable {
			return parts, errors.New("image partition extends beyond the usable disk")
		}
		if last < first {
			return parts, errors.New("image partition sectors are reversed")
		}
		parts[index] = pushPartition{offset: int64(first) * gptSectorSize, bytes: int64(last-first+1) * gptSectorSize}
		for previous := range index {
			if parts[index].offset < parts[previous].offset+parts[previous].bytes {
				return parts, errors.New("image appliance partitions overlap or are out of order")
			}
		}
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
