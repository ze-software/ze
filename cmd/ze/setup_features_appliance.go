// Design: docs/architecture/cli/plugin-modes.md — ze_appliance feature wiring
//
// ze_appliance is reserved for on-device appliance runtime features.
// Appliance build tooling (init, build, iso, kernel, initrd) lives
// in ze_setup only.
//
//go:build ze_appliance

package main

// coldplug loads, at `ze start`, the kernel driver modules the devices
// present need: gokrazy runs no udev and no modprobe.
import _ "github.com/ze-software/ze/internal/plugins/coldplug"
