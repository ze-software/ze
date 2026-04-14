# 588 -- fw-5 Firewall and Traffic Control CLI Commands

## Context

Ze needed CLI commands for firewall and traffic control visibility: `ze firewall show`, `ze firewall counters`, and `ze traffic-control show interface <name>`. These are offline commands that query the kernel directly via the backend, no daemon required. The pattern follows `ze interface show`.

## Decisions

- CLI entry points at `cmd/ze/firewall/` and `cmd/ze/tc/`, over putting show commands in `cmd/ze/show/`, because firewall and traffic-control are domain commands with their own subcommand trees, not just show targets.
- Formatting functions in `internal/component/firewall/cmd/` and `internal/component/traffic/cmd/`, over putting them in the CLI entry point, because the formatting logic is reusable by the daemon's RPC handlers (online mode) and is independently testable.
- Table names displayed without `ze_` prefix, over showing the kernel name, because users configure bare names and should see bare names. The `stripPrefix` helper handles this.
- Rate formatting uses human-readable suffixes (10mbit, 100kbit), over raw bps numbers, because that matches the config syntax.

## Consequences

- `ze firewall show` and `ze traffic-control show interface <name>` work offline on Linux (load backend, query kernel, format output).
- On non-Linux: commands fail with "not supported on $GOOS" from the backend stub.
- Monitor command (nftables trace streaming) not implemented: requires interactive terminal and daemon event subscription, deferred.
- The formatting functions have full unit test coverage (13 tests across both packages).

## Gotchas

- goimports removes imports for packages it can't resolve (new packages in the same module). Add import and usage in the same Edit call. Even then, the auto-linter hook re-runs goimports, so the import must survive two passes.
- The `helpfmt` package uses `Page.Write()`, not a `Print()` function. Check the actual API before using it.

## Files

- `cmd/ze/firewall/main.go` -- CLI entry point: show, counters
- `cmd/ze/tc/main.go` -- CLI entry point: show interface
- `cmd/ze/main.go` -- Dispatch cases for firewall and traffic-control
- `internal/component/firewall/cmd/show.go` -- Table/chain/term/counter formatting
- `internal/component/firewall/cmd/show_test.go` -- 10 tests
- `internal/component/traffic/cmd/show.go` -- QoS/class/rate formatting
- `internal/component/traffic/cmd/show_test.go` -- 3 tests
