// Design: docs/architecture/testing/runner-architecture.md -- native pre-navigation observation.
package webtesting

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"
)

// fakeObserver owns every SDK listener and worker started for one test. Cleanup
// MUST close listeners and active connections, then join workers before removing
// their directory. The fake's CLI bootstrap MUST rendezvous with this owner
// before it returns, just as the native CLI waits for its daemon socket.
// The mutex protects sessions, connection ownership and event-log writes.
type fakeObserver struct {
	mu          sync.Mutex
	test        *testing.T
	log         string
	directory   string
	listeners   []net.Listener
	connections map[net.Conn]bool
	sessions    map[string]*fakeObserverSession
	workers     sync.WaitGroup
	stopped     bool
}

type fakeObserverSession struct {
	registered bool
	evaluated  bool
	blank      bool
}

type fakeObserverBootstrap struct {
	Session string   `json:"session"`
	Args    []string `json:"args"`
}

// installFakeObserver keeps the original shell fixture's CLI behavior and adds
// the actual newline-JSON SDK boundary. No socket is guessed before the CLI
// supplies its effective owned session identity.
func installFakeObserver(t *testing.T, logPath string) string {
	t.Helper()
	directory, err := os.MkdirTemp(os.TempDir(), "ab-sdk-")
	if err != nil {
		t.Fatal(err)
	}
	fixture := &fakeObserver{
		test: t, log: logPath, directory: directory,
		connections: make(map[net.Conn]bool), sessions: make(map[string]*fakeObserverSession),
	}
	t.Cleanup(func() {
		fixture.mu.Lock()
		fixture.stopped = true
		for _, listener := range fixture.listeners {
			if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Errorf("close fake observer listener: %v", err)
			}
		}
		for connection := range fixture.connections {
			if err := connection.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Errorf("close fake observer connection: %v", err)
			}
		}
		fixture.mu.Unlock()
		fixture.workers.Wait()
		if err := os.RemoveAll(directory); err != nil {
			t.Errorf("remove joined fake observer directory: %v", err)
		}
	})
	controlPath := filepath.Join(directory, "bootstrap.sock")
	if err := fixture.listen(controlPath, fixture.bootstrap); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZE_FAKE_OBSERVER_BOOTSTRAP", controlPath)
	t.Setenv("ZE_FAKE_OBSERVER_EXECUTABLE", executable)
	t.Setenv("AGENT_BROWSER_SOCKET_DIR", directory)
	return "case \"$1\" in\n" +
		"  --ignore-https-errors|set|open|close) GORACE=\"${GORACE:-} atexit_sleep_ms=0\" \"$ZE_FAKE_OBSERVER_EXECUTABLE\" -test.run=^TestFakeObserverBootstrapCommand$ -- \"$@\" || exit $? ;;\n" +
		"esac\n"
}

// listen starts one long-lived worker per owned listener, not per request.
// Cleanup MUST close the listener and each active connection, then join it.
func (f *fakeObserver) listen(address string, answer func(net.Conn) error) error {
	t := f.test
	listener, err := net.Listen("unix", address)
	if err != nil {
		return err
	}
	f.mu.Lock()
	if f.stopped {
		f.mu.Unlock()
		return errors.Join(net.ErrClosed, listener.Close())
	}
	f.listeners = append(f.listeners, listener)
	f.workers.Add(1)
	f.mu.Unlock()
	go func() {
		defer f.workers.Done()
		for {
			connection, err := listener.Accept()
			if err != nil {
				if !errors.Is(err, net.ErrClosed) {
					t.Errorf("accept fake observer command: %v", err)
				}
				return
			}
			f.mu.Lock()
			if f.stopped {
				f.mu.Unlock()
				if err := connection.Close(); err != nil {
					t.Errorf("close late fake observer connection: %v", err)
				}
				return
			}
			f.connections[connection] = true
			f.mu.Unlock()
			if err := connection.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Errorf("bound fake observer command: %v", err)
			} else if err := answer(connection); err != nil {
				t.Errorf("answer fake observer command: %v", err)
			}
			f.mu.Lock()
			delete(f.connections, connection)
			f.mu.Unlock()
			if err := connection.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Errorf("close answered fake observer command: %v", err)
			}
		}
	}()
	return nil
}

func (f *fakeObserver) bootstrap(connection net.Conn) error {
	var request fakeObserverBootstrap
	if err := json.NewDecoder(io.LimitReader(connection, 1<<20)).Decode(&request); err != nil {
		return err
	}
	if request.Session == "" || filepath.Base(request.Session) != request.Session {
		return errors.New("fake CLI received no contained owned session")
	}
	args := request.Args
	if len(args) > 0 && args[0] == "--ignore-https-errors" {
		args = args[1:]
	}
	if len(args) == 0 {
		return errors.New("fake CLI received no action")
	}
	f.mu.Lock()
	session, exists := f.sessions[request.Session]
	f.mu.Unlock()
	if !exists {
		session = &fakeObserverSession{blank: true}
		// The bootstrap worker is the only session creator. Its SDK worker
		// is joined by the same test owner as this bootstrap worker.
		if err := f.listen(filepath.Join(f.directory, request.Session+".sock"), func(connection net.Conn) error {
			return f.answerSDK(connection, session)
		}); err != nil {
			return err
		}
		f.mu.Lock()
		f.sessions[request.Session] = session
		f.mu.Unlock()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if args[0] == "close" {
		session.registered = false
		session.evaluated = false
		session.blank = true
	}
	if args[0] == "open" && len(args) > 1 && args[1] != "about:blank" {
		if !session.registered || !session.evaluated {
			return json.NewEncoder(connection).Encode("application navigation preceded native SDK registration and evaluation")
		}
		session.blank = false
	}
	return json.NewEncoder(connection).Encode("")
}

func (f *fakeObserver) answerSDK(connection net.Conn, session *fakeObserverSession) error {
	var request renderObserverRequest
	if err := json.NewDecoder(io.LimitReader(connection, 1<<20)).Decode(&request); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var data any
	switch request.Action {
	case "addinitscript":
		if !session.evaluated {
			return errors.New("native registration preceded blank-page validation")
		}
		if request.Script != inflightInitJS {
			return errors.New("native registration received a different observer source")
		}
		session.registered = true
		data = map[string]string{"identifier": "fixture-native-script"}
	case "evaluate":
		if !session.blank {
			return errors.New("native evaluation reached an application document")
		}
		want := "(function(){if(location.href!=='about:blank'){throw new Error('request observer installation requires the owned blank page');}\n" + inflightInitJS + "\n;return window.__zeWaitInstalled===true&&Number.isFinite(window.__zeInflight)&&Number.isFinite(window.__zeLastChange);})()"
		if request.Script != want {
			return errors.New("native evaluation did not validate the observer on its owned blank page")
		}
		session.evaluated = true
		data = map[string]bool{"result": true}
	default:
		return fmt.Errorf("unexpected native action %q", request.Action)
	}
	file, err := os.OpenFile(f.log, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := fmt.Fprintln(file, "sdk "+request.Action)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return err
	}
	return json.NewEncoder(connection).Encode(map[string]any{"id": request.ID, "success": true, "data": data})
}

// TestFakeObserverBootstrapCommand is executed only by the shell fixture. The
// parent owns the SDK daemon; this short-lived CLI waits for its readiness reply.
func TestFakeObserverBootstrapCommand(t *testing.T) {
	address := os.Getenv("ZE_FAKE_OBSERVER_BOOTSTRAP")
	if address == "" {
		return
	}
	start := slices.Index(os.Args, "--")
	if start < 0 {
		t.Fatal("fake observer command has no argument separator")
	}
	connection, err := net.DialTimeout("unix", address, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := connection.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(connection).Encode(fakeObserverBootstrap{Session: os.Getenv("AGENT_BROWSER_SESSION"), Args: os.Args[start+1:]}); err != nil {
		t.Fatal(err)
	}
	var failure string
	if err := json.NewDecoder(bufio.NewReader(connection)).Decode(&failure); err != nil {
		t.Fatal(err)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	if failure != "" {
		fmt.Fprintln(os.Stderr, failure) //nolint:errcheck // A nonzero helper exit also reports the refusal.
		os.Exit(1)
	}
	os.Exit(0)
}
