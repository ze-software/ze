// Design: docs/architecture/testing/qemu-integration.md -- Docker labs on the docker-host image
// Overview: dockerlab.go -- the boot that attaches this volume
// Related: ../../../appliance/dockerhost/cidata.go -- ParseCidata, the guest side that reads it
//
// cidata.go writes the cidata volume the docker-host image reads at boot: an
// ISO 9660 image whose volume id is CIDATA, holding Lima's lima.env,
// meta-data, user-data and boot.sh. The guest finds the volume by that id on a
// virtio disk (dockerhost FindCidata) and mounts it as iso9660, exactly as it
// does under Lima, so the QEMU route and the Colima route meet one guest
// contract.
//
// The writer is Ze's own (spec D-10). Lima uses github.com/diskfs/go-diskfs,
// which is not vendored, and no third-party import enters without the
// owner's agreement; a host tool (hdiutil on macOS, xorriso or genisoimage on
// Linux) would add a per-OS dependency to the nightly and to every Mac. The
// volume holds four small files in one directory, which ISO 9660 states in
// three sectors of structure: a primary volume descriptor, its terminator,
// two one-entry path tables and the root directory.

package testqemu

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// isoSectorOctets is ISO 9660's logical sector and block size.
	isoSectorOctets = 2048
	// The volume's fixed layout, in sectors: sixteen of system area, then the
	// primary volume descriptor, the set terminator, the L and M path tables,
	// the root directory, and the files from isoFirstFileSector on.
	isoPVDSector        = 16
	isoTerminatorSector = 17
	isoPathTableLSector = 18
	isoPathTableMSector = 19
	isoRootSector       = 20
	isoFirstFileSector  = 21
	// isoPathTableOctets is one path table holding the root alone: an
	// 8-octet entry, a one-octet identifier and its padding octet.
	isoPathTableOctets = 10
	// isoRootRecordOctets is a directory record whose identifier is one
	// octet: the root in the descriptor, and "." and ".." in the root.
	isoRootRecordOctets = 34
	// cidataVolumeID is the volume identifier the guest matches, lower-cased.
	cidataVolumeID = "CIDATA"
	// cidataNameOctetsMax bounds a file name. ISO 9660 level 2 allows 31
	// octets with the ";1" version; Lima's names are at most 9.
	cidataNameOctetsMax = 28
	// cidataFileOctetsMax is the guest's own bound on one cidata file
	// (dockerhost cidataFileMax): a larger file would be refused at boot.
	cidataFileOctetsMax = 1 << 20
)

// isoRecordDate is every directory record's recording date: 2026-01-01
// 00:00 UTC, as years since 1900, month, day, hour, minute, second and
// offset. A fixed date keeps the volume a function of its files.
var isoRecordDate = [7]byte{126, 1, 1, 0, 0, 0, 0}

// cidataFile is one file of the volume, named as the guest reads it.
type cidataFile struct {
	// name is the guest's name: lower-case letters, digits, '.', '-' and '_'.
	name string
	body []byte
}

// writeCidataISO writes files as the cidata volume at path, mode 0600.
func writeCidataISO(path string, files []cidataFile) error {
	image, err := cidataISO(files)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, image, 0o600); err != nil {
		return fmt.Errorf("write the cidata volume %s: %w", path, err)
	}
	return nil
}

// cidataISO answers the ISO 9660 image holding files in its root directory,
// sorted by recorded name as ISO 9660 requires. A recorded name is the
// guest's name upper-cased, with a '.' added when it has none, and the ";1"
// version: Linux's isofs reads it back lower-cased, with ";1" and a trailing
// '.' dropped (isofs_name_translate), which is the guest's name again.
func cidataISO(files []cidataFile) ([]byte, error) {
	recorded := make([][]byte, len(files))
	rootOctets := 2 * isoRootRecordOctets
	for i, file := range files {
		name, err := isoRecordedName(file.name)
		if err != nil {
			return nil, err
		}
		if len(file.body) > cidataFileOctetsMax {
			return nil, fmt.Errorf("cidata file %s is %d bytes, above the %d the guest reads", file.name, len(file.body), cidataFileOctetsMax)
		}
		recorded[i] = name
		rootOctets += isoRecordOctets(len(name))
	}
	if rootOctets > isoSectorOctets {
		return nil, fmt.Errorf("cidata root directory needs %d bytes, above the one %d-byte sector this writer lays out", rootOctets, isoSectorOctets)
	}
	order := make([]int, len(files))
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(a, b int) int { return strings.Compare(string(recorded[a]), string(recorded[b])) })

	extents := make([]uint32, len(files))
	sector := uint32(isoFirstFileSector)
	for _, i := range order {
		extents[i] = sector
		sector += uint32((len(files[i].body) + isoSectorOctets - 1) / isoSectorOctets) //nolint:gosec // bounded by cidataFileOctetsMax
	}
	image := make([]byte, int(sector)*isoSectorOctets)

	writePrimaryDescriptor(image[isoPVDSector*isoSectorOctets:], sector)
	terminator := image[isoTerminatorSector*isoSectorOctets:]
	terminator[0] = 255
	copy(terminator[1:6], "CD001")
	terminator[6] = 1
	writePathTable(image[isoPathTableLSector*isoSectorOctets:], binary.LittleEndian)
	writePathTable(image[isoPathTableMSector*isoSectorOctets:], binary.BigEndian)

	root := image[isoRootSector*isoSectorOctets:]
	offset := putDirRecord(root, isoRootSector, isoSectorOctets, true, []byte{0})
	offset += putDirRecord(root[offset:], isoRootSector, isoSectorOctets, true, []byte{1})
	for _, i := range order {
		offset += putDirRecord(root[offset:], extents[i], uint32(len(files[i].body)), false, recorded[i]) //nolint:gosec // bounded by cidataFileOctetsMax
		copy(image[int(extents[i])*isoSectorOctets:], files[i].body)
	}
	return image, nil
}

// isoRecordedName answers the name recorded for the guest's name, and
// refuses a name the guest could not read back as written.
func isoRecordedName(name string) ([]byte, error) {
	if name == "" {
		return nil, errors.New("cidata file has no name")
	}
	if len(name) > cidataNameOctetsMax {
		return nil, fmt.Errorf("cidata file name %q is longer than %d bytes", name, cidataNameOctetsMax)
	}
	for i := range len(name) {
		c := name[i]
		if !isCidataNameOctet(c) {
			return nil, fmt.Errorf("cidata file name %q holds %q: only lower-case letters, digits, '.', '-' and '_' read back unchanged", name, c)
		}
	}
	if strings.Count(name, ".") > 1 {
		return nil, fmt.Errorf("cidata file name %q holds more than one '.', which ISO 9660 does not record", name)
	}
	var b textbuf.Buffer
	b.Str(strings.ToUpper(name))
	if !strings.Contains(name, ".") {
		b.Byte('.')
	}
	return []byte(b.Str(";1").String()), nil
}

// isCidataNameOctet reports whether c survives isofs's name translation.
func isCidataNameOctet(c byte) bool {
	if c >= 'a' && c <= 'z' {
		return true
	}
	if c >= '0' && c <= '9' {
		return true
	}
	return c == '.' || c == '-' || c == '_'
}

// isoRecordOctets is the length of a directory record whose identifier is
// nameOctets long: 33 fixed octets, the identifier, and a padding octet that
// keeps the record length even.
func isoRecordOctets(nameOctets int) int {
	length := 33 + nameOctets
	if length%2 == 1 {
		length++
	}
	return length
}

// writePrimaryDescriptor fills sector as the primary volume descriptor of a
// volume totalSectors long, whose root directory is isoRootSector.
func writePrimaryDescriptor(sector []byte, totalSectors uint32) {
	sector[0] = 1
	copy(sector[1:6], "CD001")
	sector[6] = 1
	fillSpaces(sector[8:40])             // system identifier
	fillSpaces(sector[40:72])            // volume identifier
	copy(sector[40:72], cidataVolumeID)  // the label the guest matches
	putBoth32(sector[80:], totalSectors) // volume space size
	putBoth16(sector[120:], 1)           // volume set size
	putBoth16(sector[124:], 1)           // volume sequence number
	putBoth16(sector[128:], isoSectorOctets)
	putBoth32(sector[132:], isoPathTableOctets)
	binary.LittleEndian.PutUint32(sector[140:], isoPathTableLSector)
	binary.BigEndian.PutUint32(sector[148:], isoPathTableMSector)
	putDirRecord(sector[156:], isoRootSector, isoSectorOctets, true, []byte{0})
	fillSpaces(sector[190:813]) // set, publisher, preparer, application, file identifiers
	// Creation, modification, expiration and effective dates: sixteen '0'
	// digits and a zero offset each, ISO 9660's "not specified".
	for date := 813; date < 881; date += 17 {
		for i := range 16 {
			sector[date+i] = '0'
		}
	}
	sector[881] = 1 // file structure version
}

// writePathTable fills sector with a path table holding the root alone, its
// numbers in order's byte order: the L table is little-endian, the M table
// big-endian.
func writePathTable(sector []byte, order binary.ByteOrder) {
	sector[0] = 1 // identifier length
	order.PutUint32(sector[2:], isoRootSector)
	order.PutUint16(sector[6:], 1) // the root is its own parent
}

// putDirRecord writes one directory record at the start of buf and answers
// its length.
func putDirRecord(buf []byte, extent, size uint32, directory bool, name []byte) int {
	length := isoRecordOctets(len(name))
	buf[0] = byte(length)
	putBoth32(buf[2:], extent)
	putBoth32(buf[10:], size)
	copy(buf[18:25], isoRecordDate[:])
	if directory {
		buf[25] = 2
	}
	putBoth16(buf[28:], 1) // volume sequence number
	buf[32] = byte(len(name))
	copy(buf[33:], name)
	return length
}

// putBoth32 writes value in ISO 9660's both-byte-order form: little-endian
// then big-endian.
func putBoth32(buf []byte, value uint32) {
	binary.LittleEndian.PutUint32(buf, value)
	binary.BigEndian.PutUint32(buf[4:], value)
}

// putBoth16 writes value in ISO 9660's both-byte-order form.
func putBoth16(buf []byte, value uint16) {
	binary.LittleEndian.PutUint16(buf, value)
	binary.BigEndian.PutUint16(buf[2:], value)
}

// fillSpaces fills an a-character or d-character field with spaces, its
// empty value.
func fillSpaces(field []byte) {
	for i := range field {
		field[i] = ' '
	}
}
