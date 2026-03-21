# 403 -- RPKI Config Wiring

## Objective

Wire the bgp-rpki plugin into the config pipeline (OnConfigure + WantsConfig) and prove end-to-end functionality with exhaustive functional tests and a mock RTR server.

## Decisions

- **Mock RTR server as test tool:** `cmd/ze-rtr-mock/` is a lightweight Go binary that serves fixed VRPs via RFC 8210 protocol. Configurable via `--port` and `--vrp` flags. Enables deterministic functional testing without real RPKI infrastructure.
- **Config parsing in separate file:** `rpki_config.go` extracts cache-server list from YANG JSON config. Clean separation from plugin logic.
- **13 functional tests covering all validation paths:** Valid, Invalid, NotFound, passthrough, multi-prefix, AS_SET, maxLength, timeout, cache connect, cache update, plus 3 event emission tests.

## Patterns

- **OnConfigure callback pattern:** Same as GR plugin -- register WantsConfig in registration, parse JSON in OnConfigure, start sessions from parsed config.
- **Functional test pattern for RPKI:** Start ze-rtr-mock (background) with specific VRPs, start ze-peer (background), run ze (foreground) with RPKI config, Python test plugin queries adj-rib-in state via dispatch-command.

## Gotchas

- Config must include both the rpki plugin AND the adj-rib-in plugin. Without adj-rib-in, the validation gate commands have no handler.
- RTR session lifecycle is per-cache-server. Each gets its own long-lived goroutine with retry/refresh/expire timers per RFC 8210.
- Mock RTR server must respond to both Reset Query and Serial Query to support cache-update test.

## Files

- `internal/component/bgp/plugins/rpki/rpki_config.go` -- config parsing
- `internal/component/bgp/plugins/rpki/rpki.go` -- OnConfigure + WantsConfig + session startup
- `cmd/ze-rtr-mock/main.go` -- mock RTR cache server
- `test/plugin/rpki-validate-accept.ci` -- Valid route forwarded
- `test/plugin/rpki-validate-reject.ci` -- Invalid route blocked
- `test/plugin/rpki-validate-notfound.ci` -- NotFound route forwarded
- `test/plugin/rpki-passthrough.ci` -- no RPKI, routes flow through
- `test/plugin/rpki-multi-prefix.ci` -- mixed validation per prefix
- `test/plugin/rpki-as-set.ci` -- AS_SET origin yields Invalid
- `test/plugin/rpki-maxlength.ci` -- maxLength exceeded
- `test/plugin/rpki-timeout.ci` -- fail-open timeout
- `test/plugin/rpki-cache-connect.ci` -- RTR connection + VRP count
- `test/plugin/rpki-cache-update.ci` -- RTR cache sync
