// Design: docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md -- the Active router's macvlan hears its addresses' solicitations before it holds them
//
// The Solicited-Node pre-join (solicitednode_linux.go) is Linux-only: it holds
// link-layer memberships on the macvlan. Off Linux the whole VRRP dataplane is
// unreachable anyway (no macvlan, no raw sockets), so these are no-ops that keep
// the engine wiring portable, as dataplane_other.go does.

//go:build !linux

package vrrp

import "net/netip"

func joinSolicitedNode(_, _ string, _ []netip.Addr) error { return nil }

func leaveSolicitedNode(_ string) error { return nil }
