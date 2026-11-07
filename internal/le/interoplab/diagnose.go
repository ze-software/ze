// Design: docs/architecture/testing/interop.md -- what a failed scenario reports.
// Related: lab.go -- runScenario, which reads these before its teardown.
package interoplab

import (
	"context"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// peerLogLines bounds the log tail a failed scenario reports for each peer,
// so a suite of many scenarios still prints a report a reader can scroll.
const peerLogLines = 40

// containerRunning is the state `docker inspect` reports for a live container.
const containerRunning = "running"

// peerDiagnosticSource is what a failed scenario reads from each container it
// started: the container's state, its log tail and, while it runs, the
// devices and addresses its network namespace holds. *Docker serves it.
type peerDiagnosticSource interface {
	ContainerState(ctx context.Context, name string) (string, error)
	Logs(ctx context.Context, name string, lines int) (LogResult, error)
	Exec(ctx context.Context, name string, argv []string, environ []EnvironmentVariable) (CommandResult, error)
}

var _ peerDiagnosticSource = (*Docker)(nil)

// peerLinkProbe and peerAddressProbe are the namespace reads a running peer
// answers: every device with its kind and VLAN id, one per line, then every
// address in brief form. A link built on the wrong parent, or a daemon that
// never made its device, shows here.
var (
	peerLinkProbe    = []string{"ip", "-d", "-o", "link", "show"}
	peerAddressProbe = []string{"ip", "-br", "address", "show"}
)

// diagnosePeers answers one block per container this run started, in start
// order, so a red verdict carries the cause the checker could not see: a
// container that exited, the daemon's last log lines, a device that is
// missing or sits on the wrong VLAN. started holds the containers the run
// created; a peer whose container never started has nothing to read. Each
// read that fails is reported in place of its answer, never skipped.
func diagnosePeers(ctx context.Context, source peerDiagnosticSource, peers []PeerConfig, started []string) []string {
	blocks := make([]string, 0, len(started))
	for index := range peers {
		peer := &peers[index]
		if !slices.Contains(started, peer.Container) {
			continue
		}
		blocks = append(blocks, diagnosePeer(ctx, source, peer))
	}
	return blocks
}

// diagnosePeer renders one peer's block: its state, its log tail, and the
// namespace reads when the container still runs.
func diagnosePeer(ctx context.Context, source peerDiagnosticSource, peer *PeerConfig) string {
	var block textbuf.Buffer
	block.Str("peer ").Str(peer.Name).Str(" (container ").Str(peer.Container).Str("): ")
	state, err := source.ContainerState(ctx, peer.Container)
	if err != nil {
		block.Str("state unreadable: ").Err(err).Byte('\n')
	} else {
		block.Str("state ").Str(state).Byte('\n')
	}
	logs, err := source.Logs(ctx, peer.Container, peerLogLines)
	switch {
	case err != nil:
		block.Str("  log unreadable: ").Err(err).Byte('\n')
	case strings.TrimSpace(logs.Text) == "":
		block.Str("  log: empty\n")
	default:
		block.Str("  log (last ").Int(peerLogLines).Str(" lines):\n")
		indented(&block, logs.Text)
	}
	if !strings.HasPrefix(state, containerRunning) {
		block.Str("  links and addresses: not read, the container is not running\n")
		return block.String()
	}
	for _, probe := range [][]string{peerLinkProbe, peerAddressProbe} {
		block.Str("  ").Join(probe, " ").Str(":\n")
		answer, err := source.Exec(ctx, peer.Container, probe, nil)
		if err != nil {
			block.Str("    failed: ").Err(err).Byte('\n')
			continue
		}
		indented(&block, answer.Stdout)
	}
	return block.String()
}

// indented writes text into block, each line under four spaces.
func indented(block *textbuf.Buffer, text string) {
	for line := range strings.SplitSeq(strings.TrimRight(text, "\n"), "\n") {
		block.Str("    ").Str(line).Byte('\n')
	}
}

// ContainerState answers the container's state as `docker inspect` reports
// it, with its exit code and any runtime error, so a container that exited
// says how.
func (d *Docker) ContainerState(ctx context.Context, name string) (string, error) {
	result, err := d.command(ctx, dockerCommandTimeout, DockerExecutable, dockerInspectVerb, dockerFormatFlag,
		"{{.State.Status}} exit={{.State.ExitCode}} oom={{.State.OOMKilled}} {{.State.Error}}", name)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Stdout), nil
}
