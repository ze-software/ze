//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- explicit test fragment permission.
// Related: child_scope.go -- production does not grant unnegotiated permission.

package engine

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// fragmentPermittedDataplane changes only the test SA's noninitial-fragment permission.
// RFC 4301 Section 7.3 requires that permission before constrained SAs carry fragments.
// The real backend still installs every state and policy under its capability guard.
type fragmentPermittedDataplane struct {
	dataplane.Dataplane
}

func (d fragmentPermittedDataplane) SAStateScopeSupported() bool {
	installer, ok := d.Dataplane.(dataplane.SAStateScopeInstaller)
	return ok && installer.SAStateScopeSupported()
}

func (d fragmentPermittedDataplane) InstallSA(p dataplane.SAParams) error {
	if p.Mode != dataplane.ModeTunnel {
		return fmt.Errorf("fragment fixture needs a tunnel scope with production fragment permission false")
	}
	if p.Scope == nil {
		return fmt.Errorf("fragment fixture needs a tunnel scope with production fragment permission false")
	}
	if p.Scope.NonInitialFragments() {
		return fmt.Errorf("fragment fixture needs a tunnel scope with production fragment permission false")
	}
	scope, err := dataplane.NewSAStateScope(p.Scope.Selectors(), true)
	if err != nil {
		return err
	}
	p.Scope = scope
	return d.Dataplane.InstallSA(p)
}

func fragmentScopePermission(dp dataplane.Dataplane) dataplane.Dataplane {
	return fragmentPermittedDataplane{Dataplane: dp}
}

func fragmentPermittedChild(t *testing.T, tsLocal, tsRemote string) *ChildSA {
	t.Helper()
	child := sadSelChildWithDataplane(t, false, tsLocal, tsRemote, fragmentScopePermission)
	fragmentWantPermittedScope(t, child)
	return child
}

func fragmentPermittedChildren(t *testing.T, selectors ...fragmentPorts) []*ChildSA {
	t.Helper()
	children := fragmentChildrenWithDataplane(t, fragmentScopePermission(dataplane.Get()), selectors...)
	for _, child := range children {
		fragmentWantPermittedScope(t, child)
	}
	return children
}

// fragmentWantPermittedScope reads both installed states after their creator registered
// cleanup. Permission is true, while the complete directional selectors stay unchanged.
func fragmentWantPermittedScope(t *testing.T, child *ChildSA) {
	t.Helper()
	states, err := dataplane.Get().ListSAs(child.IfID)
	if err != nil {
		t.Fatalf("read the fragment fixture states: %v", err)
	}
	for _, direction := range []dataplane.SADir{dataplane.SADirIn, dataplane.SADirOut} {
		want, err := childStateScope(childPolicies(child, direction), child.Mode)
		if err != nil {
			t.Fatalf("build the original directional fixture scope: %v", err)
		}
		if want.NonInitialFragments() {
			t.Fatal("production granted noninitial-fragment permission without negotiation")
		}
		spi, source, target := child.InboundSPI, child.RemoteAddr, child.LocalAddr
		if direction == dataplane.SADirOut {
			spi, source, target = child.OutboundSPI, child.LocalAddr, child.RemoteAddr
		}
		found := 0
		for index := range states {
			state := &states[index]
			if state.SPI != spi {
				continue
			}
			found++
			if !state.Src.Equal(source) {
				t.Fatalf("fragment fixture SPI %#x has a different state identity", spi)
			}
			if !state.Dst.Equal(target) {
				t.Fatalf("fragment fixture SPI %#x has a different state identity", spi)
			}
			if state.IfID != child.IfID {
				t.Fatalf("fragment fixture SPI %#x has a different state identity", spi)
			}
			if state.Proto != dataplane.ProtoESP {
				t.Fatalf("fragment fixture SPI %#x has a different state identity", spi)
			}
			if state.Mode != child.Mode {
				t.Fatalf("fragment fixture SPI %#x has a different state identity", spi)
			}
			if state.ReqID != child.ReqID {
				t.Fatalf("fragment fixture SPI %#x has a different state identity", spi)
			}
			if !state.StateScopeRead {
				t.Fatalf("fragment fixture SPI %#x did not retain explicit fragment permission", spi)
			}
			if state.Scope == nil {
				t.Fatalf("fragment fixture SPI %#x did not retain explicit fragment permission", spi)
			}
			if !state.Scope.NonInitialFragments() {
				t.Fatalf("fragment fixture SPI %#x did not retain explicit fragment permission", spi)
			}
			if !reflect.DeepEqual(state.Scope.Selectors(), want.Selectors()) {
				t.Fatalf("fragment fixture SPI %#x changed its directional selectors: got %+v, want %+v",
					spi, state.Scope.Selectors(), want.Selectors())
			}
		}
		if found != 1 {
			t.Fatalf("fragment fixture SPI %#x appeared %d times, want one installed state", spi, found)
		}
	}
}
