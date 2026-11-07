# Tooling one-off defects: the actual-root inventory, continued

**Spec:** `plan/post-release/engineering/spec-tooling-one-off-defects.md` (in progress)
**Branch:** `main`, Mac checkout. Nothing pushed.
**Supersedes:** the inventory parts of `33-tooling-one-off-reboot-2026-11-06.md`.
**Goal of this line of work:** `TestAnswerAnswersTheInventory` completes on the
actual repository and publishes a correct inventory.

## Where it stands

The actual-root inventory no longer refuses on any of the analyzer gaps found so
far. It now times out: the last run (HEAD 026eea2548) passed klauspost huff0 and
was still running at the 20-minute bound. Before the last four fixes it finished
its traversal in 416 to 645 s and stopped at a refusal each time.

Every stop so far was an analyzer gap, not a real effect on registration.

## Thomas's decisions in this work

| Decision | Where recorded |
|----------|----------------|
| Traversal: measure first, then Route A | `INVENTORY-TRAVERSAL-REVIEW.md`, 52e4a49989 |
| Vendored assembly: prove each body, never trust it wholesale | spec progress section, d8d12da594 |
| Depth: treat the type graph by reachability; keep a bound only for a generic instantiation that grows without end | `INVENTORY-TRAVERSAL-REVIEW.md` "Depth decision", 934d1fb214 |
| Fix the design-evidence hook so a spec edit may drop a missing cited path | a843a12de1 |

## Commits, in order

| Commit | What |
|--------|------|
| 52e4a49989, 96b5efb758 | Traversal decision and the actual-root measurement |
| e2ee3abf78 | Route A (later superseded by reachability) |
| 2bf9e52533 | A local rebind is not a mutation of a declaration |
| 5c07be9e6d | A YANG module's directory comes from its `//go:embed` |
| d04cf29cb8 | A standard-library pull linkname is a standard call |
| 8e7f58c734, d151a7c2b4 | Leaf assembly proved; local assembly macros expanded |
| ca11decbe5 | Standard-library named types external on every route; unknown refusals carry their site |
| 26fdc8726f | Elided composite literals take their stated type |
| 29900ed1a0 | Channels modelled like slices |
| a843a12de1, d8d12da594 | Hook fix; spec bookkeeping and journal row |
| 169f07baa7 | Active-path index (later deleted) |
| 934d1fb214, 3d7f439175 | Reachability for both walks (Tarjan SCC over interned frames; recursive-type fixpoint with an analysis-wide settled memo); Route A, path index and frame-equality helpers deleted; `&T{}` typed; a method value binds its receiver |
| cf304361b3 | Range variables typed for every range kind |
| 3e98133551, 4ea1005fc0, 7b04a267c9 | Test repairs for 3d7f439175's `join` field-table change. 4ea1005fc0 was committed before its test was seen passing; 7b04a267c9 completes it |
| 68791431f0 | Field addresses (`&s.f`, `&a[i]`, `&*p`) and local closures called by name |
| 026eea2548 | Constants and conversions typed; a range over a constant decided by its type |

Each fix carries tests whose named `// MUTATION:` breaks were observed red with
the tree restored byte for byte. Every changed or deleted existing test was
approved by an independent `ze-test-judge` before the edit.

## Next action

The timeout stack at 026eea2548 shows the type walk joining a flattened copy
of the ambient pool every time it meets a standard-library type
(`externalTypeWalk.external` -> `externalFlat(effects.amb)`, eleven `joinFlat`
frames deep), reached from `typeMethods` -> `externalParameters` per exposed
call. The ambient pool is large and grows during the analysis, and each growth
invalidates settled values that read it. First measure it: take a bounded CPU
profile (the harness below) and count ambient flattenings. A likely fix is to
cache the flattened ambient pool per ambient version (`effects.ambient`), so it
is flattened once per growth, not once per standard-library type.

Then rerun the actual root and fix the next stop the same way.

## How this work was run, and the traps

- **Tests:** the five actual-root tests each take about ten minutes, so the
  package run skips them (`-skip
  '^(TestAnswerAnswersTheInventory|TestAPluginCarriesItsSourceDirectoryYANGFilesAndOptionalDependencies|TestBFDCatalogKeepsStartupOrderingSeparateFromDependencies|TestAPluginWithNoRegisteredModuleTakesTheYANGBesideItsPackage|TestEveryYANGPathAPluginAnswersResolvesUnderTheCheckout)$'`)
  and `TestAnswerAnswersTheInventory` runs alone. Tags:
  `ze_le integration $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')`.
- **Red walks:** `tmp/session/2026-11-06-145007/scratch/redwalk5.py` and its
  variants apply each named break, run its test, require red, and restore the
  file's bytes. A read-only agent turns `// MUTATION:` notes into its work list.
- **Bounded profiling:** `tmp/session/2026-11-06-145007/scratch/prof-overlay/`
  (a scratch-only test added through `-overlay`, never committed) profiles the
  actual root for 240 s and writes a CPU profile even when the run would time
  out. `count-overlay/` printed counters for an older version of the walk and
  needs updating to the current counter fields.
- **Judge keys:** the weakening gate keys a test unit by its directory name
  (`inventory.<Test>`), not the Go package name (`repoinventory`). Journal row:
  `plan/journal/guard-keyed-on-a-field-its-consumer-ignores.md`, 2026-11-06.
- **Never chain a commit after a test without checking its result.** That is how
  4ea1005fc0 landed with its test still red.
- The full package run surfaced panics that focused runs had not: a panicking
  test stops the package, which hides every later test.

## Still open in this spec

- The six fixture failures the reboot handover lists (five assembly-header tests
  and `TestCommittedFamilyHelpersRetainCanonicalNames`), unchanged.
- The Phase 6 controlled partial-handoff RED, the fifteen checker proofs and the
  other phases the reboot handover lists.
- `docs/functional-tests.md` keeps an uncommitted paragraph no code produces
  (saved in `plan/handover/2026-11-06-claude-shutdown-leftovers/`).
- The private evidence archive from the claude host is at
  `~/.local/share/ze-evidence/tooling-one-off-defects-01a1a907/` on the Mac.
- Copies of the files 3d7f439175 deleted are in the git-ignored
  `backups/inventory-reachability-20261107-*/`; they can go once nobody needs
  them.
