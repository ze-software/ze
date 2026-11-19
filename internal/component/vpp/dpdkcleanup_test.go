// Design: docs/research/vpp-deployment-reference.md -- original NIC restoration receipts
// Related: dpdk.go -- actual binding, rollback and cleanup through captured operations

package vpp

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"testing"
)

// TestDPDKCleanupHealthyRoundTrip proves useful binding and exact restoration,
// including the original per-device override and avoiding repeated cleanup.
func TestDPDKCleanupHealthyRoundTrip(t *testing.T) {
	fixture := newDPDKCleanupFixture(t)
	fixture.bind(t)
	if err := fixture.binder.unbindAllWithOps(fixture.ops); err != nil {
		t.Fatal(err)
	}
	fixture.restored(t)
	if err := fixture.binder.unbindAllWithOps(fixture.ops); err != nil {
		t.Fatal(err)
	}
	if fixture.unbindCalls != 1 {
		t.Fatalf("confirmed unbind repeated: %d", fixture.unbindCalls)
	}
	if fixture.rebindCalls != 1 {
		t.Fatalf("confirmed original-driver bind repeated: %d", fixture.rebindCalls)
	}
	if fixture.overrideWrites[dpdkCleanupOverride] != 1 {
		t.Fatalf("confirmed original override restoration repeated: %d", fixture.overrideWrites[dpdkCleanupOverride])
	}
}

// TestDPDKCleanupRetainsRebindFailure requires an original-driver failure to
// remain both an error cause and an owned retry receipt after VFIO detachment.
func TestDPDKCleanupRetainsRebindFailure(t *testing.T) {
	fixture := newDPDKCleanupFixture(t)
	fixture.bind(t)
	want := errors.New("original driver rebind refused")
	fixture.rebindFailure = want
	err := fixture.binder.unbindAllWithOps(fixture.ops)
	if !errors.Is(err, want) {
		t.Fatalf("original-driver failure was discarded: %v", err)
	}
	if fixture.driver != "" {
		t.Fatalf("failed restore changed native driver to %q", fixture.driver)
	}
	original, ok := fixture.binder.savedDrivers[dpdkCleanupPCI]
	if !ok {
		t.Fatal("failed original-driver restoration lost its receipt")
	}
	if original.driver != "ixgbe" || original.override != dpdkCleanupOverride {
		t.Fatal("failed original-driver restoration lost its receipt")
	}
}

// TestDPDKCleanupRetainsOverrideFailure keeps the exact original override until
// retry confirms its restoration; useful driver restoration cannot erase it.
func TestDPDKCleanupRetainsOverrideFailure(t *testing.T) {
	fixture := newDPDKCleanupFixture(t)
	fixture.bind(t)
	want := errors.New("original driver override restoration refused")
	fixture.overrideFailure = want
	err := fixture.binder.unbindAllWithOps(fixture.ops)
	if !errors.Is(err, want) {
		t.Fatalf("original override restoration failure was discarded: %v", err)
	}
	if fixture.driver != "ixgbe" {
		t.Fatalf("independent useful driver restore did not survive: %q", fixture.driver)
	}
	if fixture.override != "ixgbe" {
		t.Fatalf("refused override restoration was reported as applied: %q", fixture.override)
	}
	original, ok := fixture.binder.savedDrivers[dpdkCleanupPCI]
	if !ok {
		t.Fatal("failed override restoration lost its original receipt")
	}
	if original.driver != "ixgbe" || original.override != dpdkCleanupOverride {
		t.Fatal("failed override restoration lost its original receipt")
	}
	fixture.overrideFailure = nil
	if err := fixture.binder.unbindAllWithOps(fixture.ops); err != nil {
		t.Fatal(err)
	}
	fixture.restored(t)
	if fixture.unbindCalls != 1 || fixture.rebindCalls != 1 {
		t.Fatal("override-only retry repeated confirmed device detachment or original bind")
	}
	if fixture.overrideWrites["ixgbe"] != 1 || fixture.overrideWrites[dpdkCleanupOverride] != 2 {
		t.Fatal("override-only retry did not retry exactly the failed override write")
	}
}

// TestDPDKCleanupBindFailureKeepsRollbackCause refuses a later input after a
// useful first bind, then requires the original failed rollback to stay visible.
func TestDPDKCleanupBindFailureKeepsRollbackCause(t *testing.T) {
	fixture := newDPDKCleanupFixture(t)
	want := errors.New("original driver rollback refused")
	fixture.rebindFailure = want
	primary := validatePCIAddress("invalid-address")
	if primary == nil {
		t.Fatal("malformed later PCI input did not produce its validation failure")
	}
	err := fixture.binder.bindAllWithOps([]DPDKInterface{
		{PCIAddress: dpdkCleanupPCI, Name: "dpdk0"},
		{PCIAddress: "invalid-address", Name: "dpdk1"},
	}, fixture.ops)
	if fixture.bindCalls != 1 {
		t.Fatalf("useful first bind calls=%d, want 1", fixture.bindCalls)
	}
	if err == nil {
		t.Fatal("later validation failure and failed rollback both disappeared")
	}
	if !strings.Contains(err.Error(), primary.Error()) {
		t.Fatalf("rollback concealed the primary validation failure: %v", err)
	}
	if !errors.Is(err, want) {
		t.Fatalf("primary validation error concealed failed rollback: %v", err)
	}
	original, ok := fixture.binder.savedDrivers[dpdkCleanupPCI]
	if !ok {
		t.Fatal("failed rollback discarded original driver custody")
	}
	if original.driver != "ixgbe" || original.override != dpdkCleanupOverride {
		t.Fatal("failed rollback discarded original driver custody")
	}
}

// TestDPDKCleanupRetriesOnlyUnconfirmedRestore requires retry to continue at
// the failed rebind, preserving the completed VFIO detach and override selection.
func TestDPDKCleanupRetriesOnlyUnconfirmedRestore(t *testing.T) {
	fixture := newDPDKCleanupFixture(t)
	fixture.bind(t)
	want := errors.New("original driver temporarily unavailable")
	fixture.rebindFailure = want
	if err := fixture.binder.unbindAllWithOps(fixture.ops); !errors.Is(err, want) {
		t.Fatalf("first restore did not retain its exact failure: %v", err)
	}
	fixture.rebindFailure = nil
	if err := fixture.binder.unbindAllWithOps(fixture.ops); err != nil {
		t.Fatalf("retry did not resume original restoration: %v", err)
	}
	fixture.restored(t)
	if fixture.unbindCalls != 1 {
		t.Fatalf("retry repeated confirmed VFIO unbind: %d", fixture.unbindCalls)
	}
	if fixture.rebindCalls != 2 {
		t.Fatalf("original rebind attempts=%d, want 2", fixture.rebindCalls)
	}
	if fixture.overrideWrites["ixgbe"] != 1 || fixture.overrideWrites[dpdkCleanupOverride] != 1 {
		t.Fatal("rebind retry repeated confirmed driver override selection")
	}
}

const dpdkCleanupPCI = "0000:01:00.0"
const dpdkCleanupPeerPCI = "0000:02:00.0"
const dpdkCleanupOverride = "original-policy"

// Per-device selection MUST NOT attach or rewrite an unconfigured matching NIC.
func TestDPDKCleanupDoesNotAttachUnconfiguredDevice(t *testing.T) {
	fixture := newDPDKCleanupFixture(t)
	if err := fixture.binder.bindAllWithOps([]DPDKInterface{{PCIAddress: dpdkCleanupPCI}}, fixture.ops); err != nil {
		t.Fatal(err)
	}
	if fixture.driver != "vfio-pci" || fixture.override != "vfio-pci" {
		t.Fatal("precondition: configured device was not usefully bound")
	}
	fixture.peerUnchanged(t)
	if err := fixture.binder.unbindAllWithOps(fixture.ops); err != nil {
		t.Fatal(err)
	}
	fixture.restored(t)
}

// Already-bound devices and their original override belong to their owner.
func TestDPDKCleanupPreservesBorrowedVFIO(t *testing.T) {
	fixture := newDPDKCleanupFixture(t)
	fixture.driver = "vfio-pci"
	if err := fixture.binder.bindAllWithOps([]DPDKInterface{{PCIAddress: dpdkCleanupPCI}}, fixture.ops); err != nil {
		t.Fatal(err)
	}
	if err := fixture.binder.unbindAllWithOps(fixture.ops); err != nil {
		t.Fatal(err)
	}
	if fixture.driver != "vfio-pci" || fixture.override != dpdkCleanupOverride {
		t.Fatal("cleanup changed borrowed VFIO state")
	}
	if fixture.detachCalls != 0 || fixture.bindCalls != 0 || fixture.unbindCalls != 0 || fixture.rebindCalls != 0 || len(fixture.overrideWrites) != 0 {
		t.Fatal("cleanup issued native writes for borrowed VFIO state")
	}
	if len(fixture.binder.savedDrivers) != 0 {
		t.Fatal("borrowed state acquired cleanup custody")
	}
	fixture.peerUnchanged(t)
}

// Initially unbound NICs must return to their original driverless state.
func TestDPDKCleanupRestoresDriverlessDevice(t *testing.T) {
	fixture := newDPDKCleanupFixture(t)
	fixture.driver = ""
	if err := fixture.binder.bindAllWithOps([]DPDKInterface{{PCIAddress: dpdkCleanupPCI}}, fixture.ops); err != nil {
		t.Fatal(err)
	}
	if fixture.driver != "vfio-pci" || fixture.override != "vfio-pci" {
		t.Fatal("precondition: driverless NIC was not usefully bound")
	}
	if err := fixture.binder.unbindAllWithOps(fixture.ops); err != nil {
		t.Fatal(err)
	}
	if fixture.driver != "" || fixture.override != dpdkCleanupOverride {
		t.Fatal("driverless NIC retained changed native state")
	}
	if fixture.unbindCalls != 1 || fixture.rebindCalls != 0 || fixture.overrideWrites[dpdkCleanupOverride] != 1 {
		t.Fatal("driverless restoration did not match its exact mutations")
	}
	if len(fixture.binder.savedDrivers) != 0 {
		t.Fatal("confirmed driverless restoration retained custody")
	}
	fixture.peerUnchanged(t)
}

// A failure before VFIO binding must restore the completed original detach.
func TestDPDKCleanupRestoresPartialBind(t *testing.T) {
	fixture := newDPDKCleanupFixture(t)
	want := errors.New("VFIO override installation failed")
	setOverride := fixture.ops.setDriverOverride
	fixture.ops.setDriverOverride = func(pci, override string) error {
		fixture.checkPCI(pci)
		if override == "vfio-pci" {
			if fixture.driver != "" || fixture.detachCalls != 1 {
				t.Fatal("failure was not reached after the actual original detach")
			}
			return want
		}
		return setOverride(pci, override)
	}
	err := fixture.binder.bindAllWithOps([]DPDKInterface{{PCIAddress: dpdkCleanupPCI}}, fixture.ops)
	if !errors.Is(err, want) {
		t.Fatalf("partial-bind cause disappeared: %v", err)
	}
	if fixture.driver != "ixgbe" || fixture.override != dpdkCleanupOverride || len(fixture.binder.savedDrivers) != 0 {
		t.Fatal("completed original-driver detach was not rolled back")
	}
	if fixture.detachCalls != 1 || fixture.bindCalls != 0 || fixture.unbindCalls != 0 || fixture.rebindCalls != 1 {
		t.Fatal("rollback tried a VFIO detach that never happened")
	}
	fixture.peerUnchanged(t)
}

// A refused exact PCI bind restores only this device's completed mutations.
func TestDPDKCleanupRestoresFailedExactBind(t *testing.T) {
	fixture := newDPDKCleanupFixture(t)
	bindDriver := fixture.ops.rebindDriver
	fixture.ops.rebindDriver = func(pci, driver string) error {
		fixture.checkPCI(pci)
		if driver == "vfio-pci" {
			fixture.bindCalls++
			return syscall.EBUSY
		}
		return bindDriver(pci, driver)
	}
	err := fixture.binder.bindAllWithOps([]DPDKInterface{{PCIAddress: dpdkCleanupPCI}}, fixture.ops)
	if !errors.Is(err, syscall.EBUSY) {
		t.Fatalf("exact PCI bind refusal disappeared: %v", err)
	}
	if fixture.driver != "ixgbe" || fixture.override != dpdkCleanupOverride {
		t.Fatal("failed bind did not preserve original driver and override")
	}
	if fixture.bindCalls != 1 || fixture.unbindCalls != 0 || fixture.rebindCalls != 1 {
		t.Fatal("rollback changed native state owned by another binder")
	}
	if len(fixture.binder.savedDrivers) != 0 {
		t.Fatal("confirmed partial rollback retained custody")
	}
	fixture.peerUnchanged(t)
}

// A refused original detach leaves the device unchanged; rollback verifies it.
func TestDPDKCleanupRecognizesUnchangedOriginalDriver(t *testing.T) {
	fixture := newDPDKCleanupFixture(t)
	want := errors.New("original detach refused")
	fixture.ops.unbindFromDriver = func(pci string) error {
		fixture.checkPCI(pci)
		return want
	}
	err := fixture.binder.bindAllWithOps([]DPDKInterface{{PCIAddress: dpdkCleanupPCI}}, fixture.ops)
	if !errors.Is(err, want) {
		t.Fatalf("original-detach refusal disappeared: %v", err)
	}
	if fixture.driver != "ixgbe" || fixture.override != dpdkCleanupOverride || len(fixture.binder.savedDrivers) != 0 {
		t.Fatal("unchanged original driver retained a false cleanup obligation")
	}
	if fixture.bindCalls != 0 || fixture.unbindCalls != 0 || fixture.rebindCalls != 0 || len(fixture.overrideWrites) != 0 {
		t.Fatal("rollback mutated an unchanged original driver")
	}
	fixture.peerUnchanged(t)
}

// A failed original-driver selection resumes without repeating confirmed detach.
func TestDPDKCleanupRetriesOnlyUnconfirmedDriverSelection(t *testing.T) {
	fixture := newDPDKCleanupFixture(t)
	fixture.bind(t)
	want := errors.New("original-driver override selection refused")
	selectionCalls := 0
	setOverride := fixture.ops.setDriverOverride
	fixture.ops.setDriverOverride = func(pci, override string) error {
		fixture.checkPCI(pci)
		if override == "ixgbe" {
			selectionCalls++
			if selectionCalls == 1 {
				return want
			}
		}
		return setOverride(pci, override)
	}
	if err := fixture.binder.unbindAllWithOps(fixture.ops); !errors.Is(err, want) {
		t.Fatalf("original-driver selection cause disappeared: %v", err)
	}
	if fixture.driver != "" || fixture.override != "vfio-pci" {
		t.Fatal("failed selection changed unconfirmed native state")
	}
	original, ok := fixture.binder.savedDrivers[dpdkCleanupPCI]
	if !ok {
		t.Fatal("failed driver selection lost original restoration custody")
	}
	if original.driver != "ixgbe" || original.override != dpdkCleanupOverride {
		t.Fatal("failed driver selection lost original restoration custody")
	}
	if fixture.unbindCalls != 1 || fixture.rebindCalls != 0 {
		t.Fatal("failed selection did not preserve the completed detach boundary")
	}
	if err := fixture.binder.unbindAllWithOps(fixture.ops); err != nil {
		t.Fatal(err)
	}
	fixture.restored(t)
	if fixture.unbindCalls != 1 || selectionCalls != 2 || fixture.rebindCalls != 1 || fixture.overrideWrites[dpdkCleanupOverride] != 1 {
		t.Fatal("driver-selection retry repeated a confirmed native step")
	}
}

// dpdkCleanupNativeDevice is observed state of an unconfigured matching NIC.
type dpdkCleanupNativeDevice struct {
	driver   string
	override string
}

// dpdkCleanupFixture models one configured device and an unconfigured peer.
// Every native operation validates its exact PCI target before changing state.
// Driver and override state are independent of the binder's custody receipts.
// Not safe for concurrent use; each test owns its fixture and runs serially.
type dpdkCleanupFixture struct {
	test            *testing.T
	binder          *DPDKBinder
	ops             dpdkBindOps
	driver          string
	override        string
	peers           map[string]dpdkCleanupNativeDevice
	overrideWrites  map[string]int
	detachCalls     int
	bindCalls       int
	unbindCalls     int
	rebindCalls     int
	unbindFailure   error
	rebindFailure   error
	overrideFailure error
}

func newDPDKCleanupFixture(t *testing.T) *dpdkCleanupFixture {
	t.Helper()
	f := &dpdkCleanupFixture{
		test: t, binder: newDPDKBinder(), driver: "ixgbe", override: dpdkCleanupOverride,
		peers:          map[string]dpdkCleanupNativeDevice{dpdkCleanupPeerPCI: {driver: "", override: "peer-policy"}},
		overrideWrites: make(map[string]int),
	}
	f.ops = dpdkBindOps{
		loadModules: func() error { return nil },
		currentDriver: func(pci string) (string, error) {
			f.checkPCI(pci)
			return f.driver, nil
		},
		readDriverOverride: func(pci string) (string, error) {
			f.checkPCI(pci)
			return f.override, nil
		},
		setDriverOverride: func(pci, override string) error {
			f.checkPCI(pci)
			f.overrideWrites[override]++
			if override == dpdkCleanupOverride && f.overrideFailure != nil {
				return f.overrideFailure
			}
			f.override = override
			return nil
		},
		unbindFromDriver: func(pci string) error {
			f.checkPCI(pci)
			f.detachCalls++
			if f.driver != "ixgbe" {
				return fmt.Errorf("fixture original driver is %q", f.driver)
			}
			f.driver = ""
			return nil
		},
		unbindFromVFIO: func(pci string) error {
			f.checkPCI(pci)
			f.unbindCalls++
			if f.unbindFailure != nil {
				return f.unbindFailure
			}
			if f.driver != "vfio-pci" {
				return errors.New("fixture device is no longer bound to vfio-pci")
			}
			f.driver = ""
			return nil
		},
		rebindDriver: func(pci, driver string) error {
			f.checkPCI(pci)
			if f.driver != "" {
				return fmt.Errorf("fixture bind attempted while driver %q remains attached", f.driver)
			}
			if f.override != driver {
				return fmt.Errorf("fixture override %q excludes driver %q", f.override, driver)
			}
			switch driver {
			case "vfio-pci":
				f.bindCalls++
			case "ixgbe":
				f.rebindCalls++
				if f.rebindFailure != nil {
					return f.rebindFailure
				}
			default:
				return fmt.Errorf("fixture received another driver %q", driver)
			}
			f.driver = driver
			return nil
		},
	}
	return f
}

func (f *dpdkCleanupFixture) bind(t *testing.T) {
	t.Helper()
	if err := f.binder.bindAllWithOps([]DPDKInterface{{PCIAddress: dpdkCleanupPCI, Name: "dpdk0"}}, f.ops); err != nil {
		t.Fatal(err)
	}
	if f.driver != "vfio-pci" {
		t.Fatalf("useful bind left native driver=%q", f.driver)
	}
	if f.override != "vfio-pci" || f.bindCalls != 1 {
		t.Fatal("useful bind did not select and bind this exact device to VFIO")
	}
	original, ok := f.binder.savedDrivers[dpdkCleanupPCI]
	if !ok {
		t.Fatal("useful bind did not retain its original driver and override")
	}
	if original.driver != "ixgbe" || original.override != dpdkCleanupOverride {
		t.Fatal("useful bind did not retain its original driver and override")
	}
	f.peerUnchanged(t)
}

func (f *dpdkCleanupFixture) restored(t *testing.T) {
	t.Helper()
	if f.driver != "ixgbe" {
		t.Fatalf("original driver was not restored: %q", f.driver)
	}
	if f.override != dpdkCleanupOverride {
		t.Fatalf("original override was not restored: %q", f.override)
	}
	if len(f.binder.savedDrivers) != 0 {
		t.Fatal("confirmed driver and override restoration retained a stale receipt")
	}
	f.peerUnchanged(t)
}

func (f *dpdkCleanupFixture) checkPCI(pci string) {
	f.test.Helper()
	if pci != dpdkCleanupPCI {
		f.test.Fatalf("native operation targeted another PCI device: %q", pci)
	}
}

func (f *dpdkCleanupFixture) peerUnchanged(t *testing.T) {
	t.Helper()
	if len(f.peers) != 1 || f.peers[dpdkCleanupPeerPCI] != (dpdkCleanupNativeDevice{driver: "", override: "peer-policy"}) {
		t.Fatal("native operation changed an unconfigured matching device or its override")
	}
}
