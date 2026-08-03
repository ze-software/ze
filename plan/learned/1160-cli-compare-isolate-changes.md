# 1160 -- cli-compare-isolate-changes

## Context

`show | compare` rendered the ENTIRE configuration with a marker column instead of only what
differed, burying one changed line under hundreds of unchanged ones on a real config. The cause was
structural: `compare` was the only data-selection pipe that never pruned. Its siblings `active` and
`inactive` clone the tree, prune it, then serialize (`ActiveContentAtPath` -> `config.PruneInactive`);
`compare` selected everything and leaned on a gutter to imply selection. Tracing it surfaced three
more defects: compare silently degraded to a plain `show` when the `changes` column was disabled;
`set cli format` was process-global and leaked across concurrent SSH/web sessions; and the two
existing compare functional tests never exercised compare at all.

## Decisions

- **`compare` prunes the YANG tree; `format` serializes it** -- over pruning the rendered diff lines.
  The pipe grammar already drew this line (`ClassifyShowPipes`: `format` = presentation `tree|config`,
  `compare`/`active`/`inactive` = data selection). Pruning the TREE makes both formats isolate with one
  implementation; pruning LINES would have fixed `| format tree` and silently left `| format config`
  dumping the whole config, because the two formats use different serializers.
- **"Unchanged" means "serializes identically"** -- over a per-node-kind comparison dispatch. There was
  nothing to reuse (`pruneActiveNode` switches only on Container+List; leaves are handled out-of-band
  by the unexported `pruneInactiveLeaves`), and no tree-vs-tree comparison exists in the config package
  (`DiffMaps` works on `map[string]any`, `PendingChanges` on the MetaTree). A hand-written nine-way
  dispatch would duplicate `diffWalkNode` and could diverge from what the gutter marks. `serializeNode`
  covers exactly the same nine kinds and handles Freeform/InlineList better than the diff walk does.
- **`PruneUnchanged` lives in `config/prune.go`** -- not merely for symmetry with its siblings: leaf
  removal touches five unexported `Tree` fields under `t.mu` with no exported equivalent, so a
  cli-side prune is impossible without widening the config API.
- **Prune BOTH directions.** A deleted node exists only in the baseline, so only baseline-vs-working
  retains it. One direction would silently drop every deletion.
- **No new config surface** -- rejected `environment cli compare default` + `ze.cli.compare` +
  `set cli compare` + a junos renderer. `| format tree` IS nested-with-context and `| format config`
  IS the flat `set` view (Ze's `display set` equivalent). A style knob would have duplicated the pipe
  grammar in config.
- **Suppress validation highlighting in pruned compare views** -- over mapping pruned lines back to
  full-content line numbers. Identical lines (closing braces; the same leaf value under two peers)
  make any alignment bind an error to the wrong parent, reintroducing the wrong-marker failure.
  A wrong highlight actively misleads; fail-safe beats fail-wrong.
- **`set cli format` moved off `env.Set` onto the per-session Model** -- `env.Set` writes a
  package-global cache AND `os.Setenv`, so one operator's choice changed every concurrent session's
  output. The docs already called it a "session override"; the fix aligned code to the documented
  contract rather than changing it.

## Consequences

- `compare` now behaves like its `active`/`inactive` siblings. Any future data-selection pipe should
  follow the same clone -> prune -> serialize shape rather than post-processing rendered text.
- Isolation is format-agnostic for free: a new `format` kind inherits it, because pruning happens
  before serialization.
- `PruneUnchanged`'s "serializes identically" rule means the serializer defines what counts as a
  change. Deactivation is caught for free (the `inactive: ` prefix changes the text). A serializer
  change silently changes prune behavior -- `TestPruneUnchangedAgreesWithSerializer` pins the link.
- Validation errors are keyed by LINE, which is why a pruned view cannot position them. Keying them
  by config PATH would lift the AC-8 limitation and also fix subtree views.
- `ProcessPipesDefaultFormat*`/`ProcessPipesDetectLog` now take an explicit `sessionFormat`; every
  new call site must decide (pass "" for no override). Precedence: session > config/env > text.

## Gotchas

- **The two existing compare `.et` tests were vacuous and that is why this bug survived.**
  `compare-with-changes.et` and `show-pipe-compare.et` both added `listen 0.0.0.0:179`. There is no
  `listen` leaf in `ze-bgp-conf.yang`: the parse fails, `SetWorkingContent` sets `treeValid = false`,
  and the editor drops into raw-text mode where the schema tree diff never runs. Their `contains=+`
  assertions passed anyway, because raw text still differs from the serialized baseline. **Any `.et`
  touching tree-aware behavior must use leaves that exist in the schema; a passing compare test proves
  nothing otherwise.** Found only because a new `.et` failed where the equivalent unit test passed --
  the unit test used `SetValue` (tree stays valid), the `.et` used `load file absolute replace`.
- **Fixing one compare path left a second one broken.** `showAlternateSource` took PRE-RENDERED text,
  so `show saved | compare` kept dumping the whole config after the working path was fixed, and every
  test passed. Two paths for one concept is the defect; the fix was to delete the second, not to
  duplicate the prune into it.
- **A comment claimed a contract the code did not honor.** The unparseable-baseline fallback asserted
  it matched `renderRawSourceAtPath`'s "legacy files stay visible" while actually returning an EMPTY
  baseline (rendering the whole config as added). A false claim in a diff is the shield that stops the
  next reviewer asking.
- Two pre-existing tests asserted `originalContent` was non-empty while making NO change -- they
  encoded the dump-everything bug. Fixed at the FIXTURE (make a real edit), not by deleting the
  assertion.
- `PruneUnchanged` had to be re-tested for the marker character: a modified leaf renders `*`, not `+`.
  An `.et` asserting `+` after modifying a value fails for a reason that has nothing to do with the
  code -- nearly recorded a Known Limitation that did not exist.
- Twice during design a new surface was proposed without grepping for the existing sibling that already
  solved it (config home -> `environment cli format default`; pruning layer -> `PruneInactive`).
  `ai/rules/architecture.md` already forbids this. The trigger to watch is any "where does this go?"
  question.

## Files

- `internal/component/config/prune.go` -- `PruneUnchanged` + serialize-compare helpers
- `internal/component/config/tree.go` -- unexported `removeValue`
- `internal/component/config/prune_unchanged_test.go` -- 11 unit tests
- `internal/component/cli/model_commands_show.go` -- `compareView` (single path), `compareViewUnpruned`,
  `sourceTree`, `sourceContent`, `comparePrunedViews`, `resolveCompareBaselineTree`
- `internal/component/cli/editor_annotated.go` -- `annotatedViewOf` (tree-parameterized)
- `internal/component/cli/model.go` -- `noValidationHighlight`, session `cliFormat`
- `internal/component/cli/model_render.go` -- validation-highlight opt-out
- `internal/component/cli/model_keys.go` -- `set cli format` off `env.Set`, `sessionFormat`
- `internal/component/command/pipe.go` -- `sessionFormat` parameter, `configuredDefault`
- `internal/component/cli/model_mode.go`, `model_ping.go`, `model_traceroute.go`,
  `internal/component/web/cli_terminal.go` -- pipe call sites
- `test/editor/pipe/show-compare-{isolates,format-config,forces-markers,nested-context,saved-source}.et`
- `test/editor/commands/compare-with-changes.et`, `test/editor/pipe/show-pipe-compare.et` -- de-vacuumed
- `docs/guide/config-editor.md` -- compare shows only what differs
