// Design: docs/architecture/appliance/docker-host.md -- the accounts Lima logs in with
// Related: boot.go -- the boot sequence that writes them

package dockerhost

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	// dockerGID is the docker group's id. Colima's containerd config gives
	// its socket to gid 1000, and Colima's provisioning adds the Lima user to
	// a group named docker with `groupadd -f docker && usermod -aG docker`,
	// tools busybox does not carry, so the group exists before Colima asks.
	dockerGID = 1000

	// sshdUID is the account sshd's privilege separation drops to. It owns
	// nothing; OpenSSH needs it to exist (--with-privsep-user=sshd).
	sshdUID = 22
)

// writeAccounts writes /etc/passwd, /etc/group, /etc/shadow, /etc/sudoers,
// the Lima user's sudoers.d entry (the file name and line Lima's
// cloud-init writes, 90-lima-users), and the user's home with its
// authorized_keys. Every account is locked: login is by public key only.
func writeAccounts(sys System, root string, c *Cidata) error {
	uid := strconv.Itoa(int(c.UID))
	primaryGroup, groups := groupLines(c)

	passwd := "root:x:0:0:root:/:/bin/sh\n" +
		"sshd:x:" + strconv.Itoa(sshdUID) + ":" + strconv.Itoa(sshdUID) + ":sshd privilege separation:/var/empty:/bin/false\n" +
		c.User + ":x:" + uid + ":" + primaryGroup + ":" + c.Comment + ":" + c.Home + ":" + c.Shell + "\n"
	shadow := "root:*:1::::::\nsshd:*:1::::::\n" + c.User + ":*:1::::::\n"
	sudoers := "root ALL=(ALL:ALL) ALL\n@includedir /etc/sudoers.d\n"
	limaUsers := c.User + " ALL=(ALL) NOPASSWD:ALL\n"

	files := []struct {
		name string
		data string
		mode os.FileMode
	}{
		{"/etc/passwd", passwd, 0o644},
		{"/etc/group", groups, 0o644},
		{"/etc/shadow", shadow, 0o600},
		{"/etc/sudoers", sudoers, 0o440},
		{"/etc/sudoers.d/90-lima-users", limaUsers, 0o440},
	}
	if err := os.MkdirAll(guest(root, "/etc/sudoers.d"), 0o750); err != nil {
		return fmt.Errorf("create /etc/sudoers.d: %w", err)
	}
	for _, f := range files {
		if err := replaceFile(guest(root, f.name), []byte(f.data), f.mode); err != nil {
			return fmt.Errorf("write %s: %w", f.name, err)
		}
	}
	return writeHome(sys, root, c)
}

// replaceFile writes data to a new file beside name and renames it over
// name, so a file already there with a read-only mode (sudoers is 0440) is
// replaced when gokrazy restarts the program within one boot. The mode is
// set after the write because WriteFile honours the umask, and sudo refuses
// a sudoers file whose mode is not the one it expects.
func replaceFile(name string, data []byte, mode os.FileMode) error {
	next := name + ".ze-new"
	if err := os.WriteFile(next, data, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(next, mode); err != nil {
		return err
	}
	return os.Rename(next, name)
}

// groupLines answers the user's primary gid and the /etc/group content. The
// user's own group has the user's uid as its gid, as useradd makes it, unless
// that uid is the docker gid: then docker is the primary group, since two
// groups cannot share one id.
func groupLines(c *Cidata) (string, string) {
	base := "root:x:0:\nsshd:x:" + strconv.Itoa(sshdUID) + ":\n"
	docker := "docker:x:" + strconv.Itoa(dockerGID) + ":" + c.User + "\n"
	if c.UID == dockerGID {
		return strconv.Itoa(dockerGID), base + docker
	}
	uid := strconv.Itoa(int(c.UID))
	return uid, base + c.User + ":x:" + uid + ":\n" + docker
}

// writeHome creates the home directory and .ssh/authorized_keys, owned by
// the user, with the modes sshd's StrictModes accepts.
func writeHome(sys System, root string, c *Cidata) error {
	gid := int(c.UID)
	if c.UID == dockerGID {
		gid = dockerGID
	}
	sshDir := c.Home + "/.ssh"
	if err := os.MkdirAll(guest(root, sshDir), 0o700); err != nil {
		return fmt.Errorf("create %s: %w", sshDir, err)
	}
	keys := sshDir + "/authorized_keys"
	if err := os.WriteFile(guest(root, keys), []byte(strings.Join(c.AuthorizedKeys, "\n")+"\n"), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", keys, err)
	}
	if err := os.Chmod(guest(root, c.Home), 0o755); err != nil { //nolint:gosec // a home directory is world-traversable, as useradd makes it
		return fmt.Errorf("chmod %s: %w", c.Home, err)
	}
	for _, p := range []string{c.Home, sshDir, keys} {
		if err := sys.Chown(p, int(c.UID), gid); err != nil {
			return fmt.Errorf("chown %s: %w", p, err)
		}
	}
	return nil
}
