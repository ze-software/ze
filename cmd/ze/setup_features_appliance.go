// Design: docs/architecture/cli/plugin-modes.md — ze_appliance feature wiring
//
// ze_appliance is reserved for on-device appliance runtime features.
// Appliance build tooling (init, build, iso, kernel, initrd) lives
// in ze_setup only.
//
//go:build ze_appliance

package main
