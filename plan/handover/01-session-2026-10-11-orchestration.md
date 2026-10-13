# Handover: orchestration session 5620b26f, ended 2026-10-11

The owner closed the laptop. Every agent stopped cleanly: work is committed,
or written up as a handoff in a per-spec state file. Nothing is running.

Paths:
- **Scratch:** `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/`. It is gitignored and exists on the Mac only.
- **State files:** in `.../state/`, named `session-state-<spec-stem>-5620b26f-603e-4d57-826d-6ef92b7fcd64.md`.

Read the state file for a spec before resuming it. Each one ends with the last agent's handoff.

## Waiting on the owner (Linux host `ssh claude`)

1. `./le setup docker-kernel install`, then the `confirm <release>` line it prints, then `sudo reboot`, then `./le setup docker-kernel check`. The AppArmor profile `ze-lab-net-sysctl` is already installed (2026-10-11).
2. After that, the appliance runs are owed. Their exact commands are in the Work Not Done section of `plan/pre-release/spec-appliance-ships-ze-kernel.md`:
   - cold amd64 `ze appliance build`, plus the readback of the MPLS options and the 128 required options
   - amd64 `mpls-boot-test`
   - `vpp-hugepages-test`
   - `gokrazy-l2tp-ppp-test`
3. Also owed on Linux:
   - the docker-hosts real-host ACs and the VRRP lab
   - least-privilege AC-8 and AC-9, the evidence release-candidate, and the rib-fib render
   - the showcase renders and validateShowcaseLab. Ask before a full 18-demo re-render (45-60 min).

## Specs in flight

| Spec | State | Next step |
|---|---|---|
| `plan/pre-release/spec-appliance-ships-ze-kernel.md` | Review loop CLEAN at round 11; gate recorded (`./le spec review check` OK). arm64 boot on cached key 34cba457 PASS | Linux runs above, scoped lint, `./le verify worktree`, then the closure commits A and B |
| `spec-bgp-live-receive-binding-and-bfd-keep-session` (claimed, in-progress) | Phase 1 done: 10 entry tests (7 red for the right reason, 2 pin restarts). Phase 2 researched, no code | Phase 2. Seam decision taken by the main thread: add a small unexported reactor interface for bind events, satisfied by `*bgpserver.EventDispatcher`, so the ordering tests can observe up / graph publish / ROUTE-REFRESH. Also cover with unit tests AC-13's "nothing advertised keeps one connection" and AC-8's "no other process receives down/up". Producer notes and order are in the state file |
| Bracket leaf-list work, recorded in `plan/spec-terminal-demo-showcase.md` | Round-5 blockers fixed in 6b95932a8b. `TestLeafListSequencesAgree` is the permanent fuzzer | Round-5 ISSUE 2 (parked in scratch `brk5/issue2-*.txt`; needs NOTE 4 first), ISSUE 3-5, NOTES 1, 3, 4, 5, and the syntax.md wording "`nop` is a `set` line marked inactive". Then gates (lint, race, editor, parse, audit), then round 6 (owner authorised). Findings: scratch `review-bracket-r5.md` |
| `plan/immediate/spec-bgp-unowned-rfc-rows.md`, `plan/spec-bgp-fsm-delayopen-and-automatic-start.md`, `plan/spec-bgp-epe-peer-segments-and-srv6-peernode.md` | Design written, split per the owner (4ab33322b8) | Independent design review of each, then implement the immediate one first |
| rfc-verdict children (`plan/pre-release/spec-rfc-verdict-fix-*.md`) | See below | See below |

### rfc-verdict children

- **bfd:** all 5 interop scenarios PASS (a1d121e6a8). Owes an independent /ze-review, then closure.
- **routing:**
  - isis-p2p-frr and ospf-ldp-sync-frr PASS.
  - Owed: the 8 `interop-rsvpte` scenarios, and two new scenarios that are not written yet, `egress-implicit-null-penultimate-pop` and `transit-resv-state-expires`.
  - Then /ze-review.
- **ike-eap:** 5 of 18 strongSwan scenarios PASS (43a8da6929). The 13 still owed are listed in the spec's Work Not Done, with their command.
- **access:**
  - The review found 6 ISSUEs; the ledger ones are fixed (a0a101e5ad, a832af8548, 1acc7eef41).
  - Owed: interop scenarios for the L2TP, PPPoE and PPP, and RADIUS/CoA fixes. See scratch `review-access/review.md`, ISSUE-2 to ISSUE-4.
  - Then re-review.
- **ospf:**
  - AC-C2 is open: 118 tagged units have no discrimination record. The list is in scratch `ospf-ac-c2-unrecorded.tsv`.
  - AC-C7 is open: RFC8362-2-1 was never re-judged.
  - 12 interop scenarios are owed, two of them on Linux only.
- **bgp:** owes /ze-review and closure. The two SHIFTED RFC4456 verdicts are resealed (917da161f8).
- **parent:** owes /ze-review and `./le verify worktree` once the children close. RFC5882-4.4-1 moved to keep-session AC-24.

## Owner decisions taken this session (all recorded in their specs)

- **Keep-session:**
  - OQ-1 A: dynamic peers keep their session.
  - OQ-2 A: the whole swap set applies to dynamic peers.
  - RFC5882-4.4-1 is folded into keep-session.
- **BGP rows:**
  - "10 give it a spec", then "Split into three".
  - MUP: "Keep it, log a warning".
  - RFC2545: "Fix it".
  - DelayOpen, Event 3 and EPE are in scope. Type-13 BSID is out. RFC9514 rows ride with EPE.
- **Appliance:**
  - One build limit: "2 hours".
  - Rounds 6 to 11 authorised.
- **Bracket:**
  - Round 6 authorised.
  - `nop` is a set line marked inactive; the hierarchical format uses `inactive:`, the Junos way.
- **RFC ledger:**
  - RFC4090 "Hand-written break".
  - RFC8907 "Drop the tag, re-judge".
  - "Allow one reseal" (used in 917da161f8; do not reseal again without a new authorisation).
  - "Approve" for the route-refresh spelling changes in RFC-tagged tests.

## Known defects found, journaled and not fixed

- **Two QEMU lab guests on the Mac collide on SSH port 2222.** `freeRunSSHPort` probes `tcp` and should probe `tcp4` (`plan/journal/parallel-copies-collide-on-a-deterministic-port.md`). Until it is fixed, run one docker-lab guest at a time.
- **colima cannot run Ze labs.** There is no `ze-kernel-probe` AppArmor profile, and its 6.8 kernel lacks MPLS. Use `./le test qemu docker-lab lab '<suite>' env '<VAR>=<scenario>'`.
- **Installer downloads over TLS always fail.** `defaultHTTPGet` cancels before the body is read. This is off the shipped path (`plan/journal/lifetime-ends-before-its-last-reader.md`).
- **A verify of a candidate tears down dynamic peers.** `plan/journal/check-mode-mutates-the-tree.md`; the keep-session Phase 3 fixes it.

## Gates not yet run or not yet explained

- Reload suite (2026-10-11, scratch `reload.log`): `config-apply-ordering-coarse-root` and `config-apply-ordering-mixed-rollback` are red with no cause found. Compare them against the parent of 1f2dd73052 before blaming anyone. `rsvpte-reload` is red on macOS: "interface: no backend configured and no OS default available".
- The plugin and editor functional suites were not run after 1f2dd73052.
- Scoped lint over bgp/reactor reports 53 findings, all in older reactor commits, e.g. fa86db31ed.
- `./le verify worktree` has not been green since 2026-10-05.
