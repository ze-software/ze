# 1059 -- archive-credential-sanitization

## Context

Archive location URLs configured under `system { archive { } }` could contain embedded credentials (`https://user:pass@host/path`). When archive operations failed, error messages included the raw URL, leaking the password into logs and CLI output. The same leak existed in the success message returned to CLI. Origin: VyOS vyos-1x T8956 (2026-06 fix for TftpC.upload() leaking URL credentials in error output).

## Decisions

- Added an exported `RedactURL(rawURL string) string` helper in the archive package over inline `url.Parse` + `Redacted()` at each site, because `cmd/archive.go` (a separate package) also needs it and 3 of the 6 sites lack a pre-parsed `*url.URL`
- Used `parsed.Redacted()` directly where a `parsed` variable was already available (ValidateLocation, archiveToLocation unsupported-scheme path) over calling the helper, to avoid redundant parsing
- Fixed all 6 sites (5 error messages, 1 success message) over the spec's original 4, because grep found two additional leak points (archiveToLocation unsupported-scheme error, cmd success message)

## Consequences

- Other packages logging URLs with credentials should follow the same `RedactURL` pattern, though a codebase-wide grep found no other user-configured URL sites at risk (install/appliance/test URLs are infrastructure-internal)
- The YANG schema defines `location` as `type string` with no constraint on embedded credentials; this is by design since HTTP basic auth via URL is a valid authentication method

## Gotchas

- The spec's original line numbers (143, 188, 196, 206) were accurate but it missed two sites: the unsupported-scheme error in `archiveToLocation` (line 216) and the success message in `cmd/archive.go` (line 131)
- The `ValidateLocation` unsupported-scheme error (`"unsupported archive scheme %q"`) only includes `parsed.Scheme` (e.g., "ftp"), not the full URL, so it was already safe despite appearing URL-adjacent

## Files

- `internal/component/config/archive/archive.go` -- added `RedactURL`, fixed 5 error messages
- `internal/component/config/archive/cmd/archive.go` -- fixed 1 success message
- `internal/component/config/archive/archive_test.go` -- added 7 redaction tests
