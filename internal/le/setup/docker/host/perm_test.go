package dockerhost

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// writeGPTImage writes a sparse image whose GPT holds four entries, the
// fourth a Linux file system partition from firstLBA to lastLBA.
func writeGPTImage(t *testing.T, firstLBA, lastLBA uint64, permType []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "image.img")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close() //nolint:errcheck // read-only test file
	header := make([]byte, 92)
	copy(header, "EFI PART")
	binary.LittleEndian.PutUint64(header[72:], 2)
	binary.LittleEndian.PutUint32(header[80:], 4)
	binary.LittleEndian.PutUint32(header[84:], 128)
	if _, err := file.WriteAt(header, sectorBytes); err != nil {
		t.Fatal(err)
	}
	entry := make([]byte, 128)
	copy(entry, permType)
	binary.LittleEndian.PutUint64(entry[32:], firstLBA)
	binary.LittleEndian.PutUint64(entry[40:], lastLBA)
	if _, err := file.WriteAt(entry, 2*sectorBytes+3*128); err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(int64(lastLBA+1) * sectorBytes); err != nil {
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

	other := writeGPTImage(t, 2048, 4095, make([]byte, 16))
	if _, err := permPartition(other); err == nil {
		t.Error("permPartition accepted a fourth partition that is not a Linux file system")
	}
}
