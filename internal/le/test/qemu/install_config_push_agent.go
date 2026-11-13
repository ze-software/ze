// Design: docs/architecture/testing/qemu-integration.md -- disposable native push identity.
package testqemu

import (
	"crypto/ed25519"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/ssh/agent"
)

// configPushAgent owns its listener and every accepted worker. Its zero value
// is uninitialized; only configPushIdentity creates an agent. A successful
// configPushIdentity MUST be paired with stop; stop MUST join every worker.
type configPushAgent struct {
	listener    net.Listener
	keyring     agent.Agent
	mu          sync.Mutex
	connections map[net.Conn]struct{}
	closed      chan struct{}
	workers     sync.WaitGroup
	once        sync.Once
	stopErr     error
}

func (*Installer) configPushIdentity(work string, private ed25519.PrivateKey) (string, string, func() error, error) {
	if len(private) != ed25519.PrivateKeySize {
		return "", "", nil, errors.New("native push identity is not an Ed25519 private key")
	}
	home, socket := filepath.Join(work, "home"), filepath.Join(work, "agent.sock")
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		return "", "", nil, err
	}
	keyring := agent.NewKeyring()
	if err := keyring.Add(agent.AddedKey{PrivateKey: private}); err != nil {
		return "", "", nil, err
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		return "", "", nil, err
	}
	if err := os.Chmod(socket, 0o600); err != nil {
		return "", "", nil, errors.Join(err, listener.Close())
	}
	owned := &configPushAgent{listener: listener, keyring: keyring, connections: make(map[net.Conn]struct{}), closed: make(chan struct{})}
	owned.workers.Go(owned.accept)
	return home, socket, owned.stop, nil
}

func (owned *configPushAgent) accept() {
	for {
		connection, err := owned.listener.Accept()
		if err != nil {
			owned.mu.Lock()
			select {
			case <-owned.closed:
			default:
				owned.stopErr = errors.Join(owned.stopErr, err)
			}
			owned.mu.Unlock()
			return
		}
		owned.mu.Lock()
		select {
		case <-owned.closed:
			owned.stopErr = errors.Join(owned.stopErr, connection.Close())
			owned.mu.Unlock()
			return
		default:
		}
		// The serial proof uses one agent client at a time. Bound even a
		// misbehaving child before giving it a worker.
		if len(owned.connections) == 8 {
			owned.stopErr = errors.Join(owned.stopErr, connection.Close())
			owned.mu.Unlock()
			continue
		}
		owned.connections[connection] = struct{}{}
		owned.workers.Go(func() { owned.serve(connection) })
		owned.mu.Unlock()
	}
}

func (owned *configPushAgent) serve(connection net.Conn) {
	err := agent.ServeAgent(owned.keyring, connection)
	owned.mu.Lock()
	defer owned.mu.Unlock()
	select {
	case <-owned.closed:
	default:
		if err != nil {
			if !errors.Is(err, io.EOF) {
				owned.stopErr = errors.Join(owned.stopErr, err)
			}
		}
	}
	delete(owned.connections, connection)
	if err := connection.Close(); err != nil {
		if !errors.Is(err, net.ErrClosed) {
			owned.stopErr = errors.Join(owned.stopErr, err)
		}
	}
}

func (owned *configPushAgent) stop() error {
	owned.once.Do(owned.close)
	return owned.stopErr
}

func (owned *configPushAgent) close() {
	owned.mu.Lock()
	close(owned.closed)
	owned.stopErr = errors.Join(owned.stopErr, owned.listener.Close())
	for connection := range owned.connections {
		if err := connection.Close(); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				owned.stopErr = errors.Join(owned.stopErr, err)
			}
		}
	}
	owned.mu.Unlock()
	owned.workers.Wait()
}
