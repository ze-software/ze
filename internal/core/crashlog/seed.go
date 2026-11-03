// Design: docs/architecture/diagnostics/crash-capture.md -- confined short-lived init capture.
package crashlog

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/ze-software/ze/internal/core/env"
)

// seedCrash is immutable after initialization. os.File serializes writes and
// close. This sink is only for the admitted, short-lived session init process.
var seedCrash *os.File

// SeedCapture owns the root and file acquired by InitSeed. Its zero value is
// uninitialized. The caller MUST Close once after normal command completion,
// never during panic unwinding: main and the runtime still need the capture.
type SeedCapture struct {
	root   *os.Root
	file   *os.File
	name   string
	header int64
}

// InitSeed arms fatal capture under the accepted config root without probing
// any pathname or fallback directory. It MUST run before Init, once per child.
// The caller retains root; the capture owns a separate crash-directory root.
func InitSeed(root *os.Root) (*SeedCapture, error) {
	if root == nil {
		return nil, fmt.Errorf("session crash capture requires an inherited directory")
	}
	if err := root.Mkdir("crash", 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	info, err := root.Lstat("crash")
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("session crash entry is not a directory; remove the conflicting crash entry before seeding")
	}
	held, err := openSeedCrashRoot(root, info)
	if err != nil {
		return nil, err
	}
	name := crashFilePrefix + time.Now().UTC().Format("20060102-150405") + "-seed-" + rand.Text() + crashFileSuffix
	file, err := held.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, errors.Join(err, held.Close())
	}
	header := appendRuntimeHeader(nil)
	if _, err := file.Write(header); err != nil {
		return nil, errors.Join(err, file.Close(), held.Remove(name), held.Close())
	}
	initialized := false
	initOnce.Do(func() {
		initialized = true
		startTime = time.Now()
		origStderr = os.Stderr
		syslogAddr = env.Get("ze.log.destination")
		seedCrash = file
		env.SetFatalOutput(origStderr)
	})
	if !initialized {
		return nil, errors.Join(fmt.Errorf("crash capture was already initialized before session admission"), file.Close(), held.Remove(name), held.Close())
	}
	if err := debug.SetCrashOutput(file, debug.CrashOptions{}); err != nil {
		return nil, errors.Join(err, file.Close(), held.Remove(name), held.Close())
	}
	capture := &SeedCapture{root: held, file: file, name: name, header: int64(len(header))}
	if err := redirectStderr(syslogAddr, ""); err != nil {
		return nil, errors.Join(err, capture.Close())
	}
	return capture, nil
}

// openSeedCrashRoot acquires the directory inspected by InitSeed.
// The caller MUST close the returned root.
func openSeedCrashRoot(root *os.Root, entry os.FileInfo) (*os.Root, error) {
	held, err := root.OpenRoot("crash")
	if err != nil {
		return nil, err
	}
	actual, err := held.Stat(".")
	if err != nil {
		return nil, errors.Join(err, held.Close())
	}
	if !os.SameFile(entry, actual) {
		return nil, errors.Join(fmt.Errorf("session crash directory changed while opening; retry le session seed-store with this session's own directory"), held.Close())
	}
	return held, nil
}

// Close drains normal stderr and removes a header-only capture. A capture with
// panic output stays under the accepted root. No former pathname is resolved.
func (c *SeedCapture) Close() error {
	Flush()
	if err := debug.SetCrashOutput(nil, debug.CrashOptions{}); err != nil {
		return errors.Join(err, c.file.Close(), c.root.Close())
	}
	info, err := c.file.Stat()
	err = errors.Join(err, c.file.Close())
	if err == nil && info.Size() == c.header {
		err = c.root.Remove(c.name)
	}
	return errors.Join(err, c.root.Close())
}
