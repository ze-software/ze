# 441 -- GR Restart Marker

## Context

Ze implemented the Receiving Speaker side of RFC 4724 (Graceful Restart) but had no way to distinguish a restart from a cold start. Every startup advertised R=0 in GR capabilities, so peers always treated ze as a fresh speaker and flushed routes. The Restarting Speaker side needed a mechanism to bridge process boundaries: shutdown writes a marker, startup reads it and sets R=1.

## Decisions

- Chose a zefs marker file (`meta/bgp/gr-marker`) over an environment variable or CLI flag, because zefs survives process exit and doesn't require the operator to pass state through the process manager.
- Chose time-bounded R=1 (RestartUntil deadline) over a boolean flag, because R=1 must expire after the restart window. No timer goroutine needed: `time.Now().Before(deadline)` naturally expires.
- Chose engine-only marker logic over plugin involvement, because plugins may be remote (TCP) or non-Go (Python). Exposing zefs to plugins would require RPC protocol changes for no benefit.
- Chose copy-and-modify in `getPluginCapabilities()` over mutating stored capabilities, because the GR plugin's original capability bytes must stay at R=0 for the next restart cycle.
- Chose `restartFunc` closure pattern over reactor method, because the SSH server and CLI model don't need to know about CapabilityInjector or zefs. Same pattern as existing `shutdownFunc`.

## Consequences

- `ze signal restart` now writes a GR marker and shuts down. `ze signal stop` shuts down without a marker. The operator decides intent.
- Interactive CLI has `restart` (with marker) and `stop` (without marker) commands, both with confirmation prompts.
- Future work: F-bit (per-family forwarding state), Selection Deferral Timer, supervisor crash recovery are out of scope for this spec.
- The `inspect-open-message` ze-peer feature proved essential for verifying R=1 in OPEN wire bytes.

## Gotchas

- `reject=stderr:contains=` is silently ignored by the .ci test runner: (a) `reject=` lines inside `stdin=peer:` blocks are not parsed (only `expect=`/`action=` are), (b) `parseReject` only supports `pattern=`, not `contains=`. Must use `reject=stderr:pattern=` outside the peer block.
- `cli.Model.Update()` uses a value receiver (bubbletea pattern). Function pointer fields survive the copy chain, but wiring `.et` test infrastructure for lifecycle callbacks (`SetRestartFunc`/`SetShutdownFunc`) revealed the callbacks are lost somewhere in the headless model update chain. Root cause not yet identified. The `.et` infrastructure additions are correct but the restart confirmation test is pending.
- The OPEN hex for GR with R=0 ends in `020440020078`. For R=1 it ends in `020440028078`. The only difference is byte 0 of the GR value: `00` vs `80`.

## Files

- `internal/component/bgp/grmarker/grmarker.go` -- marker package (Write, Read, Remove, MaxRestartTime, SetRBit)
- `internal/component/bgp/grmarker/grmarker_test.go` -- 21 unit tests
- `internal/component/bgp/reactor/peer.go:480` -- R-bit injection in getPluginCapabilities
- `internal/component/bgp/reactor/reactor.go:154` -- Config.RestartUntil field
- `cmd/ze/hub/main.go:173` -- startup marker read/remove
- `internal/component/bgp/config/loader.go:634` -- restartFunc closure
- `test/plugin/gr-marker-restart.ci` -- R=1 OPEN wire verification
- `test/plugin/gr-marker-expired.ci` -- expired marker rejection
- `test/plugin/gr-cli-restart.ci` -- lifecycle dispatch test
- `internal/component/cli/testing/headless.go` -- SetRestartFunc/SetShutdownFunc wrappers
- `internal/component/cli/testing/runner.go` -- option=lifecycle:mode=wired support
