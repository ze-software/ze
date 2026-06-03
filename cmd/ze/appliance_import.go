// Design: docs/architecture/cli/plugin-modes.md — appliance command provider import
//
//go:build !ze_stripped

package main

// Blank import: the appliance command provider registers the `appliance` root
// handler with the command registry from init(). Owner lives under
// internal/appliance per command-surface-ownership. Excluded from ze-stripped
// because appliance is build-host tooling not needed on-device.
import _ "codeberg.org/thomas-mangin/ze/internal/appliance"
