// Design: docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md -- virtual-MAC dataplane (ARP/ND ownership)
//
// The virtual-MAC ARP recipe (dataplane_linux.go) is Linux-only: it writes
// procfs sysctls and only matters where the netlink backend runs macvlans. Off
// Linux the whole VRRP dataplane is unreachable anyway (no macvlan, no raw
// sockets), so these are no-ops that keep the engine wiring portable.

//go:build !linux

package vrrp

func applyDataplaneSysctls(_, _, _ string) error { return nil }

func reassertDataplaneSysctls(_, _, _ string) {}

func revertDataplaneSysctls(_, _, _ string) {}
