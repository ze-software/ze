# 1095 — followup-subsystem: DoT/DoH, DNSSEC, SLAAC, exabgp-bridge, MCP, port-defaults

## Context

A consolidation spec of five disjoint subsystem follow-ups salvaged from the
2026-07-06 deferral triage: an internal exabgp-bridge plugin + rpc write
watchdog (AC-1/AC-2), DNS-over-TLS / DNS-over-HTTPS listeners and DNSSEC
stub-validation (AC-3/AC-4/AC-5), interface SLAAC address tracking + darwin/bsd
vet (AC-6/AC-7), MCP GET-SSE `.ci` + legacy-handler deletion (AC-8/AC-9), and
chaos port-range validation + a YANG-default lint (AC-10/AC-11). Nothing couples
the phases; each shipped as its own commit. The goal was to close verified
survivors without re-scoping them into new features.

## Decisions

- **DoT/DoH extend the shared `dnsserver` harness, not each plugin.** DoT is a
  TLS-wrapped TCP `dns.Server`; DoH is a `net/http` handler that drives the same
  `dns.Handler` through an in-memory `dns.ResponseWriter`. Chose one handler
  across four transports over per-plugin DoT/DoH so as112/geodns answer policy
  (allow-from, client-IP selection, the authoritative/recursion guard) is
  identical on every transport. (see rfc/short/rfc7858.md, rfc8484.md)
- **The listener signature folds in a leaf-cert fingerprint** so a rotated cert
  rebinds, and the self-signed fallback is generated once and cached — chosen
  over regenerating per-apply, which would change the fingerprint and churn a
  rebind on every no-op config reload.
- **DoH `send=false` (as112 allow-from denial) maps to HTTP 403**, not a hung
  connection or a fabricated DNS reply. Cleartext DNS silently drops; DoH is
  request/response, so 403 is the honest "received but refused".
- **DNSSEC is the RFC 4035 stub model, not a local validator.** off/permissive/strict
  set the EDNS0 DO bit (CD=0) and rely on a validating upstream to SERVFAIL a
  broken chain; strict rejects, permissive logs, insecure/unsigned (AD=0)
  answers are always accepted. Chose delegation-to-upstream over building an
  in-process RRSIG validator (outside the resolver's stub role). The toggle
  lives in `system { dns { dnssec-validation } }` (the existing resolver config
  container) over a new `service { resolve }` surface.
- **SLAAC is kernel-cooperating classification, not an RA client** (spec A-5).
  `addrOrigin()` maps `IFA_F_*` flags to `AddrInfo.Origin`
  (static/slaac/temporary/dynamic); ze observes what the kernel autoconfigures
  and surfaces it, riding the existing coalesced netlink monitor. (see
  rfc/short/rfc4862.md)

## Consequences

- as112 and geodns share one cert-loading helper (`LoadTLSMaterial`) and one
  cert-validity doctor check (`CheckCertMaterial`, reusing the registered
  `doctor-tls-*` codes), so a third authoritative DNS plugin gets secure
  transports + a doctor check for free.
- The DNS resolver sets the DO bit only when validation is enabled, leaving the
  default (off) query byte-identical to before — no behavior change for existing
  callers.
- `AddrInfo` gained `Origin` + valid/preferred lifetimes; they ride the existing
  `show interface` JSON and the netlink addr-event stream with no new surface.

## Gotchas

- **A hand-written plugin `.ci` that omits the `cmd=background:...ze-peer` /
  `cmd=foreground:...ze` trailer directives fails as a 45s TIMEOUT with
  "received messages: 0"**, which looks exactly like a wedged daemon or a broken
  observer. The peer/config `stdin=` blocks alone do nothing — those two `cmd=`
  lines are what launch the peer and the daemon. Copy a known-good `.ci` (e.g.
  as112-enable.ci) and mutate it; do not hand-assemble the skeleton.
- The plugin-observer harness JSON-marshals event payloads to a **string** before
  `EventBus.Emit`; a test asserting on the payload must `json.Unmarshal` it, not
  type-assert the struct.
- Config parse does **not** materialize YANG defaults for absent leaves — the Go
  parser must apply defaults that mirror the YANG (`DefaultSecureConfig` mirrors
  853/443//dns-query, `SystemConfig` mirrors dnssec off).
- Scalar `leaf listen-port { type zt:port; default N }` avoids the
  `listener_defaults.go` port-defaults lint entirely (only `uses zt:listener`
  services register there) — the right shape for a secure-listener port that
  binds the plugin's existing IPs.
- A manually-added timed IPv6 address (finite valid/preferred lifetime, so
  `IFA_F_PERMANENT` is clear) is the flag-equivalent of a SLAAC address, so an
  integration test can classify `origin=slaac` against the real kernel without a
  radvd/RA daemon.
- The test-relaxation hook counts consolidating N inline `if err != nil` checks
  into one helper as "removing assertions" and blocks it; inline the error checks
  (which adds assertions) rather than extracting a helper.
- gosec G304 on `os.ReadFile(certPath)` needs `//nolint:gosec // cert path from
  parsed operator config`, matching `checks_tls.go`.

## Files

None recorded.
