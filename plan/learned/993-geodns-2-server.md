# 993 -- geodns-2-server

## Context

The second geodns child ([[991-geodns-0-umbrella]], builds on [[992-geodns-1-config]]):
the DNS server. Bind UDP+TCP listeners from the YANG config, answer queries by
selecting the host-set from the client source IP, and synthesize SOA/NS/glue --
all via `github.com/miekg/dns`. No Sentry.

## Decisions

- **Single `"."` mux handler reading `loadState()` per request.** The handler is
  state-independent, so a host-data reload swaps the atomic snapshot with no rebind;
  only an endpoint change (the `listener` set / enabled, tracked by an
  `endpointSig` over each ip:port) stops and rebinds listeners.
- **`client-ip-source` extraction** (`clientIP`): EDNS0 client-subnet `Address` when
  present/allowed, else the packet source -- the packet source is passed in as a
  plain `netip.Addr` so the function is unit-testable without a fake ResponseWriter.
- **NOERROR-centric answers** (matching the reference): an in-zone name with no
  record is NOERROR + SOA-in-Authority; a name outside every zone is NXDOMAIN;
  `ns1..nsN.<zone>` glue is synthesized from the nameserver list.
- **SOA serial `computeSerial(soa, prev, now)`**: `auto-epoch` = `max(unix, prev+1)`
  (strictly monotonic at any rate, the default); `auto-datetime` = YYYYMMDDnn;
  `fixed` = the leaf. Computed once per generation at reload and stored in the snapshot.

## Consequences

- Real DNS resolution is verified cross-platform by an in-process listener on a free
  loopback port (UDP+TCP) -- per-source answers, SOA, negative, and live reload --
  no QEMU required.
- Per-request `recover()` keeps one bad query from taking the daemon down; bounded
  `ShutdownContext` drain on rebind/stop.

## Gotchas

- **SOA serial is uint32** (max 4.29e9 = 10 digits). A `YYYYMMDDHHMMSS` literal
  (14 digits) cannot fit, and `YYYYMMDDnn` cannot widen past 2 counter digits
  (4-digit-year * 1000 overflows) -- so `auto-datetime` is capped at 100/day; use
  `auto-epoch` for higher rates. EXA's reference `2018122500` is the `fixed` case.
- `net.ListenPacket`/`net.Listen` are banned by `noctx`; use `net.ListenConfig`.

## Files

- `internal/plugins/geodns/server.go` -- handler, EDNS0, SOA/NS, listener manager, serial
- `internal/plugins/geodns/register.go` -- engine drives the listener + serial on configure
- `internal/plugins/geodns/server_test.go` -- in-process UDP+TCP resolve test
- `rfc/short/rfc7871.md`, `rfc/short/rfc1035.md`
