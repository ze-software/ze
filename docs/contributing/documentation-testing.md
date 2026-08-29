# Documentation Testing

Ze ships native Go tools that compare documentation with the live registries
and source tree. Pre-commit verification selects the checks affected by a
change.

<!-- source: internal/le/doccheck/actions.go -- Actions -->
<!-- source: internal/le/docwiring/docwiring.go -- Run -->
<!-- source: internal/le/verifyengine/run.go -- Run, RunMode -->

## Quick start

```sh
./le doc check verify              # Run all documentation tests
./le doc wiring    # Run changed-file-aware wiring/doc/inventory gate
```

`./le doc check verify` runs every documentation checker and returns non-zero if any of
them report drift. Run it after editing documentation files, after adding or
removing plugins, or as part of review. `./le doc wiring` selects the
checks needed for the current diff and is included in `./le verify current mode full`.

## What gets checked

| Native action | What it validates |
|---------------|-------------------|
| `./le docvalid doc-drift` | Published counts and lists agree with live registries and the tree |
| `./le docvalid command-contract` | Every YANG `ze:command` has a registered handler |
| `./le docs-to-code check` | Documentation source paths and claimed symbols resolve |
| `./le doc check links` | Tracked path citations resolve |
| `./le digest` | Every `file:line` anchor in `ai/digests/*.md` resolves |
| `./le consistency` | Design references, cross-references, JSON tags, and package citations agree |
| `./le doc wiring` | Changed files trigger their documentation and inventory checks |
| `./le ste check` | No ASD-STE100 habit grew against `HEAD` |

`./le doc check verify` combines documentation drift, command validation, and
source-anchor validation. `./le doc wiring` is the changed-file-aware
pre-commit gate.

<!-- source: internal/le/docvalid/actions.go -- Answer -->
<!-- source: internal/le/docvalid/actions.go -- Answer -->
<!-- source: internal/le/docstocode/actions.go -- Answer -->
<!-- source: internal/le/doccheck/actions.go -- Answer -->
<!-- source: internal/le/consistency/consistency.go -- Answer -->
<!-- source: internal/le/docwiring/docwiring.go -- Answer -->
<!-- source: internal/le/ste/actions.go -- Answer -->

## When to run

| Situation | Recommended target |
|-----------|--------------------|
| After you write any prose, in any file | `./le ste review-changed` |
| After editing any file under `docs/` | `./le doc check verify` |
| After adding or removing a plugin | `./le doc check verify` |
| After writing a path reference in ANY tracked file | `./le doc check links` (`./le doc check verify` does not cover it) |
| After adding or renaming a YANG `ze:command` | `./le docvalid command-contract` |
| After adding a doc validator, inventory source, command source, or exported Go API | `./le doc wiring` |
| Before opening a documentation PR | `./le doc check verify` |

The full `./le doc check verify` remains the explicit documentation review target.
`./le verify current mode full` runs `./le doc wiring`, which invokes the relevant doc,
command, inventory, and wiring checks for changed files.

<!-- source: internal/le/docwiring/docwiring.go -- Answer -->
<!-- source: internal/le/verifyengine/run.go -- Run, RunMode -->

## How to interpret output

### `./le docvalid doc-drift`

```
  Documentation drift detected (N issues)

  x docs/DESIGN.md:708: claims 19 interop scenarios, actual is 32
  x docs/DESIGN.md:0: plugin "bgp-nlri-vpn" registered but missing from Shipped Plugins table
  ...
```

Each issue points at a file, a line number (0 = file-level), and a
description. Most fixes are mechanical: update a count, add a missing
table row, remove a stale entry.

### `./le docvalid command-contract`

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
| Functional test release-gate list wrong | Update `docs/functional-tests.md` to match `internal/le/functional/catalog.go` |
| Stale text parser allocation claim | Update `docs/architecture/api/text-parser.md` to describe `textparse.NewScanner` and source-linked result allocations |
| Stale source anchor path | Fix or remove the `<!-- source: ... -->` path, then rerun `./le doc check verify` |
| `CLAIM: ... names 'Sym', which is not declared there` | Read the anchored file. When the symbol moved, point the anchor at the file that DECLARES it; when the name changed, write the new one; when the symbol is gone, the sentence above the anchor is wrong too, so fix the sentence. Never reword a real symbol into prose to silence the finding: the check already ignores a token the anchored file names anywhere, so a finding means the token is absent from that file, which no call, field, parameter or env key of that file can be |
| `cannot read the anchored file, so its symbols are unverifiable` | The anchor points at a file the checker could not read or decode. Fix the path. The check fails closed here on purpose: an unreadable file proves nothing about the claims above it |
| `doc-links: ignore marker states no reason` | Write the reason inline, `<!-- doc-links: ignore (why this path cannot resolve) -->`, or delete the marker and repair the reference it was hiding. A marker with no reason is a silent allowlist |
| Plugin in registry but not in Shipped Plugins table | Add a row to `docs/DESIGN.md`'s Shipped Plugins table |
| YANG `ze:command` with no handler | Remove the YANG declaration OR write the handler in `internal/component/<area>/cmd/` or `cmd/ze/<area>/register.go` |
| Handler with no YANG `ze:command` | Add a YANG declaration in the appropriate `*-cmd.yang` schema |
## How the tools find drift

`internal/le/docvalid.Answer` imports `internal/component/plugin/all` so all
plugins register themselves, then queries `registry.All()` and
`registry.FamilyMap()`. It walks the `.ci` files and reads the native functional
suite catalog from `internal/le/functional`. It compares those facts with
claims in `docs/DESIGN.md`, `docs/comparison.md`, `README.md`,
`docs/features.md`, and `docs/functional-tests.md`.

`internal/le/docvalid.Answer` imports the same set plus the BGP cmd plugin
schema/handler packages, loads the YANG modules, and walks the schema tree
looking for `ze:command` extensions. For each extension it checks
`registry.CollectRPCHandlers()` for a matching method name.

`./le docs-to-code check` scans every Markdown source anchor under `docs/` and
refuses a missing source path or symbol. `./le docs-to-code update` regenerates
the two documentation indexes.

One walk of `docs/` feeds the path and symbol checks. An anchor is
`<!-- source: <path> -- Sym1, Sym2 -->`. `internal/le/docstocode.CheckCodeIndex`
keeps an identifier or dotted method chain from the claim and compares it with
the declarations in the anchored Go file. The scan is independent of build
tags, so a Linux declaration remains visible on a macOS host.

`internal/le/doccheck.Answer` walks the instruction corpus for paths,
`// Design:` targets and hook names. Its last two checks do not use that
corpus. `sweep_tracked` reads every file `git ls-files` names, one time, and
`check_ignore_reasons` and `check_tracked_citations` share that one walk.
Check 4 reports a `doc-links: ignore` marker that states no reason, because a
marker outside the walked corpus suppresses nothing and nobody audits it.
Check 5 reports a path reference that does not resolve, in any tracked file.
Two moves remove a finding: repair the reference, or mark its line with a
marker that states why the path cannot resolve.

The references that predate check 5 are grandfathered in
`internal/le/doccheck/testdata/doc_citation_baseline.txt`, one
`citing file<TAB>dead target` pair per line. It records pairs rather than bare
targets, so a new file that cites an already-dead target is reported. The check
compares the file with its version at `HEAD` and refuses every pair that `HEAD`
does not hold, so the baseline only shrinks.

Repair a citation and delete its pair from the baseline in the same change.
`./le doc check links` validates the result against the working tree and warns
when a baseline pair is no longer needed. The command does not rewrite the
shared baseline, so one session cannot absorb another session's unfinished
edits.

The three roots outside the tracked citation scan are `vendor/`,
`third_party/`, and `plan/handover/`: the first two hold another repository's
files, while the last records an earlier tree.

`internal/le/consistency` parses `// Design:`, `// Detail:`, `// Overview:`, and
`// Related:` comments, checks their symmetry, and reports references to
packages that no longer exist.

## Adding a new documentation check

1. Add callable Go behavior to the owning package under `internal/le/`.
2. Add one action to that package's table and register its area through
   `leroot.Register`. A related check joins an existing area rather than opening
   another root name.
3. Register the action with `internal/le/leroot` and expose it as
   `./le <area> <action>`.
4. Add the callable action to `internal/le/docwiring` when changed files should
   trigger it during pre-commit verification.
5. Add its producer and exact command to this page and `ai/INDEX.md`.
6. If agent rules need the command, edit the canonical rule point and render the
   rule corpus through `./le rules render-update`.

Repository tooling is compiled Go under `internal/le`; package-owned fixtures
belong in that package's `testdata/` directory.

## See also

- `ai/rules/writing.md` -- canonical documentation rules,
  including the BLOCKING Documentation Update Checklist for specs
- `ai/rules/repo-maintenance.md` -- required discovery updates when new
  checks, tools, or verification gates are added
- `ai/rules/repo-maintenance.md` -- which native gates enforce which rules
- `internal/le/doccheck` and `internal/le/docwiring` -- documentation checks,
  command validation, and changed-file wiring
