// Design: docs/guide/config-editor.md -- SSH shutdown must join admitted owners.
// Related: listener_start.go -- Start waits for actual Serve listener admission.

package ssh

import (
	"context"
	cryptoRand "crypto/rand"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"testing/synctest"

	charmssh "charm.land/ssh"
)

// registrationListener is real Serve's listener. Its first Accept can only run
// after the vendor has registered its listener WaitGroup; Close releases it.
type registrationListener struct {
	accepted chan struct{}
	closed   chan struct{}
	once     sync.Once
}

func (l *registrationListener) Accept() (net.Conn, error) {
	close(l.accepted)
	<-l.closed
	return nil, net.ErrClosed
}

func (l *registrationListener) Close() error {
	l.once.Do(func() { close(l.closed) })
	return nil
}

func (*registrationListener) Addr() net.Addr { return &net.TCPAddr{} }

// registrationEntropyGate holds actual ensureHostSigner before trackListener.
// It releases into the real secure reader, so Serve still creates its signer.
type registrationEntropyGate struct {
	reader  io.Reader
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (g *registrationEntropyGate) Read(data []byte) (int, error) {
	g.once.Do(func() { close(g.entered) })
	<-g.release
	return g.reader.Read(data)
}

// TestSSHServeReadinessFollowsListenerAdmission holds actual host-signer
// generation before listener registration, proves its caller cannot return,
// then proves actual Shutdown finds and closes the admitted listener.
func TestSSHServeReadinessFollowsListenerAdmission(t *testing.T) {
	t.Setenv("GODEBUG", os.Getenv("GODEBUG")+",cryptocustomrand=1")
	original := cryptoRand.Reader
	defer func() { cryptoRand.Reader = original }()
	synctest.Test(t, func(t *testing.T) {
		gate := &registrationEntropyGate{reader: original, entered: make(chan struct{}), release: make(chan struct{})}
		cryptoRand.Reader = gate
		listener := &registrationListener{accepted: make(chan struct{}), closed: make(chan struct{})}
		defer listener.Close() //nolint:errcheck // Release actual Serve on assertion failure.
		var release sync.Once
		defer release.Do(func() { close(gate.release) })
		owner := &Server{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
		network := &charmssh.Server{}
		started := make(chan error, 1)
		go func() { started <- owner.serveListener(network, listener) }()
		select {
		case <-gate.entered:
		case err := <-started:
			t.Fatalf("actual Serve returned before reaching signer generation: %v", err)
		}
		synctest.Wait()
		select {
		case err := <-started:
			t.Fatalf("serving returned before actual listener admission: %v", err)
		default:
		}
		release.Do(func() { close(gate.release) })
		<-listener.accepted
		synctest.Wait()
		select {
		case err := <-started:
			if err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatal("first actual Accept did not complete listener admission")
		}
		if err := network.Shutdown(context.Background()); err != nil {
			t.Fatal(err)
		}
		select {
		case <-listener.closed:
		default:
			t.Fatal("Shutdown did not find the listener admitted before readiness")
		}
	})
}

type registrationEntropyFailure struct {
	err error
}

func (r registrationEntropyFailure) Read([]byte) (int, error) { return 0, r.err }

// TestSSHListenerReadinessReportsEarlyServeFailure drives actual Serve through
// ensureHostSigner's genuine entropy failure before trackListener/Accept. The
// real serveListener caller must finish with that error and Serve must close
// its listener, without requiring a first Accept that will never happen.
func TestSSHListenerReadinessReportsEarlyServeFailure(t *testing.T) {
	// Go 1.26 RSA ignores custom entropy unless this documented compatibility
	// setting is enabled. SSH tests are serial; restore the global reader only
	// after synctest has joined the actual serving goroutine.
	t.Setenv("GODEBUG", os.Getenv("GODEBUG")+",cryptocustomrand=1")
	original := cryptoRand.Reader
	defer func() { cryptoRand.Reader = original }()
	want := errors.New("host-signer entropy unavailable")
	cryptoRand.Reader = registrationEntropyFailure{err: want}
	synctest.Test(t, func(t *testing.T) {
		listener := &registrationListener{accepted: make(chan struct{}), closed: make(chan struct{})}
		defer listener.Close() //nolint:errcheck // Rescue the real Serve listener on failure.
		owner := &Server{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
		network := &charmssh.Server{}
		if got := owner.serveListener(network, listener); !errors.Is(got, want) {
			t.Fatalf("actual pre-Accept Serve error = %v, want %v", got, want)
		}
		select {
		case <-listener.closed:
		default:
			t.Fatal("actual early Serve failure did not close its listener")
		}
		select {
		case <-listener.accepted:
			t.Fatal("Serve attempted Accept after host-signer generation failed")
		default:
		}
	})
}

// TestSSHStartStopImmediatelyJoinsEveryListener uses the production constructor,
// listener binding and Start/Stop, without dial/poll readiness or a sleep. Stop
// must see every bound primary/extra listener as soon as Start returns.
func TestSSHStartStopImmediatelyJoinsEveryListener(t *testing.T) {
	for _, test := range []struct {
		name  string
		count int
	}{{name: "primary", count: 1}, {name: "primary-and-extra", count: 3}} {
		t.Run(test.name, func(t *testing.T) {
			count := test.count
			addresses := make([]string, count)
			for i := range addresses {
				addresses[i] = "127.0.0.1:0"
			}
			server, err := NewServer(Config{
				Listen: addresses[0], ListenAddrs: addresses,
				HostKeyPath: filepath.Join(t.TempDir(), "host-key"),
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := server.Start(context.Background(), nil, nil); err != nil {
				t.Fatal(err)
			}
			defer server.Stop(context.Background()) //nolint:errcheck // Rescue on assertion failure.
			listeners := append([]net.Listener{server.listener}, server.extraListeners...)
			if len(listeners) != count {
				t.Fatalf("bound listeners = %d, want %d", len(listeners), count)
			}
			if err := server.Stop(context.Background()); err != nil {
				t.Fatal(err)
			}
			for i, listener := range listeners {
				conn, err := listener.Accept()
				if conn != nil {
					_ = conn.Close()
				}
				if !errors.Is(err, net.ErrClosed) {
					t.Errorf("listener %d after immediate Stop = %v, want closed", i, err)
				}
			}
		})
	}
}
