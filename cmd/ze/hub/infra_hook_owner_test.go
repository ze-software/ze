package hub

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/config/infra"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// infraHookHeldSSH isolates the hook's ownership join. The authenticated SSH
// history-owner tests separately exercise the real server's inner drain.
type infraHookHeldSSH struct {
	entered chan struct{}
	release chan struct{}
	ended   chan struct{}
	stops   int
}

func (*infraHookHeldSSH) Address() string { return "test" }

func (s *infraHookHeldSSH) Stop(context.Context) error {
	s.stops++
	close(s.entered)
	<-s.release
	close(s.ended)
	return nil
}

func runInfraHookOwner(params infra.HookParams, done chan<- struct{}) {
	infra.Run(params)
	close(done)
}

func stopInfraHookOwner(owner *pluginserver.Server, start, cleanupEnded <-chan struct{}, joined chan<- bool, done chan<- struct{}) {
	<-start
	owner.Stop()
	select {
	case <-cleanupEnded:
		joined <- true
	default:
		joined <- false
	}
	close(done)
}

// waitInfraHookStopSnapshot waits until owner has started Stop, observed as
// OnStop refusing a new hook. It answers ctx's error if that never happens.
func waitInfraHookStopSnapshot(ctx context.Context, owner *pluginserver.Server, ticks <-chan time.Time) error {
	for {
		if infraHookStopRefused(owner) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticks:
		}
	}
}

// infraHookStopRefused reports whether owner refuses a new stop hook, which
// Server.OnStop does only once Stop has started: the refusal is the snapshot
// the caller waits for, not a failure. An accepted probe hook is withdrawn.
func infraHookStopRefused(owner *pluginserver.Server) bool {
	withdraw, err := owner.OnStop(func() {})
	if err != nil {
		return true
	}
	withdraw()
	return false
}

func requireInfraHookPending(t *testing.T, done <-chan struct{}, operation string) {
	t.Helper()
	select {
	case <-done:
		t.Fatalf("%s returned before its owned cleanup joined", operation)
	default:
	}
}

// TestInfraHookJoinsRefusedSSHOwner proves a constructor admitted before stop
// cannot release its gate until its refused resource has stopped. Actual
// OnStop refusal establishes the snapshot without assuming callback order.
// MUTATION: dropping refused-resource cleanup leaves Stop unentered; detaching
// that cleanup lets infra.Run or owner.Stop return while the resource is held.
func TestInfraHookJoinsRefusedSSHOwner(t *testing.T) {
	resetAAABundleForTest(t)
	originalBuild := sshBuild
	t.Cleanup(func() {
		sshBuild = originalBuild
		infra.SetHook(nil)
	})
	operator := authWiringBcryptUser(t, "operator", "owner-secret", "admin")
	owner, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	// The real stop owner stays outside: synctest cannot quiesce a worker
	// blocked on builders' Mutex. Its channels MUST also be outside-owned.
	stopRequested := make(chan struct{})
	stopped := make(chan struct{})
	cleanupEnded := make(chan struct{})
	cleanupJoined := make(chan bool, 1)
	requestStop := sync.OnceFunc(func() { close(stopRequested) })
	go stopInfraHookOwner(owner, stopRequested, cleanupEnded, cleanupJoined, stopped)
	// The external worker MUST join after bubble cleanup releases both gates.
	defer func() {
		requestStop()
		<-stopped
	}()
	observerCtx, cancelObserver := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancelObserver()
	// Done allocates lazily: allocate outside so external cancellation never
	// closes a channel first created by the observer inside the bubble.
	_ = observerCtx.Done()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

	synctest.Test(t, func(t *testing.T) {
		resource := &infraHookHeldSSH{
			entered: make(chan struct{}),
			release: make(chan struct{}),
			ended:   cleanupEnded,
		}
		constructorEntered := make(chan struct{})
		constructorRelease := make(chan struct{})
		built := 0
		sshBuild = func(*sshBuildInputs) sshServer {
			built++
			close(constructorEntered)
			<-constructorRelease
			return resource
		}
		setupInfraHook(owner, nil, &configMutationOwner{}, func() ([]aaa.UserCredential, error) {
			return []aaa.UserCredential{operator}, nil
		}, nil)
		params := infra.HookParams{
			Reactor:   &authWiringReactor{},
			SSHConfig: infra.SSHExtractedConfig{HasConfig: true},
		}
		constructed := make(chan struct{})
		releaseConstructor := sync.OnceFunc(func() { close(constructorRelease) })
		releaseResource := sync.OnceFunc(func() { close(resource.release) })
		// The bubble worker MUST join, including after a failed assertion.
		// The external worker joins only after these gates are released.
		defer func() {
			releaseConstructor()
			releaseResource()
			<-constructed
		}()
		go runInfraHookOwner(params, constructed)
		synctest.Wait()
		select {
		case <-constructorEntered:
		default:
			t.Fatal("installed infra hook did not enter SSH construction")
		}
		requestStop()
		if err := waitInfraHookStopSnapshot(observerCtx, owner, ticker.C); err != nil {
			t.Fatalf("observe actual OnStop refusal: %v", err)
		}
		synctest.Wait()
		requireInfraHookPending(t, constructed, "infra.Run during construction")
		requireInfraHookPending(t, stopped, "owner.Stop during construction")

		releaseConstructor()
		synctest.Wait()
		select {
		case <-resource.entered:
		default:
			t.Fatal("refused SSH owner was not stopped")
		}
		requireInfraHookPending(t, resource.ended, "SSH owner Stop")
		requireInfraHookPending(t, constructed, "infra.Run during SSH cleanup")
		requireInfraHookPending(t, stopped, "owner.Stop during SSH cleanup")

		releaseResource()
		<-constructed
		<-stopped
		<-resource.ended
		if !<-cleanupJoined {
			t.Error("owner.Stop returned before SSH cleanup ended")
		}
		if built != 1 {
			t.Errorf("SSH constructions = %d, want 1", built)
		}
		if resource.stops != 1 {
			t.Errorf("SSH owner stops = %d, want 1", resource.stops)
		}
	})
}

// TestInfraHookRefusesFirstConstructionAfterStop proves the installed hook
// refuses work before infraSetup, independently of the AAA boot-build guard.
// MUTATION: removing the stopped guard resolves live users after daemon stop.
func TestInfraHookRefusesFirstConstructionAfterStop(t *testing.T) {
	resetAAABundleForTest(t)
	originalBuild := sshBuild
	t.Cleanup(func() {
		sshBuild = originalBuild
		infra.SetHook(nil)
	})
	owner, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	built := 0
	sshBuild = func(*sshBuildInputs) sshServer {
		built++
		return nil
	}
	liveCalls := 0
	setupInfraHook(owner, nil, &configMutationOwner{}, func() ([]aaa.UserCredential, error) {
		liveCalls++
		return []aaa.UserCredential{{Name: "operator"}}, nil
	}, nil)
	owner.Stop()
	infra.Run(infra.HookParams{
		Reactor:   &authWiringReactor{},
		SSHConfig: infra.SSHExtractedConfig{HasConfig: true},
	})
	if liveCalls != 0 {
		t.Errorf("live-user resolutions after stop = %d, want 0", liveCalls)
	}
	if built != 0 {
		t.Errorf("SSH constructions after stop = %d, want 0", built)
	}
}
