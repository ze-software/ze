# Learned: spec-l2tp-8b-radius -- RADIUS Auth/Acct/CoA

## What Was Built

RADIUS authentication plugin (`l2tpauthradius`) supporting PAP, CHAP-MD5,
and MS-CHAPv2. RADIUS accounting (Start/Stop/Interim). CoA and
Disconnect-Message handling per RFC 5176. Prometheus metrics for RADIUS
server health.

## Key Decisions

- **Async auth via Handled sentinel.** `handle()` returns
  `AuthResult{Handled: true}` immediately, spawns a goroutine to do the
  RADIUS round-trip, calls `AuthRespondFunc` on completion. Drain goroutine
  skips its own response when Handled is true.

- **Server failover in radius.Client.** `SendToServers()` tries servers in
  order with per-server retries and timeout. On total failure, Prometheus
  gauge `ze_radius_up` flips to 0.

- **Accounting never tears down sessions.** Per RFC 2866, accounting
  failures are non-fatal. Interim updates fire at configurable interval
  (default 300s). On subsystem Stop, all active sessions get Acct-Stop.

- **CoA replay cache.** 5-minute cache keyed on (source, code, identifier,
  authenticator). Prevents duplicate rate-change or disconnect processing.
  Per-source shared secrets for authenticator validation.

- **Unsupported attributes rejected, not ignored.** Access-Accept attributes
  like Framed-IP-Address and Session-Timeout are explicitly rejected rather
  than silently dropped. Prevents silent misconfiguration.

- **Atomic client swap on config reload.** `swapClient()` replaces the
  RADIUS client pointer; CoA listener restarts. No session disruption.

## Mistakes / Surprises

- Acct byte counters are hardcoded to 0: kernel counters are not yet wired
  through to accounting. This is a known gap, not a bug.

- MS-CHAPv2 mutual auth requires extracting MS-CHAP2-Success VSA from the
  Access-Accept and sending it back to the PPP peer. Easy to miss.

## Patterns Worth Reusing

- Replay cache keyed on (source, code, id, authenticator) is the correct
  RFC 5176 dedup strategy. Reuse for any future UDP request/response
  protocol handling.
