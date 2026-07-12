// Design: ai/digests/flow-ddos.md -- appliance conntrack init (gokrazy runs only ze)
//
//go:build !(linux && ze_appliance)

package flowexport

import "log/slog"

// ensureConntrackTracking is a no-op off the gokrazy appliance. On a normal host
// something else registers conntrack tracking -- an operator firewall rule that
// references `ct`, an explicit modprobe, or (in the functional-test VM) the
// qemu-run.py setup. Only the appliance, where the gokrazy init runs ONLY ze and
// nothing loads nf_conntrack or registers a tracking hook, needs ze to do it
// itself; that path is compiled in solely under the ze_appliance build tag (see
// conntrack_setup_appliance_linux.go).
func ensureConntrackTracking(_ *slog.Logger) {}
