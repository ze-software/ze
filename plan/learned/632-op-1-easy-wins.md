# 632 -- op-1-easy-wins

## Context

VyOS op-mode research (2026-04-18) produced a cross-vendor catalogue and a
Top-10 list of "Tier-1" operational commands ze was missing. Each was
characterised as an easy win: either a handler addition on top of an
already-shipped backend API, or an offline shell wrapper. The spec
delivered ten of the eleven; one (`show firewall group <name>`)
surfaced a real feature gap and landed as a deferral rather than a
hack. In parallel, the registration pattern for CLI commands was
restructured so every subcommand package owns its own `register.go`
and `help --ai` is driven by the registry instead of a hand-maintained
static list.

## Decisions

- **Domain-first naming, two reserved verb-first roots.** The catalogue
  now documents `<domain> <verb>` (`peer list`, `bgp summary`,
  `rib inject`, ...) as the default shape, with `show <what>` and
  `generate <what>` as the only verb-first exceptions. Rejected:
  verb-first everywhere. Reason: domain-first keeps the completion
  tree shallow; once the operator types `bgp`, the valid next tokens
  are exactly the verbs BGP offers. `show` and `generate` are kept
  because every vendor uses them and operators reach for them by
  muscle memory.
- **Bare verb argv branching instead of nested YANG containers.** The
  first cut of `bgp summary <family>` nested a `container family`
  under `summary` in YANG and registered a second `ze-bgp:summary-
  family` WireMethod. Reshaped to a single handler that branches on
  argv, one YANG container, one WireMethod. Matches the
  `show interface type <t>` precedent and removes the "extra keyword"
  the nested form would have imposed.
- **`cmdregistry` as a leaf package** (no dependencies on anything else
  under `cmd/ze/`). Rejected: putting the registry in `cmdutil`
  directly -- `cmdutil` imports `cli` for tree-walking, so any
  subcommand package (including `cli` itself) that registered back
  into `cmdutil` from its `register.go` would create an import cycle.
  `cmdregistry` is stdlib-only; `cmdutil.RegisterLocalCommand` became
  a thin passthrough to preserve the existing public API.
- **Per-package `register.go` with `init()` registration.** Rejected:
  main.go's existing centralised switch + `registerLocalCommands()`
  table. The switch stays (dispatch has package-specific quirks like
  storage setup), but every root command and local shortcut now
  registers itself, matching ze's unifying registration pattern
  ("Core discovers them through registries -- never imports
  directly"). Twenty-one packages got new `register.go` files.
- **`help --ai` reads the registry.** `cmd/ze/help_ai.go`
  `cliSubcommands()` rewritten to enumerate YANG verbs +
  `cmdregistry.ListRoot()`. Static root-command slice deleted. Adding
  a new subcommand package means adding its `register.go`; help picks
  it up automatically.
- **Family argument validation on `bgp summary`.** 32-char cap,
  `[a-z0-9/_-]+$` charset after `ToLower`. Length check runs before
  `ToLower` to prevent 1 MB inputs forcing a 1 MB allocation.
  Validation failures echo the (bounded) argument back via `%q`;
  unknown-but-valid-shape families go through the exact-or-reject
  path naming the set actually negotiated on the daemon.
- **Single-key wrapper maps for list responses.** `show interface type
  <t>` and `show interface errors` return `{"interfaces": [...]}`
  with no other top-level keys. Rationale: the `| table` pipe
  renderer (`internal/component/command/pipe_table.go`) single-key-
  unwraps wrapper maps to get at the slice. A 2-key or 3-key
  response (`interfaces` + `count` + `type`) would render as a
  key/value table with the slice collapsed to a JSON blob string;
  count is derivable via `| count`, and the type was known to the
  caller.
- **Firewall groups deferred with destination.** Adding the handler
  alone was rejected because ze's firewall model has no named-set
  primitive (no `address-group` / `network-group` / `port-group`
  YANG container, no parser). Deferral recorded in
  `plan/deferrals.md` pointing at `spec-firewall-groups`. Honesty
  ranked above "ship all 11".

## Consequences

- `plugin.PeerInfo` now carries `NegotiatedFamilies []string`. Any
  handler that needs per-family peer introspection can use this
  field without re-loading the capability struct. Populated
  unconditionally in `reactor_api.go` Peers().
- `cmd/ze/main.go` shrank by ~110 lines (centralised
  `registerRootCommands` + per-command table dropped). Subcommand
  packages grew by ~15 lines each (their `register.go`). Adding a
  new subcommand no longer requires editing `main.go`'s help list.
- The `| table` pipe renderer now has three list-returning consumers
  honoring the single-key unwrap contract: `showInterfaceBrief`,
  `showInterfaceByType`, `showInterfaceErrors`. Existing
  `handleBgpSummary` wraps `summary` which does unwrap, but the
  inner map has 7+ keys so tabulation falls back to a key/value
  rendering of the record (with `peers` collapsed). Same behaviour
  as before this batch; no regression.
- Future CLI commands should default to the single-key wrapper shape
  for list responses, or a flat record map for scalar responses.
  Anything else will render poorly through `| table`.
- `generate` is now a reserved verb-first root. If we ever add more
  reserved verbs (`reset`, `clear`?) the precedent is to update the
  catalogue's Naming Convention section first, then land the
  command.
- `show firewall group` is tracked in `plan/deferrals.md`. The
  named-set primitive is blocked on a YANG + parser extension that
  would also need to teach `MatchSourceAddress`, `MatchDestination-
  Address`, and the four *Port/*Interface match types to accept a
  group-name alternative to a literal value.

## Gotchas

- **`cmdutil` -> `cli` import edge is load-bearing.** I spent
  three write-file iterations discovering the cycle after trying to
  put the registry inside `cmdutil`. If a future refactor touches
  `cmdutil.RunCommand`, check whether the `cli.BuildCommandTree`
  import is still needed; if yes, the registry stays in
  `cmdregistry`.
- **`validateFamilyArg` ordering is intentional.** `len` check runs
  BEFORE `strings.ToLower`. Reversing the order turns a 32-byte
  charset-rejection into a 1-MB allocation followed by a 32-byte
  charset-rejection. Marked with a "Do not reorder" comment.
- **Table renderer single-key unwrap is only triggered by exactly one
  top-level key.** Add `count` or `type` next to `interfaces` and
  `| table` falls back to rendering the record with the slice as
  JSON. If a future handler wants both a slice and a count visible
  in tables, put the count inside the single-key inner map
  (`{"interfaces": {"rows": [...], "count": N}}`) -- that still
  triggers unwrap to the inner map, which then renders as a key/
  value table with the rows-string. Likely still not ideal; usually
  drop the count and rely on `| count`.
- **`handleBgpSummaryFamily` removed; tests kept their names.** The
  four tests still have `Family` in their names
  (`TestBgpSummary_FilterByFamily`, `_FamilyShorthand`,
  `_UnknownFamilyRejects`, `_FamilyArgValidation`) even though the
  handler is unified `handleBgpSummary`. Intentional: the test
  names describe the feature (family filtering), not the handler.
- **`ze host show memory` and `show system memory` return overlapping
  data.** A parallel session added `show host *` RPCs that surface
  `host.DetectMemory()` / `DetectCPU()` directly; my `show system
  memory` / `cpu` nest the same output under a `hardware` subobject
  next to Go-runtime stats. Both are correct; the catalogue notes
  column points to the other command for context. Not a bug, but
  worth knowing when writing scrapers.
- **`make ze-verify-fast` on Darwin requires `flock`.** The
  `verify-lock.sh` wrapper fails without it. Workaround: `make _ze-
  verify-fast-impl` runs the underlying target directly. The real
  fix is either installing util-linux's `flock` via brew or
  patching `verify-lock.sh` to no-op on Darwin.

## Files

**Created:**
- `cmd/ze/internal/cmdregistry/registry.go` (new leaf package; all three registries)
- `cmd/ze/diag/diag.go` (ping, traceroute, wireguard keypair)
- `cmd/ze/diag/diag_test.go`
- `cmd/ze/diag/register.go`
- `cmd/ze/bgp/register.go`, `cli/register.go`, `completion/register.go`, `config/register.go`, `data/register.go`, `environ/register.go`, `exabgp/register.go`, `firewall/register.go`, `iface/register.go`, `init/register.go`, `l2tp/register.go`, `passwd/register.go`, `plugin/register.go`, `resolve/register.go`, `schema/register.go`, `signal/register.go`, `sysctl/register.go`, `tacacs/register.go`, `tc/register.go`, `yang/register.go` (19 subcommand `register.go` files)
- `internal/component/cmd/show/system.go` (show system memory/cpu/date handlers)
- `internal/component/cmd/show/system_test.go`
- `docs/guide/command-catalogue.md` (cross-vendor tracking doc + naming convention)

**Modified:**
- `internal/component/plugin/types_bgp.go` (added `NegotiatedFamilies`)
- `internal/component/bgp/reactor/reactor_api.go` (populates it)
- `internal/component/bgp/plugins/cmd/peer/summary.go` (unified handler, argv branching, exact-or-reject guard)
- `internal/component/bgp/plugins/cmd/peer/schema/ze-peer-cmd.yang` (updated summary description)
- `internal/component/bgp/plugins/cmd/peer/summary_test.go` (4 new tests + renames)
- `internal/component/cmd/show/show.go` (type/errors branches, system RPC registrations, single-key list responses)
- `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` (system containers + interface type/errors; shared-WireMethod note)
- `cmd/ze/main.go` (registerLocalCommands trimmed; registry fallback in dispatch; blank import of diag/host)
- `cmd/ze/help_ai.go` (cliSubcommands reads from cmdregistry)
- `cmd/ze/internal/cmdutil/cmdutil.go` (delegates to cmdregistry)
- `cmd/ze/internal/cmdutil/cmdutil_test.go` (uses cmdregistry public API)
- `ai/patterns/cli-command.md` (new "Command Registration (BLOCKING)" section)
- `ai/patterns/registration.md` (CLI Local Command Registry entry updated)
- `docs/guide/command-reference.md` (ze show system section, ze interface extensions, bgp summary `<afi/safi>` row)
- `docs/guide/README.md` (catalogue link)
- `plan/deferrals.md` (firewall groups entry)
