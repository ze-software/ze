# Native catalog build observer: source handoff

November 6, 2026. Diagnostic source only. Execution remains held for MAIN's
independent actual-delta review, formatting and precondition checks. No Go,
gopls, formatter, test, build, shell script or native process was executed by
this author. Reading source and `cmp` were the only shell operations.

## Files and source boundary

- `before/catalog_cancel_test.go.txt`: exact live source snapshot. The final
  source-only `cmp` against `internal/le/doc/yangcontract/catalog_cancel_test.go`
  succeeded. Original functions and assertions remain in the after snapshot.
- `after/catalog_cancel_test.go.txt`: original source plus imports and the
  additive native observer. Main entries are `TestNativeCatalogBuildCompletes`
  and `TestNativeCatalogBuildCancellation`; the two helper entries are filtered
  child-process roles, not independent native acceptance tests.
- `overlay.json`: replaces only the existing catalog cancellation test file in
  MAIN. There is no production source replacement or producer cut here.
- `proposal.md`: earlier design. This handoff describes the actual source;
  claims that the observer is not authored are now historical.

The canonical tree was not edited. This source is deliberately unformatted.
MAIN must calculate hashes after formatting the selected after file, retain the
exact delta and compare the canonical before snapshot again before execution.

## What the observer asserts

The native child runs the checkout's `le --name <unique-name> doc yang-contract
doc-drift | json`, with `|` as one argv element. At the shipped compiler gate,
the observer checks that `/proc/<entry>/exe` is that private named binary. It
records its actual executable digest, compiler and Go executable digests,
`go.mod`/feature-gates digests, actual compiler/go argv and effective GOFLAGS.

Only actual shipped `cmd/ze` compilation is gated. Version queries, other
packages/tools and the le compile containing `ze_le_register.go` delegate to
the original tool. The fresh unmatched per-package `-trimpath` invalidates
only cmd/ze's compile action. This is an explicit diagnostic debug-path input,
not ordinary byte-identical build evidence. There is no `-a`, cold cache,
whole-tree compile, export, VM or image requirement. Existing compiler flags,
toolexec instrumentation, ambiguous dependency-forcing flags and paths needing
whole-field quoting fail the precondition rather than being overwritten.

Each helper ignores TERM/INT and reads the actual `/proc/self/status` ignored
mask before its ready receipt. The compiler helper passes its self pidfd and
still-live Go parent's pidfd via SCM_RIGHTS. The descendant and foreign helper
pass their own pidfds. The controller binds `os.Process` handles only after
the received fd and `WithHandle` fd identify the same pidfs inode. Older
anonymous-inode pidfds are refused. Actual kernel groups, ancestry, unique
tokens and positive socket exchanges are checked before signaling the entry.

The cancellation root signals only the retained native entry handle. The
original five-second observation requires ordinary exit 1, a cancellation
diagnostic, no executing owned builder/gate/descendant, closed owned sockets,
an unreleased compiler marker and a live responsive foreign sentinel. Zombies
are recorded as nonexecuting, matching the original carrier's alive policy.
No fixture group cleanup precedes these assertions. One worker owns each
direct child's Wait and records its actual return timestamp, so simultaneous
timer/exit readiness cannot accept a late return. A native-return timeout stays
red while owned-state, socket and foreign-sentinel assertions still run.
Failure fallback targets only retained, verified handles.

The normal root stops the same descendant, releases the gate, delegates the
real compiler, and requires the native issues JSON without a live-catalog
generation/parse failure. Standing documentation issues retain exit 1; empty
issues require exit 0. Its original minute bound stays in place. This is a
build-phase witness; there is no independent daemon-exec marker here, and the
catalog-running phase remains a separate obligation.

## Verification owed to MAIN

MAIN separately resolved the selected patched Go tree as
`/home/thomas/.cache/ze/stdpatch/aeca112d3dcc206bdff51822fea40919a276d63cd2a1e68a0e4e2bc230d56615/goroot`,
Go 1.27.3, through native job `phase6-selected-patched-goroot-20261106` log
`ad75154c`. MAIN directly read that tree's package Gcflags action hashing,
compiler flag order, TOOLEXEC_IMPORTPATH injection and WithHandle guarantee.
That replaces the proposal's Go 1.27.2 comparison. This author did not rerun it.

Before any execution, independently review pidfs binding, received-fd lifetime,
partial-start cleanup and the raw fixture delta. MAIN must format only the after
source, hash all artifacts, and verify both exact native roots compile and run
with zero skips. A failed precondition or compile is not discrimination.

Run the normal root, then cancellation, separately through MAIN admission with
all feature tags, `-race -p=1 -count=1 -timeout=10m`, this absolute overlay and
package `./internal/le/doc/yangcontract`. MAIN must read each named PASS/FAIL
line and immediately retain each private `case-*/` evidence directory outside
managed scratch. `events.jsonl` has `Event`, `UTC`, `Data`; preserve raw stdout,
stderr and native command identity. Do not print all private data as a report.

Select the two exact new roots only. The preserved older helper's build case
still contains its pre-existing `GOFLAGS=-a`; running the entire package would
execute that unrelated broad-forcing path. This package did not change it or
authorize its execution.

Only after the observer passes independent review and clean execution should
a separate existing-file overlay author the caller-context-detachment producer
cut described in `proposal.md`, carrying it through the native child GOFLAGS.
The unchanged cancellation root must show the intended semantic RED, then
restored PASS. No cut has been authored or applied by this package.

## Do not assume

There is no runtime result, measured coverage or formatter evidence. The pidfs
precondition, compile argv shape and native output shape are source designs
until MAIN runs them. Partial-start fallback deliberately does not sweep any
unidentified descendant; the independent review must judge that boundary before
native execution. All source-role processes are bounded, but it is not a claim
that every possible unrelated launcher failure can be reclaimed by this probe.
No old test is removed, weakened or retargeted. No whole Phase 6 criterion,
catalog-running route, other five native routes or overall spec is closed.
