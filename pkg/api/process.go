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
	// Note: ACK is controlled by serial prefix (#N), not per-process state
	syncEnabled atomic.Bool // Whether to wait for wire transmission (default: false)

	// Wire encoding for API messages (default: WireEncodingHex = 0)
	wireEncodingIn  atomic.Uint32 // Inbound: events ZeBGP→Process
	wireEncodingOut atomic.Uint32 // Outbound: commands Process→ZeBGP

	// ZeBGP→Process request tracking
	nextSerial      atomic.Uint64          // Counter for alpha serial generation
	pendingRequests map[string]chan string // serial -> response channel
	pendingMu       sync.Mutex             // Protects pendingRequests

	// Registered plugin commands (tracked for cleanup on death)
	registeredCommands []string
	registeredMu       sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
}

// NewProcess creates a new process with the given configuration.
func NewProcess(config ProcessConfig) *Process {
	return &Process{
		config:          config,
		pendingRequests: make(map[string]chan string),
	}
}

// Running returns true if the process is running.
func (p *Process) Running() bool {
	return p.running.Load()
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

// WireEncodingIn returns the inbound wire encoding (events ZeBGP→Process).
func (p *Process) WireEncodingIn() WireEncoding {
	// Safe: only values 0-3 are ever stored (WireEncodingHex..WireEncodingText).
	return WireEncoding(p.wireEncodingIn.Load()) //nolint:gosec // Bounded to 0-3
}

// WireEncodingOut returns the outbound wire encoding (commands Process→ZeBGP).
func (p *Process) WireEncodingOut() WireEncoding {
	// Safe: only values 0-3 are ever stored (WireEncodingHex..WireEncodingText).
	return WireEncoding(p.wireEncodingOut.Load()) //nolint:gosec // Bounded to 0-3
}

// SetWireEncodingIn sets the inbound wire encoding.
func (p *Process) SetWireEncodingIn(enc WireEncoding) {
	p.wireEncodingIn.Store(uint32(enc))
}

// SetWireEncodingOut sets the outbound wire encoding.
func (p *Process) SetWireEncodingOut(enc WireEncoding) {
	p.wireEncodingOut.Store(uint32(enc))
}

// SetWireEncoding sets both inbound and outbound wire encoding.
func (p *Process) SetWireEncoding(enc WireEncoding) {
	p.wireEncodingIn.Store(uint32(enc))
	p.wireEncodingOut.Store(uint32(enc))
}

// AddRegisteredCommand tracks a command registered by this process.
func (p *Process) AddRegisteredCommand(name string) {
	p.registeredMu.Lock()
	defer p.registeredMu.Unlock()
	p.registeredCommands = append(p.registeredCommands, name)
}

// RemoveRegisteredCommand removes a command from tracking.
func (p *Process) RemoveRegisteredCommand(name string) {
	p.registeredMu.Lock()
	defer p.registeredMu.Unlock()
	for i, cmd := range p.registeredCommands {
		if cmd == name {
			p.registeredCommands = append(p.registeredCommands[:i], p.registeredCommands[i+1:]...)
			return
		}
	}
}

// RegisteredCommands returns a copy of the registered command names.
func (p *Process) RegisteredCommands() []string {
	p.registeredMu.Lock()
	defer p.registeredMu.Unlock()
	result := make([]string, len(p.registeredCommands))
	copy(result, p.registeredCommands)
	return result
}

// Name returns the process name from config.
func (p *Process) Name() string {
	return p.config.Name
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
// Lines starting with @N are routed to pending request handlers.
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

		// Check for @N response prefix (ZeBGP→Process request response)
		if serial, response, ok := parseResponseSerial(line); ok {
			p.pendingMu.Lock()
			if ch, found := p.pendingRequests[serial]; found {
				select {
				case ch <- response:
				default:
					// Channel full or closed, ignore
				}
			}
			p.pendingMu.Unlock()
			continue // Don't send to normal command channel
		}

		p.lines <- line
	}
}

// parseResponseSerial extracts @N prefix from response line.
// Returns (serial, rest, true) if @N prefix found, ("", "", false) otherwise.
func parseResponseSerial(line string) (string, string, bool) {
	if len(line) < 2 || line[0] != '@' {
		return "", "", false
	}
	// Find space after @serial
	idx := 1
	for idx < len(line) && line[idx] != ' ' {
		idx++
	}
	if idx == 1 {
		return "", "", false // Just "@" with no serial
	}
	serial := line[1:idx]
	rest := ""
	if idx < len(line) {
		rest = line[idx+1:] // Skip the space
	}
	return serial, rest, true
}

// Stop terminates the process.
func (p *Process) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
}

// SendShutdown sends a shutdown signal to the process via stdin.
// This allows the process to exit gracefully before being killed.
// Uses synchronous write to ensure delivery before process termination.
// The message is written directly to stdin, bypassing the async write queue.
// Returns true if the shutdown signal was sent successfully.
func (p *Process) SendShutdown() bool {
	if !p.running.Load() {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stdin == nil || !p.running.Load() {
		return false
	}
	// Write synchronously - shutdown is critical and must be delivered
	// JSON format: {"answer": "shutdown"}
	_, err := p.stdin.Write([]byte("{\"answer\": \"shutdown\"}\n"))
	return err == nil
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
	// Check if process is running and context is valid
	if !p.running.Load() || p.writeQueue == nil || p.ctx == nil {
		return os.ErrClosed
	}

	data := []byte(event + "\n")

	// Non-blocking send with backpressure.
	// Use context to detect shutdown instead of relying on channel close.
	select {
	case p.writeQueue <- data:
		return nil
	case <-p.ctx.Done():
		// Process shutting down
		return os.ErrClosed
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

// SendRequest sends a request to the process and waits for response.
// Uses alpha serial encoding to avoid collision with process numeric serials.
// Returns the response text (after @serial prefix) or error on timeout.
func (p *Process) SendRequest(ctx context.Context, command string) (string, error) {
	if !p.running.Load() {
		return "", os.ErrClosed
	}

	// Generate alpha serial
	n := p.nextSerial.Add(1) - 1
	serial := encodeAlphaSerial(n)

	// Create response channel
	respCh := make(chan string, 1)

	// Register pending request
	p.pendingMu.Lock()
	p.pendingRequests[serial] = respCh
	p.pendingMu.Unlock()

	// Cleanup on exit
	defer func() {
		p.pendingMu.Lock()
		delete(p.pendingRequests, serial)
		p.pendingMu.Unlock()
	}()

	// Send request: #serial command
	request := fmt.Sprintf("#%s %s", serial, command)
	if err := p.WriteEvent(request); err != nil {
		return "", err
	}

	// Wait for response
	select {
	case resp := <-respCh:
		return resp, nil
	case <-ctx.Done():
		return "", ctx.Err()
	case <-p.ctx.Done():
		return "", os.ErrClosed
	}
}

// writeLoop drains the write queue and writes to stdin.
// Stops on context cancellation or write error (EPIPE indicates process died).
func (p *Process) writeLoop() {
	for {
		select {
		case data := <-p.writeQueue:
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
		case <-p.ctx.Done():
			// Process shutting down
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

	// Cancel context to stop writeLoop goroutine.
	// This is safe even if Stop() already cancelled it (cancel is idempotent).
	if p.cancel != nil {
		p.cancel()
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
// Sends shutdown signal to each process before termination to allow graceful exit.
func (pm *ProcessManager) Stop() {
	// Send shutdown signal synchronously to all processes.
	// SendShutdown writes directly to stdin (bypassing async queue),
	// so the message is in the pipe buffer before we proceed.
	pm.mu.RLock()
	for _, proc := range pm.processes {
		proc.SendShutdown()
	}
	pm.mu.RUnlock()

	// Cancel context and stop all processes
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
