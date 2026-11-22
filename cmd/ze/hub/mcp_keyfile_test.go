// Design: docs/architecture/mcp/overview.md -- the MCP TLS private key file check
// Related: mcp_keyfile.go -- checkKeyFilePermissions, the check these tests drive

//go:build ze_mcp

package hub

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/privilege"
)

// writeKeyForTest writes a key file at path with exactly mode, whatever the
// umask.
func writeKeyForTest(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte("dummy"), 0o600))
	require.NoError(t, os.Chmod(path, mode))
}

// requireKeyRefusal asserts err is an operator error whose problem names every
// string in problem, whose why is set, and whose fix is the command fix.
func requireKeyRefusal(t *testing.T, err error, fix string, problem ...string) {
	t.Helper()
	require.Error(t, err)
	refusal, ok := errors.AsType[*opererr.Error](err)
	require.True(t, ok, "the refusal must be an operator error (what, why, fix), got %T: %v", err, err)
	for _, want := range problem {
		assert.Contains(t, refusal.Problem(), want, "what is wrong")
	}
	assert.NotEmpty(t, refusal.Why(), "why it matters")
	assert.Equal(t, fix, refusal.Fix(), "the command that fixes it")
}

// VALIDATES: a regular key file of mode 0600 passes.
// PREVENTS: the path walk refusing the plain layout every install uses.
func TestCheckKeyFilePathAcceptsRegularFile(t *testing.T) {
	key := filepath.Join(t.TempDir(), "tls.key")
	writeKeyForTest(t, key, 0o600)
	require.NoError(t, checkKeyFilePermissions(key))
}

// VALIDATES: a symlink to a correct key file (regular, 0600, owned by the user
// ze runs as, in a directory only that user writes) passes.
// PREVENTS: the old refusal of every symlink, which broke secret mounts built
// from links.
func TestCheckKeyFilePathAcceptsSymlinkToCorrectFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.key")
	writeKeyForTest(t, target, 0o600)
	link := filepath.Join(dir, "tls.key")
	require.NoError(t, os.Symlink("real.key", link))
	require.NoError(t, checkKeyFilePermissions(link))
}

// VALIDATES: the Kubernetes projected-secret layout passes: a sticky,
// world-writable volume directory holding tls.key -> ..data/tls.key and
// ..data -> a timestamped directory that holds the file.
// PREVENTS: the directory rule refusing the sticky volume directory, which
// only lets an entry's owner replace the entry.
func TestCheckKeyFilePathAcceptsProjectedSecretLayout(t *testing.T) {
	volume := filepath.Join(t.TempDir(), "secret")
	require.NoError(t, os.Mkdir(volume, 0o700))
	stamped := filepath.Join(volume, "..2026_11_22_06_38_17.000000001")
	require.NoError(t, os.Mkdir(stamped, 0o755))
	writeKeyForTest(t, filepath.Join(stamped, "tls.key"), 0o400)
	require.NoError(t, os.Symlink(filepath.Base(stamped), filepath.Join(volume, "..data")))
	link := filepath.Join(volume, "tls.key")
	require.NoError(t, os.Symlink("..data/tls.key", link))
	require.NoError(t, os.Chmod(volume, 0o777|os.ModeSticky))

	require.NoError(t, checkKeyFilePermissions(link))
}

// VALIDATES: a symlink to a world-readable key file is refused, naming the
// link and its target, with chmod on the target as the fix.
// PREVENTS: following a link turning into a bypass of the mode rule.
func TestCheckKeyFilePathRefusesSymlinkToWorldReadableFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "loose.key")
	writeKeyForTest(t, target, 0o644)
	link := filepath.Join(dir, "tls.key")
	require.NoError(t, os.Symlink(target, link))

	requireKeyRefusal(t, checkKeyFilePermissions(link), "chmod 600 "+target,
		link, target, "0644", "group/other permissions")
}

// VALIDATES: a symlink to a directory is refused as not a regular file.
// PREVENTS: a link to a directory reaching tls.LoadX509KeyPair.
func TestCheckKeyFilePathRefusesSymlinkToDirectory(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "keys")
	require.NoError(t, os.Mkdir(target, 0o700))
	link := filepath.Join(dir, "tls.key")
	require.NoError(t, os.Symlink(target, link))

	err := checkKeyFilePermissions(link)
	requireKeyRefusal(t, err,
		"set environment mcp tls key to the private key file itself, then restart ze: "+privilege.RestartRemedy(),
		link, target, "a directory", "not a regular file")
}

// VALIDATES: a symlink to a character device is refused as not a regular file.
// PREVENTS: a link to a device reaching tls.LoadX509KeyPair.
func TestCheckKeyFilePathRefusesSymlinkToDevice(t *testing.T) {
	info, err := os.Lstat("/dev/null")
	if err != nil || info.Mode()&fs.ModeDevice == 0 {
		t.Skip("no /dev/null character device on this host")
	}
	link := filepath.Join(t.TempDir(), "tls.key")
	require.NoError(t, os.Symlink("/dev/null", link))

	err = checkKeyFilePermissions(link)
	requireKeyRefusal(t, err,
		"set environment mcp tls key to the private key file itself, then restart ze: "+privilege.RestartRemedy(),
		link, "/dev/null", "a device", "not a regular file")
}

// VALIDATES: a dangling symlink is refused with a message naming the link and
// its target, and the command that repoints the link.
// PREVENTS: a bare "no such file" that names neither the link nor its target.
func TestCheckKeyFilePathRefusesDanglingSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "gone.key")
	link := filepath.Join(dir, "tls.key")
	require.NoError(t, os.Symlink(target, link))

	err := checkKeyFilePermissions(link)
	requireKeyRefusal(t, err, "ln -sfn /path/to/private.key "+link,
		"dangling symlink", link, "points to "+target)
	assert.ErrorIs(t, err, fs.ErrNotExist)
}

// VALIDATES: a symlink to a key file another user owns is refused, with chown
// as the fix. Creating such a file needs root, so the test runs as root only.
// TestJudgeKeyPathRefusesEntriesAnotherUserControls proves the same judgment
// without root.
// PREVENTS: a link letting another user's file stand in as the key.
func TestCheckKeyFilePathRefusesSymlinkToAnotherUsersFile(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("creating a file owned by another user needs root; the judgment is covered by TestJudgeKeyPathRefusesEntriesAnotherUserControls")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "theirs.key")
	writeKeyForTest(t, target, 0o600)
	require.NoError(t, os.Chown(target, 65534, 65534))
	link := filepath.Join(dir, "tls.key")
	require.NoError(t, os.Symlink(target, link))

	requireKeyRefusal(t, checkKeyFilePermissions(link), "sudo chown root "+target,
		link, target, "owned by uid 65534")
}

// VALIDATES: a symlink loop is refused once the walk passes the kernel's limit
// of 40 links, for a loop through two links (a -> b -> a) and for a link to
// itself (a -> a), with the command that repoints the configured link.
// PREVENTS: following links turning a loop into a walk that never ends, or a
// bare ELOOP that names neither the limit nor the fix.
func TestCheckKeyFilePathRefusesSymlinkLoop(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "a.key")
	second := filepath.Join(dir, "b.key")
	require.NoError(t, os.Symlink("b.key", first))
	require.NoError(t, os.Symlink("a.key", second))
	self := filepath.Join(dir, "self.key")
	require.NoError(t, os.Symlink("self.key", self))

	for _, link := range []string{first, self} {
		t.Run(filepath.Base(link), func(t *testing.T) {
			requireKeyRefusal(t, checkKeyFilePermissions(link), "ln -sfn /path/to/private.key "+link,
				"key file "+link+" follows more than 40 symlinks")
		})
	}
}

// VALIDATES: the walk refuses a symlink owned by another user even when it
// sits in a sticky, world-writable directory (mode 01777) and points at a
// correct key, with `chown -h` as the fix. Changing a link's owner needs root,
// so the test runs as root only. TestJudgeKeyPathRefusesEntriesAnotherUserControls
// proves the same judgment without root.
// PREVENTS: the sticky exception for the directory letting the link's owner,
// who may replace the link in a sticky directory, point ze at a key of their
// choosing.
func TestCheckKeyFilePathRefusesAnotherUsersLinkInStickyDirectory(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("changing a symlink's owner to another user needs root; the judgment is covered by TestJudgeKeyPathRefusesEntriesAnotherUserControls")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "real.key")
	writeKeyForTest(t, target, 0o600)
	sticky := filepath.Join(dir, "sticky")
	require.NoError(t, os.Mkdir(sticky, 0o700))
	link := filepath.Join(sticky, "tls.key")
	require.NoError(t, os.Symlink(target, link))
	require.NoError(t, os.Lchown(link, 65534, 65534))
	require.NoError(t, os.Chmod(sticky, 0o777|os.ModeSticky))

	requireKeyRefusal(t, checkKeyFilePermissions(link), "sudo chown -h root "+link,
		"symlink "+link, "owned by uid 65534")
}

// VALIDATES: a symlink that sits in a directory other users may write, without
// the sticky bit, is refused even though its target is correct.
// PREVENTS: the swap the old blanket symlink refusal existed to stop: a user
// who can write the link's directory replacing the link with one to a key of
// their choosing.
func TestCheckKeyFilePathRefusesLinkInWritableDirectory(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.key")
	writeKeyForTest(t, target, 0o600)
	shared := filepath.Join(dir, "shared")
	require.NoError(t, os.Mkdir(shared, 0o700))
	link := filepath.Join(shared, "tls.key")
	require.NoError(t, os.Symlink(target, link))
	require.NoError(t, os.Chmod(shared, 0o777))

	requireKeyRefusal(t, checkKeyFilePermissions(link), "chmod go-w "+shared,
		shared, link, "writable by group or other users", "0777")
}

// VALIDATES: a key file whose own directory other users may write is refused.
// PREVENTS: a user replacing the target of a correct link in place.
func TestCheckKeyFilePathRefusesTargetInWritableDirectory(t *testing.T) {
	dir := t.TempDir()
	shared := filepath.Join(dir, "shared")
	require.NoError(t, os.Mkdir(shared, 0o700))
	target := filepath.Join(shared, "real.key")
	writeKeyForTest(t, target, 0o600)
	require.NoError(t, os.Chmod(shared, 0o770))
	link := filepath.Join(dir, "tls.key")
	require.NoError(t, os.Symlink(target, link))

	requireKeyRefusal(t, checkKeyFilePermissions(link), "chmod go-w "+shared,
		shared, link, "writable by group or other users", "0770")
}

// VALIDATES: each entry another user controls is refused by the judgment the
// walk applies: a key file, a symlink and a directory owned by another uid,
// with the chown command that fixes each.
// PREVENTS: the owner rule going untested on hosts where the suite runs
// without root and cannot create another user's file.
func TestJudgeKeyPathRefusesEntriesAnotherUserControls(t *testing.T) {
	const euid, other = int64(1000), int64(4242)
	cases := []struct {
		name  string
		entry keyPathEntry
		fix   string
	}{
		{"key file", keyPathEntry{path: "/srv/keys/tls.key", role: keyPathKey, mode: 0o600, uid: other},
			"sudo chown 1000 /srv/keys/tls.key"},
		{"symlink", keyPathEntry{path: "/srv/keys/link.key", role: keyPathLink, mode: fs.ModeSymlink | 0o777, uid: other},
			"sudo chown -h 1000 /srv/keys/link.key"},
		{"directory", keyPathEntry{path: "/srv/keys", role: keyPathDirectory, mode: fs.ModeDir | 0o700, uid: other},
			"sudo chown 1000 /srv/keys"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireKeyRefusal(t, judgeKeyPathEntry("/etc/ze/tls.key", tc.entry, euid), tc.fix,
				tc.entry.path, "owned by uid 4242", "(uid 1000)")
			trusted := tc.entry
			trusted.uid = euid
			assert.NoError(t, judgeKeyPathEntry("/etc/ze/tls.key", trusted, euid), "the user ze runs as may own it")
			trusted.uid = 0
			assert.NoError(t, judgeKeyPathEntry("/etc/ze/tls.key", trusted, euid), "root may own it")
		})
	}
}

// VALIDATES: a writable directory passes only with the sticky bit, and a
// group-writable one is refused like a world-writable one.
// PREVENTS: the sticky exception widening to every writable directory.
func TestJudgeKeyPathDirectoryWritableNeedsSticky(t *testing.T) {
	sticky := keyPathEntry{path: "/run/secrets", role: keyPathDirectory, mode: fs.ModeDir | fs.ModeSticky | 0o777}
	assert.NoError(t, judgeKeyPathEntry("/run/secrets/tls.key", sticky, 0))

	groupWritable := keyPathEntry{path: "/run/secrets", role: keyPathDirectory, mode: fs.ModeDir | 0o775}
	requireKeyRefusal(t, judgeKeyPathEntry("/run/secrets/tls.key", groupWritable, 0), "chmod go-w /run/secrets",
		"/run/secrets", "writable by group or other users", "0775")
}
