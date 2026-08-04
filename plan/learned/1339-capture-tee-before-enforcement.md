# 1339 - a capture tee belongs on the bytes, not on the dispatch hook

From `plan/spec-improve-3-event-replay.md`, the per-peer protocol event capture
and the `ze-test replay` harness.

## The reusable lesson: an observer hook cannot see what a bug capture needs

Ze already had a raw-message hook, `MessageObserver` / `addMessageObserver`
(`reactor/reactor_notify.go`), and the mrt plugin uses it. Reusing it for a
capture was the obvious move and it is wrong, for one reason that generalizes:
**the hook fires where the message is DELIVERED, and a bug capture must record
where the message ARRIVED.**

`Session.processMessage` (`reactor/session_read.go`) does all of this before it
ever calls `s.onMessageReceived`:

| Step | What it does to the bytes |
|------|---------------------------|
| RFC 7606 enforcement | tombstones attributes, and replaces one malformed UPDATE with N synthesized withdraw-only bodies |
| family validation | returns early on a non-negotiated family; nothing is dispatched |
| RFC 4486 prefix limits | returns early on teardown; nothing is dispatched |

Every one of those is a case where the peer sent something and the observer sees
something else, or nothing. Those are exactly the sessions an operator captures.
So the tee sits in `Session.teeCapture`, on the complete wire message, before
`processMessage` is entered.

The general form: **when a feature exists to record what came IN, place it at the
input boundary, not at the first hook that happens to carry the same argument
type.** A hook's signature tells you what it receives; only its call site tells
you what it has already lost.

## Trap: a second read path means a second tee, at the same logical point

Coalescing is default ON and `readAndProcessCoalesced`
(`reactor/session_coalesce.go`) is a separate function with its own header and
body reads. Two tee calls are unavoidable. What is avoidable is capturing at the
wrong place in the coalesced path: `flushCoalesce` dispatches ONE synthetic
UPDATE whose header the peer never sent, so a tee there records N peer messages
as one, and a capture taken with coalescing on would not replay the same as the
same traffic taken with it off. Both tees therefore sit on the individual wire
message, and `TestSessionCaptureIdenticalAcrossReadPaths` pins the two streams
byte for byte.

## Trap: the hot path may not wait for the debug feature

The tee runs on the session read goroutine. It queues a pooled item and returns;
one long-lived writer goroutine owns the file. A FULL queue sheds the event and
counts it, because a stalled BGP read loop is worse than a gap in a capture. The
gap is then written into the stream as a `drops` event, so replay reports it
rather than reading a gap as a quiet peer. A backpressure design that blocks, or
one that drops silently, would each have been worse than this one.

## Trap: an exported symbol whose only caller is a test

`Writer.Written()`, `Writer.Seq()`, `Writer.Remaining()` and a `Redacted` alias
of `redact.Placeholder` all shipped with a test as their only caller. They read
as API and they were scaffolding. `ai/rules/completion.md` is the check that
catches it: every exported symbol needs a non-test caller, and the fix is to
delete, not to invent a caller.

## Files

- `internal/core/capture/` -- the format: `capture.go`, `writer.go`, `reader.go`
- `internal/component/bgp/reactor/capture_replay.go` -- `sessionCapture`, `teeCapture`, `CaptureConfigEvent`
- `internal/test/cli/cmd_replay.go` -- `runReplay`, which drives `Session.ReadAndProcess`
- `test/plugin/bgp-capture-replay.ci`
