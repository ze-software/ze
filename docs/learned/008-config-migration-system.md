# 008 — Config Migration System

## Objective

Build a database-style migration system for ExaBGP/Ze configuration files, using heuristic version detection and sequential in-memory migrations rather than explicit version fields.

## Decisions

- No version field in config files — heuristic detection from config structure (presence of `group-updates` at neighbor level = v1, presence of `neighbor` keyword = v2, presence of `peer` + `template.match` = v3). This avoids a chicken-and-egg problem with the field itself needing migration.
- Default behavior is strict: refuse to start with deprecated config, require explicit `ze bgp config upgrade`. Opt-in `--auto-upgrade` flag exists for those who accept the risk.
- Migrations are idempotent by design — safe to run multiple times on the same config.
- Baseline v1 = ExaBGP main (2025-12), not "Ze v1" — migration starts from ExaBGP compatibility.
- `backup before write` is mandatory when modifying files; backup location defaults to same directory as original.

## Patterns

- Migration pipeline: Tokenize → Parse (lenient, accepts deprecated fields) → Detect version → Apply migrations → Validate (strict) → Convert to types.
- Three-version chain: v1 (ExaBGP main, RIB opts at neighbor level) → v2 (Ze intermediate, RIB in `rib {}` block) → v3 (Ze target, `neighbor` → `peer`, `peer` globs → `template.match`).

## Gotchas

- v2→v3 migration must preserve insertion order of `match` blocks because they apply in config-file order, not by specificity — `RemoveAllOrdered` / `AddOrdered` required.
- `multi-session` and `operational` ExaBGP capabilities are parsed but warned about, not implemented in Ze — the check command surfaces these.

## Files

- `internal/component/config/migration/` — api.go, detect.go, migrate.go, static.go, helpers.go
- `cmd/ze/config/cmd_check.go`, `cmd_migrate.go`, `cmd_dump.go`, `cmd_fmt.go`
