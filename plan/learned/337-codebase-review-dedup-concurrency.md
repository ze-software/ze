# 337 — Codebase Review: Deduplication and Concurrency

## Objective

Address code duplication and concurrency issues found during full codebase review. Two duplication hotspots (Open message encoding, config file loading) and goroutine lifecycle violations in the test runner.

## Decisions

- Extracted `writeFixedFields()` helper in `message/open.go` to share the Version/MyAS/HoldTime/BGPIdentifier preamble between `WriteTo()` and `writeToExtended()` — eliminated ~18 duplicated lines
- Extracted `loadConfigData()` in `cmd/ze/config/main.go` to share stdin/file reading across `cmd_dump.go` and `cmd_fmt.go` — `cmd_check.go` and `cmd_migrate.go` kept their own logic due to different return types
- Created `internal/core/syncutil/wait.go` with a context-aware WaitGroup helper — deduplicated a pattern that appeared in multiple goroutine shutdown paths
- Fixed goroutine lifecycle violations in `runner_exec.go` — eliminated untracked `go func()` calls, ensured proper synchronization
- Plugin runner exit code in `process.go` is now logged instead of silently discarded

## Patterns

- When deduplicating, verify the return types and error handling match before extracting — `cmd_check.go` returns `checkResult` while others return `int`, so forcing a shared helper would have added complexity
- The `syncutil` helper pattern (context + WaitGroup) recurs whenever a goroutine pool needs graceful shutdown with timeout — worth having as a shared utility
- Buffer-first encoding helpers (`writeFixedFields`) should write at an offset parameter, not return bytes — consistent with the project's `WriteTo(buf, off) int` convention

## Gotchas

- The Open message dedup was subtle: `writeToExtended()` writes at different offsets (extended format markers) so the shared helper must take `bodyOff` as a parameter, not assume fixed positions
- `loadConfigData()` couldn't cover all 4 config commands because `cmd_check` and `cmd_migrate` wrap the result in command-specific types before returning

## Files

- `internal/component/bgp/message/open.go` — extracted `writeFixedFields()`
- `cmd/ze/config/main.go` — added `loadConfigData()` helper
- `cmd/ze/config/cmd_dump.go`, `cmd_fmt.go` — use shared helper
- `internal/core/syncutil/wait.go` — new context-aware WaitGroup utility
- `internal/test/runner/runner_exec.go` — goroutine lifecycle fixes
- `internal/component/plugin/process/process.go` — runner exit code logging
