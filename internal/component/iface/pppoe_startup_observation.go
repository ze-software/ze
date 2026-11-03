//go:build !integration || !linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- startup cardinality proof.
// Related: pppoe_startup_observation_integration_linux.go -- integration observer.

package iface

// observePPPoEStartup records nothing outside Linux integration builds.
// The startup path supplies only counts; this seam cannot alter configuration.
func observePPPoEStartup(_, _ int) {}
