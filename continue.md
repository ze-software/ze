# Continue: the spec-closing sweep (session close-them-all, 2026-09-07/08)

Stopped on request mid-way. Nothing is half-committed: the interrupted closure was
killed before it wrote anything, and `git status` shows no edit of its spec or of
`plan/.citation-baseline`.

## 1. Decisions

### Still waiting on Thomas

| Decision | State |
|---|---|
| Run `plan/immediate/spec-verification-debt-clearing.md`? | `ready`, unstarted. It is the only thing that opens the push gate |
| Read `plan/spec-liveness-event-tears-down-bgp-peer.md` | `design`, 13 ACs, written this session, never reviewed by him |
| Event naming in that spec | Proceeding as `liveness` / `peer-down` / `keepalive-expired`. His own phrasing was "tcp failure with IP". Recorded as Q-1; renaming before implementation costs three constants and one YANG grouping name |

### Answered, and what each answer authorized

| Answer | Words | What it authorized |
|---|---|---|
| 2026-09-07 | "ok for the tests" | The six RFC-tagged IS-IS tests, `c.SendHello()` gaining a level argument |
| 2026-09-08 | "the 2 ci are approved" | `test/l2tp/rfc2661-emitted-control-shape.ci` and `rfc/discrimination/rfc2661.json`. Landed at `db8d8ac81`, approval row in `test/rfc-changed/5e2d7854.md` |
| 2026-09-08 | "TestMD5PeersForListener approved" | The `s4.Port` to `s4.LocalPort` setup-field change. **Given, but unrecordable: see below** |
| 2026-09-08 | "Demote 42 unheld specs to ready" | Done at `9e17145f0`. In-progress went 52 to 8 |

### The approval he gave that the tooling cannot store

All seven RFC-tagged tests are now genuinely approved, and `test/rfc-changed/1bfe298a.md`
is corrected to say so. It is still UNTRACKED, and every route to landing it fails:

- Committing it refuses, because it is session `1bfe298a`'s shard and any current session
  is not that session (`ForeignShardProblems`, called at `internal/le/commit/prepare.go:221`).
  No override keyword exists.
- Committing under `session 1bfe298a` passes that check and then destroys the rows:
  `rfcChangeProblems` returns an empty keep set when the commit changes no tag carrier, and
  a `.md` is not a carrier, so `PruneLanded(root, rfcShard, nil)` rewrites the file and
  drops every row (`internal/le/commit/prepare.go:258`, `internal/le/testweakened/shard.go:192`).
- Renaming it into the current session's shard hits the same prune.

So an owner approval given AFTER the change landed has no home. An `rfc-changed` row only
survives a commit that changes the tagged test in the same commit. **This is the same shape
of problem as `spec-verification-debt-clearing` and belongs folded into it.** Do not force
the file in; landing it empty is worse than leaving it corrected and untracked.

**The 2026-09-06 approval conflict is resolved by those answers, not by the old
record.** Untracked `test/rfc-changed/1bfe298a.md` asserted that Thomas approved all
seven tagged tests on 2026-09-06, while the committed rows in
`plan/verification-debt/1bfe298a.md` for the same commits said he had never been asked.
He has now approved all seven himself, on the dates above, and that file is corrected to
record the approvals actually given. It stays UNTRACKED because no route lands it with its
rows intact, not because the correction is unfinished.

Note for whoever picks this up: no command can clear a row whose gate is
`owner approval for an RFC-tagged test change`. `clearDebtWith`
(`internal/le/commit/actions.go:290`) diverts that gate name and
`independent critical review` into an unrunnable set before a runner is chosen, and
`passed` is only ever filled from the runnable ones. So those ledger rows stay open
even though the approval now exists, and hand-editing them to `cleared` is not the
repair. `spec-verification-debt-clearing` is.

## 2. Where the sweep stands

Of 63 in-progress specs, 3 were held by live sessions and 60 were held by nobody.
Triage classified all 60. **All 15 closable specs are now closed, and in-progress is 3** —
exactly the three a live session holds.

Closed here: `isis-per-level-hello-timers`, `l2tp-shaper-upload-rate-is-not-enforced`,
`ldp-keepalive-time-proposal`, `peer-local-port-overwritten-by-remote-port`,
`plugin-respawn-leaf-restarts-nothing`, `router-advertisement`,
`mgmt-version-header-suppress`, `vpp-isolated-cpus`,
`rfcgate-2-deferred-nonunit-evidence-backfill`, `cli-show-bgp-answer-shapes`,
`image-server-listen-interface-drops-entries`, `qemu-targets-boot-the-shipped-kernel`,
`plugin-declares-answer-shape`, `bgp-pcap-decode`,
`commit-end-reports-what-each-peer-took`.

Two of the fifteen carried a proof gap, and neither was accepted. `bgp-pcap-decode`
claimed AC-2 on Ze's own reader, built in the same commit as the writer, so it agreed by
construction; `tcpdump` turned out to be installed here and dissected a Ze-written capture
independently, so the criterion is genuinely proven. `commit-end-reports-what-each-peer-took`
had a committed `.ci` nobody had run; running it went RED over a real defect and the fix is
in HEAD.

**The closure record, for reference:**

| Spec | Note |
|---|---|
| `spec-plugin-declares-answer-shape` | CLOSED 2026-09-08. Its gate found a plugin's declared column and address-field names were length-bounded and nothing else, while the completer offers each as a `\| display` candidate and the renderers write it as a header — so an ESC in a declared name reached the operator's terminal as an ANSI sequence and a tab broke the completion format |
| `spec-bgp-pcap-decode` | CLOSED 2026-09-08. The tree no longer holds the file, so the name is written bare. The AC-2 proof gap is gone rather than accepted: `tcpdump` is installed on this host and dissected a ze-written capture as BGP. One item is homed, `plan/spec-bgp-pcap-decode-real-capture-fixture.md`, for a fixture taken off a real network |
| `spec-commit-end-reports-what-each-peer-took` | CLOSED 2026-09-08. The tree no longer holds the file, so the name is written bare. The proof gap is gone rather than accepted: `test/plugin/commit-end-per-peer-report.ci` was RUN, went RED at HEAD over a real defect (the error sentence naming each refused peer reached no caller, because `(*Server).dispatchCommandResponse` discards the whole response when a handler returns an error beside it), and is GREEN after the fix |
| `spec-image-server-listen-interface-drops-entries` | CLOSED 2026-09-08. The tree no longer holds the file, so the name is written bare |
| `spec-qemu-targets-boot-the-shipped-kernel` | CLOSED 2026-09-08. The tree no longer holds the file, so the name is written bare. All 7 ACs re-verified against the producers that survived the `eae282592` migration |

**The 45 that are NOT closable.** 42 are partly built and 3 were never started
(`finish-l2tp`, `mpls-9-rsvp-te-one-to-one-backup`, `bgp-deferred-confederation-otc`).
For those, `in-progress` is a status lie, not finished work: nobody holds them.
The `more` session's point, which is right and shaped this sweep, is that the
truthful status for an unheld spec with outstanding ACs is `ready`, keeping the
Phase field. It is what makes the WIP cap of 12 mean something again.

**Done at `9e17145f0`.** 44 specs demoted, in-progress 52 to 8, ready 21 to 62. The three
that had not reached `ready` went where their own text put them: `finish-l2tp` to
`skeleton`, `mpls-9-rsvp-te-one-to-one-backup` and `bgp-deferred-confederation-otc` to
`design`. Two specs whose prose claimed "UNCOMMITTED" over landed code were corrected in
the same commit and now name the SHA.

That left 8 at `in-progress`; the 5 awaiting closure have since closed, so **3 remain**,
all held by live sessions:
`spec-verify-scope-5-suite-coverage-map` (main-43), `spec-ipsec-rfc9190` (session 9cd3fea7)
and `spec-test-peer-open-inherits-zes-identity` (session ebcdd865). The last two were found
during the sweep, not before it, by their per-spec state files being written minutes
earlier. **Re-ask the peers before touching any spec: two of the three holders named at the
start of this session had closed and deleted their specs by the end of it.**

Two findings from that sweep, recorded as journal rows rather than fixed:

1. **8 of the 44 specs are refused by their own write-time validator at HEAD content**
   (`hookValidateSpec`), six for design documents their code declares and the spec never
   names. Proven to predate the status edit by restoring `in-progress` on one and
   re-running the hook. Any session editing those 8 is blocked by someone else's
   authoring gap.
2. **The `plan/` shell-write guard fails open two ways.** `governedSed`
   (`internal/le/hookruntime/bash.go:34`) requires the literal `plan/` in the same command
   segment, so `while read f; do sed -i ... "$f"; done` rewrote 44 specs unrefused while a
   `cat >` heredoc creating one file was blocked. And `governedRuntime`
   (`bash.go:37`) is `\b(?:perl|ruby)\b`, so a `python3` heredoc writing into `plan/`
   is not caught at all.

Four specs carry status text their own git history contradicts, claiming
"UNCOMMITTED" or "in flight" over code that landed: `commit-stages-in-a-private-index`,
`ledger-shards-per-commit-session`, `test-parse-ci-parser-refuses-an-unread-directive`,
`interop-image-copies-a-prebuilt-ze`.

## 3. Constraints a restarting session must know

- **Close ONE spec at a time.** The skill requires it and the reason is real: closures
  commit, and the index is shared with four other live sessions.
- **`./le` does NOT rebuild on every invocation.** The normal path calls
  `warn_when_stale` (`le`), which only prints "bin/le is older than committed sources"
  when a COMMITTED source is newer than the binary; `build_le` runs on `--update` or
  when the binary is missing. So an edit under `internal/le/` becomes live for another
  session when that session next updates, not the instant you save it. This session
  believed the opposite and sequenced work around it: the ledger dedup was held behind
  a running closure, and closures behind the dedup, for a hazard that was not there.
  The real care owed is still real, just smaller: a committed non-compiling
  `internal/le/` breaks every session that updates after it.
- **A closure costs 210k to 300k tokens and 30 to 90 minutes.**
- **`./le verify worktree` pins its subject at launch** (`internal/le/verify/lifecycle.go:150`
  resolves an empty commit to `HEAD`, rev-parsed at `:350`) and runs over an hour. Its
  verdict therefore always describes an ancestor of the tree it was asked about, and
  every commit made during it lands "not FRESH-green" by construction. Do not start one
  during a closure; carry the debt row instead.
- **Run Go tests with the tag set from `feature-gates.txt`.** A bare `go test` produces
  phantom reds here, and one closure agent was fooled by exactly that.
- **The agent-spawn hook refuses prompts matching a `ze-*` skill trigger.** The words
  `BLOCKER`, `audit ... change` and `review ... code` in an agent prompt get the call
  blocked (`internal/le/hookruntime/agent.go:19`). Name the skill or reword.

Sessions holding specs when this stopped, do not close these: `more` holds
`fixit-ci-runner-cannot-test-stdin`, `catalog` holds `daemon-backed-command-catalog`,
`main-43` holds `verify-scope-5-suite-coverage-map` (its phase 3 is blocked on a
wording decision from Thomas).

## 4. Product defects found and fixed by the gates, worth knowing

- A RADIUS-supplied subscriber rate overflowed on multiply: `ParseRateBps("18446744074gbit")`
  returned 290 Mbit/s and no error, so a subscriber was shaped at a rate nobody
  configured (`internal/component/traffic/config.go:189`).
- VPP worker cores were drawn from the kernel's isolated-CPU list without asking whether
  those CPUs are online, so a hotplugged-out CPU could take a worker after a passing
  `ze config validate` (`internal/component/vpp/cpuset.go`).
- The RA doctor check fed a logical interface name to a `/proc` path keyed by the OS
  device, so an aliased interface silently returned "unknown" and the warning never fired
  (`internal/plugins/iface/ra/doctor.go`, fixed with `osDeviceOf`).
- `restartHandshake` delivered the post-startup callback before startup completed, so a
  plugin exiting between two startup phases got a replacement that ran `OnAllPluginsReady`
  twice, the first time before the registries were frozen
  (`internal/component/plugin/server/restart.go`).
- `request shutdown` could not stop a daemon during startup: the shutdown function was
  wired hundreds of milliseconds after a plugin could dispatch, and a BGP reactor in the
  config masked it (`cmd/ze/hub/main.go`).
- `generateOpen` in the test peer patched only the 2-octet AS and left Ze's own AS in the
  RFC 6793 AS4 capability, so Ze correctly answered Bad Peer AS. Fixed by
  `patchAS4Capability` (`internal/test/peer/peer.go:890`). **89 `test/plugin/*.ci` files
  carry that shape, so this is the likely mechanism behind the standing ~100-case plugin
  red. Worth re-running that suite first thing.**

## 5. Ledger and gate findings

- The verification-debt ledger was deduplicated at `de31341fd`: 3587 rows became 1270,
  with the distinct `(shard, gate, reason, status)` count unchanged at 1270. A row now
  keeps its first covered commit and appends `(+N more)`; the later subjects are
  recoverable from `git log -- plan/verification-debt/<session>.md`. Currently 1081 open,
  192 cleared. **This did not open the push gate.** `refusePushWithDebt` still refuses on
  any open row.
- `./le rfc check` reads only the tags a commit ADDS against `HEAD^`, so three RFC 2661
  requirements bound at `functional/verify` sat "proven" by a prose table with no
  discrimination record and the gate never asked. Fixed for rfc2661; the ratchet's blind
  spot is not.
- `27a41cb32` deleted the flat `test/weakened.md` without migrating it into
  `test/weakened/`, and `acceptedRows` (`internal/le/testweakened/audit.go`) reads only the
  directory. That is why `./le commit audit` re-flags eight already-accepted weakenings.
- RFC 4861 and RFC 8106 have no `rfc/short/` summary at all, so 151 MUSTs are unenrolled
  and invisible to the ledger. Skeleton at `plan/pre-release/spec-rfc4861-rfc8106-enrolment.md`.
- LDP cannot emit a Notification message at all: `wire.go` has the type constant and no
  encoder, so every fatal LDP error closes the TCP connection with nothing on the wire.
  A peer proposing KeepAlive Time 0 is accepted, sets hold time to 0, and dies on the next
  read deadline reported as a keepalive expiry. Both are inside the liveness spec's scope.

## 6. THE RED LIST: what to focus on next session

**This is the work. Everything else in this file is context.** The verification-debt
ledger holds 1080 open rows, and not one of them clears until the whole verification runs
green, so these reds ARE the push gate.

### The sweep that measured it

Thomas ran `debt-clear` as a 45-piece sweep on the other machine (macOS) on 2026-09-08,
00:16 to 01:40 UTC. Result: **30 pieces green, 15 red, 0 rows cleared, 1080 still open.**
Its own result file is machine-local at
`tmp/session/2026-09-08-78638422-.../scratch/sweep-result-20260908-011648.txt`; the numbers
below are copied here because that path does not exist on this box.

    red this sweep, re-run them: 1 3 4 5 7 10 12 22 23 24 33 40 41 44 45
    proven: 2 6 8 9 11 13 14 15 16 17 18 19 20 21 25 26 27 28 29 30 31 32 34 35 36 37 38 39 42 43

**Piece 1 is the one that matters:** it carries lint (60 findings) and the full unit run
(**36 failing packages**). The other fourteen are functional and test-health stages. 36
failing packages at a committed SHA is the product being red, not scaffolding noise.

### Read the sweep with this caveat, or you will chase a past

It ran over `6b7077807`, which is an **ancestor of this machine's HEAD by 51 commits**. The
two checkouts also disagree about origin: the Mac had `origin/main` at `6b7077807`, this box
has it at `e9a8155a3`, thirteen commits later. Settle that first, or the next sweep measures
a different past again — the branch is Thomas's to move.

Several commits since plausibly move those reds, though NONE has been measured against a
sweep piece and no such claim should be made without running it:

| Landed since the sweep | Plausibly touches |
|---|---|
| `patchAS4Capability` in the test peer | the standing ~100-case plugin red; 89 `.ci` share the shape |
| `TestReclaimRunsOnlyForAVMBackedRuntime` pin (`20c836767`) | one unit failure, deterministic on any host without `colima` |
| `ParseRateBps` overflow guard, `restartHandshake` ordering, the control-character refusal | their own packages' tests |

**So the first action is the same command at the current HEAD.** It re-measures the fifteen,
tests resumption (it should skip the thirty proven), and does it against a tree where a third
of this session's fixes exist.

### One defect in the sweep itself

An abandoned worktree, `r20`, was left at **0.8 GB** because it held five *untracked* files,
so the sweep does not reclaim its own space when a piece is killed. Every killed piece leaks
another 0.8 GB. The rule it applies is the right rule in the wrong place: untracked output
inside a throwaway verify worktree is generated, which is the one category
`ai/rules/never-destroy-work.md` explicitly permits acting on.

### Reds seen from this machine, independent of the sweep

`TestNativeImplementationFixture` in `internal/le/rfc` and `internal/le/rules` fails on
hand-pinned digest drift, a class with 22 rows in `plan/journal/hardcoded-count-in-test.md`.
`./le spec citation` fails on 8 dangling refs in `spec-remove-takes-the-working-tree-copy.md`
and `spec-verification-debt-clearing.md`. `./le doc check verify` is red across the BGP
command surface and `../gh-pages/`. `exabgp api-reload` fails intermittently on an unchanged
tree. The `ui` suite fails cases including `le-ste-answers`, which hangs past 340s.
`./le verify lint run` was last seen red on `internal/component/plugin/leaf_test.go` (nilnil,
from `8788a29f8`) plus darwin-flavor findings.

Two entries that WERE in this list are now gone and are recorded here so nobody re-chases
them: `internal/component/bgp/plugins/rib` built again once its session committed, and
`ExtractRemovePrivateASOps` was reported for having no cross-package NON-TEST caller while
having three in-package ones, so landing it changed nothing.

## 6b. Graceful Restart: its own handover

`plan/handoff-graceful-restart-sender-facts.md` carries the deepest finding of this
session and is the one document to read before touching Graceful Restart. In short:
**Ze advertises GR for no address family, Ze does not retain routes when a peer restarts,
and the 32 `.ci` that should have caught either pass over dead code** — measured with a
control, not inferred. PATHS-LIMIT enforcement is dead for a third, unrelated reason.
Three journal rows, no fixes, and the question the fix must settle first is which families
Ze may honestly claim, which depends on what its FIB does on restart.

## 7. The tidy-up after `open` and `ipsec` (2026-09-08)

Thomas asked this session to liaise with the `open` and `ipsec` sessions, finish what they
left incomplete, and commit as it went.

**`open` is done.** It committed at `c7aa4e63e` (33 files) and declined a review round
from this session, correctly: the fifth round is the last a session may spend on its own
(`ai/rules/planning.md`), and a sixth is Thomas's decision recorded through
`./le spec session review record --owner-authorised`. Handing the round to another context
would produce a sixth round with no such decision behind it. Its spec stays `in-progress`
deliberately, with the reason in `tmp/session/.closure-ack-test-peer-open-inherits-zes-identity`,
and `plan/learned/008-mirror-asserts-sameness.md` stays uncommitted for the same reason.
It handed over one spec, `spec-test-peer-open-mirrors-five-more-sender-facts`, closed on
2026-09-08 and so no longer on disk:
five sender facts the test peer still asserts about Ze by mirroring them, of which the
Graceful Restart restart time is the one that makes a real test vacuous.

**`ipsec` is green and blocked on one signature.** It holds 45 files covering RFC 9190
Section 5.4-2 through 5.4-5 and `RFC5216-5.4-2`, and cannot commit because the change moves
seven RFC 7296-tagged tests. The whole diff in all seven is one added `nil`, from
`maintainSA` gaining a `*serverCertRecheck` parameter. It needs Thomas's verbatim words to
quote in `test/rfc-changed/b5d2a9bd.md`, because an author writing their own approval row
is a forgery. **Two NAI tests are RED at HEAD until that commit lands**: `anonymousNAI` is
in HEAD (`internal/core/eap/nai.go`) and its call site is not, because the wiring hunk sits
in `internal/core/eap/peer.go` beside that session's unlanded OCSP work.

**Work this session did while waiting, all committed:**

- 16 specs that their own write-time validator refused now pass (`2d4f31fcc`, `af45d1735`).
  Ten had named the pre-commit gate by its old spelling, five never named a design document
  their code declares, and one was refused because two Go files declared it at `plan/`
  after it moved to `plan/immediate/`.
- Seven more gained the entry point their validator asked for, each read at the producer
  (`f40c40aad`). Refusals are down from 56 to 33, and the 33 are skeletons missing whole
  sections — a question about whether a skeleton should be held to that at all.
- `hookValidateSpec` no longer fails open (`908d22186`). It is a PostToolUse hook, so the
  file exists when it runs; a failed read returned 0, reporting success having checked
  nothing. That is the same class as the comment three lines above it records.
- A red test at HEAD fixed (`20c836767`): `TestReclaimRunsOnlyForAVMBackedRuntime` never
  pinned `commandAvailable`, so it returned early on any host without `colima`.
- Eight groups of abandoned work landed (`3f25545ed`, `df5b6c25a`, `76ce2d39a`, `a8092e624`,
  `23aff0e3b`, `7b5532f68`, `6cf33dee0`, `9817f3be8`, `e308381fc`), judged one at a time
  against HEAD rather than by hunting for owners.
- `ai/RFC-REQUIREMENTS.md` caught up with landed work (`de0fbda9c`).

**What is left uncommitted, and why each stays:**

| File | Why |
|---|---|
| the 42 `ipsec` files | waiting on Thomas's signature |
| `docs/features.md` | names a `pki certificate … ocsp-response` leaf HEAD does not have; belongs to that commit |
| `test/weakened/ad601f6a.md` | the gate refuses a foreign ledger shard: carrying it publishes another session's record under your subject |
| `plan/learned/008-mirror-asserts-sameness.md` | `open` holds it deliberately |
| `test/rfc-changed/1bfe298a.md` | the seven approvals, correctly recorded, with no route to land that preserves the rows |
| `sdk` | a stray 15-byte file containing `=== iface- ===`, the tail of a truncated shell redirect. Deleting it needs Thomas's word |
