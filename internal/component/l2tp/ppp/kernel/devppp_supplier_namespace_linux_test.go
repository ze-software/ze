// Design: docs/architecture/l2tp/bng-5-pppoe.md -- setup namespace restoration

//go:build linux

package kernel

import (
	"errors"
	"os"
	"reflect"
	"testing"
)

// Regular files below model identity and cleanup only. They never reach setns,
// capture, SessionConfig or a public channel/unit authority constructor.
type supplierNamespaceFixture struct {
	original *capturedSessionNamespace
	target   *capturedSessionNamespace
	unit     *CreatedUnit
	creator  *UnitCreator
	ops      pppSetupNamespaceOps
	trace    []string
}

func newSupplierNamespaceFixture(t *testing.T) *supplierNamespaceFixture {
	t.Helper()
	original, err := os.CreateTemp(t.TempDir(), "caller-identity")
	if err != nil {
		t.Fatal(err)
	}
	target, err := os.CreateTemp(t.TempDir(), "transport-identity")
	if err != nil {
		if closeErr := original.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal(err)
	}
	fixture := &supplierNamespaceFixture{
		original: &capturedSessionNamespace{file: original},
		target:   &capturedSessionNamespace{file: target}, unit: &CreatedUnit{},
	}
	t.Cleanup(func() {
		if err := errors.Join(fixture.original.Close(), fixture.target.Close()); err != nil {
			t.Error(err)
		}
	})
	fixture.creator = &UnitCreator{create: func(string) (*CreatedUnit, error) {
		fixture.trace = append(fixture.trace, "create")
		return fixture.unit, nil
	}}
	fixture.ops = pppSetupNamespaceOps{
		lock:    func() { fixture.trace = append(fixture.trace, "lock") },
		unlock:  func() { fixture.trace = append(fixture.trace, "unlock") },
		current: func() (*capturedSessionNamespace, error) { return fixture.original, nil },
		enter: func(file *os.File) error {
			if file == target {
				fixture.trace = append(fixture.trace, "target")
				return nil
			}
			if file == original {
				fixture.trace = append(fixture.trace, "restore")
				return nil
			}
			return errors.New("namespace worker entered another file")
		},
	}
	return fixture
}

// Successful creation restores before unlock and preserves the exact original
// partial result pointer. The logical result still grants no native authority.
func TestOriginalPPPSupplierNamespaceRestoresBeforeReturn(t *testing.T) {
	fixture := newSupplierNamespaceFixture(t)
	unit, err := createPPPSetupUnitThreadWith(fixture.creator, fixture.target, "original", fixture.ops)
	if err != nil {
		t.Fatal(err)
	}
	if unit != fixture.unit {
		t.Fatal("worker replaced its original result")
	}
	if unit.ready {
		t.Fatal("namespace model minted a public native grant")
	}
	if want := []string{"lock", "target", "create", "restore", "unlock"}; !reflect.DeepEqual(fixture.trace, want) {
		t.Fatalf("trace=%v want%v", fixture.trace, want)
	}
	if _, err := fixture.original.file.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("worker retained caller namespace: %v", err)
	}
}

// An uncertain original creation and failed restoration both survive. No unlock
// occurs, so the dedicated production worker's exiting goroutine retires its
// wrong-namespace OS thread rather than returning it to unrelated work.
func TestOriginalPPPSupplierNamespaceRestoreFailureRetainsPartialCause(t *testing.T) {
	fixture := newSupplierNamespaceFixture(t)
	// Private readiness state witness only: no self/file/backend/native receipt
	// is minted. The separate native rejection roots supply genuine retry
	// authority before the same shared finish producer revokes it.
	fixture.unit.ready = true
	creation := errors.New("original creation outcome uncertain")
	restoration := errors.New("original caller restoration refused")
	fixture.creator.create = func(string) (*CreatedUnit, error) {
		fixture.trace = append(fixture.trace, "create")
		return fixture.unit, creation
	}
	enter := fixture.ops.enter
	fixture.ops.enter = func(file *os.File) error {
		if file == fixture.original.file {
			fixture.trace = append(fixture.trace, "restore")
			return restoration
		}
		return enter(file)
	}
	unit, err := createPPPSetupUnitThreadWith(fixture.creator, fixture.target, "original", fixture.ops)
	if unit != fixture.unit {
		t.Fatal("restore failure discarded partial unit owner")
	}
	if !errors.Is(err, creation) {
		t.Fatalf("lost original creation cause: %v", err)
	}
	if !errors.Is(err, restoration) {
		t.Fatalf("lost restoration cause: %v", err)
	}
	if unit.ready {
		t.Fatal("restore failure retained the private readiness witness")
	}
	if unit.CreationRejected() {
		t.Fatal("restore failure retained a rejection receipt")
	}
	if want := []string{"lock", "target", "create", "restore"}; !reflect.DeepEqual(fixture.trace, want) {
		t.Fatalf("failed restore returned thread: %v", fixture.trace)
	}
}

// A provider panic cannot skip restoration or erase the original error value.
func TestOriginalPPPSupplierNamespacePanicStillRestores(t *testing.T) {
	fixture := newSupplierNamespaceFixture(t)
	cause := errors.New("original provider panic")
	fixture.creator.create = func(string) (*CreatedUnit, error) { fixture.trace = append(fixture.trace, "create"); panic(cause) }
	unit, err := createPPPSetupUnitThreadWith(fixture.creator, fixture.target, "original", fixture.ops)
	if unit != nil {
		t.Fatal("panicked provider invented a returned unit")
	}
	if !errors.Is(err, cause) {
		t.Fatalf("lost provider panic: %v", err)
	}
	if want := []string{"lock", "target", "create", "restore", "unlock"}; !reflect.DeepEqual(fixture.trace, want) {
		t.Fatalf("panic trace=%v", fixture.trace)
	}
}

// If the source namespace identity cannot be inspected, creation never runs.
func TestOriginalPPPSupplierNamespaceUnknownRefusesBeforeCreate(t *testing.T) {
	fixture := newSupplierNamespaceFixture(t)
	if err := fixture.target.Close(); err != nil {
		t.Fatal(err)
	}
	unit, err := createPPPSetupUnitThreadWith(fixture.creator, fixture.target, "original", fixture.ops)
	if !errors.Is(err, os.ErrClosed) {
		t.Fatalf("unknown namespace cause=%v", err)
	}
	if unit != nil {
		t.Fatal("unknown namespace created a unit")
	}
	if want := []string{"lock", "unlock"}; !reflect.DeepEqual(fixture.trace, want) {
		t.Fatalf("unknown namespace performed work: %v", fixture.trace)
	}
}
