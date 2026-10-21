// Design: docs/architecture/testing/qemu-integration.md -- Docker labs on the docker-host image

package testqemu

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

// readCidataISO reads image back the way the guest does: the volume id from
// the primary volume descriptor (dockerhost volumeID reads the same 32
// octets), then the root directory through the descriptor's root record, each
// name translated as Linux's isofs_name_translate does with map=normal.
func readCidataISO(t *testing.T, image []byte) (string, map[string][]byte) {
	t.Helper()
	pvd := image[isoPVDSector*isoSectorOctets:]
	if pvd[0] != 1 || string(pvd[1:6]) != "CD001" {
		t.Fatalf("sector 16 is not a primary volume descriptor: % x", pvd[:7])
	}
	terminator := image[isoTerminatorSector*isoSectorOctets:]
	if terminator[0] != 255 || string(terminator[1:6]) != "CD001" {
		t.Fatalf("sector 17 is not the set terminator: % x", terminator[:7])
	}
	if total := binary.LittleEndian.Uint32(pvd[80:]); int(total)*isoSectorOctets != len(image) {
		t.Fatalf("volume space size %d sectors, image %d bytes", total, len(image))
	}
	label := strings.ToLower(strings.TrimRight(string(pvd[40:72]), " "))
	rootExtent := binary.LittleEndian.Uint32(pvd[156+2:])
	rootSize := binary.LittleEndian.Uint32(pvd[156+10:])
	root := image[int(rootExtent)*isoSectorOctets : int(rootExtent)*isoSectorOctets+int(rootSize)]

	files := map[string][]byte{}
	for offset, index := 0, 0; offset < len(root) && root[offset] != 0; index++ {
		record := root[offset : offset+int(root[offset])]
		offset += len(record)
		if index < 2 {
			continue // "." and ".."
		}
		if binary.LittleEndian.Uint32(record[2:]) != binary.BigEndian.Uint32(record[6:]) {
			t.Fatalf("record % x: extent byte orders differ", record)
		}
		name := strings.ToLower(string(record[33 : 33+int(record[32])]))
		name = strings.TrimSuffix(strings.TrimSuffix(name, ";1"), ".")
		extent := int(binary.LittleEndian.Uint32(record[2:]))
		size := int(binary.LittleEndian.Uint32(record[10:]))
		files[name] = image[extent*isoSectorOctets : extent*isoSectorOctets+size]
	}
	return label, files
}

// VALIDATES: spec D-10 and TestDockerLabWritesCidata's row: the cidata volume
// docker-lab writes carries the label cidata and the file set the guest's
// ParseCidata and BootScript read, each body intact.
// METHOD: write the Lima file set, read it back by the guest's own lookups.
// PREVENTS: a volume the guest's FindCidata skips (wrong label) or whose
// names it reads back changed (user-data as user-data., lima.env as LIMA.ENV).
func TestDockerLabWritesCidata(t *testing.T) {
	guest := dockerLabGuest{
		instanceID: "docker-lab-42", publicKey: "ssh-ed25519 AAAAC3Nz test",
		shares: []dockerLabShare{{tag: "zelab0", point: "/Users/someone/ze"}},
	}
	files := guest.cidataFiles()
	image, err := cidataISO(files)
	if err != nil {
		t.Fatal(err)
	}
	label, read := readCidataISO(t, image)
	if label != "cidata" {
		t.Errorf("volume label %q, want cidata", label)
	}
	for _, name := range []string{"lima.env", "meta-data", "user-data", "boot.sh"} {
		body, ok := read[name]
		if !ok {
			t.Errorf("the volume has no %s; it holds %v", name, keys(read))
			continue
		}
		var want []byte
		for _, file := range files {
			if file.name == name {
				want = file.body
			}
		}
		if !bytes.Equal(body, want) {
			t.Errorf("%s read back as %q, want %q", name, body, want)
		}
	}
	if len(read) != len(files) {
		t.Errorf("the volume holds %v, want exactly the %d files written", keys(read), len(files))
	}
}

// VALIDATES: a name the guest could not read back unchanged is refused.
// PREVENTS: a file the guest's boot looks for under one name recorded under
// another, which the boot reports only as a missing file.
func TestCidataRefusesANameItCannotRecord(t *testing.T) {
	for _, name := range []string{"", "User-Data", "a.b.c", "space name", strings.Repeat("a", cidataNameOctetsMax+1)} {
		if _, err := cidataISO([]cidataFile{{name: name}}); err == nil {
			t.Errorf("cidataISO accepted the name %q", name)
		}
	}
	if _, err := cidataISO([]cidataFile{{name: "big", body: make([]byte, cidataFileOctetsMax+1)}}); err == nil {
		t.Error("cidataISO accepted a file above the guest's bound")
	}
	if _, err := cidataISO([]cidataFile{{name: "max", body: make([]byte, cidataFileOctetsMax)}}); err != nil {
		t.Errorf("cidataISO refused a file at the guest's bound: %v", err)
	}
}

func keys(m map[string][]byte) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	return names
}
