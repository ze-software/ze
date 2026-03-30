// Design: plan/spec-iface-0-umbrella.md -- Non-Linux monitor stub
// Overview: iface.go -- shared types and topic constants

//go:build !linux

package iface

import (
	"fmt"
	"runtime"

	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// Monitor is a stub on non-Linux platforms. The netlink monitor
// requires Linux; on other platforms it cannot be created.
type Monitor struct{}

// NewMonitor returns an error on non-Linux platforms.
func NewMonitor(_ ze.Bus) (*Monitor, error) {
	return nil, fmt.Errorf("iface monitor not supported on %s", runtime.GOOS)
}

// Start returns an error on non-Linux platforms.
func (m *Monitor) Start() error {
	return fmt.Errorf("iface monitor not supported on %s", runtime.GOOS)
}

// Stop is a no-op stub.
func (m *Monitor) Stop() {}
