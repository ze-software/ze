# 1145 — Plugin `internal` keyword

Spec: `spec-plugin-internal-keyword.md`

## Context

Plugin declarations lived under a single YANG list named `external`. Whether a plugin ran in-process or as a separate process was implied by which leaf was set (`use <builtin>` vs `run <path>`). The `internal` keyword makes the in-process intent explicit.

## Decisions

1. **Explicit `internal` list, sibling to `external`.** `plugin { internal <name> { use <builtin> } }` declares a built-in plugin running in-process. Maps to the same `PluginConfig{Internal:true}` as the existing `external { use }` form.
2. **Back-compat preserved.** `external { use X }` and `external { run ze.X }` still resolve to `Internal=true`. No existing config breaks.
3. **Doctor advisory for external-of-builtin.** `ze doctor` emits `doctor-plugin-external-builtin` (warning severity) when an external plugin's `run` command resolves to a built-in plugin name via token matching (split on whitespace, strip `ze.` prefix, `filepath.Base`, check against `AvailableInternalPlugins()`).
4. **Built-in names from registry.** Doctor and YANG validator both query `AvailableInternalPlugins()` / `registry.Has()` at call time, never a hardcoded list.

## Consequences

- Config surface is clearer: `internal` = in-process goroutine, `external` = forked process.
- `ExtractPluginsFromTree` iterates both `internal` and `external` lists, enforcing cross-list name uniqueness.
- `graph.go` process-binding resolution checks `internal` list before `external`.
- YANG `ze:validate "internal-plugin-name"` validator provides tab-completion for built-in names in the CLI editor.

## Gotchas

- **Validator registration test is fragile.** `TestCheckAllValidatorsRegistered_AllPresent` hardcodes every `ze:validate` name. Adding a new `ze:validate` to any YANG file requires updating that test.
- **Map iteration order.** `GetList("internal")` returns a Go map; plugin slice order is non-deterministic. Downstream code (plugin server startup) is resilient to this, but slice-position assertions in tests would flake.
- **`internal` list has only `name` + `use`.** No `encoder`, `respawn`, or `timeout` leaves. Converting existing `external { use X; encoder json }` to `internal` means dropping the `encoder` line (internal plugins don't use wire encoding).

## Files

None recorded.
