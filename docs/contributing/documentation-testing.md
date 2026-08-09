# Documentation Testing

Ze ships several tools that validate documentation against the live code.
They live in `scripts/` and are exposed as `make ze-*` targets. The full
documentation check is still explicit, while `make ze-verify` runs a
changed-file-aware wiring, documentation, command, and inventory gate.

<!-- source: mk/inventory.mk -- ze-doc-test and ze-verify-wiring-docs -->
<!-- source: scripts/status/verify_run.go -- stagesForMode -->

## Quick start

```sh
make ze-doc-test              # Run all documentation tests
make ze-verify-wiring-docs    # Run changed-file-aware wiring/doc/inventory gate
```

`ze-doc-test` runs every documentation checker and returns non-zero if any of
them report drift. Run it after editing documentation files, after adding or
removing plugins, or as part of review. `ze-verify-wiring-docs` selects the
checks needed for the current diff and is included in `make ze-verify`.

## What gets checked

| Tool | Make target | What it validates |
|------|-------------|-------------------|
| `scripts/docvalid/doc_drift.go` | `ze-doc-drift` | `docs/DESIGN.md` plugin counts, family lists, `.ci` test totals, interop scenario count, fuzz target count, Go test count, compared to the live plugin registry, family registry, and filesystem walk. Also `docs/comparison.md` family rows, README test-count claims, `docs/features.md` status labels, `docs/functional-tests.md` release-gate suite claims derived from the Makefile, and narrow forbidden stale-claim checks such as the old text parser allocation claim. |
| `scripts/docvalid/commands.go` | `ze-validate-commands` | Every YANG `ze:command` declaration has a registered RPC or local CLI handler, and every registered RPC handler has a matching YANG declaration. |
| `scripts/dev/code_to_docs.py --check` | `ze-doc-check-stale`, `ze-doc-test` and `ze-regen-check-readonly` | Two things: every `<!-- source: ... -->` path under `docs/` points to an existing source file or directory, AND `ai/CODE-TO-DOCS.md` itself matches what the generator would write now. Check mode never writes. The freshness half was added 2026-07-20: before it, check mode built the index in memory and compared nothing, so a stale `ai/CODE-TO-DOCS.md` reported "all references valid" and exit 0 (it had drifted by 24 code paths). The two failures report separately: a stale FILE names the regen target, a broken ANCHOR prints `MISSING: <path>` with its referencing doc and line. |
| `scripts/dev/digest_check.py` | `ze-digest-check` and `ze-doc-test` | Every `file:line` anchor in `ai/digests/*.md` resolves to a real file (subsystem-relative via each digest's `<!-- digest-base: -->` header) and an in-range line. Keeps the hand-maintained flow digests from rotting silently as code moves. |
| `scripts/lint/consistency.go` | `ze-consistency` | Mixed code/doc consistency: `// Design:` references on `.go` files, cross-reference bidirectionality (`// Detail:` <-> `// Overview:`), stale package references in docs and scripts. |
| `scripts/dev/verify_wiring_docs.py` | `ze-verify-wiring-docs` | Changed-file-aware router used by `make ze-verify`. It runs wiring checks for new exported Go symbols, `ze-validate-commands` for command sources, `ze-doc-test` and stale doc-index checks for source-anchored docs, plus inventory checks for plugin/YANG/registration sources. |
| `scripts/dev/ste_check.py --check` | `ze-ste-check`, and `commit_helper.py create` | The six banned ASD-STE100 habits (synonym rotation, hedging, frozen verbs, marketing adjectives, run-ons, phrasal verbs) in every changed file. Each file is compared against its own HEAD version, and it fails when a habit grew, so a document nobody touched can never fail. The BLOCKING form runs at commit time over the commit's own files. Read the whole tree with `make ze-ste-review`. Rule: `ai/rules/writing.md`. |

`ze-doc-test` runs doc drift, command validation, and stale source-anchor validation unconditionally and reports
a combined verdict. `ze-verify-wiring-docs` is the changed-file-aware gate used
by `make ze-verify`; it delegates to the direct targets in the table only when
the current diff touches matching sources. `ze-consistency` is left standalone
because it covers both documentation and code-style concerns and is run as part
of code review, not doc review.

<!-- source: scripts/docvalid/doc_drift.go -- runChecks, checkForbiddenDocClaims -->
<!-- source: scripts/docvalid/commands.go -- main -->
<!-- source: scripts/dev/code_to_docs.py -- check_mode -->
<!-- source: scripts/lint/consistency.go -- package doc -->
<!-- source: scripts/dev/verify_wiring_docs.py -- selected_targets -->
<!-- source: scripts/dev/ste_check.py -- review, read_baseline -->

## When to run

| Situation | Recommended target |
|-----------|--------------------|
| After you write any prose, in any file | `make ze-ste-review-changed` |
| After editing any file under `docs/` | `make ze-doc-test` |
| After adding or removing a plugin | `make ze-doc-test` |
| After adding or renaming a YANG `ze:command` | `make ze-validate-commands` |
| After adding a doc validator, inventory source, command source, or exported Go API | `make ze-verify-wiring-docs` |
| Before opening a documentation PR | `make ze-doc-test` |

The full `ze-doc-test` remains the explicit documentation review target.
`make ze-verify` runs `ze-verify-wiring-docs`, which invokes the relevant doc,
command, inventory, and wiring checks for changed files.

<!-- source: scripts/dev/verify_wiring_docs.py -- selected_targets -->
<!-- source: scripts/status/verify_run.go -- stagesForMode -->

## How to interpret output

### `ze-doc-drift`

```
  Documentation drift detected (N issues)

  x docs/DESIGN.md:708: claims 19 interop scenarios, actual is 32
  x docs/DESIGN.md:0: plugin "bgp-nlri-vpn" registered but missing from Shipped Plugins table
  ...
```

Each issue points at a file, a line number (0 = file-level), and a
description. Most fixes are mechanical: update a count, add a missing
table row, remove a stale entry.

### `ze-validate-commands`

```
# Command Validation

YANG commands: 97
Registered handlers: 69

## YANG commands with no handler (30)

  ze-show:bgp-decode  (show > bgp > decode in ze-bgp-tools-cmd)
  ...

## Handlers with no YANG command (0)
```

Two-direction check. Both directions are contract bugs:
- YANG declares a command but no Go code registered an RPC or local handler -> dead command
- RPC handler registered but YANG doesn't declare it -> command unreachable from CLI

## How to fix common issues

| Issue | Fix |
|-------|-----|
| Plugin count claim wrong in DESIGN.md | Update the number; the script reports the actual count |
| Family list missing entries in DESIGN.md | Add the missing entries; the script lists which |
| `.ci` test count claim wrong | Update the count or phrase it as an approximate dated claim |
| Feature inventory row has no status | Add one of: Supported, Partial, Experimental, Stub-backed, Rejected, Future |
| Functional test release-gate list wrong | Update `docs/functional-tests.md` to match `ze-functional-test` in the Makefile |
| Stale text parser allocation claim | Update `docs/architecture/api/text-parser.md` to describe `textparse.NewScanner` and source-linked result allocations |
| Stale source anchor path | Fix or remove the `<!-- source: ... -->` path, then rerun `make ze-doc-test` |
| Plugin in registry but not in Shipped Plugins table | Add a row to `docs/DESIGN.md`'s Shipped Plugins table |
| YANG `ze:command` with no handler | Remove the YANG declaration OR write the handler in `internal/component/<area>/cmd/` or `cmd/ze/<area>/register.go` |
| Handler with no YANG `ze:command` | Add a YANG declaration in the appropriate `*-cmd.yang` schema |
## How the tools find drift

`scripts/docvalid/doc_drift.go` imports `internal/component/plugin/all` so all
plugins register themselves at init, then queries `registry.All()` and
`registry.FamilyMap()`, walks the filesystem for `.ci` files, derives the
functional release-gate suite list from the Makefile, and compares those live
counts/lists against documented claims in `docs/DESIGN.md`,
`docs/comparison.md`, `README.md`, `docs/features.md`, and
`docs/functional-tests.md`. It also scans narrowly scoped stale claims that
previously escaped the broad live-data checks.

`scripts/docvalid/commands.go` imports the same set plus the BGP cmd plugin
schema/handler packages, loads the YANG modules, and walks the schema tree
looking for `ze:command` extensions. For each extension it checks
`registry.CollectRPCHandlers()` for a matching method name.

`scripts/dev/code_to_docs.py --check` scans every markdown source anchor under
`docs/`, extracts referenced code paths, and fails if any referenced file or
directory is missing. The same script regenerates `ai/CODE-TO-DOCS.md` when run
without `--check`; check mode is read-only.

`scripts/lint/consistency.go` walks `.go` files, parses `// Design:`,
`// Detail:`, `// Overview:`, `// Related:` comments, checks for asymmetries,
and scans `docs/`/`scripts/` for references to packages that no longer exist.

## Adding a new documentation check

1. Write the check as a `//go:build ignore` Go program in `scripts/docvalid/`,
   following the patterns in `doc_drift.go`.
2. Add a `make ze-foo-check` target to `mk/inventory.mk` or the owning `mk/`
   file.
3. Add the new target to `ze-doc-test` if failure should fail the umbrella.
4. Add the new target to `scripts/dev/verify_wiring_docs.py` if changed files
   should trigger it during `make ze-verify`.
5. Add a row to the table in this file.
6. Update `ai/rules/repo-maintenance.md`, `ai/INDEX.md`, or
   `ai/NAVIGATION.md` when the new check changes what future agents should run
   or discover.
7. Add a help entry in the Makefile or owning `mk/` quick reference.

## See also

- `ai/rules/writing.md` -- canonical documentation rules,
  including the BLOCKING Documentation Update Checklist for specs
- `ai/rules/repo-maintenance.md` -- required discovery updates when new
  checks, tools, or verification gates are added
- `ai/rules/repo-maintenance.md` -- which hooks and make gates enforce which rules
- `mk/inventory.mk` -- owning make targets for documentation, inventory,
  command validation, and wiring/doc gates
