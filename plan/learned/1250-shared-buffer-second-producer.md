# 1250 -- A shared buffer outlives the comment that says only one goroutine touches it

## Context

Functional test 97 (`test/plugin/bmp-locrib.ci`) had been flaky long enough to be
handed forward twice in `plan/handover/`, always described as one load-dependent
failure. It was two independent data races on shared mutable buffers, one in the
daemon and one in the test runner itself, and the second was hiding evidence
about the first. Both had the same shape: a buffer documented as single-writer,
plus a second producer added later by a feature whose author never re-read the
comment. The daemon race corrupted BMP framing on the wire; the runner race
silently deleted the collector's log lines from the capture that
`expect=stderr:pattern=` is evaluated against, so the test failed claiming a
pattern was missing that the process had in fact printed.

## Decisions

- **Serialize encode-and-flush together, not just the flush.**
  `senderSession.writeMu` is held across `scratchFor` -> encode -> `sendLocked`
  in every `write*`. Locking only the write would have left the two producers
  free to interleave inside the one `scratch` array, which is where the
  corruption actually happened.
- **Explicit `Lock`/`defer Unlock` per method over an `encodeAndSend(need, func)`
  funnel.** The funnel is tidier and impossible to forget, but the closure
  allocates on the BGP-UPDATE -> Route Monitoring hot path that `sender.go`'s own
  comment calls allocation-free. Chose repetition plus a "Caller MUST hold
  writeMu" contract on `scratchFor` and `sendLocked` (`ai/rules/api-contracts.md`).
- **Publish `ss.conn` only after `sendInitiation` returns.** RFC 7854 Section 4.3
  requires Initiation first, and `rfc/short/rfc7854.md` [RFC7854-x-6] already
  claimed ze did this ("the sender always emits Initiation as the first message
  on a fresh connection") -- but `ss.conn` was published before the Initiation
  write, so a concurrent producer could precede it. The ledger asserted a
  property the code did not have.
- **`lockedBuilder` over reusing `syncWriter`** for the runner's shared client
  accumulators: `syncWriter` re-scans its whole buffer for its wait-pattern on
  every `Write`, which is quadratic for a general accumulator.
- **The new cap announces itself.** `lockedBuilder` replaced *uncapped*
  `strings.Builder`s, so its 10 MB cap was new behaviour. A silent cap is a guard
  that neither denies nor speaks (`ai/rules/fail-closed-guards.md`): a positive
  `expect=stdout:pattern=` whose needle lands past the cap fails over a capture
  that looks complete. Both accumulators now append a one-shot truncation marker.
- **Re-stamped the 8 stale rfc7606 audit verdicts rather than re-judging them**,
  but only after proving the staleness was mechanical: 20 files carry rfc7606
  tags, only 4 changed, every hunk is a package requalification or the inserted
  approval header, no RFC-requirement tag moved anywhere in the range, and
  `rfc/short/rfc7606.md` is byte-identical so every `requirement_sha` still
  matches. Independently re-derived by a second reviewer before it was accepted.

## Consequences

- **Fixing the runner capture race will turn some green tests red, correctly.**
  228 orchestrated `.ci` files run more than one process against the shared
  accumulators. 18 carry `reject=stderr`, 2 `reject=stdout`, and 23 embed
  `runtime_fail` -- whose observer sentinel needs no declared assertion at all.
  A forbidden line or a sentinel that used to be dropped is now captured. Those
  are pre-existing false-greens surfacing, not new breakage.
- The BMP Loc-RIB path still does blocking socket I/O on an EventBus subscriber
  goroutine, which `pkg/ze/eventbus.go` says MUST NOT happen. That is
  pre-existing and NOT a regression from `writeMu` -- concurrent `conn.Write`
  calls already serialize on the socket's own write lock
  (`internal/poll.FD.Write` holds `fd.writeLock()` across the whole call,
  EAGAIN waits included), so a producer already waited out any in-flight write.
  Deferred to `plan/spec-fixit-bmp-sender-blocking-and-reload.md`; the remedy is
  a bounded per-session send queue, and the obvious shortcut (`TryLock`-and-drop)
  silently loses Route Monitoring messages.

## Gotchas

- **`tagged_unit_shas` fingerprints the WHOLE enclosing file and keys on
  `file:line`.** Inserting a 9-line header into a tagged test file stales every
  verdict referencing it and shifts every key by +9. A commit that edits tagged
  tests MUST regenerate `ai/RFC-REQUIREMENTS.md` in the same commit; skipping it
  lands the red on the next session as a cross-commit diff.
- **Go already serializes concurrent `conn.Write`.** Byte-level interleaving was
  never the mechanism; the shared *encode buffer* was. Reaching for a mutex "so
  writes do not interleave" would have been the right fix for the wrong reason,
  and would have missed that the corruption happens before the write.
- **A diagnostic tool's own failure can look exactly like the bug it hunts.**
  `stress-repro.py` appended `-v` after the test selector, so the runners read it
  as a test name and every invocation died with `Error: test "-v" not found` --
  scored as `*** REPRODUCED ***`. It also honours `ZE_TEST_NO_BUILD=1`, so a
  landed fix still "reproduces" against a stale `bin/ze`. Both cost a full cycle.
  See `plan/learned/HOOK-FRICTION.md` F10.
- **`session-end-summary.sh` truncates the per-session state file** and carries
  forward only text between the first and second `## Session:` headings, so
  hand-written digests written per `.claude/rules/post-compaction.md` are
  destroyed. Keep detail in a sibling `tmp/session/notes-<SID>.md`. See
  HOOK-FRICTION F9.
- `sender_test.go` is RFC-tagged, and the `rfc-tagged-test` hook scopes to the
  ENCLOSING function -- so an append whose anchor overlaps a tagged function is
  blocked even though it adds nothing to that function. New file instead.

## Files

- `internal/component/bgp/plugins/bmp/sender.go` -- `writeMu`, `sendLocked`,
  Initiation-before-publish, reconnect backoff on Initiation failure
- `internal/component/bgp/plugins/bmp/sender_concurrency_test.go` (new) -- two
  producers over one session; Initiation-precedes-producer ordering guard
- `internal/component/bgp/plugins/bmp/bmp_locrib.go` -- KNOWN DEFECT note
- `internal/test/runner/runner_exec_util.go` -- `lockedBuilder`, truncation
  marker, `syncWriter` short-write fix, `peerOutput.stderr`
- `internal/test/runner/runner_exec.go` -- shared accumulators
- `scripts/dev/stress-repro.py` -- sub-suite selector, `-v` order, `--any-failure`
- `ai/rules/flaky-under-load.md`, `ai/INDEX.md` -- tool discovery
- `ai/RFC-REQUIREMENTS.md`, `rfc/audit/rfc7606.json` -- ledger + verdict re-stamp
