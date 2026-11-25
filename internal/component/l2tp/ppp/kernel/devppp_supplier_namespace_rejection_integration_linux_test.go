// Design: docs/architecture/l2tp/bng-5-pppoe.md -- namespace failure revokes actual creation rejection
// Related: devppp_supplier_namespace_linux_test.go -- non-authority namespace models

//go:build integration && linux

package kernel

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink"
	"github.com/ze-software/ze/internal/test/userns"
)

// Actual creation and a real exclusive-name NACK pass through actual setup's
// original-channel namespace runner. Clean completion MUST retain rejection;
// otherwise an implementation which always revokes could pass both negatives.
func TestOriginalPPPSupplierNamespaceNativeCleanCollisionRetainsRejection(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	creator, original := supplierNamespaceNativeCreator(t)
	socket, _, _, _ := supplierCaptureNativeFiles(t)
	channel, err := CreatePPPChannel(socket)
	if channel != nil {
		t.Cleanup(func() {
			if err := channel.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	unit, cause := createPPPSetupUnit(creator, channel, original.Identity().InterfaceName)
	supplierNamespaceNativeRejected(t, unit, cause)
	if err := unit.Close(); err != nil {
		t.Fatal(err)
	}
	if !unit.CreationRejected() {
		t.Fatal("clean namespace completion or original-file Close erased native rejection")
	}
	supplierNamespaceNativeSurvivor(t, original)
}

// The creator and negative ACK are real. Only namespace switching is modeled:
// the real rejected owner MUST lose retry authority after the modeled restore
// fails, preserving the native cause and the original partial pointer.
func TestOriginalPPPSupplierNamespaceNativeRestoreFailureRevokesRejection(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	creator, original := supplierNamespaceNativeCreator(t)
	fixture := newSupplierNativeNamespaceModel(t)
	restoration := errors.New("modeled original namespace restoration refused")
	enter := fixture.ops.enter
	fixture.ops.enter = func(file *os.File) error {
		if file == fixture.caller.file {
			fixture.trace = append(fixture.trace, "restore")
			return restoration
		}
		return enter(file)
	}
	var rejected *CreatedUnit
	var creation error
	observed := &UnitCreator{create: func(name string) (*CreatedUnit, error) {
		fixture.trace = append(fixture.trace, "create")
		rejected, creation = creator.Create(name)
		supplierNamespaceNativeRejected(t, rejected, creation)
		return rejected, creation
	}}
	unit, err := createPPPSetupUnitThreadWith(observed, fixture.target, original.Identity().InterfaceName, fixture.ops)
	if unit != rejected {
		t.Fatal("failed restoration discarded the actual rejected unit owner")
	}
	if !errors.Is(err, creation) {
		t.Fatalf("lost original native creation cause: %v", err)
	}
	if !errors.Is(err, restoration) {
		t.Fatalf("lost original restoration cause: %v", err)
	}
	if unit.CreationRejected() {
		t.Fatal("failed restoration retained actual native rejection retry authority")
	}
	if unit.ready {
		t.Fatal("failed restoration retained readiness")
	}
	if want := []string{"lock", "target", "create", "restore"}; !reflect.DeepEqual(fixture.trace, want) {
		t.Fatalf("failed restoration unlocked its worker: %v", fixture.trace)
	}
	supplierNamespaceNativeSurvivor(t, original)
}

// Closing the model's caller file after the real NACK makes its later Close
// fail with the actual os.ErrClosed cause. Successful modeled restoration does
// not authorize retry when namespace-handle cleanup itself failed.
func TestOriginalPPPSupplierNamespaceNativeCloseFailureRevokesRejection(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	creator, original := supplierNamespaceNativeCreator(t)
	fixture := newSupplierNativeNamespaceModel(t)
	var rejected *CreatedUnit
	var creation error
	observed := &UnitCreator{create: func(name string) (*CreatedUnit, error) {
		fixture.trace = append(fixture.trace, "create")
		rejected, creation = creator.Create(name)
		supplierNamespaceNativeRejected(t, rejected, creation)
		if err := fixture.caller.file.Close(); err != nil {
			t.Fatal(err)
		}
		return rejected, creation
	}}
	unit, err := createPPPSetupUnitThreadWith(observed, fixture.target, original.Identity().InterfaceName, fixture.ops)
	if unit != rejected {
		t.Fatal("failed namespace Close discarded the actual rejected owner")
	}
	if !errors.Is(err, creation) {
		t.Fatalf("lost original native creation cause: %v", err)
	}
	if !errors.Is(err, os.ErrClosed) {
		t.Fatalf("lost original namespace-file close cause: %v", err)
	}
	if unit.CreationRejected() {
		t.Fatal("failed namespace Close retained actual native rejection retry authority")
	}
	if unit.ready {
		t.Fatal("failed namespace Close retained readiness")
	}
	if want := []string{"lock", "target", "create", "restore", "unlock"}; !reflect.DeepEqual(fixture.trace, want) {
		t.Fatalf("namespace Close failure changed restoration order: %v", fixture.trace)
	}
	supplierNamespaceNativeSurvivor(t, original)
}

func supplierNamespaceNativeCreator(t *testing.T) (*UnitCreator, *CreatedUnit) {
	t.Helper()
	backend, err := iface.OpenBackend("kernel")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	})
	creator, err := NewUnitCreator(backend)
	if err != nil {
		t.Fatal(err)
	}
	original, err := creator.Create("zc-ns-original")
	if original != nil {
		t.Cleanup(func() {
			if err := original.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	if original == nil {
		t.Fatal("actual creator returned no original native unit")
	}
	if !original.ready {
		t.Fatal("actual successful native creation has no readiness")
	}
	if original.CreationRejected() {
		t.Fatal("actual successful native creation became rejection")
	}
	supplierNamespaceNativeSurvivor(t, original)
	return creator, original
}

// This checks the actual producer's receipt before the tested namespace finish.
// It never implements or assigns a NativeLinkCreationRejected marker.
func supplierNamespaceNativeRejected(t *testing.T, unit *CreatedUnit, cause error) {
	t.Helper()
	if unit != nil {
		t.Cleanup(func() {
			if err := unit.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if !errors.Is(cause, unix.EEXIST) {
		t.Fatalf("actual exclusive-name collision=%v, want native EEXIST", cause)
	}
	if unit == nil {
		t.Fatal("actual native rejection lost its original partial file owner")
	}
	if !unit.Attempted() {
		t.Fatal("actual native rejection erased its submitted operation")
	}
	if !unit.CreationRejected() {
		t.Fatalf("original native NACK has no retry authority before namespace failure: %v", cause)
	}
}

func supplierNamespaceNativeSurvivor(t *testing.T, original *CreatedUnit) {
	t.Helper()
	if err := original.WithDescriptor(func(fd int) error {
		number, err := ioctlGetInt(fd, pppiocGUnit)
		if err != nil {
			return err
		}
		if number != original.UnitNumber() {
			return fmt.Errorf("original native unit changed from %d to %d", original.UnitNumber(), number)
		}
		return nil
	}); err != nil {
		t.Fatalf("rejected attempt damaged original native file: %v", err)
	}
	identity := original.Identity()
	link, err := netlink.LinkByIndex(int(identity.NativeIndex))
	if err != nil {
		t.Fatal(err)
	}
	if link.Type() != "ppp" {
		t.Fatalf("surviving original device has type %q", link.Type())
	}
	if link.Attrs().Name != identity.InterfaceName {
		t.Fatal("rejected attempt renamed the surviving original device")
	}
	if _, err := original.Counters(); err != nil {
		t.Fatalf("rejected attempt retired the surviving original reader: %v", err)
	}
}

// These regular files prove only private identity/cleanup sequencing. Enter
// never calls setns, and no model file is installed on a CreatedUnit/channel.
// Caller Close is exercised by the production thread runner; cleanup accepts
// its deliberate double-close and still reports every other file error.
type supplierNativeNamespaceModel struct {
	caller *capturedSessionNamespace
	target *capturedSessionNamespace
	ops    pppSetupNamespaceOps
	trace  []string
}

func newSupplierNativeNamespaceModel(t *testing.T) *supplierNativeNamespaceModel {
	t.Helper()
	caller, err := os.CreateTemp(t.TempDir(), "native-rejection-caller-model")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := caller.Close(); err != nil {
			if !errors.Is(err, os.ErrClosed) {
				t.Error(err)
			}
		}
	})
	target, err := os.CreateTemp(t.TempDir(), "native-rejection-target-model")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := target.Close(); err != nil {
			t.Error(err)
		}
	})
	fixture := &supplierNativeNamespaceModel{
		caller: &capturedSessionNamespace{file: caller},
		target: &capturedSessionNamespace{file: target},
	}
	fixture.ops = pppSetupNamespaceOps{
		lock:    func() { fixture.trace = append(fixture.trace, "lock") },
		unlock:  func() { fixture.trace = append(fixture.trace, "unlock") },
		current: func() (*capturedSessionNamespace, error) { return fixture.caller, nil },
		enter: func(file *os.File) error {
			if file == target {
				fixture.trace = append(fixture.trace, "target")
				return nil
			}
			if file == caller {
				fixture.trace = append(fixture.trace, "restore")
				return nil
			}
			return errors.New("namespace model received another file")
		},
	}
	return fixture
}
