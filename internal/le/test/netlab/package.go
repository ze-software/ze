// Design: docs/guide/netlab.md -- a lab that carries its own daemon definition.
package testnetlab

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Package creates a new, self-contained lab directory without requiring netlab
// on the exporting machine. An existing destination belongs to its operator.
func (c *Checker) Package(destination string) (err error) {
	if destination == "" {
		return errors.New("package destination is required")
	}
	if err := os.Mkdir(destination, 0o755); err != nil {
		return fmt.Errorf("create new lab directory %s: %w", destination, err)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, c.FS.RemoveAll(destination))
		}
	}()
	if err := c.buildLab(destination); err != nil {
		return err
	}
	for _, name := range []string{"Dockerfile.tools", "README.md"} {
		if err := c.FS.CopyFile(filepath.Join(c.Root, "contrib", "netlab", name), filepath.Join(destination, name)); err != nil {
			return fmt.Errorf("package %s: %w", name, err)
		}
	}
	return nil
}
