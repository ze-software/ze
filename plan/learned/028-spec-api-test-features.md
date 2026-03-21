# 028 — API Test Features

## Objective

Implement remaining API features (teardown, notification, watchdog) to reach ExaBGP functional-test parity, targeting 12/14 API tests passing.

## Decisions

- `check` test failure was caused by API version mismatch: default is version 7 (ze format), but the test expects version 6 (ExaBGP format). Fixed by adding `version 6;` to the config content block.
- Multi-session qualifiers (`announcement` test) are not supported by design — ZeBGP uses a different multi-peer architecture.

## Patterns

- API bindings from templates are now properly inherited (previously broken; bug caused `mup4.conf` and `mup6.conf` to reference wrong `.run` files).

## Gotchas

- MUP API (`mup4`, `mup6`) was not in scope but was encountered — `parseSAFI()` only supported unicast/nlri-mpls/mpls-vpn; MUP needed a separate spec.
- Template match inheritance was silently not applying API bindings (`bgp.go:1200-1206`).
- The functional test reporter had a bug where all messages with `1:` index merged into one in diagnostics — actual comparison was correct, only display was wrong.

## Files

- `internal/component/plugin/command.go` — teardown handler
- `internal/component/plugin/watchdog.go` — watchdog subsystem
- `internal/reactor/reactor.go` — teardown/watchdog integration
