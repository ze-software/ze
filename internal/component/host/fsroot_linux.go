// Design: plan/spec-host-0-inventory.md — hardware inventory detection

//go:build linux

package host

import (
	"errors"
	"io/fs"
	"path/filepath"
)

// recordSysfsErr appends a DetectError to errs when err represents a
// real failure the operator should see. Absence (fs.ErrNotExist) and
// nil are silently ignored — the kernel simply doesn't expose that
// field on this box, and AC-21 requires us to omit rather than
// report. Permission-denied and every other error surface so the
// operator knows which path they need read access to.
func recordSysfsErr(errs *[]DetectError, path string, err error) {
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		return
	}
	*errs = append(*errs, DetectError{Path: path, Err: err.Error()})
}

// sysfsPath joins path components under Root + "/sys". Used by every
// Linux section detector to build sysfs paths that are testable by
// redirecting Root at a fixture tree.
func (d *Detector) sysfsPath(parts ...string) string {
	return filepath.Join(append([]string{d.root(), "sys"}, parts...)...)
}

// procPath joins path components under Root + "/proc".
func (d *Detector) procPath(parts ...string) string {
	return filepath.Join(append([]string{d.root(), "proc"}, parts...)...)
}

// root normalises empty Root to "/". Kept Linux-only because non-Linux
// detectors return ErrUnsupported before touching the filesystem, and
// keeping the method in this build-tagged file avoids an unused-method
// warning on darwin builds.
func (d *Detector) root() string {
	if d.Root == "" {
		return "/"
	}
	return d.Root
}
