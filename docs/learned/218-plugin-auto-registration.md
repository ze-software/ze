# 218 — Plugin Auto-Registration

## Objective

Replace manual plugin registration lists with `init()` self-registration so adding a new plugin requires only creating its `register.go` file, with no changes to engine dispatch code.

## Decisions

- Registry (`internal/plugin/registry/`) is a true leaf package with zero project imports — this is the only way to allow plugins to import the registry without creating cycles (plugins ← registry ← plugins would cycle).
- `Register()` returns an error instead of panicking — panicking in `init()` is hard to test and produces unhelpful crash messages; returning an error allows test assertions on duplicate registration.
- `cli.BaseConfig()` eliminates field duplication between `Registration` and `PluginConfig` — both needed the same fields (Name, Description, etc.) and were previously kept in sync manually.
- `gen-plugin-imports.go` script generates `internal/plugin/all/all.go` with blank imports — this file is the single place that triggers all plugin `init()` functions.

## Patterns

- Leaf-package registry with `init()` self-registration is the canonical Ze plugin pattern. Any package that needs to be discovered imports the registry but is not imported by it.
- Code generation for the "blank import everything" aggregator file avoids manual maintenance of a list that must match the filesystem.

## Gotchas

- None.

## Files

- `internal/plugin/registry/` — leaf registry package
- `internal/plugin/all/all.go` — generated blank-import aggregator
- `scripts/gen-plugin-imports.go` — generator for all.go
- `internal/plugin/cli/` — BaseConfig() and RunPlugin() shared CLI helpers
