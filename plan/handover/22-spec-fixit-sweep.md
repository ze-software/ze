# 22 -- spec-fixit sweep: state, decisions banked, next moves

Written 2026-07-27 at the end of a session whose goal was "implement all the
spec-fixit we have in the plan folder". That goal is NOT met and this records
exactly where it stands, so the next session starts from evidence rather than
re-deriving it.

## Rationale (what was agreed, and why)

- Thomas answered three blocking questions this session (see "Decisions banked").
  Two are acted on; one (BMP queue) is specified but not implemented.
- Defects were fixed at the source rather than recorded, per
  `ai/rules/no-parking.md`. Every fix is mutation-verified: disable the code
  under test, confirm the test goes red, revert.
- **A fixed defect is not a closed spec.** Closure additionally needs the
  Implementation Audit, Goal Validation, Review Gate artifact
  (`scripts/dev/review_gate.py`, enforced by `commit_helper.py`) and Pre-Commit
  Verification sections filled, then the two-commit sequence. None of the specs
  touched below have had that done.
- The BMP queue was deliberately NOT started: it is a memory-architecture change
  to a hot path, not a small edit. Reason is recorded in its spec and summarised
  below.

## Decisions banked from Thomas (2026-07-27)

| # | Question | Answer | Status |
|---|----------|--------|--------|
| 1 | May the RFC-tagged `TestOTCEgressNoStampProvider` be changed? | **Approved**, fix test + code | DONE (`c398e97f0`) |
| 2 | BMP policy when a collector wedges | **"do what bird does"** | RESEARCHED + SPECIFIED, not implemented |
| 3 | `bgp-session-fsm-lifecycle` AC-4 (FSM Error on second OPEN) | **Drop AC-4** | DONE, but see the correction below (`e929099ed`) |

**Correction that matters for trust in #3.** The option text put to Thomas said
"the current code already matches Section 8.2.2 here". That was asserted without
reading the producer and was FALSE: `handleOpen` had no state gate at all. AC-4's
prescribed remedy (FSM Error, code 5) was still wrong per RFC 4271 Section 8.2.2,
so dropping it was right, but the AC was pointing at a real defect.

## Commits from this session (all on `main`, unpushed)

| SHA | What |
|-----|------|
| `71f91c170` | IS-IS LSDB field-discipline comment corrected (it claimed 2 fields mutate post-publication; 4 do -- what forces an atomic is the CONJUNCTION of post-publication mutation AND off-lock read). Deleted `IsReceivedPurge`, an exported accessor with only test callers whose doc claimed "the engine reads this" |
| `c398e97f0` | `resolveSrcRole` recovers OTC src-role from config when meta lacks it, closing a leak guard that any caller without ingress metadata skipped |
| `e929099ed` | `handleOpen` state gate: refuses a second OPEN on an Established/OpenConfirm session with Cease + FSM transition |
| `f091c69f1` | `startSender` made idempotent (latent, not live -- see below) |
| `5381a1bb1` | BMP queue storage-shape constraint recorded in its spec |

## The near-miss worth knowing about

`e929099ed`'s FIRST version sent the NOTIFICATION and closed the socket but did
NOT fire the FSM event. It passed its own tests. Independent review caught that
this skipped the entire peer-closed cascade: `peer_run.go`'s
`from == fsm.StateEstablished` branch owns `stopBFDClient`, `raiseSessionDropped`
and `notifyPeerClosed`, and `notifyPeerClosed` is the sole producer of the
`SessionStateDown` that makes `adj_rib_in` clear `peerUp` and drop stored routes.
The "fix" would have left a dead peer marked UP with its routes retained and
replayed on reconnect -- arguably worse than the bug being fixed.

RFC 4271 Section 8.2.2 said so too, and the first version had quoted it
selectively: the Event 19 termination action list also mandates "deletes all
routes associated with this connection" and "changes its state to Idle".

Lesson for the next session: **run the independent review pass on every one of
these before closing.** Two of this session's own fixes were wrong in a way the
author could not see.

## Next moves, in priority order

### 1. BMP send queue (`plan/spec-fixit-bmp-sender-blocking-and-reload.md`)

Fully specified, decision banked, NOT implemented. Read the spec's Task section:
BIRD's design is recorded there with `proto/bmp/` citations (master `02d082a7`).

Summary: bounded queue counted in **BYTES** not messages; on overflow **reset the
session** (never drop, never block); full RIB re-dump on reconnect.

**The constraint that shapes the work** (`5381a1bb1`): the queue must be a
POOLED BYTE RING, not a `[][]byte`. `senderSession.scratch` is allocated once per
collector and its comment records that it "keeps the BGP-UPDATE -> BMP Route
Monitoring hot path allocation-free". A queue of `[]byte` elements must copy each
message out of `scratch` before the producer returns, which is a heap allocation
per Route Monitoring message on that path -- banned by
`ai/rules/buffer-first.md` and `ai/rules/memory-architecture.md`. BIRD avoids it
by packing into pooled pages rather than queueing message objects.

Needs: a design pass against `memory-architecture.md` (pool strategy by goroutine
shape: producer set + one drain goroutine per session) and an allocation
assertion in its tests.

Defect 2 of that spec (`startSender` idempotency) is DONE and the spec records
that Stage-2 configure is delivered once per plugin PROCESS startup, so the
doubling was latent.

### 2a. OTC closure: review came back FINDINGS, fixes landed, RE-REVIEW PENDING

The independent review required for closure of
`spec-fixit-otc-src-role-meta-fallback` returned **FINDINGS**, not clean, so the
spec is NOT closed and `review_gate.py record` was NOT run. What it found, all
of it now fixed in `276096afb`:

- **BLOCKER: the RFC9234-5-4 tag was vacuous.** `TestOTCEgressNoStampProvider`
  had a `dest` with no `LocalAS`, so the stamp was refused by the inner
  `localASN > 0` guard rather than by the destination-role gate the tag names.
  Deleting that gate entirely left the whole `role` package green while
  `ze-rfc-check` still reported the requirement proven in both polarities.
- **The sibling call site was never swept.** `getFilterConfig` is called twice,
  two lines apart; `c398e97f0` fixed the source read and left the destination
  read with the identical zero-value trap. `filterRemoteRoles` is written only
  from the OPEN Role capability, and `validateOpenRolePair` accepts a peer that
  sent none when `strict` is unset, so `destRemoteRole` was `""` and a route
  carrying OTC was forwarded to a configured Provider -- an RFC 9234 Section 5
  MUST NOT. `resolveDestRole` closes it.
- Two properties the code asserted only in comments (malformed value takes the
  fallback; meta beats config) had no gating test. Both do now.

**The judgement call to re-examine if you disagree:** `resolveDestRole` feeds
the RFC gates ONLY. Export-set matching still uses the capability value, because
`unknown` there is an operator-selected export target, not a missing answer.
Resolving it there too would silently retarget a documented knob.

**Round 2 also came back FINDINGS** (`d373d9f40`, `6766b15e4`). It found a
THIRD reader of `getFilterConfig` -- `OTCIngressFilter` (`otc.go:312`), 155
lines above the two that sit together -- still gated on the capability-only
role. For a peer that sent no Role capability that value is `""`, so the guard
at `otc.go:320` returned early and skipped ALL THREE Section 5 ingress MUSTs:
leak detection from a Customer/RS-Client, the Peer ASN mismatch check, and the
ingress stamp. Without the stamp the attribute never propagates, so a leak
cannot be caught hops away -- which is the whole point of OTC.

The reviewer proved it with a live experiment rather than by inference, and
showed the existing subtest for that branch (`config_but_no_remote_role`) PINS
the permissive outcome: its `{role: provider}` fixture complements to Customer,
and no ingress rule bites a Customer sending a route without OTC. It read as
coverage while proving nothing. `resolveDestRole` is now `resolvePeerRole` and
serves all three readers; `TestOTCIngressStampsWhenPeerSentNoRoleCapability`
gates it.

**Twice in a row the miss was the same rule** (`ai/rules/before-writing-code.md`
Sibling Call-Site Audit). Round 1 swept one of three call sites, round 2 swept
two of three. If you touch this plugin, enumerate every `getFilterConfig` caller
FIRST -- there are three, at `otc.go:312`, `:467` and `:468`.

**Still open, none of them blocking the OTC fix, all needing a home:**

| What | Where | Why it matters |
|------|-------|----------------|
| Suppressions from an INFERRED role log at `Debug` only and have no metric | `otc.go:481`, `:495` | after this change a peer whose role was never negotiated can have advertisements silently withdrawn, including on a config typo that was previously inert |
| Config keyed by peer NAME when no remote IP resolves, while all three readers look up by address | `config.go:193-198` | such a peer gets no config, so the RFC gates revert to permissive and the fallback is defeated |
| Capability-learned roles never cleared on session down | `role.go:61` is the only clearer | a peer that once advertised a role and reconnects without one keeps the stale value, which `resolvePeerRole` PREFERS over config |

The last two are now documented under "Known limits" in
`docs/architecture/meta/role.md` so they are at least visible to operators, but
documenting is not fixing.

Closure still needs a round-3 review verdict CLEAN against the current hashes,
then `review_gate.py record`, then commit B. Do not record an artifact against
`276096afb` -- the code has moved twice since.

**One thing needs you:** `tmp/delete-d3c58d3d.sh` removes
`internal/component/bgp/plugins/role/zz_reviewscratch_test.go`, untracked
scratch the first reviewer left behind. Deleting a `_test.go` is hook-gated, so
an agent cannot do it.

### 2. Closure work on specs whose defects are already fixed

`spec-fixit-otc-src-role-meta-fallback` and `spec-fixit-bgp-session-fsm-lifecycle`
have their code landed. They need audit + goal validation + review gate +
pre-commit verification, then the two-commit closure.
`spec-fixit-isis-lsdb-entry-race` is a skeleton (unfilled ACs) whose fix landed in
`7f3bfd338` plus `71f91c170`; closing it means filling the ACs retroactively.

### 3. ~~Known test gaps left open~~ CLOSED 2026-07-27 (`22156e6a7`)

Both gaps are closed, all assertions mutation-verified:

- `test/plugin/open-in-established.ci` -- the functional test. Placed in
  `test/plugin/` rather than the `test/parse/open-in-established.ci` the spec's
  user story named, because `test/parse/` is for config parsing
  (`ai/rules/testing.md` directory table) and this needs a scripted peer.
- `TestSecondOpenInOpenConfirmIsRefused` -- the OpenConfirm arm.

One finding worth carrying: the `.ci` observer waits on the **`session-drops`
counter**, not on peer state. The refused OPEN is injected the instant the
session establishes, so `established` lasts milliseconds -- an event-stream
observer missed it and a 250 ms poll missed it, both while the Cease was already
on the wire. If you write another test around a fast teardown, reach for a
monotonic counter rather than a transient state.

## Triage of the untouched specs (2026-07-27)

The goal "implement all the spec-fixit" is NOT met: 20 specs, 3 with code
landed, 1 closed as stale, 15 unstarted. What follows is the cheap part done, so
the next session spends its context on work rather than on re-deriving state.

**Verify before working. One of these was already fixed.**
`spec-fixit-sleeps-cli-harness` was closed on a single grep (`a95f0b7f8`) -- the
work had landed and nobody had told the spec. Two more were checked this
session and are genuinely LIVE:

| Spec | Keystone claim | Verified at |
|------|----------------|-------------|
| `forward-rail-initial-sync-ordering` | forwarding rail never consults `ShouldQueue` | 3 non-test callers, all injection-rail: `reactor_api_batch.go:111`, `:241`, `reactor_api_forward.go:103`. None in `forward_rs.go` / `forwardUpdateCore` |
| `stored-route-relay-hardening` | ADD-PATH replay still refused | `reactor_api_relay.go:328` returns `errRelayAddPath`, pinned by `reactor_api_relay_test.go:432` |
| `migrate-sleeps-infra`, `sleeps-qemu-bulk` | sleeps remain to convert | 101 `time.sleep(` across `test/**/*.ci`, against a ratchet ceiling of 101 (`test/.ci-sleep-baseline`, sum of `125 -11 -12 -1`) |

The sleeps number is worth pausing on: the count sits EXACTLY on the ceiling, so
there is zero headroom. Any new `.ci` sleep fails `check_ci_sleep_ratchet`
immediately, and every conversion must append its own signed delta line rather
than edit the integer. Whoever works those two specs should expect the gate to
bite on unrelated test work in the meantime.

**The remaining set is not homogeneous, and sizing it by file length misleads.**
`forward-rail-initial-sync-ordering` is the smallest file (6.7K) but is a
hot-path reactor ordering change: `ai/rules/testing.md` makes
`make ze-race-reactor` mandatory for it. `stored-route-relay-hardening` (19K) is
explicitly an INVESTIGATION spec -- its own Task section records that the
previous round's guessing at that layer "produces worse outcomes than the bug
being fixed", having added a guard that failed an entire replay over a correctly
suppressed route. Neither is a quick win; pick them with a full context budget.

**What this session proved about pace.** Every one of the five defects fixed
here needed independent review, and three of the five had a further defect found
only in that review -- twice the same missed sibling call site. Budget for the
review round; it is not optional overhead on this codebase.

## Environment warnings for whoever picks this up

- **This working tree is shared with another active session.** At handover it had
  uncommitted work in `internal/component/doctor/`, `internal/core/diagnostic/`,
  `internal/test/{cli,peer,runner}/` and several `.ci` files, plus new untracked
  files. Check `git status` before assuming a red is yours.
- `ai/DOCS-TO-CODE.md` is left MODIFIED in the tree. Its only content change is
  two files belonging to that other session (`internal/test/peer/listen_ttl.go`,
  `internal/test/runner/needs_path.go`); their commit should carry it. Do not
  fold it into an unrelated commit.
- **Not mine, currently red:** the other session's new `doctor-config-bgp-peer`
  check fails `ze-test ui 104` (`doctor-bgp-listen`) and `105` (`doctor-bgp-md5`)
  -- two `.ci` files they have not updated.
- `make ze-verify` cannot go green while that session is mid-edit. The commits
  above used the Known-Red scope-to-changed path with attribution.

## Verification command

    make ze-verify

Delete this file in the commit that completes its last item.
