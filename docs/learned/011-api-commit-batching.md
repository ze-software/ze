# 011 — API Commit Batching

## Objective

Replace sleep-based timing in ExaBGP test scripts with explicit commit semantics (`commit start` / `commit end`) to make BGP UPDATE generation deterministic and enable the ExaBGP .ci test suite to pass.

## Decisions

- Commits are named (always require a name) rather than anonymous — enables concurrent commits from the same API client.
- `commit <name> end` flushes without EOR; `commit <name> eor` flushes with EOR — the distinction is explicit because EOR semantics differ between initial sync and API batches.
- Grouping is controlled by `rib { group-updates }` config, not by the commit command itself — the commit controls when to flush, not how to group.
- `announce route ...` without a commit prefix sends immediately — backward compatible with scripts that don't use batching.
- Process I/O race condition existed in `process.go ReadCommand`: orphaned goroutines were dropping data on timeout. Fixed with a single reader goroutine + channel.

## Patterns

- Route conflicts within a commit: last-wins for announces, announce+withdraw of same prefix cancels both.
- Routes are queued in a `Transaction` struct (per commit name, held in `CommitManager`), flushed on `end`/`eor`.

## Gotchas

- iBGP attribute defaults were missing: `announce route` was not sending empty AS_PATH or LOCAL_PREF 100, causing test failures. Fixed in attribute parsing.
- Phase 3 (RIB configuration integration), Phase 4 (config formatter), and Phase 5 (test conversion) were not completed — only self-check API test support and core commit commands were implemented.

## Files

- `internal/component/plugin/commit.go` — CommitManager, Transaction, handleCommit dispatch
- `internal/component/plugin/commit_manager.go` — concurrent commit tracking
- `internal/component/plugin/process.go` — process I/O race fix
