package dockerhost

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestCidataParsesLimaFiles verifies the cidata reader takes what the boot
// needs from the files Lima writes: lima.env, meta-data and user-data.
//
// VALIDATES: plan/spec-ze-image-docker-host.md Phase 1, D-3 (cloud-init's
// part done in Go). Method: parse the fixture copied from a Lima 2.2.1 render.
// PREVENTS: a user, key or mount Lima sends being dropped before the boot
// writes passwd, authorized_keys or mounts.
func TestCidataParsesLimaFiles(t *testing.T) {
	c, err := ParseCidata(filepath.Join("testdata", "cidata"))
	if err != nil {
		t.Fatalf("ParseCidata: %v", err)
	}
	if c.InstanceID != "iid-1761040000" {
		t.Errorf("instance id = %q", c.InstanceID)
	}
	if c.Hostname != "lima-ze-probe" {
		t.Errorf("hostname = %q", c.Hostname)
	}
	if c.User != "thomas" || c.UID != 501 || c.Home != "/home/thomas.linux" || c.Shell != "/bin/bash" {
		t.Errorf("user = %q uid %d home %q shell %q", c.User, c.UID, c.Home, c.Shell)
	}
	if c.VsockPort != 2222 {
		t.Errorf("vsock port = %d", c.VsockPort)
	}
	wantKeys := []string{
		"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExample1 thomas@mac",
		"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExample2 lima",
	}
	if !slices.Equal(c.AuthorizedKeys, wantKeys) {
		t.Errorf("keys = %q", c.AuthorizedKeys)
	}
	wantMounts := []Mount{
		{Tag: "vz-rosetta", Point: "/mnt/lima-rosetta", FSType: "virtiofs", Options: "defaults"},
		{Tag: "mount0", Point: "/Users/thomas", FSType: "virtiofs", Options: "rw"},
	}
	if !slices.Equal(c.Mounts, wantMounts) {
		t.Errorf("mounts = %+v", c.Mounts)
	}
}

// TestCidataRefusesUnsafeValues verifies every value that reaches passwd,
// sudoers or a path is checked before use (spec Security Review: input
// validation), with the boundaries of the spec's numeric table.
//
// VALIDATES: LIMA_CIDATA_UID 1-65534, LIMA_CIDATA_VSOCK_PORT 0-4294967295,
// user, hostname and home patterns. Method: one fixture per bad value.
// PREVENTS: a crafted cidata writing a root account or a path outside /home.
func TestCidataRefusesUnsafeValues(t *testing.T) {
	for _, tc := range []struct {
		name, file, from, to string
		ok                   bool
	}{
		{"uid 0 is root", "lima.env", "LIMA_CIDATA_UID=501", "LIMA_CIDATA_UID=0", false},
		{"uid last valid", "lima.env", "LIMA_CIDATA_UID=501", "LIMA_CIDATA_UID=65534", true},
		{"uid above", "lima.env", "LIMA_CIDATA_UID=501", "LIMA_CIDATA_UID=65535", false},
		{"vsock port last valid", "lima.env", "LIMA_CIDATA_VSOCK_PORT=2222", "LIMA_CIDATA_VSOCK_PORT=4294967295", true},
		{"vsock port above", "lima.env", "LIMA_CIDATA_VSOCK_PORT=2222", "LIMA_CIDATA_VSOCK_PORT=4294967296", false},
		{"vsock port none", "lima.env", "LIMA_CIDATA_VSOCK_PORT=2222", "LIMA_CIDATA_VSOCK_PORT=0", true},
		{"user with a colon", "lima.env", "LIMA_CIDATA_USER=thomas", "LIMA_CIDATA_USER=th:omas", false},
		{"user root", "lima.env", "LIMA_CIDATA_USER=thomas", "LIMA_CIDATA_USER=root", false},
		{"home escapes", "lima.env", "LIMA_CIDATA_HOME=/home/thomas.linux", "LIMA_CIDATA_HOME=/home/../etc", false},
		{"home outside /home", "lima.env", "LIMA_CIDATA_HOME=/home/thomas.linux", "LIMA_CIDATA_HOME=/etc", false},
		{"hostname with a slash", "meta-data", "local-hostname: lima-ze-probe", "local-hostname: a/b", false},
		{"key with a newline escape", "user-data", `thomas@mac"`, `thomas@mac\nssh-rsa AAAA evil"`, false},
		{"relative mount point", "user-data", "[mount0, /Users/thomas,", "[mount0, Users/thomas,", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := copyCidata(t)
			path := filepath.Join(dir, tc.file)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), tc.from) {
				t.Fatalf("fixture %s has no %q", tc.file, tc.from)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), tc.from, tc.to, 1)), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err = ParseCidata(dir)
			if tc.ok && err != nil {
				t.Errorf("refused a valid value: %v", err)
			}
			if !tc.ok && err == nil {
				t.Errorf("accepted %q", tc.to)
			}
		})
	}
}

// copyCidata copies the cidata fixture into a temp dir the test can edit.
func copyCidata(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"lima.env", "param.env", "meta-data", "user-data"} {
		data, err := os.ReadFile(filepath.Join("testdata", "cidata", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
