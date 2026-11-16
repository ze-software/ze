// Design: docs/architecture/appliance/on-device-installer.md -- the shared GPT partition-table leaf

package gptdisk

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gokrazy/tools/packer"

	"github.com/ze-software/ze/internal/test/gptfixture"
)

// errStopped is what stoppingDisk answers once its write budget is spent.
var errStopped = errors.New("the disk stopped accepting writes")

// memDisk is a disk in memory.
type memDisk []byte

func (disk memDisk) ReadAt(data []byte, offset int64) (int, error) {
	if offset < 0 || offset+int64(len(data)) > int64(len(disk)) {
		return 0, io.EOF
	}
	return copy(data, disk[offset:]), nil
}

func (disk memDisk) WriteAt(data []byte, offset int64) (int, error) {
	if offset < 0 || offset+int64(len(data)) > int64(len(disk)) {
		return 0, io.ErrShortWrite
	}
	return copy(disk[offset:], data), nil
}

func (memDisk) Sync() error { return nil }

// stoppingDisk writes one sector at a time and stops after budget sectors,
// so a test can cut the table rewrite at any sector, inside a write too. A
// negative budget never stops. It counts the sectors it wrote.
type stoppingDisk struct {
	inner interface {
		io.ReaderAt
		io.WriterAt
		Sync() error
	}
	budget  int
	written int
}

func (disk *stoppingDisk) ReadAt(data []byte, offset int64) (int, error) {
	return disk.inner.ReadAt(data, offset)
}

func (disk *stoppingDisk) WriteAt(data []byte, offset int64) (int, error) {
	for done := 0; done < len(data); done += SectorBytes {
		if disk.budget >= 0 && disk.written >= disk.budget {
			return done, errStopped
		}
		end := min(done+SectorBytes, len(data))
		if _, err := disk.inner.WriteAt(data[done:end], offset+int64(done)); err != nil {
			return done, err
		}
		disk.written++
	}
	return len(data), nil
}

func (disk *stoppingDisk) Sync() error {
	if disk.budget >= 0 && disk.written >= disk.budget {
		return errStopped
	}
	return disk.inner.Sync()
}

// grownImage answers the layout's image followed by extraSectors of zeros: an
// image written raw to a larger disk.
func grownImage(layout gptfixture.Layout, extraSectors uint64) memDisk {
	image := layout.Image()
	return append(memDisk(image), make([]byte, extraSectors*SectorBytes)...)
}

// firmwareView reads a disk the way UEFI firmware does: the primary table,
// and when it is invalid the backup at the disk's last sector. It answers
// which copy it read.
func firmwareView(disk io.ReaderAt, diskBytes uint64) (Table, string, error) {
	table, err := Parse(disk, diskBytes)
	if err == nil {
		return table, "primary", nil
	}
	backup, backupErr := parseAt(disk, diskBytes, diskBytes/SectorBytes-1)
	if backupErr != nil {
		return Table{}, "", errors.Join(err, backupErr)
	}
	return backup, "backup", nil
}

// reseal recomputes the primary header's checksum after a test changed a
// header field, so the parse reaches the check of that field.
func reseal(disk memDisk) {
	header := disk[SectorBytes : 2*SectorBytes]
	binary.LittleEndian.PutUint32(header[16:20], headerChecksum(header))
}

// VALIDATES: the table the gokrazy packer writes parses into its four
// partitions at the packer's bounds; a fixture of the same layout parses;
// and each broken property is refused with its reason (spec
// TestParseValidatesHeaderAndEntries).
// PREVENTS: a reader that accepts what the kernel would not read, or that
// trusts an offset or a count from a disk before bounding it.
func TestParseValidatesHeaderAndEntries(t *testing.T) {
	t.Run("packer", func(t *testing.T) {
		const diskBytes = 2 << 30
		path := filepath.Join(t.TempDir(), "packer.img")
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close() //nolint:errcheck // a test file
		pack := packer.Pack{UseGPT: true, UseGPTPartuuid: true, Partuuid: 0x2a, FirstPartitionOffsetSectors: 8192}
		if err := pack.Partition(file, diskBytes); err != nil {
			t.Fatal(err)
		}
		table, err := Parse(file, diskBytes)
		if err != nil {
			t.Fatalf("Parse refused the packer's table: %v", err)
		}
		const last = diskBytes/SectorBytes - 1
		if table.AlternateLBA != last || table.LastUsableLBA != last-33 || table.FirstUsableLBA != 34 {
			t.Fatalf("header bounds: alternate %d first usable %d last usable %d", table.AlternateLBA, table.FirstUsableLBA, table.LastUsableLBA)
		}
		// writeGPT: 100 MB boot from the first partition offset, two 500 MB
		// roots, then the persistent partition up to permSize.
		const mb = 1 << 20 / SectorBytes
		wants := [4][2]uint64{
			{8192, 8192 + 100*mb - 1},
			{8192 + 100*mb, 8192 + 600*mb - 1},
			{8192 + 600*mb, 8192 + 1100*mb - 1},
			{8192 + 1100*mb, last - 34},
		}
		if len(table.Entries) != 4 {
			t.Fatalf("%d used entries, want 4", len(table.Entries))
		}
		for index, want := range wants {
			entry, ok := table.Partition(index + 1)
			if !ok || entry.FirstLBA != want[0] || entry.LastLBA != want[1] {
				t.Errorf("partition %d = %+v, want sectors %d..%d", index+1, entry, want[0], want[1])
			}
		}
	})

	t.Run("fixture", func(t *testing.T) {
		layout := gptfixture.Appliance(1024)
		image := memDisk(layout.Image())
		table, err := Parse(image, uint64(len(image)))
		if err != nil {
			t.Fatal(err)
		}
		for index, partition := range layout.Partitions {
			entry, ok := table.Partition(index + 1)
			if !ok || entry.FirstLBA != partition.First || entry.LastLBA != partition.Last || entry.Type != partition.Type || entry.GUID != partition.GUID {
				t.Errorf("partition %d = %+v, want %+v", index+1, entry, partition)
			}
		}
	})

	for _, test := range []struct {
		name   string
		disk   func() memDisk
		size   func(memDisk) uint64
		reason string
	}{
		{name: "no MBR signature", reason: "no MBR signature", disk: func() memDisk {
			disk := memDisk(gptfixture.Appliance(1024).Image())
			disk[510] = 0
			return disk
		}},
		{name: "no protective record", reason: "no protective 0xEE record", disk: func() memDisk {
			disk := memDisk(gptfixture.Appliance(1024).Image())
			disk[462+4] = 0x83
			return disk
		}},
		{name: "signature", reason: "no GPT header signature", disk: func() memDisk {
			disk := memDisk(gptfixture.Appliance(1024).Image())
			disk[SectorBytes] ^= 1
			return disk
		}},
		{name: "header checksum", reason: "header checksum does not match", disk: func() memDisk {
			disk := memDisk(gptfixture.Appliance(1024).Image())
			disk[SectorBytes+48] ^= 1
			return disk
		}},
		{name: "entry checksum", reason: "entry array checksum does not match", disk: func() memDisk {
			disk := memDisk(gptfixture.Appliance(1024).Image())
			disk[2*SectorBytes+3*entryBytes+40] ^= 1
			return disk
		}},
		{name: "entry size", reason: "only 128-byte entries", disk: func() memDisk {
			disk := memDisk(gptfixture.Appliance(1024).Image())
			binary.LittleEndian.PutUint32(disk[SectorBytes+84:], 256)
			reseal(disk)
			return disk
		}},
		{name: "too many entries", reason: "above 128", disk: func() memDisk {
			disk := memDisk(gptfixture.Appliance(1024).Image())
			binary.LittleEndian.PutUint32(disk[SectorBytes+80:], 129)
			reseal(disk)
			return disk
		}},
		{name: "no entry", reason: "declares no entry", disk: func() memDisk {
			disk := memDisk(gptfixture.Appliance(1024).Image())
			binary.LittleEndian.PutUint32(disk[SectorBytes+80:], 0)
			reseal(disk)
			return disk
		}},
		{name: "reversed entry", reason: "partition 2 ends at sector", disk: func() memDisk {
			layout := gptfixture.Appliance(1024)
			layout.Partitions[1].First, layout.Partitions[1].Last = layout.Partitions[1].Last, layout.Partitions[1].First
			return memDisk(layout.Image())
		}},
		{name: "overlap", reason: "partitions 2 and 3 overlap", disk: func() memDisk {
			layout := gptfixture.Appliance(1024)
			layout.Partitions[2].First = layout.Partitions[1].Last
			return memDisk(layout.Image())
		}},
		{name: "inside the table", reason: "inside the table's own sectors", disk: func() memDisk {
			layout := gptfixture.Appliance(1024)
			layout.Partitions[0].First = 33
			return memDisk(layout.Image())
		}},
		{name: "past the last usable sector", reason: "after the last usable sector", disk: func() memDisk {
			layout := gptfixture.Appliance(1024)
			layout.Partitions[3].Last = layout.LastUsable() + 1
			return memDisk(layout.Image())
		}},
		{name: "disk one sector smaller than the table", reason: "larger than this disk", disk: func() memDisk {
			return memDisk(gptfixture.Appliance(1024).Image())
		}, size: func(disk memDisk) uint64 { return uint64(len(disk)) - SectorBytes }},
		{name: "not whole sectors", reason: "not a whole number of 512-byte sectors", disk: func() memDisk {
			return memDisk(gptfixture.Appliance(1024).Image())
		}, size: func(disk memDisk) uint64 { return uint64(len(disk)) - 1 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			disk := test.disk()
			size := uint64(len(disk))
			if test.size != nil {
				size = test.size(disk)
			}
			_, err := Parse(disk, size)
			if !errors.Is(err, ErrInvalidTable) {
				t.Fatalf("Parse = %v, want ErrInvalidTable", err)
			}
			if !strings.Contains(err.Error(), test.reason) {
				t.Fatalf("Parse = %q, want the reason %q", err, test.reason)
			}
		})
	}
}
