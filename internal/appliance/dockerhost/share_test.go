package dockerhost

import (
	"os"
	"path/filepath"
	"testing"
)

// VALIDATES: the shares Lima writes in user-data (Colima's home over virtiofs
// with "rw,nofail", Rosetta with "defaults") are mounted at the host's own
// paths with the fstab-only words dropped, after /mnt and /Users are covered
// by a tmpfs so their mount points can be created on the read-only root.
// PREVENTS: a lab bind mount of a host path failing with "mkdir /Users:
// read-only file system" because the home share was never mounted (spec R-7).
func TestShareMountsFollowLimaUserData(t *testing.T) {
	root := t.TempDir()
	sys := &recordingSystem{root: root}
	if err := mountShareRoots(sys); err != nil {
		t.Fatal(err)
	}
	c := &Cidata{Mounts: []Mount{
		{Tag: "vz-rosetta", Point: "/mnt/lima-rosetta", FSType: "virtiofs", Options: "defaults"},
		{Tag: "lima-84e0", Point: "/Users/thomas", FSType: "virtiofs", Options: "rw,nofail"},
		{Tag: "data", Point: "/home/x/ro", FSType: "9p", Options: "ro,trans=virtio,comment=cloudconfig"},
	}}
	if err := mountShares(sys, root, c); err != nil {
		t.Fatalf("mountShares: %v", err)
	}
	want := []MountRequest{
		{Source: fsTmpfs, Target: "/mnt", FSType: fsTmpfs, Data: "mode=0755"},
		{Source: fsTmpfs, Target: "/Users", FSType: fsTmpfs, Data: "mode=0755"},
		{Source: "vz-rosetta", Target: "/mnt/lima-rosetta", FSType: "virtiofs"},
		{Source: "lima-84e0", Target: "/Users/thomas", FSType: "virtiofs"},
		{Source: "data", Target: "/home/x/ro", FSType: "9p", Data: "trans=virtio", ReadOnly: true},
	}
	if len(sys.mounts) != len(want) {
		t.Fatalf("mounts %+v, want %+v", sys.mounts, want)
	}
	for i := range want {
		if sys.mounts[i] != want[i] {
			t.Errorf("mount %d is %+v, want %+v", i, sys.mounts[i], want[i])
		}
	}
	for _, point := range []string{"mnt/lima-rosetta", "Users/thomas", "home/x/ro"} {
		if info, err := os.Stat(filepath.Join(root, point)); err != nil || !info.IsDir() {
			t.Errorf("mount point %s not created: %v", point, err)
		}
	}
}
