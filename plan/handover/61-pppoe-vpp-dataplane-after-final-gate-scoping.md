# Handover 61: PPPoE VPP data plane, after final-gate scoping

Written 2026-11-22 on the server (`claude`), main checkout. Read this instead
of handover 60: its items 1 and 2 landed, and its item 3 is now scoped into
the steps below. Its traps are carried. Nothing is pushed.

The owning spec is `plan/post-release/features/spec-pppoe-vpp-dataplane.md`
(in-progress, phase 2/7). Claim it before editing:
`./le spec claim spec-pppoe-vpp-dataplane.md`. Its Checkpoint table ("Final
gates"), Resume checkpoint, "Real-VPP product lab — 2026-11-22" and
"Implementation designs — 2026-11-22 (awaiting owner decisions)" carry today's
evidence and both designs in full.

## Landed on 2026-11-22 (this session)

| Commit | What |
|--------|------|
| `2c9fbd7770` | A real PPPoE server session proves the unit-bound IPv6 carrier and the native release (two cuts) |
| `38267f5e0c` | RADIUS accounting counts the original PPP unit, never a name |
| `0a92e78161` | A real session's Framed-IPv6-Route is owned by its original unit (AC-24, one cut) |
| `45abb5326e` | Spec records the server wiring, unit-bound accounting and route receipt; accounting spec amended to count the unit |
| `ef5b6423fd` | Spec test tables name the tests that exist and those owed |
| `b6388ee211` | PADS follows the session's creation, and PADT its deletion (five server tests, two cuts) |
| `c879124c37` | Spec records the IPv4 product lab pass and the PADS/PADT order; PPPoE operator clear row |
| this handover's commit | The `product-framed` pass, both implementation designs, the unpoliced-subscriber row, this handover |
| `400c99ef00` | Spec records the loss and policer designs; handover 61 |
| the 2026-11-23 recording commit | The owner's VPP rules and D-1 decision, the lifecycle spec skeleton, the shaping research, three journal rows, the digest correction, this update |
| `179f4d4f03`, `8cf5201adb` | The owner's lifecycle rules and D-1, then his loss refinement with the two recovery forms |
| the review-recording commit | The lifecycle review applied: his systemd rule (R-9, VPP page row 9), proposals marked, fifteen choices, the questions below |

## Baseline now

- Real-VPP IPv4 product lab `./le test deployment vpp-pppoe product`: 4 of 4
  (pppd client, namespace, VLAN, 100 subscribers),
  `job-vpp-product-fc5ac019.log`.
- `./le test deployment vpp-pppoe product-framed`: 1 of 1,
  `job-vpp-framed-52302217.log`.
- The three server native tests (carrier, teardown, framed route) pass in
  Ze's guest on runtime tree `2b23af13`.
- The `authradius` package passes.

Logs are under `tmp/session/2026-11-22-203554/scratch/`.

## Owner rules of 2026-11-23, and where they are recorded

Thomas decided how Ze manages VPP, and D-1 = option C:

1. Ze refuses to run VPP unmanaged; with an external VPP the operator accepts
   that Ze may terminate and restart it.
2. Ze knows how to manage and restart an external VPP: a systemd unit, or a
   direct start of the binary as with the embedded runtime.
3. After a failure: alive, forwarding, reconnected and state readable means
   continue from where it is; possibly crashed or stopped forwarding means
   reflect it in Ze and update IGP and BGP peers.

Refined the same day (lifecycle spec R-4 to R-7, the two pages' "Design
decisions", the PPPoE spec's D-1 section):

- One timer for the alive test and the loss declaration; a probe missed inside
  it is not a loss.
- "Recovered" has two forms. (1) Inside the timer: reconnect to the same
  pidfd-proven process and a full readback match; continue, no kill, no
  withdrawal, PPPoE sessions kept (option C, scoped to this form). (2) After
  the declaration: kill through the retained pidfd, exit proof, reconfigure as
  at boot; PPPoE drains per OD-2 and subscribers redial.
- Blackholing during recovery is accepted; routes are withdrawn only when
  neither form succeeds. No F bit on re-open. The recovery deadline and the
  crash-loop limit (N restarts in a window, then withdrawal and a `ze doctor`
  report) are the main session's proposals P-1 and P-2, not his words.
- Corrected by the review: "Ze is the only restarter, Restart=no" (the former
  R-8) was proposal P-3, and his systemd words reverse it: "systemd
  configuration: we can not be sure that an OS update will not change the
  settings so we should detect the settings and adapt". The lifecycle spec
  records it as R-9 and step 2a; the VPP page gained row 9.
- Value still open for Thomas: the single timer (today a 1 s tick and a 5 s
  probe timeout).
- The first rows written for rule (c) (adopt "when alive, forwarding and
  readable", and "update peers" on a possible crash) are still in the pages'
  tables. A row changes only with Thomas: removing or rewording them needs his
  `./le commit create ... decision-change "<his words>"` from his terminal.

| Where | What |
|-------|------|
| `docs/research/vpp-deployment-reference.md`, "Design decisions" | Rules 1 to 3, then four rows of the refinement (one timer; kill, exit proof, restart; the two recovery forms; blackhole accepted, withdrawal only when neither form succeeds), each naming the earlier row it narrows. Row 6 also carries "Ze the only restarter (Restart=no)", which was a proposal: row 9 (his systemd words, detect and adapt) supersedes that sentence and its forbids clause, and correcting row 6 needs his `decision-change`. A note in "Socket Transport" says they replace "Ze neither kills that process" and are not built |
| `docs/guide/vpp.md`, "Running against an externally supervised VPP" | The same note |
| `docs/architecture/l2tp/bng-5-pppoe.md`, "Design decisions" | D-1 resolved: kill and restart when Ze cannot adopt, option C readback when it adopts; a refinement row scoping adoption to form (1) inside the timer and every declared loss to a kill, restart and OD-2 drain; a note on the "VPP session ownership" invariant |
| `plan/post-release/features/spec-vpp-failure-lifecycle.md` (new skeleton) | His five quotes, the rules (R-1 to R-7, R-9), findings 1 to 11, the design draft with the systemd step 2a, fifteen choices, the pages made wrong, the review section; owns two journal rows |
| `plan/post-release/features/spec-pppoe-vpp-dataplane.md`, "Owner decisions of 2026-11-23: D-1 resolved" and "VPP subscriber shaping research — 2026-11-23" | F-1 to F-5, changes 1 and 2, readback table, E-1 to E-4 under the rules, owed tests; the shaping summary and recommendation; the ng row corrected |
| `ai/digests/vpp-dataplane.md` | Corrected against today's code (connect path, disconnected emitter, external API loss) |

## Independent design review of the lifecycle spec (2026-11-23)

A strongest-tier review ran on the lifecycle skeleton. Verdict: not
approvable as recorded, with three blocking findings, eleven important ones
and seven notes. Each is applied in the spec or recorded as an open choice;
its section "Independent design review" lists them. The blocking ones: the
"only restarter" rule was a proposal his systemd words reverse; the
detect-and-adapt mechanism was missing; the recovery deadline, the crash-loop
limit and "Ze stops restarting" are not in his words.

## Owner questions now open

| # | Question | Where |
|---|----------|-------|
| 1 | Which reading of the one timer: (a) it bounds the alive test, the declaration and the reconnect; (b) a failed alive test is the declaration, and "reconnect within a time limit" is a second value | Lifecycle choice 10 (R-4) |
| 2 | Whether he adopts the recovery deadline P-1 and the crash-loop limit P-2, and their values (and with P-2, whether a unit at its start limit is reset) | Choice 9 |
| 3 | The single timer's value | Choice 1 (P-4) |
| 4 | After a withdrawal: keep retrying, or wait for an operator command | Choice 11 |
| 5 | The bound on form (1)'s readback: its own bound with mutations fenced, or inside the timer | Choice 12 |
| 6 | Ze's own BFD, dead and hold timers during recovery: run as the RFCs say, or held | Choice 13 |
| 7 | A foreign process on the API socket: may Ze kill it | Choice 14 |
| 8 | The lifecycle spec's bucket: `release/fixes`, a split, or post-release (not moved) | Choice 15 |
| 9 | Starting an inactive unit at boot; the stub's method; event names | Choices 4, 7, 8 |
| 10 | `decision-change` from his terminal: VPP page row 6 (its "only restarter" sentence and forbids clause, superseded by row 9), VPP page rows 3 and 4 (rule (c), narrowed), rows 7 and 8 (P-1 and P-2 in the recorder's wording), and `bng-5-pppoe.md` rows 1 and 2 (narrowed by row 3) | Lifecycle "Pages this work makes wrong" |

## Remaining, in order

| # | Work | First action |
|---|------|--------------|
| 1 | Owner choices: the lifecycle questions in "Owner questions now open" below, then E-1 (unknown-outcome creations on form 1), C-1 (where a configured VPP rate lives), and the shaping plugin licence route: a Ze-written plugin as an Apache-2.0 diff in the VPP tree or an AGPL-3.0 plugin directory, or carry osvbng's GPL-3.0+ plugin | Present them to Thomas from the two specs |
| 2 | The lifecycle spec's design | Fresh session, `/ze-spec plan/post-release/features/spec-vpp-failure-lifecycle.md`, after question 8 settles its bucket; read VPP's and systemd's source for the ASSUMED facts first |
| 3 | The VPP shaping spec | Fresh session, `/ze-spec`; references `~/Code/github.com/veesix-networks/osvbng-vpp` `plugins/osvbng_qos_sched` and VyOS with accel-ppp-ng; the recommendation is in the PPPoE spec's shaping section |
| 4 | PPPoE OD-2 and the F-1 and F-3 fixes | After the lifecycle spec's design: OD-2's drain runs on every declared loss (form 2), never inside the timer; form (1) keeps the sessions through option C |
| 5 | The per-session VPP policer wiring | The Filter-Id/RADIUS path needs no decision; the configured rate waits on C-1. This fixes the open defect: VPP subscribers forward unpoliced today |
| 6 | Lab discrimination (revert, rebuild, red) for `product` and `product-framed` | Spec "Real-VPP product lab" |
| 7 | VPP accounting and idle detection (AC-17) | After `spec-pppoe-subscribers-produce-no-accounting-or-telemetry` |
| 8 | AC-8 (IA_PD) | After `spec-l2tp-ipv6-subscriber` wires the prefix allocator |
| 9 | Scale on a known host | Thomas |

Reference clones on disk: `~/Code/github.com/veesix-networks/osvbng` (753e934),
`~/Code/github.com/veesix-networks/osvbng-vpp`, `~/Code/github.com/vyos/vyos-1x`
(db29bdc89), `~/Code/github.com/accel-ppp/accel-ppp-ng` (7e4ec88).

## Waiting on Thomas

- The push of everything above.
- `./le setup docker kernel install` and a reboot, so the host runs a kernel
  with the 0013b fix and the netdev option.
- The Docker store on the server: Docker kernel builds need 40 GB free.
- The choices in item 1 above.

## Traps

Carried from handover 60:

- A guest command resolves its image from the current kernel recipe. To run
  an older kernel, pass both `kernel` and its matching `image` from
  `~/.cache/ze/docker-host/amd64/<key>/docker-host.img`.
- A guest test binary must be built with `CGO_ENABLED=0` (no race): the guest
  has no libc.
- `ppp_open` needs CAP_NET_ADMIN in the opener's user namespace; a test that
  opens `/dev/ppp` must call `userns.Enter` first.
- Scoped lint refuses to plan while any session edits a package the plan
  depends on; rerun when the edits stop.
- A fixture split that moves assertions into a helper trips the weakening
  gate; it needs a ze-test-judge approval even when nothing is lost.

Met on 2026-11-22:

- `auth-method pap` alone still offers CHAP first. For a PAP-only test peer,
  set `authentication-allow [pap]`.
- A PADT cannot prove route withdrawal: deleting the unit flushes its routes.
  Withdraw through an IPv6CP close instead.
- A packet socket bound to a down device returns ENETDOWN on its first read.
- Overlay cuts (`go test -overlay`) avoid touching the shared tree.
- `le test unit pkg` needs the `path` keyword.
- `le test l2tp` needs `-a`.
- Run a net-admin `.ci` through `./le test qemu ci test <path>`.
- `rfc-approval` on `./le commit create` is only for changes already landed;
  pending judge ledgers are picked up automatically.
