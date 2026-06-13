# 891 -- Granular Debug

## Context

Ze's debug system was a simple on/off toggle per subsystem using individual ZeFS keys (`state/debug/all`, `state/debug/{subsystem}`). Operators needed richer debug control: per-flag filtering (e.g. only BGP UPDATE messages), direction filtering (send/receive), instance scoping (specific neighbor), and the ability to save/restore debug configurations as named profiles. The old system couldn't group settings atomically or validate flag names against what each subsystem actually supports.

## Decisions

- Chose a separate debug YANG registry over reusing the config YANG registry, because debug configuration is not committed config and must not appear in `show configuration` or `commit`
- Chose toggle semantics (same command is its own undo) over explicit enable/disable pairs, because CLI history (`arrow-up`) becomes the natural undo mechanism
- Chose JSON profiles in `debug.zefs` over individual ZeFS keys, because a single profile key stores the complete debug state atomically; individual keys could not support named profiles or atomic restore
- Chose pre-extracted flag metadata on the Module struct over runtime YANG parsing, because full goyang parsing for flag enumeration extraction is disproportionate to the validation need; the YANG Content field remains available for future schema-based features
- Chose unexported filterHandler with public ConfigureFilter/ClearFilter API over exported FilterHandler, because all filter manipulation goes through the registry and exposing the handler type adds no value
- Chose validation-skip when no debug YANG module covers a module prefix over rejecting all flags, because this allows progressive rollout as plugins add their debug YANG registrations
- Chose not to auto-apply debug state on reboot over auto-applying (like the old system), because if debug caused a crash, a clean start is safer
- Collapsed `direction` from a dedicated grammar keyword into a scope kind, because direction is protocol-specific (BGP send/receive, L2TP control/data) and baking it into the generic grammar leaks protocol assumptions; non-protocol plugins simply don't declare a "direction" scope

## Consequences

- Any plugin can register debug flags by adding a `register_debug.go` in its `yang/` package with a `debugyang.RegisterModule(...)` call; same pattern as config YANG, doctor checks, and capabilities
- The old `ze debug enable/disable/show` CLI is gone; operators must learn the new toggle grammar
- The old `state/debug/*` ZeFS key format is gone; existing debug.zefs files will not auto-migrate (they'll be clean on first use)
- The `show debug` path is currently via `ze debug show` (offline), not via the online `show` verb; an online `show debug` with YANG dispatch would require a command YANG module
- Every slogutil.Logger() now wraps its handler with filterHandler, adding a nil-check branch to every log record (measured at zero cost when no filters are configured via the `!hasFlags && !hasScopes` fast path)

## Gotchas

- ZeFS `List(prefix)` fails when the prefix has a trailing slash because `strings.Split` creates an empty segment that won't match any tree child; always use `"debug/profile"` not `"debug/profile/"`
- `replace_all` on type names catches substrings in function/test names (e.g. `FilterHandler` -> `filterHandler` also hit `TestFilterHandler` -> `TestfilterHandler`); do targeted replacements instead
- The wiring-docs check flags exported functions that are only called from within the same package; unexporting them or ensuring a cross-package caller resolves this

## Files

- `internal/component/debug/yang/register.go` -- debug YANG module registry
- `internal/core/slogutil/filter.go` -- filterHandler (flag/direction/scope filtering)
- `internal/core/slogutil/slogutil.go` -- filterRegistry, ConfigureFilter, ClearFilter
- `internal/core/slogutil/debug.go` -- rewritten (removed old key-based resolution)
- `internal/plugins/debug/debug.go` -- rewritten (toggle-based CLI handler)
- `internal/plugins/debug/profile.go` -- profile storage
- `internal/plugins/debug/show.go` -- structured display
- `internal/plugins/debug/register.go` -- updated registration meta
- `internal/component/bgp/yang/register_debug.go` -- BGP debug flag registration
- `pkg/zefs/keys.go` -- KeyDebugProfile replaces KeyDebugAll + KeyDebugSubsystem
- `cmd/ze/hub/main.go` -- removed ApplyDebugFlags call
- `docs/features.md` -- updated debug feature row
- `docs/guide/command-reference.md` -- updated debug CLI reference
- `ai/patterns/debug-registration.md` -- plugin author guide for debug flag registration
- `ai/INDEX.md` -- added debug registration to feature-kind table
