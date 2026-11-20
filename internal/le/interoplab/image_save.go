// Design: docs/contributing/preview-artifacts.md -- portable local images.
// Related: docker_image_cache.go -- the image cache saves and loads through the same bound.
package interoplab

import (
	"context"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"time"
)

// dockerImageArchiveTimeout bounds one `docker image save` or `docker image
// load`: an archive of a large lab image is a few gigabytes written or read
// on the docker CLI's side.
const dockerImageArchiveTimeout = 10 * time.Minute

// SaveImage exports the immutable image inspected by a build tool. The caller
// owns a new output path; no mutable tag is accepted at this boundary.
func (d *Docker) SaveImage(ctx context.Context, reference, output string) error {
	if !immutableImageID(reference) {
		return errors.New("image export requires an immutable sha256 ID")
	}
	if !filepath.IsAbs(output) {
		return errors.New("image export requires an absolute output path")
	}
	_, err := d.command(ctx, dockerImageArchiveTimeout, DockerExecutable, "image", "save", "--output", output, reference)
	return err
}

// immutableImageID reports whether reference is a content-addressed image ID,
// `sha256:` and 64 hexadecimal digits, rather than a tag another build can move.
func immutableImageID(reference string) bool {
	digest, valid := strings.CutPrefix(reference, "sha256:")
	if !valid || len(digest) != 64 {
		return false
	}
	_, err := hex.DecodeString(digest)
	return err == nil
}
