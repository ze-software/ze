// Design: docs/architecture/core-design.md -- ze:backend commit-time feature gate
// Related: cmd_validate.go -- runValidation consults trafficDefaultBackend
//
// On non-Linux platforms the traffic plugin has no default backend
// (internal/component/traffic/default_other.go). Returning "" triggers the
// walker's empty-backend rejection, matching what the daemon reports on
// startup when the user has not set `traffic-control.backend`.

//go:build !linux

package config

func trafficDefaultBackend() string { return "" }
