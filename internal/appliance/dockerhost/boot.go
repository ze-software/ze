// Design: docs/architecture/appliance/docker-host.md -- the boot sequence before Lima's boot.sh
// Related: cidata.go -- the parsed cidata this sequence applies
// Related: accounts.go -- passwd, group, sudoers and keys
// Related: supervise.go -- the services the boot starts

package dockerhost

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// MountRequest is one mount the boot asks the System for. The fields are the
// mount(2) arguments in a form a test can compare, with the flags the boot
// uses spelled as booleans.
type MountRequest struct {
	Source   string
	Target   string
	FSType   string
	Data     string
	ReadOnly bool
	Bind     bool
}

// System is what the boot needs from the kernel. The guest passes the Linux
// implementation (system_linux.go); a test passes a recorder.
type System interface {
	// Mount performs one mount request.
	Mount(m MountRequest) error
	// Sethostname sets the kernel host name.
	Sethostname(name string) error
	// Chown sets the owner of a guest path.
	Chown(path string, uid, gid int) error
	// FindCidata answers the block device that holds the volume labeled
	// cidata.
	FindCidata() (string, error)
}

const (
	// etcData is the tmpfs mounted over /etc. gokrazy's /etc is part of the
	// read-only squashfs root (its tmpfs /etc is only for old FAT roots,
	// gokrazy mount.go mountCompat), and Colima and Lima write there:
	// accounts, containerd and docker configs, unit files, ssh keys.
	etcData = "mode=0755,size=64M"

	// fsTmpfs is both the source and the type of every tmpfs mount the guest
	// makes.
	fsTmpfs = "tmpfs"

	// machineIDName is the guest's persistent identity on /perm, which gokrazy
	// init's MachineID reads first.
	machineIDName = "/perm/machine-id"

	// homeStore is the persistent backing of /home.
	homeStore = "/perm/home"

	// bootPath is the PATH boot.sh and the services run with: the busybox
	// applets and the shipped binaries live in these directories.
	bootPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
)

// guest answers the host path of a guest path under root ("/" in the guest).
func guest(root, guestPath string) string {
	return filepath.Join(root, filepath.FromSlash(guestPath))
}

// consoleBanner answers the first line the guest prints on its console: the
// kernel's own banner from /proc/version. Under vz the serial log starts
// about one second into the boot, after the kernel printed "Linux version",
// so without this line the log never names the kernel that booted.
func consoleBanner(root string) (string, error) {
	version, err := os.ReadFile(guest(root, "/proc/version")) //nolint:gosec // fixed path under the guest root
	if err != nil {
		return "", fmt.Errorf("read /proc/version: %w", err)
	}
	return "ze-docker-host: kernel " + strings.TrimSpace(string(version)), nil
}

// Boot runs the boot sequence over root: /etc made writable at a usable
// size, the cidata volume mounted and parsed, the machine id persisted,
// /home and /usr/local made writable, the accounts and keys written, the
// host name set and systemd's per-user marker created. It answers the parsed
// cidata, whose BootScript runs Lima's boot.sh.
//
// Boot MUST run before the supervisor starts sshd: sshd reads the accounts
// and keys Boot writes.
func Boot(sys System, root string) (*Cidata, error) {
	if err := mountWritableEtc(sys, root); err != nil {
		return nil, err
	}

	device, err := sys.FindCidata()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(guest(root, CidataMount), 0o700); err != nil {
		return nil, fmt.Errorf("create %s: %w", CidataMount, err)
	}
	// The options are the ones Lima's own cloud-init boot hook uses
	// (cidata.TEMPLATE.d/user-data): root-only, executable.
	if err := sys.Mount(MountRequest{Source: device, Target: CidataMount, FSType: "iso9660", ReadOnly: true,
		Data: "mode=0700,dmode=0700,overriderockperm,uid=0"}); err != nil {
		return nil, fmt.Errorf("mount cidata %s at %s: %w", device, CidataMount, err)
	}
	c, err := ParseCidata(guest(root, CidataMount))
	if err != nil {
		return nil, err
	}

	if err := persistMachineID(root); err != nil {
		return nil, err
	}
	if err := mountWritableTrees(sys, root); err != nil {
		return nil, err
	}
	if err := writeAccounts(sys, root, c); err != nil {
		return nil, err
	}
	if err := sys.Sethostname(c.Hostname); err != nil {
		return nil, fmt.Errorf("set host name %s: %w", c.Hostname, err)
	}
	if err := os.WriteFile(guest(root, "/etc/hostname"), []byte(c.Hostname+"\n"), 0o644); err != nil { //nolint:gosec // /etc/hostname is world-readable
		return nil, fmt.Errorf("write /etc/hostname: %w", err)
	}
	if err := writeUserRuntime(sys, root, c); err != nil {
		return nil, err
	}
	if err := seedContainerdConfig(root); err != nil {
		return nil, err
	}
	return c, nil
}

// writableOverlays are the read-only squashfs trees the guest writes into,
// each covered by an overlay whose upper layer lives on the /run tmpfs, so
// the writes last for one boot.
var writableOverlays = []struct{ dir, runtime string }{
	// Lima's 25-guestagent-base.sh installs lima-guestagent under
	// /usr/local/bin (spec R-3).
	{dir: "/usr/local", runtime: "/run/ze-docker-host/usr-local"},
	// busybox modprobe (modprobe-small) scans every module under
	// /lib/modules/<release> unless it finds its modules.dep.bb index, which
	// its first call writes there. On the read-only tree each call scanned
	// again: Lima's 00-modprobe.sh makes 21 calls, 64 to 89 s of boot before
	// the guest agent (spec R-1). Busybox ash runs its own modprobe applet
	// in-process, so the index is what makes every caller fast.
	{dir: "/lib/modules", runtime: "/run/ze-docker-host/lib-modules"},
}

// mountWritableTrees backs /home with /perm/home and puts a tmpfs upper
// layer over each writableOverlays tree.
func mountWritableTrees(sys System, root string) error {
	if err := os.MkdirAll(guest(root, homeStore), 0o755); err != nil { //nolint:gosec // every account reads its home through /home
		return fmt.Errorf("create %s: %w", homeStore, err)
	}
	// gokrazy init creates /perm/home 0700 for its programs' own homes
	// (/perm/home/dhcp, /perm/home/randomd), which keeps every account from
	// reaching its home through /home: sshd then cannot read authorized_keys.
	if err := os.Chmod(guest(root, homeStore), 0o755); err != nil { //nolint:gosec // every account reads its home through /home
		return fmt.Errorf("chmod %s: %w", homeStore, err)
	}
	if err := sys.Mount(MountRequest{Source: homeStore, Target: "/home", Bind: true}); err != nil {
		return fmt.Errorf("bind %s on /home: %w", homeStore, err)
	}
	for _, overlay := range writableOverlays {
		upper := overlay.runtime + "/upper"
		work := overlay.runtime + "/work"
		for _, dir := range []string{upper, work} {
			if err := os.MkdirAll(guest(root, dir), 0o755); err != nil { //nolint:gosec // the overlay serves every account
				return fmt.Errorf("create %s: %w", dir, err)
			}
		}
		data := "lowerdir=" + overlay.dir + ",upperdir=" + upper + ",workdir=" + work
		if err := sys.Mount(MountRequest{Source: "overlay", Target: overlay.dir, FSType: "overlay", Data: data}); err != nil {
			return fmt.Errorf("mount the %s overlay: %w", overlay.dir, err)
		}
	}
	return nil
}

// persistMachineID keeps one machine id per guest disk: generated on the
// first boot into /perm/machine-id, and copied to /etc/machine-id on every
// boot. The image carries none, so two guests never share one (spec AC-6).
func persistMachineID(root string) error {
	stored := guest(root, machineIDName)
	id, err := os.ReadFile(stored) //nolint:gosec // fixed path on the guest's own /perm
	if errors.Is(err, fs.ErrNotExist) {
		id, err = newMachineID()
		if err != nil {
			return err
		}
		if err := os.WriteFile(stored, id, 0o444); err != nil { //nolint:gosec // the machine id is world-readable, as systemd's is
			return fmt.Errorf("write %s: %w", machineIDName, err)
		}
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", machineIDName, err)
	}
	// 0644, not 0444: a restarted program rewrites it within the same boot.
	if err := os.WriteFile(guest(root, "/etc/machine-id"), id, 0o644); err != nil { //nolint:gosec // world-readable by design
		return fmt.Errorf("write /etc/machine-id: %w", err)
	}
	return nil
}

// newMachineID answers 32 lowercase hex digits and a newline, the
// machine-id(5) format.
func newMachineID() ([]byte, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, fmt.Errorf("generate a machine id: %w", err)
	}
	return []byte(hex.EncodeToString(raw[:]) + "\n"), nil
}

// writeUserRuntime creates /run/user/<uid> and the systemd/private marker
// boot.sh waits for before it runs provision.user scripts: boot.sh loops
// until the file exists unless /sbin/openrc-run does (spec R-2).
func writeUserRuntime(sys System, root string, c *Cidata) error {
	runtimeDir := "/run/user/" + strconv.Itoa(int(c.UID))
	if err := os.MkdirAll(guest(root, runtimeDir+"/systemd"), 0o700); err != nil {
		return fmt.Errorf("create %s: %w", runtimeDir, err)
	}
	marker := runtimeDir + "/systemd/private"
	if err := os.WriteFile(guest(root, marker), nil, 0o600); err != nil {
		return fmt.Errorf("create %s: %w", marker, err)
	}
	for _, p := range []string{runtimeDir, runtimeDir + "/systemd", marker} {
		if err := sys.Chown(p, int(c.UID), int(c.UID)); err != nil {
			return fmt.Errorf("chown %s: %w", p, err)
		}
	}
	return nil
}

// BootScript answers Lima's boot.sh run the way Lima's cloud-init hook runs
// it: from the cidata mount, with LIMA_CIDATA_MNT exported. A method of the
// parsed cidata, so it cannot run before Boot mounted and checked the
// volume. boot.sh writes /run/lima-boot-done itself; this program never does.
func (c *Cidata) BootScript(ctx context.Context) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "/bin/sh", CidataMount+"/boot.sh")
	cmd.Dir = "/"
	cmd.Env = []string{
		"LIMA_CIDATA_MNT=" + CidataMount,
		"PATH=" + bootPath,
		"HOME=/",
		"TERM=dumb",
	}
	return cmd
}
