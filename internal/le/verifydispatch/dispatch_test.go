// VALIDATES: verifyworktree dispatches exact registered identities inside the requested root.
// PREVENTS: a missing owner, missing handler, root leak, or concurrent override certifying a stage.
package verifydispatch

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/verifyengine"
)

const probeRoot = "verify-dispatch-probe"

type probeText string

func (p probeText) Text() string { return string(p) }

var probe struct {
	sync.Mutex
	registered sync.Once
	args       []string
	root       string
}

func registerProbe() {
	probe.registered.Do(func() {
		leroot.Register(probeRoot, leroot.GroupReport, func(args []string) (any, int) {
			root, err := lepath.Root()
			if err != nil {
				return probeText(err.Error()), 2
			}
			probe.Lock()
			probe.args = slices.Clone(args)
			probe.root = root
			probe.Unlock()
			if len(args) > 0 {
				if args[0] == "stderr" {
					return make(chan int), 99
				}
			}
			return probeText("captured stdout"), 37
		}, registry.Meta{
			Description: "verify dispatch capture probe",
			Mode:        "offline",
			Section:     registry.SectionTest,
		})
	})
}

func TestDispatchCapturesStdoutAndPreservesIdentityCodeArgsAndRoot(t *testing.T) {
	registerProbe()
	originalRoot := env.Get(lepath.RootKey)
	if err := env.Set(lepath.RootKey, "before-dispatch"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := env.Set(lepath.RootKey, originalRoot); err != nil {
			t.Error(err)
		}
	})

	root := t.TempDir()
	identity := verifyengine.Identity{Name: "ze-probe", Command: probeRoot, Args: []string{"alpha", "beta"}}
	result := RunAction(context.Background(), root, identity)
	if !result.Registered || !result.Completed || result.Code != 37 || result.Failure != nil {
		t.Fatalf("dispatch result = %#v", result)
	}
	if result.Identity.Name != identity.Name || result.Identity.Command != identity.Command {
		t.Errorf("identity = %#v, want %#v", result.Identity, identity)
	}
	if !slices.Equal(result.Identity.Args, identity.Args) {
		t.Errorf("identity args = %q, want %q", result.Identity.Args, identity.Args)
	}
	if result.Output != "captured stdout\n" {
		t.Errorf("stdout = %q", result.Output)
	}

	probe.Lock()
	gotArgs := slices.Clone(probe.args)
	gotRoot := probe.root
	probe.Unlock()
	if !slices.Equal(gotArgs, identity.Args) {
		t.Errorf("handler args = %q, want %q", gotArgs, identity.Args)
	}
	wantRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	if gotRoot != wantRoot {
		t.Errorf("handler root = %q, want %q", gotRoot, wantRoot)
	}
	if restored := env.Get(lepath.RootKey); restored != "before-dispatch" {
		t.Errorf("root restored to %q, want before-dispatch", restored)
	}
}

func TestDispatchCapturesStderrFromTheRegisteredHandler(t *testing.T) {
	registerProbe()
	identity := verifyengine.Identity{Name: "ze-probe-error", Command: probeRoot, Args: []string{"stderr"}}
	result := RunAction(context.Background(), t.TempDir(), identity)
	if !result.Registered || !result.Completed || result.Code != 1 {
		t.Fatalf("dispatch result = %#v", result)
	}
	if !strings.Contains(result.Output, "answered a payload that does not encode") {
		t.Errorf("stderr was not captured: %q", result.Output)
	}
}

func TestDispatchRefusesUnownedMissingAndMismatchedRoots(t *testing.T) {
	root := t.TempDir()
	identity := verifyengine.Identity{Name: "ze-refusal", Command: "refusal-probe", Args: []string{"exact"}}
	resolve := func() (string, error) { return root, nil }
	missing := func(string) registry.LocalDataHandler { return nil }

	unowned := dispatch(context.Background(), root, identity,
		func(string) bool { return false }, missing, resolve)
	if unowned.Registered || unowned.Completed || unowned.Code != 2 {
		t.Fatalf("unowned result = %#v", unowned)
	}
	if unowned.Failure == nil || unowned.Failure.Kind != "unowned-root" {
		t.Errorf("unowned failure = %#v", unowned.Failure)
	}

	missingHandler := dispatch(context.Background(), root, identity,
		func(string) bool { return true }, missing, resolve)
	if missingHandler.Registered || missingHandler.Completed || missingHandler.Code != 2 {
		t.Fatalf("missing-handler result = %#v", missingHandler)
	}
	if missingHandler.Failure == nil || missingHandler.Failure.Kind != "missing-handler" {
		t.Errorf("missing-handler failure = %#v", missingHandler.Failure)
	}

	otherRoot := t.TempDir()
	mismatched := dispatch(context.Background(), root, identity,
		func(string) bool { return true }, missing,
		func() (string, error) { return otherRoot, nil })
	if mismatched.Registered || mismatched.Completed || mismatched.Code != 2 {
		t.Fatalf("mismatched-root result = %#v", mismatched)
	}
	if mismatched.Failure == nil || mismatched.Failure.Kind != "root-mismatch" {
		t.Errorf("mismatched-root failure = %#v", mismatched.Failure)
	}

	absent := filepath.Join(root, "absent")
	missingRoot := dispatch(context.Background(), absent, identity,
		func(string) bool { return true }, missing, resolve)
	if missingRoot.Failure == nil || missingRoot.Failure.Kind != "root-missing" {
		t.Errorf("missing-root failure = %#v", missingRoot.Failure)
	}
}

func TestDispatchSerializesRootOverrides(t *testing.T) {
	firstRoot := t.TempDir()
	secondRoot := t.TempDir()
	firstEntered := make(chan struct{})
	secondEntered := make(chan struct{})
	var overlap atomic.Int32
	var active atomic.Int32

	handler := func(args []string) (any, int) {
		if active.Add(1) > 1 {
			overlap.Store(1)
		}
		defer active.Add(-1)
		if args[0] == "first" {
			close(firstEntered)
			select {
			case <-secondEntered:
			case <-time.After(100 * time.Millisecond):
			}
			return nil, 11
		}
		close(secondEntered)
		return nil, 12
	}
	lookup := func(string) registry.LocalDataHandler { return handler }
	owns := func(string) bool { return true }

	var first, second verifyengine.ActionResult
	var calls sync.WaitGroup
	calls.Add(2)
	go func() {
		defer calls.Done()
		first = dispatch(context.Background(), firstRoot,
			verifyengine.Identity{Name: "first", Command: "probe", Args: []string{"first"}},
			owns, lookup, lepath.Root)
	}()
	<-firstEntered
	go func() {
		defer calls.Done()
		second = dispatch(context.Background(), secondRoot,
			verifyengine.Identity{Name: "second", Command: "probe", Args: []string{"second"}},
			owns, lookup, lepath.Root)
	}()
	calls.Wait()

	if overlap.Load() != 0 {
		t.Error("two handlers observed different root overrides concurrently")
	}
	if !first.Completed || first.Code != 11 {
		t.Errorf("first result = %#v", first)
	}
	if !second.Completed || second.Code != 12 {
		t.Errorf("second result = %#v", second)
	}
}
