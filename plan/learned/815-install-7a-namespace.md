# 815: install-7a namespace migration (appliance -> install appliance)

## Context

spec-install-7a: move `cmd/ze/appliance/` under `cmd/ze/install/appliance/` to unify all installation and fleet management under `ze install`.

## Decisions

- **Copy, don't rename.** Go has no package rename tool. Copied all 38 files to new location, updated imports, deleted old import from `cmd/ze/main.go`. Old directory becomes orphaned (no imports) and should be deleted.
- **No root command registration at new location.** The old `register.go` registered "appliance" as a root command via `cmdregistry.RegisterRoot()`. At the new location, "appliance" is nested under "install", so root registration is removed. The old `register.go` in the orphaned directory still has the registration but is not imported.
- **Deprecated alias is pure dispatch.** `cmd/ze/main.go` "appliance" case prints a deprecation warning to stderr, then delegates to `zeinstall.Run(append([]string{"appliance"}, args[1:]...))`. No code duplication.
- **Help text updated to canonical path.** All `ze appliance` references in `usage()` changed to `ze install appliance`. Docs updated with deprecation note.

## Consequences

- `ze install --help` now shows local, remote, and appliance.
- Existing scripts using `ze appliance` continue to work with a stderr warning.
- The old `cmd/ze/appliance/` directory is orphaned and should be `git rm -r`'d.
- The `install/register.go` Subs field includes "appliance" for help completeness.

## Gotchas

- The auto-linter hook runs golangci-lint; if another agent is also running lint concurrently, the hook reports "parallel golangci-lint is running". This is benign and does not indicate a code problem.
- Blank imports of the appliance package from `main.go` are not needed since `install/main.go` imports it directly.

## Files

None recorded.
