# Spec: web-plugin-extension-registry

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Handoff | - |
| Updated | 2026-08-16 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Let a plugin extend the web interface through a registration system, the way a
plugin already extends the CLI.

**Owner statement, 2026-08-16 (the source of this spec).** The web interface
today is something Ze can offer only when the code for it is wired in directly.
It is not a plugin surface, and an extension cannot add to it. The direction
asked for is a registration system plus a generic registration hook, matched to
the one the CLI carries.

**The state of the code is NOT recorded here, on purpose.** No producer was read
before this file was written, so this document states an intent and never a
finding (`ai/rules/evidence.md`). Before design, the implementer MUST identify:

- the function that assembles the web route table,
- the process-boundary features that a plugin can use, and
- the operation of the CLI hook that this feature will follow.

**One rule already assumes the surface exists.** `ai/rules/plugins.md`, under
"The Removal Test", lists "web/looking-glass routes" among the surfaces that
must disappear when a plugin's directory and its blank import are removed. So
the removal invariant is already written for plugin-owned web routes. The
mechanism that would make a plugin own one is what this spec is about.

## Sketch (not a design)

| Element | Note |
|---------|------|
| Model to copy | the CLI path a plugin uses today: registration, then discovery by the core |
| Registered unit | open. A route, a page, a fragment, a nav entry, an asset, or several of these |
| Who registers | a plugin, in-process and out-of-process alike, or the answer is that only one of the two is reachable |
| What travels | markup or data, and which side renders it |
| Removal test | deleting the plugin removes its pages, its nav entries and its assets, and every other page keeps working |

## Open questions

- What does a web contribution need that a CLI command does not: a template, an
  asset, a stylesheet, a CSP entry, an authorization rule, an SSE lifetime?
- Does an out-of-process plugin send markup, or send data that the core renders?
  Markup from a plugin makes the plugin an author of the page's HTML, with the
  escaping and CSP questions that follow.
- Does the looking glass share the mechanism, or keep its own?
- Is one registry enough for pages and fragments, or does htmx make a fragment a
  different unit from a page?
- What must the removal test verify, and which gate runs it
  (`ai/rules/plugins.md`, "Mechanical Check")?

## Why this is in `plan/future`

No operator loses anything today: every web page Ze ships is wired in and works.
Nothing on the wire is affected, and no configuration is accepted and then
ignored. It is an architecture change that makes a surface extensible, so it
belongs here per `plan/future/README.md`.
