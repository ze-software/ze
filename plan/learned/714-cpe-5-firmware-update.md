# 714 -- Firmware update check (cpe-5)

## Context

Ze needed a way for deployed routers to detect when newer firmware is available
without performing the update itself (gokrazy handles actual image updates).
The requirement came from VyOS-style `system { update-check { url ... } }` config.

## Decisions

- Implemented as a system config extension (YANG container alongside host, dns, tuning),
  not a standalone component. The feature is small enough (one goroutine + HTTP fetch)
  that a dedicated component would be over-engineering.
- Uses the report bus (`report.RaiseWarning` / `report.ClearWarning`) rather than a
  custom event type. This means `show warnings` and `show system update` both surface
  the information without additional wiring.
- Version comparison is lexicographic, not semver. Ze uses `YY.MM.DD` date-based
  versioning where lexicographic ordering is correct. Non-numeric prefixes ("dev")
  on either side short-circuit to "not newer."
- The router's web UI intentionally does NOT expose its version (security hardening
  against fingerprinting).
- `ze update-serve` is a separate CLI command for build infrastructure. It serves
  `/version.json` with arch validation and `/<goos>/<goarch>` for binary download.
- Each check request sends `X-Ze-Arch` header so the server can reject mismatched
  architectures with a 404.
- URL validation requires HTTPS; HTTP is allowed only for `127.0.0.1` and `localhost`
  (with proper boundary checks to prevent `localhost.evil.com` bypass).

## Mistakes

- Initial `isNewer` didn't account for `version.Release()` returning "dev" in test
  builds, causing all comparisons to fail. Fixed by guarding against non-numeric
  first characters on both sides.
- Initial `fetchVersion` didn't check HTTP status code, causing confusing JSON parse
  errors on 404/500 responses.
- Initial `ValidateUpdateCheckURL` used `HasPrefix("http://localhost")` which would
  accept `http://localhost.evil.com`. Fixed to require `http://localhost/`,
  `http://localhost:`, or exact `http://localhost`.
- Reload path initially missed URL validation, allowing an HTTP URL to bypass the
  HTTPS requirement after SIGHUP.

## Patterns Reused

- System config extraction: same `GetContainer` + `Get` pattern as dns, peeringdb,
  tuning, console, conntrack.
- Show RPC: same `pluginserver.RPCRegistration` + `plugin.Response{Data: map}` pattern
  as system-memory, system-cpu, system-conntrack.
- Daemon lifecycle: same startup/reload/shutdown pattern as conntrack and console
  (start at config load, restart on reload, stop on shutdown).
- Report bus integration: same `report.RaiseWarning`/`ClearWarning` with source/code/subject
  triple used by BGP and config subsystems.

## Files

None recorded.
