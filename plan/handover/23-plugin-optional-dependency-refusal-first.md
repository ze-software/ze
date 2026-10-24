# 23 Run the plugin optional-dependency refusal, so refuse-by-default can close

Owner order (2026-10-24): implement
spec-plugin-optional-dependency-refusal (status `ready` then; now closed)
in its own fresh session, now.

## Why it runs now

`plan/release/fixes/spec-refuse-instead-of-defaulting.md` (in progress, session
6b51c217) is implemented apart from six owned journal rows. Each needs the
refusal this spec designs: a feature that is not compiled into the binary must
be refused by name, never read as "not configured". Writing a second not-built
mechanism inside refuse-by-default would be layering (`ai/rules/no-layering.md`),
so refuse-by-default closes after this spec lands.

| Row (`plan/journal/silent-fall-through.md`) | Producer |
|---|---|
| 2026-10-15 `cos.Resolve` nil maps with no resolver | `internal/core/cos/cos.go` |
| 2026-10-21 FIB autoload starts with no writer | `Server.appendDataPlaneWriter`, `internal/component/plugin/server/startup_autoload.go` |
| 2026-10-20 plugins started after `ErrMissingDependency` | `getConfigPathPlugins` |
| 2026-10-20 MRT table dump and `show system` NTP block skipped when not built | mrt, ntp |
| 2026-10-20 ddos responder, ExaBGP bridge, flow export declare no BGP dependency | ddos, exabgp, flowexport |
| 2026-10-20 redistribute `destination`, ls_export `source` not checked against the registry | redistribute, ls_export |

The last four are already in that spec's "Journal Rows This Spec Closes". The
first two are refuse-by-default's: they call the refusal this spec provides.

## How to start

| Step | Action |
|------|--------|
| 1 | `git fetch`; `git rev-list --count HEAD..@{u}` must be 0 |
| 2 | claim spec-plugin-optional-dependency-refusal, then `/ze-implement` through subagents |
| 3 | Re-verify every producer at HEAD first: refuse-by-default changed nearby code, uncommitted at the time of writing |
| 4 | When it lands, tell session 6b51c217 (or the session that closes refuse-by-default) the name of the not-built error, so `cos.Resolve` and `appendDataPlaneWriter` call it |

## Shared-tree warnings

- refuse-by-default's large diff is uncommitted in main (radius, ldp, bgp
  rib/format/reactor, `pkg/plugin/rpc` `RemoteAS`, iface, plugin dispatch and
  `RawJSON`, le arch). Do not carry those hunks; re-read `git diff` per file
  before editing.
- `RawJSON.MarshalJSON` (`internal/component/plugin/types.go`) now refuses a
  top-level JSON scalar: a command answer must be an object, an array or null.
- `./le arch refuse-by-default check` (uncommitted, joins the verify stages with
  refuse-by-default) refuses a new line that answers a zero or a default where
  it has no answer; a real guard carries `// refuse-by-default: guards <Symbol> -- <why>`.
