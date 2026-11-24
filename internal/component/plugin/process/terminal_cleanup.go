// Design: docs/architecture/api/process-protocol.md -- original process cleanup outcome
// Related: process.go -- actual engine return, child reap and stderr join
package process

import (
	"crypto/rand"
	"log/slog"
	"net"
	"os/exec"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// terminalCleanup belongs to one Process. Its mutex serializes Stage 1
// admission, bounded cause capture and actual terminal completion. The zero
// value is an ordinary, undeclared lifetime and reserves nothing. A completed
// lifetime cannot admit ownership. Safe for concurrent use.
type terminalCleanup struct {
	mu            sync.Mutex
	owner         *report.ResourceWarning
	name          string
	cause         string
	streamFailure string
	ended         bool
}

// These shares leave room for the process name and terminal status in the
// original cause. A separate latest cause carries an incomplete stderr read.
const terminalCleanupCauseMax = 320
const terminalCleanupNameMax = 64
const terminalCleanupStatusMax = 64

func fitTerminalCleanupText(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	for limit > 0 {
		if utf8.RuneStart(value[limit]) {
			break
		}
		limit--
	}
	return value[:limit]
}

// CheckTerminalCleanupDeclaration checks the compiled promise without
// reserving capacity. A foreign external program owns its declaration
// regardless of whether a compiled registration happens to have the same name.
// Safe for concurrent use; this MUST precede accepted Stage 1 publication.
func (p *Process) CheckTerminalCleanupDeclaration(declared bool) error {
	if !p.config.IsCompiled() {
		return nil
	}
	compiled := registry.Lookup(p.registryName)
	if compiled == nil {
		if !declared {
			return nil // An ordinary process makes no cleanup promise.
		}
		return opererr.New("Plugin "+p.config.Name+" has no compiled cleanup declaration",
			"Ze cannot verify what its terminal exit confirms", "ze show warnings")
	}
	if compiled.ExitConfirmsCleanup != declared {
		return opererr.New("Plugin "+p.config.Name+" disagrees with its compiled cleanup declaration",
			"Ze cannot accept this process's resource cleanup promise", "ze show warnings")
	}
	return nil
}

// AdmitTerminalCleanup reserves the original process's diagnostic capacity.
// Internal Start MUST call this before RunEngine; external accepted Stage 1
// MUST call it before OnStarted. Completion MUST use finishTerminalCleanup
// after the actual engine return or child reap and stderr join. Safe for
// concurrent use. An ordinary declaration consumes no reservation.
func (p *Process) AdmitTerminalCleanup(declared bool) error {
	if err := p.CheckTerminalCleanupDeclaration(declared); err != nil {
		return err
	}
	if !declared {
		return nil
	}
	cleanup := &p.terminal
	cleanup.mu.Lock()
	defer cleanup.mu.Unlock()
	if cleanup.ended {
		return opererr.New("Plugin "+p.config.Name+" ended before cleanup admission",
			"The ended process cannot start native resource work", "ze show warnings")
	}
	if cleanup.owner != nil {
		return nil
	}
	owner, err := report.NewResourceWarning("plugin-process", "cleanup-"+rand.Text())
	if err != nil {
		return opererr.Wrap(err, "Plugin "+p.config.Name+" cannot reserve cleanup reporting",
			"Native work cannot start without space for its original failure", "ze show warnings")
	}
	cleanup.owner = owner
	cleanup.name = fitTerminalCleanupText(p.config.Name, terminalCleanupNameMax)
	return nil
}

// terminalCleanupConn preserves bridge discovery and binds the failure
// recorder to this exact runner. Connection closure is not cleanup proof.
type terminalCleanupConn struct {
	net.Conn
	bridge  *rpc.DirectBridge
	cleanup *terminalCleanup
}

func (conn *terminalCleanupConn) Bridge() *rpc.DirectBridge { return conn.bridge }

// ReportTerminalCleanupFailure retains the first bounded cleanup cause for
// this runner even after its RPC connection closes. The runner MUST call it
// before returning; the host publishes it at actual completion. This method
// never clears a reservation or confirms release. Safe for concurrent use.
func (conn *terminalCleanupConn) ReportTerminalCleanupFailure(cause string) error {
	return conn.cleanup.record(cause)
}

func (cleanup *terminalCleanup) record(cause string) error {
	if strings.TrimSpace(cause) == "" {
		return opererr.New("Plugin cleanup failure has no cause",
			"The operator cannot identify what resource remains", "ze show warnings")
	}
	cleanup.mu.Lock()
	defer cleanup.mu.Unlock()
	if cleanup.ended {
		return opererr.New("Plugin cleanup failure arrived after its terminal outcome",
			"A completed process cannot change its original resource report", "ze show warnings")
	}
	if cleanup.owner == nil {
		return opererr.New("Plugin did not declare terminal cleanup ownership",
			"The host has no original cleanup reservation for this runner", "ze show warnings")
	}
	if cleanup.cause == "" {
		cleanup.cause = fitTerminalCleanupText(cause, terminalCleanupCauseMax)
	}
	return nil
}

// captureTerminalStderr inspects structured failure metadata before the relay
// filter. It accepts one bounded cause only; no stderr line grants successful
// cleanup authority. The declaration is accepted before native startup, so
// pre-declaration output cannot publish or reserve ownership.
func (p *Process) captureTerminalStderr(line string) {
	level, message, attrs := slogutil.ParseLogLine(line)
	if level < slog.LevelWarn {
		return
	}
	var code, cause string
	// ParseLogLine bounds attributes by this scanner's single bounded line.
	for offset := 0; offset+1 < len(attrs); offset += 2 {
		key, ok := attrs[offset].(string)
		if !ok {
			continue
		}
		value, ok := attrs[offset+1].(string)
		if !ok {
			continue
		}
		if key == "code" {
			code = value
		}
		if key == "error" {
			cause = value
		}
	}
	if code != codeDoctorPluginCleanup {
		return
	}
	if cause == "" {
		cause = message
	}
	cleanup := &p.terminal
	cleanup.mu.Lock()
	defer cleanup.mu.Unlock()
	if cleanup.owner == nil {
		return
	}
	if cleanup.ended {
		return
	}
	if cleanup.cause == "" {
		cleanup.cause = fitTerminalCleanupText(cause, terminalCleanupCauseMax)
	}
}

func (p *Process) noteTerminalStderrFailure(err error) {
	if err == nil {
		return
	}
	p.terminal.mu.Lock()
	defer p.terminal.mu.Unlock()
	p.terminal.streamFailure = report.FitResourceWarningCause(err.Error())
}

// finishTerminalCleanup consumes only actual terminal evidence for this
// original process. The caller MUST join all stderr before an external call.
// A successful same-name replacement has another reservation and cannot
// reach this one. Unresolved warnings remain protected in the report store.
func (p *Process) finishTerminalCleanup(success bool, status string) {
	cleanup := &p.terminal
	cleanup.mu.Lock()
	defer cleanup.mu.Unlock()
	if cleanup.ended {
		return
	}
	cleanup.ended = true
	if cleanup.owner == nil {
		return
	}
	if success {
		if cleanup.cause == "" {
			if cleanup.streamFailure == "" {
				if err := cleanup.owner.Close(); err != nil {
					logger().Error("plugin cleanup reservation could not close", "plugin", cleanup.name,
						"error", err, "code", codeDoctorPluginCleanup)
				}
				return
			}
		}
	}
	var cause strings.Builder
	cause.WriteString("Plugin ")
	cause.WriteString(cleanup.name)
	cause.WriteString(" cleanup is unconfirmed (")
	cause.WriteString(fitTerminalCleanupText(status, terminalCleanupStatusMax))
	cause.WriteString("): ")
	if cleanup.cause == "" {
		cause.WriteString("no cleanup failure cause was received")
	} else {
		cause.WriteString(cleanup.cause)
	}
	episode, err := cleanup.owner.NewEpisode(codeDoctorPluginCleanup,
		report.FitResourceWarningCause(cause.String()), "ze show warnings", map[string]any{"process": p.config.Name})
	if err == nil {
		if cleanup.streamFailure != "" {
			err = episode.Update(report.FitResourceWarningCause("Final stderr is incomplete: " + cleanup.streamFailure))
		}
	}
	if err == nil {
		err = episode.Publish()
	}
	if err != nil {
		logger().Error("plugin cleanup warning could not publish", "plugin", cleanup.name,
			"error", err, "code", codeDoctorPluginCleanup)
	}
}

// actualForkCleanupSuccess refuses a missing native state and every Wait
// failure, even if a cause line claimed success. Reaping alone cannot prove
// arbitrary plugin resource cleanup; only its accepted declaration gives
// a genuine zero exit that meaning.
func actualForkCleanupSuccess(command *exec.Cmd, waitErr error) bool {
	if waitErr != nil {
		return false
	}
	if command.ProcessState == nil {
		return false
	}
	return command.ProcessState.Success()
}
