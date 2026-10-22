// Design: docs/architecture/appliance/docker-host.md -- the guest program's entry point and its Linux calls
// Related: boot.go -- the boot sequence it runs
// Related: supervise.go -- the services it keeps running

package dockerhost

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	// shippedDir holds the static third-party binaries the image carries
	// (busybox, OpenSSH, sudo), placed by the image's extra files.
	shippedDir = "/usr/lib/ze-docker-host"
	// selfPath is where gokrazy installs this program; the systemctl shim is
	// a link to it.
	selfPath = "/user/ze-docker-host"
	// bootedMarker records that this boot already ran the sequence, so a
	// restart by gokrazy does not mount or provision twice.
	bootedMarker = "/run/ze-docker-host/booted"
	// sshHostKey is the sshd host key, kept on /perm so Lima's known_hosts
	// entry survives a restart.
	sshHostKey = "/perm/ze-docker-host/ssh/ssh_host_ed25519_key"
	// sshdConfig allows what Lima uses (public keys, exec, forwarding to a
	// unix socket for the Docker socket) and nothing else.
	sshdConfig = "Port 22\n" +
		"HostKey " + sshHostKey + "\n" +
		"PermitRootLogin no\n" +
		"PasswordAuthentication no\n" +
		"KbdInteractiveAuthentication no\n" +
		"PubkeyAuthentication yes\n" +
		"AuthorizedKeysFile .ssh/authorized_keys\n" +
		"AllowTcpForwarding yes\n" +
		"AllowStreamLocalForwarding yes\n" +
		"StreamLocalBindUnlink yes\n" +
		"Subsystem sftp " + shippedDir + "/sftp-server\n" +
		"PidFile /run/sshd.pid\n"
)

// shippedLinks are the links to the shipped binaries, beside the busybox
// applets.
var shippedLinks = map[string]string{
	"/usr/bin/sudo":       shippedDir + "/sudo",
	"/usr/sbin/sshd":      shippedDir + "/sshd",
	"/usr/bin/ssh-keygen": shippedDir + "/ssh-keygen",
	"/usr/bin/systemctl":  selfPath,
	// Docker's static release, unpacked under shippedDir/docker: dockerd
	// finds containerd, the shim, runc, docker-proxy and docker-init on PATH.
	"/usr/bin/dockerd":                 shippedDir + "/docker/dockerd",
	"/usr/bin/containerd":              shippedDir + "/docker/containerd",
	"/usr/bin/containerd-shim-runc-v2": shippedDir + "/docker/containerd-shim-runc-v2",
	"/usr/bin/runc":                    shippedDir + "/docker/runc",
	"/usr/bin/ctr":                     shippedDir + "/docker/ctr",
	"/usr/bin/docker":                  shippedDir + "/docker/docker",
	"/usr/bin/docker-proxy":            shippedDir + "/docker/docker-proxy",
	"/usr/bin/docker-init":             shippedDir + "/docker/docker-init",
	// Static legacy iptables, dockerd's firewall backend (spec A-5): the
	// static dockerd's nftables backend execs nft, which is not shipped.
	"/usr/sbin/iptables":          shippedDir + "/xtables-legacy-multi",
	"/usr/sbin/iptables-save":     shippedDir + "/xtables-legacy-multi",
	"/usr/sbin/iptables-restore":  shippedDir + "/xtables-legacy-multi",
	"/usr/sbin/ip6tables":         shippedDir + "/xtables-legacy-multi",
	"/usr/sbin/ip6tables-save":    shippedDir + "/xtables-legacy-multi",
	"/usr/sbin/ip6tables-restore": shippedDir + "/xtables-legacy-multi",
}

// exitDoNotRestart is the exit status gokrazy's supervisor reads as "do not
// restart this program" (gokrazy supervise.go).
const exitDoNotRestart = 125

// RunGuest is the program gokrazy starts: the boot sequence, the shipped
// binaries made reachable, sshd and the supervisor started, then Lima's
// boot.sh. It returns only on a failure before the supervisor serves; once
// it serves, it runs for the life of the guest.
//
// A failure before the boot marker is written answers exitDoNotRestart:
// the sequence is half applied (cidata already mounted, /etc already
// replaced), so a restart would fail on its first step and hide the real
// error under a loop of different ones.
func RunGuest() int {
	console := openConsole()
	banner, err := consoleBanner("/")
	if err != nil {
		banner = "ze-docker-host: kernel unknown: " + err.Error()
	}
	_, _ = fmt.Fprintln(console, banner) //nolint:errcheck // console
	// A restart after the boot sequence ran is refused, not served
	// (refuseRestart says why).
	if err := refuseRestart(bootedMarker); err != nil {
		_, _ = fmt.Fprintln(console, "ze-docker-host:", err) //nolint:errcheck // console
		return exitDoNotRestart
	}
	c, err := Boot(linuxSystem{}, "/")
	if err != nil {
		_, _ = fmt.Fprintln(console, "ze-docker-host: boot failed:", err) //nolint:errcheck // console
		return exitDoNotRestart
	}
	if err := installBinaries(); err != nil {
		_, _ = fmt.Fprintln(console, "ze-docker-host: install binaries:", err) //nolint:errcheck // console
		return exitDoNotRestart
	}
	if err := prepareSSHD(); err != nil {
		_, _ = fmt.Fprintln(console, "ze-docker-host: prepare sshd:", err) //nolint:errcheck // console
		return exitDoNotRestart
	}
	if err := os.WriteFile(bootedMarker, nil, 0o600); err != nil {
		_, _ = fmt.Fprintln(console, "ze-docker-host: write", bootedMarker, err) //nolint:errcheck // console
		return exitDoNotRestart
	}
	return serveForever(console, c)
}

// serveForever serves the supervisor, starts sshd, runs boot.sh, and then
// never returns: the supervisor lives as long as the guest.
func serveForever(console io.Writer, c *Cidata) int {
	sup, err := NewSupervisor("/", 0, append([]BuiltinUnit{FixedUnit(UnitSpec{
		Name:      "sshd.service",
		Argv:      []string{shippedDir + "/sshd", "-D", "-e", "-f", "/etc/ssh/sshd_config"},
		Restart:   RestartAlways,
		StopGrace: stopGraceDefault,
	})}, DockerUnits()...))
	if err != nil {
		_, _ = fmt.Fprintln(console, "ze-docker-host: supervisor:", err) //nolint:errcheck // console
		return 1
	}
	sup.SetLog(console)
	if _, err := sup.Serve(ShimSocket); err != nil {
		_, _ = fmt.Fprintln(console, "ze-docker-host: serve", ShimSocket, err) //nolint:errcheck // console
		return 1
	}
	if err := sup.Start("sshd.service"); err != nil {
		_, _ = fmt.Fprintln(console, "ze-docker-host: start sshd:", err) //nolint:errcheck // console
	}
	cmd := c.BootScript(context.Background())
	cmd.Stdout = console
	cmd.Stderr = console
	if err := cmd.Run(); err != nil {
		_, _ = fmt.Fprintln(console, "ze-docker-host: Lima boot.sh:", err) //nolint:errcheck // console
	}
	// This loop has no end by design: the supervisor and its socket serve
	// for the life of the guest, and gokrazy would restart an exited program.
	select {}
}

// openConsole answers a writer to the kernel console and stdout: the
// console is what the Lima serial log and a QEMU serial capture, stdout is
// gokrazy's log ring.
func openConsole() io.Writer {
	console, err := os.OpenFile("/dev/console", os.O_WRONLY, 0)
	if err != nil {
		return os.Stdout
	}
	return io.MultiWriter(os.Stdout, console)
}

// installBinaries covers each binDirs entry with a tmpfs and links busybox's
// applets and the shipped binaries into it.
func installBinaries() error {
	sys := linuxSystem{}
	for _, dir := range binDirs {
		if err := sys.Mount(MountRequest{Source: fsTmpfs, Target: dir, FSType: fsTmpfs, Data: "mode=0755,size=1M"}); err != nil {
			return fmt.Errorf("mount tmpfs on %s: %w", dir, err)
		}
	}
	busybox := shippedDir + "/busybox"
	out, err := exec.Command(busybox, "--list-full").Output() //nolint:noctx // a local listing that ends on its own
	if err != nil {
		return fmt.Errorf("%s --list-full: %w", busybox, err)
	}
	for _, link := range appletLinks(string(out)) {
		if err := os.Symlink(busybox, link); err != nil && !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("link %s: %w", link, err)
		}
	}
	for link, target := range shippedLinks {
		_ = os.Remove(link) //nolint:errcheck // a busybox applet of the same name gives way
		if err := os.Symlink(target, link); err != nil {
			return fmt.Errorf("link %s: %w", link, err)
		}
	}
	// Lima's scripts run as /bin/bash; busybox has no bash applet, so bash is
	// busybox ash under that name, as alpine-lima does.
	if err := os.WriteFile("/bin/bash", []byte("#!/bin/sh\nexec /bin/sh \"$@\"\n"), 0o755); err != nil { //nolint:gosec // a shell wrapper is executable by everyone
		return fmt.Errorf("write /bin/bash: %w", err)
	}
	return nil
}

// prepareSSHD writes sshd_config, creates the privilege separation
// directory and generates the host key on the first boot.
func prepareSSHD() error {
	if err := os.MkdirAll("/etc/ssh", 0o755); err != nil { //nolint:gosec // sshd reads its config as any user it serves
		return fmt.Errorf("create /etc/ssh: %w", err)
	}
	if err := os.WriteFile("/etc/ssh/sshd_config", []byte(sshdConfig), 0o644); err != nil { //nolint:gosec // sshd_config is world-readable
		return fmt.Errorf("write sshd_config: %w", err)
	}
	if err := os.MkdirAll("/var/empty", 0o755); err != nil { //nolint:gosec // sshd requires its privilege separation dir root-owned and not group or world writable, 0755 is its documented mode
		return fmt.Errorf("create /var/empty: %w", err)
	}
	if _, err := os.Stat(sshHostKey); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(sshHostKey), 0o700); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(sshHostKey), err)
	}
	keygen := exec.Command(shippedDir+"/ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", sshHostKey) //nolint:noctx // a local key generation that ends on its own
	if out, err := keygen.CombinedOutput(); err != nil {
		return fmt.Errorf("ssh-keygen: %w: %s", err, out)
	}
	return nil
}

// linuxSystem is the System the guest runs with.
type linuxSystem struct{}

func (linuxSystem) Mount(m MountRequest) error {
	var flags uintptr
	if m.ReadOnly {
		flags |= unix.MS_RDONLY
	}
	if m.Bind {
		flags |= unix.MS_BIND
	}
	if m.Remount {
		flags |= unix.MS_REMOUNT
	}
	if err := os.MkdirAll(m.Target, 0o755); err != nil { //nolint:gosec // a mount point is traversed by every account
		return err
	}
	return unix.Mount(m.Source, m.Target, m.FSType, flags, m.Data)
}

func (linuxSystem) Sethostname(name string) error {
	return unix.Sethostname([]byte(name))
}

func (linuxSystem) Chown(path string, uid, gid int) error {
	return os.Lchown(path, uid, gid)
}

// cidataLabelOffset is where an ISO 9660 primary volume descriptor's volume
// identifier sits: sector 16 (16 * 2048), plus 40 bytes into the
// descriptor (ECMA-119 section 8.4.6). The field is 32 bytes, space padded.
const cidataLabelOffset = 16*2048 + 40

// FindCidata answers the virtio block device whose ISO 9660 volume id is
// cidata. There is no udev in the guest, so /dev/disk/by-label does not
// exist; the label is read from each disk.
func (linuxSystem) FindCidata() (string, error) {
	devices, err := filepath.Glob("/dev/vd[a-z]")
	if err != nil {
		return "", err
	}
	for _, device := range devices {
		if volumeID(device) == "cidata" {
			return device, nil
		}
	}
	return "", fmt.Errorf("no disk among %v carries the ISO 9660 label cidata: Lima attaches its cidata volume as the last virtio disk", devices)
}

// volumeID answers a device's ISO 9660 volume identifier, lower-cased (an
// hdiutil-made volume says CIDATA), or empty when it has none.
func volumeID(device string) string {
	f, err := os.Open(device) //nolint:gosec // a /dev/vd* block device
	if err != nil {
		return ""
	}
	defer f.Close() //nolint:errcheck // read-only
	var id [32]byte
	if _, err := f.ReadAt(id[:], cidataLabelOffset); err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimRight(string(id[:]), " \x00"))
}
