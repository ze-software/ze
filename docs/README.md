# Ze Documentation

This directory contains user guides, feature inventories, implementation references, plugin documentation, migration notes, and research material. Current behavior should be checked against the source anchors in each document and, for registered runtime surfaces, against the generated registries or runtime introspection commands.

## Information Types

| Type | Location | Source of truth | Purpose |
|------|----------|-----------------|---------|
| Orientation | `architecture.md`, `DESIGN.md`, `why-ze.md` | Code plus current feature inventory | Explain what Ze is and where it fits |
| User guide | `guide/` | YANG schemas, command handlers, plugins | Operator-facing setup and feature usage |
| Feature inventory | `features.md`, `features/` | Source anchors and status labels | Current shipped, partial, experimental, and rejected capabilities |
| RFC status | `features/rfc-status.md` | Source anchors, tests, implementation notes, and explicit gaps | Standards support ledger with what is implemented and what is left |
| Architecture reference | `architecture/` | Go packages named in source anchors | Internal design, data flow, wire format, config, testing, and decisions |
| API and wire reference | `architecture/api/`, `architecture/wire/` | YANG RPC schemas, parser/encoder code | External command, plugin IPC, text, JSON, and BGP wire contracts |
| Plugin development | `plugin-overview.md`, `plugin-development/` | Plugin registry, SDK, RPC packages | How built-in and external plugins are registered, started, and called |
| ExaBGP migration | `exabgp/`, `config-migration.md` | Migration command and compatibility bridge | Mapping from ExaBGP concepts to Ze |
| Contributing and tests | `contributing/`, `functional-tests.md` | Native `./le` actions and Go test runners | How documentation and behavior are tested |
| Research and background | `research/` | Historical notes and external references | Design input, not a current behavior contract unless explicitly source-anchored |
| Assets | `logo/`, diagrams | Generated or static assets | Images and presentation material |

## Current Entry Points

| Need | Start here |
|------|------------|
| Run Ze | `guide/README.md` |
| Check project status | `guide/status.md` |
| Check feature support | `features.md` |
| Check RFC support and gaps | `features/rfc-status.md` |
| Understand architecture | `architecture.md`, then `architecture/core-design.md` |
| Understand plugins | `plugin-overview.md`, then `guide/plugins.md` or `plugin-development/README.md` |
| Check command/API behavior | `guide/command-reference.md`, `architecture/api/commands.md` |
| Check config syntax | `config-reference.md`, `guide/configuration.md` |
| Migrate from ExaBGP | `exabgp/exabgp-migration.md`, `exabgp/` |

## Relationships

`features.md` is the public capability index. Each feature row should point to a guide or feature page and carry source anchors for implementation evidence.

`guide/` explains how to use the feature. It should not invent behavior absent from the code or YANG schema.

`architecture/` explains why and how the implementation is shaped. It should name concrete packages, files, and runtime registration paths.

`plugin-development/` describes the external contract. It must match `pkg/plugin/sdk/`, `pkg/plugin/rpc/`, and the 5-stage plugin server startup code.

`research/` can contain old comparisons and design inputs. Do not treat it as current product documentation without checking its source anchors.

## Runtime Checks

| Surface | Check |
|---------|-------|
| Registered plugins | `./ze --plugins` |
| All commands | `./ze help command` (filterable, `--json` for tooling) |
| Root CLI verbs | `./ze help ai` |
| Daemon API endpoints | `./ze help ai api` (`ze-show:*`, `ze-set:*`, ...) |
| YANG modules | `./ze schema list` |
| Config validity | `./ze config validate <file>` |
| Feature status | `docs/features.md` plus source anchors |
