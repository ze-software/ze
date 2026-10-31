package network

import (
	"context"
	"errors"
	"testing"
)

// TestAddressWatchRegistry proves the one-slot address watcher registry: with
// nothing registered WatchAddresses answers ErrAddressWatchNotRegistered,
// never a channel that stays silent; once registered it calls the watcher; a
// second registration and a nil watcher are build defects. The slot is saved
// and restored around the test, because the netlink backend fills it at
// init() in a binary that links it.
func TestAddressWatchRegistry(t *testing.T) {
	addressWatch.Lock()
	saved := addressWatch.watch
	addressWatch.watch = nil
	addressWatch.Unlock()
	t.Cleanup(func() {
		addressWatch.Lock()
		addressWatch.watch = saved
		addressWatch.Unlock()
	})

	changes, err := WatchAddresses(context.Background())
	if !errors.Is(err, ErrAddressWatchNotRegistered) {
		t.Fatalf("unregistered watch answered %v, want ErrAddressWatchNotRegistered", err)
	}
	if changes != nil {
		t.Fatal("unregistered watch answered a channel")
	}

	want := make(chan struct{})
	RegisterAddressWatcher(func(context.Context) (<-chan struct{}, error) { return want, nil })
	got, err := WatchAddresses(context.Background())
	if err != nil {
		t.Fatalf("registered watch: %v", err)
	}
	if got != want {
		t.Fatal("WatchAddresses did not answer the registered watcher's channel")
	}

	for name, register := range map[string]func(){
		"second": func() { RegisterAddressWatcher(func(context.Context) (<-chan struct{}, error) { return want, nil }) },
		"nil":    func() { RegisterAddressWatcher(nil) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("registration did not panic")
				}
			}()
			register()
		})
	}
}
