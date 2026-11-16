// Design: docs/architecture/appliance/kernel-profiles.md -- Loading driver modules on the device: the device walk
// Overview: coldplug.go -- the cold-plug pass that matches these aliases

package coldplug

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// modaliasBytesMax bounds one modalias file: sysfs serves an attribute in one
// page, so a longer file is not a sysfs attribute.
const modaliasBytesMax = 4096

// deviceWalk is what one walk of the sysfs device tree found.
type deviceWalk struct {
	// aliases holds each distinct modalias, sorted, so the pass is the same
	// on every boot of the same hardware.
	aliases []string
	// unreadable counts the directories and modalias files the walk could
	// not read; firstUnreadable is the first of them, for the log line. A
	// device under an unreadable entry may need a module the pass never
	// learns about, so a count above zero is reported, never dropped.
	unreadable      int
	firstUnreadable error
}

// walkDevices reads every modalias file under <sysRoot>/devices. The kernel
// publishes one in each device directory that a driver module can bind.
// fs.WalkDir does not follow symbolic links, which matters here: sysfs links
// every device to its subsystem, its driver and its parent, so following them
// would loop.
func walkDevices(sysRoot string) (deviceWalk, error) {
	root := filepath.Join(sysRoot, "devices")
	if _, err := os.Stat(root); err != nil {
		return deviceWalk{}, err
	}
	var walk deviceWalk
	seen := make(map[string]bool)
	note := func(err error) {
		walk.unreadable++
		if walk.firstUnreadable == nil {
			walk.firstUnreadable = err
		}
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			note(err)
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.Name() != "modalias" {
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		alias, readErr := readModalias(path)
		if readErr != nil {
			note(readErr)
			return nil
		}
		if alias == "" {
			return nil
		}
		if !seen[alias] {
			seen[alias] = true
			walk.aliases = append(walk.aliases, alias)
		}
		return nil
	})
	if err != nil {
		return deviceWalk{}, err
	}
	slices.Sort(walk.aliases)
	return walk, nil
}

// readModalias reads one modalias attribute, without its trailing newline.
func readModalias(path string) (string, error) {
	file, err := os.Open(path) //nolint:gosec // a sysfs attribute under the device tree being walked
	if err != nil {
		return "", err
	}
	defer file.Close() //nolint:errcheck // read-only sysfs attribute; nothing to flush
	data, err := io.ReadAll(io.LimitReader(file, modaliasBytesMax+1))
	if err != nil {
		return "", err
	}
	if len(data) > modaliasBytesMax {
		return "", fmt.Errorf("%s is longer than the %d bytes a sysfs attribute holds", path, modaliasBytesMax)
	}
	return strings.TrimSpace(string(data)), nil
}
