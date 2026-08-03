# fixit-ci-plugin-suite-nine

Clearing nine plugin-suite reds: what actually produces "flaky".

`verify` had been red on `main` on every run for at least three days (15/15 runs
back to 2026-07-25), each with a DIFFERENT set of failing plugin tests. Nine
failed in run 30343424675. All nine are cleared; the plugin suite went from
`503/512 failed 9` to `514/514` in a full `make ze-verify`.

The rotating cast was the tell. Nine failures, seven distinct proximate causes,
but only four underlying shapes, and two of them were product defects rather than
test defects.

## Shape 1: a guard that cannot tell "not yet" from "no more"

`processForward` (`internal/component/bgp/plugins/rs/server_withdrawal.go`)
dropped an entire UPDATE when `!peer.Up`. Its rationale covered exactly one case:
the peer is DOWN, so `handleStateDown` will withdraw everything anyway. But
`PeerState` is created by `handleOpen` as well as by `handleState`, and the engine
relays a peer's OPEN and its first UPDATE from the session READ path while the
state transition travels the FSM goroutine. So `!Up` is also true for a peer that
is merely NOT YET UP.

The loss was permanent, not transient: `dispatchStructured` has already advanced
`seenMsgID` past the message, so the cut `handleStateUp` captures excludes it from
the replay rail too. A route arriving one scheduling beat before the peer-up event
was gone for good, on a healthy session.

Fix: `PeerState.StateSeen`, set by `handleState` for BOTH polarities. `!Up` now
means DOWN only after a state event has been observed.

**Generalize:** a boolean that means "the thing is true" cannot express "we have
not been told yet". Whenever a guard reads `!x` as a decision, ask what `!x` means
before anything has set `x`. If the two readings differ in consequence, the type
is wrong, not the guard.

The mirror image is STILL OPEN and now recorded in
`plan/known-failures/bgp-plugin-rs-forward-duplicate-and-order.md`:
`selectForwardTargets` requires `Up` of the TARGETS, and `buildReplayRoutes` emits
announcements only, so a destination that loses the same race gets the announce
via replay and never the withdraw. Owned by
`plan/spec-fixit-stored-route-relay-hardening.md`.

## Shape 2: two builders, one route, two byte strings

`writeRelayPayload` (`internal/component/bgp/reactor/relay_payload.go`) strips the
stored MP_REACH and re-synthesizes a single-NLRI one, but APPENDED it, so every
attribute the source had placed AFTER MP_REACH ended up in front of it. The live
forward rail relays the source's bytes untouched. Same route, two encodings,
selected by whether the destination was an established forward target at forward
time.

RFC 4271 leaves attribute order free, so both are legal BGP and nothing downstream
complains. What it breaks is any consumer comparing the two, which is what an
exact-hex functional test is.

Fix: emit the replacement at the POSITION of the span it replaces.

**Generalize:** this is the third round of this exact defect in this tree (see the
ordering notes in `reactor/peer_rib_routes.go`, which record two earlier ones).
When N builders can emit the same object, byte-identity between them is a property
that needs a test, not a convention. It never shows up as "wrong output"; it shows
up as an intermittent test, because the rail is chosen by scheduling.

## Shape 3: the observer owns shutdown but waits on the wrong thing

Five of the nine. An in-daemon observer plugin runs `request shutdown`, so it
decides when the daemon dies, while the ze-peer processes it never looks at are
still asserting on the wire. Every variant failed the same way:

- waiting for the observer's OWN inbound condition (adj-rib-in has the route) and
  shutting down, while the dest peer had not been sent anything yet;
- `api.quiesce()` as the only barrier: it drains the forward pool, which says
  nothing when the destination was not yet an established target at forward time
  and the route therefore arrives later on the replay rail;
- shutting down on a collector's marker file that says nothing about the BGP
  session.

The repo already had the right barrier, documented: `api.wait_peer_eor_sent()`
(`test/scripts/ze_api.py`), whose docstring calls itself "the barrier an observer
MUST hold before it dispatches `request shutdown` in any test whose ze-peer asserts
the EOR frame". It was simply not used.

The state-based barrier for "the dest actually got the route" is
`updates-sent - eor-sent >= 1`: `IncrUpdatesSent`
(`internal/component/bgp/reactor/reactor_notify.go`) counts every UPDATE written
INCLUDING the End-of-RIB, and `eor-sent` is the separate marker counter, so only
the difference is the forwarded route. Comparing `updates-sent` against 0 is
satisfied by the peer's own initial-sync EOR and proves nothing.

**Two traps inside this shape**, each of which cost a full debug cycle:

1. Do NOT gate such a predicate on `state == established`. A check-mode ze-peer
   without `linger` closes the instant its rule matches, so the very success you
   are polling for puts the session into "connecting" a moment later. The predicate
   becomes unsatisfiable in exactly the run that passed.
2. Budget the poll. Every `dispatch_until` attempt is a full engine RPC; 60 of them
   on a starved daemon outlast the test's own timeout and turn a run whose wire
   assertions ALREADY PASSED into an opaque timeout. Put `quiesce()` first so the
   poll is a safety net, not the mechanism.

## Shape 4: the test asks for something the product cannot do

`test/plugin/eor.ci` sent `update text nlri <family> eor` twice and asserted the
markers on the wire. Its header explained that graceful-restart was removed "to
prevent automatic EOR on session establishment". That premise is false:
`sendInitialRoutes` (`internal/component/bgp/reactor/peer_initial_sync.go`) emits
one per NEGOTIATED family unconditionally.

Those markers claim the per-family slot RFC 4724 Section 2 allows, so the explicit
EOR is refused at BOTH gates in `AnnounceEOR`
(`internal/component/bgp/reactor/reactor_api_forward.go`): `peer.ShouldQueue()`
before the drain, `!peer.ClaimInitialSyncEOR(fam)` after it. There is no third
window: before establishment the peer is skipped entirely and the call returns "no
established peers to send to". **The sends could never put a marker on the wire.**
Their only observable effect was to raise once ze-peer, satisfied by the REAL
markers, had closed the session.

The test passed for years because `sendInitialRoutes` satisfied its expectations.

**Generalize:** a passing test is not evidence that its stated mechanism ran. Here
a second producer satisfied the assertion silently. If a test names the producer it
intends to exercise, something has to pin that it was that producer, or the test
survives the feature's removal.

## Environmental, worth separating out

- **`/dev/kmsg`**: world-readable, but `dmesg_restrict=1` refuses a reader without
  CAP_SYSLOG, which is every GitHub runner. The handler returned a bare
  `open /dev/kmsg: operation not permitted`, naming the syscall and hiding both
  remedies. It now names CAP_SYSLOG and the sysctl, and the test models the third
  outcome it always had (Linux, handler present, kernel says no) instead of only
  the non-Linux stub's.
- **BFD ports**: BFD listens on the ports RFC 5881/5883 FIX. All 14 BFD tests
  co-bind `0.0.0.0:3784/3785` under `SO_REUSEPORT`, and the kernel HASHES each
  inbound datagram to one socket in that group
  (`internal/component/bfd/transport/udp_linux.go`), so reflected echoes landed on
  sibling daemons and `ze_bfd_echo_rtt_us` stayed empty. A port an RFC fixes cannot
  be partitioned by unique addresses: they now share
  `option=exclusive:group=bfd-ports`.
- **A hardcoded port**: `bfd-echo-handshake.ci` pinned telemetry to 19274, outside
  every range the runner hands out, so a sibling with a shifted range collided and
  ze logged `metrics server failed to start: address already in use`. Take ports
  from `$PORT`/`$PORT2`, always.

## Two build-recipe drifts, same class

- The functional runner built the `ze-test` helper with `-tags "ze_test"` alone
  while building the daemon from `TestBuildTags()`. Feature-gated plugins register
  in `init()`, so the helper's registry was missing every one of them and `ze-test
  plugin-external as112` exited 1 with "unknown registered plugin" before the
  plugin under test could say anything. Two recipes for one binary; both now derive
  their feature set from `feature-gates.txt`.
- The `await=stderr` fence returned early on timeout, skipping the output
  collection at the bottom of the function, so the report showed "expected 0 /
  received 0" and NOTHING the daemon said, which is the one thing that explains why
  the needle never arrived. Diagnosing tests 37/213 was impossible until that was
  fixed, and took one edit.

**Generalize:** when a diagnosis stalls because the failure report is empty, fix
the report first. It is almost always cheaper than the next round of guessing.

## Method

`scripts/dev/stress-repro.py <suite> --test N --any-failure` reproduced six of the
nine on invocation 1 to 32 where a plain rerun never did. `--any-failure` is
mandatory for assertion flakes: without it only a crash counts and the evidence is
discarded.

Two of the reproductions were the harness rather than the product, and both were
initially mistaken for real: running N copies of ONE test concurrently collides on
any fixed global resource the test owns (the BFD RFC ports, a hardcoded telemetry
port). Before believing a stress reproduction, ask whether the concurrency it
creates is a concurrency the real suite ever has.

## Related

- `plan/known-failures/bgp-plugin-rs-forward-duplicate-and-order.md` -- the
  destination-side half of Shape 1, still open
- `ai/rules/testing.md` -- the stress reproducer, and why "passes in
  isolation" is a symptom rather than a conclusion
- `ai/skills/ze-test.md` -- the draft-first test workflow this produced

## Files

None recorded.
