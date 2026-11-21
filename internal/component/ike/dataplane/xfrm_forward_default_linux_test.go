//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- namespace forward default.
// Related: engine/rfc4301_forward_default_integration_linux_test.go -- native kernel observation.

package dataplane

import (
	"errors"
	"slices"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/privilege"
)

var _ ForwardDefaultInstaller = (*xfrmBackend)(nil)
var _ ForwardDefaultInstaller = (*xfrmMobikeBackend)(nil)

type forwardDefaultFixture struct {
	current            xfrmPolicyDefaults
	writes             []xfrmDefaultAction
	readErr, writeErr  error
	loseNextRead       bool
	loseReadAfterWrite bool
	// loseReadAfterRefusedWrite fails the read that follows the next write the
	// fixture refuses with writeErr.
	loseReadAfterRefusedWrite bool
}

func newForwardDefaultFixture(t *testing.T) *forwardDefaultFixture {
	t.Helper()
	fixture := &forwardDefaultFixture{current: xfrmPolicyDefaults{
		in: xfrmDefaultBlock, fwd: xfrmDefaultAccept, out: xfrmDefaultBlock,
	}}
	read, write := xfrmDefaultsRead, xfrmDefaultWrite
	t.Cleanup(func() { xfrmDefaultsRead, xfrmDefaultWrite = read, write })
	xfrmDefaultsRead = func() (xfrmPolicyDefaults, error) {
		if fixture.loseNextRead {
			fixture.loseNextRead = false
			return xfrmPolicyDefaults{}, unix.EIO
		}
		return fixture.current, fixture.readErr
	}
	xfrmDefaultWrite = func(value xfrmDefaultAction) error {
		fixture.writes = append(fixture.writes, value)
		if fixture.writeErr != nil {
			if fixture.loseReadAfterRefusedWrite {
				fixture.loseReadAfterRefusedWrite = false
				fixture.loseNextRead = true
			}
			return fixture.writeErr
		}
		fixture.current.fwd = value
		if fixture.loseReadAfterWrite {
			fixture.loseReadAfterWrite = false
			fixture.loseNextRead = true
		}
		return nil
	}
	return fixture
}

// TestXFRMForwardDefaultKeepsOriginalAcrossReloadAndRollback checks a reload that
// changes the forward default and is then rolled back.
//
// VALIDATES: the reload's bypass writes accept over the running block, the undo puts
// the running block back, and the release restores the namespace's original value.
// PREVENTS: an undo that "restores" a reload that never wrote, which would leave the
// rollback assertions green with the reload's own change missing.
// METHOD: apply discard, then bypass, reading the fixture's value and write log after
// each step, then undo and release.
func TestXFRMForwardDefaultKeepsOriginalAcrossReloadAndRollback(t *testing.T) {
	fixture := newForwardDefaultFixture(t)
	original := fixture.current
	backend := &xfrmBackend{}
	if _, err := backend.ApplyForwardDefault(SPActionDiscard); err != nil {
		t.Fatal(err)
	}
	undo, err := backend.ApplyForwardDefault(SPActionBypass)
	if err != nil {
		t.Fatal(err)
	}
	wantWrites := []xfrmDefaultAction{xfrmDefaultBlock, xfrmDefaultAccept}
	if fixture.current.fwd != xfrmDefaultAccept || !slices.Equal(fixture.writes, wantWrites) {
		t.Fatalf("reload to bypass: fwd=%d writes=%v, want fwd=%d writes=%v",
			fixture.current.fwd, fixture.writes, xfrmDefaultAccept, wantWrites)
	}
	if err := undo(); err != nil {
		t.Fatal(err)
	}
	if fixture.current.fwd != xfrmDefaultBlock || backend.forwardDefault.original != original.fwd {
		t.Fatalf("rollback lost running or original disposition: current=%+v lease=%+v", fixture.current, backend.forwardDefault)
	}
	if err := backend.ReleaseForwardDefault(); err != nil {
		t.Fatal(err)
	}
	if fixture.current != original || backend.forwardDefault != nil {
		t.Fatalf("release: defaults=%+v lease=%+v, want %+v and no lease", fixture.current, backend.forwardDefault, original)
	}
}

func TestXFRMForwardDefaultRefusesAnObservedForeignChange(t *testing.T) {
	fixture := newForwardDefaultFixture(t)
	backend := &xfrmBackend{}
	if _, err := backend.ApplyForwardDefault(SPActionDiscard); err != nil {
		t.Fatal(err)
	}
	fixture.current.fwd = xfrmDefaultAccept
	writes := len(fixture.writes)
	if _, err := backend.ApplyForwardDefault(SPActionDiscard); err == nil {
		t.Error("reload overwrote an unrelated policy manager's changed default")
	}
	if err := backend.ReleaseForwardDefault(); err == nil {
		t.Error("release overwrote an unrelated policy manager's changed default")
	}
	if len(fixture.writes) != writes || fixture.current.fwd != xfrmDefaultAccept || backend.forwardDefault == nil {
		t.Fatalf("foreign change was overwritten or claim forgotten: writes=%v current=%+v lease=%+v", fixture.writes, fixture.current, backend.forwardDefault)
	}
	fixture.current.fwd = xfrmDefaultBlock
	if err := backend.ReleaseForwardDefault(); err != nil {
		t.Fatalf("release after the owner reconciled its change: %v", err)
	}
}

func TestXFRMForwardDefaultRefusalRestoresTheUnclaimedNamespace(t *testing.T) {
	fixture := newForwardDefaultFixture(t)
	original := fixture.current
	fixture.writeErr = unix.EPERM
	backend := &xfrmBackend{}
	if _, err := backend.ApplyForwardDefault(SPActionDiscard); !errors.Is(err, unix.EPERM) {
		t.Fatalf("apply error=%v, want original EPERM", err)
	}
	if fixture.current != original || backend.forwardDefault != nil {
		t.Fatalf("refusal changed namespace or retained a restored claim: defaults=%+v lease=%+v", fixture.current, backend.forwardDefault)
	}
}

func TestXFRMForwardDefaultLostReadbackRollsBackTheLandedWrite(t *testing.T) {
	fixture := newForwardDefaultFixture(t)
	original := fixture.current
	fixture.loseReadAfterWrite = true
	backend := &xfrmBackend{}
	if _, err := backend.ApplyForwardDefault(SPActionDiscard); !errors.Is(err, unix.EIO) {
		t.Fatalf("apply error=%v, want readback EIO", err)
	}
	if fixture.current != original || backend.forwardDefault != nil || len(fixture.writes) != 2 {
		t.Fatalf("lost readback did not restore original: defaults=%+v lease=%+v writes=%v", fixture.current, backend.forwardDefault, fixture.writes)
	}
}

// TestXFRMForwardDefaultRefusedWriteWithLostReadbackKeepsNoStaleClaim checks the
// apply whose write the kernel refuses and whose readback then fails.
//
// VALIDATES: the refused write changed nothing, so the restore finds the value it
// expects, releases the claim, and a later apply installs its disposition.
// PREVENTS: the lease keeping the attempted value while the kernel still holds the
// earlier one, which made the restore report a foreign change, kept the lease, and
// refused every later apply until restart.
func TestXFRMForwardDefaultRefusedWriteWithLostReadbackKeepsNoStaleClaim(t *testing.T) {
	fixture := newForwardDefaultFixture(t)
	original := fixture.current
	fixture.writeErr = unix.EIO
	fixture.loseReadAfterRefusedWrite = true
	backend := &xfrmBackend{}
	_, err := backend.ApplyForwardDefault(SPActionDiscard)
	if !errors.Is(err, unix.EIO) {
		t.Fatalf("apply error=%v, want the refused write's EIO", err)
	}
	if errors.Is(err, errXFRMForwardDefaultChanged) {
		t.Fatalf("apply error=%v reports a foreign change; only Ze's own write failed", err)
	}
	if fixture.current != original || backend.forwardDefault != nil {
		t.Fatalf("refused write left a claim or a change: defaults=%+v lease=%+v", fixture.current, backend.forwardDefault)
	}
	fixture.writeErr = nil
	if _, err := backend.ApplyForwardDefault(SPActionDiscard); err != nil {
		t.Fatalf("later apply refused after a refused write: %v", err)
	}
	if fixture.current.fwd != xfrmDefaultBlock {
		t.Fatalf("later apply did not install block: defaults=%+v", fixture.current)
	}
}

// TestXFRMForwardDefaultMissingKernelOperationNamesTheKernelBuild checks the refusal
// for a kernel without the XFRM default-policy operation.
//
// VALIDATES: its fix is the runtime kernel build, the step the text sends the
// operator to, for each errno that means the operation is absent.
// PREVENTS: a reboot offered as the fix before any kernel that has the operation
// was built or installed.
func TestXFRMForwardDefaultMissingKernelOperationNamesTheKernelBuild(t *testing.T) {
	for _, errno := range []unix.Errno{unix.EINVAL, unix.EPROTONOSUPPORT, unix.ENOPROTOOPT, unix.EOPNOTSUPP} {
		fixture := newForwardDefaultFixture(t)
		fixture.readErr = errno
		_, err := (&xfrmBackend{}).ApplyForwardDefault(SPActionDiscard)
		refusal, ok := errors.AsType[*opererr.Error](err)
		if !ok || !errors.Is(err, errno) {
			t.Fatalf("%v: refusal=%v, want an operator error preserving the errno", errno, err)
		}
		if refusal.Fix() != RuntimeKernelBuildFix() {
			t.Errorf("%v: fix=%q, want the runtime kernel build %q", errno, refusal.Fix(), RuntimeKernelBuildFix())
		}
	}
}

func TestXFRMForwardDefaultUnreadableStateIsNeverOverwritten(t *testing.T) {
	fixture := newForwardDefaultFixture(t)
	backend := &xfrmBackend{}
	if _, err := backend.ApplyForwardDefault(SPActionDiscard); err != nil {
		t.Fatal(err)
	}
	writes := len(fixture.writes)
	fixture.readErr = unix.EIO
	if err := backend.ReleaseForwardDefault(); !errors.Is(err, unix.EIO) {
		t.Fatalf("release error=%v, want unreadable namespace EIO", err)
	}
	if backend.forwardDefault == nil || len(fixture.writes) != writes {
		t.Fatalf("unreadable release forgot the claim or wrote: lease=%+v writes=%v", backend.forwardDefault, fixture.writes)
	}
	fixture.readErr = nil
	if err := backend.ReleaseForwardDefault(); err != nil {
		t.Fatal(err)
	}
}

func TestXFRMForwardDefaultUnreadableInitialStateAcquiresNoLease(t *testing.T) {
	fixture := newForwardDefaultFixture(t)
	original := fixture.current
	fixture.readErr = unix.EPERM
	backend := &xfrmBackend{}
	if _, err := backend.ApplyForwardDefault(SPActionDiscard); !errors.Is(err, unix.EPERM) {
		t.Fatalf("initial apply error=%v, want EPERM", err)
	}
	if backend.forwardDefault != nil || len(fixture.writes) != 0 || fixture.current != original {
		t.Fatalf("unreadable namespace was claimed or changed: lease=%+v writes=%v defaults=%+v", backend.forwardDefault, fixture.writes, fixture.current)
	}
}

func TestXFRMForwardDefaultFailedRestorationKeepsItsLeaseForRetry(t *testing.T) {
	fixture := newForwardDefaultFixture(t)
	backend := &xfrmBackend{}
	if _, err := backend.ApplyForwardDefault(SPActionDiscard); err != nil {
		t.Fatal(err)
	}
	fixture.writeErr = unix.EIO
	if err := backend.ReleaseForwardDefault(); !errors.Is(err, unix.EIO) {
		t.Fatalf("release error=%v, want original EIO", err)
	}
	if backend.forwardDefault == nil || fixture.current.fwd != xfrmDefaultBlock {
		t.Fatalf("failed restoration lost protection: defaults=%+v lease=%+v", fixture.current, backend.forwardDefault)
	}
	if err := backend.CheckForwardDefault(); !errors.Is(err, unix.EIO) {
		t.Fatalf("live check lost the incomplete restoration after its read recovered: %v", err)
	}
	fixture.writeErr = nil
	if err := backend.ReleaseForwardDefault(); err != nil {
		t.Fatal(err)
	}
	if backend.forwardDefault != nil || fixture.current.fwd != xfrmDefaultAccept {
		t.Fatal("retry did not restore the original namespace and release its claim")
	}
}

func TestXFRMForwardDefaultHostRefusalHasAnOperatorFix(t *testing.T) {
	fixture := newForwardDefaultFixture(t)
	fixture.readErr = unix.EPERM
	backend := &xfrmBackend{}
	_, err := backend.ApplyForwardDefault(SPActionDiscard)
	refusal, ok := errors.AsType[*opererr.Error](err)
	if !ok || !errors.Is(err, unix.EPERM) {
		t.Fatalf("default-policy host refusal=%v, want an operator error preserving EPERM", err)
	}
	if refusal.Problem() == "" || refusal.Why() == "" || refusal.Fix() != privilege.DaemonCapabilitiesFix {
		t.Fatalf("default-policy host refusal has no exact capability repair: %+v", refusal)
	}
}

func TestXFRMForwardDefaultForeignChangeHasAnOperatorFix(t *testing.T) {
	fixture := newForwardDefaultFixture(t)
	backend := &xfrmBackend{}
	if _, err := backend.ApplyForwardDefault(SPActionDiscard); err != nil {
		t.Fatal(err)
	}
	fixture.current.fwd = xfrmDefaultAccept
	err := backend.ReleaseForwardDefault()
	refusal, ok := errors.AsType[*opererr.Error](err)
	if !ok {
		t.Fatalf("foreign-change refusal=%v, want an operator error", err)
	}
	if refusal.Problem() == "" || refusal.Why() == "" || refusal.Fix() != "ip xfrm policy getdefault" {
		t.Fatalf("foreign-change refusal has no exact inspection step: %+v", refusal)
	}
}

func TestXFRMForwardDefaultLiveCheckReadsWithoutChangingItsClaim(t *testing.T) {
	fixture := newForwardDefaultFixture(t)
	backend := &xfrmBackend{}
	if err := backend.CheckForwardDefault(); err != nil {
		t.Fatal(err)
	}
	if len(fixture.writes) != 0 || backend.forwardDefault != nil {
		t.Fatal("the unclaimed host check changed the namespace")
	}
	if _, err := backend.ApplyForwardDefault(SPActionDiscard); err != nil {
		t.Fatal(err)
	}
	writes := len(fixture.writes)
	if err := backend.CheckForwardDefault(); err != nil {
		t.Fatal(err)
	}
	fixture.current.fwd = xfrmDefaultAccept
	err := backend.CheckForwardDefault()
	if _, ok := errors.AsType[*opererr.Error](err); !ok {
		t.Fatalf("foreign value check=%v, want operator refusal", err)
	}
	if len(fixture.writes) != writes || backend.forwardDefault == nil || fixture.current.fwd != xfrmDefaultAccept {
		t.Fatal("a live foreign-value check changed or forgot the claim")
	}
	fixture.readErr = unix.EIO
	if err := backend.CheckForwardDefault(); !errors.Is(err, unix.EIO) {
		t.Fatalf("unreadable live check=%v, want EIO", err)
	}
	if len(fixture.writes) != writes || backend.forwardDefault == nil {
		t.Fatal("an unreadable live check changed the claim")
	}
	fixture.readErr = nil
	fixture.current.fwd = xfrmDefaultBlock
	if err := backend.ReleaseForwardDefault(); err != nil {
		t.Fatal(err)
	}
	if err := backend.CheckForwardDefault(); err != nil {
		t.Fatal(err)
	}
	if backend.forwardDefault != nil {
		t.Fatal("the live check reacquired a released claim")
	}
}
