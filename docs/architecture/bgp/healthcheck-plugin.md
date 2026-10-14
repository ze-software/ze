# Healthcheck Plugin

The healthcheck plugin monitors service availability with a shell command and
controls BGP route announcement through watchdog groups. It has feature parity
with ExaBGP's `healthcheck.py`, and it diverges from it in four places on
purpose.

<!-- source: internal/component/bgp/plugins/healthcheck/healthcheck.go -- plugin entry point -->
<!-- source: internal/component/bgp/plugins/healthcheck/fsm.go -- states and transitions -->
<!-- source: internal/component/bgp/plugins/healthcheck/probe.go -- probe execution -->
<!-- source: internal/component/bgp/plugins/healthcheck/hooks.go -- hook execution -->
<!-- source: internal/component/bgp/plugins/healthcheck/ip.go -- IP management through iface -->
<!-- source: internal/component/bgp/plugins/healthcheck/config.go -- config parsing and the disable leaf -->

## The decisions

**One watchdog group with a MED override per health state.** Multiple route
definitions per probe were rejected. Route attributes live in BGP config and
only the MED varies per state. The `watchdog announce <name> med <N>` extension
is reusable by any plugin that needs per-dispatch MED control.

**Per-state community and AS_PATH variation was dropped.** ExaBGP's per-state
AS_PATH carries a defect: it reads `options.as_path` instead of the resolved
variable. An operator who needs per-state communities defines separate watchdog
groups.

**Labels were dropped from ip-setup.** Ze uses netlink and tracks its addresses
internally. It does not shell out to `ip`.

**Disable is a config reload, not a polled file.** `ze config set ... disable
true` fits the config-driven model. ExaBGP polls a file.

**A reload or a `reset` stops a probe without waiting for its exit.** The probe
set is one config transaction: verify parses and validates the candidate, apply
makes it the running set, and rollback reinstalls the previous set. An
unchanged probe keeps running. A removed or changed probe is canceled, and its
exit withdraws the route through `request bgp watchdog withdraw` and removes its
addresses on its own. Apply does not wait for that, because bgp-watchdog is
another participant of the same transaction and the round trip can take 5 s.
`clear bgp healthcheck <name>` works the same way, so its reply can return
before the old probe's withdraw has run: a caller that reads the route straight
after the reply can still see it announced.

The probe that replaces a canceled one, by name or by watchdog group, waits for
that probe's exit before it adds an address or dispatches, so a late withdraw
never lands after its announce. The wait is a sum: one predecessor can take its
in-flight check (`timeout`), its in-flight hooks (30 s each) and the 5 s exit
dispatch, and a chain of reloads adds one such term for each generation. A
probe canceled while it waits does nothing. A rollback restarts the previous
probes with a fresh FSM, so rise and fall progress is not restored.
<!-- source: internal/component/bgp/plugins/healthcheck/healthcheck.go -- applyConfig, retire, startProbe, runProbe, handleReset -->
<!-- source: internal/component/bgp/plugins/healthcheck/config_tx.go -- newHealthcheckConfigTx -->

**Metric mode is the default**, matching ExaBGP. Withdraw-on-down is opt-in.

**`show bgp healthcheck` answers a row set with or without a probe name, and it
declares the `map` shape.** One command path carries one answer-shape
declaration. So the named-probe branch answers a one-element row set rather
than a bare object.

The two branches carry different field sets on purpose: the list gives three
fields for each probe and the detail gives ten. No column order fits both, which
is why the declaration is `map` and not `tab`. The list branch walks the probe
names in ascending order rather than in map order. `| first` and `| last`
therefore select the same probe on every call.
<!-- source: internal/component/bgp/plugins/healthcheck/healthcheck.go -- handleShow, commandDecls -->

External plugins cannot use ip-setup. The configure and config-verify callbacks
reject it.

## Constraints

**MED override bypasses watchdog dedup.** The pool tracks announced and
withdrawn state, not command content. Without the bypass, a MED change on an
already-announced route is dropped in silence.

**Config change detection compares `ProbeConfig` with `reflect.DeepEqual`.**
Reordering a leaf-list therefore triggers a reconfigure. Config reordering is
rare, so this is accepted.

**Shell execution uses the `cmd.Cancel` process-group kill pattern** for both
probe and hook timeouts. Any future shell execution in a plugin uses the same
pattern, or it leaks children on timeout.

The YANG schema was written with every leaf of every phase present from the
start. This front-loaded the work and removed cross-phase schema migrations.
