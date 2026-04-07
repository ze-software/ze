# Documentation Testing

Ze ships several tools that validate documentation against the live code.
They live in `scripts/` and are exposed as `make ze-*` targets.

## Quick start

```sh
make ze-doc-test       # Run all documentation tests
```

This is the umbrella target. It runs every documentation checker and
returns non-zero if any of them report drift. Run it after editing
documentation files, after adding/removing plugins, or as part of review.

## What gets checked

| Tool | Make target | What it validates |
|------|-------------|-------------------|
| `scripts/docvalid/doc_drift.go` | `ze-doc-drift` | `docs/DESIGN.md` plugin counts, family lists, `.ci` test totals, interop scenario count, fuzz target count, Go test count -- compared to the live plugin registry, family registry, and filesystem walk. Also `docs/comparison.md` family rows. |
| `scripts/docvalid/commands.go` | `ze-validate-commands` | Every YANG `ze:command` declaration has a registered RPC handler, and every registered RPC handler has a matching YANG declaration. |
| `scripts/lint/consistency.go` | `ze-consistency` | Mixed code/doc consistency: `// Design:` references on `.go` files, cross-reference bidirectionality (`// Detail:` <-> `// Overview:`), stale package references in docs and scripts. |

`ze-doc-test` runs the first two unconditionally and reports a combined
verdict. `ze-consistency` is left standalone because it covers both
documentation and code-style concerns (file size limits, plugin structure
completeness) and is run as part of code review, not doc review.

<!-- source: scripts/docvalid/doc_drift.go -- runChecks -->
<!-- source: scripts/docvalid/commands.go -- main -->
<!-- source: scripts/lint/consistency.go -- package doc -->

## When to run

| Situation | Recommended target |
|-----------|--------------------|
| After editing any file under `docs/` | `make ze-doc-test` |
| After adding or removing a plugin | `make ze-doc-test` |
| After adding or renaming a YANG `ze:command` | `make ze-validate-commands` |
| Before opening a documentation PR | `make ze-doc-test` |

`make ze-doc-test` is **not** part of `make ze-verify` today because the
codebase has pre-existing drift that has not been triaged. Once that backlog
is cleared, the target should be moved into `ze-verify`'s dependency list.

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

  ze-show:bgp-decode  (show > bgp > decode in ze-cli-show-cmd)
  ...

## Handlers with no YANG command (0)
```

Two-direction check. Both directions are contract bugs:
- YANG declares a command but no Go code registered a handler -> dead command
- Handler registered but YANG doesn't declare it -> command unreachable from CLI

## How to fix common issues

| Issue | Fix |
|-------|-----|
| Plugin count claim wrong in DESIGN.md | Update the number; the script reports the actual count |
| Family list missing entries in DESIGN.md | Add the missing entries; the script lists which |
| `.ci` test count claim wrong | Update the count |
| Plugin in registry but not in Shipped Plugins table | Add a row to `docs/DESIGN.md`'s Shipped Plugins table |
| YANG `ze:command` with no handler | Remove the YANG declaration OR write the handler in `internal/component/<area>/cmd/` |
| Handler with no YANG `ze:command` | Add a YANG declaration in the appropriate `*-cmd.yang` schema |

## How the tools find drift

`scripts/docvalid/doc_drift.go` imports `internal/component/plugin/all` so all
plugins register themselves at init, then queries `registry.All()` and
`registry.FamilyMap()`, walks the filesystem for `.ci` files, and compares
those live counts/lists against hardcoded patterns in `docs/DESIGN.md` and
`docs/comparison.md`.

`scripts/docvalid/commands.go` imports the same set plus the BGP cmd plugin
schema/handler packages, loads the YANG modules, and walks the schema tree
looking for `ze:command` extensions. For each extension it checks
`registry.CollectRPCHandlers()` for a matching method name.

`scripts/lint/consistency.go` walks `.go` files, parses `// Design:`,
`// Detail:`, `// Overview:`, `// Related:` comments, checks for asymmetries,
and scans `docs/`/`scripts/` for references to packages that no longer exist.

## Adding a new documentation check

1. Write the check as a `//go:build ignore` Go program in `scripts/docvalid/`,
   following the patterns in `doc_drift.go`.
2. Add a `make ze-foo-check` target to `Makefile`.
3. Add the new target to `ze-doc-test` if failure should fail the umbrella.
4. Add a row to the table in this file.
5. Add a help entry in the Makefile's `Documentation testing:` section.

## See also

- `.claude/rules/documentation.md` -- canonical documentation rules
  including the BLOCKING Documentation Update Checklist for specs
- `.claude/hooks/check-doc-drift.sh` -- PreToolUse hook that runs
  `check-doc-drift` (advisory, exit 1) on every `git commit`
- `Makefile` -- search for `ze-doc-test` to see the umbrella recipe
