// Design: docs/architecture/cli/plugin-modes.md -- failed upgrade recovery.

package local

import (
	"errors"
	"io"
	"os"

	"github.com/ze-software/ze/cmd/ze/install"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// nativeReplacement pins the prior executable inode until installation ends.
// Its file is nil only for a fresh installation with no executable to restore.
type nativeReplacement struct {
	previous *os.File
	target   nativeTarget
	changed  bool
}

func captureNativeReplacement(target nativeTarget) (*nativeReplacement, error) {
	previous, err := os.Open(target.binary)
	if errors.Is(err, os.ErrNotExist) {
		return &nativeReplacement{target: target}, nil
	}
	if err != nil {
		return nil, err
	}
	return &nativeReplacement{previous: previous, target: target}, nil
}

func (replacement *nativeReplacement) close() error {
	if replacement.previous == nil {
		return nil
	}
	return replacement.previous.Close()
}

func (replacement *nativeReplacement) restore() error {
	if !replacement.changed {
		return nil
	}
	if replacement.previous == nil {
		return nil
	} // A fresh installation retains its binary for recovery.
	guard, err := storage.AcquireMaintenance(replacement.target.configDir)
	if err != nil {
		return err
	}
	if _, err := replacement.previous.Seek(0, io.SeekStart); err != nil {
		return errors.Join(err, guard.Close())
	}
	return errors.Join(copyInstalledBytes(replacement.previous, replacement.target.binary), guard.Close())
}

func (rt *nativeRuntime) recoverInstall(replacement *nativeReplacement, service install.NativeService, stopped install.StoppedService) error {
	if replacement.changed {
		if service != nil {
			if _, err := service.Stop(replacement.target.binary, replacement.target.configDir); err != nil {
				return err
			}
		}
	}
	if err := replacement.restore(); err != nil {
		return err
	}
	if stopped != nil {
		return stopped.Restore()
	}
	return nil
}
