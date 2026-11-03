//go:build linux || darwin || freebsd || openbsd || netbsd

// Design: docs/contributing/running-commands.md -- inherited directory admission.
package seedinit

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func inheritedDirectory(label string) (*os.Root, *os.File, error) {
	var st unix.Stat_t
	if err := unix.Fstat(4, &st); err != nil {
		return nil, nil, fmt.Errorf("missing inherited session directory: %w; launch through le session seed-store", err)
	}
	if st.Mode&unix.S_IFMT != unix.S_IFDIR {
		return nil, nil, fmt.Errorf("descriptor 4 is not a directory; launch through le session seed-store")
	}
	if st.Uid != uint32(os.Geteuid()) {
		return nil, nil, fmt.Errorf("inherited directory belongs to another user; launch through le session seed-store")
	}
	if st.Mode&0o077 != 0 {
		return nil, nil, fmt.Errorf("inherited directory is not private; restrict the session directory to mode 0700")
	}
	unix.CloseOnExec(4)
	folder := os.NewFile(4, label)
	// Convert the capability, never its durable label. Descriptor spellings
	// stay here, outside ZeFS APIs and import intents.
	root, err := os.OpenRoot(inheritedDirectoryPath)
	if err != nil {
		return nil, nil, errors.Join(err, folder.Close())
	}
	actual, err := root.Stat(".")
	if err != nil {
		return nil, nil, errors.Join(err, root.Close(), folder.Close())
	}
	held, err := folder.Stat()
	if err != nil {
		return nil, nil, errors.Join(err, root.Close(), folder.Close())
	}
	if !os.SameFile(held, actual) {
		return nil, nil, errors.Join(fmt.Errorf("inherited directory conversion changed identity; check the host descriptor filesystem before running le session seed-store"), root.Close(), folder.Close())
	}
	return root, folder, nil
}
