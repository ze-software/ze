// Design: docs/architecture/plugin-manager-wiring.md -- process lifetime ownership
// Related: shutdown.go -- startup publication and terminal cleanup transfer
package plugin

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	parent "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// Sealing admission must not wait on the startup lock. A later first spawn
// must refuse rather than publish an owner after the seal returned.
func TestShutdownSealDoesNotWaitForSpawnLock(t *testing.T) {
	m := NewManager()
	if err := m.StartAll(t.Context(), nil, nil); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	sealed := make(chan any, 1)
	go func() {
		finish := m.SealProcessAdmission()
		finish()
		sealed <- m.GetProcessManager()
	}()
	select {
	case pm := <-sealed:
		if pm != nil {
			m.mu.Unlock()
			t.Fatal("unstarted manager produced a process owner")
		}
	case <-time.After(time.Second):
		m.mu.Unlock()
		t.Fatal("seal waited for the process-start lock")
	}
	m.mu.Unlock()
	if err := m.SpawnMore([]parent.PluginConfig{{Name: "sealed-start", Internal: true}}); !errors.Is(err, process.ErrDaemonStopping) {
		t.Fatalf("SpawnMore after seal = %v, want ErrDaemonStopping", err)
	}
	if pm := m.GetProcessManager(); pm != nil {
		t.Fatal("sealed startup published a process manager")
	}
}

// A failed startup may call stopAllLocked while the server holds dependency
// cleanup open. That path must leave the captured manager parent and provider
// live until the terminal owner has joined the provider and finishes transfer.
func TestShutdownSealKeepsParentLiveUntilOrderedOwnerFinishes(t *testing.T) {
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	const name = "test-sealed-manager-provider"
	if err := registry.Register(registry.Registration{Name: name, Description: "test terminal manager cleanup", CLIHandler: func([]string) int { return 0 }, RunEngine: func(conn net.Conn) int {
		var b [1]byte
		for {
			if _, err := conn.Read(b[:]); err != nil {
				return 0
			}
		}
	}}); err != nil {
		t.Fatal(err)
	}
	m := NewManager()
	if err := m.StartAll(t.Context(), nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := m.SpawnMore([]parent.PluginConfig{{Name: name, Internal: true}}); err != nil {
		t.Fatal(err)
	}
	finish := m.SealProcessAdmission()
	pm, ok := m.GetProcessManager().(*process.ProcessManager)
	if !ok {
		t.Fatal("seal lost the published process owner")
	}
	t.Cleanup(finish)
	if pm == nil {
		t.Fatal("seal lost the published owner")
	}
	provider := pm.GetProcess(name)
	if provider == nil {
		t.Fatal("published owner lost its provider")
	}
	t.Cleanup(func() { pm.StopInOrder(func([]parent.PluginConfig) [][]string { return [][]string{{name}} }) })
	m.mu.Lock()
	m.stopAllLocked()
	m.mu.Unlock()
	if m.ctx.Err() != nil {
		t.Fatal("failed startup canceled the ordered owner's parent")
	}
	select {
	case <-provider.Done():
		t.Fatal("failed startup stopped provider before ordered cleanup")
	default:
	}
	pm.StopInOrder(func([]parent.PluginConfig) [][]string { return [][]string{{name}} })
	select {
	case <-provider.EngineDone():
	default:
		t.Fatal("ordered owner did not join provider finalizers")
	}
	finish()
	if !errors.Is(m.ctx.Err(), context.Canceled) {
		t.Fatalf("finished owner left parent live: %v", m.ctx.Err())
	}
}

// Addr is reached from the actual external Process.Start while SpawnMore holds
// the manager startup lock. Refuse before forking after releasing the gate:
// this question needs the lock interleaving, not a shell or TLS client.
type shutdownAddrListener struct {
	net.Listener
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (l *shutdownAddrListener) Addr() net.Addr {
	l.once.Do(func() { close(l.entered); <-l.release })
	return &net.UnixAddr{Name: "invalid-connect-back-address", Net: "unix"}
}

// Seal then GetProcessManager must capture the published owner while the real
// SpawnMore call remains inside Start. A getter on mu blocks before Stop can
// wake connect-back. The address refusal keeps this test independent of forks.
// MUTATION: restore GetProcessManager's mu.RLock; capture then misses its bound.
func TestShutdownSealCapturesOwnerWhileSpawnMoreHoldsStartupLock(t *testing.T) {
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	gated := &shutdownAddrListener{Listener: listener, entered: make(chan struct{}), release: make(chan struct{})}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(gated.release) }) }
	m := NewManager()
	if err := m.StartAll(t.Context(), nil, nil); err != nil {
		t.Fatal(err)
	}
	m.acceptor = ipc.NewPluginAcceptor(gated, "a-plugin-secret-of-at-least-32-chars", nil)
	started := make(chan struct{})
	var spawnErr error
	t.Cleanup(func() {
		release()
		m.acceptor.Stop()
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Error("gated SpawnMore did not return")
		}
		if err := m.StopAll(t.Context()); err != nil {
			t.Error(err)
		}
	})
	go func() {
		spawnErr = m.SpawnMore([]parent.PluginConfig{{Name: "capture-owner", Run: "unused-before-address-refusal"}})
		close(started)
	}()
	select {
	case <-gated.entered:
	case <-time.After(time.Second):
		t.Fatal("SpawnMore did not enter its process-start gate")
	}
	type capturedOwner struct {
		pm     *process.ProcessManager
		finish func()
	}
	captured := make(chan capturedOwner, 1)
	go func() {
		finish := m.SealProcessAdmission()
		pm, ok := m.GetProcessManager().(*process.ProcessManager)
		if !ok {
			captured <- capturedOwner{finish: finish}
			return
		}
		captured <- capturedOwner{pm: pm, finish: finish}
	}()
	select {
	case owner := <-captured:
		defer owner.finish()
		if owner.pm == nil {
			t.Fatal("seal lost the process owner published before Start")
		}
		release()
		owner.pm.StopInOrder(func([]parent.PluginConfig) [][]string { return nil })
		select {
		case <-started:
			if spawnErr == nil {
				t.Fatal("invalid address unexpectedly started a plugin")
			}
		case <-time.After(time.Second):
			t.Fatal("gated SpawnMore did not finish its refused Start")
		}
		if m.GetProcessManager() != owner.pm {
			t.Fatal("sealed owner changed while its generation retired")
		}
	case <-time.After(time.Second):
		t.Fatal("seal then getter waited for SpawnMore's startup lock")
	}
}

// A reusable Stop already joining its process owner can lose to the terminal
// seal. Its later bookkeeping must keep the exact captured owner available.
// MUTATION: clear procManager without rechecking admissionClosed after Stop.
func TestShutdownSealKeepsOwnerCapturedDuringReusableStop(t *testing.T) {
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	const name = "test-seal-during-reusable-stop"
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	if err := registry.Register(registry.Registration{Name: name, Description: "test seal during stop", CLIHandler: func([]string) int { return 0 }, RunEngine: func(conn net.Conn) int {
		var b [1]byte
		for {
			if _, err := conn.Read(b[:]); err != nil {
				break
			}
		}
		close(entered)
		<-release
		return 0
	}}); err != nil {
		t.Fatal(err)
	}
	m := NewManager()
	if err := m.StartAll(t.Context(), nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := m.SpawnMore([]parent.PluginConfig{{Name: name, Internal: true}}); err != nil {
		t.Fatal(err)
	}
	original := m.GetProcessManager()
	stopped := make(chan struct{})
	var stopErr error
	go func() { stopErr = m.StopAll(t.Context()); close(stopped) }()
	t.Cleanup(func() {
		unblock()
		select {
		case <-stopped:
			if stopErr != nil {
				t.Error(stopErr)
			}
		case <-time.After(time.Second):
			t.Error("reusable Stop did not finish after releasing cleanup")
		}
	})
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("reusable Stop did not reach engine cleanup")
	}
	finish := m.SealProcessAdmission()
	defer finish()
	if m.GetProcessManager() != original {
		t.Fatal("seal changed the process owner")
	}
	unblock()
	select {
	case <-stopped:
		if stopErr != nil {
			t.Fatal(stopErr)
		}
	case <-time.After(time.Second):
		t.Fatal("reusable Stop did not join the released engine")
	}
	if m.GetProcessManager() != original {
		t.Fatal("reusable Stop erased the terminal owner's captured pointer")
	}
}

// IssueLeaf is the existing injected authority boundary in ensureAcceptor.
// Holding it means actual SpawnMore passed its entry check but has not yet
// published a ProcessManager. The existing valid certificate fixture keeps
// listener preparation healthy when the gate is released.
type shutdownIssueAuthority struct {
	*testAuthority
	entered chan struct{}
	release chan struct{}
}

func (a *shutdownIssueAuthority) IssueLeaf(name string, hosts []string) (tls.Certificate, error) {
	close(a.entered)
	<-a.release
	return a.testAuthority.IssueLeaf(name, hosts)
}

// A terminal seal that wins during real acceptor preparation must still refuse
// the first process publication after preparation returns. This is the gap the
// entry-only seal test cannot select, without adding a production test seam.
// MUTATION: remove prepareProcessManager's admissionClosed recheck; SpawnMore
// then publishes a manager and starts this engine after the seal returned nil.
func TestShutdownSealRefusesOwnerAfterAcceptorPreparation(t *testing.T) {
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	const name = "test-seal-during-acceptor-preparation"
	var starts atomic.Int64
	if err := registry.Register(registry.Registration{Name: name, Description: "test admission during acceptor preparation", CLIHandler: func([]string) int { return 0 }, RunEngine: func(conn net.Conn) int {
		starts.Add(1)
		if err := conn.Close(); err != nil {
			return 1
		}
		return 0
	}}); err != nil {
		t.Fatal(err)
	}
	authority := &shutdownIssueAuthority{testAuthority: newTestAuthority(t), entered: make(chan struct{}), release: make(chan struct{})}
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(authority.release) }) }
	m := NewManager()
	m.SetHubAuthority(authority)
	m.SetHubConfig(&parent.HubConfig{Servers: []parent.HubServerConfig{{Name: "local", Host: "127.0.0.1", Port: 0, Secret: "seal-during-preparation-secret"}}})
	if err := m.StartAll(t.Context(), nil, nil); err != nil {
		t.Fatal(err)
	}
	spawned := make(chan struct{})
	var spawnErr error
	t.Cleanup(func() {
		unblock()
		select {
		case <-spawned:
		case <-time.After(time.Second):
			t.Error("SpawnMore did not return after certificate issuance was released")
		}
		if pm, ok := m.GetProcessManager().(*process.ProcessManager); ok {
			if pm != nil {
				pm.StopInOrder(func([]parent.PluginConfig) [][]string { return nil })
			}
		}
		if m.acceptor != nil {
			m.acceptor.Stop()
		}
		if err := m.StopAll(t.Context()); err != nil {
			t.Error(err)
		}
	})
	go func() { spawnErr = m.SpawnMore([]parent.PluginConfig{{Name: name, Internal: true}}); close(spawned) }()
	select {
	case <-authority.entered:
	case <-time.After(time.Second):
		t.Fatal("SpawnMore did not reach real certificate preparation")
	}
	finish := m.SealProcessAdmission()
	defer finish()
	if m.GetProcessManager() != nil {
		t.Fatal("first owner was published before acceptor preparation completed")
	}
	unblock()
	select {
	case <-spawned:
	case <-time.After(time.Second):
		t.Fatal("SpawnMore did not return after releasing acceptor preparation")
	}
	if !errors.Is(spawnErr, process.ErrDaemonStopping) {
		t.Fatalf("SpawnMore after preparation seal = %v, want ErrDaemonStopping", spawnErr)
	}
	if m.GetProcessManager() != nil {
		t.Fatal("acceptor preparation published a process owner after its terminal seal")
	}
	if starts.Load() != 0 {
		t.Fatal("acceptor preparation started an engine after its terminal seal")
	}
	if m.acceptor == nil {
		t.Fatal("certificate preparation never produced its actual acceptor")
	}
}
