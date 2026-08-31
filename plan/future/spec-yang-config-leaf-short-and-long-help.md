# Spec: yang-config-leaf-short-and-long-help

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Depends | `spec-yang-short-and-long-command-help` (closed) |
| Phase | - |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-31 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`spec-yang-short-and-long-command-help`, now closed, gives every COMMAND node a
declared short summary and a declared long help text, and deletes the four
truncation guesses that stood in for them. It leaves CONFIG nodes alone,
because the owner scoped that work to commands.

The same defect remains on the config side, over a larger corpus: 1241 leaf and
leaf-list descriptions, 443 container and list descriptions, and 277 enum
descriptions, each rendered by a surface that needs one line and by a surface
that needs a paragraph, with no declaration of which is which.

The config renderers that guess today:

| Surface | Producer | What it does with one string |
|---------|----------|------------------------------|
| Web editor input title | `internal/component/web/handler_config_leaf.go` `buildLeafField` | whole string as a `title` attribute |
| Web editor information badge | `internal/component/web/fragment.go` `buildFieldMeta` | whole string as a tooltip |
| Web editor placeholder | `internal/component/web/view.go` `fieldPlaceholder` | whole string in a placeholder, for a leaf with no value and no default |
| Sidebar heading | `internal/component/web/fragment.go` `nodeDescription` | whole container or list description |
| Website config reference row | `internal/le/site/configscript.go` | whole string in a list row AND in the detail paragraph |
| Website config mirror | `internal/le/site/config.go` | whole string, whitespace collapsed |
| `llms.txt` config roots | `internal/le/site/derived.go` `writeLLMSConfigRoots` | cut at 180 characters with an ellipsis |
| Site search index | `internal/le/site/search.go` `flattenConfigSection` | capped by `searchConfigBodyCap` |
| YANG analysis output | `internal/component/config/yang/cli/format.go`, `cli/doc.go` | hard character truncation at 47 and 57 |

Fourteen config descriptions already contain a blank-line paragraph break, and
one leaf runs past 900 characters quoting an RFC. So the config corpus has a
wider spread than the command corpus and a heuristic serves it no better.

One constraint is specific to this side and does not apply to commands:
`ai/rules/config.md` makes a leaf `description` a load-bearing contract, since
it must name any environment variable that overrides the leaf. A split must
decide which half carries that obligation before any text moves.

## Status

Not started. Homed here so the deferral row in
`plan/deferrals/yang-short-and-long-command-help.md` names a destination that
exists. **It needs Thomas to say whether it runs** (`ai/rules/planning.md`): it
is a distinct, larger, separable piece of work, not a defect walked into.

The command-side spec is the prerequisite. It settles the mechanism (a declared
`ze:help` extension rather than a convention), the shape rule for a summary, and
the gate that enforces it. This spec would extend all three to config nodes
rather than re-decide them.
