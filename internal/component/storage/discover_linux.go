//go:build linux

// Design: plan/learned/808-smart-management.md — SMART disk health management
// Related: manager.go — Manager.poll calls discoverBlockDevices

package storage

import (
	"os"
	"path/filepath"
)

// discoverBlockDevices returns the names of top-level block devices
// (partitions excluded) by walking /sys/class/block/.
func discoverBlockDevices() []string {
	entries, err := os.ReadDir("/sys/class/block")
	if err != nil {
		return nil
	}
	var devices []string
	for _, e := range entries {
		name := e.Name()
		if isPartition(filepath.Join("/sys/class/block", name)) {
			continue
		}
		devices = append(devices, name)
	}
	return devices
}

func isPartition(dev string) bool {
	_, err := os.Stat(filepath.Join(dev, "partition"))
	return err == nil
}
