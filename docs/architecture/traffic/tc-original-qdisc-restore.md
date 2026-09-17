# Restoring an Interface's Original tc Qdisc

The netlink traffic backend replaces an interface's root qdisc when it applies a
`traffic-control { }` config. On withdrawal it must put back what was there
before, and "before" survives a daemon restart. That needs a persisted snapshot.

## The snapshot lives in the shared state store

<!-- source: internal/plugins/traffic/netlink/snapshot_linux.go -- tcSnapshotStore, loadTCSnapshots, saveTCSnapshots -->

The snapshot is versioned JSON in the daemon's selected `database/` tree,
through `internal/core/statestore`, under `KeyTrafficTCSnapshot`. The same
owning handle serves configuration and runtime state for the daemon's lifetime.
Explicit-file startup selects the tree beside that file without a
`ze.config.dir` pin.

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

`tcOps` is the narrow unexported interface over the ten netlink calls the
backend makes (link lookup, qdisc list, add, replace and delete, class list and
add, filter list, add and delete). `netlinkOps` is the production adapter. The
snapshot and restore branches are therefore testable without a live interface.

`qdiscAdd` and `filterDel` belong to the ingress policer. The policer attaches
at the shared `clsact` hook, whose qdisc object the mirror and sampling paths
also hang filters on, so it is ADDED rather than replaced and only the
policer's own filter priority is deleted. Restore clears that priority
unconditionally, before the root qdisc goes back: the desired end state is "no
policer at that priority", and asking the kernel for it is correct whether or
not this process installed one. That also clears a policer orphaned by a
restart, which a remembered-state check would miss.

## Reapplying a root with the same handle

<!-- source: internal/plugins/traffic/netlink/backend_linux.go -- replaceRootQdisc -->

Ze uses root handle `1:0`. After a crash that root remains in the kernel.
Linux routes a replacement with the same handle through `qdisc_change`
(`net/sched/sch_api.c`). HTB has no change callback, so this request returns
`EINVAL`, even with a valid HTB version and an available kernel scheduler.

The backend reads the live root before replacement. If its handle matches the
requested handle, it deletes that root before installing the replacement.
This also removes its old classes and filters before the backend rebuilds them.
The original snapshot remains durable throughout both operations. Restoration
uses the same path because the original qdisc can also have handle `1:0`.

## Restart evidence

`storage/consumer-restart tc` creates a private dummy interface with an
`fq_codel` root carrying a custom limit and quantum. It starts Ze from an
explicit config, waits for HTB in the kernel, and kills that daemon after the
snapshot was published. A second daemon starts from the same config and its
shutdown must restore the original handle and both custom parameters. The
snapshot key must then be absent.

The draft carrier is `test/draft/traffic/storage-tc-restart.ci`. It requires
Linux, `CAP_NET_ADMIN`, and `iproute2`, so a host without those capabilities
uses the disposable QEMU guest. It changes only its own dummy interface.
<!-- source: internal/test/fixture/storage_consumer_tc.go -- storageTCRestart -->
