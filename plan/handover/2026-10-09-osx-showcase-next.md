# Next on macOS: the showcase video, then nothing else first

Handover for work not yet started. Written 2026-10-09 on Linux, for the owner's
next session on macOS.

**Owner instruction (2026-10-09): the next session picks these two specs, in
this order, before any other work.**

| Order | Spec | Status | Why this order |
|-------|------|--------|----------------|
| 1 | `plan/immediate/spec-appliance-kernel-vpn-modules.md` | skeleton | the showcase IPsec chapter needs esp4 and xfrm_interface, and the appliance kernel must carry them too |
| 2 | `plan/spec-terminal-demo-showcase.md` | skeleton | the video; it `Depends` on spec 1 |

Each spec holds its own owner decisions, storyboard, work plan and risks. Start
each one with `./le spec claim spec <path>` and run it through `/ze-spec`, then
`/ze-implement`. One spec per session (`.claude/rules/session-start.md`).

## RATIONALE (verify this matches what we agreed)

- The showcase is one long terminal session for the site hero and a 60 to 90 s
  README cut, showing RPKI, IRR, BFD, OSPF, IPsec, VRRP tracking, eBPF traffic
  counting and commit-confirmed, each configured live in the SSH editor and then
  demonstrated. Decisions are in the showcase spec, "Owner decisions".
- A feature that does not enable on a live commit is a Ze defect to fix, never a
  tape workaround.
- WireGuard is NOT a chapter until `plan/spec-wireguard-runtime-proof.md` lands;
  that spec comes after these two. PPP and L2TP are skipped.

## What macOS changes

- The demo renderer runs Linux containers (`--privileged`, kernel modules,
  network namespaces). Rendering on a Mac goes through the Docker VM, whose
  kernel decides whether esp4 and xfrm_interface exist: check that first, it is
  showcase risk R-2.
- A demo daemon that never starts under host load was seen twice on 2026-10-09:
  `plan/journal/startup-wait-expires-under-load.md`. A long session meets it
  more often; showcase risk R-3 says to fix it if it recurs.

## Also waiting on macOS, not part of this order

- `plan/immediate/spec-osx-interface-listing-check.md`: the stdlib interface
  listing (commit `52eb8b4f77`) was only compiled for darwin, never run. The
  owner did not place it in the order above; ask before starting it ahead of
  spec 1.

## In flight: validated construction, Phase 1

`plan/spec-validated-construction-and-state-types.md` is `in-progress` and runs
by batches. Its resume file is `plan/handover/12-validated-construction-phase-1.md`:
read it first, then the spec's Phase 1 section. The owner placed the showcase
specs first; ask before resuming this one ahead of them.

| Batch | State on 2026-10-09 |
|-------|---------------------|
| 1 (T3, `ArgDef` constructors) | Done, `9cbd3485ca` |
| 2a, 2b (YANG Go imports, strict `DefaultLoader`) | Done, `7d69d6a669`, `5eaa197f32` |
| 3 (T1, `yang.Resolved`), 3b (guard splits) | Done, `180df07922`, `93ee8363ce` |
| 4a (C-T2a, every route validates) | Done, `62842be4c7` |
| 4b (C-T2b, `StreamingHandler` takes `ValidatedArgs`) | Done, `eaff5d644b` |
| 4c (C-T2c, `LocalDataHandler` takes `ValidatedArgs`) | Done, `e4b25d9bc2`. The data-handler registry moved into `command` rather than the value into an internal package (handover 12, batch 4c) |
| 4d (`LocalHandler`) | Not started. Same move as 4c: `LocalHandler` and its registry into `command` (handover 12 note) |
| 4e (`pluginserver.Handler`, ~340 handlers in 69 dirs, one commit) | Not started. Owner waiver of `.claude/rules/foreign-files.md` for this batch only, limits in handover 12 |
| Close | `/ze-review`, then the two closure commits |

Owed across all batches:

- `./le verify worktree`: never run over these commits.
- `./le --update` before `test/ui/cli-argument-refused-plugin-route.ci` runs, because it needs the rebuilt `le` test plugin.
- `internal/component/cli` needs `-timeout` above 10m under `-race`.

Follow-ups outside Phase 1, each already recorded:

- Remove the goyang `replace` line in `go.mod` once https://github.com/openconfig/goyang/pull/317 is released. The CLA is signed.
- Tag `goyang_enum_numbering_test.go` as an RFC7950-9.6.4.2-1 positive so that verdict can become enforced.
- RFC 7950 7.2.2-1, the submodule include check, is a recorded gap.
- Grammar R1: five root commands (`explain`, `generate wireguard keypair`, `skills`, `support`, `validate config`) need an owner grammar decision (`plan/journal/gate-red-where-nothing-blocks-on-it.md`).
- `go mod vendor` would revert the hand-patched netlink (`f0d9c75df4`): patch `vendor/` by hand.

## Open: closing the link-flap test spec

`plan/spec-fixit-flap-test-cannot-build-its-own-stimulus.md` is `in-progress`.
Its "Evidence 2026-10-09" section (`a7ca55d1bf`) records one green run of
`iface-link-flap-during-commit` on an amd64 KVM guest against `98ee050ee9`, and
a forced red that proves the overlap guard. Resume file:
`tmp/session/2026-10-09-12d06ccf-2460-42c7-a707-30bb0a427796/state/session-state-spec-fixit-flap-test-cannot-build-its-own-stimulus-12d06ccf-2460-42c7-a707-30bb0a427796.md`.
Work it in this session, not a new agent. Remaining:

- The zero-drops assertion has no red. A 4 KiB monitor buffer failed on the
  coalescing assertion instead, and a 4 KiB counter socket stayed green. Likely
  fix: read `netlinkDrops08` every round in `ifaceLinkFlap08`
  (`internal/test/fixture/plugin_fixture_08_flap.go`), before the coalescing
  check, then force the red again.
- More than one green run, for the load-dependent drop concern.
- Add the Deliverables, Security and Documentation checklists the spec lacks.
  Then `/ze-close`: `/ze-review`, `./le spec review record` and `check`,
  commits A and B.
- Commit A repoints the spec-path citation in
  `plan/handover-verification-debt-4526b941.md` (its "resolved but not closed"
  bullet).
- `./le verify worktree` has never run over this work.
- A forced red runs only in a private `git clone --depth 1` of HEAD, never in
  the shared tree. A `git archive` export fails, because `le test qemu run`
  fingerprints HEAD. The run recipe is in the resume file.

## Delete this handover

When spec 2 is claimed, this file has done its job: remove it in that session's
first commit.
