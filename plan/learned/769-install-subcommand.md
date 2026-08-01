# 769: ze install subcommand (fork pattern)

## Context

spec-install-4: provisioning server subcommand that generates ze config from CLI flags and forks `ze -` to start DHCP+PXE, TFTP, and image servers.

## Decisions

- **Subcommand, not separate binary.** `ze install serve` is dispatched from `cmd/ze/main.go` like all other subcommands. No `cmd/ze-install/` binary. Single binary simplifies deployment on gokrazy appliances.
- **Fork pattern, not hub.Run() import.** Generates config string, forks self via `os.Executable()`, pipes config to child stdin with NUL sentinel. Same pattern as `ze-chaos | ze -` but self-contained. No hub import, no ephemeral zefs.
- **NUL sentinel.** Config ends with `\x00` so ze can start parsing without waiting for EOF. Pipe stays open; EOF signals shutdown. Pattern established by ze-chaos.

## Consequences

- ze install has no dependency on hub internals. Config format is the only coupling.
- Signal forwarding (SIGTERM/SIGINT) from parent to child is required for clean shutdown.
- The child ze handles its own storage resolution via the stdin config path.

## Gotchas

- `fmt.Fprintf` to `strings.Builder` is blocked by the `block-sprintf-new.sh` hook even on cold paths. Use `b.WriteString()` chains instead.
- `fmt.Sprintf` for error messages is also blocked. Use `fmt.Errorf` (allowed) or `errors.New`.
- `exec.Command` is blocked by linter; must use `exec.CommandContext`.
- `TestAvailablePlugins` in `cmd/ze/main_test.go` maintains a hardcoded expected plugin list. New plugins from specs 1-3 (imageserver, tftpserver) must be added to this list.

## Files

None recorded.
