// Design: docs/architecture/static-routes.md -- device presence as an activity input
// Related: link.go -- the linkSource these fakes stand in for
// Related: inject_test.go, locrib_test.go -- the tests that drive them

package static

import (
	"fmt"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// fakeLinks is a linkSource a test drives: state answers what the test set,
// and send delivers one event on the live subscription of a name, as the iface
// resolver's fan-out does. canceled counts the subscriptions ended per name.
type fakeLinks struct {
	mu       sync.Mutex
	up       map[string]bool
	index    map[string]int
	subs     map[string]chan iface.LinkEvent
	canceled map[string]int
}

func newFakeLinks() *fakeLinks {
	return &fakeLinks{
		up:       map[string]bool{},
		index:    map[string]int{},
		subs:     map[string]chan iface.LinkEvent{},
		canceled: map[string]int{},
	}
}

func (f *fakeLinks) subscribe(name string) (<-chan iface.LinkEvent, func()) {
	ch := make(chan iface.LinkEvent, 8)
	f.mu.Lock()
	f.subs[name] = ch
	f.mu.Unlock()
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.subs[name] == ch {
				delete(f.subs, name)
			}
			f.canceled[name]++
			close(ch)
		})
	}
}

func (f *fakeLinks) state(name string) (bool, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.up[name], f.index[name]
}

// send delivers one event for name. It holds f.mu across the send so a
// concurrent cancel cannot close the channel under it.
func (f *fakeLinks) send(t *testing.T, name string, kind iface.LinkEventKind, index int) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	ch, ok := f.subs[name]
	if !ok {
		t.Fatalf("no live subscription for %q: the route manager is not watching it", name)
	}
	ch <- iface.LinkEvent{Name: name, Kind: kind, Index: index}
}

func (f *fakeLinks) subscribed(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.subs[name]
	return ok
}

func (f *fakeLinks) cancels(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.canceled[name]
}

// staticOp is one write the route manager made: verb is "apply" or "remove"
// for the named-table backend, "insert" or "remove" for the Loc-RIB sink.
type staticOp struct {
	verb  string
	route staticRoute
	path  locrib.Path
}

// opRecorder is a routeBackend and a routeSink at once that reports every
// write on a channel, so a test waits for the write a link event causes
// instead of sleeping. Safe for concurrent use: the link watcher writes from
// its own goroutine.
type opRecorder struct {
	ops chan *staticOp
}

func newOpRecorder() *opRecorder { return &opRecorder{ops: make(chan *staticOp, 64)} }

func (o *opRecorder) applyRoute(r staticRoute) error {
	o.ops <- &staticOp{verb: "apply", route: r}
	return nil
}

func (o *opRecorder) removeRoute(r staticRoute) error {
	o.ops <- &staticOp{verb: "remove", route: r}
	return nil
}

func (o *opRecorder) listRoutes() ([]installedStaticRoute, error) { return nil, nil }
func (o *opRecorder) close() error                                { return nil }

func (o *opRecorder) InsertForward(_ family.Family, prefix netip.Prefix, p locrib.Path) {
	o.ops <- &staticOp{verb: "insert", route: staticRoute{Prefix: prefix}, path: p}
}

func (o *opRecorder) Remove(_ family.Family, prefix netip.Prefix, _ redistevents.ProtocolID, _ uint32) {
	o.ops <- &staticOp{verb: "remove", route: staticRoute{Prefix: prefix}}
}

func (o *opRecorder) Flush() {}

// next returns the next n writes, failing the test when they do not arrive
// within a bound far above a watcher's latency.
func (o *opRecorder) next(t *testing.T, n int) []*staticOp {
	t.Helper()
	out := make([]*staticOp, 0, n)
	deadline := time.After(5 * time.Second)
	for len(out) < n {
		select {
		case op := <-o.ops:
			out = append(out, op)
		case <-deadline:
			t.Fatalf("got %d of %d expected writes: %+v", len(out), n, out)
		}
	}
	return out
}

// none fails the test when a write is pending. The caller makes it only after
// a synchronous call, or after next collected every write the event owed.
func (o *opRecorder) none(t *testing.T) {
	t.Helper()
	select {
	case op := <-o.ops:
		t.Fatalf("unexpected write %+v", op)
	default: // nothing pending, as expected
	}
}

// newLinkTestManager builds a route manager whose named-table writes and
// Loc-RIB writes both reach rec, and whose device facts come from links.
func newLinkTestManager(rec *opRecorder, links *fakeLinks) *routeManager {
	rm := newRouteManager(rec)
	rm.setLocRIB(nil, rec)
	rm.links = links
	return rm
}

// linkTestIfaceBackend is an iface.Backend that answers GetLink and
// GetInterface for a fixed set of names, so checkNextHopResolvable, which a
// main-table route runs at insert, resolves the device a test brought up. The
// embedded Backend is nil: a test that reaches any other method panics.
type linkTestIfaceBackend struct {
	iface.Backend
	byName map[string]int
}

func (b *linkTestIfaceBackend) Close() error { return nil }

func (b *linkTestIfaceBackend) GetInterface(name string) (*iface.InterfaceInfo, error) {
	idx, ok := b.byName[name]
	if !ok {
		return nil, fmt.Errorf("link test iface backend: interface %q not found", name)
	}
	return &iface.InterfaceInfo{Name: name, OsName: name, Index: idx, State: iface.LinkStateUp}, nil
}

func (b *linkTestIfaceBackend) GetLink(name string) (*iface.InterfaceInfo, error) {
	return b.GetInterface(name)
}

// linkTestIfaceBackendName is not "netlink" or "vpp": other test files of this
// binary register those.
const linkTestIfaceBackendName = "static-link-test"

var (
	linkTestIfaceBackendInstance = &linkTestIfaceBackend{byName: map[string]int{}}
	linkTestIfaceRegOnce         sync.Once
)

// loadLinkTestIfaceBackend loads the fake iface backend answering for names,
// registering it once per test binary (RegisterBackend refuses a duplicate and
// cannot unregister, so a second -count pass reuses the registration), and
// closes it on cleanup so a later no-backend test sees a clean global.
func loadLinkTestIfaceBackend(t *testing.T, names map[string]int) {
	t.Helper()
	linkTestIfaceRegOnce.Do(func() {
		if err := iface.RegisterBackend(linkTestIfaceBackendName, func() (iface.Backend, error) {
			return linkTestIfaceBackendInstance, nil
		}); err != nil {
			t.Fatalf("register link test iface backend: %v", err)
		}
	})
	linkTestIfaceBackendInstance.byName = names
	if err := iface.LoadBackend(linkTestIfaceBackendName); err != nil {
		t.Fatalf("load link test iface backend: %v", err)
	}
	t.Cleanup(func() { _ = iface.CloseBackend() })
}
