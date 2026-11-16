package dockerhost

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/test/gptfixture"
)

// sectorBytes is the logical sector gok writes the GPT with.
const sectorBytes = gptfixture.SectorBytes

// writeGPTImage writes a sparse image with a valid GPT of four partitions,
// the fourth of type permType from firstLBA to lastLBA, and the backup copy
// after it, as gok lays the image out.
func writeGPTImage(t *testing.T, firstLBA, lastLBA uint64, permType []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "image.img")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close() //nolint:errcheck // read-only test file
	layout := gptfixture.Layout{EntryCount: 128, FirstUsable: 34, Partitions: []gptfixture.Partition{
		{Type: gptfixture.TypeEFISystem, First: 40, Last: 47},
		{Type: gptfixture.TypeLinuxRootAMD64, First: 48, Last: 55},
		{Type: gptfixture.TypeLinuxFilesystem, First: 56, Last: 63},
		{First: firstLBA, Last: lastLBA},
	}}
	copy(layout.Partitions[3].Type[:], permType)
	// The last usable sector is lastLBA, then 32 sectors of backup entries
	// and the backup header.
	layout.DiskSectors = lastLBA + 1 + 32 + 1
	if err := file.Truncate(int64(layout.DiskSectors * sectorBytes)); err != nil { //nolint:gosec // a small test image
		t.Fatal(err)
	}
	if err := layout.Write(file); err != nil {
		t.Fatal(err)
	}
	return path
}

// VALIDATES: /perm's offset and size are read from the image's GPT entry 4,
// an unformatted partition is seen as such and an ext4 magic as formatted.
// PREVENTS: formatting at a constant offset that a changed layout moves,
// which would overwrite the root file system.
func TestPermPartitionIsReadFromTheGPT(t *testing.T) {
	image := writeGPTImage(t, 2048, 4095, linuxDataTypeGUID)
	span, err := permPartition(image)
	if err != nil {
		t.Fatalf("permPartition: %v", err)
	}
	if span.offset != 2048*sectorBytes || span.size != 2048*sectorBytes {
		t.Fatalf("span %+v, want offset %d size %d", span, 2048*sectorBytes, 2048*sectorBytes)
	}
	formatted, err := hasExt4(image, span)
	if err != nil || formatted {
		t.Fatalf("hasExt4 on a blank partition: %v %v", formatted, err)
	}
	file, err := os.OpenFile(image, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte{0x53, 0xef}, span.offset+ext4MagicOffset); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if formatted, err := hasExt4(image, span); err != nil || !formatted {
		t.Fatalf("hasExt4 on an ext4 magic: %v %v", formatted, err)
	}

	// A used fourth entry of another type reaches permPartition's own type
	// rule; an all-zero type is an unused entry, so partition 4 is absent.
	other := writeGPTImage(t, 2048, 4095, gptfixture.TypeLinuxRootAMD64[:])
	if _, err := permPartition(other); err == nil || !strings.Contains(err.Error(), "is not a Linux file system partition") {
		t.Errorf("permPartition on a fourth partition that is not a Linux file system = %v, want the type refusal", err)
	}
	unused := writeGPTImage(t, 2048, 4095, make([]byte, 16))
	if _, err := permPartition(unused); err == nil || !strings.Contains(err.Error(), "has no partition 4") {
		t.Errorf("permPartition on an unused fourth entry = %v, want the missing-partition refusal", err)
	}
}
