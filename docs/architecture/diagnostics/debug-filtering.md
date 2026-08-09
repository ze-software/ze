# Granular Debug

Debug is not an on/off switch per subsystem. An operator selects flags (for
example BGP UPDATE messages only), a direction, and an instance scope (one
neighbor), and saves the result as a named profile.

<!-- source: internal/plugins/debug/debug.go -- toggle CLI handler -->
<!-- source: internal/core/slogutil/filter.go -- filterHandler -->
<!-- source: internal/component/debug/yang/register.go -- debug YANG module registry -->

## A separate YANG registry

Debug settings are registered in their own YANG registry, not in the config
registry. Debug state is not committed configuration: it must not appear in
`show configuration` and must not take part in `commit`.

A plugin declares its debug flags by adding a `register_debug.go` in its `yang/`
package that calls `debugyang.RegisterModule(...)`. This is the same shape as
config YANG, doctor checks and capabilities.

<!-- source: internal/component/bgp/yang/register_debug.go -- BGP debug flag registration -->

Flag metadata is pre-extracted onto the module struct rather than parsed from
YANG at runtime. Full goyang parsing to enumerate flag names is out of
proportion to a validation need. The YANG content stays available for a later
schema-driven feature.

When no debug module covers a module prefix, validation is skipped rather than
rejecting every flag. This lets plugins adopt debug registration one at a time.

## Toggle semantics

The same command is its own undo. CLI history makes the arrow-up key the
natural undo, so there is no enable and disable pair to remember.

`direction` is a scope kind, not a grammar keyword. Direction is
protocol-specific: BGP has send and receive, L2TP has control and data. Baking
it into the generic grammar would leak a protocol assumption into every plugin,
and a plugin with no directional traffic simply declares no direction scope.

## Profiles are one JSON value

A profile is one JSON document under a single key in `debug.zefs`, not a set of
individual keys. One key stores the complete state, so a save and a restore are
atomic. The old `state/debug/*` key layout could express neither a named
profile nor an atomic restore, and it is gone with no migration path.

<!-- source: internal/plugins/debug/profile.go -- profile load, save and modify -->
<!-- source: pkg/zefs/keys.go -- KeyDebugProfile -->

Debug state is not re-applied on reboot. The old system did re-apply it. If
debug output contributed to a crash, a clean start is the safer default.

## Cost on the log path

Every `slogutil.Logger()` wraps its handler with `filterHandler`, which adds one
branch to each log record. The `!hasFlags && !hasScopes` fast path makes that
branch free when no filter is configured.

<!-- source: internal/core/slogutil/debug.go -- subsystem validation and matching -->
<!-- source: internal/plugins/debug/cmd/handlers.go -- live debug state query -->
<!-- source: internal/core/duration/duration.go -- shared duration parsing for CLI input -->

## One ZeFS trap

`List(prefix)` fails when the prefix ends in a slash. `strings.Split` then
produces an empty final segment, which matches no child in the tree. Use
`"debug/profile"`, never `"debug/profile/"`.
