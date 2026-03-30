// Design: plan/spec-iface-0-umbrella.md -- Non-Linux interface management stub
// Overview: iface.go -- shared types and topic constants

//go:build !linux

package iface

import (
	"fmt"
	"runtime"
)

// CreateDummy returns an error on non-Linux platforms.
func CreateDummy(_ string) error {
	return fmt.Errorf("interface management not supported on %s", runtime.GOOS)
}

// CreateVeth returns an error on non-Linux platforms.
func CreateVeth(_, _ string) error {
	return fmt.Errorf("interface management not supported on %s", runtime.GOOS)
}

// CreateBridge returns an error on non-Linux platforms.
func CreateBridge(_ string) error {
	return fmt.Errorf("interface management not supported on %s", runtime.GOOS)
}

// CreateVLAN returns an error on non-Linux platforms.
func CreateVLAN(_ string, _ int) error {
	return fmt.Errorf("interface management not supported on %s", runtime.GOOS)
}

// DeleteInterface returns an error on non-Linux platforms.
func DeleteInterface(_ string) error {
	return fmt.Errorf("interface management not supported on %s", runtime.GOOS)
}

// AddAddress returns an error on non-Linux platforms.
func AddAddress(_, _ string) error {
	return fmt.Errorf("interface management not supported on %s", runtime.GOOS)
}

// RemoveAddress returns an error on non-Linux platforms.
func RemoveAddress(_, _ string) error {
	return fmt.Errorf("interface management not supported on %s", runtime.GOOS)
}

// SetMTU returns an error on non-Linux platforms.
func SetMTU(_ string, _ int) error {
	return fmt.Errorf("interface management not supported on %s", runtime.GOOS)
}
