// Design: docs/architecture/appliance/docker-host.md -- the systemctl shim Lima's and Colima's scripts call
// Related: supervise.go -- the supervisor the shim drives

package dockerhost

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// ShimSocket is the supervisor's control socket in the guest.
const ShimSocket = "/run/ze-docker-host/systemctl.sock"

const (
	// shimMessageMax bounds one request or answer line.
	shimMessageMax = 64 << 10
	// shimDeadline bounds one exchange; stop waits stopGraceMax at most.
	shimDeadline = stopGraceMax + 5*time.Second
)

// systemctl exit statuses (systemctl(1), "Exit status", and LSB for
// is-active): 0 success, 1 failure, 3 not active, 4 no permission, 5 no
// such unit.
const (
	exitOK         = 0
	exitFailure    = 1
	exitInactive   = 3
	exitPermission = 4
	exitNoUnit     = 5
)

type shimRequest struct {
	Args []string `json:"args"`
}

type shimAnswer struct {
	Code   int    `json:"code"`
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}

// verb is one systemctl verb: whether it changes state (admin only) and
// what it does to each unit named.
type verb struct {
	changes bool
	run     func(s *Supervisor, units []string) shimAnswer
}

// verbs is the systemctl surface the guest answers: the verbs Lima's
// install-systemd, Colima's provisioning and Lima's boot scripts (reload, from
// 11-colorterm-environment.sh) run, plus stop and is-enabled.
// An unknown verb is refused by name.
var verbs = map[string]verb{
	"daemon-reload": {changes: true, run: func(*Supervisor, []string) shimAnswer {
		// Units are read from disk when started, so a reload has nothing to
		// refresh.
		return shimAnswer{}
	}},
	"enable":      {changes: true, run: (*Supervisor).verbKnown},
	"is-enabled":  {run: (*Supervisor).verbIsEnabled},
	"start":       {changes: true, run: (*Supervisor).verbStart},
	"stop":        {changes: true, run: (*Supervisor).verbStop},
	"restart":     {changes: true, run: (*Supervisor).verbRestart},
	"try-restart": {changes: true, run: (*Supervisor).verbTryRestart},
	"is-active":   {run: (*Supervisor).verbIsActive},
	"reload":      {changes: true, run: (*Supervisor).verbReload},
}

// Serve listens on socket and answers shim requests one at a time until the
// returned stop func is called. The caller MUST call stop; it closes the
// listener and waits for the accept loop to end.
func (s *Supervisor) Serve(socket string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(socket), 0o755); err != nil { //nolint:gosec // the shim runs as any account and must reach the socket
		return nil, fmt.Errorf("create %s: %w", filepath.Dir(socket), err)
	}
	if err := os.Remove(socket); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("remove stale %s: %w", socket, err)
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", socket, err)
	}
	// Every user may connect: `systemctl is-active` runs unprivileged. The
	// peer uid decides what a request may change.
	if err := os.Chmod(socket, 0o666); err != nil { //nolint:gosec // see above: the peer uid is the guard
		_ = listener.Close() //nolint:errcheck // already failing
		return nil, fmt.Errorf("chmod %s: %w", socket, err)
	}
	done := make(chan struct{})
	go s.acceptLoop(listener, done)
	return func() {
		_ = listener.Close() //nolint:errcheck // closing ends the loop either way
		<-done
	}, nil
}

// acceptLoop answers connections in turn: each exchange is bounded by
// shimDeadline, so one stuck client delays the next by at most that.
func (s *Supervisor) acceptLoop(listener *net.UnixListener, done chan<- struct{}) {
	defer close(done)
	for {
		conn, err := listener.AcceptUnix()
		if err != nil {
			return
		}
		s.answer(conn)
	}
}

// answer reads one request, dispatches it and writes the answer.
func (s *Supervisor) answer(conn *net.UnixConn) {
	defer conn.Close()                                 //nolint:errcheck // the exchange is over
	_ = conn.SetDeadline(time.Now().Add(shimDeadline)) //nolint:errcheck // a failed deadline leaves the read unbounded only on a broken socket
	reply := shimAnswer{Code: exitFailure}
	uid, err := peerUID(conn)
	if err != nil {
		reply.Stderr = "ze-docker-host: cannot read the caller's uid: " + err.Error() + "\n"
		writeAnswer(conn, reply)
		return
	}
	line, err := bufio.NewReaderSize(io.LimitReader(conn, shimMessageMax), shimMessageMax).ReadBytes('\n')
	if err != nil {
		reply.Stderr = "ze-docker-host: read request: " + err.Error() + "\n"
		writeAnswer(conn, reply)
		return
	}
	var request shimRequest
	if err := json.Unmarshal(line, &request); err != nil {
		reply.Stderr = "ze-docker-host: parse request: " + err.Error() + "\n"
		writeAnswer(conn, reply)
		return
	}
	writeAnswer(conn, s.dispatch(uid, request.Args))
}

func writeAnswer(conn *net.UnixConn, reply shimAnswer) {
	data, err := json.Marshal(reply)
	if err != nil {
		return
	}
	_, _ = conn.Write(append(data, '\n')) //nolint:errcheck // the client reports a missing answer itself
}

// dispatch runs one systemctl command line for a caller with uid.
func (s *Supervisor) dispatch(uid uint32, args []string) shimAnswer {
	var words []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue // --system, --quiet, --no-block: no effect on this guest
		}
		words = append(words, arg)
	}
	if len(words) == 0 {
		return shimAnswer{Code: exitFailure, Stderr: "systemctl: no verb given; this guest answers " + verbList() + "\n"}
	}
	name := words[0]
	v, ok := verbs[name]
	if !ok {
		return shimAnswer{Code: exitFailure, Stderr: "Unknown command verb '" + name + "': this guest answers " + verbList() + "\n"}
	}
	if v.changes && uid != s.adminUID {
		return shimAnswer{Code: exitPermission, Stderr: "systemctl " + name + ": only root may change a unit; run it with sudo\n"}
	}
	units := make([]string, 0, len(words)-1)
	for _, unit := range words[1:] {
		if !strings.Contains(unit, ".") {
			unit += ".service"
		}
		units = append(units, unit)
	}
	return v.run(s, units)
}

func verbList() string {
	names := make([]string, 0, len(verbs))
	for name := range verbs {
		names = append(names, name)
	}
	slices.Sort(names)
	return strings.Join(names, ", ")
}

// notFound answers exit 5 naming the unit and the units the guest knows.
func (s *Supervisor) notFound(action, unit string) shimAnswer {
	return shimAnswer{Code: exitNoUnit, Stderr: "Failed to " + action + " " + unit + ": Unit " + unit +
		" not found. Units this guest knows: " + strings.Join(s.knownUnits(), ", ") + "\n"}
}

// knownUnits lists the built-in units and the unit files on disk.
func (s *Supervisor) knownUnits() []string {
	names := make([]string, 0, len(s.builtin))
	for name := range s.builtin {
		names = append(names, name)
	}
	entries, err := os.ReadDir(guest(s.root, unitDir))
	if err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".service") {
				names = append(names, e.Name())
			}
		}
	}
	slices.Sort(names)
	return names
}

func (s *Supervisor) verbKnown(units []string) shimAnswer {
	for _, unit := range units {
		if _, err := s.lookup(unit); err != nil {
			return s.failure("enable", unit, err)
		}
	}
	return shimAnswer{}
}

func (s *Supervisor) verbIsEnabled(units []string) shimAnswer {
	for _, unit := range units {
		if _, err := s.lookup(unit); err != nil {
			return s.failure("check", unit, err)
		}
	}
	return shimAnswer{Stdout: strings.Repeat("enabled\n", len(units))}
}

func (s *Supervisor) verbStart(units []string) shimAnswer {
	for _, unit := range units {
		if err := s.Start(unit); err != nil {
			return s.failure("start", unit, err)
		}
	}
	return shimAnswer{}
}

func (s *Supervisor) verbStop(units []string) shimAnswer {
	for _, unit := range units {
		if err := s.Stop(unit); err != nil {
			return s.failure("stop", unit, err)
		}
	}
	return shimAnswer{}
}

func (s *Supervisor) verbRestart(units []string) shimAnswer {
	for _, unit := range units {
		if _, err := s.lookup(unit); err != nil {
			return s.failure("restart", unit, err)
		}
		if err := s.Stop(unit); err != nil {
			return s.failure("restart", unit, err)
		}
		if err := s.Start(unit); err != nil {
			return s.failure("restart", unit, err)
		}
	}
	return shimAnswer{}
}

// verbReload signals each unit's main process to reread its configuration.
func (s *Supervisor) verbReload(units []string) shimAnswer {
	for _, unit := range units {
		if err := s.Reload(unit); err != nil {
			return s.failure("reload", unit, err)
		}
	}
	return shimAnswer{}
}

// verbTryRestart restarts the units that are running and leaves the others
// alone, as systemd does.
func (s *Supervisor) verbTryRestart(units []string) shimAnswer {
	for _, unit := range units {
		if !s.Active(unit) {
			continue
		}
		if reply := s.verbRestart([]string{unit}); reply.Code != exitOK {
			return reply
		}
	}
	return shimAnswer{}
}

// verbIsActive prints active or inactive per unit; the exit status is 0
// only when every unit is active.
func (s *Supervisor) verbIsActive(units []string) shimAnswer {
	reply := shimAnswer{}
	for _, unit := range units {
		if s.Active(unit) {
			reply.Stdout += "active\n"
			continue
		}
		reply.Stdout += "inactive\n"
		reply.Code = exitInactive
	}
	return reply
}

// failure maps an error to its systemctl exit status and message.
func (s *Supervisor) failure(action, unit string, err error) shimAnswer {
	if errors.Is(err, errUnitNotFound) {
		return s.notFound(action, unit)
	}
	if errors.Is(err, errUnitInactive) {
		return shimAnswer{Code: exitFailure, Stderr: "Failed to " + action + " " + unit + ": " + err.Error() + "; start it with: systemctl start " + unit + "\n"}
	}
	return shimAnswer{Code: exitFailure, Stderr: "Failed to " + action + " " + unit + ": " + err.Error() + "\n"}
}

// RunShim is the `systemctl` program: it sends args to the supervisor on
// socket, prints the answer and returns its exit status.
func RunShim(socket string, args []string, stdout, stderr io.Writer) int {
	dialer := net.Dialer{Timeout: shimDeadline}
	conn, err := dialer.DialContext(context.Background(), "unix", socket)
	if err != nil {
		_, _ = io.WriteString(stderr, "systemctl: the ze-docker-host supervisor is not answering on "+socket+": "+err.Error()+
			"\nThis guest's systemctl is a shim; check that /user/ze-docker-host is running (gokrazy's web status page lists it).\n") //nolint:errcheck // nothing else to report to
		return exitFailure
	}
	defer conn.Close()                                 //nolint:errcheck // the exchange is over
	_ = conn.SetDeadline(time.Now().Add(shimDeadline)) //nolint:errcheck // a broken socket fails the write below
	request, err := json.Marshal(shimRequest{Args: args})
	if err != nil {
		_, _ = io.WriteString(stderr, "systemctl: encode request: "+err.Error()+"\n") //nolint:errcheck // nothing else to report to
		return exitFailure
	}
	if _, err := conn.Write(append(request, '\n')); err != nil {
		_, _ = io.WriteString(stderr, "systemctl: send request to "+socket+": "+err.Error()+"\n") //nolint:errcheck // nothing else to report to
		return exitFailure
	}
	line, err := bufio.NewReaderSize(io.LimitReader(conn, shimMessageMax), shimMessageMax).ReadBytes('\n')
	if err != nil {
		_, _ = io.WriteString(stderr, "systemctl: no answer from "+socket+": "+err.Error()+"\n") //nolint:errcheck // nothing else to report to
		return exitFailure
	}
	var reply shimAnswer
	if err := json.Unmarshal(line, &reply); err != nil {
		_, _ = io.WriteString(stderr, "systemctl: parse answer: "+err.Error()+"\n") //nolint:errcheck // nothing else to report to
		return exitFailure
	}
	_, _ = io.WriteString(stdout, reply.Stdout) //nolint:errcheck // the exit status still carries the answer
	_, _ = io.WriteString(stderr, reply.Stderr) //nolint:errcheck // the exit status still carries the answer
	return reply.Code
}
