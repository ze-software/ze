// Design: docs/guide/docker.md -- `ssh -p 2222 YOUR_USERNAME@127.0.0.1`, the login the guide gives
// Overview: compose.go -- the proof these sessions serve
//
// composessh.go is the operator's side of the Compose proof: a password login
// on the container's published SSH port, an exec of `show config running |
// json`, and an interactive SSH editor session on a pseudo-terminal, the only
// surface on which an operator commits a change to a running container.

package testdeployment

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// composeDialTimeout bounds one TCP connect and SSH handshake.
	composeDialTimeout = 5 * time.Second
	// composeExecTimeout bounds one exec command's answer.
	composeExecTimeout = 30 * time.Second
	// editorQuiet is how long the editor's terminal must stay silent before the
	// next line is typed: the editor has then rendered its answer to the last.
	editorQuiet = 500 * time.Millisecond
	// editorSettleMax bounds the wait for that silence.
	editorSettleMax = 15 * time.Second
	// runningQuery is the exec command that answers the running configuration.
	runningQuery = "show config running | json"
)

// composeLogin is one authenticated SSH connection to the container.
type composeLogin struct {
	client  *ssh.Client
	hostKey string
}

func (l *composeLogin) close() {
	l.client.Close() //nolint:errcheck // the answers already read are authoritative
}

// dialCompose logs in once. A non-empty wantHostKey pins the server key: a
// container that presents another key is not the installation that was
// initialized, and the login is refused with that reason.
func dialCompose(ctx context.Context, address, user, password, wantHostKey string) (*composeLogin, error) {
	var seen string
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			seen = ssh.FingerprintSHA256(key)
			if wantHostKey != "" && seen != wantHostKey {
				return errHostKeyChanged{want: wantHostKey, got: seen}
			}
			return nil
		},
		Timeout: composeDialTimeout,
	}
	dialCtx, cancel := context.WithTimeout(ctx, composeDialTimeout)
	defer cancel()
	connection, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", address)
	if err != nil {
		return nil, err
	}
	if err := connection.SetDeadline(time.Now().Add(composeDialTimeout)); err != nil {
		connection.Close() //nolint:errcheck // the deadline failure is the answer
		return nil, err
	}
	transport, channels, requests, err := ssh.NewClientConn(connection, address, config)
	if err != nil {
		connection.Close() //nolint:errcheck // the handshake failure is the answer
		return nil, err
	}
	if err := connection.SetDeadline(time.Time{}); err != nil {
		transport.Close() //nolint:errcheck // the deadline failure is the answer
		return nil, err
	}
	return &composeLogin{client: ssh.NewClient(transport, channels, requests), hostKey: seen}, nil
}

// errHostKeyChanged is a login refused because the container's SSH host key is
// not the one the first container presented: the key lives in the store, so a
// new key means a new store.
type errHostKeyChanged struct{ want, got string }

func (e errHostKeyChanged) Error() string {
	var tb textbuf.Buffer
	return tb.Str("the SSH host key changed from ").Str(e.want).Str(" to ").Str(e.got).
		Str(": the new container does not hold the store the first one initialized").String()
}

// awaitComposeLogin retries the login until the daemon's listener answers or
// timeout passes. A changed host key or a refused password ends the wait at
// once: neither is a listener still starting.
func awaitComposeLogin(ctx context.Context, address, user, password, wantHostKey string, timeout time.Duration) (*composeLogin, error) {
	deadline := time.Now().Add(timeout)
	var last error
	for {
		login, err := dialCompose(ctx, address, user, password, wantHostKey)
		if err == nil {
			return login, nil
		}
		last = err
		if _, changed := errors.AsType[errHostKeyChanged](err); changed {
			return nil, err
		}
		if strings.Contains(err.Error(), "unable to authenticate") {
			return nil, errors.New("SSH refused the password ze init stored for " + user + ": " + err.Error())
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !time.Now().Before(deadline) {
			var tb textbuf.Buffer
			return nil, errors.New(tb.Str("no SSH login at ").Str(address).Str(" within ").Str(timeout.String()).
				Str(": ").Err(last).String())
		}
		time.Sleep(pollInterval)
	}
}

// refuseWrongPassword requires the daemon to refuse a wrong password, so the
// accepted login is proven to be checked against the stored credential.
func refuseWrongPassword(ctx context.Context, address, user, wrong, hostKey string) error {
	login, err := dialCompose(ctx, address, user, wrong, hostKey)
	if err == nil {
		login.close()
		return errors.New("SSH accepted a wrong password for " + user + ", so the login proves no stored credential")
	}
	if !strings.Contains(err.Error(), "unable to authenticate") {
		return errors.New("a wrong password ended in an error that is not an authentication refusal: " + err.Error())
	}
	return nil
}

// runningLeaf answers the value at path in the running configuration, and
// whether the configuration holds it.
func (l *composeLogin) runningLeaf(path []string) (string, bool, error) {
	session, err := l.client.NewSession()
	if err != nil {
		return "", false, err
	}
	defer session.Close() //nolint:errcheck // the answer is authoritative
	var stderr textbuf.Buffer
	session.Stderr = &stderr
	done := make(chan struct{})
	timer := time.AfterFunc(composeExecTimeout, func() {
		session.Close() //nolint:errcheck // the bound ends the exec
		close(done)
	})
	output, err := session.Output(runningQuery)
	if !timer.Stop() {
		<-done
		return "", false, errors.New("`" + runningQuery + "` did not answer within " + composeExecTimeout.String())
	}
	if err != nil {
		return "", false, errors.New("`" + runningQuery + "`: " + err.Error() + ": " + strings.TrimSpace(stderr.String()))
	}
	return leafAt(output, path)
}

// leafAt answers the string at path in a JSON document, and whether the
// document holds that path. A path through a value that is not an object, or a
// leaf that is not a string, is an error rather than "absent".
func leafAt(document []byte, path []string) (string, bool, error) {
	var node any
	if err := json.Unmarshal(document, &node); err != nil {
		return "", false, errors.New("`" + runningQuery + "` answered no JSON: " + err.Error())
	}
	for index, key := range path {
		object, ok := node.(map[string]any)
		if !ok {
			return "", false, errors.New("`" + runningQuery + "` holds no object at " + strings.Join(path[:index], " "))
		}
		child, present := object[key]
		if !present {
			return "", false, nil
		}
		node = child
	}
	value, ok := node.(string)
	if !ok {
		return "", true, errors.New("`" + runningQuery + "` holds a non-string value at " + strings.Join(path, " "))
	}
	return value, true, nil
}

// editInSSHEditor opens the SSH editor on a pseudo-terminal, types each line
// once the editor has answered the one before, then waits for applied to
// report the change in the running configuration. It answers the editor's last
// screen lines, which explain a commit that did not apply.
func editInSSHEditor(ctx context.Context, address, user, password, hostKey string, lines []string,
	applied func() (bool, error), timeout time.Duration,
) ([]string, error) {
	login, err := dialCompose(ctx, address, user, password, hostKey)
	if err != nil {
		return nil, err
	}
	defer login.close()
	session, err := login.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close() //nolint:errcheck // the running configuration is the answer

	screen := newTerminalRecorder()
	session.Stdout = screen
	session.Stderr = screen
	stdin, err := session.StdinPipe()
	if err != nil {
		return nil, err
	}
	if err := session.RequestPty("xterm-256color", 40, 160, ssh.TerminalModes{}); err != nil {
		return nil, err
	}
	if err := session.Shell(); err != nil {
		return nil, err
	}
	for _, line := range lines {
		if !screen.settle(ctx, editorQuiet, editorSettleMax) {
			return screen.tail(), errors.New("the SSH editor did not settle before `" + line + "`")
		}
		if _, err := stdin.Write([]byte(line + "\r")); err != nil {
			return screen.tail(), err
		}
	}

	deadline := time.Now().Add(timeout)
	for {
		ok, err := applied()
		if err != nil {
			return screen.tail(), err
		}
		if ok {
			stdin.Write([]byte("exit\r")) //nolint:errcheck // the change is committed; leaving is courtesy
			return screen.tail(), nil
		}
		if ctx.Err() != nil {
			return screen.tail(), ctx.Err()
		}
		if !time.Now().Before(deadline) {
			return screen.tail(), errors.New("the commit did not reach the running configuration within " + timeout.String())
		}
		time.Sleep(pollInterval)
	}
}

// terminalEscape matches the control sequences a terminal program writes:
// CSI, OSC and the two-byte escapes.
var terminalEscape = regexp.MustCompile("\x1b\\[[0-?]*[\\x20-\\x2f]*[\\x40-\\x7e]|\x1b\\][^\x07\x1b]*(?:\x07|\x1b\\\\)|\x1b[\\x40-\\x5f]")

// terminalRecorder keeps the bounded text of a terminal session and when it
// last wrote. Safe for concurrent use.
type terminalRecorder struct {
	mu      sync.Mutex
	pending string
	last    time.Time
	wrote   bool
	seen    *collector
}

func newTerminalRecorder() *terminalRecorder {
	return &terminalRecorder{seen: newCollector()}
}

// Write records output with its escapes removed, one line per row.
func (r *terminalRecorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.last = time.Now()
	r.wrote = true
	r.pending += string(p)
	if len(r.pending) > lineMax {
		r.pending = r.pending[len(r.pending)-lineMax:]
	}
	rows := strings.Split(r.pending, "\n")
	r.pending = rows[len(rows)-1]
	for _, row := range rows[:len(rows)-1] {
		text := strings.TrimSpace(strings.ReplaceAll(terminalEscape.ReplaceAllString(row, ""), "\r", " "))
		if text != "" {
			r.seen.add(text)
		}
	}
	return len(p), nil
}

// settle waits until the terminal has written and then stayed silent for
// quiet, at most bound. It answers whether it settled.
func (r *terminalRecorder) settle(ctx context.Context, quiet, bound time.Duration) bool {
	deadline := time.Now().Add(bound)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return false
		}
		r.mu.Lock()
		silent := r.wrote && time.Since(r.last) >= quiet
		r.mu.Unlock()
		if silent {
			return true
		}
		time.Sleep(quiet / 5)
	}
	return false
}

func (r *terminalRecorder) tail() []string { return r.seen.tailLines() }
