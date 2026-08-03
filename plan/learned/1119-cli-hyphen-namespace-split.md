# cli-hyphen-namespace-split

Added the R9 CLI naming convention (compound token vs namespace split) and its
grammar-gate check, then migrated every command it flagged from hyphenated
compound tokens to two-token namespace forms.

## The convention (R9)

A hyphen in a command token joins **one indivisible name** (term of art:
`as-set`, `graceful-restart`, `adj-rib-in`; a single attribute: `max-prefix`).
When the left part is an **object/namespace** with members, it is two tokens and
the left becomes a container node (object-rooted; the tree mirrors the plugin
tree). `show traffic stat`, not `show traffic-stat`.

Rule prose: `ai/rules/cli.md` "Compound Token vs Namespace Split".
Check: `grammar.CheckSiblings` (R9) in `internal/component/command/grammar/checker.go`,
wired into the static gate `scripts/checks/cli_grammar.go`. R9 is the one rule
that needs sibling context, so it runs ONLY in the static gate (per-command
registration cannot see siblings). It fires only when the colliding namespace
literally exists as a sibling, so genuine compounds are never flagged.

## Migration mechanics (what changed, what stayed)

- Renaming a command = moving/renaming its YANG `ze:command` container so the
  space-joined path changes. **Wire methods do NOT change** (handlers are keyed
  on the wire method, not the path). This kept `plugin/all` snapshots stable and
  handlers untouched.
- Multiple `-cmd` modules merge a same-path container (traffic-cmd + trafficusage
  both declare `container traffic`); use the **identical `description`** on the
  shared namespace container across modules or startup warns
  `YANG command description mismatch`.
- The `pendingNamespaceSplit` map in `cli_grammar.go` is the staged-migration
  ledger: list a shipped R9 violator there so the gate reports it as debt without
  blocking; delete the entry when the command is renamed. It is now empty.

## Traps for the next agent

- **Literal command-path couplings change in lockstep with the path**, and they
  are scattered: a streaming-handler registration
  (`trafficstat/cmd/traffic.go` `RegisterStreamingHandler`/`MonitorProvider.Prefix`),
  a web command builder (`web/page_tools.go`, `web/handler_l2tp.go`), a plugin
  `OnExecuteCommand` switch + `CommandDecl` + `PluginCommand` (`policyroute`), and
  an internal cross-plugin caller (`ddos/detect/characterize.go`). Grep every
  literal, not just the YANG.
- **A bare keyword container (no YANG leaf) passes its trailing value as
  `args[0]`; a `leaf` routes the value to `ctx.Selector` instead.** The l2tp
  teardown handlers read `args[0]` (`parseIDArg(args)`), so dropping the
  redundant `teardown` word to `clear l2tp session id <id>` used a bare `id`
  container (NOT a leaf) to keep the handler working. Verified: `reason`/`cause`/
  `actor` keyword args still parse as trailing args.
- **A config name can be overloaded with a command name.** `flow-export` is both
  a config container (`flowexport-conf.yang`) and a shipped command; only the
  command container moved to `show flow export`. The config stayed. Blanket
  find/replace would corrupt config.
- **Offline `ze <subsystem>` CLIs forward text to the daemon** and may omit the
  verb: `l2tp/cli/show.go` forwarded a verb-less `l2tp tunnel ...`. After the
  rename it now prepends `clear` (matching `cmdShow`'s `show` prefix). This SSH
  path has no in-tree functional test.
- **Wire-method vs path can legitimately diverge.** The memory swap kept wire
  methods stable, so `show system memory` (OS view) maps to
  `ze-show:system-memory-map` and `show runtime memory` (Go allocator) to
  `ze-show:system-memory`. Internal-only; documented.
- **The command name also appears in prose the compiler/tests do not check:**
  YANG module and leaf `description` strings, code comments, `docs/`, digests, and
  `ze:related` workbench annotations. The literal-coupling grep above (dispatch
  strings) is NOT enough -- a `/ze-review` completeness pass caught stale
  `show bgp-health` / `show flow-recent` / `clear l2tp ... teardown` in
  `ze-peer-cmd.yang` description, `ze-flowexport-conf.yang` description,
  `command-catalogue.md`, and comments AFTER the migration commit. Grep every
  string form of the old name, not just code. Keep the old spelling only in
  `revision "..."` history logs and the R9 rule's before/after examples.
- **`ze:related command="..."` is production-validated, not cosmetic.**
  `ValidateRelatedAgainstCommandTree` (`config/related.go`) is called from
  `config/yang_schema.go` at schema load and walks the tree token-by-token over
  the static prefix (`commandTreeHasPath`). A rename that leaves an old
  `ze:related` command (e.g. `show bgp-health` when the tree now has
  `show`->`bgp`->`health`) is a real schema-load error, so the ze:related must be
  renamed in lockstep. Unit coverage: `config/related_test.go`
  `TestRelatedExtension_RejectsCommandNotInTree` (stub tree mirrors the nested
  real structure).

## Verification that mattered

The grammar gate (`valid=True`, 0 R9, 0 pending) proves the tree; the plugin
functional suite (`make ze-plugin-test`, 503/503) proves end-to-end dispatch of
the restructured commands (`clear l2tp session id <id>`/`all`, `show system
memory`). Unit tests + gate alone would not have proven the bare-`id`-container
arg passing.

## QEMU end-to-end (Linux-only renamed commands)

Two renamed commands are `option=needs-linux`, so they SKIP the native darwin
suite and are validated only in the QEMU Alpine VM (`make
ze-qemu-needs-linux-test`, kernel 6.12.13):

- `show flow recent` (`test/plugin/ddos-flow-recent.ci`, index 153): **PASS** in
  QEMU. It uses the in-daemon `ze_api` external-plugin `dispatch-command` pattern
  (not `ze cli -c`), asserts dispatch reachability + list shape + `dst` filter +
  usage guard, so it does not depend on real traffic counters. `characterize.go`
  querying the renamed `show flow recent` is proven here end-to-end.
- The surrounding ddos flood tests (`ddos-detect-*`, `ddos-direction`,
  `ddos-policy`, `ddos-bps-amplification`) FAIL in QEMU, but that is the
  pre-existing firewall-concurrency deadlock / BPS-trigger-not-firing on the old
  Alpine kernel (documented in `plan/deferrals.md` under
  spec-ddos-direction-allowlist), NOT the rename.

`show policy routes` CANNOT be validated in the Alpine QEMU VM and its
`skip-os:value=darwin` + `skip-env:var=ZE_QEMU` is correct -- do NOT "fix" it to
`option=needs-linux` (it will FAIL in QEMU, not skip). Proven by temporarily
flipping it and running `make ze-qemu-debug NOBUILD=1 RUN='bin/ze-test-linux-arm64
bgp plugin -v 441'`: the daemon aborts at firewall config-apply with
`nftables apply: firewallnft: flush: netlink receive: operation not supported`
(nft genl unsupported on the stock Alpine kernel), so it never reaches command
dispatch. The command needs a native Linux CI host; its dispatch is otherwise
covered by `cmd_show_test.go` + the grammar gate.

Harness gotcha found while doing this: the ze-test verbose flag must come BEFORE
the index (`bgp plugin -v 441`), because Go's `flag.Parse` (`cmd_bgp.go`
`fs.Parse(args[1:])`) stops at the first positional. The `ze-qemu-debug` help
example `... 79 -v` in `mk/test-integration.mk` is stale and errors with
`test "-v" not found`.

## Files

None recorded.
