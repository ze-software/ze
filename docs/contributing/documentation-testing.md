# Documentation Testing

Ze ships several tools that validate documentation against the live code.
They live in `scripts/` and are exposed as `make ze-*` targets. The full
documentation check is still explicit, while `make ze-precommit-verify` runs a
changed-file-aware wiring, documentation, command, and inventory gate.

<!-- source: mk/check-docs.mk -- ze-doc-verify and ze-doc-wiring-check -->
<!-- source: scripts/status/verify_run.go -- stagesForMode -->

## Quick start

```sh
make ze-doc-verify              # Run all documentation tests
make ze-doc-wiring-check    # Run changed-file-aware wiring/doc/inventory gate
```

`ze-doc-verify` runs every documentation checker and returns non-zero if any of
them report drift. Run it after editing documentation files, after adding or
removing plugins, or as part of review. `ze-doc-wiring-check` selects the
checks needed for the current diff and is included in `make ze-precommit-verify`.

## What gets checked

| Tool | Make target | What it validates |
|------|-------------|-------------------|
| `scripts/docvalid/doc_drift.go` | `ze-doc-drift-check` | `docs/DESIGN.md` plugin counts, family lists, `.ci` test totals, interop scenario count, fuzz target count, Go test count, compared to the live plugin registry, family registry, and filesystem walk. Also `docs/comparison.md` family rows, README test-count claims, `docs/features.md` status labels, `docs/functional-tests.md` release-gate suite claims derived from the Makefile, and narrow forbidden stale-claim checks such as the old text parser allocation claim. |
| `scripts/docvalid/commands.go` | `ze-command-contract-check` | Every YANG `ze:command` declaration has a registered RPC or local CLI handler, and every registered RPC handler has a matching YANG declaration. |
| `scripts/dev/code_to_docs.py --check` | `ze-doc-index-check`, `ze-doc-verify` and `ze-generated-files-check` | Two things: every `<!-- source: ... -->` path under `docs/` points to an existing source file or directory, and every SYMBOL the anchor names after its `--` is declared in the `.go` file it points at. Check mode never writes. It does NOT check that `ai/CODE-TO-DOCS.md` is current: that file is generated on demand and gitignored, so git holds no copy that can go stale, and a comparison would only fail on a clone where it was never generated. The failures report separately: a broken ANCHOR prints `MISSING: <path>` with its referencing doc and line, and an undeclared symbol prints `CLAIM:` with the doc, the line, the anchored file and the token. |
| `scripts/dev/check_doc_links.py` | `ze-doc-links-check`, and the `--md-only` subset at the end of `ze-generated-files-reconcile` | Five checks over the path references in the tree: every backticked path and markdown link in `ai/`, `.claude/rules/` and the `plan/` meta documents resolves; every `// Design:` target resolves; every backticked `*.sh` filename and `c_*`/`check_*` function name in the hook-describing documents names something in the tree; every `doc-links: ignore` marker states a reason; and every path reference in every OTHER tracked file resolves too. The last two checks read every TRACKED file, not the walked corpus. A marker no other check reads is audited, and a dead path in any tracked file fails the gate. `make ze-doc-verify` does NOT run this script: `ze-doc-links-check` is its own `ze-precommit-verify` stage. |
| `scripts/dev/digest_check.py` | `ze-digest-check` and `ze-doc-verify` | Every `file:line` anchor in `ai/digests/*.md` resolves to a real file (subsystem-relative via each digest's `<!-- digest-base: -->` header) and an in-range line. Keeps the hand-maintained flow digests from rotting silently as code moves. |
| `scripts/lint/consistency.go` | `ze-consistency-check` | Mixed code/doc consistency: `// Design:` references on `.go` files, cross-reference bidirectionality (`// Detail:` <-> `// Overview:`), stale package references in docs and scripts. |
| `scripts/dev/verify_wiring_docs.py` | `ze-doc-wiring-check` | Changed-file-aware router used by `make ze-precommit-verify`. It runs wiring checks for new exported Go symbols, `ze-command-contract-check` for command sources, `ze-doc-verify` and stale doc-index checks for source-anchored docs, plus inventory checks for plugin/YANG/registration sources. |
| `scripts/dev/ste_check.py --check` | `ze-ste-check`, and `commit_helper.py create` | The six banned ASD-STE100 habits (synonym rotation, hedging, frozen verbs, marketing adjectives, run-ons, phrasal verbs) in every changed file. Each file is compared against its own HEAD version, and it fails when a habit grew, so a document nobody touched can never fail. The BLOCKING form runs at commit time over the commit's own files. Read the whole tree with `make ze-ste-review`. Rule: `ai/rules/writing.md`. |

`ze-doc-verify` runs doc drift, command validation, and source-anchor validation
(path and symbol) unconditionally and reports
a combined verdict. `ze-doc-wiring-check` is the changed-file-aware gate used
by `make ze-precommit-verify`; it delegates to the direct targets in the table only when
the current diff touches matching sources. `ze-consistency-check` is left standalone
because it covers both documentation and code-style concerns and is run as part
of code review, not doc review.

<!-- source: scripts/docvalid/doc_drift.go -- runChecks, checkForbiddenDocClaims -->
<!-- source: scripts/docvalid/commands.go -- main -->
<!-- source: scripts/dev/code_to_docs.py -- check_anchor_symbols, anchor_symbol_tokens, go_declarations -->
<!-- source: scripts/dev/check_doc_links.py -- check_markdown, check_design_refs, check_hook_names, check_ignore_reasons, check_tracked_citations, sweep_tracked, check_baseline_growth -->
<!-- source: scripts/lint/consistency.go -- package doc -->
<!-- source: scripts/dev/verify_wiring_docs.py -- selected_targets -->
<!-- source: scripts/dev/ste_check.py -- review, read_baseline -->

## When to run

| Situation | Recommended target |
|-----------|--------------------|
| After you write any prose, in any file | `make ze-ste-review-changed` |
| After editing any file under `docs/` | `make ze-doc-verify` |
| After adding or removing a plugin | `make ze-doc-verify` |
| After writing a path reference in ANY tracked file | `make ze-doc-links-check` (`ze-doc-verify` does not cover it) |
| After adding or renaming a YANG `ze:command` | `make ze-command-contract-check` |
| After adding a doc validator, inventory source, command source, or exported Go API | `make ze-doc-wiring-check` |
| Before opening a documentation PR | `make ze-doc-verify` |

The full `ze-doc-verify` remains the explicit documentation review target.
`make ze-precommit-verify` runs `ze-doc-wiring-check`, which invokes the relevant doc,
command, inventory, and wiring checks for changed files.

<!-- source: scripts/dev/verify_wiring_docs.py -- selected_targets -->
<!-- source: scripts/status/verify_run.go -- stagesForMode -->

## How to interpret output

### `ze-doc-drift-check`

```
  Documentation drift detected (N issues)

  x docs/DESIGN.md:708: claims 19 interop scenarios, actual is 32
  x docs/DESIGN.md:0: plugin "bgp-nlri-vpn" registered but missing from Shipped Plugins table
  ...
```

Each issue points at a file, a line number (0 = file-level), and a
description. Most fixes are mechanical: update a count, add a missing
table row, remove a stale entry.

### `ze-command-contract-check`

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
| Stale source anchor path | Fix or remove the `<!-- source: ... -->` path, then rerun `make ze-doc-verify` |
| `CLAIM: ... names 'Sym', which is not declared there` | Read the anchored file. When the symbol moved, point the anchor at the file that DECLARES it; when the name changed, write the new one; when the symbol is gone, the sentence above the anchor is wrong too, so fix the sentence. Never reword a real symbol into prose to silence the finding: the check already ignores a token the anchored file names anywhere, so a finding means the token is absent from that file, which no call, field, parameter or env key of that file can be |
| `cannot read the anchored file, so its symbols are unverifiable` | The anchor points at a file the checker could not read or decode. Fix the path. The check fails closed here on purpose: an unreadable file proves nothing about the claims above it |
| `doc-links: ignore marker states no reason` | Write the reason inline, `<!-- doc-links: ignore (why this path cannot resolve) -->`, or delete the marker and repair the reference it was hiding. A marker with no reason is a silent allowlist |
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
directory is missing. The same script writes `ai/CODE-TO-DOCS.md` when run
without `--check`; check mode is read-only. That output is gitignored: it is a
pure derivation of the tree that no code reads, it rebuilds in about a second,
and tracking it meant a diff on every commit that added a source anchor.

One walk of `docs/` also feeds the symbol half. An anchor is
`<!-- source: <path> -- Sym1, Sym2 -->`, and the tokens after the `--` used to
be discarded. `anchor_symbol_tokens` keeps a token only when it is an identifier
or a dotted chain of them, so a description holding a space or a hyphen is prose
and is never checked. `go_declarations` then reads the anchored `.go` file's own
text for top-level funcs, methods, types, vars, consts, struct fields and
interface methods. It is a text scan rather than a `gopls` query, so it carries
no build context and finds a `//go:build linux` declaration on a macOS host.
`check_anchor_symbols` compares the two, and the `report=` argument `main()`
passes decides whether a finding is printed.

`scripts/dev/check_doc_links.py` walks the instruction corpus for paths,
`// Design:` targets and hook names. Its last two checks do not use that
corpus. `sweep_tracked` reads every file `git ls-files` names, one time, and
`check_ignore_reasons` and `check_tracked_citations` share that one walk.
Check 4 reports a `doc-links: ignore` marker that states no reason, because a
marker outside the walked corpus suppresses nothing and nobody audits it.
Check 5 reports a path reference that does not resolve, in any tracked file.
Two moves remove a finding: repair the reference, or mark its line with a
marker that states why the path cannot resolve.

The references that predate check 5 are grandfathered in
`scripts/dev/doc_citation_baseline.txt`, one `citing file<TAB>dead target` pair
per line. It records pairs rather than bare targets, so a NEW file that cites
an already-dead target is reported. `check_baseline_growth` compares the file
against its own version at HEAD. It refuses every pair HEAD does not hold, so
the baseline only shrinks. The comparison is over the pairs, never over their
number: a repair and a new dead citation in one commit leave the total unmoved
and are refused all the same.

Repair a citation to remove its pair, then delete that line from the baseline.
`python3 scripts/dev/check_doc_links.py --write-baseline` regenerates the whole
file from the WORKING TREE, so in a checkout several sessions share it absorbs
whatever they are part way through editing. Use it to shrink the file, and read
the diff it produces before you keep it. A pair the tree no longer carries
prints a `WARN` line, and the exit code stays 0. One session's repair therefore
never reds another session's run. Three roots are outside check 5:
`vendor/` and `third_party/` hold another repository's files, and
`plan/handover/` records the tree as it was.

`scripts/lint/consistency.go` walks `.go` files, parses `// Design:`,
`// Detail:`, `// Overview:`, `// Related:` comments, checks for asymmetries,
and scans `docs/`/`scripts/` for references to packages that no longer exist.

## Adding a new documentation check

1. Write the check as a `//go:build ignore` Go program in `scripts/docvalid/`,
   following the patterns in `doc_drift.go`.
2. Add a `make ze-foo-check` target to `mk/check-docs.mk` or the owning `mk/`
   file.
3. Add the new target to `ze-doc-verify` if failure should fail the umbrella.
4. Add the new target to `scripts/dev/verify_wiring_docs.py` if changed files
   should trigger it during `make ze-precommit-verify`.
5. Add a row to the table in this file.
6. Update `ai/INDEX.md`, and the discovery-surface table of
   `ai/rules/repo-maintenance.md`, when the new check changes what future
   agents should run or discover. That rule file is GENERATED: edit its point
   file under `ai/rules/points/repo-maintenance/`, then run
   `make ze-rules-condensed-update`.
7. Add a help entry in the Makefile or owning `mk/` quick reference.

## See also

- `ai/rules/writing.md` -- canonical documentation rules,
  including the BLOCKING Documentation Update Checklist for specs
- `ai/rules/repo-maintenance.md` -- required discovery updates when new
  checks, tools, or verification gates are added
- `ai/rules/repo-maintenance.md` -- which hooks and make gates enforce which rules
- `mk/check-docs.mk` -- owning make targets for documentation, inventory,
  command validation, and wiring/doc gates
