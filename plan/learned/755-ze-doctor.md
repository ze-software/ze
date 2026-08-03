# 755: ze doctor System Readiness Checks

## Context
AI agents and operators had no way to verify that a host was ready to run ze before starting the daemon. Missing config, expired TLS certs, absent kernel modules, and unreachable VPP sockets would cause silent failures or cryptic errors at runtime.

## Decision
Added `ze doctor` as an offline command (no running daemon required) that reads config, probes the host, and reports structured diagnostics. Two output modes: human-readable text and `--json` for agent consumption.

Key choices:
- **Diagnostic code taxonomy**: codes like `doctor-storage-unavailable`, `doctor-tls-expired`, `doctor-iface-missing` from the `diagnostic` package. Lower-kebab-case to match the agent tooling convention.
- **Severity model**: error = cannot start, warning = will start but degraded. The `ready` flag in JSON output is false if any error-severity diagnostic exists.
- **Platform split**: `checks_linux.go` for kernel modules (`/proc/modules`), interface link state (`/sys/class/net/`), XFRM support. `checks_other.go` stubs for macOS/test builds.
- **Shared resolve package**: `cmd/ze/internal/resolve/` extracts storage and config path resolution previously duplicated between `cmd/ze/main.go` and `cmd/ze/doctor/`.
- **Extensibility rule**: `ai/rules/repo-maintenance.md` requires any future feature that adds a runtime dependency to include a corresponding doctor check.

## Consequences
- Agents call `ze doctor --json` before daemon start and parse the result.
- 36 tests covering normal, degraded, and failure cases.
- Interface checks verify both existence and link-up state (Linux only).
- TLS checks split into `doctor-tls-expired` (valid PEM, past expiry) and `doctor-tls-invalid` (unparseable PEM).

## Gotchas
- `checks_linux.go` reads `/proc/modules` line-by-line; module names are the first whitespace-delimited field, not always the full `lsmod` format.
- Port probing uses `net.DialTimeout` with a short timeout; firewalled ports show as errors even if the service would bind fine. This is intentional (better to warn than miss a conflict).
- The `resolve` package extraction changed `cmd/ze/main.go` imports, which touches the critical startup path.

## Files

None recorded.
