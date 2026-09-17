// Design: docs/architecture/zefs-format.md -- secure framed-tree integrity and salvage.
// Related: check.go -- public integrity reports and artifact dispatch.

//go:build linux || darwin || freebsd

package zefs

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

const frameDirectoryFlags = unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW | unix.O_CLOEXEC

// OpenDirectory checks every path component from / without resolving
// symlinks. The returned descriptor pins the directory for subsequent I/O.
// The caller MUST close it. Ancestors may be root-owned or caller-owned.
// Writable shared ancestors require the sticky bit until a caller-private
// directory prevents outsiders from traversing the remaining path. With
// create, a missing component is made 0700 and durably recorded in its parent;
// an existing component is never altered. Every refusal wraps fs.ErrPermission.
func OpenDirectory(path string, create bool) (*os.File, error) {
	absolute := path
	if !filepath.IsAbs(absolute) {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		// Do not clean away a symlink component followed by "..".
		absolute = cwd + "/" + path
	}
	absolute = trustedFramePath(absolute)
	fd, err := unix.Open("/", frameDirectoryFlags, 0)
	if err != nil {
		return nil, err
	}
	current := os.NewFile(uintptr(fd), "/")
	depth, privateDepth := 0, -1
	for part := range strings.SplitSeq(absolute, "/") {
		if part == "" {
			continue
		}
		if part == "." {
			continue
		}
		if part == ".." {
			if depth > 0 {
				depth--
			}
			if depth < privateDepth {
				privateDepth = -1
			}
		} else {
			depth++
		}
		name := filepath.Join(current.Name(), part)
		next, err := openDirectoryComponent(current, part, name, create)
		if err != nil {
			return nil, errors.Join(err, current.Close())
		}
		if err := current.Close(); err != nil {
			return nil, errors.Join(err, next.Close())
		}
		current = next
		var stat unix.Stat_t
		if err := unix.Fstat(int(current.Fd()), &stat); err != nil {
			return nil, errors.Join(err, current.Close())
		}
		if stat.Uid != 0 {
			if stat.Uid != uint32(os.Geteuid()) {
				return nil, errors.Join(fmt.Errorf("zefs: %s: unsafe ancestor %s owner %d: %w", path, name, stat.Uid, fs.ErrPermission), current.Close())
			}
		}
		if privateDepth < 0 && stat.Mode&0o022 != 0 {
			if stat.Mode&unix.S_ISVTX == 0 {
				return nil, errors.Join(fmt.Errorf("zefs: %s: writable ancestor %s mode %#o: %w", path, name, stat.Mode&0o7777, fs.ErrPermission), current.Close())
			}
		}
		if privateDepth < 0 && stat.Uid == uint32(os.Geteuid()) && stat.Mode&0o077 == 0 {
			privateDepth = depth
		}
	}
	return current, nil
}

// openDirectoryComponent opens one component beneath parent, creating it 0700
// when create is set and it is absent. A created component is fsynced with its
// parent so the name is durable before any descendant is written into it.
func openDirectoryComponent(parent *os.File, part, name string, create bool) (*os.File, error) {
	fd, err := unix.Openat(int(parent.Fd()), part, frameDirectoryFlags, 0)
	created := false
	if create && errors.Is(err, unix.ENOENT) {
		mkdirErr := unix.Mkdirat(int(parent.Fd()), part, 0o700)
		if mkdirErr != nil && !errors.Is(mkdirErr, unix.EEXIST) {
			return nil, mkdirErr
		}
		created = mkdirErr == nil
		fd, err = unix.Openat(int(parent.Fd()), part, frameDirectoryFlags, 0)
	}
	if err != nil {
		return nil, fmt.Errorf("zefs: open %s: unsafe component %s: %w: %w", parent.Name(), name, err, fs.ErrPermission)
	}
	next := os.NewFile(uintptr(fd), name)
	if !created {
		return next, nil
	}
	// The umask can widen Mkdirat's mode; the descriptor pins the new inode.
	if err := next.Chmod(0o700); err != nil {
		return nil, errors.Join(err, next.Close())
	}
	if err := errors.Join(next.Sync(), parent.Sync()); err != nil {
		return nil, errors.Join(err, next.Close())
	}
	return next, nil
}

func walkFrameTree(path string, visit func(string, []byte) error) (retErr error) {
	root, err := OpenDirectory(path, false)
	if err != nil {
		return err
	}
	defer func() { retErr = errors.Join(retErr, root.Close()) }()
	return walkFrameDirectory(root, visit)
}

// walkFrameDirectory borrows root; its caller MUST retain and close root.
// An explicit stack avoids recursion for filesystem-controlled depth. Native
// path, descriptor and memory limits apply; there is no smaller tree namespace
// limit. Nonblocking opens reject substituted FIFOs without waiting for writers.
func walkFrameDirectory(root *os.File, visit func(string, []byte) error) (retErr error) {
	if err := checkFrameNode(root, true); err != nil {
		return err
	}
	type directory struct {
		file *os.File
		key  string
	}
	stack := []directory{{file: root}}
	defer func() {
		for _, dir := range stack[1:] {
			retErr = errors.Join(retErr, dir.file.Close())
		}
	}()
	for len(stack) > 0 {
		dir := &stack[len(stack)-1]
		entries, err := dir.file.ReadDir(1)
		if errors.Is(err, io.EOF) {
			if len(stack) == 1 {
				break
			}
			closeErr := dir.file.Close()
			stack = stack[:len(stack)-1]
			if closeErr != nil {
				return closeErr
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("zefs: read directory %s: %w", dir.file.Name(), err)
		}
		name := entries[0].Name()
		key := name
		if dir.key != "" {
			key = dir.key + "/" + name
		}
		fd, err := unix.Openat(int(dir.file.Fd()), name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		if err != nil {
			return fmt.Errorf("zefs: open %s: %w", filepath.Join(root.Name(), key), err)
		}
		file := os.NewFile(uintptr(fd), filepath.Join(root.Name(), key))
		info, err := file.Stat()
		if err != nil {
			return errors.Join(err, file.Close())
		}
		if err := checkFrameNode(file, info.IsDir()); err != nil {
			return errors.Join(err, file.Close())
		}
		if info.IsDir() {
			stack = append(stack, directory{file: file, key: key})
			continue
		}
		frame, err := readFrameFile(file, info.Size())
		closeErr := file.Close()
		if err != nil {
			return errors.Join(fmt.Errorf("zefs: read %s: %w", file.Name(), err), closeErr)
		}
		if closeErr != nil {
			return closeErr
		}
		if err := visit(key, frame); err != nil {
			return err
		}
	}
	return nil
}

func checkFrameNode(file *os.File, directory bool) error {
	var stat unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &stat); err != nil {
		return fmt.Errorf("zefs: stat %s: %w", file.Name(), err)
	}
	mode := uint32(0o600)
	kind := uint32(unix.S_IFREG)
	if directory {
		mode = 0o700
		kind = unix.S_IFDIR
	}
	if uint32(stat.Mode)&unix.S_IFMT != kind { //nolint:unconvert // Mode is uint16 on darwin and freebsd
		return fmt.Errorf("zefs: %s: refusing non-regular node mode %#o", file.Name(), stat.Mode)
	}
	if uint32(stat.Mode)&0o7777 != mode { //nolint:unconvert // Mode is uint16 on darwin and freebsd
		return fmt.Errorf("zefs: %s: mode %#o; run chmod %03o %q", file.Name(), stat.Mode&0o7777, mode, file.Name())
	}
	if stat.Uid != uint32(os.Geteuid()) {
		return fmt.Errorf("zefs: %s: owner %d differs from caller %d; run maintenance as the store owner", file.Name(), stat.Uid, os.Geteuid())
	}
	return nil
}

// readFrameFile uses the opened inode's size as its read bound. A concurrent
// append cannot extend the read indefinitely, and a changed size is an error.
func readFrameFile(file *os.File, size int64) ([]byte, error) {
	if size < 0 {
		return nil, fmt.Errorf("zefs: %s: negative frame size %d", file.Name(), size)
	}
	if uint64(size) > uint64(^uint(0)>>1) {
		return nil, fmt.Errorf("zefs: %s: frame size %d exceeds addressable memory", file.Name(), size)
	}
	frame := make([]byte, int(size))
	if _, err := io.ReadFull(file, frame); err != nil {
		return nil, err
	}
	var extra [1]byte
	n, err := file.Read(extra[:])
	if n != 0 {
		return nil, fmt.Errorf("zefs: %s: frame grew while reading", file.Name())
	}
	if !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, err
		}
		return nil, io.ErrNoProgress
	}
	return frame, nil
}

func repairFrameTree(srcPath, dstPath string) (report *RepairReport, retErr error) {
	source, err := OpenDirectory(srcPath, false)
	if err != nil {
		return nil, err
	}
	defer func() { retErr = errors.Join(retErr, source.Close()) }()
	if err := checkFrameNode(source, true); err != nil {
		return nil, err
	}
	// Split preserves intermediate components so OpenDirectory can reject
	// a link even when the following component is "..".
	parentPath, destination := filepath.Split(dstPath)
	if destination == "" {
		return nil, fmt.Errorf("zefs: repair output %s requires a new directory name", dstPath)
	}
	parent, err := OpenDirectory(parentPath, false)
	if err != nil {
		return nil, err
	}
	defer func() { retErr = errors.Join(retErr, parent.Close()) }()
	rel, err := filepath.Rel(source.Name(), parent.Name())
	if err != nil {
		return nil, err
	}
	if rel != ".." {
		if !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("zefs: repair output %s must be outside source %s", dstPath, srcPath)
		}
	}
	// The random sibling name is created and later published relative to the
	// same pinned parent, so a pathname replacement cannot redirect either step.
	stageName := ".zefs-repair-" + rand.Text()
	if err := unix.Mkdirat(int(parent.Fd()), stageName, 0o700); err != nil {
		return nil, err
	}
	defer func() { retErr = errors.Join(retErr, removeRepairStage(parent, stageName)) }()
	fd, err := unix.Openat(int(parent.Fd()), stageName, frameDirectoryFlags, 0)
	if err != nil {
		return nil, err
	}
	stage := os.NewFile(uintptr(fd), filepath.Join(parent.Name(), stageName))
	defer func() { retErr = errors.Join(retErr, stage.Close()) }()
	if err := stage.Chmod(0o700); err != nil {
		return nil, err
	}
	report = &RepairReport{SourcePath: srcPath, OutputPath: dstPath}
	err = walkFrameDirectory(source, func(key string, frame []byte) error {
		_, _, next, decodeErr := DecodeNetcapstringRef(frame, 0)
		if decodeErr == nil {
			if next != len(frame) {
				decodeErr = fmt.Errorf("trailing bytes at offset %d", next)
			}
		}
		if decodeErr != nil {
			report.Skipped = append(report.Skipped, EntryStatus{Key: key, Status: integrityStatus(decodeErr), Error: decodeErr.Error()})
			report.SkippedCount++
			return nil
		}
		if err := writeRepairFrame(stage, key, frame); err != nil {
			return err
		}
		report.Recovered = append(report.Recovered, key)
		report.RecoveredCount++
		return nil
	})
	if err != nil {
		return report, err
	}
	if err := stage.Sync(); err != nil {
		return report, err
	}
	if err := RenameNoReplace(parent, stageName, destination); err != nil {
		return report, fmt.Errorf("zefs: publish repaired tree %s: %w", dstPath, err)
	}
	if err := parent.Sync(); err != nil {
		return report, err
	}
	return report, nil
}

// writeRepairFrame installs one good frame into private staging. Directory and
// file creation, chmod and durability barriers all use pinned descriptors.
func writeRepairFrame(stage *os.File, key string, frame []byte) (retErr error) {
	fd, err := unix.Openat(int(stage.Fd()), ".", frameDirectoryFlags, 0)
	if err != nil {
		return err
	}
	parent := os.NewFile(uintptr(fd), stage.Name())
	defer func() { retErr = errors.Join(retErr, parent.Close()) }()
	for {
		part, remaining, found := strings.Cut(key, "/")
		if !found {
			break
		}
		mkdirErr := unix.Mkdirat(int(parent.Fd()), part, 0o700)
		if mkdirErr != nil {
			if !errors.Is(mkdirErr, unix.EEXIST) {
				return mkdirErr
			}
		}
		fd, err := unix.Openat(int(parent.Fd()), part, frameDirectoryFlags, 0)
		if err != nil {
			return err
		}
		child := os.NewFile(uintptr(fd), filepath.Join(parent.Name(), part))
		if mkdirErr == nil {
			if err := child.Chmod(0o700); err != nil {
				return errors.Join(err, child.Close())
			}
			if err := child.Sync(); err != nil {
				return errors.Join(err, child.Close())
			}
			if err := parent.Sync(); err != nil {
				return errors.Join(err, child.Close())
			}
		}
		if err := checkFrameNode(child, true); err != nil {
			return errors.Join(err, child.Close())
		}
		if err := parent.Close(); err != nil {
			parent = child
			return err
		}
		parent = child
		key = remaining
	}
	fd, err = unix.Openat(int(parent.Fd()), key, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o600)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), filepath.Join(parent.Name(), key))
	defer func() { retErr = errors.Join(retErr, file.Close()) }()
	if err := file.Chmod(0o600); err != nil {
		return err
	}
	if _, err := file.Write(frame); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	return parent.Sync()
}

// removeRepairStage removes only the private staging entry through the retained
// parent descriptor. A successful rename leaves that name absent. Descendants
// are removed without following links, including when cleanup follows failure.
func removeRepairStage(parent *os.File, name string) (retErr error) {
	fd, err := unix.Openat(int(parent.Fd()), name, frameDirectoryFlags, 0)
	if errors.Is(err, unix.ENOENT) {
		return nil
	}
	if err != nil {
		return err
	}
	type directory struct {
		file *os.File
		name string
	}
	stack := []directory{{file: os.NewFile(uintptr(fd), name), name: name}}
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
			fd := int(parent.Fd())
			if len(stack) > 0 {
				fd = int(stack[len(stack)-1].file.Fd())
			}
			if err := unix.Unlinkat(fd, name, unix.AT_REMOVEDIR); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		entry := entries[0]
		if entry.IsDir() {
			fd, err := unix.Openat(int(dir.file.Fd()), entry.Name(), frameDirectoryFlags, 0)
			if err != nil {
				return err
			}
			stack = append(stack, directory{file: os.NewFile(uintptr(fd), entry.Name()), name: entry.Name()})
			continue
		}
		if err := unix.Unlinkat(int(dir.file.Fd()), entry.Name(), 0); err != nil {
			return err
		}
	}
	return nil
}
