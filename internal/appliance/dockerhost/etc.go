// Design: docs/architecture/appliance/docker-host.md -- the boot sequence before Lima's boot.sh
// Related: boot.go -- the sequence that makes /etc writable first

package dockerhost

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// etcSnapshotMax bounds the bytes mountWritableEtc holds in memory while it
// moves the squashfs /etc onto the tmpfs. gokrazy's own /etc is a handful of
// small files and symlinks; anything near this bound is not that tree.
const etcSnapshotMax = 1 << 20

// etcEntry is one entry of /etc as the squashfs holds it. Exactly one of a
// directory, a symlink (target set) or a regular file (data set) is recorded.
type etcEntry struct {
	rel    string
	mode   fs.FileMode
	target string
	data   []byte
}

// mountWritableEtc mounts a tmpfs over the read-only squashfs /etc and puts
// the entries gokrazy shipped there (hosts, the resolv.conf and localtime
// symlinks, ssl) back on it. The tmpfs hides the squashfs entries, so they
// are read first and written after.
func mountWritableEtc(sys System, root string) error {
	etc := guest(root, "/etc")
	entries, err := snapshotEtc(etc)
	if err != nil {
		return err
	}
	if err := sys.Mount(MountRequest{Source: fsTmpfs, Target: "/etc", FSType: fsTmpfs, Data: etcData}); err != nil {
		return fmt.Errorf("mount a tmpfs (%s) on /etc: %w", etcData, err)
	}
	for _, e := range entries {
		if err := restoreEtcEntry(etc, e); err != nil {
			return err
		}
	}
	return nil
}

// snapshotEtc reads every entry under etc, parents before children (WalkDir
// order), refusing a tree larger than etcSnapshotMax.
func snapshotEtc(etc string) ([]etcEntry, error) {
	var entries []etcEntry
	octets := 0
	walkErr := filepath.WalkDir(etc, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == etc {
			return nil
		}
		rel, err := filepath.Rel(etc, path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		e := etcEntry{rel: rel, mode: info.Mode()}
		if d.Type()&fs.ModeSymlink != 0 {
			e.target, err = os.Readlink(path)
			if err != nil {
				return err
			}
			entries = append(entries, e)
			return nil
		}
		if d.IsDir() {
			entries = append(entries, e)
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("%s is a %s, and only directories, files and symlinks are carried to the writable /etc", path, d.Type())
		}
		e.data, err = os.ReadFile(path) //nolint:gosec // path is under the guest's own /etc
		if err != nil {
			return err
		}
		octets += len(e.data)
		if octets > etcSnapshotMax {
			return fmt.Errorf("the squashfs /etc holds more than %d bytes, more than gokrazy ships there", etcSnapshotMax)
		}
		entries = append(entries, e)
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("read the squashfs /etc before mounting a tmpfs over it: %w", walkErr)
	}
	return entries, nil
}

// restoreEtcEntry writes one snapshot entry under etc. The fresh tmpfs is
// empty, so an entry already there means the mount did not hide the old
// tree; it is kept rather than overwritten.
func restoreEtcEntry(etc string, e etcEntry) error {
	path := filepath.Join(etc, e.rel)
	_, statErr := os.Lstat(path)
	if statErr == nil {
		return nil
	}
	if !errors.Is(statErr, fs.ErrNotExist) {
		return fmt.Errorf("restore /etc/%s: %w", e.rel, statErr)
	}
	if e.mode&fs.ModeSymlink != 0 {
		if err := os.Symlink(e.target, path); err != nil {
			return fmt.Errorf("restore /etc/%s: %w", e.rel, err)
		}
		return nil
	}
	if e.mode.IsDir() {
		if err := os.Mkdir(path, e.mode.Perm()); err != nil {
			return fmt.Errorf("restore /etc/%s: %w", e.rel, err)
		}
		return nil
	}
	if err := os.WriteFile(path, e.data, e.mode.Perm()); err != nil {
		return fmt.Errorf("restore /etc/%s: %w", e.rel, err)
	}
	return nil
}
