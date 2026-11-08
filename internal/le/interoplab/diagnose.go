// Design: docs/architecture/testing/interop.md -- what a failed scenario reports.
// Related: lab.go -- runScenario, which reads these before its teardown.
package interoplab

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// peerLogLines bounds the tail of each log file a failed scenario reports for a
// peer (logFileProbe), so a suite of many scenarios still prints a report a
// reader can scroll.
const peerLogLines = 40

// The bounds of the container log a failed scenario reports for each peer
// (writePeerLog). A daemon that exits at startup prints its first error near the
// top and a cascade of follow-on failures after it, so a tail alone can hide the
// cause: a log of up to peerLogHeadLines+peerLogTailLines lines is printed whole,
// and a longer one as its first lines, the error lines among those omitted (up to
// peerLogOmittedErrorsMax), and its last lines.
const (
	peerLogHeadLines        = 40
	peerLogTailLines        = 40
	peerLogOmittedErrorsMax = 20
)

// containerRunning is the state `docker inspect` reports for a live container.
const containerRunning = "running"

// peerDiagnosticSource is what a failed scenario reads from each container it
// started: the container's state, its log and, while it runs, the
// devices and addresses its network namespace holds. *Docker serves it.
type peerDiagnosticSource interface {
	ContainerState(ctx context.Context, name string) (string, error)
	LogsWhole(ctx context.Context, name string) (LogResult, error)
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
// container that exited, the daemon's log with its first lines, a device that is
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

// diagnosePeer renders one peer's block: its state, its log (writePeerLog), and, when
// the container still runs, the namespace reads and the tail of each log file
// the peer declared.
func diagnosePeer(ctx context.Context, source peerDiagnosticSource, peer *PeerConfig) string {
	var block textbuf.Buffer
	block.Str("peer ").Str(peer.Name).Str(" (container ").Str(peer.Container).Str("): ")
	state, err := source.ContainerState(ctx, peer.Container)
	if err != nil {
		block.Str("state unreadable: ").Err(err).Byte('\n')
	} else {
		block.Str("state ").Str(state).Byte('\n')
	}
	logs, err := source.LogsWhole(ctx, peer.Container)
	switch {
	case err != nil:
		block.Str("  log unreadable: ").Err(err).Byte('\n')
	case strings.TrimSpace(logs.Text) == "":
		block.Str("  log: empty\n")
	default:
		writePeerLog(&block, logs.Text)
	}
	if !strings.HasPrefix(state, containerRunning) {
		block.Str("  links, addresses and log files: not read, the container is not running\n")
		return block.String()
	}
	probes := make([][]string, 0, 2+len(peer.LogFiles))
	probes = append(probes, peerLinkProbe, peerAddressProbe)
	for _, file := range peer.LogFiles {
		probes = append(probes, logFileProbe(file))
	}
	for _, probe := range probes {
		block.Str("  ").Join(probe, " ").Str(":\n")
		answer, err := source.Exec(ctx, peer.Container, probe, nil)
		if err != nil {
			block.Str("    failed: ").Err(err).Byte('\n')
			continue
		}
		if strings.TrimSpace(answer.Stdout) == "" {
			block.Str("    (empty)\n")
			continue
		}
		indented(&block, answer.Stdout)
	}
	return block.String()
}

// writePeerLog writes a container log into block: whole when it holds at most
// peerLogHeadLines+peerLogTailLines lines, otherwise its first and last lines with
// the error lines found between them, each part under a heading that says what was
// cut, so the first error a daemon printed is never hidden behind its tail.
func writePeerLog(block *textbuf.Buffer, text string) {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	total := len(lines)
	if total <= peerLogHeadLines+peerLogTailLines {
		block.Str("  log (all ").Int(int64(total)).Str(" lines):\n")
		indentedLines(block, lines)
		return
	}
	omitted := lines[peerLogHeadLines : total-peerLogTailLines]
	block.Str("  log (").Int(int64(total)).Str(" lines), first ").Int(peerLogHeadLines).Str(" lines:\n")
	indentedLines(block, lines[:peerLogHeadLines])
	errorLines := make([]string, 0, peerLogOmittedErrorsMax)
	errorsTotal := 0
	for _, line := range omitted {
		if !isErrorLogLine(line) {
			continue
		}
		errorsTotal++
		if len(errorLines) < peerLogOmittedErrorsMax {
			errorLines = append(errorLines, line)
		}
	}
	block.Str("  log, the ").Int(int64(len(omitted))).Str(" lines between not shown")
	if errorsTotal == 0 {
		block.Str(", none of them an error line\n")
	} else {
		block.Str(", the first ").Int(int64(len(errorLines))).Str(" of their ").
			Int(int64(errorsTotal)).Str(" error lines:\n")
		indentedLines(block, errorLines)
	}
	block.Str("  log, last ").Int(peerLogTailLines).Str(" lines:\n")
	indentedLines(block, lines[total-peerLogTailLines:])
}

// isErrorLogLine answers whether a log line reports an error: Ze's slog record at
// ERROR, the `error:` line ze prints before it exits, or a Go panic. Other daemons'
// error lines that match none of these stay in the omitted count, never dropped
// from it.
func isErrorLogLine(line string) bool {
	if strings.Contains(line, "level=ERROR") {
		return true
	}
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "error:") {
		return true
	}
	return strings.HasPrefix(trimmed, "panic:")
}

// logFileProbe reads the last peerLogLines lines of a log file the peer
// declared (PeerConfig.LogFiles). tail answers non-zero for a file the
// daemon never created, which the block reports as a failed read.
func logFileProbe(file string) []string {
	return []string{"tail", "-n", strconv.Itoa(peerLogLines), file}
}

// indented writes text into block, each line under four spaces.
func indented(block *textbuf.Buffer, text string) {
	for line := range strings.SplitSeq(strings.TrimRight(text, "\n"), "\n") {
		block.Str("    ").Str(line).Byte('\n')
	}
}

// indentedLines writes each line into block under four spaces.
func indentedLines(block *textbuf.Buffer, lines []string) {
	for _, line := range lines {
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
