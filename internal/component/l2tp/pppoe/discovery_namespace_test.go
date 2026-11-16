// Design: docs/architecture/l2tp/bng-5-pppoe.md -- discovery namespace ownership

package pppoe

import (
	"errors"
	"reflect"
	"testing"
)

type discoveryNamespaceFixture struct {
	trace []string
	fail  string
	cause error
}

func (f *discoveryNamespaceFixture) step(name string) error {
	f.trace = append(f.trace, name)
	if f.fail == name {
		return f.cause
	}
	return nil
}

func (f *discoveryNamespaceFixture) LockThread()   { f.trace = append(f.trace, "lock") }
func (f *discoveryNamespaceFixture) UnlockThread() { f.trace = append(f.trace, "unlock") }
func (f *discoveryNamespaceFixture) Current() (discoveryNamespace, error) {
	if err := f.step("current"); err != nil {
		return nil, err
	}
	return discoveryNamespaceFixtureHandle{f: f, name: "original"}, nil
}
func (f *discoveryNamespaceFixture) Named(name string) (discoveryNamespace, error) {
	if err := f.step("named:" + name); err != nil {
		return nil, err
	}
	return discoveryNamespaceFixtureHandle{f: f, name: "target"}, nil
}
func (f *discoveryNamespaceFixture) OpenSocket() (int, error) {
	if err := f.step("socket"); err != nil {
		return -1, err
	}
	return 0, nil // fd zero is an owned Unix descriptor too.
}
func (f *discoveryNamespaceFixture) CloseSocket(fd int) error {
	if fd != 0 {
		return errors.New("wrong socket closed")
	}
	return f.step("socket-close")
}

type discoveryNamespaceFixtureHandle struct {
	f    *discoveryNamespaceFixture
	name string
}

func (h discoveryNamespaceFixtureHandle) Enter() error { return h.f.step("enter:" + h.name) }
func (h discoveryNamespaceFixtureHandle) Close() error { return h.f.step("close:" + h.name) }

// Goal: publish a socket only after the original namespace is restored and
// both namespace handles are closed. The fake records the actual helper order.
func TestDiscoveryNamespacePublishesAfterRestoration(t *testing.T) {
	f := &discoveryNamespaceFixture{}
	fd, err := openDiscoveryNamespace("dataplane", f)
	if err != nil {
		t.Fatal(err)
	}
	if fd != 0 {
		t.Fatalf("socket = %d, want descriptor zero", fd)
	}
	want := []string{"lock", "current", "named:dataplane", "enter:target", "socket", "enter:original", "close:target", "close:original", "unlock"}
	if !reflect.DeepEqual(f.trace, want) {
		t.Fatalf("operations = %v, want %v", f.trace, want)
	}
}

// Goal: every failed namespace or socket operation refuses publication, closes
// acquired resources, and only restores a thread which was actually changed.
func TestDiscoveryNamespaceFailureReleasesResources(t *testing.T) {
	cause := errors.New("namespace operation failed")
	for _, tc := range []struct {
		fail string
		want []string
	}{
		{"current", []string{"lock", "current", "unlock"}},
		{"named:dataplane", []string{"lock", "current", "named:dataplane", "close:original", "unlock"}},
		{"enter:target", []string{"lock", "current", "named:dataplane", "enter:target", "close:target", "close:original", "unlock"}},
		{"socket", []string{"lock", "current", "named:dataplane", "enter:target", "socket", "enter:original", "close:target", "close:original", "unlock"}},
		{"enter:original", []string{"lock", "current", "named:dataplane", "enter:target", "socket", "enter:original", "close:target", "close:original", "socket-close"}},
		{"close:target", []string{"lock", "current", "named:dataplane", "enter:target", "socket", "enter:original", "close:target", "close:original", "socket-close", "unlock"}},
		{"close:original", []string{"lock", "current", "named:dataplane", "enter:target", "socket", "enter:original", "close:target", "close:original", "socket-close", "unlock"}},
	} {
		t.Run(tc.fail, func(t *testing.T) {
			f := &discoveryNamespaceFixture{fail: tc.fail, cause: cause}
			fd, err := openDiscoveryNamespace("dataplane", f)
			if !errors.Is(err, cause) {
				t.Fatalf("error = %v, want %v", err, cause)
			}
			if fd != -1 {
				t.Fatalf("failed creation published socket %d", fd)
			}
			if !reflect.DeepEqual(f.trace, tc.want) {
				t.Fatalf("operations = %v, want %v", f.trace, tc.want)
			}
		})
	}
}

// Goal: an empty namespace retains the caller's namespace without changing a
// thread or opening namespace handles; its one socket still has a clear owner.
func TestDiscoveryNamespaceEmptyDoesNotSwitchThreads(t *testing.T) {
	f := &discoveryNamespaceFixture{}
	fd, err := openDiscoveryNamespace("", f)
	if err != nil {
		t.Fatal(err)
	}
	if fd != 0 {
		t.Fatalf("socket = %d, want 0", fd)
	}
	if !reflect.DeepEqual(f.trace, []string{"socket"}) {
		t.Fatalf("operations = %v", f.trace)
	}
}
