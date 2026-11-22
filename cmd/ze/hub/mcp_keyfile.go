// Design: docs/architecture/mcp/overview.md -- the MCP TLS private key file check
// Related: service_mcp.go -- loadMCPTLSConfig, the caller

//go:build ze_mcp

package hub

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// keyPathHopsMax bounds the symlinks one key path may follow. It is the Linux
// kernel's own limit (MAXSYMLINKS), so a link loop is refused rather than
// walked forever, and no path the kernel opens is refused for its length.
const keyPathHopsMax = 40

// keyPathRole names what one path entry is to the key file: a directory the
// path passes through, a symlink it follows, or the file it resolves to.
type keyPathRole uint8

const (
	keyPathRoleUnspecified keyPathRole = iota
	keyPathDirectory
	keyPathLink
	keyPathKey
)

// keyPathEntry is one entry met while resolving a key file path, as Lstat
// reported it. uid is int64 so the stat uid (uint32) and os.Geteuid (int)
// compare without a narrowing conversion.
type keyPathEntry struct {
	path string
	role keyPathRole
	mode fs.FileMode
	uid  int64
}

// checkKeyFilePermissions refuses a private-key file another local user could
// read or replace. It holds for every user, root included: running as root
// does not stop an unprivileged user from reading a group- or world-readable
// key, or from swapping a key that sits behind a path they can write.
//
// A symlink is followed, so a secret mount built from links works: a
// Kubernetes projected secret (tls.key -> ..data/tls.key, ..data ->
// ..<timestamp>) and a link to a systemd credential. Following a link is safe
// only when nobody else can retarget it, so the path is resolved one entry at
// a time, the way the kernel does, and EVERY entry on it is judged, the rule
// OpenSSH's secure-path check applies to an authorized_keys file:
//   - every directory the path passes through, the link's own directory and
//     the target's included, is owned by root or by the user ze runs as, and
//     is not writable by group or other unless it is sticky (a sticky
//     directory such as /tmp or a Kubernetes secret volume lets only an
//     entry's owner replace it, and every entry is owned as below);
//   - every symlink followed is owned by root or by the user ze runs as;
//   - the file the path resolves to is a regular file owned by root or by the
//     user ze runs as, with no group or other permission bit (mode 0600 or
//     tighter).
//
// A link whose target does not exist, a target that is a directory or a
// device, and a chain longer than keyPathHopsMax are refused.
func checkKeyFilePermissions(keyFile string) error {
	return walkKeyPath(keyFile, int64(os.Geteuid()))
}

// walkKeyPath resolves keyFile entry by entry and judges each entry against
// euid. It returns nil only when the path resolves to a key file and every
// entry on the way passed.
func walkKeyPath(keyFile string, euid int64) error {
	start, err := filepath.Abs(keyFile)
	if err != nil {
		return opererr.Wrap(err, "cannot resolve key file "+keyFile+" to an absolute path",
			"Ze checks every directory on the path to a private key before it loads it",
			"set environment mcp tls key to the absolute path of the private key, then restart ze: "+privilege.RestartRemedy())
	}

	root, err := lstatKeyPathEntry("/")
	if err != nil {
		return keyPathUnreadable(keyFile, "/", "", "", err)
	}
	root.role = keyPathDirectory
	if err := judgeKeyPathEntry(keyFile, root, euid); err != nil {
		return err
	}

	pending := keyPathComponents(start)
	current := "/"
	hops := 0
	linkPath, linkTarget := "", ""
	resolved := false
	for len(pending) > 0 {
		name := pending[0]
		pending = pending[1:]
		if name == ".." {
			// current was judged when the walk entered it, and so was each of
			// its ancestors, so stepping back up needs no new judgment.
			current = filepath.Dir(current)
			resolved = false
			continue
		}

		next := filepath.Join(current, name)
		entry, err := lstatKeyPathEntry(next)
		if err != nil {
			return keyPathUnreadable(keyFile, next, linkPath, linkTarget, err)
		}

		if entry.mode&fs.ModeSymlink != 0 {
			entry.role = keyPathLink
			if err := judgeKeyPathEntry(keyFile, entry, euid); err != nil {
				return err
			}
			hops++
			if hops > keyPathHopsMax {
				return opererr.New(
					"key file "+keyFile+" follows more than "+keyPathNumber(keyPathHopsMax)+" symlinks",
					"the links loop, or chain further than the kernel itself follows",
					"ln -sfn /path/to/private.key "+keyFile)
			}
			target, err := os.Readlink(next)
			if err != nil {
				return keyPathUnreadable(keyFile, next, linkPath, linkTarget, err)
			}
			linkPath, linkTarget = next, target
			if filepath.IsAbs(target) {
				current = "/"
			}
			pending = append(keyPathComponents(target), pending...)
			continue
		}

		if len(pending) > 0 {
			entry.role = keyPathDirectory
		} else {
			entry.role = keyPathKey
		}
		if err := judgeKeyPathEntry(keyFile, entry, euid); err != nil {
			return err
		}
		current = next
		resolved = entry.role == keyPathKey
	}

	// refuse-by-default: guards loadMCPTLSConfig -- a path whose last step is
	// "..", or that is "/" itself, names a directory and no key file was judged.
	if !resolved {
		return opererr.New("key file "+keyFile+" resolves to the directory "+current+", not a regular file",
			"a private key must be a plain file that only its owner can read",
			"set environment mcp tls key to the private key file itself, then restart ze: "+privilege.RestartRemedy())
	}
	return nil
}

// lstatKeyPathEntry reports one path entry without following it. The role is
// left for the caller, which alone knows where the entry sits on the path.
func lstatKeyPathEntry(path string) (keyPathEntry, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return keyPathEntry{}, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return keyPathEntry{}, errors.New("the file system reports no owner for " + path)
	}
	return keyPathEntry{path: path, mode: info.Mode(), uid: int64(stat.Uid)}, nil
}

// keyPathComponents splits a path into the names the walk resolves, dropping
// the empty and "." names a slash run or a "./" leaves.
func keyPathComponents(path string) []string {
	names := strings.Split(path, "/")
	components := make([]string, 0, len(names))
	for _, name := range names {
		if name == "" {
			continue
		}
		if name == "." {
			continue
		}
		components = append(components, name)
	}
	return components
}

// judgeKeyPathEntry refuses one entry another local user could use to read or
// replace the key. keyFile is the path the operator configured, named in every
// message so the operator can find the setting.
func judgeKeyPathEntry(keyFile string, entry keyPathEntry, euid int64) error {
	switch entry.role {
	case keyPathDirectory:
		return judgeKeyPathDirectory(keyFile, entry, euid)
	case keyPathLink:
		if !keyPathOwnerTrusted(entry.uid, euid) {
			return opererr.New(
				"symlink "+entry.path+" on the path to key file "+keyFile+" is owned by uid "+keyPathNumber(entry.uid)+
					", not by root or by the user ze runs as (uid "+keyPathNumber(euid)+")",
				"the owner of a link can replace it and point ze at a key of their choosing",
				keyPathOwnerFix(entry.path, euid, true))
		}
		return nil
	case keyPathKey:
		return judgeKeyPathKey(keyFile, entry, euid)
	case keyPathRoleUnspecified:
		panic("BUG: key path entry judged before its role was set")
	}
	panic("BUG: unknown key path role " + keyPathNumber(int64(entry.role)))
}

// judgeKeyPathDirectory refuses a directory on the key path that another user
// could change: one they own, or one they may write without the sticky bit.
func judgeKeyPathDirectory(keyFile string, entry keyPathEntry, euid int64) error {
	if !entry.mode.IsDir() {
		return opererr.New(
			entry.path+" on the path to key file "+keyFile+" is "+keyPathKind(entry.mode)+", not a directory",
			"the path to the private key cannot be resolved",
			"set environment mcp tls key to the private key file itself, then restart ze: "+privilege.RestartRemedy())
	}
	if !keyPathOwnerTrusted(entry.uid, euid) {
		return opererr.New(
			"directory "+entry.path+" on the path to key file "+keyFile+" is owned by uid "+keyPathNumber(entry.uid)+
				", not by root or by the user ze runs as (uid "+keyPathNumber(euid)+")",
			"the owner of a directory can replace the key, or the link to it, inside that directory",
			keyPathOwnerFix(entry.path, euid, false))
	}
	if entry.mode.Perm()&0o022 != 0 {
		if entry.mode&fs.ModeSticky == 0 {
			return opererr.New(
				"directory "+entry.path+" on the path to key file "+keyFile+" is writable by group or other users (mode "+
					keyPathMode(entry.mode)+")",
				"any user who can write that directory can replace the key, or the link to it",
				"chmod go-w "+entry.path)
		}
	}
	return nil
}

// judgeKeyPathKey refuses the resolved key file unless it is a regular file
// that only root or the user ze runs as owns and only its owner can access.
func judgeKeyPathKey(keyFile string, entry keyPathEntry, euid int64) error {
	shown := keyFile
	if entry.path != keyFile {
		shown = keyFile + " (resolved to " + entry.path + ")"
	}
	if !entry.mode.IsRegular() {
		return opererr.New(
			"key file "+shown+" is "+keyPathKind(entry.mode)+", not a regular file",
			"a private key must be a plain file that only its owner can read",
			"set environment mcp tls key to the private key file itself, then restart ze: "+privilege.RestartRemedy())
	}
	if !keyPathOwnerTrusted(entry.uid, euid) {
		return opererr.New(
			"key file "+shown+" is owned by uid "+keyPathNumber(entry.uid)+
				", not by root or by the user ze runs as (uid "+keyPathNumber(euid)+")",
			"the owner of a key file can read it and replace the key",
			keyPathOwnerFix(entry.path, euid, false))
	}
	if entry.mode.Perm()&0o077 != 0 {
		return opererr.New(
			"key file "+shown+" has mode "+keyPathMode(entry.mode)+"; group/other permissions must be cleared",
			"any other local user can read the private key",
			"chmod 600 "+entry.path)
	}
	return nil
}

// keyPathOwnerTrusted reports whether uid may own an entry on the key path:
// root, which ze trusts in any case, or the user ze runs as.
func keyPathOwnerTrusted(uid, euid int64) bool {
	if uid == 0 {
		return true
	}
	return uid == euid
}

// keyPathOwnerFix is the command that hands path to the user ze runs as. A
// link is changed with -h, so chown does not follow it to its target.
func keyPathOwnerFix(path string, euid int64, link bool) string {
	owner := "root"
	if euid != 0 {
		owner = keyPathNumber(euid)
	}
	if link {
		return "sudo chown -h " + owner + " " + path
	}
	return "sudo chown " + owner + " " + path
}

// keyPathNumber formats a uid or a count for a refusal.
func keyPathNumber(v int64) string {
	var tb textbuf.Buffer
	return tb.Int(v).String()
}

// keyPathMode formats the permission bits of mode as four octal digits, the
// form chmod takes (0644).
func keyPathMode(mode fs.FileMode) string {
	perm := uint32(mode.Perm())
	return string([]byte{'0', '0' + byte(perm>>6&7), '0' + byte(perm>>3&7), '0' + byte(perm&7)})
}

// keyPathKind names what a non-regular entry is, for the refusal.
func keyPathKind(mode fs.FileMode) string {
	if mode.IsDir() {
		return "a directory"
	}
	if mode&fs.ModeDevice != 0 {
		return "a device"
	}
	if mode&fs.ModeNamedPipe != 0 {
		return "a named pipe"
	}
	if mode&fs.ModeSocket != 0 {
		return "a socket"
	}
	if mode.IsRegular() {
		return "a regular file"
	}
	return "a special file"
}

// keyPathUnreadable explains an entry the walk could not inspect. A missing
// entry behind a link is a dangling link: the message names the link, its
// target and the entry that is missing.
func keyPathUnreadable(keyFile, path, linkPath, linkTarget string, err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		if linkPath != "" {
			return opererr.Wrap(err,
				"key file "+keyFile+" is a dangling symlink: "+linkPath+" points to "+linkTarget+", and "+path+" does not exist",
				"Ze cannot load a private key from a link with nothing behind it",
				"ln -sfn /path/to/private.key "+linkPath)
		}
		return opererr.Wrap(err,
			"key file "+keyFile+" does not exist ("+path+" is missing)",
			"MCP serves HTTPS only with the private key of its certificate",
			"set environment mcp tls key to the private key of that certificate, then restart ze: "+privilege.RestartRemedy())
	}
	return opererr.Wrap(err,
		"cannot inspect "+path+" on the path to key file "+keyFile,
		"Ze checks every directory and link on the path, so that no other user can swap the key",
		"give the user ze runs as access to "+path+", or set environment mcp tls key to a key file it can read")
}
