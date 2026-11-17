// Design: docs/architecture/container/runtime.md -- the description contribution: what a participant may mount
// Related: participant.go -- FileMount, which a participant contributes
// Related: supervise.go -- run, which judges each contribution before the create
// Related: describe_linux.go -- the description, which mounts what was admitted

package container

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// runtimeMountPoints are the container paths under which the runtime mounts
// a filesystem of its own (describe_linux.go, defaultMounts). A contributed
// mount at or under one would hide what the runtime mounts there, or be
// hidden by it, so it is refused. TestRuntimeMountPointsCoverTheDefaultMounts
// holds this list to the description's.
var runtimeMountPoints = []string{"/dev", "/proc", "/sys"}

// checkFileMount judges one mount a participant contributed, beside the
// mounts already admitted for the same container. It admits a regular file
// of the host, named by a clean absolute path, mounted at a clean absolute
// path of the container that is not the root, lies under no path the runtime
// mounts and no other contribution names. A symbolic link is refused: what it
// names could change between this check and the mount.
func checkFileMount(mount FileMount, admitted []FileMount) error {
	target := mount.ContainerPath
	if err := checkMountPath("the container path", target); err != nil {
		return err
	}
	if target == "/" {
		return errors.New("the container path is the container's root, which is not a file")
	}
	for _, point := range runtimeMountPoints {
		if isAtOrUnder(target, point) {
			return errors.New("the container path " + strconv.Quote(target) + " lies under " + point + ", which the runtime mounts itself")
		}
	}
	for _, other := range admitted {
		if other.ContainerPath == target {
			return errors.New("the container path " + strconv.Quote(target) + " is contributed twice")
		}
	}
	if err := checkMountPath("the host path", mount.HostPath); err != nil {
		return err
	}
	info, err := os.Lstat(mount.HostPath)
	if err != nil {
		return errors.New("the host path " + strconv.Quote(mount.HostPath) + " cannot be read: " + err.Error())
	}
	if !info.Mode().IsRegular() {
		return errors.New("the host path " + strconv.Quote(mount.HostPath) + " is not a regular file")
	}
	return nil
}

// checkMountPath refuses a path that is not absolute or not in clean form:
// a relative path, a trailing separator, a doubled separator, a dot or a
// dot-dot element.
func checkMountPath(which, path string) error {
	if !filepath.IsAbs(path) {
		return errors.New(which + " " + strconv.Quote(path) + " is not absolute")
	}
	if filepath.Clean(path) != path {
		return errors.New(which + " " + strconv.Quote(path) + " is not in clean form")
	}
	return nil
}

// isAtOrUnder answers whether path is point or lies under it. Both are clean
// absolute paths.
func isAtOrUnder(path, point string) bool {
	if path == point {
		return true
	}
	return strings.HasPrefix(path, point+"/")
}
