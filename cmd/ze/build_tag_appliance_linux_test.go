// Design: docs/architecture/appliance/kernel-profiles.md -- Loading driver modules on the device
//
//go:build ze_core && ze_appliance && linux

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/cmd/ze/startprep"
)

// TestZeApplianceBinaryRegistersModuleColdplug proves the appliance flavor
// carries the driver-module cold-plug pass as a `ze start` step, which is the
// only way a NIC whose driver is a module gets a driver on gokrazy.
//
// MUTATION: removing the coldplug blank import from
// setup_features_appliance.go, or its startprep.Register, leaves the step out.
func TestZeApplianceBinaryRegistersModuleColdplug(t *testing.T) {
	assert.Contains(t, startprep.Names(), "kernel-module-coldplug")
}
