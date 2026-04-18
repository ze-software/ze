// Design: plan/spec-host-0-inventory.md — hardware inventory detection
//
// Package host reads the physical hardware inventory from sysfs, procfs,
// and netlink. It is read-only and stateless: every Detect call returns
// a fresh Inventory value. No pools, no caches, no globals. Safe for
// concurrent use.
//
// Platform: Linux implementations live in *_linux.go files; other
// platforms get a stub that returns ErrUnsupported.
//
// Consumers include:
//   - internal/component/cmd/show/host.go   online `show host *` RPCs
//   - cmd/ze/host                           offline `ze host show`
//   - internal/component/cmd/show/system.go `show system *` enrichment
//
// Future spec-host-1-observability will add caching, Prometheus export,
// and hardware-change events on the report bus. Future spec-host-2-tuning
// will add governor/IRQ/ethtool writes driven by the inventory shape.
package host
