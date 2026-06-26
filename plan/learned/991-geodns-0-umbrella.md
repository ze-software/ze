# 991 -- geodns-0-umbrella

## Context

EXA runs a standalone `geodns` daemon (git.exa.net.uk/tech/dev/surfprotect/geodns)
that answers DNS by the client source IP: each customer-IP class gets its own
host->address records, so the same name resolves to different proxies. The goal
was to port it into Ze as a self-contained edge plugin (`internal/plugins/geodns`),
dropping Sentry and moving all configuration -- daemon settings AND the per-source
host records -- into the YANG config tree. Implemented as a spec set (umbrella +
config / server / observability children).

## Decisions

- **Edge plugin, SDK engine.** geodns is a config-driven engine nothing depends on,
  so `internal/plugins/geodns` mirrors tftpserver/dhcpserver (registry.Register +
  `sdk.NewWithConn` + OnConfigure). Runs in-process (goroutine over net.Pipe), so
  `show`/metrics read the same address space as the engine.
- **Reuse `github.com/miekg/dns`** (already vendored v1.1.72) for messages, the
  server framework, and EDNS0 client-subnet -- no new dependency, no hand-rolled wire.
- **CIDR longest-prefix source model** with named, reusable `host-set`s referenced by
  `source` prefixes. Removes the reference's string-prefix quirk AND preserves its
  "internal" sharing (many prefixes -> one set); `external` is the `0.0.0.0/0`/`::/0`
  catch-all.
- **`client-ip-source` config enum** (edns0 / packet / edns0-then-packet) so geodns
  works behind CoreDNS and standalone, instead of the reference's EDNS0-only behavior.

## Consequences

- The whole steering set lives in `service geodns { ... }`, validated at commit with
  rollback; no watched folder. Live reload is the OnConfigure path.
- Resolution is testable cross-platform via an in-process listener on loopback (the
  reference's approach) -- no QEMU needed for the core behavior, only Linux for the
  optional CoreDNS interop.
- The reference's `geoupdate` AXFR experiment and `src/geodns/client.go` are out of scope.

## Gotchas

- `internal/component/plugin/all/all.go` is generated AND shared; concurrent sessions'
  `make generate` clobbered the geodns import mid-build (the runner rebuilds `ze` from
  it, so the schema vanished -> "unknown field in service: geodns"). Re-run
  `make generate` and re-check before building.
- ze's YANG loader has **no `leafref`** -- the `source.host-set` reference is enforced
  by the plugin verifier (`parseConfig`), not the schema. See [[992-geodns-1-config]].
- SOA serial is uint32: a literal `YYYYMMDDHHMMSS` does not fit. See [[993-geodns-2-server]].

## Files

- `internal/plugins/geodns/` (all) -- the plugin
- `internal/component/plugin/all/all.go` -- generated blank imports
- Children: [[992-geodns-1-config]], [[993-geodns-2-server]], [[994-geodns-3-observability-cli]]
