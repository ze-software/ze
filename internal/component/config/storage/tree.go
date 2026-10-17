// Design: docs/architecture/storage-backends.md -- descriptor-relative tree I/O.
package storage

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/pkg/zefs"
)

var ErrCorrupt = errors.New("corrupt storage value")

type treeEncoding struct {
	root   *os.File
	folder *os.File
	// lock is the tree whose owner lock covers this tree's writes. It names
	// every frame a write stages in folder, so that owner can tell its own
	// crash leftovers from another lock holder's live write (stageName).
	lock string
	// sync is instance-local so crash barriers can be exercised without global hooks.
	sync func(*os.File) error
}

func secureNode(file *os.File, directory bool) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	// Fstat inspects the opened inode, not a replaceable pathname.
	var st unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &st); err != nil {
		return err
	}
	mode := uint32(0o600)
	kind := uint32(unix.S_IFREG)
	if directory {
		mode = 0o700
		kind = unix.S_IFDIR
	}
	if uint32(st.Mode)&unix.S_IFMT != kind { //nolint:unconvert // Mode is uint16 on darwin and freebsd
		return fmt.Errorf("%w: %s mode %s: remove unsafe node", ErrPermissions, file.Name(), info.Mode())
	}
	if uint32(st.Mode)&0o7777 != mode { //nolint:unconvert // Mode is uint16 on darwin and freebsd
		return fmt.Errorf("%w: %s mode %04o: chmod %04o %s", ErrPermissions, file.Name(), st.Mode&0o7777, mode, file.Name())
	}
	if int(st.Uid) != os.Geteuid() {
		return fmt.Errorf("%w: %s owner %d, caller %d: run maintenance as owner %d", ErrPermissions, file.Name(), st.Uid, os.Geteuid(), st.Uid)
	}
	return nil
}

func openNode(parent *os.File, name string, directory bool) (*os.File, error) {
	flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
	if directory {
		flags |= unix.O_DIRECTORY
	}
	fd, err := unix.Openat(int(parent.Fd()), name, flags, 0)
	path := filepath.Join(parent.Name(), name)
	if err != nil {
		if errors.Is(err, unix.ELOOP) {
			return nil, fmt.Errorf("%w: symlink %s: remove unsafe node", ErrPermissions, path)
		}
		if errors.Is(err, unix.ENOTDIR) {
			return nil, fmt.Errorf("%w: non-directory %s: remove unsafe node", ErrPermissions, path)
		}
		return nil, fmt.Errorf("storage node %s: %w", path, err)
	}
	file := os.NewFile(uintptr(fd), path)
	if err := secureNode(file, directory); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return file, nil
}

func openFolder(path string) (*os.File, error) {
	return openFolderMode(path, false)
}

// openFolderMode is the one no-follow walk to the containing folder, declared
// in zefs; storage only names the refusal as its own permission error.
func openFolderMode(path string, create bool) (*os.File, error) {
	folder, err := zefs.OpenDirectory(path, create)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return nil, fmt.Errorf("%w: %w", ErrPermissions, err)
		}
		return nil, err
	}
	return folder, nil
}

func (t *treeEncoding) barrier(file *os.File) error {
	if t.sync != nil {
		return t.sync(file)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync %s: %w", file.Name(), err)
	}
	return nil
}

func (t *treeEncoding) parent(key string, create bool) (*os.File, string, error) {
	if err := validKey(key); err != nil {
		return nil, "", err
	}
	parts := strings.Split(key, "/")
	current, err := openNode(t.root, ".", true)
	if err != nil {
		return nil, "", err
	}
	for _, part := range parts[:len(parts)-1] {
		next, err := openNode(current, part, true)
		if errors.Is(err, fs.ErrNotExist) {
			if create {
				if err = unix.Mkdirat(int(current.Fd()), part, 0o700); err == nil {
					err = t.barrier(current)
				}
				if err == nil {
					next, err = openNode(current, part, true)
				}
			}
		}
		closeErr := current.Close()
		if err != nil {
			return nil, "", errors.Join(err, closeErr)
		}
		if closeErr != nil {
			return nil, "", errors.Join(closeErr, next.Close())
		}
		current = next
	}
	return current, parts[len(parts)-1], nil
}

func (t *treeEncoding) has(key string) bool {
	parent, leaf, err := t.parent(key, false)
	if err != nil {
		return false
	}
	defer parent.Close() //nolint:errcheck // existence probe.
	file, err := openNode(parent, leaf, false)
	if err != nil {
		return false
	}
	return file.Close() == nil
}

func (t *treeEncoding) ReadFile(key string) ([]byte, error) {
	parent, leaf, err := t.parent(key, false)
	if err != nil {
		return nil, err
	}
	defer parent.Close() //nolint:errcheck // read handle.
	file, err := openNode(parent, leaf, false)
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck // read handle.
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() < 0 {
		return nil, fmt.Errorf("%w: negative frame size: %s", ErrCorrupt, key)
	}
	if uint64(info.Size()) > uint64(int(^uint(0)>>1)) {
		return nil, fmt.Errorf("%w: frame too large for this process: %s", ErrCorrupt, key)
	}
	frame := make([]byte, int(info.Size()))
	if _, err := io.ReadFull(file, frame); err != nil {
		return nil, fmt.Errorf("%w: key %s: %w", ErrCorrupt, key, err)
	}
	var extra [1]byte
	if _, err := file.Read(extra[:]); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: frame changed during read: %s", ErrCorrupt, key)
	}
	data, _, next, err := zefs.DecodeNetcapstringRef(frame, 0)
	if err != nil {
		return nil, fmt.Errorf("%w: key %s: %w", ErrCorrupt, key, err)
	}
	if next != len(frame) {
		return nil, fmt.Errorf("%w: key %s: trailing bytes at %d", ErrCorrupt, key, next)
	}
	return data, nil
}

func (t *treeEncoding) WriteFile(key string, data []byte, _ fs.FileMode) error {
	parent, leaf, err := t.parent(key, true)
	if err != nil {
		return err
	}
	defer parent.Close() //nolint:errcheck // barriers report errors before close.
	old, err := openNode(parent, leaf, false)
	if err == nil {
		if err := old.Close(); err != nil {
			return err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	frame, err := zefs.EncodeNetcapstring(data, len(data))
	if err != nil {
		return err
	}
	return installBytes(t.folder, parent, t.lock, leaf, frame, t.barrier)
}

// stagePrefix opens every name installBytes stages a frame under.
const stagePrefix = ".ze-storage-"

// stageName names a frame staged for a write that lock's owner lock covers:
// stagePrefix, lock, a dot, then rand.Text. The random part is base32, so it
// holds no dot, and the last dot of a name always ends its lock. Two trees in
// one folder therefore never stage a name the other's sweep matches, even when
// one tree's name extends the other's (`database`, `database.replaced-<stamp>`).
// An empty lock is the one write that holds no owner lock: a loose config a
// memory store publishes (WriteConfigFile). Its name, stagePrefix then a dot
// then rand.Text, is reserved: stagedFor is never asked about an empty lock
// and stagedLegacy refuses the dot, so no sweep ever removes it.
func stageName(lock string) string {
	return stagePrefix + lock + "." + rand.Text()
}

// stagedFor answers whether entry is a frame stageName produced for lock.
func stagedFor(entry, lock string) bool {
	random, ok := strings.CutPrefix(entry, stagePrefix+lock+".")
	if !ok {
		return false
	}
	return isRandomText(random)
}

// stagedLegacy answers whether entry is a frame in the form every write used
// before frames named their lock: stagePrefix then rand.Text, with no dot.
// Nothing produces that form any more, so only a crash before the change left
// one, and the live owner removes it.
func stagedLegacy(entry string) bool {
	random, ok := strings.CutPrefix(entry, stagePrefix)
	if !ok {
		return false
	}
	return isRandomText(random)
}

// isRandomText answers whether text could be a rand.Text result: non-empty,
// and every byte in the RFC 4648 base32 alphabet it draws from. The length is
// not checked, because rand.Text documents that a later Go may return more.
func isRandomText(text string) bool {
	if text == "" {
		return false
	}
	for i := range len(text) {
		c := text[i]
		if c >= 'A' && c <= 'Z' {
			continue
		}
		if c >= '2' && c <= '7' {
			continue
		}
		return false
	}
	return true
}

// sweepStages removes the frames a crashed write left in folder, the entries
// leftover matches: a crash between the staging create and the rename leaves
// one, and the write's own cleanup never runs. The caller MUST hold the owner
// lock whose frames leftover matches. Only a holder of that lock stages such a
// name, so while the caller holds it none of them belongs to a live write.
// Every other entry, including a frame another lock's holder may be writing
// right now, is left alone, and so is any matching entry that is not a regular
// file.
//
// The sweep is best-effort and never fails the open. A leftover holds no
// published data and no reader lists it, so one that cannot be inspected or
// removed is reported on log, with the command that removes it, and the sweep
// moves on.
//
// The removals are not synced. A crash that loses one brings back a name
// outside the key namespace, and the next owner open removes it again; a sync
// would make nothing durable that matters (storage-backends.md, "Durable
// publication").
func sweepStages(folder *os.File, leftover func(string) bool, log *slog.Logger) {
	listing, err := duplicateFolder(folder)
	if err != nil {
		warnLeftover(log, folder.Name(), "could not open the folder to look for leftover staged writes", folderFix(folder), err)
		return
	}
	names, err := listing.Readdirnames(-1)
	if err := errors.Join(err, listing.Close()); err != nil {
		warnLeftover(log, folder.Name(), "could not list the folder for leftover staged writes", folderFix(folder), err)
		return
	}
	for _, name := range names {
		if !leftover(name) {
			continue
		}
		path := filepath.Join(folder.Name(), name)
		var st unix.Stat_t
		err := unix.Fstatat(int(folder.Fd()), name, &st, unix.AT_SYMLINK_NOFOLLOW)
		if errors.Is(err, unix.ENOENT) {
			continue
		}
		if err != nil {
			warnLeftover(log, path, "could not inspect a leftover staged write", "rm "+path, err)
			continue
		}
		if st.Mode&unix.S_IFMT != unix.S_IFREG {
			continue
		}
		err = unix.Unlinkat(int(folder.Fd()), name, 0)
		if errors.Is(err, unix.ENOENT) {
			continue
		}
		if err != nil {
			warnLeftover(log, path, "could not remove a leftover staged write", "rm "+path, err)
		}
	}
}

// warnLeftover reports one sweep failure: what failed on which path, why the
// open goes on regardless, and the command an operator runs instead.
func warnLeftover(log *slog.Logger, path, problem, fix string, err error) {
	log.Warn(problem,
		"path", path,
		"error", err,
		"why", "a crashed write leaves a staged frame that holds no published data and that no reader lists, so the store opens anyway",
		"fix", fix)
}

// folderFix is the operator's command when the folder itself cannot be read:
// list its leftover frames, then remove them while ze is stopped.
func folderFix(folder *os.File) string {
	return "ls -la " + folder.Name() + " and, with ze stopped, rm its " + stagePrefix + "* files"
}

// installBytes publishes data as leaf in parent through a frame staged in
// stage under stageName(lock).
func installBytes(stage, parent *os.File, lock, leaf string, data []byte, syncFile func(*os.File) error) (retErr error) {
	name := stageName(lock)
	fd, err := unix.Openat(int(stage.Fd()), name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o600)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), filepath.Join(stage.Name(), name))
	defer func() {
		if err := unix.Unlinkat(int(stage.Fd()), name, 0); err != nil && !errors.Is(err, unix.ENOENT) {
			retErr = errors.Join(retErr, err)
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		return errors.Join(err, file.Close())
	}
	if _, err := file.Write(data); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := syncFile(file); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := unix.Renameat(int(stage.Fd()), name, int(parent.Fd()), leaf); err != nil {
		return err
	}
	// Two barriers make the value durable: the frame above, then the
	// destination directory that now names it. The staging directory is not
	// synced. When it is the destination (the import intent and
	// WriteConfigFile) that sync would repeat this one. When it is distinct
	// (a tree key write) it would only make the temporary name's departure
	// durable, and a `.ze-storage-*` name a crash leaves behind is outside the
	// key namespace, ignored by readers, and removed by the next owner open
	// (sweepStages; storage-backends.md, "Durable publication").
	return syncFile(parent)
}

func duplicateFolder(folder *os.File) (*os.File, error) {
	fd, err := unix.Openat(int(folder.Fd()), ".", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), folder.Name()), nil
}

func nodeStat(folder *os.File, name string) error {
	var st unix.Stat_t
	return unix.Fstatat(int(folder.Fd()), name, &st, unix.AT_SYMLINK_NOFOLLOW)
}

func makeStage(folder *os.File, prefix string) (string, error) {
	name := prefix + rand.Text()
	if err := unix.Mkdirat(int(folder.Fd()), name, 0o700); err != nil {
		return "", err
	}
	fd, err := unix.Openat(int(folder.Fd()), name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return "", errors.Join(err, unix.Unlinkat(int(folder.Fd()), name, unix.AT_REMOVEDIR))
	}
	stage := os.NewFile(uintptr(fd), filepath.Join(folder.Name(), name))
	if err := errors.Join(stage.Chmod(0o700), stage.Close()); err != nil {
		return "", errors.Join(err, unix.Unlinkat(int(folder.Fd()), name, unix.AT_REMOVEDIR))
	}
	return name, nil
}

// removeStage never resolves the retained parent's pathname. Its bounded
// directory reads and explicit stack also avoid filesystem-controlled recursion.
func removeStage(parent *os.File, name string) (retErr error) {
	root, err := openNode(parent, name, true)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	type directory struct {
		file *os.File
		name string
	}
	stack := []directory{{file: root, name: name}}
	defer func() {
		for _, dir := range stack {
			retErr = errors.Join(retErr, dir.file.Close())
		}
	}()
	for len(stack) > 0 {
		dir := &stack[len(stack)-1]
		entries, err := dir.file.ReadDir(1)
		if errors.Is(err, io.EOF) {
			name := dir.name
			closeErr := dir.file.Close()
			stack = stack[:len(stack)-1]
			if closeErr != nil {
				return closeErr
			}
			folder := parent
			if len(stack) > 0 {
				folder = stack[len(stack)-1].file
			}
			if err := unix.Unlinkat(int(folder.Fd()), name, unix.AT_REMOVEDIR); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		entry := entries[0]
		if entry.IsDir() {
			child, err := openNode(dir.file, entry.Name(), true)
			if err != nil {
				return err
			}
			stack = append(stack, directory{file: child, name: entry.Name()})
			continue
		}
		if err := unix.Unlinkat(int(dir.file.Fd()), entry.Name(), 0); err != nil {
			return err
		}
	}
	return parent.Sync()
}

func (t *treeEncoding) Remove(key string) error {
	parent, leaf, err := t.parent(key, false)
	if err != nil {
		return err
	}
	file, err := openNode(parent, leaf, false)
	if err != nil {
		return errors.Join(err, parent.Close())
	}
	if err := file.Close(); err != nil {
		return errors.Join(err, parent.Close())
	}
	err = unix.Unlinkat(int(parent.Fd()), leaf, 0)
	if err == nil {
		err = t.barrier(parent)
	}
	err = errors.Join(err, parent.Close())
	if err != nil {
		return err
	}
	// Each iteration removes one key component; prune only empty ancestors.
	for dir := filepath.Dir(key); dir != "."; dir = filepath.Dir(dir) {
		parent, leaf, err := t.parent(dir, false)
		if err != nil {
			return err
		}
		err = unix.Unlinkat(int(parent.Fd()), leaf, unix.AT_REMOVEDIR)
		if errors.Is(err, unix.ENOTEMPTY) {
			return parent.Close()
		}
		if err == nil {
			err = t.barrier(parent)
		}
		err = errors.Join(err, parent.Close())
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *treeEncoding) list(prefix string) ([]string, error) {
	var keys []string
	queue := []string{"."}
	for len(queue) > 0 {
		dir := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		var file *os.File
		var err error
		if dir == "." {
			file, err = openNode(t.root, ".", true)
		} else {
			parent, leaf, parentErr := t.parent(dir, false)
			if parentErr != nil {
				return nil, parentErr
			}
			file, err = openNode(parent, leaf, true)
			err = errors.Join(err, parent.Close())
		}
		if err != nil {
			return nil, err
		}
		// Each call consumes the next directory entries; EOF bounds the walk.
		for {
			entries, readErr := file.ReadDir(128)
			if readErr != nil {
				if !errors.Is(readErr, io.EOF) {
					return nil, errors.Join(readErr, file.Close())
				}
			}
			for _, entry := range entries {
				name := entry.Name()
				if dir != "." {
					name = dir + "/" + name
				}
				child, err := openNode(file, entry.Name(), entry.IsDir())
				if err != nil {
					return nil, errors.Join(err, file.Close())
				}
				if err := child.Close(); err != nil {
					return nil, errors.Join(err, file.Close())
				}
				if entry.IsDir() {
					queue = append(queue, name)
					continue
				}
				if strings.HasPrefix(name, prefix) {
					keys = append(keys, name)
				}
			}
			if errors.Is(readErr, io.EOF) {
				break
			}
		}
		if err := file.Close(); err != nil {
			return nil, err
		}
	}
	slices.Sort(keys)
	return keys, nil
}
func (t *treeEncoding) close() error { return errors.Join(t.root.Close(), t.folder.Close()) }

func (t *treeEncoding) published(name string) error {
	root, err := openNode(t.folder, name, true)
	if err != nil {
		return err
	}
	if err := t.root.Close(); err != nil {
		return errors.Join(err, root.Close())
	}
	t.root = root
	return nil
}
