// Design: docs/contributing/gh-pages.md -- snapshot descriptors refuse aliases and nonregular files before reading bytes.
package site

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// publicationOpenFile acquires each path component from the previously opened
// directory without resolving symbolic links. os.Root.OpenFile itself can
// follow an alias after O_NOFOLLOW fails, so this boundary uses openat directly.
// The path/depth checks bound the loop, and every acquired descriptor is owned
// until it is closed or returned to the snapshot reader.
func publicationOpenFile(root *os.Root, name string) (*os.File, error) {
	if err := publicationPath(name); err != nil {
		return nil, err
	}
	parts := strings.Split(name, "/")
	if len(parts) > publicationDepthMax {
		return nil, errors.New("artifact file exceeds the path component limit")
	}
	directory, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	for _, part := range parts[:len(parts)-1] {
		fd, openErr := unix.Openat(int(directory.Fd()), part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		closeErr := directory.Close()
		if openErr != nil {
			return nil, errors.Join(openErr, closeErr)
		}
		if closeErr != nil {
			return nil, errors.Join(closeErr, unix.Close(fd))
		}
		directory = os.NewFile(uintptr(fd), part)
	}
	fd, openErr := unix.Openat(int(directory.Fd()), parts[len(parts)-1], unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	closeErr := directory.Close()
	if openErr != nil {
		return nil, errors.Join(openErr, closeErr)
	}
	if closeErr != nil {
		return nil, errors.Join(closeErr, unix.Close(fd))
	}
	file := os.NewFile(uintptr(fd), name)
	info, err := file.Stat()
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	if !info.Mode().IsRegular() {
		return nil, errors.Join(fmt.Errorf("artifact path %s is not a regular file", name), file.Close())
	}
	return file, nil
}
