//go:build integration && linux

package imagestore

import (
	"bytes"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

// fileCapabilityBindService is a file capability attribute, revision 2,
// effective, that permits binding a port below 1024 and nothing else: the
// capability a container's server program commonly carries.
var fileCapabilityBindService = []byte{
	0x01, 0x00, 0x00, 0x02, // revision 2, effective
	0x00, 0x04, 0x00, 0x00, // permitted, low word: bit 10, bind service
	0x00, 0x00, 0x00, 0x00, // inheritable, low word
	0x00, 0x00, 0x00, 0x00, // permitted, high word
	0x00, 0x00, 0x00, 0x00, // inheritable, high word
}

// TestApplyXattrFileCapabilityKept proves a file capability reads back
// from the applied file, on a file whose owner is not the applier's, and
// that a user attribute reads back from a file and a folder.
//
// Method: as root in the QEMU guest, apply a file owned by user 1000 with
// the capability, then read the attribute back. Setting the owner clears a
// file capability, so a store that set the attribute before the owner
// would read back none.
func TestApplyXattrFileCapabilityKept(t *testing.T) {
	if unix.Geteuid() != 0 {
		t.Fatalf("this test sets a file capability and another owner, so it runs as root in the QEMU guest, not as user %d", unix.Geteuid())
	}
	store := newAppliedStore(t, testLimits())
	server := fileEntry("server", "binary\n", 0o755)
	server.uid = 1000
	server.gid = 1000
	server.records = map[string]string{
		paxAttributePrefix + attributeFileCapability: string(fileCapabilityBindService),
		paxAttributePrefix + "user.origin":           "image",
	}
	data := folderEntry("data", 0o755)
	data.records = map[string]string{paxAttributePrefix + "user.origin": "folder"}
	store.mustApply(t, server, data)

	read := func(name, attribute string) []byte {
		t.Helper()
		value := make([]byte, 256)
		n, err := unix.Lgetxattr(filepath.Join(store.rootPath, name), attribute, value)
		if err != nil {
			t.Fatalf("%s: read %s: %v", name, attribute, err)
		}
		return value[:n]
	}
	if got := read("server", attributeFileCapability); !bytes.Equal(got, fileCapabilityBindService) {
		t.Errorf("server: capability %x, want %x", got, fileCapabilityBindService)
	}
	if got := read("server", "user.origin"); !bytes.Equal(got, []byte("image")) {
		t.Errorf("server: user.origin %q, want image", got)
	}
	if got := read("data", "user.origin"); !bytes.Equal(got, []byte("folder")) {
		t.Errorf("data: user.origin %q, want folder", got)
	}
}
