// Design: plan/spec-host-0-inventory.md — hardware inventory detection

//go:build linux

package host

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// dmiFields maps a DMI leaf file name to the DMIInfo setter. This table
// IS the contract between /sys/class/dmi/id/ and the exported struct;
// adding a field means adding a row here.
var dmiFields = []struct {
	leaf string
	set  func(*DMIInfo, string)
}{
	{"sys_vendor", func(d *DMIInfo, v string) { d.SystemVendor = v }},
	{"product_name", func(d *DMIInfo, v string) { d.SystemProduct = v }},
	{"product_version", func(d *DMIInfo, v string) { d.SystemVersion = v }},
	{"product_serial", func(d *DMIInfo, v string) { d.SystemSerial = v }},
	{"board_vendor", func(d *DMIInfo, v string) { d.BoardVendor = v }},
	{"board_name", func(d *DMIInfo, v string) { d.BoardProduct = v }},
	{"board_version", func(d *DMIInfo, v string) { d.BoardVersion = v }},
	{"board_serial", func(d *DMIInfo, v string) { d.BoardSerial = v }},
	{"bios_vendor", func(d *DMIInfo, v string) { d.BIOSVendor = v }},
	{"bios_version", func(d *DMIInfo, v string) { d.BIOSVersion = v }},
	{"bios_date", func(d *DMIInfo, v string) { d.BIOSDate = v }},
	{"bios_revision", func(d *DMIInfo, v string) { d.BIOSRevision = v }},
	{"chassis_vendor", func(d *DMIInfo, v string) { d.ChassisVendor = v }},
	{"chassis_type", func(d *DMIInfo, v string) { d.ChassisType = v }},
	{"chassis_serial", func(d *DMIInfo, v string) { d.ChassisSerial = v }},
}

// DetectDMI reads /sys/class/dmi/id/* under the Detector's Root. Fields
// not present on disk are left empty (omitempty in JSON). Permission
// errors are returned as non-fatal partial results via detectErrors.
//
// The top-level error is only returned when the /sys/class/dmi/id/
// directory itself is unreadable — an indication the DMI interface
// isn't mounted or the container lacks /sys access.
func (d *Detector) DetectDMI() (*DMIInfo, error) {
	return d.detectDMI()
}

func (d *Detector) detectDMI() (*DMIInfo, error) {
	dir := d.sysfsPath("class/dmi/id")
	if _, err := os.Stat(dir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrUnsupported
		}
		return nil, err
	}
	info := &DMIInfo{}
	for _, f := range dmiFields {
		path := filepath.Join(dir, f.leaf)
		v, readErr := readDMIField(path)
		if readErr != nil {
			recordSysfsErr(&info.Errors, path, readErr)
			continue
		}
		if v != "" {
			f.set(info, v)
		}
	}
	return info, nil
}

// readDMIField reads one DMI leaf file. Returns "" with nil error when
// the file is absent (field simply isn't provided by firmware). Returns
// a non-nil error on permission-denied or other IO trouble; the caller
// records it via recordSysfsErr so the operator can see which path
// they need read access to.
func readDMIField(path string) (string, error) {
	b, err := os.ReadFile(path) //nolint:gosec // caller-scoped /sys/class/dmi path
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
