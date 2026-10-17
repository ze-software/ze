package yang

import (
	"errors"
	"sync"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"
)

// sharedProbeModule is a valid module no shipped package registers, so its
// presence in a Resolved names the module set that Resolved was built from.
const sharedProbeModule = `module ze-probe-shared {
    namespace "urn:ze:probe-shared";
    prefix shared;
    leaf probe { type string; }
}`

// TestDefaultLoaderResolvesOncePerModuleSet proves DefaultLoader resolves a
// registered module set once and hands the same Resolved to every caller,
// sequential or concurrent, and that a change to the set is resolved again.
//
// VALIDATES: two calls, and 32 concurrent calls, answer one pointer; a module
// registered for the test is in the next answer and the cached one never held
// it; after the restore the answer no longer holds it.
// PREVENTS: every CLI model, editor session and restart resolving the whole
// model again (178 parallel .et tests starved a loaded machine), and a cache
// that answers a test from a module set it did not register.
// Method: pointer identity. Each resolution allocates a new Resolved, so one
// pointer across every call is one resolution.
func TestDefaultLoaderResolvesOncePerModuleSet(t *testing.T) {
	first, err := DefaultLoader()
	if err != nil {
		t.Fatalf("DefaultLoader: %v", err)
	}
	again, err := DefaultLoader()
	if err != nil {
		t.Fatalf("DefaultLoader, second call: %v", err)
	}
	if again != first {
		t.Fatal("a second DefaultLoader call resolved the unchanged module set again")
	}

	answers := make([]*Resolved, 32)
	var wg sync.WaitGroup
	for i := range answers {
		wg.Go(func() {
			answers[i], _ = DefaultLoader() //nolint:errcheck // the shipped set resolved above; the pointer is the assertion
		})
	}
	wg.Wait()
	for i, answer := range answers {
		if answer != first {
			t.Fatalf("concurrent caller %d got a Resolved of its own, not the shared one", i)
		}
	}

	restore := RegisterModuleForTest("ze-probe-shared.yang", sharedProbeModule)
	probed, err := DefaultLoader()
	if err != nil {
		restore()
		t.Fatalf("DefaultLoader with a probe registered: %v", err)
	}
	if probed == first {
		restore()
		t.Fatal("a registered probe module was answered from the cached Resolved of the set without it")
	}
	if probed.GetModule("ze-probe-shared") == nil {
		restore()
		t.Fatal("the Resolved answered after registering the probe does not hold it")
	}
	if first.GetModule("ze-probe-shared") != nil {
		restore()
		t.Fatal("the shared Resolved gained a module registered after it was resolved")
	}

	restore()
	restored, err := DefaultLoader()
	if err != nil {
		t.Fatalf("DefaultLoader after the restore: %v", err)
	}
	if restored.GetModule("ze-probe-shared") != nil {
		t.Fatal("after the restore DefaultLoader still answered the probe's module set")
	}
}

// TestDefaultLoaderSharedSchemaIsReadOnly proves the Resolved every caller
// shares cannot take a new module and is read, not rebuilt, by GetEntry.
//
// VALIDATES: a Loader over the shared module set refuses AddModuleFromText and
// Resolve with ErrLoaderResolved; GetEntry answers the same entry pointer on
// every call for every module, from 16 goroutines at once.
// PREVENTS: one caller's addition reaching every other holder of the shared
// schema unchecked, and GetEntry building entries concurrently into the shared
// set (a data race the -race run of this test catches).
// Method: the Loader is the only API that adds a module, so the test wraps the
// shared set in one; the concurrent walk runs under the race detector.
func TestDefaultLoaderSharedSchemaIsReadOnly(t *testing.T) {
	shared, err := DefaultLoader()
	if err != nil {
		t.Fatalf("DefaultLoader: %v", err)
	}

	loader := &Loader{modules: shared.set()}
	if err := loader.AddModuleFromText("ze-probe-shared.yang", sharedProbeModule); !errors.Is(err, ErrLoaderResolved) {
		t.Fatalf("adding a module to the shared set answered %v, want ErrLoaderResolved", err)
	}
	if _, err := loader.Resolve(); !errors.Is(err, ErrLoaderResolved) {
		t.Fatalf("resolving the shared set again answered %v, want ErrLoaderResolved", err)
	}
	if shared.GetModule("ze-probe-shared") != nil {
		t.Fatal("a refused addition still reached the shared set")
	}

	names := shared.ModuleNames()
	if len(names) == 0 {
		t.Fatal("the shared schema holds no module")
	}
	entries := make(map[string]*gyang.Entry, len(names))
	for _, name := range names {
		entries[name] = shared.GetEntry(name)
	}
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			for _, name := range names {
				if shared.GetEntry(name) != entries[name] {
					t.Errorf("GetEntry(%q) rebuilt the entry tree instead of reading the resolved one", name)
				}
			}
		})
	}
	wg.Wait()
}
