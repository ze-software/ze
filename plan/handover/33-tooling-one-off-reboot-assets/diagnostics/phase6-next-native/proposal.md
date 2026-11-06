# Next Phase 6 native proof: catalog build cancellation

Source-only proposal, November 6, 2026. No command below has run. No test,
production source, formatter, Go tool, Docker or QEMU operation was executed.
The named new roots below do not yet exist. MAIN must author and independently
review their additive observer before these are runnable instructions.

## Selected obligation and current gap

Choose only P-18's native `doc yang-contract doc-drift` **build phase** next.
It needs no VM, image, kernel, capability or deployment setup. Do not claim
its catalog-running phase, the other five native routes, an admission wait,
whole drift correctness, or Phase 6 closure from this package.

The actual route is `internal/le/doc/yangcontract/register.go:init` ->
`actions.go:runDrift` -> `drift.go:checker.run` ->
`command_surfaces.go:checkPublishedCommandSurfaces` ->
`loadLiveCommandCatalog` -> `BuildDaemon` -> `runCatalogProcess`.
`BuildDaemon` uses the actual checkout's `ze_core` plus declared feature tags,
a five-minute child bound, and `go build ... ./cmd/ze`. `runCatalogProcess`
uses `runner.StartServerProcess`, waits, and defers checked `Close`.

Existing `catalog_cancel_test.go:TestCatalogActionCancellation` has real
compiler and daemon producers, a foreign sleeper, nonzero cancellation,
owned-death and no-next-stage assertions. Its entry is `Answer` inside
`TestCatalogCancellationConsumerHelper`, not the native composition root.
Its child sleeps normally rather than ignoring TERM. Retaining a current run
of it is useful source evidence, not the missing native witness.

## Native entry and cache-safe gate

The additive observer should launch the actual checkout launcher with argv
`./le --name tooling-phase6-native-01a1a907 doc yang-contract doc-drift | json`
(the final `|` is one argv element, not a shell pipeline). Keep the launcher
in main, use no source copy, preserve its exit and both output streams.

Use a `-toolexec` helper, not the existing test's PATH `go` wrapper:
`internal/le/register.go:activatePatchedGOROOT` and
`gen.activate` prepend the patched GOROOT/bin before native dispatch, bypassing
that PATH shim. Pass every `-V=full` request straight to the real tool.

Only gate the `compile` tool when `TOOLEXEC_IMPORTPATH` is exactly
`github.com/ze-software/ze/cmd/ze`, the argv source list includes `main.go`
from the checkout's `cmd/ze`, and excludes `cmd/ze/ze_le_register.go`.
The outer launcher compiles le with `ze_le` and that source present; the
inner shipped daemon compilation omits it. Delegate every other tool and
package unchanged. Log the actual argv and selected Go input identities;
absence or ambiguity of this exact compiler readiness fails the precondition.

A warm target cache must not bypass readiness. The bounded candidate forcing
is a fresh per-case unmatched source prefix in the *single-package* flag:

`-gcflags=github.com/ze-software/ze/cmd/ze=-trimpath=<private-case>/unused-prefix`

Append this and the helper's absolute `-toolexec` path to the native child's
effective GOFLAGS. The unique prefix must match none of the actual compile
inputs or working directories. It changes debug path recording and the
selected main-package action identity; it is diagnostic build input, not a
claim of byte-identical ordinary build output. Preserve that distinction in
the proof. No `-a`, `all` pattern, whole compile matrix, cold cache, export,
or cache mutation is proposed. Dependencies retain ordinary warm-cache use.

Primary local Go 1.27.2 comparison source supports the mechanism:
`/usr/local/go/src/cmd/go/internal/work/exec.go:367` hashes
`p.Internal.Gcflags` into that package's compile action;
`gc.go:136` appends package flags after the default trimpath option;
`shell.go:639` supplies `TOOLEXEC_IMPORTPATH` to tool execution;
`/usr/local/go/src/cmd/compile/doc.go:119` describes the prefix as recorded
source-path removal. MAIN must confirm these details in the **selected patched
toolchain**, not infer that the comparison installation is the actual tool.
The native compiler argv and receipt, not a cache assumption, gate the proof.

## Observation and ownership requirements

1. Establish a separately owned foreign process/socket, with a positive ready
   exchange, outside the native action's groups. Preserve its process handle
   and connection through every assertion.
2. Start the launcher with a retained direct `os.Process` handle and one Wait
   owner. Do **not** wrap that outer action in `runner.StartServerProcess`:
   its pre-reap group cleanup can mask a producer leak. Use direct output
   files or separately bounded drain, and postpone fixture fallback cleanup
   until after the product assertions.
3. At the precise shipped compiler gate, establish the blocked helper and a
   TERM/INT-ignoring descendant holding its own ready socket. Readiness must
   follow installation of the ignored dispositions. Record exact identities,
   parent/group relationships and command inputs. Acquire identity-pinned
   Linux process handles while that ancestry is still live; no numeric PID
   from a stale file grants deletion rights.
4. Signal **only** the retained native action handle with SIGTERM after that
   readiness. Preserve the existing five-second cancellation assertion:
   ordinary nonzero native exit, cancellation diagnostic, every owned handle
   dead and owned socket EOF, foreign process alive and connection still
   responsive, and no compiler-release/next-stage marker. Do not accept raw
   signal death, compile failure before readiness, SKIP or no execution.
5. Keep the existing five-minute compiler-readiness bound and product's
   five-minute build bound. An unready case fails; no timeout is raised.
   Failure fallback uses only the retained entry and verified owned handles,
   after recording the failed oracle. Never kill a guessed group after its
   leader was reaped, never sweep names, and never touch the foreign sentinel.
6. A separate normal control releases the same gate, delegates the real tool,
   and requires native JSON with the real drift report. Because readiness
   proves this root reached its actual daemon compile, valid `issues` JSON
   without `could not generate or parse the live per-command catalog` proves
   that the native consumer passed that compile/catalog boundary. Existing
   unrelated documentation findings remain visible; reconcile exit 0/1 with
   the report instead of requiring that all project documentation be clean.

The observer requires new implementation; this proposal does not assert that
the existing numeric-ps carrier already provides these identity guarantees.

## Producer cut and exact MAIN queue

After the additive observer is independently reviewed, the proposed roots are
`TestNativeCatalogBuildCancellation` and `TestNativeCatalogBuildCompletes` in
an overlay of the existing `catalog_cancel_test.go`. Preserve every old root
and assertion. Tests execute the native launcher; they do not call `Answer`
as their entry. Do not use the native action's 1 exit alone as discrimination.

MAIN can run each root as a separate admitted job with its established
all-feature tags and absolute reviewed overlay path:

```sh
CLAUDE_CODE_SESSION_ID=01a1a907-3278-7971-98dc-dbc41d4a54a5 CGO_ENABLED=1 \
  bin/le-tooling-resume-01a1a907/le job run \
  label phase6-native-catalog-build-cancel quiet command \
  go test -race -tags "$tooling_test_tags" -p=1 \
  -overlay "$reviewed_catalog_observer_overlay" \
  -run '^TestNativeCatalogBuildCancellation$' -count=1 -timeout=10m -v \
  ./internal/le/doc/yangcontract

CLAUDE_CODE_SESSION_ID=01a1a907-3278-7971-98dc-dbc41d4a54a5 CGO_ENABLED=1 \
  bin/le-tooling-resume-01a1a907/le job run \
  label phase6-native-catalog-build-normal quiet command \
  go test -race -tags "$tooling_test_tags" -p=1 \
  -overlay "$reviewed_catalog_observer_overlay" \
  -run '^TestNativeCatalogBuildCompletes$' -count=1 -timeout=10m -v \
  ./internal/le/doc/yangcontract
```

For causal RED, make only this approved diagnostic source cut in
`command_surfaces.go:BuildDaemon`: use
`context.WithTimeout(context.WithoutCancel(ctx), daemonBuildTimeout)` in place
of its current caller-rooted timeout. Do not alter `runDrift`, checker return,
gate, assertions, signals or resource cleanup. The unchanged cancellation
root must fail its five-second native-return/owned-lifetime assertion while
the compiler gate is still demonstrably reached; the normal gate-release
control remains usable. Source cut uses an existing-file Go overlay carried
into the native launcher's child GOFLAGS. The clean observer-only overlay and
cut overlay must be separate exact artifacts.

Run order: clean normal and cancellation; exact producer cut with unchanged
cancellation; exact restoration with cancellation; do not repeat a passed
unchanged control merely for a newer timestamp. A compiler/setup/provenance
failure is not the intended RED. Hash and retain source snapshots, effective
GOFLAGS, native binary and selected compiler identity, exact argv, readiness,
owned/foreign receipts, raw outputs and named test verdicts immediately outside
managed scratch. All execution stays MAIN and serial, after the GoBGP slot.

## Remaining feasibility checks

The unmatched per-package flag must be checked against the actual selected
patched Go source/argv. Native JSON handling and outer Wait cleanup must be
reviewed against actual observer code, not this design. No native observer or
source cut has been authored or run by this package. No new approval or scope
choice is inferred. If one of these conditions fails, report that precondition
rather than substituting a broad cold-cache build or calling the helper's
direct Answer route a native composition-root proof.
