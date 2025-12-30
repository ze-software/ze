package api

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// Backpressure constants matching ExaBGP behavior.
// ExaBGP: WRITE_QUEUE_HIGH_WATER=1000, WRITE_QUEUE_LOW_WATER=100.
const (
	// WriteQueueHighWater is the maximum items before dropping events.
	WriteQueueHighWater = 1000

	// WriteQueueLowWater is the threshold to resume after backpressure.
	// Reserved for future hysteresis support (pause at high, resume at low).
	// Currently unused - we use simple drop-when-full semantics.
	WriteQueueLowWater = 100 //nolint:unused // Reserved for future hysteresis

	// RespawnLimit is max respawns per RespawnWindow before disabling.
	// ExaBGP: respawn_number=5 per ~63 seconds.
	RespawnLimit = 5

	// RespawnWindow is the time window for respawn limit tracking.
	RespawnWindow = 60 * time.Second
)

// Respawn errors.
var (
	ErrRespawnLimitExceeded = errors.New("respawn limit exceeded")
	ErrProcessDisabled      = errors.New("process disabled due to respawn limit")
	ErrProcessNotFound      = errors.New("process not found")
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

	// Write queue for backpressure management
	writeQueue chan []byte

	// Backpressure stats
	queueDropped atomic.Uint64 // Total events dropped due to full queue
	warnedOnce   atomic.Bool   // Whether we've warned about backpressure

	running atomic.Bool

	// Session state (per-process API connection state)
	ackEnabled  atomic.Bool // Whether to send "done" responses (default: true)
	syncEnabled atomic.Bool // Whether to wait for wire transmission (default: false)

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
}

// NewProcess creates a new process with the given configuration.
func NewProcess(config ProcessConfig) *Process {
	p := &Process{
		config: config,
	}
	// Default: ack enabled, sync disabled
	p.ackEnabled.Store(true)
	return p
}

// Running returns true if the process is running.
func (p *Process) Running() bool {
	return p.running.Load()
}

// AckEnabled returns true if ACK responses are enabled for this process.
// When enabled (default), "done" responses are sent after commands.
func (p *Process) AckEnabled() bool {
	return p.ackEnabled.Load()
}

// SetAck enables or disables ACK responses for this process.
func (p *Process) SetAck(enabled bool) {
	p.ackEnabled.Store(enabled)
}

// SyncEnabled returns true if sync mode is enabled for this process.
// When enabled, announce/withdraw waits for wire transmission before ACK.
func (p *Process) SyncEnabled() bool {
	return p.syncEnabled.Load()
}

// SetSync enables or disables sync mode for this process.
func (p *Process) SetSync(enabled bool) {
	p.syncEnabled.Store(enabled)
}

// QueueSize returns the current number of items in the write queue.
func (p *Process) QueueSize() int {
	if p.writeQueue == nil {
		return 0
	}
	return len(p.writeQueue)
}

// QueueDropped returns the total number of events dropped due to backpressure.
func (p *Process) QueueDropped() uint64 {
	return p.queueDropped.Load()
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

	// Create write queue for backpressure management
	p.writeQueue = make(chan []byte, WriteQueueHighWater)

	// Start single reader goroutine for stdout
	go p.readLines()

	// Start write loop goroutine for backpressure management
	go p.writeLoop()

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

// WriteEvent writes an event to the process stdin via the write queue.
// Uses non-blocking send with backpressure - events are dropped if queue is full.
func (p *Process) WriteEvent(event string) error {
	if !p.running.Load() || p.writeQueue == nil {
		return os.ErrClosed
	}

	data := []byte(event + "\n")

	// Handle potential panic from sending to closed channel.
	// Race: monitor() can close writeQueue between running check and send.
	defer func() {
		if r := recover(); r != nil {
			// Channel was closed during send - process shutting down
			p.queueDropped.Add(1)
		}
	}()

	// Non-blocking send with backpressure
	select {
	case p.writeQueue <- data:
		return nil
	default:
		// Queue full - drop event and increment counter
		if p.queueDropped.Add(1) == 1 {
			// First drop - log warning once
			if p.warnedOnce.CompareAndSwap(false, true) {
				fmt.Fprintf(os.Stderr, "PROCESS %s: backpressure active, dropping events (queue full)\n", p.config.Name)
			}
		}
		return nil // Don't return error to avoid blocking caller
	}
}

// writeLoop drains the write queue and writes to stdin.
// Stops on write error (EPIPE indicates process died).
func (p *Process) writeLoop() {
	for data := range p.writeQueue {
		p.mu.Lock()
		if p.stdin == nil || !p.running.Load() {
			p.mu.Unlock()
			continue
		}
		_, err := p.stdin.Write(data)
		p.mu.Unlock()

		if err != nil {
			// Write failed (EPIPE, closed pipe, etc.) - process likely dead
			// Stop processing queue; monitor() will handle cleanup
			p.running.Store(false)
			return
		}
	}
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

	// Close write queue to stop writeLoop goroutine
	if p.writeQueue != nil {
		close(p.writeQueue)
	}

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

	// Respawn tracking: name -> list of respawn timestamps
	respawnTimes map[string][]time.Time

	// Disabled processes (respawn limit exceeded)
	disabled map[string]bool

	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
}

// ProcessWriter is the interface for writing events to a process.
// Used for testing with mock implementations.
type ProcessWriter interface {
	WriteEvent(data string) error
}

// Ensure Process implements ProcessWriter.
var _ ProcessWriter = (*Process)(nil)

// NewProcessManager creates a new process manager.
func NewProcessManager(configs []ProcessConfig) *ProcessManager {
	return &ProcessManager{
		configs:      configs,
		processes:    make(map[string]*Process),
		respawnTimes: make(map[string][]time.Time),
		disabled:     make(map[string]bool),
	}
}

// GetProcess returns a process by name, or nil if not found.
func (pm *ProcessManager) GetProcess(name string) *Process {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.processes[name]
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

// IsDisabled returns true if the named process is disabled due to respawn limit.
func (pm *ProcessManager) IsDisabled(name string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.disabled[name]
}

// Respawn restarts a process, enforcing respawn limits.
// Returns ErrRespawnLimitExceeded if limit exceeded within window.
// Returns ErrProcessDisabled if process was previously disabled.
// Returns ErrProcessNotFound if process name not in configuration.
// Returns error if ProcessManager was not started (ctx is nil).
func (pm *ProcessManager) Respawn(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check if ProcessManager was started
	if pm.ctx == nil {
		return errors.New("process manager not started")
	}

	// Check if already disabled
	if pm.disabled[name] {
		return ErrProcessDisabled
	}

	// Find config
	var cfg *ProcessConfig
	for i := range pm.configs {
		if pm.configs[i].Name == name {
			cfg = &pm.configs[i]
			break
		}
	}
	if cfg == nil {
		return ErrProcessNotFound
	}

	// Check respawn enabled
	if !cfg.RespawnEnabled && !cfg.Respawn {
		return nil // Respawn not enabled, nothing to do
	}

	now := time.Now()

	// Clean up old respawn times (outside window)
	var validTimes []time.Time
	for _, t := range pm.respawnTimes[name] {
		if now.Sub(t) < RespawnWindow {
			validTimes = append(validTimes, t)
		}
	}

	// Check limit
	if len(validTimes) >= RespawnLimit {
		pm.disabled[name] = true
		fmt.Fprintf(os.Stderr, "PROCESS %s: respawn limit exceeded (%d in %v), process disabled\n",
			name, RespawnLimit, RespawnWindow)
		return ErrRespawnLimitExceeded
	}

	// Record this respawn
	validTimes = append(validTimes, now)
	pm.respawnTimes[name] = validTimes

	// Stop existing process if running
	if proc, ok := pm.processes[name]; ok && proc.Running() {
		proc.Stop()
		// Wait briefly for stop
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = proc.Wait(ctx)
		cancel()
	}

	// Start new process
	newProc := NewProcess(*cfg)
	if err := newProc.StartWithContext(pm.ctx); err != nil {
		return err
	}
	pm.processes[name] = newProc

	return nil
}
