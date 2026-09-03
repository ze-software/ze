# System Update: the backend, and the version check

`docs/architecture/core-design.md` Section 20 states what the update surface is.
This page states why it has that shape, and what the version check inside it
must keep doing.

<!-- source: internal/component/config/system/backend.go -- UpdateBackend, the factory registry, ActiveBackend -->
<!-- source: internal/component/config/system/update.go -- UpdateChecker -->

## The backend

Ze had two independent globals: `UpdateChecker` (a passive version check) and
`SelfUpdater` (download, verify, stage, restart). Neither carried backend
identity or platform awareness, so a gokrazy appliance, where the image is
managed by gokrazy, answered a firmware command with "not configured".

**One `UpdateBackend` interface with a factory registry.** `NewBackend(platform,
cfg, opts)` selects at startup from `host.DetectPlatform()`. Factories register
through `init()`, the same pattern the traffic backends and the firewall
backends use.

**`BackendName` is a string type, not an enum**, because the value appears
directly in JSON output.

**The implementation is split by build tag.** `backend_ze_distro.go` (tag
`ze_distro`) wraps `UpdateChecker` and `SelfUpdater`. `backend_ze_appliance.go`
(tag `!ze_distro`) is a stub that answers "unsupported in minimal build". A
minimal build then compiles without the self-update dependency chain and still
registers the `ze-self-update` name.
<!-- source: internal/component/config/system/backend_ze_distro.go -- newZeBackend -->
<!-- source: internal/component/config/system/backend_ze_appliance.go -- minimal-build stub -->

**The gokrazy backend probes the management socket over HTTP rather than
importing the gokrazy management library.** The socket path and the auth header
live in `internal/core/gokrazyutil/`, so the backend does not couple to the
gokrazy web proxy.
<!-- source: internal/component/config/system/backend_gokrazy.go -- gokrazyBackend -->
<!-- source: internal/core/gokrazyutil/gokrazyutil.go -- socket path and auth header -->

**Platform detection runs once, through `sync.OnceValues`.** The platform does
not change at runtime.

**Doctor checks are platform-aware.** The writable-binary warning is skipped on
gokrazy, and an update-check config present on gokrazy raises a warning, because
gokrazy ignores it.

## The version check

**It is a system config extension, a YANG container beside host, dns and
tuning. It is not a component.** One goroutine and an HTTP fetch do not earn
one.

**It reports through the report bus**, using `report.RaiseWarning` and
`report.ClearWarning`, not a custom event type. `show warnings` and `show system
update` both surface it with no extra wiring.

**Version comparison is lexicographic, not semver.** Ze versions are `YY.MM.DD`,
where lexicographic order is correct. A non-numeric first character on either
side, such as the `dev` of a test build, short-circuits to "not newer". Without
that guard every comparison in a test build fails.

**Each request carries an `X-Ze-Arch` header**, so the server answers 404 for a
mismatched architecture. `ze update-serve` is the build-infrastructure side: it
serves `/version.json` with arch validation and `/<goos>/<goarch>` for the
binary.

**The router's web UI does not expose its version.** This is deliberate
hardening against fingerprinting.

## The URL rule, and the way it was got wrong

`ValidateUpdateCheckURL` requires HTTPS. Plain HTTP is accepted only from the
host itself: `127.0.0.1`, `::1` and `localhost`. The rule is
`config.ValidateFetchURL`, which every URL Ze reads a file from answers to, so
the update manifest and a RIR delegation mirror are held to one rule rather
than to two spellings of it.

The host is compared after parsing, never by prefix. A prefix test reads
`http://127.0.0.1.example.com` as loopback, which is a host the operator does
not own. This function carried that test until 2026-09-03, on the `127.0.0.1`
branch, and it guards a self-update manifest's download URL as well as the
`update-check` leaf. A port is part of the address rather than of the host, so
`http://127.0.0.1:8080` is the same host as `http://127.0.0.1`.
<!-- source: internal/component/config/validators.go -- ValidateFetchURL -->
<!-- source: internal/component/config/system/update.go -- ValidateUpdateCheckURL -->

**The reload path validates too.** It did not at first, so a SIGHUP could
install an HTTP URL that the initial load would have refused.

`fetchVersion` checks the HTTP status code before it parses. Without that check
a 404 or a 500 surfaces as a confusing JSON parse error.
