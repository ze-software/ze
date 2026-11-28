# Handover: fib-kernel label ownership spec

Originally written 2026-11-24; trimmed 2026-11-28 after `spec-fib-dummy` closed
(`3f3e418525`, `4cbf5dbf47`, follow-up verification in `plan/learned/054-fib-dummy.md`).
No session holds a claim on this spec. Remove this file through
`./le commit create ... remove` once the spec is approved and set to `ready`.

## The spec

| Spec | Status | Last commit | Judge state |
|------|--------|-------------|-------------|
| `plan/release/fixes/spec-fib-kernel-label-ownership.md` | design | `3f3e418525` (citer update; last design edit `ab580c27e5`) | Fifth judge refused digest `dd2f9a383fcc` |

This spec was split out of `spec-fib-dummy` because `fib-kernel`'s ownership of
MPLS label routes is defective without the dummy and needed a design of its own.
`spec-fib-dummy` has landed and added `fib-kernel`'s targeted label replay on
open; re-read `internal/plugins/fib/kernel/{fibkernel,mplsentry,register}.go`
before implementing or revising this spec.

## Fifth judge refusal (`dd2f9a383fcc`)

Record: `tmp/spec-design/judged/spec-fib-kernel-label-ownership.md`. The fourth
judge's defect is resolved; one new defect, with the fix the judge gave:

- **The superseded record has no slot priority.** `supersededTables`
  (`map[string][]uint32`), `recordSupersededTableLocked`,
  `appendSupersededTableLocked` and `supersededDeleteError{table, count, err}`
  hold a table only, and `cleanupOtherSupersededTablesLocked` /
  `cleanupAllSupersededTablesLocked` delete through `delListedLocked` ->
  `delRichRoute(prefix, table)`. Path: push at main/0, system-RIB route at
  metric 20 moves to 30 and the cleanup delete at 20 fails, then a withdraw
  deletes in main with no priority, removing the push and leaving the
  metric-20 route forwarding a withdrawn prefix with no record. A table move
  whose old-table delete fails reaches the same place.
- **Fix:** the superseded record keeps the slot priority of each failed
  cleanup delete (`supersededDeleteError` carries them;
  `appendSupersededTableLocked` takes the previous metric from `prevFwd`);
  `ownsSlotLocked`, the cleanup set, `delListedLocked`, the withdraw and the
  flush read it by slot; the moved-table delete in `cleanupAfterInstallLocked`
  names the previous slot. Add an AC and a unit test (refused cleanup beside a
  push at main/0, then a withdraw: the push stays, the superseded route goes at
  its own priority). No RFC-tagged test reads `supersededTables`.
- **Non-blocking:** the guarded IPv4 priority-0 delete has a race with a device
  flush between lookup and delete; name the push's gateway, device and encap in
  that delete, or list the race under Known limits.

Next: one revision for this, then the sixth judge.

## Choice waiting for Thomas

| Spec | Choice | Options | Recommended |
|------|--------|---------|-------------|
| `spec-fib-kernel-label-ownership` | C-2: after a restart, LDP's leftover push at priority 0 and the system RIB installing the prefix at metric 20 | (a) keep the push until LDP replays, departing from `core-design.md` "FIB Kernel"; (b) remove it as the page says, so the FEC's traffic leaves unlabeled until LDP's label mapping returns | (a): keeps restarts hitless for labeled traffic in a BGP-free core |

C-2 is recorded in the spec's `### Choices for Thomas`, outside the design
digest, so recording his answer owes no new judgment. Once the judge approves,
the main session presents the design whole (its Design presentation, every
departure with its before and after, and C-2) and hands Thomas the approval
line from his own terminal:
`./le spec design approve spec plan/release/fixes/spec-fib-kernel-label-ownership.md words "<what he said>"`.

## Decisions taken that a successor must not reopen

- C-1's old option (a) was ruled out by RFC 4724 Section 4.1; ownership is
  saved in the plugin state store and every store write is made from recorded
  state, never from an error return.
