// Design: docs/research/vpp-deployment-reference.md -- NIC driver matrix and DPDK bind sequence
// Related: config.go -- DPDKInterface struct with PCI addresses

package vpp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DPDKBinder manages DPDK NIC driver binding and unbinding.
// It saves original drivers so they can be restored on teardown.
// MUST call UnbindAll on shutdown to restore original drivers.
type DPDKBinder struct {
	// savedDrivers maps PCI address to original driver name.
	savedDrivers map[string]string
	// addedNewIDs tracks vendor:device strings written to vfio-pci/new_id.
	addedNewIDs map[string]bool
}

// NewDPDKBinder creates a new DPDK NIC binder.
func NewDPDKBinder() *DPDKBinder {
	return &DPDKBinder{
		savedDrivers: make(map[string]string),
		addedNewIDs:  make(map[string]bool),
	}
}

// vfioModules are the kernel modules required for vfio-pci binding.
var vfioModules = []string{"vfio", "vfio_pci", "vfio_iommu_type1"}

// sysfsDevDir is the sysfs path prefix for PCI devices.
const sysfsDevDir = "/sys/bus/pci/devices"

// BindAll binds all configured DPDK interfaces to vfio-pci.
// MUST be called before starting VPP.
// Caller MUST call UnbindAll on teardown to restore original drivers.
func (d *DPDKBinder) BindAll(interfaces []DPDKInterface) error {
	if len(interfaces) == 0 {
		return nil
	}
	if err := loadVFIOModules(); err != nil {
		return fmt.Errorf("dpdk: load vfio modules: %w", err)
	}

	for _, iface := range interfaces {
		if err := ValidatePCIAddress(iface.PCIAddress); err != nil {
			_ = d.UnbindAll() // rollback already-bound NICs
			return fmt.Errorf("dpdk: %w", err)
		}
		if err := d.bindPCI(iface.PCIAddress); err != nil {
			_ = d.UnbindAll() // rollback already-bound NICs
			return fmt.Errorf("dpdk: bind %s: %w", iface.PCIAddress, err)
		}
	}
	return nil
}

// UnbindAll restores all NICs to their original drivers.
// Safe to call multiple times.
func (d *DPDKBinder) UnbindAll() error {
	var errs []string
	for pci, driver := range d.savedDrivers {
		if err := unbindFromVFIO(pci); err != nil {
			errs = append(errs, fmt.Sprintf("%s unbind: %v", pci, err))
			continue
		}
		if err := triggerPCIRescan(); err != nil {
			errs = append(errs, fmt.Sprintf("pci rescan: %v", err))
			continue
		}
		_ = rebindDriver(pci, driver) // best effort
		delete(d.savedDrivers, pci)
	}
	for vendorDevice := range d.addedNewIDs {
		_ = removeFromVFIO(vendorDevice)
		delete(d.addedNewIDs, vendorDevice)
	}
	if len(errs) > 0 {
		return fmt.Errorf("dpdk unbind errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// bindPCI binds a single PCI device to vfio-pci.
func (d *DPDKBinder) bindPCI(pci string) error {
	driver, err := currentDriver(pci)
	if err != nil {
		return fmt.Errorf("read current driver: %w", err)
	}
	if driver != "" {
		d.savedDrivers[pci] = driver
	}

	if driver == "vfio-pci" {
		return nil // already bound
	}

	if driver != "" {
		if err := unbindFromDriver(pci); err != nil {
			return fmt.Errorf("unbind from %s: %w", driver, err)
		}
	}

	vendorDevice, err := readVendorDevice(pci)
	if err != nil {
		return fmt.Errorf("read vendor:device: %w", err)
	}

	if err := bindToVFIO(vendorDevice); err != nil {
		return fmt.Errorf("bind to vfio-pci: %w", err)
	}
	d.addedNewIDs[vendorDevice] = true

	return nil
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

// readVendorDevice reads the PCI vendor:device ID for vfio-pci new_id.
func readVendorDevice(pci string) (string, error) {
	vendorPath := filepath.Join(sysfsDevDir, pci, "vendor")
	devicePath := filepath.Join(sysfsDevDir, pci, "device")

	vendor, err := readSysfsValue(vendorPath)
	if err != nil {
		return "", fmt.Errorf("read vendor: %w", err)
	}
	device, err := readSysfsValue(devicePath)
	if err != nil {
		return "", fmt.Errorf("read device: %w", err)
	}

	// Strip 0x prefix if present.
	vendor = strings.TrimPrefix(vendor, "0x")
	device = strings.TrimPrefix(device, "0x")

	return vendor + " " + device, nil
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

// bindToVFIO binds a PCI device to vfio-pci via new_id.
func bindToVFIO(vendorDevice string) error {
	path := "/sys/bus/pci/drivers/vfio-pci/new_id"
	return writeSysfs(path, vendorDevice)
}

// removeFromVFIO removes a vendor:device pair from vfio-pci/remove_id.
func removeFromVFIO(vendorDevice string) error {
	path := "/sys/bus/pci/drivers/vfio-pci/remove_id"
	return writeSysfs(path, vendorDevice)
}

// triggerPCIRescan triggers a PCI bus rescan to rediscover devices.
func triggerPCIRescan() error {
	return writeSysfs("/sys/bus/pci/rescan", "1")
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
	return fmt.Errorf("loadModule not implemented on this platform")
}

// writeSysfs writes a value to a sysfs path.
func writeSysfs(path, value string) error {
	return os.WriteFile(path, []byte(value), 0o200)
}

// readSysfsValue reads and trims a sysfs file value.
func readSysfsValue(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is built from validated PCI address + fixed sysfs suffix
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
