// Design: docs/architecture/testing/ci-format.md — child processes of one predecessor case
// Related: cmd_exabgp.go — the suite runner that starts and judges them

package cli

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// exaEvents is what the mock BGP server reports on its stdout while a case
// runs: the port it listened on, and each point in the script where the fixture
// asks the runner to signal ze. The mock is a separate process holding only the
// TCP session, so this stream is the only way it can ask for either.
type exaEvents struct {
	port   chan int
	signal chan string
}

type exaProcess struct {
	name   string
	cmd    *exec.Cmd
	stdout lockedBuffer
	stderr lockedBuffer
	done   chan struct{}
	mu     sync.Mutex
	err    error
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Append(s string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, _ = b.b.WriteString(s)
	_ = b.b.WriteByte('\n')
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

func startExaProcess(ctx context.Context, name, program string, args, env []string, events *exaEvents) (*exaProcess, error) {
	cmd := exec.CommandContext(ctx, program, args...) //nolint:gosec // program and args target repository-owned compatibility fixtures.
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	proc := &exaProcess{name: name, cmd: cmd, done: make(chan struct{})}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// The readers are joined BEFORE Wait, which is the ordering StdoutPipe
	// documents: Wait closes the pipe once it sees the command exit, so a read
	// still in flight loses whatever the child wrote last. The verdict is read
	// off this buffer, so a server that printed "successful" and exited zero was
	// reported as "did not report success" whenever Wait won that race, which is
	// a red saying nothing about the software under test. Both readers end on
	// the EOF the child's exit produces, so joining them cannot outlive it.
	copiers := &sync.WaitGroup{}
	copiers.Add(2)
	go copyExaOutputTracked(copiers, stdout, &proc.stdout, events)
	go copyExaOutputTracked(copiers, stderr, &proc.stderr, nil)
	go proc.reap(copiers, events)
	return proc, nil
}

// copyExaOutputTracked drains one pipe and reports that it reached EOF, which
// is what lets reap call Wait only once nothing is still reading.
func copyExaOutputTracked(copiers *sync.WaitGroup, r io.Reader, dst *lockedBuffer, events *exaEvents) {
	defer copiers.Done()
	copyExaOutput(r, dst, events)
}

// reap records the child's exit status once both pipes are drained, then wakes
// everything waiting on this process.
func (p *exaProcess) reap(copiers *sync.WaitGroup, events *exaEvents) {
	copiers.Wait()
	err := p.cmd.Wait()
	p.mu.Lock()
	p.err = err
	p.mu.Unlock()
	close(p.done)
	if events != nil {
		close(events.port)
		close(events.signal)
	}
}

func copyExaOutput(r io.Reader, dst *lockedBuffer, events *exaEvents) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		dst.Append(line)
		if events == nil {
			continue
		}
		if value, ok := strings.CutPrefix(line, "PORT "); ok {
			port, err := strconv.Atoi(strings.TrimSpace(value))
			if err == nil && port > 0 {
				select {
				case events.port <- port:
				default:
				}
			}
			continue
		}
		// `SIGNAL <name>` is the mock reporting that the script reached a signal
		// step. The buffer holds one slot for each config the fixture names, so
		// a full channel is a defect rather than a busy run: say so where the
		// failure output shows it, because the reload will now never happen.
		if value, ok := strings.CutPrefix(line, "SIGNAL "); ok {
			name := strings.TrimSpace(value)
			select {
			case events.signal <- name:
			default:
				dst.Append("runner: dropped SIGNAL " + name)
			}
		}
	}
}

func (p *exaProcess) Err() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

func (p *exaProcess) Running() bool {
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

func stopExaProcess(p *exaProcess) {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return
	}
	if !p.Running() {
		return
	}
	pid := p.cmd.Process.Pid
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	select {
	case <-p.done:
		return
	case <-time.After(500 * time.Millisecond):
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
	<-p.done
}

// deliverExaBGPReloads performs the reload a fixture's signal step asks for.
//
// The mock BGP server runs in a separate process from the ze under test and
// holds only the TCP session, so it cannot signal ze. It reports each signal
// step on its stdout instead, and this worker owns the answer: it writes the
// next migrated config over the path ze reads, then asks ze to reload it.
//
// The fixture names ExaBGP's reload signal, SIGUSR1. ze reloads on SIGHUP
// (handleSIGHUPReload, cmd/ze/hub/main_reload.go), so the name is TRANSLATED
// here, as the bridge translates every command a fixture sends. It is not an
// equivalence: ze does nothing at all on SIGUSR1.
//
// Overwriting the file is enough because stageSIGHUPCandidate reads
// os.ReadFile(configPath) and stores what it finds as the candidate version,
// which diskConfigLoaders then reads back before the active version and before
// the file itself.
//
// The signal goes to ze's own pid and NOT to the process group, because the
// group also holds the bridge's python scripts.
//
// Lifecycle goroutine (one for each running case): it ends when the server
// exits and reap closes signalCh.
func deliverExaBGPReloads(signalCh <-chan string, client *exaProcess, configs exabgpClientConfigs) {
	delivered := 0
	for name := range signalCh {
		if delivered >= len(configs.reloads) {
			client.stdout.Append("runner: " + name + " has no config left to reload")
			continue
		}
		if err := os.WriteFile(configs.path, []byte(configs.reloads[delivered]), 0o600); err != nil {
			client.stdout.Append("runner: " + name + ": " + err.Error())
			continue
		}
		delivered++
		if client.cmd.Process == nil {
			client.stdout.Append("runner: " + name + ": ze is not running")
			continue
		}
		if err := syscall.Kill(client.cmd.Process.Pid, syscall.SIGHUP); err != nil {
			client.stdout.Append("runner: " + name + ": " + err.Error())
		}
	}
}
