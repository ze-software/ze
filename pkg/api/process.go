package api

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// Process represents an external subprocess.
type Process struct {
	config ProcessConfig
	cmd    *exec.Cmd

	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	// Buffered reader for stdout
	reader *bufio.Reader

	// Channel for lines read from stdout (single reader goroutine)
	lines chan string

	running atomic.Bool

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
}

// NewProcess creates a new process with the given configuration.
func NewProcess(config ProcessConfig) *Process {
	return &Process{
		config: config,
	}
}

// Running returns true if the process is running.
func (p *Process) Running() bool {
	return p.running.Load()
}

// Start spawns the process.
func (p *Process) Start() error {
	return p.StartWithContext(context.Background())
}

// StartWithContext spawns the process with the given context.
func (p *Process) StartWithContext(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.ctx, p.cancel = context.WithCancel(ctx)

	// Create command
	// #nosec G204 - Run command is from trusted configuration, not user input
	p.cmd = exec.CommandContext(p.ctx, "/bin/sh", "-c", p.config.Run)

	// Set working directory if specified
	if p.config.WorkDir != "" {
		p.cmd.Dir = p.config.WorkDir
	}

	// Set up pipes
	var err error
	p.stdin, err = p.cmd.StdinPipe()
	if err != nil {
		return err
	}

	p.stdout, err = p.cmd.StdoutPipe()
	if err != nil {
		_ = p.stdin.Close()
		return err
	}
	p.reader = bufio.NewReader(p.stdout)

	p.stderr, err = p.cmd.StderrPipe()
	if err != nil {
		_ = p.stdin.Close()
		_ = p.stdout.Close()
		return err
	}

	// Create new process group
	p.cmd.SysProcAttr = nil // Default is fine for now

	// Start process
	if err := p.cmd.Start(); err != nil {
		_ = p.stdin.Close()
		_ = p.stdout.Close()
		_ = p.stderr.Close()
		return err
	}

	p.running.Store(true)

	// Create channel for stdout lines
	p.lines = make(chan string, 100)

	// Start single reader goroutine for stdout
	go p.readLines()

	// Copy stderr to os.Stderr for debugging
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := p.stderr.Read(buf)
			if n > 0 {
				fmt.Fprintf(os.Stderr, "PROCESS STDERR: %s", string(buf[:n]))
			}
			if err != nil {
				break
			}
		}
	}()

	// Monitor process
	p.wg.Add(1)
	go p.monitor()

	return nil
}

// readLines continuously reads lines from stdout and sends to channel.
func (p *Process) readLines() {
	for {
		line, err := p.reader.ReadString('\n')
		if err != nil {
			close(p.lines)
			return
		}
		// Trim newline
		if len(line) > 0 && line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
		}
		p.lines <- line
	}
}

// Stop terminates the process.
func (p *Process) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
}

// Wait waits for the process to exit.
func (p *Process) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// WriteEvent writes an event to the process stdin.
func (p *Process) WriteEvent(event string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stdin == nil || !p.running.Load() {
		return os.ErrClosed
	}

	_, err := p.stdin.Write([]byte(event + "\n"))
	return err
}

// ReadCommand reads a command from the process stdout.
// Returns context.DeadlineExceeded on timeout, io.EOF when process exits.
func (p *Process) ReadCommand(ctx context.Context) (string, error) {
	select {
	case line, ok := <-p.lines:
		if !ok {
			return "", io.EOF
		}
		return line, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// monitor waits for the process to exit.
func (p *Process) monitor() {
	defer p.wg.Done()

	// Wait for process to exit
	_ = p.cmd.Wait()

	p.running.Store(false)

	// Close pipes
	p.mu.Lock()
	if p.stdin != nil {
		_ = p.stdin.Close()
	}
	if p.stdout != nil {
		_ = p.stdout.Close()
	}
	if p.stderr != nil {
		_ = p.stderr.Close()
	}
	p.mu.Unlock()
}

// ProcessManager manages multiple external processes.
type ProcessManager struct {
	configs   []ProcessConfig
	processes map[string]*Process

	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
}

// NewProcessManager creates a new process manager.
func NewProcessManager(configs []ProcessConfig) *ProcessManager {
	return &ProcessManager{
		configs:   configs,
		processes: make(map[string]*Process),
	}
}

// Start starts all configured processes.
func (pm *ProcessManager) Start() error {
	return pm.StartWithContext(context.Background())
}

// StartWithContext starts all configured processes with the given context.
func (pm *ProcessManager) StartWithContext(ctx context.Context) error {
	pm.ctx, pm.cancel = context.WithCancel(ctx)

	for _, cfg := range pm.configs {
		proc := NewProcess(cfg)
		if err := proc.StartWithContext(pm.ctx); err != nil {
			// Stop already started processes
			pm.Stop()
			return err
		}

		pm.mu.Lock()
		pm.processes[cfg.Name] = proc
		pm.mu.Unlock()
	}

	return nil
}

// Stop stops all processes.
func (pm *ProcessManager) Stop() {
	if pm.cancel != nil {
		pm.cancel()
	}

	pm.mu.Lock()
	for _, proc := range pm.processes {
		proc.Stop()
	}
	pm.mu.Unlock()

	// Wait for all processes
	pm.mu.RLock()
	for _, proc := range pm.processes {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = proc.Wait(ctx)
		cancel()
	}
	pm.mu.RUnlock()

	pm.mu.Lock()
	pm.processes = make(map[string]*Process)
	pm.mu.Unlock()
}

// Wait waits for all processes to stop.
func (pm *ProcessManager) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		pm.mu.RLock()
		for _, proc := range pm.processes {
			_ = proc.Wait(ctx)
		}
		pm.mu.RUnlock()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ProcessCount returns the number of running processes.
func (pm *ProcessManager) ProcessCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	count := 0
	for _, proc := range pm.processes {
		if proc.Running() {
			count++
		}
	}
	return count
}

// IsRunning returns true if the named process is running.
func (pm *ProcessManager) IsRunning(name string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	proc, ok := pm.processes[name]
	if !ok {
		return false
	}
	return proc.Running()
}
