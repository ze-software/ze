# 867: YANG Rename and Command Ownership

**Spec:** `plan/spec-yang-rename-ownership.md`
**Date:** 2026-06-08

## Context

All YANG files lived in `schema/` directories. The name "schema" was misleading: YANG is declarative data, not code. Additionally, command YANG (-cmd.yang) lived alongside config YANG in component/ directories, meaning removing a plugin's command surface required editing component code.

## Decisions

1. **Rename all `schema/` to `yang/`** -- uniform naming: YANG files go in `yang/`, regardless of whether they're config or command schemas.

2. **Config YANG stays in component/, command YANG moves to plugins/** -- the folder test (delete plugin = feature gone) requires command surfaces to be in plugins/. Config YANG is intrinsic to the component.

3. **Four subsystems kept intrinsic** -- iface (4 cmd files, core network ops), firewall, ike (ipsec security stack), doctor (infrastructure diagnostics). Their command surfaces are inseparable from the component.

4. **`config/schema/cli/` intentionally kept** -- this is the `ze schema` CLI command package, not a YANG directory. Renaming it would rename the user-facing command.

## Consequences

- 119 `yang/` directories (up from 15 pre-existing + 99 renamed + 17 new plugin dirs, minus 6 emptied)
- Codegen (`yang_glue.go`) covers all YANG registration; no hand-written embed.go/register.go needed
- `yang_glue.go` acronyms map expanded: MRT, TFTP, AS, RR, PoolStats, AsPath, FlowExport, AAA, IPsec, PPPoE
- New plugin naming convention: `plugins/<subsystem>-cmd/` for command-only YANG plugins

## Gotchas

- **Codegen doesn't clean up**: `yang_glue.go` generates for dirs with .yang files but doesn't remove embed.go/register.go from dirs that lose all their .yang files. Must delete manually.
- **External test packages**: `package schema_test` needs separate handling from `package schema` when updating package declarations (different word boundary).
- **Glob depth in scripts**: `git ls-files -- 'internal/*/schema/'` misses deeper nesting. Use `git ls-files --deleted` or `find` instead.
- **Variable name drift**: hand-written embed.go used custom capitalization (ZeVppConfYANG) that diverges from codegen convention (ZeVPPConfYANG). Adding acronyms to yang_glue.go resolves this but must be done before consumers are updated.
- **Blank imports in cmd/ packages**: when cmd YANG moves to a plugin, any blank import of the old yang/ package in the component's cmd/ code must update to the new plugin path.

## Files

None recorded.
