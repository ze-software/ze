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
| the answers-recording commit | His answers A-1 to A-7: VPP page rows 10 to 15, `bng-5-pppoe.md` row 4, the lifecycle rules and design draft, the PPPoE refinements, his `decision-change` step below |

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
- Answered later the same day (see "Owner questions" below): two limits in
  place of the single timer, adoption on the presence of data with
  background reconciliation, the deadlines adopted, retrying after a
  withdrawal, Ze's own timers running, a foreign process reported and not
  refused, and the spec kept post-release.
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

## Owner questions (answered 2026-11-23)

Thomas answered them on 2026-11-23. The answers are A-1 to A-7 in the
lifecycle spec ("His answers to the open choices"), rules R-4 to R-7 and
R-10 to R-12, VPP page rows 10 to 15, `bng-5-pppoe.md` row 4, and the PPPoE
spec's "Owner refinements" block.

| # | Question | Answer |
|---|----------|--------|
| 1 | Which reading of the one timer | Answered: (b). A failed alive test is the declaration; a second limit bounds the reconnect-and-verify attempt (A-1) |
| 2 | Adopting the recovery deadline P-1 and the crash-loop limit P-2 | Answered: both adopted (A-2). Default values are proposed at the lifecycle design review. A unit at its start limit is reset before the next retry (recorder, confirm at design) |
| 3 | The timer values | Answered with question 2: the alive-test time and the reconnect-and-verify limit are proposed at the design review |
| 4 | After a withdrawal | Answered: keep retrying with a growing backoff; interfaces and routes return by themselves; `ze doctor` reports throughout (A-3) |
| 5 | The bound on form (1)'s readback | Answered: none. The presence of configured data adopts; reconciliation runs in the background and repairs (A-4, his words in the spec) |
| 6 | Ze's own BFD, dead and hold timers during recovery | Answered: they run as the RFCs say (A-5) |
| 7 | A foreign process on the API socket | Answered: reported, never refused or killed; Ze keeps working with it (A-6, his words in the spec) |
| 8 | The lifecycle spec's bucket | Answered: post-release, not moved (A-7) |
| 9 | Starting an inactive unit at boot; the stub's method; event names | Not put to him this round: choices 4, 7 and 8 are proposed at the lifecycle design review |
| 10 | The `decision-change` that removes the superseded rows | His step, below |

## Thomas's step: the decision-change

Rows 10 to 15 of the VPP page and row 4 of `bng-5-pppoe.md` restate
everything the older rows hold that still stands, so the older rows can go
whole. Only Thomas can commit that, from his own terminal. The working-tree
edits are made in the same sitting, just before he runs the command, so no
other session's commit of these two pages meets them first:

1. `docs/research/vpp-deployment-reference.md`, "Design decisions": delete
   rows 3 to 8, the rows that open "After a failure, when the VPP process is
   alive", "When VPP could have crashed", "One timer", "As soon as a loss is
   declared", "Recovered has exactly two forms" and "Blackholing during the
   restart is accepted". In the systemd row, delete the sentence that opens
   "This supersedes the second sentence". In the rows that open "Two
   limits", "Form (1) adopts", "A recovery deadline" and "After a
   withdrawal", delete the closing sentence that opens "This supersedes",
   "This narrows" or "This adopts": each names a row that is gone.
2. `docs/architecture/l2tp/bng-5-pppoe.md`, "Design decisions": delete rows 1
   to 3 (the D-1 row, "When Ze adopts the VPP", "Adoption happens only inside
   the single VPP loss timer"). In the remaining row, delete the closing
   sentence that opens "This supersedes the row".
3. From his terminal, with his own words in the last argument (the quoted
   text is only a suggestion):

   `./le commit create replace subject "docs: drop the VPP lifecycle rows that later rows supersede" file docs/research/vpp-deployment-reference.md file docs/architecture/l2tp/bng-5-pppoe.md decision-change "remove the VPP lifecycle rows of 2026-11-23 that the later rows of the same day supersede"`

   then run the script at the `script=` path it prints. The command refuses a
   pipe, needs his terminal on stdin and stdout, and writes one
   `Decision-changed:` trailer per page.

## Remaining, in order

| # | Work | First action |
|---|------|--------------|
| 1 | Owner choices: the lifecycle questions are answered ("Owner questions" above); his `decision-change` step remains. Still open: E-1 (unknown-outcome creations on form 1), C-1 (where a configured VPP rate lives), and the shaping plugin licence route: a Ze-written plugin as an Apache-2.0 diff in the VPP tree or an AGPL-3.0 plugin directory, or carry osvbng's GPL-3.0+ plugin | Present them to Thomas from the two specs |
| 2 | The lifecycle spec's design | Fresh session, `/ze-spec plan/post-release/features/spec-vpp-failure-lifecycle.md` (post-release, not moved); read VPP's and systemd's source for the ASSUMED facts first; propose the values of the two limits, the deadline and the crash-loop limit, and choices 4, 7 and 8 at its design review |
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
