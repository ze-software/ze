# 795 -- YANG-Typed Arguments for Operational Commands

## Context

Operational commands (show, set, log) accepted untyped string arguments with
ad-hoc validation in each handler. Completion hints for static sets (log levels,
FD limit "max", goroutine modes) were manually wired in Go code via ValueHints
callbacks. This meant three separate places needed updating for each new
argument: the handler, the ValueHints wiring, and the help text. YANG command
containers already existed but contained no argument metadata.

## Decisions

- Declared argument types as YANG leaves inside ze:command containers, over
  extending ValueHints or adding a new ze:arg extension. YANG leaves naturally
  carry enum, uint, string+pattern, and union types that goyang already parses.
- Added a second pass in mergeYANGEntry for leaf children of ze:command nodes,
  over changing the main Config filter. Leaves have Config == TSUnset (inherited,
  not explicit); changing the filter would pick up config leaves too.
- Put ArgDefs on command.Node and Command struct, over auto-populating
  ValueHints. ArgDefs carry type metadata for validation, not just completion.
- Two-phase dispatcher validation (keyword extraction then positional matching),
  over single-pass or validate-on-complete. Phase 1 uses leaf names as keyword
  detectors; Phase 2 validates remaining positional args against enum/union types.
- Kept handler signature `func(ctx, args []string)` unchanged, over passing
  parsed typed args. Changing the signature would require modifying all 23
  handlers for no benefit since handlers already parse args.
- Runtime-dynamic ValueHints (plugin families for rib) kept as Go callbacks,
  over declaring them in YANG. Plugin family sets are determined at runtime by
  which plugins register.

## Consequences

- Adding a new typed argument to any command requires only a YANG leaf
  declaration. Completion, validation, and documentation derive automatically.
- Duration arguments remain as patterned strings (YANG has no duration type).
  Handlers still parse the duration string.
- Positional args that match no ArgDef are rejected by the dispatcher when
  enum/union ArgDefs exist. Handlers with string ArgDefs accept any positional.
- The PathToArgDefs function builds a path-to-defs map that flows through
  LoadBuiltinsWithAliases into RegisterOptions, adding a parameter to both
  LoadBuiltins and LoadBuiltinsWithAliases.

## Gotchas

- goyang's EnumType.ToInt is a map[string]int64. Iterating it produces
  non-deterministic order. ArgDef.EnumValues must be sorted explicitly for
  stable completion output and test assertions.
- The mergeYANGEntry filter checks `child.Config != TSFalse`, but goyang does
  NOT propagate `config false` to leaf descendants. Leaves inside config false
  containers have `Config == TSUnset`. The fix is a targeted second pass that
  checks `entry.Type != nil`, not changing the main filter.
- YANG pattern regexes are XSD-style (implicitly anchored). Go regexp requires
  explicit `^` and `$` anchors, so compileYANGPattern adds them.
- Union type ArgDefs need EnumValues populated at the top level (from the enum
  member) for the completer, plus UnionDefs with the full member list for the
  validator. Both are set during extraction.

## Files

- `internal/component/command/node.go` -- ArgDef type, ArgKind constants, ArgDefs field on Node
- `internal/component/command/argvalidate.go` -- ValidateArgString for each ArgKind (new file)
- `internal/component/command/argvalidate_test.go` -- validation unit tests (new file)
- `internal/component/command/completer.go` -- matchChildren reads ArgDefs for suggestions
- `internal/component/command/completer_test.go` -- ArgDefs completion tests
- `internal/component/command/valuehints.go` -- removed wireLogSetHints, wireFDSetHints
- `internal/component/config/yang/command.go` -- extractArgDefs, yangTypeToArgDef, PathToArgDefs
- `internal/component/config/yang/command_test.go` -- YANG extraction and ArgDefsPopulated tests
- `internal/component/plugin/server/command.go` -- ArgDefs on Command/RegisterOptions, validateCommandArgs
- `internal/component/plugin/server/command_test.go` -- dispatcher validation tests
- `internal/component/plugin/server/server.go` -- PathToArgDefs wiring
- `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` -- leaves on 20 show commands
- `internal/component/cmd/set/yang/ze-cli-set-cmd.yang` -- union leaf on set system file-descriptors
- `internal/component/cmd/log/yang/ze-cli-log-cmd.yang` -- leaves on log set and log recent
- `internal/component/cli/client/main.go` -- mergeArgDefs from YANG tree
- `internal/component/cli/client/main_test.go` -- updated log level test to check ArgDefs
- `docs/architecture/api/commands.md` -- documented ArgDefs architecture
- `test/ui/completion-words-goroutine-modes.ci` -- functional test (new file)
- `test/ui/completion-words-audit-keywords.ci` -- functional test (new file)
