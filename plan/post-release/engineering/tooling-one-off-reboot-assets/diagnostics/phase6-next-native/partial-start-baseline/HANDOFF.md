# Controlled partial-start baseline: MAIN handoff

## Current revision: startup abort owner retained

The fresh source-only continuation preserved the refused draft and review in
`refused-v2/`, then repaired only this controlled baseline's abort envelope.
No formatter, compiler, test, native process, Go tool or script ran. A fresh
actual-delta safety review is required before MAIN executes this version.
The older refusal applies to the snapshot, not as approval of this revision.

The controller now supplies its original absolute ten-second safety deadline
to both surrogate and helper. A helper scheduled late cannot acquire a fresh
ten-second startup window. Its one control reader starts before hashing;
EOF, expiry and an invalid instruction set abort under the same mutex that
guards child Start and installation of its actual owner. Hashing checks abort
between bounded 32 KiB reads, retaining the original 128 MiB per-input bound.
An abort prevents a later fork or joins the already installed child through
its existing sole Wait. No abort is reported settled before that Wait ends.

The supervisor no longer discards `controller.native`. The surrogate and
supervisor retain their actual Wait bridge through settlement, using the
unchanged helper/surrogate/supervisor hard test bounds of 40/45/50 seconds.
Their joins have no new shorter give-up timer. The helper's constructor defer
joins its control worker and any child before returning. A timely safety
ownership grant ends the abort reader; it does not become a post-grant child
cleanup watchdog that could mask the intended old failure.

`nativeCatalogProbe.finish`, all protected oracle assertions, the independent
subreaper/adopted Wait path, and both refused native roots remain unchanged.
This is not the product P1/P2 repair. In particular, the old full native roots
remain refused and the five-minute foreign-sentinel defect is still open.
Only the exact controlled root below is proposed for execution after review.

The remaining operational preconditions are the same: Linux pidfs/SCM_RIGHTS,
the reviewed native prctl ABI, local regular compile inputs within the hash
bound, and no runtime result yet. No claim is made that a kernel task which
cannot complete its own Wait has been settled; the retained owner must not
be replaced with a success or a guessed PID/group signal.

`ABORT-REVISION.md` records the five-part diagnosis and source checks.

## Original baseline boundary

Source only, November 6. The author ran no formatter, hasher, compiler, test,
native process or script. Canonical source is unchanged and still compares
equal to `before/catalog_cancel_test.go.txt`. The refused native observer and
independent refusal are retained under `refused-v1/`; `after/` still matches
that source exactly. No P1/P2 repair has been implemented.

## Exact files

- `catalog_cancel_test.go.txt`: copied refused source, a test-only guarded
  startup seam, `TestNativeCatalogPartialHandoffCleanup` and its controlled
  surrogate-parent helper. The actual `nativeCatalogProbe.finish` remains
  unchanged. The ordinary native helper path remains refused.
- `../overlay-partial-start-baseline.json`: one existing-file mapping to this
  baseline. Do not use the ordinary native overlay for the quick discriminator.
- `DIAGNOSES.md`: five-part P1/P2 diagnoses and the source repair still owed.

## Precondition and oracle boundary

Before the helper may publish the gate packet, the test-only seam transfers
both the actual child's Start pidfd and the helper's self pidfd to a separate
safety endpoint. The existing pidfs binder verifies both. The test acknowledges
that independent ownership only after both are bound. A pre-handoff failure
closes the safety control, waits for the helper's own deferred child join,
and refuses any force if ownership remains unknown.

The child's ordinary packet is sent to an independent observer endpoint. The
actual controller does not receive or bind it. Once every safety handle and
the child's TERM-ignoring ready socket are proven, the test calls the actual
broken controller `finish`. It then requires child death, socket EOF, helper
death, an acknowledgement emitted only after actual child Wait, and a positive
foreign-sentinel exchange. Final safety cleanup runs afterward, including when
an oracle or precondition fails. It cannot turn a failure into PASS.

The proof runs in an isolated supervisor test process which becomes a Linux
child subreaper before any fixture child starts. After the oracle, verified
handles stop any old survivor; the supervisor waits for its adopted gate/child
through those exact handles. A process which its original owner already reaped
may answer ECHILD only alongside a kernel handle proving it is gone. Each Wait
has one worker which retains and finally releases its handle; a bounded miss
does not race Release against that worker. No wildcard wait, PID/group sweep
or outside process is used.

The tiny test-only prctl bridge uses the retained Linux SYS_PRCTL values 157
(amd64) and 167 (arm64), and PR_SET_CHILD_SUBREAPER 0x24; unsupported native
architectures fail this precondition. It changes only the isolated supervisor,
so no parent-session subreaper state needs restoration. This follows the
existing isolated-supervisor idea in
`internal/le/test/qemu/run_descendant_linux_test.go`; it does not execute that
QEMU fixture, its tree staging, or any QEMU action. MAIN's reviewer must check
the selected ABI definitions and adopted-child Wait path before execution.

The surrogate parent is a test process, not Go or le. The real compiler binary
is hashed as an input by the existing helper, but never invoked: the test never
sends the release command. No native le build, catalog or VM/image run occurs.
All original product five-second/five-minute/minute bounds remain unchanged.
This test proves only the fixture's partial-start boundary.

## MAIN next action

Independently review this new delta before any execution, especially the early
startup-abort guard and safety acknowledgement boundary. MAIN may then format
ONLY this baseline file, compute hashes and run this exact root serially:

```sh
CLAUDE_CODE_SESSION_ID=01a1a907-3278-7971-98dc-dbc41d4a54a5 CGO_ENABLED=1 \
  bin/le-tooling-resume-01a1a907/le job run \
  label phase6-partial-handoff-old-baseline quiet command \
  go test -race -tags "$tooling_test_tags" -p=1 \
  -overlay /home/thomas/Code/github.com/ze-software/ze/main/tmp/session/2026-11-06-080923/scratch/phase6-next-native/overlay-partial-start-baseline.json \
  -run '^TestNativeCatalogPartialHandoffCleanup$' -count=1 -timeout=1m -v \
  ./internal/le/doc/yangcontract
```

Do not execute the full package or either refused native root. The preserved
old package contains a pre-existing `-a` carrier outside this scoped command.

The intended old semantic RED is a still-live child/open child socket and
missing child-join acknowledgement after actual broken `finish`, while the
foreign sentinel still answers. A compile, setup, pidfs, guard or transfer
failure is not the intended RED. Read the named root and exact failure, then
immediately retain all `partial-*/` evidence outside managed scratch. The last
oracle event is emitted before final independent safety cleanup.

Only after MAIN observes the valid RED may a fresh continuation implement
P1's controller/child abort-and-join protocol and P2's case-rooted sentinel
lifetime. That continuation must preserve this probe and prove both repaired
startup handoff and abort/EOF handling throughout startup, before retrying the
native roots. No new product timeout is authorized. About a dozen additional
source-only calls plus the isolated-reaping guard were used in this continuation;
total context is approaching
the planned package boundary, so use a fresh author for the actual fix.
