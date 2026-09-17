// Design: docs/architecture/storage-backends.md -- descriptor-relative tree I/O.
package storage

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/pkg/zefs"
)

var ErrCorrupt = errors.New("corrupt storage value")

type treeEncoding struct {
	root   *os.File
	folder *os.File
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
	if uint32(st.Mode)&unix.S_IFMT != kind {
		return fmt.Errorf("%w: %s mode %s: remove unsafe node", ErrPermissions, file.Name(), info.Mode())
	}
	if uint32(st.Mode)&0o7777 != mode {
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

func openFolderMode(path string, create bool) (*os.File, error) {
	absolute := path
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		absolute = cwd + "/" + path
	}
	// Darwin's /tmp and /var are system aliases; resolve only those known
	// prefixes, then reject symlinks in every remaining component.
	if runtime.GOOS == "darwin" {
		for _, alias := range []string{"/tmp", "/var"} {
			if absolute == alias {
				absolute = "/private" + absolute
				break
			}
			if strings.HasPrefix(absolute, alias+"/") {
				absolute = "/private" + absolute
				break
			}
		}
	}
	parts := strings.Split(strings.TrimPrefix(absolute, "/"), "/")
	current, err := openArtifactFolder("/")
	if err != nil {
		return nil, err
	}
	depth, privateDepth := 0, -1
	for _, part := range parts {
		if part == "" {
			continue
		}
		fd, err := unix.Openat(int(current.Fd()), part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		created := false
		if create && errors.Is(err, unix.ENOENT) {
			mkdirErr := unix.Mkdirat(int(current.Fd()), part, 0o700)
			if mkdirErr != nil && !errors.Is(mkdirErr, unix.EEXIST) {
				return nil, errors.Join(mkdirErr, current.Close())
			}
			created = mkdirErr == nil
			fd, err = unix.Openat(int(current.Fd()), part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		}
		if err != nil {
			return nil, errors.Join(fmt.Errorf("%w: %s: %v", ErrPermissions, absolute, err), current.Close())
		}
		next := os.NewFile(uintptr(fd), filepath.Join(current.Name(), part))
		if created {
			if err := next.Chmod(0o700); err != nil {
				return nil, errors.Join(err, next.Close(), current.Close())
			}
			if err := errors.Join(next.Sync(), current.Sync()); err != nil {
				return nil, errors.Join(err, next.Close(), current.Close())
			}
		}
		if err := current.Close(); err != nil {
			return nil, errors.Join(err, next.Close())
		}
		current = next
		var st unix.Stat_t
		if err := unix.Fstat(fd, &st); err != nil {
			return nil, errors.Join(err, current.Close())
		}
		trusted := int(st.Uid) == os.Geteuid()
		if st.Uid == 0 {
			trusted = true
		}
		if !trusted {
			return nil, errors.Join(fmt.Errorf("%w: untrusted directory owner %d: %s", ErrPermissions, st.Uid, current.Name()), current.Close())
		}
		switch part {
		case ".":
		case "..":
			if depth > 0 {
				depth--
			}
			if depth < privateDepth {
				privateDepth = -1
			}
		default:
			depth++
		}
		// A caller-owned private ancestor prevents outsiders from reaching
		// later writable directories. Leaving it through ".." ends that protection.
		if privateDepth < 0 && int(st.Uid) == os.Geteuid() && st.Mode&0o077 == 0 {
			privateDepth = depth
		}
		if privateDepth < 0 && st.Mode&0o022 != 0 {
			if st.Mode&unix.S_ISVTX == 0 {
				return nil, errors.Join(fmt.Errorf("%w: writable directory %s mode %04o; remove group/other write permission", ErrPermissions, current.Name(), st.Mode&0o7777), current.Close())
			}
		}
	}
	return current, nil
}

// Artifact directories can be ordinary output directories. Only the artifact
// inode carries the secret-bearing 0600/owner requirement.
func openArtifactFolder(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
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
			next.Close()
			return nil, "", closeErr
		} //nolint:errcheck // primary close failure retained.
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
		return nil, fmt.Errorf("%w: key %s: %v", ErrCorrupt, key, err)
	}
	var extra [1]byte
	if _, err := file.Read(extra[:]); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: frame changed during read: %s", ErrCorrupt, key)
	}
	data, _, next, err := zefs.DecodeNetcapstringRef(frame, 0)
	if err != nil {
		return nil, fmt.Errorf("%w: key %s: %v", ErrCorrupt, key, err)
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
		if err = old.Close(); err != nil {
			return err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	frame, err := zefs.EncodeNetcapstring(data, len(data))
	if err != nil {
		return err
	}
	return installBytes(t.folder, parent, leaf, frame, t.barrier)
}

func installBytes(stage, parent *os.File, leaf string, data []byte, syncFile func(*os.File) error) (retErr error) {
	name := ".ze-storage-" + rand.Text()
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
	if err := syncFile(parent); err != nil {
		return err
	}
	return syncFile(stage)
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
		parent.Close()
		return err
	} //nolint:errcheck // primary node error.
	if err = file.Close(); err != nil {
		parent.Close()
		return err
	} //nolint:errcheck // primary close error.
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
