# 565 -- bfd-3b-frr-interop

## Context

Stage 3 (`plan/learned/560-bfd-3-bgp-client.md`) wired BGP peer
opt-in through to the BFD plugin but the functional test
(`test/plugin/bgp-bfd-opt-in.ci`) was a wiring smoke test only:
no second BFD speaker, no actual three-way handshake, no
wire-format verification. Stage 3b closes that gap by adding a
scenario under `test/interop/scenarios/` that runs ze and FRR
side-by-side in Docker containers and asserts both the BGP and
BFD sessions reach Up.

This is the highest-value test for catching wire-format
regressions in ze's BFD codec because FRR's `bfdd` is an
independent RFC 5880 implementation and any drift in the
Control-packet layout, state machine, or timer negotiation shows
up as a session that never reaches Up.

## Decisions

- **Scenario-based under `test/interop/scenarios/33-bfd-frr/`**
  rather than a new top-level test framework. The existing
  interop runner already handles Docker container lifecycles,
  scenario discovery, and per-scenario ze + FRR / BIRD / GoBGP
  configurations. Adding a BFD scenario means two files
  (`ze.conf`, `frr.conf`) plus a `check.py` that imports
  `FRR.wait_bfd_up`, and `run.py` picks it up automatically.

- **`FRR.wait_bfd_up` added to `test/interop/interop.py`.** The
  existing `FRR` class already has `wait_session` (BGP) plus
  route assertion helpers; the BFD helper shadows the BGP
  polling pattern and scrapes `vtysh -c "show bfd peers"` for
  `Status: up`. That output is stable across FRR versions going
  back to 7.x and works for both single-hop and multi-hop
  peers.

- **Shared `daemons` file enables `bfdd=yes` globally.** The
  interop runner mounts one `test/interop/daemons` file across
  every scenario. Making `bfdd=yes` adds ~50ms to FRR container
  startup and idles when no `bfd` stanza is configured, so the
  cost to other scenarios is negligible. The alternative (per-
  scenario daemons override) would require runner changes and
  add complexity for every future BFD scenario.

- **Single-hop peer only in the first scenario.** RFC 5881
  single-hop is the common case and matches ze's default
  transport binding. Multi-hop interop can land as a second
  scenario (`34-bfd-multihop-frr`) when a use case materializes;
  the scaffolding is ready.

- **BGP session + BFD session both asserted.** The check.py
  calls `frr.wait_session(ZE_IP)` first, then
  `frr.wait_bfd_up(ZE_IP)`. Both must pass for the test to
  succeed. BFD-only success with BGP in Idle would indicate
  that ze's Stage 3 wiring (startBFDClient on Established) is
  not firing.

## Consequences

- **First real wire-format check between ze and an RFC 5880
  peer.** If ze's Control-packet encoder drops a bit or
  mis-orders a field, the session never reaches Up and the
  scenario fails. This is a regression safety net that the
  in-process unit tests and the Stage 3 wiring smoke test
  cannot provide.

- **Requires Docker + FRR image to run.** Interop tests are not
  part of `make ze-verify`; they run under `make ze-interop`
  which operators execute locally or in CI that has Docker
  available. A contributor without Docker sees the scenario as
  "present but not running" in local test runs, same as every
  other interop scenario.

- **Scope for Stage 5 authentication interop.** With the
  scaffold in place, a follow-up scenario can test
  authentication: configure ze and FRR with matching SHA1
  keys and verify the session reaches Up. Without matching
  keys, the session should fail to establish. Tracked
  informally; no deferral row yet because the scaffold is
  trivially extensible.

- **Scope for Stage 6 echo mode interop.** Similarly, once the
  Stage 6b echo transport lands, a scenario can verify echo
  packets flow between ze and FRR's bfdd echo mode.

## Gotchas

- **Interop scenarios are not part of `make ze-verify`.** The
  runner is invoked separately (`python3 test/interop/run.py`)
  because it needs Docker. A local developer who edits ze's
  BFD code and runs `make ze-verify` WILL NOT see an interop
  failure; they have to run the interop suite explicitly.

- **The existing scenario `01-ebgp-ipv4-frr/ze.conf` uses a
  flat config format** (`peer 172.30.0.3 { router-id ... }`)
  that the current ze parser REJECTS. Either that scenario is
  broken in the main branch, or the flat format was a pre-
  release legacy parser that got removed. Stage 3b uses the
  modern YANG form (`peer peer1 { connection { remote { ip ...
  } ... } }`) because it actually validates. A separate
  cleanup of the other scenarios to use the modern form is a
  follow-up.

- **FRR's `show bfd peers` output contains the peer IP as a
  bare address, not in a labelled field.** Substring matching
  for `ZE_IP` plus `Status: up` is enough to detect a healthy
  session without parsing JSON.

- **FRR bfdd's default transmit rate is 300 ms.** ze's
  profile in `33-bfd-frr/ze.conf` matches (300 000 µs) so
  the timers agree out of the box.

## Files

- `test/interop/scenarios/33-bfd-frr/ze.conf` (new) -- BFD
  enabled profile, BGP peer with `connection { bfd { ... } }`
  opt-in, single-hop single-peer config.
- `test/interop/scenarios/33-bfd-frr/frr.conf` (new) -- FRR
  traditional config with `bfd { peer 172.30.0.2 }` and
  `neighbor 172.30.0.2 bfd` inside `router bgp`.
- `test/interop/scenarios/33-bfd-frr/check.py` (new) -- waits
  for BGP Established then BFD Up.
- `test/interop/interop.py` -- `FRR.wait_bfd_up` added.
- `test/interop/daemons` -- `bfdd=yes`.
- `plan/deferrals.md` -- row closed.
