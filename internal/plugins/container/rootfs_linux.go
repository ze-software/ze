// Design: docs/architecture/container/runtime.md -- the container's root: an overlay of the image below the writable layer
// Related: library_linux.go -- the runtime library that mounts and unmounts the root

package container

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/opererr"
)

// The plugin's directories in the run-time tree, which a reboot empties. The
// runtime library keeps one state directory per container under
// libraryStateRoot, and each container's root is mounted under
// rootMountDirectory. Both are root-owned and private to the daemon.
const (
	runtimeDirectory   = "/run/ze-container"
	libraryStateRoot   = runtimeDirectory + "/state"
	rootMountDirectory = runtimeDirectory + "/root"
)

// The directories of an overlay inside the store's writable folder.
const (
	overlayUpper = "upper"
	overlayWork  = "work"
)

// mountRoot mounts the root of the named container: an overlay whose
// read-only lower layer is the image folder, and whose upper and work
// directories are in the container's writable folder, so what the container
// writes lands on disk and the image is never changed. It answers the mount
// point. The caller MUST call unmountRoot on it once the container is gone.
func mountRoot(name string, ready Ready) (string, error) {
	for _, folder := range []string{ready.ImageFolder, ready.WritableFolder} {
		if err := checkOverlayFolder(name, folder); err != nil {
			return "", err
		}
	}
	upper := filepath.Join(ready.WritableFolder, overlayUpper)
	work := filepath.Join(ready.WritableFolder, overlayWork)
	// The root of an overlay takes the mode of its upper directory, so the
	// upper directory takes the image root's: a container's / reads as its
	// image says, and a user the entrypoint switches to can still enter it.
	image, statErr := os.Stat(ready.ImageFolder)
	if statErr != nil {
		return "", rootFailed(name, statErr)
	}
	if err := os.MkdirAll(upper, image.Mode().Perm()); err != nil {
		return "", rootFailed(name, err)
	}
	// The work directory is the overlay's own, and nothing else reads it.
	if err := os.MkdirAll(work, 0o700); err != nil {
		return "", rootFailed(name, err)
	}
	target := filepath.Join(rootMountDirectory, name)
	if err := os.MkdirAll(target, 0o700); err != nil {
		return "", rootFailed(name, err)
	}
	options := "lowerdir=" + ready.ImageFolder + ",upperdir=" + upper + ",workdir=" + work
	if err := unix.Mount("overlay", target, "overlay", unix.MS_NODEV, options); err != nil {
		removeErr := os.Remove(target)
		return "", errors.Join(rootFailed(name, err), removeErr)
	}
	return target, nil
}

// unmountRoot unmounts a root mountRoot mounted and removes its mount point.
func unmountRoot(target string) error {
	if err := unix.Unmount(target, 0); err != nil {
		return err
	}
	return os.Remove(target)
}

// checkOverlayFolder refuses a store folder the overlay options cannot carry:
// a relative path, or one holding the separators of the option string.
func checkOverlayFolder(name, folder string) error {
	if !filepath.IsAbs(folder) {
		return rootFailed(name, errors.New("the store answered the relative folder "+folder))
	}
	if strings.ContainsAny(folder, ",:\\") {
		return rootFailed(name, errors.New("the store folder "+folder+" holds a comma, a colon or a backslash, which an overlay mount cannot name"))
	}
	return nil
}

func rootFailed(name string, err error) error {
	return opererr.Wrap(err,
		"container "+name+": its root could not be mounted",
		"the root is an overlay of the image below the container's writable layer, and without it the container has no files",
		"ze request container restart name "+name,
	)
}
