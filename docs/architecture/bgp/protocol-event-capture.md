# Protocol Event Capture and Replay

<!-- source: internal/core/capture/capture.go -- Header, Event, RedactPayload -->
<!-- source: internal/core/capture/writer.go -- Writer, WriteMessage, WriteConfig -->
<!-- source: internal/core/capture/reader.go -- Reader, MaxLineLen -->
<!-- source: internal/component/bgp/reactor/capture_replay.go -- sessionCapture, teeCapture, CaptureConfigEvent -->
<!-- source: internal/test/cli/cmd_replay.go -- runReplay -->
<!-- source: internal/component/doctor/checks_bgp_capture.go -- the capture directory check -->

When a BGP session misbehaves on an operator's box, capture records what the
peer sent, and replay feeds that recording back through the same state machine
on a developer's desk.

Capture is off by default. A disabled capture costs one nil comparison for each
received message and nothing else.

## Enabling it

```
bgp {
    peer peer1 {
        capture {
            enabled true
            directory /var/lib/ze/capture
            maximum-size 100
            on-limit rotate
        }
    }
}
```

`maximum-size` is a hard cap in megabytes, range 1 to 1024. `on-limit rotate`
moves a full file aside once and starts a fresh one, so a peer uses at most
twice the cap on disk. `on-limit stop` closes the file and captures nothing
further, so a peer uses at most the cap. `ze doctor` reports a directory the
daemon cannot write, under the code `doctor-bgp-capture-directory`.

One file per peer, named `bgp-<peer-address>.jsonl`. A new session moves the
previous session's file to `<file>.1` rather than truncating it: the session
that failed is the one an operator wants, and a peer reconnects within seconds.

## Where the tee sits, and why it sits there

`Session.teeCapture` is called from BOTH read paths, on the complete wire
message, after the body read and before anything consumes the buffer:
`readAndProcessMessage` (`session_read.go`) and `readAndProcessCoalesced`
(`session_coalesce.go`).

That placement is the whole design.

- It sits AHEAD of RFC 7606 enforcement, which tombstones attributes and
  synthesizes withdrawals. The existing `MessageObserver` hook fires after that
  short-circuit, so it cannot see the malformed UPDATE a bug capture exists to
  record.
- It sits AHEAD of coalescing, so a coalesced batch is captured as the N
  messages the peer sent rather than as the one synthetic UPDATE the reactor
  builds. Coalescing is on by default, and it has its own read path, which is
  why there are two tees rather than one.

## The file format

JSONL. The first line is a header; every later line is one event. Keys are
kebab-case and bytes are base64.

| Header field | Content |
|--------------|---------|
| `format`, `version` | `ze-capture` and `1`. A reader refuses any other version rather than guessing |
| `peer`, `started`, `daemon-version` | who, when, and which build |
| `local-as`, `peer-as`, `router-id` | the session identity. A replay rebuilds the SAME session from these: an iBGP capture replayed as eBGP takes a different branch in OPEN validation and stops reproducing the run it was fed |
| `coalesce` | whether the coalesced read path was active |

Every event carries `seq`, `ts` and `type`. `seq` is monotonic per file, which
is how a spliced or truncated file is detected.

| Event type | Fields |
|------------|--------|
| `message` | `direction`, `msg-type`, `len`, `data` (the full wire message, header included), and the source and context ids when the session has them |
| `config` | `op`, `tx-id`, `payload` |
| `session` | `event` (`capture-start`, `connect`, `disconnect`, `drops`, `capture-stop`) and `drops` |

## The bounds

`capture.Writer` encodes each line into a reusable buffer and checks its exact
length against the remaining budget, so a line that would cross the cap is
refused WHOLE. A file never exceeds the cap by one byte and never ends in a
half-written line. A byte reserve is held back from the cap so the closing
`drops` tally and `capture-stop` line always fit.

The hand-off between the session read goroutine and the one writer goroutine is
a queue of fixed depth. A full queue SHEDS the event and counts it: a stalled
BGP read loop is a far worse outcome than a gap in a debug capture. The count is
written into the stream as a `drops` event, so a replay never mistakes a gap for
a quiet peer, and it is exported as `ze_bgp_capture_dropped_events_total`,
labelled by peer.

A config payload is the only unbounded field, because a reconcile carries the
whole BGP config tree. One that would cross the line bound is replaced by a
marker naming the byte count.

## What a capture holds

Every inbound BGP message of the session, complete, exactly as the peer sent it.
That is the peer's routing data: prefixes, AS paths, communities, next hops, its
router id and its capabilities.

No captured MESSAGE can carry a local secret, because a TCP-MD5 key
authenticates the TCP segment and never travels in the BGP payload. Every config
PAYLOAD passes `RedactPayload`, which replaces the value of any leaf whose NAME
matches the secret-key predicate and any bcrypt-shaped string. That predicate is
a name heuristic, so a new secret-bearing leaf must spell one of the names it
knows, and it fails closed: a payload it cannot parse is replaced entirely.

Treat a capture file the way you treat a routing-table dump when you ship it in
a bug report. Nothing uploads it.

## Replaying

```
ze-test replay [--json] [--local-as N] [--peer-as N] [--router-id N] <capture-file|->
```

A capture file of `-` is read from stdin, which is how a file arriving from
another machine is replayed without a temporary one.

`runReplay` builds a `Session` with the identity the header records, gives it a
fake clock started at a fixed epoch, installs a stub connection, and feeds every
captured message through `Session.ReadAndProcess`, the same function the
daemon's read loop calls. There is no second decoder: the announced and
withdrawn prefixes the report shows come off the `WireUpdate` the real path
built, after RFC 7606 enforcement. The three flags override the header, and the
header is used when a flag is zero.

The report names the FSM state after each message, the prefixes each UPDATE
carried, the config operations the capture recorded, and any NOTIFICATION the
session wrote back. That is what a developer compares between two builds.

Replay reproduces message-driven behavior. It does not advance the clock, so a
hold-timer expiry is not reproduced; that needs an event-queue layer this does
not have.

## Related

- `docs/functional-tests.md`, "Replaying a captured BGP session" -- the tool root
- `test/plugin/bgp-capture-replay.ci` -- capture then replay, end to end
