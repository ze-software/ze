# 1259 -- bgp-plugin-speaker

## Context

The entire `.ci` suite stayed green while ze's route replay emitted NEXT_HOP twice (RFC 7606 Section 3(g)), because `ze-test peer` asserts only the bytes it was told to expect; FRR caught the bug, nothing in-tree did. The goal was a minimal, independent Python BGP speaker for interop-style testing, built like ExaBGP's process model: a fixed engine (the plumbing) plus a per-test plugin loaded via `importlib` (the only code that changes). It exists to catch the class of bug a compliant-but-lenient in-tree peer cannot: wire output that a strict independent peer rejects, without needing a full Docker daemon.

## Decisions

- One engine per test, per-instance router-id (ExaBGP-style) over shared state, so multiple engines never collide; proven live by scenario 49 (two speakers at distinct IPs, both Established).
- Tests are dynamically loaded plugins with only `NAME` + `on_update` (+ optional `on_end`) over a built-in validator: the engine validates NOTHING on its own, so a check exists only because a test wrote it, and every plugin ships red/green unit fixtures. This is what keeps a hand-rolled speaker from accreting its own broad, buggy validator.
- Interop scenario over a `.ci` functional test for the dup-NEXT_HOP proof, because the bug lives in the wire-mode re-encode path (`buildWireModeUpdate`, `reactor_api_batch.go`) which only fires when ze forwards to a DISTINCT receiving peer, and the `.ci` harness is single-IP (documented at `test/plugin/adj-rib-in-replay-on-peerup.ci`). The Docker harness gives every peer its own IP, so scenario 48 is stronger coverage than any `.ci` could be.
- Distinct `_TIMEOUT` vs `_CLOSED` sentinels in the engine's socket reads (review ISSUE 1) over a single `None`, so an idle gap does not end the session and a delayed duplicate cannot yield a false GREEN.

## Consequences

- The interop harness starts speaker sidecars by file presence (`speaker-args`/`speaker2-args` in `test/interop/interop.py`), same as the injector; new per-test checks are a small plugin file plus a scenario, not new plumbing.
- Scenario 48 was PROVEN to discriminate: with the `buildWireModeUpdate` NEXT_HOP dedup reverted the speaker fails "type 3 appears more than once" (RED); with the fix, GREEN. Scenario 47 was also re-verified non-vacuous, but via its live-relay Path 2, not the replay Path 1 its comments claim; fixing that comment attribution belongs to the relay-shape (Thread A) work, not this spec.
- The engine is deliberately not a mature BGP stack (Known Limitation): it decodes only what plugins need, and it does not parse the peer's OPEN (KA cadence uses its own hold-time/3, harmless while both sides use 90s). It complements the Docker daemons (FRR/BIRD/GoBGP), it does not replace them.
- Local Docker-image state trap: a RED experiment leaves `ze-interop:latest` as a fix-reverted build; any `run.py` without `NO_BUILD=1` rebuilds from correct source.

## Gotchas

- The most expensive trap: a test knob (`--stop-after-updates 1`) made the scenario look non-discriminating across 11 Docker runs, spawning two WRONG conclusions ("scenario 47 is vacuous", "adj-rib-in replay races the established-peers set"). The first UPDATE is the verbatim initial-sync forward; the dup arrives LATER on the delta-replay re-announce, and the speaker was quitting before it. With `--stop-after-updates 0` the scenario discriminates. The "no established peers to send to" error was ze finishing replay to an already-disconnected speaker, not a pre-establishment race.
- Static path analysis ("RS forwarding is `buildFwdBody`, never `buildWireModeUpdate`") was incomplete: the live-relay announce path DOES reach `buildWireModeUpdate`. Empirical byte-level evidence had to correct the code-reading twice, in both directions.
- Independent review found the engine's `_recv_exact` conflated timeout with EOF (the stay-connected design was inoperative and discrimination held only by burst luck), an uncrashable-decode gap, and unguarded `sendall`; all fixed and unit-tested before closure.

## Files

- `test/interop/speaker/engine.py` (fixed engine: OPEN/KEEPALIVE/UPDATE framing, plugin dispatch, timeout/close sentinels)
- `test/interop/speaker/plugins/no_duplicate_attribute.py` (first plugin, RFC 7606 Section 3(g))
- `test/interop/speaker/test_engine.py` (10 unit tests incl. red/green fixtures and the review-fix tests)
- `test/interop/interop.py` (speaker sidecar startup, `SPEAKER_IP`/`SPEAKER2_IP`)
- `test/interop/scenarios/48-rfc7606-speaker-dup-attr/` (discriminating dup-NEXT_HOP scenario)
- `test/interop/scenarios/49-speaker-two-instance/` (two-engine collision-free proof)
- `docs/architecture/testing/interop.md`, `ai/INDEX.md` (discovery)
