// Design: docs/contributing/preview-artifacts.md -- portable local images.
package interoplab

import (
	"context"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"time"
)

// SaveImage exports the immutable image inspected by a build tool. The caller
// owns a new output path; no mutable tag is accepted at this boundary.
func (d *Docker) SaveImage(ctx context.Context, reference, output string) error {
	digest, valid := strings.CutPrefix(reference, "sha256:")
	if !valid || len(digest) != 64 {
		return errors.New("image export requires an immutable sha256 ID")
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return err
	}
	if !filepath.IsAbs(output) {
		return errors.New("image export requires an absolute output path")
	}
	_, err := d.command(ctx, 10*time.Minute, DockerExecutable, "image", "save", "--output", output, reference)
	return err
}
