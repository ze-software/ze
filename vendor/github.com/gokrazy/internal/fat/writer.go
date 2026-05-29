package fat

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
)

const (
	sectorSize        = uint16(512)
	sectorsPerCluster = uint8(4)

	clusterSize = int(sectorSize) * int(sectorsPerCluster)

	// unusableClusters is the number of clusters which are always unusable in a
	// FAT: the first two entries have special meaning (copy of the media
	// descriptor and file system state).
	unusableClusters = uint16(2)

	// endOfChain marks the end of a cluster chain in the FAT.
	endOfChain = uint16(0xFFFF)

	// hardDisk is the media descriptor for a hard disk (as opposed to floppy).
	hardDisk = uint8(0xF8)

	// clean describes a cleanly unmounted FAT file system.
	clean = uint16(0xFFFF)
)

type paddingWriter struct {
	w     io.Writer
	count int
	padTo int
}

func (pw *paddingWriter) Write(p []byte) (n int, err error) {
	pw.count += int(len(p))
	return pw.w.Write(p)
}

func (pw *paddingWriter) Flush() error {
	if pw.count%pw.padTo == 0 {
		return nil
	}
	remainder := pw.padTo - (pw.count % pw.padTo)
	pw.count += remainder
	_, err := pw.w.Write(make([]byte, remainder))
	return err
}

type entry interface {
	Name() [8]byte
	Ext() [3]byte
	FullName() string
	Attr() uint8
	Size() uint32
	FirstCluster() uint16
	Date() uint16
	Time() uint16
}

type common struct {
	name         string
	ext          string
	modTime      time.Time
	size         uint32
	firstCluster uint16
}

var empty = [8]byte{' ', ' ', ' ', ' ', ' ', ' ', ' ', ' '}

func (c *common) FullName() string {
	if len(c.ext) > 0 {
		return c.name + "." + c.ext
	}
	return c.name
}

func (c *common) Name() [8]byte {
	var result [8]byte
	copy(result[:], empty[:])
	copy(result[:], []byte(c.name))
	return result
}

func (c *common) Ext() [3]byte {
	var result [3]byte
	copy(result[:], empty[:3])
	copy(result[:], []byte(c.ext))
	return result
}

func (c *common) Size() uint32 {
	return c.size
}

func (c *common) FirstCluster() uint16 {
	return c.firstCluster
}

func (c *common) Time() uint16 {
	return uint16(c.modTime.Hour())<<11 |
		uint16(c.modTime.Minute())<<5 |
		uint16(c.modTime.Second()/2)
}

func (c *common) Date() uint16 {
	return uint16(c.modTime.Year()-1980)<<9 |
		uint16(c.modTime.Month())<<5 |
		uint16(c.modTime.Day())
}

const (
	attrReadOnly  = 0x01
	attrHidden    = 0x02
	attrSystem    = 0x04
	attrVolumeId  = 0x08
	attrDirectory = 0x10
	attrArchive   = 0x20
	attrLongName  = attrReadOnly | attrHidden | attrSystem | attrVolumeId
)

type file struct {
	common
}

func (f *file) Attr() uint8 {
	return 0x1 // read-only
}

type directory struct {
	common
	entries []entry
	byName  map[string]entry
	parent  *directory
}

func (d *directory) Attr() uint8 {
	return attrDirectory
}

type Writer struct {
	w io.Writer

	// dataTmp is a temporary file to which all file data will be
	// written. Calling Flush will write the appropriate headers (for
	// which the file data must be known) to the writer, then append
	// dataTmp’s contents.
	dataTmp *os.File

	// fat is a File Allocation Table holding one entry for each
	// sector in the data area, pointing to the FAT entry index of the
	// next sector or (with special value 0xFFFF) marking the end of
	// the file.
	fat []uint16

	root *directory

	pending *fatUpdatingWriter

	TotalSectors int // populated after Flush
}

// NewWriter returns a Writer which will write a FAT16B file system
// image to w once Flush is called.
//
// Because the position of the data area in the resulting image
// depends on the size of the file allocation table and number of root
// directory entries, a temporary file is used to store data until
// Flush is called.
func NewWriter(w io.Writer) (*Writer, error) {
	f, err := ioutil.TempFile("", "writefat")
	if err != nil {
		return nil, err
	}

	return &Writer{
		w:       w,
		dataTmp: f,
		root: &directory{
			byName: make(map[string]entry),
		},
		fat: []uint16{
			(uint16(0xFF) << 8) | uint16(hardDisk), // media descriptor
			clean,                                  // file system state
		},
	}, nil
}

func (fw *Writer) currentCluster() uint16 {
	return unusableClusters + uint16(len(fw.fat)-2)
}

func (fw *Writer) dir(path string) (*directory, error) {
	cur := fw.root
	for _, component := range strings.Split(path, "/") {
		if component == "" {
			continue
		}
		if _, ok := cur.byName[component]; !ok {
			dir := &directory{
				common: common{
					name: component,
				},
				parent: cur,
				byName: make(map[string]entry),
			}
			cur.entries = append(cur.entries, dir)
			cur.byName[component] = dir
		}
		var ok bool
		cur, ok = cur.byName[component].(*directory)
		if !ok {
			return nil, fmt.Errorf("path %q invalid: component %q identifies a file", path, component)
		}
	}
	return cur, nil
}

// Mkdir creates an empty directory with the given full path,
// e.g. Mkdir("usr/share/lib").
func (fw *Writer) Mkdir(path string, modTime time.Time) error {
	if fw.pending != nil {
		if err := fw.pending.Close(); err != nil {
			return err
		}
		fw.pending = nil
	}
	d, err := fw.dir(path)
	d.common.modTime = modTime.UTC()
	return err
}

type fatUpdatingWriter struct {
	fw    *Writer
	pw    *paddingWriter
	count uint32
	file  *file
}

func (fuw *fatUpdatingWriter) Write(p []byte) (n int, err error) {
	fuw.count += uint32(len(p))
	return fuw.pw.Write(p)
}

func (fuw *fatUpdatingWriter) Close() error {
	if err := fuw.pw.Flush(); err != nil {
		return err
	}
	fw := fuw.fw // for convenience
	if fuw.count == 0 {
		if fuw.file != nil {
			fuw.file.firstCluster = 0
		}
		return nil
	}
	for i := 0; i < fuw.pw.count/clusterSize; i++ {
		// Append a pointer to the next FAT entry
		fw.fat = append(fw.fat, fw.currentCluster()+1)
	}
	fw.fat[len(fw.fat)-1] = endOfChain
	if fuw.file != nil {
		fuw.file.size = uint32(fuw.count)
	}
	return nil
}

// Exists reports whether the specified file exists. It creates directories like
// Mkdir while checking, so should only be used if you are about to call File.
func (fw *Writer) Exists(path string) (bool, error) {
	if fw.pending != nil {
		if err := fw.pending.Close(); err != nil {
			return false, err
		}
		fw.pending = nil
	}

	dir, err := fw.dir(filepath.Dir(path))
	if err != nil {
		return false, err
	}
	_, exists := dir.byName[filepath.Base(path)]
	return exists, nil
}

// File creates a file with the specified path and modTime. The
// returned io.Writer stays valid until the next call to File, Flush,
// Mkdir or Exists.
func (fw *Writer) File(path string, modTime time.Time) (io.Writer, error) {
	if fw.pending != nil {
		if err := fw.pending.Close(); err != nil {
			return nil, err
		}
		fw.pending = nil
	}
	dir, err := fw.dir(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	filename := filepath.Base(path)
	parts := strings.Split(filename+".", ".")
	f := &file{
		common: common{
			name:         parts[0],
			ext:          parts[1],
			modTime:      modTime.UTC(),
			firstCluster: fw.currentCluster()}}
	dir.entries = append(dir.entries, f)
	dir.byName[filename] = f
	fw.pending = &fatUpdatingWriter{
		fw: fw,
		pw: &paddingWriter{
			w:     fw.dataTmp,
			padTo: clusterSize,
		},
		file: f,
	}
	return fw.pending, nil
}

func (fw *Writer) writeFAT() error {
	w := &paddingWriter{
		w:     fw.w,
		padTo: int(sectorSize)}

	for _, entry := range fw.fat {
		if err := binary.Write(w, binary.LittleEndian, entry); err != nil {
			return err
		}
	}

	return w.Flush()
}

func dirEntryCount(d *directory) int {
	count := 1 // volume label
	for _, e := range d.entries {
		count++                                // short file name entry
		count += (len(e.FullName()) + 12) / 13 // long file name entries
	}
	return count
}

func (fw *Writer) usableFATEntries() int {
	return len(fw.fat) - int(unusableClusters)
}

func (fw *Writer) writeBootSector(w io.Writer, fatSectors, reservedSectors int) (int, error) {
	dataSectors := fw.usableFATEntries() * int(sectorsPerCluster)
	rootDirEntries := dirEntryCount(fw.root)
	// The root directory must span an integral number of sectors:
	const (
		dirEntrySize     = 32 // bytes
		entriesPerSector = (int(sectorSize) / dirEntrySize)
	)
	rootDirSectors := ((rootDirEntries + entriesPerSector - 1) / entriesPerSector)
	rootDirEntries = rootDirSectors * entriesPerSector
	totalSectors := reservedSectors + rootDirSectors + fatSectors + dataSectors
	var (
		jumpCode            = [3]byte{0xEB, 0x3C, 0x90}
		OEM                 = [8]byte{'g', 'o', 'k', 'r', 'a', 'z', 'y', '!'}
		volumeLabel         = [11]byte{'g', 'o', 'k', 'r', 'a', 'z', 'y', ' ', ' ', ' ', ' '}
		fileSystemType      = [8]byte{'F', 'A', 'T', '1', '6', ' ', ' ', ' '}
		bootCode            = [448]byte{}
		bootSectorSignature = [2]byte{0x55, 0xAA}
	)
	for _, v := range []interface{}{
		jumpCode,                // jump code: intel 80x86 jump instruction
		OEM,                     // OEM
		sectorSize,              // in bytes
		sectorsPerCluster,       // i.e. each FAT entry covers sectorsPerCluster*sectorSize bytes
		uint16(reservedSectors), // reserved sectors
		uint8(1),                // one copy of the FAT
		uint16(rootDirEntries),  // root directory entries
		uint16(0),               // 0 = use uint32 number of sectors following later
		hardDisk,                // media descriptor
		uint16(fatSectors),      // number of sectors per FAT
		uint16(32),              // (only for bootcode) number of sectors per track
		uint16(4),               // (only for bootcode) number of heads
		uint32(1),               // no hidden sectors
		uint32(totalSectors),    // total number of sectors
		uint8(0x80),             // (only for bootcode) drive number
		uint8(0),                // (only for bootcode) current head
		uint8(0x29),             // magic value: boot signature
		uint32(0xf3f37b84),      // TODO: volume ID
		volumeLabel,
		fileSystemType,
		bootCode,
		bootSectorSignature,
	} {
		if err := binary.Write(w, binary.LittleEndian, v); err != nil {
			return 0, err
		}
	}
	return totalSectors, nil
}

func shortFileName(name string, seen map[string]bool) (primary, ext string) {
	return shortFileNameBoth(strings.ToUpper(name), seen)
}

func shortFileNameWrite(name string, seen map[string]bool) (primary, ext string) {
	// TODO(correctness): convert to upper-case. cannot do this right away for
	// backwards compatibility: older gokrazy FAT readers only look for
	// lower-case filenames.
	return shortFileNameBoth(name, seen)
}

func shortFileNameBoth(name string, seen map[string]bool) (primary, ext string) {
	if name == "." || name == ".." {
		return name + strings.Repeat(" ", 8-len(name)), "   "
	}
	basis := name
	// TODO(correctness): convert to OEM charset
	basis = strings.Replace(basis, " ", "", -1)
	for strings.HasPrefix(basis, ".") {
		basis = strings.TrimPrefix(basis, ".")
	}
	fit := true
	primary = basis
	if idx := strings.LastIndex(primary, "."); idx > -1 {
		primary = primary[:idx]
	}
	if len(primary) > 8 {
		primary = primary[:8]
		fit = false
	}
	ext = basis
	if idx := strings.LastIndex(ext, "."); idx > -1 {
		ext = ext[idx+1:]
		if len(ext) > 3 {
			ext = ext[:3]
			fit = false
		}
		if len(ext) < 3 {
			ext = ext + strings.Repeat(" ", 3-len(ext))
		}
	} else {
		ext = "   "
	}
	if !fit {
		// Generate numeric tail
		for n := 1; n <= 999999; n++ {
			tail := "~" + strconv.Itoa(n)
			suggestion := primary + tail
			if len(primary)+len(tail) > 8 {
				suggestion = primary[:8-len(tail)] + tail
			}
			if !seen[suggestion] {
				primary = suggestion
				seen[primary] = true
				break
			}
		}
	}
	if len(primary) < 8 {
		primary = primary + strings.Repeat(" ", 8-len(primary))
	}

	return primary, ext
}

func (fw *Writer) writeDirEntries(w io.Writer, d *directory) error {
	allEntries := d.entries
	// For non-root directories, add dot and dotdot
	if d.parent != nil {
		allEntries = append([]entry{
			&directory{
				common: common{
					name:         ".",
					firstCluster: d.firstCluster,
				},
				parent: d,
			},
			&directory{
				common: common{
					name:         "..",
					firstCluster: d.parent.firstCluster,
				},
				parent: d.parent,
			},
		}, allEntries...)
	}

	if d.parent == nil {
		// For the root directory, include a volume label directory entry as
		// first entry, too:
		for _, v := range []interface{}{
			[11]byte{'g', 'o', 'k', 'r', 'a', 'z', 'y', ' ', ' ', ' ', ' '},
			uint8(attrVolumeId),
			[20]byte{},
		} {
			if err := binary.Write(w, binary.LittleEndian, v); err != nil {
				return err
			}
		}
	}

	seen := make(map[string]bool)
	for _, entry := range allEntries {
		// Long Directory Entry
		name := entry.FullName()
		chunks := (len(name) + 12) / 13                    // rounded up to 13 bytes
		buf := bytes.Repeat([]byte{0xFF, 0xFF}, chunks*13) // padded with 0xFFFF
		padded := []rune(name)
		if len(name)%13 != 0 {
			padded = append([]rune(name), 0)
		}
		for i, enc := range utf16.Encode(padded) {
			binary.LittleEndian.PutUint16(buf[i*2:], enc)
		}
		primary, ext := shortFileNameWrite(name, seen)
		if name != "." && name != ".." {
			checksum := uint8(0)
			for _, ch := range []byte(primary + ext) {
				checksum = (((checksum & 1) << 7) | ((checksum & 0xFE) >> 1)) + ch
			}
			for i := chunks - 1; i >= 0; i-- {
				order := byte(i + 1) // 1-based
				if i == chunks-1 {
					order |= 0x40 // LAST_LONG_ENTRY
				}
				namebuf := buf[i*13*2:]
				for _, v := range []interface{}{
					order,               // order in the sequence of long dir entries
					namebuf[0 : 0+10],   // characters 1-5
					byte(attrLongName),  // always attrLongName
					byte(0),             // always 0 (reserved)
					checksum,            // checksum over the corresponding short directory entry
					namebuf[10 : 10+12], // characters 6-11
					uint16(0),           // always 0 (older tools may interpret this as first cluster)
					namebuf[22 : 22+4],  // characters 12-13
				} {
					if err := binary.Write(w, binary.LittleEndian, v); err != nil {
						return err
					}
				}
			}
		}

		// Short directory entry
		var primaryb [8]byte
		copy(primaryb[:], []byte(primary))
		var extb [3]byte
		copy(extb[:], []byte(ext))
		for _, v := range []interface{}{
			primaryb,
			extb,
			entry.Attr(),
			[10]byte{}, // reserved
			entry.Time(),
			entry.Date(),
			entry.FirstCluster(),
			entry.Size(), // file size in bytes
		} {
			if err := binary.Write(w, binary.LittleEndian, v); err != nil {
				return err
			}
		}
	}
	return nil
}

func (fw *Writer) writeDir(d *directory) error {
	oldFat := fw.fat
	offset, err := fw.dataTmp.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	// Write the directory entries to populate d.firstCluster
	if err := fw.writeDir1(d); err != nil {
		return err
	}

	// Reset the FAT to before the write and write the directory entries again,
	// this time with the correct d.firstCluster.
	if _, err := fw.dataTmp.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	fw.fat = oldFat
	if err := fw.writeDir1(d); err != nil {
		return err
	}
	return nil
}

func (fw *Writer) writeDir1(d *directory) error {
	for _, e := range d.entries {
		if e.Attr() != attrDirectory {
			continue
		}
		if err := fw.writeDir1(e.(*directory)); err != nil {
			return err
		}
	}

	d.firstCluster = fw.currentCluster()

	fuw := &fatUpdatingWriter{
		fw: fw,
		pw: &paddingWriter{
			w:     fw.dataTmp,
			padTo: clusterSize,
		},
	}

	if err := fw.writeDirEntries(fuw, d); err != nil {
		return err
	}
	if err := fuw.Close(); err != nil {
		return err
	}

	return nil
}

func fullSectors(bytes int) int {
	sectors := bytes / int(sectorSize)
	if bytes%int(sectorSize) > 0 {
		sectors++
	}
	return sectors
}

func fullClusters(bytes int) int {
	clusters := bytes / clusterSize
	if bytes%clusterSize > 0 {
		clusters++
	}
	return clusters
}

// Flush writes the image. The Writer must not be used after calling
// Flush.
func (fw *Writer) Flush() error {
	if fw.pending != nil {
		if err := fw.pending.Close(); err != nil {
			return err
		}
		fw.pending = nil
	}

	// Write all non-root directory entries recursively
	for _, e := range fw.root.entries {
		if e.Attr() != attrDirectory {
			continue
		}
		if err := fw.writeDir(e.(*directory)); err != nil {
			return err
		}
	}

	// Blow up FAT to at least 4085 usable entries so that 16-bit FAT values
	// must be used, which is more convenient and the only size of FAT
	// values we support.
	if padding := 4085 - fw.usableFATEntries(); padding > 0 {
		pad := make([]uint16, padding)
		fw.fat = append(fw.fat, pad...)
	}

	// TODO: why fullSectors, the FAT is in clusters?!
	fatSectors := fullSectors(len(fw.fat) * 2)

	// We only need to reserve the boot sector, but the number of reserved
	// sectors must be aligned to clusters (at least on the Raspberry Pi 3).
	reservedSectors := fullClusters(1*int(sectorSize)) * int(sectorsPerCluster)

	pw := &paddingWriter{w: fw.w, padTo: clusterSize}
	totalSectors, err := fw.writeBootSector(pw, fatSectors, reservedSectors)
	if err != nil {
		return err
	}
	fw.TotalSectors = totalSectors
	if err := pw.Flush(); err != nil {
		return err
	}

	if err := fw.writeFAT(); err != nil {
		return err
	}

	// root directory
	pw = &paddingWriter{
		w:     fw.w,
		padTo: int(sectorSize),
	}
	if err := fw.writeDirEntries(pw, fw.root); err != nil {
		return err
	}
	if err := pw.Flush(); err != nil {
		return err
	}

	// data area
	if _, err := fw.dataTmp.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if _, err := io.Copy(fw.w, fw.dataTmp); err != nil {
		return err
	}

	fw.dataTmp.Close()
	return os.Remove(fw.dataTmp.Name())
}
