// Design: plan/spec-iface-0-umbrella.md -- Non-Linux bridge management stub
// Overview: iface.go -- shared types and topic constants

//go:build !linux

package iface

import (
	"fmt"
	"runtime"
)

// BridgeAddPort returns an error on non-Linux platforms.
func BridgeAddPort(_, _ string) error {
	return fmt.Errorf("interface management not supported on %s", runtime.GOOS)
}

// BridgeDelPort returns an error on non-Linux platforms.
func BridgeDelPort(_ string) error {
	return fmt.Errorf("interface management not supported on %s", runtime.GOOS)
}

// BridgeSetSTP returns an error on non-Linux platforms.
func BridgeSetSTP(_ string, _ bool) error {
	return fmt.Errorf("interface management not supported on %s", runtime.GOOS)
}
