// Design: docs/architecture/core-design.md -- atomic reactor authority handover.
package plugin

import (
	"reflect"
	"sync"
	"testing"
)

type handoverTestReactor struct {
	mockReactor
	mu         sync.Mutex
	tree       map[string]any
	onRegister func()
	onSet      func()
}

func (r *handoverTestReactor) GetConfigTree() map[string]any {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tree
}

func (r *handoverTestReactor) SetConfigTree(tree map[string]any) {
	if r.onSet != nil {
		r.onSet()
	}
	r.mu.Lock()
	r.tree = tree
	r.mu.Unlock()
}

func (r *handoverTestReactor) RegisterCacheConsumer(name string, unordered bool) {
	if r.onRegister != nil {
		r.onRegister()
	}
	r.mockReactor.RegisterCacheConsumer(name, unordered)
}

// The replay callback is the exact held-call boundary used by attachment.
// A live reader cannot see a partially prepared lifetime, and early/late cache
// declarations reach that lifetime once, without resurrecting an unregistered one.
func TestCoordinatorAttachPublishesOnlyPreparedAuthority(t *testing.T) {
	old := map[string]any{"authority": "previous"}
	candidate := map[string]any{"authority": "candidate"}
	c := NewCoordinator(old)
	c.RegisterCacheConsumer("early", true)
	c.RegisterCacheConsumer("removed", true)
	c.UnregisterCacheConsumer("removed")
	entered, release := make(chan struct{}), make(chan struct{})
	var unblock sync.Once
	t.Cleanup(func() { unblock.Do(func() { close(release) }) })
	r := &handoverTestReactor{tree: candidate, onRegister: func() { close(entered); <-release }}
	attached := make(chan error, 1)
	go func() { attached <- c.AttachReactor(r) }()
	<-entered
	if c.mu.TryRLock() {
		published := c.reactors["bgp"]
		c.mu.RUnlock()
		if published != nil {
			t.Error("reactor became publicly visible before its cache replay completed")
		}
	}
	read := make(chan map[string]any, 1)
	go func() { read <- c.GetConfigTree() }()
	late := make(chan struct{})
	go func() { c.RegisterCacheConsumer("late", false); close(late) }()
	// Only the early replay blocks; the late declaration must finish ordinarily.
	r.onRegister = nil
	unblock.Do(func() { close(release) })
	if err := <-attached; err != nil {
		t.Fatal(err)
	}
	<-late
	if got := <-read; !reflect.DeepEqual(got, candidate) {
		t.Errorf("concurrent reader observed %v, want selected candidate %v", got, candidate)
	}
	if got := c.GetConfigTree(); !reflect.DeepEqual(got, candidate) {
		t.Errorf("attach replaced prepared authority with prior committed map: %v", got)
	}
	for _, name := range []string{"early", "late"} {
		if got := r.cacheRegisterCalls[name]; got != 1 {
			t.Errorf("%s registration count=%d, want exactly once", name, got)
		}
	}
	if _, present := r.cacheConsumers["removed"]; present {
		t.Error("attach resurrected a removed cache consumer")
	}
	t.Run("uninitialized authority refuses", func(t *testing.T) {
		empty := NewCoordinator(old)
		if err := empty.AttachReactor(&handoverTestReactor{}); err == nil {
			t.Error("uninitialized reactor was published using implicit prior-map fallback")
		}
	})
}

// SetConfigTree must remain owned until the selected reactor has accepted it.
// Otherwise detach snapshots the old map and the accepted write is stranded on
// the detached lifetime. The mutex probe is deterministic while the setter is
// held at its real delegation boundary; no scheduling sleep proves exclusion.
func TestCoordinatorDetachSerializesAcceptedAuthority(t *testing.T) {
	old := map[string]any{"authority": "previous"}
	accepted := map[string]any{"authority": "accepted"}
	c := NewCoordinator(old)
	r := &handoverTestReactor{tree: old}
	if err := c.AttachReactor(r); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var unblock sync.Once
	t.Cleanup(func() { unblock.Do(func() { close(release) }) })
	r.onSet = func() { close(entered); <-release }
	written := make(chan struct{})
	go func() { c.SetConfigTree(accepted); close(written) }()
	<-entered
	if c.mu.TryRLock() {
		c.mu.RUnlock()
		t.Error("configuration delegation released authority ownership before the map write")
	}
	detached := make(chan error, 1)
	go func() { detached <- c.DetachReactor(r) }()
	unblock.Do(func() { close(release) })
	<-written
	if err := <-detached; err != nil {
		t.Fatal(err)
	}
	if got := c.GetConfigTree(); !reflect.DeepEqual(got, accepted) {
		t.Errorf("detach retained %v, want latest accepted authority %v", got, accepted)
	}
	if c.Reactor("bgp") != nil {
		t.Error("detached reactor remained published")
	}
}
