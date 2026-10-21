package dockerhost

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// recordingSystem is a System that performs nothing privileged: it records
// each mount, hostname and chown so the test can assert the sequence. A tmpfs
// mounted on /etc empties root/etc, as the real mount hides the squashfs /etc.
type recordingSystem struct {
	root     string
	mounts   []MountRequest
	hostname string
	chowns   map[string][2]int
}

func (r *recordingSystem) Mount(m MountRequest) error {
	r.mounts = append(r.mounts, m)
	if m.FSType == fsTmpfs && m.Target == "/etc" {
		etc := filepath.Join(r.root, "etc")
		if err := os.RemoveAll(etc); err != nil {
			return err
		}
		return os.Mkdir(etc, 0o755)
	}
	return nil
}

func (r *recordingSystem) Sethostname(name string) error {
	r.hostname = name
	return nil
}

func (r *recordingSystem) Chown(path string, uid, gid int) error {
	if r.chowns == nil {
		r.chowns = map[string][2]int{}
	}
	r.chowns[path] = [2]int{uid, gid}
	return nil
}

func (r *recordingSystem) FindCidata() (string, error) { return "/dev/vdb", nil }

func (r *recordingSystem) mounted(target string) (MountRequest, bool) {
	for _, m := range r.mounts {
		if m.Target == target {
			return m, true
		}
	}
	return MountRequest{}, false
}

// TestDockerHostBootSequenceOverATempRoot runs the boot over a temp root with
// the cidata fixture already where the mount would put it, and asserts every
// file and request the Lima contract needs.
//
// VALIDATES: spec Phase 1 boot (D-3, R-2, R-3, A-3): passwd, group with the
// docker group, sudoers, authorized_keys, hostname, the fresh /etc tmpfs holding the squashfs /etc entries, the
// /usr/local overlay, the machine id persisted to /perm, the systemd private
// marker, and the boot.sh command with LIMA_CIDATA_MNT.
// PREVENTS: a guest Lima cannot log in to, or a boot.sh that waits forever.
func TestDockerHostBootSequenceOverATempRoot(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"perm", "etc"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// The squashfs /etc gokrazy ships: a file, a symlink, a nested symlink.
	if err := os.WriteFile(filepath.Join(root, "etc", "hosts"), []byte("127.0.0.1 localhost\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/proc/net/pnp", filepath.Join(root, "etc", "resolv.conf")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "etc", "ssl"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/cacerts", filepath.Join(root, "etc", "ssl", "ca-bundle.pem")); err != nil {
		t.Fatal(err)
	}
	cidataDir := filepath.Join(root, "mnt", "lima-cidata")
	if err := os.MkdirAll(cidataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	fixture := copyCidata(t)
	for _, name := range []string{"lima.env", "param.env", "meta-data", "user-data"} {
		data, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cidataDir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// gokrazy init creates /perm/home 0700 before the program runs.
	if err := os.Mkdir(filepath.Join(root, "perm", "home"), 0o700); err != nil {
		t.Fatal(err)
	}

	sys := &recordingSystem{root: root}
	booted, err := Boot(sys, root)
	if err != nil {
		t.Fatalf("Boot: %v", err)
	}

	// /home is a bind of /perm/home: at 0700 no account but root reaches its
	// home, and sshd refuses authorized_keys (the Lima ssh failure).
	home, err := os.Stat(filepath.Join(root, "perm", "home"))
	if err != nil {
		t.Fatal(err)
	}
	if mode := home.Mode().Perm(); mode != 0o755 {
		t.Errorf("/perm/home mode = %o after Boot, want 755", mode)
	}

	etc, ok := sys.mounted("/etc")
	if !ok || etc.FSType != fsTmpfs || !strings.Contains(etc.Data, "size=64M") {
		t.Errorf("/etc not a fresh 64M tmpfs: %+v", etc)
	}
	if data, err := os.ReadFile(filepath.Join(root, "etc", "hosts")); err != nil || string(data) != "127.0.0.1 localhost\n" {
		t.Errorf("etc/hosts not carried over: %q, %v", data, err)
	}
	for link, want := range map[string]string{"resolv.conf": "/proc/net/pnp", "ssl/ca-bundle.pem": "/cacerts"} {
		if got, err := os.Readlink(filepath.Join(root, "etc", link)); err != nil || got != want {
			t.Errorf("etc/%s -> %q, %v; want %q", link, got, err, want)
		}
	}
	cidata, ok := sys.mounted("/mnt/lima-cidata")
	if !ok || cidata.Source != "/dev/vdb" || cidata.FSType != "iso9660" || !cidata.ReadOnly {
		t.Errorf("cidata mount = %+v", cidata)
	}
	binfmt, ok := sys.mounted(binfmtMiscDir)
	if !ok || binfmt.FSType != "binfmt_misc" {
		t.Errorf("binfmt_misc mount = %+v, %v; want binfmt_misc on %s", binfmt, ok, binfmtMiscDir)
	}
	local, ok := sys.mounted("/usr/local")
	if !ok || local.FSType != "overlay" || !strings.Contains(local.Data, "lowerdir=/usr/local") {
		t.Errorf("/usr/local overlay = %+v", local)
	}
	if sys.hostname != "lima-ze-probe" {
		t.Errorf("hostname set to %q", sys.hostname)
	}

	read := func(rel string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		return string(data)
	}
	if !strings.Contains(read("etc/passwd"), "thomas:x:501:501:Thomas Mangin:/home/thomas.linux:/bin/bash\n") {
		t.Errorf("passwd:\n%s", read("etc/passwd"))
	}
	if !strings.Contains(read("etc/group"), "docker:x:1000:thomas\n") {
		t.Errorf("group:\n%s", read("etc/group"))
	}
	if read("etc/sudoers.d/90-lima-users") != "thomas ALL=(ALL) NOPASSWD:ALL\n" {
		t.Errorf("sudoers.d: %q", read("etc/sudoers.d/90-lima-users"))
	}
	if !strings.Contains(read("etc/sudoers"), "@includedir /etc/sudoers.d") {
		t.Errorf("sudoers does not include sudoers.d")
	}
	keys := filepath.Join("home", "thomas.linux", ".ssh", "authorized_keys")
	if !strings.Contains(read(keys), "thomas@mac\n") {
		t.Errorf("authorized_keys: %q", read(keys))
	}
	if got := sys.chowns["/"+filepath.ToSlash(keys)]; got != [2]int{501, 501} {
		t.Errorf("authorized_keys owner = %v", got)
	}
	if read("etc/hostname") != "lima-ze-probe\n" {
		t.Errorf("hostname file %q", read("etc/hostname"))
	}

	machineID := read("perm/machine-id")
	if !regexp.MustCompile(`^[0-9a-f]{32}\n$`).MatchString(machineID) {
		t.Errorf("machine id %q is not 32 lowercase hex", machineID)
	}
	if read("etc/machine-id") != machineID {
		t.Errorf("/etc/machine-id differs from /perm/machine-id")
	}
	if _, err := os.Stat(filepath.Join(root, "run", "user", "501", "systemd", "private")); err != nil {
		t.Errorf("systemd private marker: %v", err)
	}

	cmd := booted.BootScript(t.Context())
	if !slices.Equal(cmd.Args, []string{"/bin/sh", "/mnt/lima-cidata/boot.sh"}) {
		t.Errorf("boot.sh argv = %q", cmd.Args)
	}
	if !slices.Contains(cmd.Env, "LIMA_CIDATA_MNT=/mnt/lima-cidata") {
		t.Errorf("boot.sh env lacks LIMA_CIDATA_MNT: %q", cmd.Env)
	}

	// A second boot keeps the machine id: it is the guest's identity.
	if _, err := Boot(&recordingSystem{}, root); err != nil {
		t.Fatalf("second Boot: %v", err)
	}
	if read("perm/machine-id") != machineID {
		t.Errorf("machine id changed across boots")
	}
}

// TestConsoleBannerNamesTheBootedKernel verifies the first console line the
// guest prints carries the kernel's own banner from /proc/version.
//
// VALIDATES: spec AC-1, "the boot log shows Ze's kernel banner": under vz the
// serial log starts about one second into the boot, after the kernel's
// "Linux version" line, so the program repeats it. Method: a temp root with
// a /proc/version fixture; a root without one is an error naming the path.
// PREVENTS: a serial log that never says which kernel booted.
func TestConsoleBannerNamesTheBootedKernel(t *testing.T) {
	root := t.TempDir()
	if _, err := consoleBanner(root); err == nil || !strings.Contains(err.Error(), "/proc/version") {
		t.Errorf("consoleBanner without /proc/version = %v, want an error naming the path", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "proc"), 0o755); err != nil {
		t.Fatal(err)
	}
	version := "Linux version 7.2.9-ze (root@builder) (gcc 12.2.0) #1 SMP PREEMPT\n"
	if err := os.WriteFile(filepath.Join(root, "proc", "version"), []byte(version), 0o444); err != nil {
		t.Fatal(err)
	}
	got, err := consoleBanner(root)
	if err != nil {
		t.Fatalf("consoleBanner: %v", err)
	}
	if want := "ze-docker-host: kernel " + strings.TrimSpace(version); got != want {
		t.Errorf("consoleBanner = %q, want %q", got, want)
	}
}
