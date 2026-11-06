// Design: docs/architecture/testing/interop.md -- actual consumed image provenance.
package interoplab

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// PeerImage observes the image ID consumed by the registered peer's container.
// Its declared image tag is never a fallback for an unavailable observation.
func (l *Lab) PeerImage(ctx context.Context, peer string) (string, error) {
	config, err := l.peer(peer)
	if err != nil {
		return "", err
	}
	result, err := l.docker.command(ctx, dockerCommandTimeout,
		DockerExecutable, dockerInspectVerb, dockerFormatFlag, "{{.Image}}", config.Container)
	if err != nil {
		return "", err
	}
	identity := strings.TrimSpace(result.Stdout)
	const prefix = "sha256:"
	if !strings.HasPrefix(identity, prefix) {
		return "", errors.New("container inspect returned no sha256 image ID")
	}
	var digest [32]byte
	if len(identity) != len(prefix)+hex.EncodedLen(len(digest)) {
		return "", errors.New("container inspect returned an invalid image ID length")
	}
	if _, err := hex.Decode(digest[:], []byte(identity[len(prefix):])); err != nil {
		return "", fmt.Errorf("container inspect returned an invalid image ID: %w", err)
	}
	return identity, nil
}
