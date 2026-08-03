# 713: textbuf allocation sweep

## Context

Completed the printf allocation analysis (`plan/analysis-printf-allocations.md`) residual work:
eliminated all `strconv.Itoa` + concat patterns and enhanced the `textbuf` library.

## What was done

- Converted 689 `fmt.Sprintf` to textbuf.Buffer (P0-P3 were already done; this session
  finished P4 benchmark + P2 residual mechanical replacements)
- Converted ~254 `strconv.Itoa` + concat to textbuf.Buffer chaining
- Fixed 26 `textbuf.Int/Uint` + concat (same double-allocation problem)
- Enhanced `internal/core/textbuf/textbuf.go`:
  - `Reset(size ...int)` for explicit init and optional pre-sizing
  - `Slice()` for zero-copy borrowed string via unsafe.String
  - `String()` stays safe (copy)
  - `Bool(v)`, `Len()` methods
  - Convenience functions: `IntStr`, `UintStr`, `StrInt`, `StrUint`, `StrIntStr`, `StrUintStr`
  - Removed hidden `init()` branch from every method call
- persist/rr plugins: struct-level Buffer field with `Slice()` for per-UPDATE commands
- Updated `/ze-review` and `/ze-review-deep` to catch these patterns
- Updated `ai/rules/performance.md` to ban single Itoa concat

## Lessons

1. **`strconv.Itoa(n) + "suffix"` allocates twice.** Itoa creates a string, then `+`
   creates another. `textbuf.Buffer` builds into a stack array and allocates once on
   `String()`. The rule now bans all Itoa-in-concat, not just multi-numeric.

2. **Standalone textbuf functions (`textbuf.Int(n)`) in concat have the same problem.**
   `textbuf.Int(n) + "ms"` still allocates twice. Use `textbuf.IntStr(n, "ms")` which
   appends into a single buffer.

3. **Convenience functions must use the same buffer pattern internally.** The first
   implementation of `IntStr` used `string(b) + suffix` which is the exact anti-pattern.
   Correct: append everything into one `[128]byte`, return one `string()`.

4. **`Slice()` (zero-copy) is only safe for synchronous call chains.** Goroutines that
   outlive the current dispatch cannot share a struct-level Buffer (data race). The
   persist plugin's synchronous dispatch loop is safe; rr's replay goroutines are not.

5. **Review skills must explicitly grep for allocation patterns.** Claude's training
   defaults to `fmt.Sprintf` and `strconv.Itoa + concat`. The hook catches Write/Edit
   but not worktree code or script-committed code. The review grep is the second line
   of defense.

## Residual

295 `fmt.Sprintf` remain, all using `%v`, `%T`, `%q`, `%x`, `%X`, or width-padded
formats that require the `fmt` reflection machinery. No further reduction possible
without removing functionality.

## Files

None recorded.
