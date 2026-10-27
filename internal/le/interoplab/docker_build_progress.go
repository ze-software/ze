// Design: docs/architecture/testing/interop.md -- immutable images and bounded build diagnostics.
// Related: docker.go -- Build selects streaming only when Progress is supplied.
package interoplab

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/le/le/tail"
)

// buildWithProgress reads identity from the unique tag, never from build output
// or the mutable cache tag. Docker's image inspect Id is content-addressable:
// https://github.com/moby/moby/blob/master/api/types/image/image_inspect.go.
// A failed streamed build releases its own tag even if cancellation raced export.
func (d *Docker) buildWithProgress(ctx context.Context, image ImageResult, command ProcessCommand, progress io.Writer) (ImageResult, error) {
	if d.endpointRefusal != nil {
		return image, d.endpointRefusal
	}
	output := &buildProgressWriter{output: progress}
	stdout := tail.New(outputExcerptBytesMax)
	stderr := tail.New(outputExcerptBytesMax)
	command.Stdout = io.MultiWriter(stdout, output)
	command.Stderr = io.MultiWriter(stderr, output)
	result, err := d.run(ctx, command)
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()
	_, err = commandOutcome(result, err, command.Timeout, commandOperation(command.Arguments), command.Arguments)
	output.mu.Lock()
	err = errors.Join(err, output.err)
	output.mu.Unlock()
	if err == nil {
		image.Reference, err = d.builtImageReference(ctx, image.retainedTag)
	}
	if err != nil {
		if cleanupErr := d.ReleaseImage(context.WithoutCancel(ctx), image); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("release failed build's owned tag: %w", cleanupErr))
		}
		return image, err
	}
	image.Built = true
	return image, nil
}

// builtImageReference asks the daemon for the ID while the build's unique tag
// holds it. An unreadable answer is an error, never a fallback to the shared tag.
func (d *Docker) builtImageReference(ctx context.Context, retainedTag string) (string, error) {
	result, err := d.command(ctx, dockerCommandTimeout,
		DockerExecutable, dockerImageObject, dockerInspectVerb, dockerFormatFlag, "{{.Id}}", retainedTag)
	if err != nil {
		return "", err
	}
	reference := strings.TrimSpace(result.Stdout)
	const prefix = "sha256:"
	if !strings.HasPrefix(reference, prefix) {
		return "", errors.New("docker image inspect returned no sha256 image ID for the build's owned tag")
	}
	var digest [32]byte
	if len(reference) != len(prefix)+hex.EncodedLen(len(digest)) {
		return "", errors.New("docker image inspect returned an invalid image ID length for the build's owned tag")
	}
	if _, err := hex.Decode(digest[:], []byte(reference[len(prefix):])); err != nil {
		return "", fmt.Errorf("docker image inspect returned an invalid image ID for the build's owned tag: %w", err)
	}
	return reference, nil
}

// buildProgressWriter serializes the process runner's two pipe-copy goroutines.
// Its zero value is unused: buildWithProgress supplies the non-nil destination.
// Safe for concurrent use; the destination MUST be used only through this writer
// until Build returns. The separate diagnostic tails have one producer each.
type buildProgressWriter struct {
	mu     sync.Mutex
	output io.Writer
	err    error
}

func (w *buildProgressWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	count, err := w.output.Write(data)
	if err == nil && count < len(data) {
		err = io.ErrShortWrite
	}
	if w.err == nil {
		w.err = err
	}
	return count, err
}
