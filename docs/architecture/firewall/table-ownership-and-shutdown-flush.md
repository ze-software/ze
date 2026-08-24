# Firewall Table Ownership and Shutdown Flush

Seven producers register nft tables with one shared registry: the firewall
engine, copp, policy-routes, ddos-local, firewall-irr, the FlowSpec bridge and
the anomaly-shape responder. All of them run in process. This document holds
the three rules that keep their tables visible, reconciled and removed, the
traps a test plugin met while proving the last one, and the one-time removal
that clears what a build before Rule 2 was enforced left in the kernel.

<!-- source: internal/component/firewall/registry.go -- ApplyAll, FlushAllTables -->
<!-- source: internal/component/firewall/backend.go -- LoadBackend, loadBackendLocked -->

## Rule 1: the backend loads on demand

A config with `control-plane-protection` and no `firewall {}` block never loaded
the nft backend, so `ApplyAll` returned "backend not loaded" and copp's apply
failed. `ApplyAll` now loads the OS-default backend when a producer registers
tables and no firewall section loaded one. `LoadBackend` was split so `ApplyAll`
can load atomically while it holds the backend mutex. The fix sits at the shared
altitude, so every registry consumer gets it.

## Rule 2: a producer's table name carries the `ze_` prefix

The nft backend uses `firewall.Table.Name` verbatim and adds nothing. It
reconciles and deletes only kernel tables whose name starts with `ze_`. A table
registered under a bare name is therefore outside every sweep: a withdraw leaves
it installed, and the next reconcile adds a second copy of each rule to it
instead of replacing it.

**The rule is now enforced, not conventional.** `RegisterTables` refuses a name
that does not carry the prefix and returns an error to the producer, so a bare
name never reaches the kernel and the producer is told rather than losing its
rules quietly.

| Producer | Name | Where it is set |
|----------|------|-----------------|
| firewall engine | `tableNamePrefix + name` | `ParseFirewallConfig` |
| policy-routes | `ze_pr` | `policyRoutingTable` |
| copp | `ze_copp` | `coppTableName` |
| ddos-local | `ze_ddos-local` | `tableName` |
| firewall-irr | `ze_irr_iface`, and config names via `tableNamePrefix` | `ifaceTableName` |
| FlowSpec bridge | `ze_flowspec` | `tableName` |
| anomaly-shape | `ze_anomaly-shape`, `ze_anomaly-shape6` | `tableNameV4`, `tableNameV6` |

<!-- source: internal/component/firewall/registry.go -- RegisterTables -->
<!-- source: internal/plugins/copp/translate.go -- coppTableName -->
<!-- source: internal/plugins/ddos/local/responder.go -- tableName -->
<!-- source: internal/plugins/flowspec-firewall/state.go -- tableName -->
<!-- source: internal/plugins/anomaly/shape/match.go -- tableNameV4 -->

Four producers shipped bare names: copp, ddos-local, the FlowSpec bridge and
anomaly-shape. Each leaked its table. The other three carried the prefix from
their first commit, so those four are the whole population and `legacyTables`
holds an entry for each of them: `copp`, `ddos-local`, `flowspec`,
`anomaly-shape` and `anomaly-shape6`. `ddos-local` picks its family from the
victim prefix and its chain from the hook, so it is one name in two families
with two chains.

## Rule 2a: the tables those builds left are removed once

Renaming a producer does not reach a table already in the kernel, so an upgraded
router keeps enforcing the rules its previous build installed, for the life of
the box. `legacy_tables.go` names what those builds wrote and the backend
deletes each one, logging that it did.

**Once means once.** The removal runs on the FIRST reconcile of the process and
on no later one, gated on `LegacySweepPending`. Every current producer carries
the prefix and `RegisterTables` refuses a bare name, so a table that appears
under a legacy name while ze is running belongs to somebody else by
construction. This is a migration with an end, not a standing rule that deletes
a matching table in every release from here on.

The decision needs three facts together -- the name, the address family, and
every chain the table holds -- because these names are ordinary words and
another tool that programs nftables can use one of them. Deleting that tool's
table would be worse than the stale rule the removal exists to clear.

The chain test reads names and nothing else: it does not read the hook, the type
or the priority. `ingress` and `input` are generic, so a foreign table whose
chains happen to carry only names ze used still matches. The one-shot gate is
what bounds that residual, because it limits the candidates to tables already in
the kernel when ze started.

One case needs its own trigger. The removal runs inside `Backend.Apply`, and
`ApplyAll` returns before it reaches a backend when the merged desired set is
empty and none is loaded. A box with no `firewall {}` section, whose FlowSpec
source has stopped announcing, therefore gets no reconcile that could reach the
stale table. An owner whose own registration is event-driven -- the FlowSpec
bridge, the anomaly-shape responder, ddos-local -- runs one empty reconcile at
startup while the removal is pending, and `ApplyAll` loads a backend for that
one reconcile. copp needs no such trigger: it registers its table whenever it is
configured, so it always drives a reconcile of its own.

Remove an entry from `legacy_tables.go` when no supported upgrade path starts
from a build that wrote it. The file is written to be deleted.

<!-- source: internal/component/firewall/legacy_tables.go -- legacyTables, IsLegacyTable, legacySweepPending -->
<!-- source: internal/plugins/firewall/nft/backend_linux.go -- Apply, isLegacyTable -->
<!-- source: internal/plugins/ddos/local/register.go -- the startup empty reconcile -->

## Rule 3: teardown belongs to the firewall engine, gated by config

`ProcessManager.Stop` cancels every plugin at once with no dependency order, so
copp's own post-`Run` withdraw raced the firewall engine's `CloseBackend`. Once
the backend closed, the table could not be removed: a fresh backend refuses to
delete a `ze_` table that is not in its own applied set.

The `flush-on-shutdown` boolean leaf, default true, drives `FlushAllTables` from
the engine's post-`Run` path. One ordered actor holds a live backend and runs
before `CloseBackend`, so there is no race. copp's own withdraw was removed.
Removing the config while the daemon runs still withdraws through
`OnConfigApply`.

**The default behavior changed.** An orderly stop now removes every ze-owned nft
table. Set `firewall { flush-on-shutdown false; }` to use ze as a one-shot
provisioner that programs rules and exits. This is process lifecycle only and is
unrelated to BGP graceful restart, which never restarts the daemon.

**Keep-on-crash is automatic.** The teardown runs on the orderly-stop path only.
SIGKILL, a panic and a power loss all bypass it, so the tables persist.

The firewall engine parses `flush-on-shutdown` only when a `firewall` section is
present. A copp-only config keeps the package default of true. `atomic.Bool`
zeroes to false, so the fail-safe default needs a `Store(true)` at init.

## The injector plugin, and why it exists

`fakeddos` emits a synthetic `AttackDetected`, waits, then emits
`AttackCleared`. It is the first and only test of the while-running withdraw
path: `removeMitigation` to `RegisterTables(nil)` to `ApplyAll` delete. The copp
withdraw test exercises the shutdown flush path only, so the responder's
clear-while-running withdraw had never run against a real backend.
<!-- source: internal/test/plugins/fakeddos/fakeddos.go -- synthetic attack injector -->

**`events.Event.Emit` counts out-of-process subscribers only.** An in-process
subscriber is delivered to synchronously and is not counted, so `Emit` returns 0
after it has just run the handler. `fakeddos` first looped "emit until the count
is above zero", which never terminates. Never gate on the return count for
in-process delivery. `fakeddos` re-emits idempotently until the driver's trigger
file appears, and the driver creates that file only after it has seen the table.

**A test plugin cannot take an "unused" signal as a control channel.** Unhandled
means Go's default disposition applies, and for SIGUSR2 that terminates the
process. SIGUSR1 is already the reactor status dump, and the hub catches INT,
TERM and HUP only. Use a filesystem handshake: the daemon's working directory is
the per-test tmpfs directory, which is the same directory the driver reads
`daemon.pid` from, so a trigger file needs no path coordination.

**nft 1.1.1 renders a MASKED source-address match raw.** `ip saddr
192.0.2.0/24` becomes `@nh,96,32 & 0xffffff00 == 0xc0000200`. Only an unmasked
literal renders cooked. A version-robust `.ci` assertion matches either form.
