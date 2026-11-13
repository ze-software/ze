// Design: docs/architecture/appliance/remote-operations.md -- SSH key-authenticated config upload.
// Related: cmd_config_push_hostkey_test.go -- host trust policy controls.
// Related: cmd_config_push.go -- the actual agent/SSH transport exercised here.

package appliance

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// configPushSSHPeer owns one TCP connection, one real ssh-agent connection,
// and their workers. Close releases and joins both, including failed handshakes.
// Each fixture serves one upload; no worker survives test cleanup.
type configPushSSHPeer struct {
	listener      net.Listener
	agentListener net.Listener
	mu            sync.Mutex
	connections   []net.Conn
	workers       sync.WaitGroup
	closing       bool
	received      chan string
	status        uint32
	receipt       string
}

func configPushSigner(t *testing.T) (ssh.Signer, ed25519.PrivateKey) {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	return signer, private
}

func newConfigPushSSHPeer(t *testing.T, status uint32, receipt string, wrongHost bool) *configPushSSHPeer {
	t.Helper()
	host, _ := configPushSigner(t)
	identity, private := configPushSigner(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	agentPath := filepath.Join(t.TempDir(), "agent.sock")
	agentListener, err := net.Listen("unix", agentPath)
	if err != nil {
		_ = listener.Close() //nolint:errcheck // Fixture construction cleanup.
		t.Fatal(err)
	}
	peer := &configPushSSHPeer{
		listener: listener, agentListener: agentListener,
		received: make(chan string, 1), status: status, receipt: receipt,
	}
	t.Cleanup(peer.close)
	keyring := agent.NewKeyring()
	if err := keyring.Add(agent.AddedKey{PrivateKey: private}); err != nil {
		t.Fatal(err)
	}
	server := &ssh.ServerConfig{PublicKeyCallback: func(metadata ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if metadata.User() != "operator" {
			return nil, errors.New("unexpected SSH username")
		}
		if string(key.Marshal()) != string(identity.PublicKey().Marshal()) {
			return nil, errors.New("unexpected SSH identity")
		}
		return &ssh.Permissions{}, nil
	}}
	server.AddHostKey(host)
	peer.workers.Add(2)
	go peer.serveAgent(keyring)
	go peer.serveSSH(server)
	trust := host.PublicKey()
	if wrongHost {
		other, _ := configPushSigner(t)
		trust = other.PublicKey()
	}
	home := t.TempDir()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.Mkdir(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "known_hosts"), []byte(knownhosts.Line([]string{listener.Addr().String()}, trust)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("SSH_AUTH_SOCK", agentPath)
	return peer
}

func (p *configPushSSHPeer) accept(listener net.Listener) (net.Conn, error) {
	conn, err := listener.Accept()
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	if p.closing {
		p.mu.Unlock()
		_ = conn.Close() //nolint:errcheck // Cleanup won admission.
		return nil, net.ErrClosed
	}
	p.connections = append(p.connections, conn)
	p.mu.Unlock()
	return conn, nil
}

func (p *configPushSSHPeer) serveAgent(keyring agent.Agent) {
	defer p.workers.Done()
	conn, err := p.accept(p.agentListener)
	if err != nil {
		return
	}
	defer conn.Close()                  //nolint:errcheck // Fixture cleanup also closes admitted connections.
	_ = agent.ServeAgent(keyring, conn) //nolint:errcheck // The client normally closes after its upload.
}

func (p *configPushSSHPeer) serveSSH(config *ssh.ServerConfig) {
	defer p.workers.Done()
	conn, err := p.accept(p.listener)
	if err != nil {
		return
	}
	defer conn.Close() //nolint:errcheck // Fixture cleanup also closes admitted connections.
	transport, channels, _, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	defer transport.Close() //nolint:errcheck // Client may already have disconnected.
	request, ok := <-channels
	if !ok {
		return
	}
	if request.ChannelType() != "session" {
		_ = request.Reject(ssh.UnknownChannelType, "session required")
		return
	}
	channel, requests, err := request.Accept()
	if err != nil {
		return
	}
	defer channel.Close() //nolint:errcheck // Client may already have disconnected.
	operation, ok := <-requests
	if !ok {
		return
	}
	if operation.Type != "subsystem" {
		_ = operation.Reply(false, nil)
		return
	}
	var subsystem struct{ Name string }
	if err := ssh.Unmarshal(operation.Payload, &subsystem); err != nil {
		_ = operation.Reply(false, nil)
		return
	}
	if subsystem.Name != "ze-config-upload" {
		_ = operation.Reply(false, nil)
		return
	}
	if err := operation.Reply(true, nil); err != nil {
		return
	}
	var header [4]byte
	if _, err := io.ReadFull(channel, header[:]); err != nil {
		return
	}
	size := binary.BigEndian.Uint32(header[:])
	if size > 4<<20 {
		return
	}
	content := make([]byte, size)
	if _, err := io.ReadFull(channel, content); err != nil {
		return
	}
	var extra [1]byte
	n, err := channel.Read(extra[:])
	if n != 0 {
		return
	}
	if !errors.Is(err, io.EOF) {
		return
	}
	p.received <- string(content)
	if _, err := io.WriteString(channel, p.receipt); err != nil {
		return
	}
	_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{p.status})) //nolint:errcheck // Client closure is an expected end.
}

func (p *configPushSSHPeer) close() {
	p.mu.Lock()
	p.closing = true
	p.mu.Unlock()
	_ = p.listener.Close()      //nolint:errcheck // Repeated test cleanup is harmless.
	_ = p.agentListener.Close() //nolint:errcheck // Repeated test cleanup is harmless.
	p.mu.Lock()
	for _, conn := range p.connections {
		_ = conn.Close() //nolint:errcheck // Client closure is an expected end.
	}
	p.mu.Unlock()
	p.workers.Wait()
}

// TestConfigPushSSHUsesRealAgentAndUploadSubsystem requires the actual
// transport, rather than the replaceable command function, to deliver the
// exact private payload over a real pinned, key-authenticated SSH subsystem.
func TestConfigPushSSHUsesRealAgentAndUploadSubsystem(t *testing.T) {
	peer := newConfigPushSSHPeer(t, 0, "configuration applied\n", false)
	const content = "# private upload marker 8a3f\nset environment log level info\n"
	result := sshExecReal(peer.listener.Addr().String(), "operator", "ze-config-upload", content)
	if result.Err != nil {
		t.Fatalf("real agent-authenticated configuration upload failed: %v", result.Err)
	}
	select {
	case received := <-peer.received:
		if received != content {
			t.Fatal("SSH subsystem did not receive the exact configuration bytes")
		}
	default:
		t.Fatal("configuration push reported success without reaching its SSH receiver")
	}
}

// TestConfigPushSSHRefusalIsFailure requires a nonzero receiver exit to remain
// a failure even when the transport successfully delivered the complete body.
func TestConfigPushSSHRefusalIsFailure(t *testing.T) {
	peer := newConfigPushSSHPeer(t, 1, "configuration refused\n", false)
	result := sshExecReal(peer.listener.Addr().String(), "operator", "ze-config-upload", sshUploadTransportContent)
	if result.Err == nil {
		t.Fatal("remote upload refusal reported success")
	}
	select {
	case <-peer.received:
	default:
		t.Fatal("refusal test never reached the registered upload receiver")
	}
}

const sshUploadTransportContent = "set environment log level info\n"

// TestConfigPushSSHRequiresAcceptanceReceipt prevents zero-exit protocol
// confusion from being presented as a successful configuration application.
func TestConfigPushSSHRequiresAcceptanceReceipt(t *testing.T) {
	peer := newConfigPushSSHPeer(t, 0, "unknown operation\n", false)
	result := sshExecReal(peer.listener.Addr().String(), "operator", "ze-config-upload", sshUploadTransportContent)
	if result.Err == nil {
		t.Fatal("wrong protocol receipt reported a successful apply")
	}
	select {
	case <-peer.received:
	default:
		t.Fatal("receipt test never reached the registered upload receiver")
	}
}

// TestConfigPushSSHWrongHostKeyStopsBeforePayload checks the trust decision at
// the actual network boundary, in addition to the existing callback controls.
func TestConfigPushSSHWrongHostKeyStopsBeforePayload(t *testing.T) {
	peer := newConfigPushSSHPeer(t, 0, "configuration applied\n", true)
	result := sshExecReal(peer.listener.Addr().String(), "operator", "ze-config-upload", sshUploadTransportContent)
	if result.Err == nil {
		t.Fatal("changed host key was accepted by the real push transport")
	}
	if !strings.Contains(result.Err.Error(), "host key mismatch") {
		t.Fatalf("host key refusal lost its cause: %v", result.Err)
	}
	select {
	case <-peer.received:
		t.Fatal("configuration payload reached an untrusted SSH host")
	default:
	}
}
