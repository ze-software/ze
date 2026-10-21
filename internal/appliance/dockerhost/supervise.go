// Design: docs/architecture/appliance/docker-host.md -- the service supervisor behind the systemctl shim
// Related: systemctl.go -- the verbs and the shim client
// Related: unitfile.go -- the unit files Lima writes

package dockerhost

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

const (
	// restartBackoffMin and restartBackoffMax bound the wait before a dying
	// service is started again: never a tight loop (spec Security Review,
	// Resource).
	restartBackoffMin = time.Second
	restartBackoffMax = 30 * time.Second
	// restartResetAfter is how long a service must run before its backoff
	// returns to the minimum.
	restartResetAfter = time.Minute
	// stopGrace is how long a stopped service gets between SIGTERM and
	// SIGKILL, systemd's default TimeoutStopSec is longer; Lima's agent and
	// sshd exit at once on SIGTERM.
	stopGrace = 10 * time.Second
)

// UnitSpec is a service the supervisor can run: its full unit name, the
// argv, and whether it is started again when it exits.
type UnitSpec struct {
	Name    string
	Argv    []string
	Restart RestartPolicy
}

// RestartPolicy is a unit's Restart= setting, the two values the guest's
// units use.
type RestartPolicy uint8

const (
	// RestartUnspecified is the zero value and never a valid policy.
	RestartUnspecified RestartPolicy = iota
	// RestartNo leaves an exited service stopped.
	RestartNo
	// RestartOnFailure starts the service again when it exits non-zero or
	// on a signal.
	RestartOnFailure
	// RestartAlways starts the service again whenever it exits.
	RestartAlways
)

// Supervisor runs the guest's services and answers the systemctl shim.
// Built-in units (sshd) are given at construction; every other unit is read
// from /etc/systemd/system when it is started, so a unit Lima writes is known
// as soon as it exists. Safe for concurrent use.
type Supervisor struct {
	root     string
	adminUID uint32
	builtin  map[string]UnitSpec
	log      io.Writer

	mu    sync.Mutex
	units map[string]*service
}

// NewSupervisor answers a supervisor over root ("/" in the guest). adminUID
// is the only uid allowed to change a unit's state (0 in the guest). log
// receives every service's output; nil discards it.
func NewSupervisor(root string, adminUID uint32, builtin []UnitSpec) (*Supervisor, error) {
	s := &Supervisor{root: root, adminUID: adminUID, builtin: map[string]UnitSpec{}, log: io.Discard, units: map[string]*service{}}
	for _, spec := range builtin {
		if len(spec.Argv) == 0 {
			return nil, fmt.Errorf("built-in unit %s has no command", spec.Name)
		}
		if spec.Restart == RestartUnspecified {
			return nil, fmt.Errorf("built-in unit %s has no restart policy", spec.Name)
		}
		s.builtin[spec.Name] = spec
	}
	return s, nil
}

// SetLog sends every service's output to log. It MUST be called before the
// first service starts.
func (s *Supervisor) SetLog(log io.Writer) {
	s.log = log
}

// Start starts the unit unless it is running.
func (s *Supervisor) Start(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if current, ok := s.units[name]; ok && current.active() {
		return nil
	}
	spec, err := s.lookup(name)
	if err != nil {
		return err
	}
	svc, err := startService(spec, s.log)
	if err != nil {
		return err
	}
	s.units[name] = svc
	return nil
}

// Stop stops the unit and waits for it to exit. Stopping a unit that is not
// running succeeds, as in systemd.
func (s *Supervisor) Stop(name string) error {
	s.mu.Lock()
	svc, ok := s.units[name]
	delete(s.units, name)
	s.mu.Unlock()
	if !ok {
		return nil
	}
	svc.stop()
	return nil
}

// Active reports whether the unit's service is running or waiting to be
// started again.
func (s *Supervisor) Active(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	svc, ok := s.units[name]
	return ok && svc.active()
}

// errUnitNotFound marks a unit neither built in nor written to the unit
// directory; systemctl answers it with exit status 5.
var errUnitNotFound = errors.New("unit not found")

// lookup answers the built-in unit or the one in /etc/systemd/system.
func (s *Supervisor) lookup(name string) (UnitSpec, error) {
	if spec, ok := s.builtin[name]; ok {
		return spec, nil
	}
	spec, err := readUnitFile(s.root, name)
	if errors.Is(err, os.ErrNotExist) {
		return UnitSpec{}, fmt.Errorf("%s: %w", name, errUnitNotFound)
	}
	return spec, err
}

// service is one running unit: one goroutine for its lifecycle, which starts
// the process again under the unit's restart policy until stop is called.
type service struct {
	spec UnitSpec
	log  io.Writer
	quit chan struct{}
	done chan struct{}

	mu       sync.Mutex
	process  *os.Process
	stopping bool
}

// startService starts the first process synchronously, so a command that
// cannot run is reported to the systemctl caller, then hands the process to
// the lifecycle goroutine. The caller MUST call stop to end the service.
func startService(spec UnitSpec, log io.Writer) (*service, error) {
	svc := &service{spec: spec, log: log, quit: make(chan struct{}), done: make(chan struct{})}
	cmd, err := svc.launch()
	if err != nil {
		return nil, fmt.Errorf("start %s: %w", spec.Name, err)
	}
	go svc.supervise(cmd)
	return svc, nil
}

// launch starts one process of the service and records it.
func (svc *service) launch() (*exec.Cmd, error) {
	cmd := exec.Command(svc.spec.Argv[0], svc.spec.Argv[1:]...) //nolint:gosec,noctx // the argv is a unit the admin uid installed; the lifecycle is stop's, not a context's
	cmd.Stdout = svc.log
	cmd.Stderr = svc.log
	cmd.Env = []string{"PATH=" + bootPath}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	svc.mu.Lock()
	svc.process = cmd.Process
	svc.mu.Unlock()
	return cmd, nil
}

// supervise waits for each process and starts the next one under the
// restart policy, with a bounded backoff. It ends when stop is called or the
// policy leaves the service stopped, and closes done.
func (svc *service) supervise(cmd *exec.Cmd) {
	defer close(svc.done)
	backoff := restartBackoffMin
	for {
		started := time.Now()
		err := cmd.Wait()
		if svc.stopped() {
			return
		}
		if !svc.restartAfter(err) {
			return
		}
		if time.Since(started) > restartResetAfter {
			backoff = restartBackoffMin
		}
		_, _ = fmt.Fprintln(svc.log, "ze-docker-host:", svc.spec.Name, "exited:", err, "- starting it again in", backoff) //nolint:errcheck // the log is the console; nothing to do if it fails
		select {
		case <-svc.quit:
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, restartBackoffMax)
		next, err := svc.launch()
		if err != nil {
			_, _ = fmt.Fprintln(svc.log, "ze-docker-host: start", svc.spec.Name, "failed:", err) //nolint:errcheck // the log is the console
			return
		}
		cmd = next
	}
}

// restartAfter applies the restart policy to an exit.
func (svc *service) restartAfter(exit error) bool {
	switch svc.spec.Restart {
	case RestartAlways:
		return true
	case RestartOnFailure:
		return exit != nil
	case RestartNo, RestartUnspecified:
		return false
	}
	return false
}

func (svc *service) stopped() bool {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	return svc.stopping
}

// active reports whether the lifecycle goroutine is still running.
func (svc *service) active() bool {
	select {
	case <-svc.done:
		return false
	default:
		return true
	}
}

// stop ends the service: SIGTERM, then SIGKILL after stopGrace, and waits
// for the lifecycle goroutine. MUST be called once, by the owner of the
// service (the Supervisor, under its map removal).
func (svc *service) stop() {
	svc.mu.Lock()
	svc.stopping = true
	process := svc.process
	svc.mu.Unlock()
	close(svc.quit)
	if process != nil {
		_ = process.Signal(syscall.SIGTERM) //nolint:errcheck // a process that already exited is the goal
	}
	select {
	case <-svc.done:
		return
	case <-time.After(stopGrace):
	}
	if process != nil {
		_ = process.Kill() //nolint:errcheck // a process that already exited is the goal
	}
	<-svc.done
}
