// Design: docs/research/vpp-deployment-reference.md -- NIC driver matrix and DPDK bind sequence
// Related: config.go -- DPDKInterface struct with PCI addresses

package vpp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
)

var errLoadmoduleNotImplementedOnThisPlatform = errors.New("loadModule not implemented on this platform")

// DPDKBinder manages DPDK NIC driver binding and unbinding.
// It saves original drivers so they can be restored on teardown.
// MUST call UnbindAll on shutdown to restore original drivers.
type DPDKBinder struct {
	// savedDrivers retains original per-device state until restoration succeeds.
	savedDrivers map[string]dpdkDriverRestore
}

type dpdkDriverRestore struct {
	driver   string
	override string
}

// newDPDKBinder creates a new DPDK NIC binder.
func newDPDKBinder() *DPDKBinder {
	return &DPDKBinder{
		savedDrivers: make(map[string]dpdkDriverRestore),
	}
}

// vfioModules are the kernel modules required for vfio-pci binding.
var vfioModules = []string{"vfio", "vfio_pci", "vfio_iommu_type1"}

// sysfsDevDir is the sysfs path prefix for PCI devices.
const sysfsDevDir = "/sys/bus/pci/devices"

// dpdkBindOps captures the native boundaries for one binding or cleanup call.
// Every callback MUST be nonnil; nativeDPDKBindOps supplies the production set.
// The table is immutable while an operation uses it.
type dpdkBindOps struct {
	loadModules        func() error
	currentDriver      func(string) (string, error)
	readDriverOverride func(string) (string, error)
	setDriverOverride  func(string, string) error
	unbindFromDriver   func(string) error
	unbindFromVFIO     func(string) error
	rebindDriver       func(string, string) error
}

func nativeDPDKBindOps() dpdkBindOps {
	return dpdkBindOps{
		loadModules: loadVFIOModules, currentDriver: currentDriver,
		readDriverOverride: readDriverOverride, setDriverOverride: setDriverOverride,
		unbindFromDriver: unbindFromDriver, unbindFromVFIO: unbindFromVFIO,
		rebindDriver: rebindDriver,
	}
}

// BindAll binds all configured DPDK interfaces to vfio-pci.
// MUST be called before starting VPP.
// Caller MUST call UnbindAll on teardown to restore original drivers.
func (d *DPDKBinder) BindAll(interfaces []DPDKInterface) error {
	return d.bindAllWithOps(interfaces, nativeDPDKBindOps())
}

// bindAllWithOps preserves one captured operation set through rollback.
func (d *DPDKBinder) bindAllWithOps(interfaces []DPDKInterface, ops dpdkBindOps) error {
	if len(interfaces) == 0 {
		return nil
	}
	if err := ops.loadModules(); err != nil {
		return fmt.Errorf("dpdk: load vfio modules: %w", err)
	}

	for _, iface := range interfaces {
		if err := validatePCIAddress(iface.PCIAddress); err != nil {
			return errors.Join(fmt.Errorf("dpdk: %w", err), d.unbindAllWithOps(ops))
		}
		if err := d.bindPCIWithOps(iface.PCIAddress, ops); err != nil {
			return errors.Join(fmt.Errorf("dpdk: bind %s: %w", iface.PCIAddress, err), d.unbindAllWithOps(ops))
		}
	}
	return nil
}

// UnbindAll restores all NICs to their original drivers.
// Safe to call multiple times.
func (d *DPDKBinder) UnbindAll() error {
	return d.unbindAllWithOps(nativeDPDKBindOps())
}

// unbindAllWithOps uses the same captured native boundaries during cleanup.
func (d *DPDKBinder) unbindAllWithOps(ops dpdkBindOps) error {
	var failure error
	for pci, original := range d.savedDrivers {
		if err := restoreDPDKDevice(pci, original, ops); err != nil {
			failure = errors.Join(failure, fmt.Errorf("dpdk: restore %s: %w", pci, err))
			continue
		}
		delete(d.savedDrivers, pci)
	}
	return failure
}

// restoreDPDKDevice observes the original device on every attempt. Confirmed
// detach, original-driver bind and override writes MUST NOT be repeated. A
// foreign replacement driver is not this binder's resource to unbind.
func restoreDPDKDevice(pci string, original dpdkDriverRestore, ops dpdkBindOps) error {
	driver, err := ops.currentDriver(pci)
	if err != nil {
		return fmt.Errorf("read current driver: %w", err)
	}
	if driver != "" && driver != original.driver {
		if driver != "vfio-pci" {
			return fmt.Errorf("current driver %q differs from owned vfio-pci and original %q", driver, original.driver)
		}
		if err := ops.unbindFromVFIO(pci); err != nil {
			return fmt.Errorf("unbind vfio-pci: %w", err)
		}
		driver, err = ops.currentDriver(pci)
		if err != nil {
			return fmt.Errorf("verify vfio-pci detachment: %w", err)
		}
		if driver != "" {
			return fmt.Errorf("vfio-pci detachment left driver %q", driver)
		}
	}
	if driver == "" && original.driver != "" {
		// A pre-existing override may exclude the original driver. Select it
		// only for this exact bind, then restore the original override below.
		if err := setDPDKOverride(pci, original.driver, ops); err != nil {
			return err
		}
		if err := ops.rebindDriver(pci, original.driver); err != nil {
			return fmt.Errorf("bind original driver %s: %w", original.driver, err)
		}
		actual, err := ops.currentDriver(pci)
		if err != nil {
			return fmt.Errorf("verify original driver: %w", err)
		}
		if actual != original.driver {
			return fmt.Errorf("original driver %q not restored: current driver %q", original.driver, actual)
		}
	}
	return setDPDKOverride(pci, original.override, ops)
}

func setDPDKOverride(pci, wanted string, ops dpdkBindOps) error {
	actual, err := ops.readDriverOverride(pci)
	if err != nil {
		return fmt.Errorf("read driver override: %w", err)
	}
	if actual == wanted {
		return nil
	}
	if err := ops.setDriverOverride(pci, wanted); err != nil {
		return fmt.Errorf("set driver override %q: %w", wanted, err)
	}
	actual, err = ops.readDriverOverride(pci)
	if err != nil {
		return fmt.Errorf("verify driver override: %w", err)
	}
	if actual != wanted {
		return fmt.Errorf("driver override %q not restored: current override %q", wanted, actual)
	}
	return nil
}

// bindPCI binds a single PCI device to vfio-pci.
func (d *DPDKBinder) bindPCI(pci string) error {
	return d.bindPCIWithOps(pci, nativeDPDKBindOps())
}

// bindPCIWithOps binds one device through the caller's captured operations.
func (d *DPDKBinder) bindPCIWithOps(pci string, ops dpdkBindOps) error {
	if _, pending := d.savedDrivers[pci]; pending {
		return errors.New("device has unresolved original-state restoration")
	}
	original, err := readDPDKOriginalState(pci, ops)
	if err != nil {
		return err
	}
	if original.driver == "vfio-pci" {
		return nil // already bound; existing native state remains borrowed
	}
	// Take custody before the first native mutation, including driverless NICs.
	d.savedDrivers[pci] = original

	if original.driver != "" {
		if err := ops.unbindFromDriver(pci); err != nil {
			return fmt.Errorf("unbind from %s: %w", original.driver, err)
		}
	}

	if err := setDPDKOverride(pci, "vfio-pci", ops); err != nil {
		return err
	}
	if err := ops.rebindDriver(pci, "vfio-pci"); err != nil {
		return fmt.Errorf("bind to vfio-pci: %w", err)
	}
	actual, err := ops.currentDriver(pci)
	if err != nil {
		return fmt.Errorf("verify vfio-pci binding: %w", err)
	}
	if actual != "vfio-pci" {
		return fmt.Errorf("vfio-pci did not bind: current driver %q", actual)
	}
	return nil
}

// readDPDKOriginalState is the read-only prerequisite shared by binding and
// doctor. An existing VFIO attachment stays borrowed and needs no override.
func readDPDKOriginalState(pci string, ops dpdkBindOps) (dpdkDriverRestore, error) {
	driver, err := ops.currentDriver(pci)
	if err != nil {
		return dpdkDriverRestore{}, fmt.Errorf("read current driver: %w", err)
	}
	if driver == "vfio-pci" {
		return dpdkDriverRestore{driver: driver}, nil
	}
	override, err := ops.readDriverOverride(pci)
	if err != nil {
		return dpdkDriverRestore{}, fmt.Errorf("read original driver override: %w", err)
	}
	return dpdkDriverRestore{driver: driver, override: override}, nil
}

// currentDriver reads the current driver for a PCI device from sysfs.
// Returns empty string if no driver is bound.
func currentDriver(pci string) (string, error) {
	link := filepath.Join(sysfsDevDir, pci, "driver")
	target, err := os.Readlink(link)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return filepath.Base(target), nil
}

// readDriverOverride reads exact original per-device selection through the raw
// getter, or an unambiguous legacy value when that getter is absent.
func readDriverOverride(pci string) (string, error) {
	return readDriverOverrideAt(filepath.Join(sysfsDevDir, pci))
}

// readDriverOverrideAt preserves the complete native reader with an explicit
// device-directory boundary. The caller supplies the original device path.
func readDriverOverrideAt(deviceDir string) (string, error) {
	value, rawErr := readSysfsValue(filepath.Join(deviceDir, "driver_override_raw"))
	if rawErr == nil {
		return value, nil
	}
	if !errors.Is(rawErr, os.ErrNotExist) {
		return "", fmt.Errorf("read native driver_override_raw: %w", rawErr)
	}
	value, err := readSysfsValue(filepath.Join(deviceDir, "driver_override"))
	if err != nil {
		return "", errors.Join(rawErr, err)
	}
	if value == "(null)" {
		return "", opererr.Wrap(rawErr, "The original driver override at "+deviceDir+" is ambiguous",
			"The legacy driver_override value cannot distinguish an unset override from the literal policy (null).",
			"Upgrade to a Ze kernel providing driver_override_raw before binding this NIC")
	}
	return value, nil
}

func setDriverOverride(pci, driver string) error {
	return setDriverOverrideAt(filepath.Join(sysfsDevDir, pci), driver)
}

func setDriverOverrideAt(deviceDir, driver string) error {
	value := driver
	if value == "" {
		value = "\n"
	}
	return writeSysfs(filepath.Join(deviceDir, "driver_override"), value)
}

// unbindFromDriver unbinds a PCI device from its current driver.
func unbindFromDriver(pci string) error {
	path := filepath.Join(sysfsDevDir, pci, "driver", "unbind")
	return writeSysfs(path, pci)
}

// unbindFromVFIO unbinds a PCI device from vfio-pci.
func unbindFromVFIO(pci string) error {
	path := "/sys/bus/pci/drivers/vfio-pci/unbind"
	return writeSysfs(path, pci)
}

// rebindDriver rebinds a PCI device to the specified driver.
func rebindDriver(pci, driver string) error {
	bindPath := "/sys/bus/pci/drivers/" + driver + "/bind"
	return writeSysfs(bindPath, pci)
}

// loadVFIOModules loads the required kernel modules for vfio-pci.
func loadVFIOModules() error {
	for _, mod := range vfioModules {
		if err := loadModule(mod); err != nil {
			return fmt.Errorf("modprobe %s: %w", mod, err)
		}
	}
	return nil
}

// loadModule loads a kernel module via modprobe.
// This is a platform-specific operation; see dpdk_linux.go and dpdk_other.go.
var loadModule = func(_ string) error {
	return errLoadmoduleNotImplementedOnThisPlatform
}

// writeSysfs writes a value to a sysfs path.
func writeSysfs(path, value string) error {
	return os.WriteFile(path, []byte(value), 0o200)
}

// readSysfsValue removes only the sysfs framing newline. Override policy may
// contain meaningful spaces, tabs or carriage returns, which MUST be retained.
func readSysfsValue(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is built from validated PCI address + fixed sysfs suffix
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(data), "\n"), nil
}
