// Design: docs/architecture/testing/interop.md -- Raw frame injection from inside a peer container.
// Related: bgp/isis_inject.go -- the IS-IS own-LSP purge injected as Ze.
// Related: pppoe/check_padr_replay.go -- replaying a captured PADR as the client.
// Related: pppoe/check_pap.go -- replaying a PAP Authenticate-Request as the client.
package interoplab

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/pcap"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// replayCapturePath is where SendFrameInContainer writes its one-frame
// capture inside the peer container, before tcpreplay reads it back.
const replayCapturePath = "/tmp/ze-interop-replay.pcap"

// FrameSender puts one Ethernet frame on interfaceName inside the named peer
// container. SendFrameInContainer is the production sender; the type exists
// so a unit test can substitute a fake that scripts the peer's answer.
type FrameSender func(ctx context.Context, lab CheckerLab, peer, interfaceName string, frame []byte) error

// SendFrameInContainer sends frame on interfaceName of the named peer container
// as though the container itself had sent it. It writes the frame as a
// one-record pcap inside the container and runs tcpreplay there, so the image
// MUST carry tcpreplay.
//
// The send runs inside the container rather than from the checker process,
// because a Docker container's network namespace belongs to root on a rootful
// daemon: an unprivileged checker can neither open /proc/<pid>/ns/net (the
// kernel's ptrace access check refuses another user's process) nor setns into
// it (that needs CAP_SYS_ADMIN). `docker exec` already runs in the namespace.
func SendFrameInContainer(ctx context.Context, lab CheckerLab, peer, interfaceName string, frame []byte) error {
	if lab == nil {
		return errors.New("frame sender has no lab")
	}
	if len(frame) == 0 {
		return errors.New("frame to send is empty")
	}
	if interfaceName == "" {
		return errors.New("frame sender has no interface name")
	}

	var capture bytes.Buffer
	if err := pcap.WriteFileHeader(&capture, uint32(len(frame)), pcap.LinkTypeEthernet); err != nil { //nolint:gosec // one Ethernet frame, far below 4 GiB
		return fmt.Errorf("build replay capture: %w", err)
	}
	if err := pcap.WriteRecord(&capture, time.Unix(0, 0), frame, len(frame)); err != nil {
		return fmt.Errorf("build replay capture: %w", err)
	}

	var tb textbuf.Buffer
	shell := tb.Str("echo ").Str(base64.StdEncoding.EncodeToString(capture.Bytes())).
		Str(" | base64 -d > ").Str(replayCapturePath).
		Str(" && tcpreplay -q -i ").Str(interfaceName).Str(" ").Str(replayCapturePath).String()
	result, err := lab.Exec(ctx, peer, []string{"sh", "-c", shell}, nil)
	if err != nil {
		return fmt.Errorf("replay frame in %s: %w", peer, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("tcpreplay in %s exited %d: %s", peer, result.ExitCode, strings.TrimSpace(result.Stderr))
	}
	return nil
}
