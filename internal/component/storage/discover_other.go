//go:build !linux

// Design: plan/learned/808-smart-management.md — SMART disk health management
// Related: manager.go — Manager.poll calls discoverBlockDevices

package storage

// discoverBlockDevices is a no-op on non-Linux platforms.
func discoverBlockDevices() []string { return nil }
