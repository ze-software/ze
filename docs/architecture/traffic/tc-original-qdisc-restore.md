# Restoring an Interface's Original tc Qdisc

The netlink traffic backend replaces an interface's root qdisc when it applies a
`traffic-control { }` config. On withdrawal it must put back what was there
before, and "before" survives a daemon restart. That needs a persisted snapshot.

## The snapshot lives in the shared state store

<!-- source: internal/plugins/traffic/netlink/snapshot_linux.go -- tcSnapshotStore, loadTCSnapshots, saveTCSnapshots -->

The snapshot is a versioned JSON blob in the shared zefs store
(`database.zefs`) through `internal/core/statestore`, under
`KeyTrafficTCSnapshot`. It is not a loose file, so appliance state stays inside
the managed, backed-up store.

- A missing key yields an empty set with no error: there is nothing to restore.
- A blob that fails to parse, or that carries an unsupported version, fails the
  backend LOUDLY. Silently discarding restore state loses the operator's original
  qdisc.
- When no snapshots remain the key is REMOVED, so a stale blob cannot outlive the
  config that produced it.

## A snapshot is validated against the live link before it is used

<!-- source: internal/plugins/traffic/netlink/snapshot_linux.go -- validateLink, currentBootID -->

`validateLink` refuses a snapshot whose boot ID, interface name, ifindex or
hardware address does not match the live link. The boot ID comes from
`/proc/sys/kernel/random/boot_id`, so a snapshot taken before a reboot cannot be
applied after one. All four attributes must agree: a persisted snapshot names a
device identity, not a device name.

## `noqueue` is restored by DELETING the root

<!-- source: internal/plugins/traffic/netlink/ops_linux.go -- tcOps.qdiscDel -->
<!-- source: internal/plugins/traffic/netlink/snapshot_linux.go -- restoredByDelete -->

`noqueue` is the kernel's own representation of "no queueing discipline
configured". It is the default root on every virtual interface (veth, dummy,
bridge, and anything else the kernel gives no real queue), so it is the state a
QoS config is most often applied FROM. It is not an exotic corner.

Re-entering that state means deleting the root qdisc. ADDING a qdisc named
`noqueue` is not the inverse operation.

`restoredByDelete()` marks that case, and the code that builds a replacement
qdisc refuses a delete-restored snapshot explicitly rather than producing
something that looks right.

## A qdisc that cannot be reproduced exactly is refused

<!-- source: internal/plugins/traffic/netlink/snapshot_linux.go -- snapshot and restore error paths -->

Both the snapshot side and the restore side fail with
`qdisc %q cannot be snapshotted exactly by backend tc` and
`qdisc %q cannot be restored exactly by backend tc`. This is exact-or-reject
(`ai/rules/protocol.md`) applied to the restore path: an approximate restore is
an operator config silently rewritten.

## The `tcOps` seam

<!-- source: internal/plugins/traffic/netlink/ops_linux.go -- tcOps, netlinkOps -->

`tcOps` is the narrow unexported interface over the eight netlink calls the
backend makes (link lookup, qdisc list, replace and delete, class list and add,
filter list and add). `netlinkOps` is the production adapter. The snapshot and
restore branches are therefore testable without a live interface.
