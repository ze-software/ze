# Handover: fib-dummy and fib-kernel label ownership specs

Written 2026-11-24 by the session that designed both specs, at its context
handoff threshold. No session holds a claim on either spec. Remove this file
through `./le commit create ... remove` once both specs are approved and set to
`ready`.

## What Thomas asked

Transcribed: "can you create a spec for a dummy backend which does nothing for
the fib". Asked what it is for, he chose both purposes: a control-plane-only box
(route reflector or route server that must never program forwarding) and
testing and benchmarking the RIB path without kernel cost, written to
first-release standard at `spec-fib-dummy`.

## The specs

| Spec | Status | Last commit | Judge state |
|------|--------|-------------|-------------|
| `spec-fib-dummy` | ready | e38da3f4aa (choices and `ready` transition committed after approval) | Approved by judge at digest 6e51c0e84de3 and approved by Thomas (C-1a, C-2a, C-3a, C-4a, C-5a, C-6b) on 2026-11-24 |
| `plan/release/fixes/spec-fib-kernel-label-ownership.md` | design | ab580c27e5 | Four judges refused; a fifth judge was judging digest dd2f9a383fcc when this was written |
| `plan/release/fixes/spec-bgp-graceful-restart-rfc-defects.md` | design (amended) | 4f5789f7cb | D7 recorded, not designed or judged |

The label-ownership spec was split out of the dummy spec because fib-kernel's
ownership of MPLS label routes is defective without the dummy and needed a
design of its own. D7 (an IGP install during BGP selection deferral deletes the
retained BGP route, against RFC 4724 Section 4.1) was found by a label-ownership
judge and is owned by the GR spec.

## In flight when this was written

1. A ze-work agent was revising `spec-fib-dummy` for the tenth refusal. The
   main-session decisions it was given:
   - D1: EPE's unacquired replay answers the opening writer alone, as one
     `ApplyReplay` batch tagged with the request's token holding the
     `ActionRemoveLabelSource` entry then every `desiredLocked` add, built by
     giving `publishLocked`'s body a choice of apply function. `reloadConfig`
     is not reordered. Check LDP, OSPF SR and RSVP-TE for the same live-batch
     exposure during the reload window.
   - D2: bound AC-22 (2) and Before-and-after item 15 to a box whose kernel
     holds no leftover label entry, and add it to the outcomes Depends ties to
     the label-ownership spec.
   - A-13/AC-23: the 1.5R bound holds for a neighbour following RFC 2205
     Section 3.7's SHOULD with no refresh lost, counted from fib-dummy's stop.
   If `git status` shows the dummy spec modified, that revision landed in the
   working tree: commit it (prose, `no-test`), then spawn the eleventh judge.
   If not, redo those three items from the tenth judge record.
2. A fifth ze-design-judge was judging `spec-fib-kernel-label-ownership.md` at
   dd2f9a383fcc. Read its record in
   `tmp/spec-design/judged/spec-fib-kernel-label-ownership.md`: on approve,
   `./le spec design check` prints `ready to present`; on refuse, revise and
   judge again.

## Update: the tenth-refusal revision landed

The revision in item 1 above is committed (digest 1b71ffcdb3c5). One deviation
from the brief was accepted: EPE's unacquired replay answer sends the cleanup as
its own tagged `ApplyReplay` batch and the adds only once it is accepted,
because both writers' `handleMPLSEntry` keep applying a batch after a refused
entry, so a single batch would install adds behind a refused cleanup.

Before the eleventh judge, one gap needs a design decision (the main session's,
not Thomas's, unless it changes what he decided):

- **Live label applies during the reload window.** Between the new writer's
  start (`autoLoadForNewConfigPaths`) and the displaced writer's stop
  (`stopCollectedProcesses`, `reload.go`), a live label apply (a new LDP FEC, an
  OSPF SR change, an RSVP-TE refresh) reaches both writers and is refused
  through fib-dummy. LDP and OSPF SR keep it wanted but re-apply only when a
  writer next opens, so it stays out of fib-kernel until re-signalled.
- Options: (a) after the reload stops a displaced `ProgramsFIB` writer, the
  remaining writer issues one more targeted label replay request; (b) reorder
  the swap in `reloadConfig` (rejected so far: compensation ordering needs its
  own design); (c) record it as a limitation.
- Recommended: (a). It reuses the replay request the spec already adds (AC-21),
  needs no reordering, and closes the window for every label source at once.
  Write it as an AC with a test where a live apply lands in the window.

## Update: the label-ownership fifth judge refused (dd2f9a383fcc)

Record: `tmp/spec-design/judged/spec-fib-kernel-label-ownership.md`. The
fourth judge's defect is resolved; one new defect, with the fix the judge gave:

- **The superseded record has no slot priority.** `supersededTables`
  (`map[string][]uint32`), `recordSupersededTableLocked`,
  `appendSupersededTableLocked` and `supersededDeleteError{table, count, err}`
  hold a table only, and `cleanupOtherSupersededTablesLocked` /
  `cleanupAllSupersededTablesLocked` delete through `delListedLocked` →
  `delRichRoute(prefix, table)`. Path: push at main/0, system-RIB route at
  metric 20 moves to 30 and the cleanup delete at 20 fails, then a withdraw
  deletes in main with no priority, removing the push and leaving the
  metric-20 route forwarding a withdrawn prefix with no record. A table move
  whose old-table delete fails reaches the same place.
- **Fix:** the superseded record keeps the slot priority of each failed
  cleanup delete (`supersededDeleteError` carries them; `appendSupersededTableLocked`
  takes the previous metric from `prevFwd`); `ownsSlotLocked`, the cleanup set,
  `delListedLocked`, the withdraw and the flush read it by slot; the moved-table
  delete in `cleanupAfterInstallLocked` names the previous slot. Add an AC and a
  unit test (refused cleanup beside a push at main/0, then a withdraw: the push
  stays, the superseded route goes at its own priority). No RFC-tagged test
  reads `supersededTables`.
- **Non-blocking:** the guarded IPv4 priority-0 delete has a race with a device
  flush between lookup and delete; name the push's gateway, device and encap in
  that delete, or list the race under Known limits.

Next: one revision for this, then the sixth judge.

## Choices waiting for Thomas

| Spec | Choice | Options | Recommended |
|------|--------|---------|-------------|
| spec-fib-dummy | C-6: a reload replaces `bus { endpoint k { internal fib-kernel; } }` with `fib { dummy { } }` | (a) refuse with "restart ze", departing from `hub-architecture.md` "What the daemon does at start" and the hub-engine door; (b) stop fib-kernel (respecting `flush-on-stop`) and start the dummy, departing only from the `doctor-and-health-checks.md` "read only at boot" note | (b): keeps the owner rule that only external plugins need a restart |
| spec-fib-kernel-label-ownership | C-2: after a restart, LDP's leftover push at priority 0 and the system RIB installing the prefix at metric 20 | (a) keep the push until LDP replays, departing from `core-design.md` "FIB Kernel"; (b) remove it as the page says, so the FEC's traffic leaves unlabeled until LDP's label mapping returns | (a): keeps restarts hitless for labeled traffic in a BGP-free core |

Both choices are recorded in each spec's `### Choices for Thomas`, outside the
design digest, so recording his answer owes no new judgment. Once a spec's
judge approves, the main session presents the design whole (its Design
presentation, every departure with its before and after, both choices) and
hands Thomas the approval line from his own terminal:
`./le spec design approve spec <path> words "<what he said>"`.

## Decisions taken in this session that a successor must not reopen

- The dummy is its own plugin, `fib { dummy { } }`, with no leaf. It declares
  ProgramsFIB with no data plane, departing from the `plugin-system.md`
  Registration fields wording (presented as `departs`). fib-p4 is untouched.
- `fib dummy` beside another FIB writer is refused by a rule inside the guard
  spec's `evaluateDataPlaneVerdict`, reading a no-plane mark on the
  registration.
- MPLS label entries are refused by the dummy and reported through each
  source's existing `doctor-fib-mpls-no-writer` check.
- RSVP-TE deliberately does not replay refused adds: a refused install rejects
  the reservation and releases its label (`reservation.go`); its forwarding
  returns at the neighbour's next Resv or Path refresh.
- The route-path benchmark lives in the dummy spec (AC-19), not in the
  perf-next umbrella, whose children are all closed.
- Label-ownership C-1's old option (a) was ruled out by RFC 4724 Section 4.1;
  ownership is saved in the plugin state store and every store write is made
  from recorded state, never from an error return.

## Other specs amended in this session (committed)

- `spec-plugin-restart-bounded-and-configuration-true.md`: item 1 and the
  2026-11-12 retained-entry journal row split with spec-fib-dummy AC-17.
- `spec-doctor-coverage-10-forwarding.md`: AC-111,
  `doctor-fib-kernel-routes-no-writer`.
- `spec-perf-next-0-umbrella.md`: records the existing entry-point tests of
  its closed children.

## Tooling gap noticed

The design digest (`DesignDigest`, `docs/contributing/spec-workflow.md`) hashes
only Task, Acceptance Criteria, Design documents and Design presentation. Both
specs keep their deciding mechanics in Data Flow, so an edit there after
approval would owe no new judgment.
