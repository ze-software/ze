# 1061 -- Service Source Address

## Context

Ze's outbound services inconsistently supported source-address binding. Only BGP (via
`network.RealDialer`), TACACS+, and RADIUS allowed operators to specify which local IP
to use for outbound connections. Six other services (BMP, RPKI, FlowExport, LDP, IRR,
Managed) used bare `net.Dialer` or `tls.Dialer` with no source binding, preventing
loopback or management VRF deployments. VyOS T9013 (BMP source-interface) exposed the
gap. The goal was to add `source-address` to all outbound services and generalize
`network.RealDialer` as the standard TCP dialer.

## Decisions

- Chose `source-address` (TACACS+/RADIUS precedent) over `local-address` (BGP/IPsec
  convention) because BGP's `local-address` is mandatory per-peer with listener semantics,
  while infrastructure services need optional outbound-only source binding.
- Generalized `network.RealDialer` by adding `Timeout time.Duration` over creating a new
  abstraction, because RealDialer with zero MD5/TTL has no overhead vs bare `net.Dialer`.
- Used per-service YANG leaves over a shared grouping, because each service has different
  config structure and the leaf is one line.
- LDP wires existing `transport-address` to the dialer over adding a new leaf, because
  RFC 5036 says transport-address IS the TCP session binding address.
- Managed client splits TCP via RealDialer + `tls.Client()` upgrade over wrapping
  `tls.Dialer`, because `tls.Dialer.NetDialer` is `*net.Dialer` (not an interface).
- FlowExport uses inline `net.DialUDP` source binding over a UDP abstraction, because
  it is the only UDP outbound service.

## Consequences

- All operator-relevant outbound services now support `source-address` in YANG config.
- `network.RealDialer` is the standard TCP dialer for BMP, RPKI, LDP, IRR, and Managed
  (in addition to BGP). Future socket options (e.g., SO_BINDTODEVICE) are added once.
- `source-interface` (resolve interface name to IP) is a natural follow-up.

## Gotchas

- RPKI, IRR, and Managed YANG files initially used `type string` instead of `zt:ip-address`,
  caught in review. Always use `zt:ip-address` for source-address leaves and verify the
  YANG module imports `ze-types`.
- Managed's `tls.Dialer` split required removing the redundant `RealDialer.Timeout` since
  the context already had `WithTimeout(ctx, connectTimeout)`.
- RPKI's `connectAndSync` used `dialer.Dial()` (no context). Switched to `DialContext`;
  a later critical review threaded a stopCh-cancellable context so shutdown is not blocked
  for the 30s connect timeout (the initial `context.Background()` had no cancellation).

## Critical Review Findings (fixed in follow-up)

The initial implementation shipped two defects and no behavioral tests; a critical
review caught and fixed them. Record these -- both are recurring shapes.

- **FlowExport source-address was inert (feature-not-wired).** `CollectorConfig` had a
  `json:"source-address"` tag, but the production path is `parseCollectorMap`, a hand-written
  `map[string]any` extractor -- NOT `json.Unmarshal` into the struct. The tag was dead; the
  field was always empty. The parse test passed anyway because `ze config validate` only
  checks the YANG schema, not the Go extraction. **Lesson:** a json tag does nothing when the
  struct is populated by a manual map extractor; add an explicit `m["<leaf>"]` read AND a unit
  test asserting the value reaches the struct. See `RECURRING-PATTERNS.md` (feature-not-wired).
- **Managed TLS hostname verification silently dropped.** Splitting `tls.Dialer.DialContext()`
  into `net.Dialer`/`tls.Client()` is NOT behavior-preserving: `tls.Dialer` infers `ServerName`
  from the dial address when empty (`crypto/tls/tls.go` ~L160-166); `tls.Client` does not. Empty
  `ServerName` + `InsecureSkipVerify=false` (the default) -> `crypto/x509` skips `VerifyHostname`
  (runs only when `DNSName != ""`) -> any CA-signed cert for any host passes (MITM). **Lesson:**
  when replacing `tls.Dialer` with a manual TCP+`tls.Client` split, set `tlsConf.ServerName`
  from the host of the dial address explicitly (`serverNameFromAddr`).
- **All 8 planned unit tests were absent** so both bugs shipped undetected. Source binding is
  provable without a live peer: bind a non-local RFC 5737 (`192.0.2.x`) source and assert the
  dial fails with `cannot assign requested address`. Connect `Timeout` is testable by dialing an
  unrouted TEST-NET address with a no-deadline context and asserting a fast return. Extracting
  one-line dialer/ServerName builders into named helpers (`ldpSessionDialer`, `serverNameFromAddr`)
  makes the AC behavior unit-testable without real network peers.

## Files

- `internal/core/network/network.go` -- added Timeout to RealDialer
- `internal/component/bgp/plugins/bmp/` -- YANG + config + sender dialer
- `internal/component/bgp/plugins/rpki/` -- YANG + config + RTR session dialer
- `internal/plugins/flowexport/` -- YANG + config + UDP source binding
- `internal/plugins/ldp/register.go` -- wired transport-address to dialer
- `internal/component/bgp/plugins/filter_irr/` + `resolve/irr/` -- YANG + config + dialer
- `internal/component/managed/client.go` + `plugin/types.go` -- config + TCP+TLS split
