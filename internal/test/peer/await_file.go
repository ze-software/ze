// Design: docs/architecture/testing/ci-format.md -- option=await-file
// Related: peer.go -- Config.AwaitFile, runMessageLoop calls awaitFile before any send
// Related: established_file_test.go -- option=established-file, the file another peer block writes

package peer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"
)

// awaitFileMax bounds the option=await-file wait. It covers the daemon bringing
// up a second session under a loaded verify run, and it stays well inside the
// 20s timeout a .ci peer block normally declares, so a missing file fails
// with this package's message rather than as a bare test timeout.
const awaitFileMax = 15 * time.Second

// awaitFilePoll is the interval between two checks for the awaited file.
const awaitFilePoll = 20 * time.Millisecond

// awaitFile blocks until Config.AwaitFile exists. It returns nil at once when
// no file was declared or the file is already there, and an error naming the
// file when awaitFileMax passes or ctx ends first.
func (p *Peer) awaitFile(ctx context.Context) error {
	if p.config.AwaitFile == "" {
		return nil
	}
	deadline := time.Now().Add(awaitFileMax)
	for {
		_, err := os.Stat(p.config.AwaitFile)
		if err == nil {
			p.printf("\nawaited %s, sending\n", p.config.AwaitFile)
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("await-file %q: %w", p.config.AwaitFile, err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("await-file %q did not appear within %s: the peer block that declares "+
				"option=established-file:path=%s never received the daemon's first UPDATE, so that session "+
				"never reached Established", p.config.AwaitFile, awaitFileMax, p.config.AwaitFile)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("await-file %q: %w", p.config.AwaitFile, ctx.Err())
		case <-time.After(awaitFilePoll):
		}
	}
}
