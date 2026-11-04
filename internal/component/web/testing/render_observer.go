// Design: docs/architecture/testing/runner-architecture.md -- native pre-navigation request observation.
// Related: https://github.com/vercel-labs/agent-browser/tree/v0.26.0/cli/src/native -- persistent CDP session actions.
package webtesting

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// renderObserver binds native SDK commands to one owned daemon endpoint. Browser
// publishes it only after successful registration and MUST discard it at owned
// Close. It owns no connection or worker and is single-owner.
type renderObserver struct {
	network string
	address string
}

type renderObserverRequest struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	Script string `json:"script"`
}

type renderObserverResponse struct {
	ID      string          `json:"id"`
	Success *bool           `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
	Warning string          `json:"warning"`
}

// renderObserverScript registers before the first application navigation, using
// the native daemon's persistent page session. Evaluating the same source on its
// owned blank page surfaces browser exceptions; it is not a late-page fallback.
func (b *Browser) renderObserverScript(script string) error {
	if b.observer != nil {
		return nil
	}
	if b.session == "" {
		return errors.New("install render observer: no owned browser session")
	}
	if !b.daemonStarted {
		if err := b.runAgentEnsureDaemon("open", "about:blank"); err != nil {
			return fmt.Errorf("start owned blank page for render observation: %w", err)
		}
	}
	observer, err := newRenderObserverAddress(b.session)
	if err != nil {
		return err
	}
	validationScript := "(function(){if(location.href!=='about:blank'){throw new Error('request observer installation requires the owned blank page');}\n" + script + "\n;return window.__zeWaitInstalled===true&&Number.isFinite(window.__zeInflight)&&Number.isFinite(window.__zeLastChange);})()"
	validated, err := observer.command(b.ctx, "evaluate", validationScript)
	if err != nil {
		return fmt.Errorf("evaluate request observer on owned blank page: %w", err)
	}
	var validation struct {
		Result *bool `json:"result"`
	}
	if err := json.Unmarshal(validated, &validation); err != nil {
		return fmt.Errorf("decode request observer evaluation: %w", err)
	}
	if validation.Result == nil {
		return errors.New("request observer evaluation returned no installation verdict")
	}
	if !*validation.Result {
		return errors.New("request observer evaluation did not install finite observation state")
	}
	registered, err := observer.command(b.ctx, "addinitscript", script)
	if err != nil {
		return fmt.Errorf("register request observer before navigation: %w", err)
	}
	var registration struct {
		Identifier string `json:"identifier"`
	}
	if err := json.Unmarshal(registered, &registration); err != nil {
		return fmt.Errorf("decode request observer registration: %w", err)
	}
	if registration.Identifier == "" {
		return errors.New("request observer registration returned no native script identifier")
	}
	b.observer = observer
	return nil
}

// newRenderObserverAddress follows the pinned native connection.rs discovery:
// explicit socket directory, XDG runtime directory, home, then platform temp.
// Windows reads the daemon's actual .port sidecar instead of guessing a port.
func newRenderObserverAddress(session string) (*renderObserver, error) {
	directory, err := renderObserverSocketDir()
	if err != nil {
		return nil, err
	}
	if runtime.GOOS != "windows" {
		return &renderObserver{network: "unix", address: filepath.Join(directory, session+".sock")}, nil
	}
	data, err := os.ReadFile(filepath.Join(directory, session+".port"))
	if err != nil {
		return nil, fmt.Errorf("read owned browser port: %w", err)
	}
	portText := strings.TrimSpace(string(data))
	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("decode owned browser port: %w", err)
	}
	if port == 0 {
		return nil, errors.New("owned browser returned an unspecified port")
	}
	return &renderObserver{network: "tcp", address: net.JoinHostPort("127.0.0.1", portText)}, nil
}

func renderObserverSocketDir() (string, error) {
	if directory := os.Getenv("AGENT_BROWSER_SOCKET_DIR"); directory != "" {
		return directory, nil
	}
	if directory := os.Getenv("XDG_RUNTIME_DIR"); directory != "" {
		return filepath.Join(directory, "agent-browser"), nil
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".agent-browser"), nil
	}
	// Native dirs::home_dir also consults the account when HOME is absent.
	account, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("resolve native browser socket home: %w", err)
	}
	if account.HomeDir != "" {
		return filepath.Join(account.HomeDir, ".agent-browser"), nil
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(os.TempDir(), "agent-browser"), nil
	}
	// agentEnv removes TMPDIR, so native temp_dir uses the Unix default.
	return filepath.Join("/tmp", "agent-browser"), nil
}

// command owns and closes one newline-JSON connection. Its context bounds dial,
// write and read; the cancellation callback MUST be joined before return. The
// native response is limited to 1 MiB and MUST be complete and identity-matched.
func (o *renderObserver) command(parent context.Context, action, script string) (_ json.RawMessage, err error) {
	ctx, cancel := context.WithTimeout(parent, Bound(parent, agentTimeout))
	defer cancel()
	connection, err := (&net.Dialer{}).DialContext(ctx, o.network, o.address)
	if err != nil {
		return nil, errors.Join(err, context.Cause(ctx))
	}
	defer func() { err = errors.Join(err, context.Cause(ctx), connection.Close()) }()
	deadline, _ := ctx.Deadline() // WithTimeout above always supplies this command's deadline.
	if err := connection.SetDeadline(deadline); err != nil {
		return nil, err
	}
	interrupted := make(chan struct{})
	var interruptErr error
	stop := context.AfterFunc(ctx, func() {
		interruptErr = connection.SetDeadline(time.Now())
		close(interrupted)
	})
	defer func() {
		if !stop() {
			<-interrupted
			err = errors.Join(err, interruptErr)
		}
	}()
	const requestID = "ze-render-observer"
	if err := json.NewEncoder(connection).Encode(renderObserverRequest{ID: requestID, Action: action, Script: script}); err != nil {
		return nil, fmt.Errorf("send native observer %s: %w", action, err)
	}
	const responseBytesMax = 1 << 20
	line, err := bufio.NewReader(io.LimitReader(connection, responseBytesMax+1)).ReadBytes('\n')
	if len(line) > responseBytesMax {
		return nil, errors.New("native observer response exceeds 1 MiB")
	}
	if err != nil {
		return nil, fmt.Errorf("read complete native observer response: %w", err)
	}
	var response renderObserverResponse
	if err := json.Unmarshal(line, &response); err != nil {
		return nil, fmt.Errorf("decode native observer response: %w", err)
	}
	if response.ID != requestID {
		return nil, errors.New("native observer response does not match its request identity")
	}
	if response.Success == nil {
		return nil, errors.New("native observer response has no success verdict")
	}
	if !*response.Success {
		return nil, fmt.Errorf("native observer %s failed: %s", action, response.Error)
	}
	return response.Data, nil
}
