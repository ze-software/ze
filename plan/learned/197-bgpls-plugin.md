# 197 — BGP-LS Plugin

## Objective

Create a BGP-LS family plugin for decode/encode of Link-State NLRI.

## Decisions

- Type aliases (re-exports from `nlri`) used instead of moving types, because BGP-LS types are referenced by multiple unrelated packages that cannot be updated atomically.
- CLI decode uses the built-in decoder rather than the plugin, because the plugin's parser is incomplete for Link TLVs — CLI output must be correct even when plugin coverage is partial.
- `pluginFamilyMap` (CLI-side, family→format) and `familyToPlugin` (engine-side, family→plugin name) are distinct maps serving different purposes; do not conflate them.

## Patterns

- None beyond type alias approach for gradual migration.

## Gotchas

- Type aliases create an invisible coupling: if the original type in `nlri` changes structure, the alias breaks silently for consumers not updated. Prefer migration to direct import over long-lived aliases.

## Files

- `internal/component/bgp/plugins/nlri/ls/` — BGP-LS plugin
- `internal/bgp/nlri/` — Link-State types (via alias)
