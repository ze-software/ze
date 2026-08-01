# 1085 - unify-tree-deactivation

## Context
The config `Tree` encoded "this leaf-list member is deactivated" as an in-band
`inactive:` string prefix glued into the member value (`multiValues`), separate
from the whole-leaf out-of-band `inactiveValues` map. The prefix leaked out of
the config package through `GetSlice`/`GetMultiValues`/`ToMap` and was
string-sniffed at 7 BGP config/reactor sites, carried a collision risk (a value
literally starting with `inactive:`), and forced duplicate verb/serialize
branches. Goal: one representation (out-of-band), delete the prefix, preserve all
externally observable behavior.

## Decisions
- Extended the out-of-band winner: `Tree.inactiveMembers map[string]map[string]bool` sibling to `multiValues` (members stay clean), over keeping the in-band prefix and deleting the sibling map (collision-free, explicit).
- Effective-config accessors (`GetSlice`/`GetMultiValues`/`ToMap`) return **active-only** clean values; a dedicated `GetMultiValuesState` serves serialize/diff/reactor. Chose this over "surface a structured `inactive` field in `ToMap`" once tracing proved the functional consumers read `GetMultiValues`/`GetSlice` (raw), NOT `ToMap` (ToMap only feeds plugin JSON). This was the spec's original central premise and it was FALSE.
- Reactor filter chain moved from `[]string` to `filterapi.FilterRef{Name, Inactive}`, threaded end-to-end, over keeping `[]string` with a re-attached prefix (full removal: no `inactive:` on any value/logic path). A single `FilterRefStrings` display seam re-materializes `inactive:name` for the birdwatcher-style plugin protocol and the peer/policy commands (byte-identical output).
- Kept the method NAMES `DeactivateMultiValue`/`ActivateMultiValue`/`MultiValueMemberState` (rewrote bodies) so producers (parser/setparser/editor) needed no changes.
- Inline `[ inactive:MEMBER ]` promoted to a first-class documented input shorthand, normalized at the parse boundary (`stripInactiveMemberPrefix`) for typed + untyped lists, over deleting it as an invalid form (it is documented in `docs/guide/redistribution.md`).

## Consequences
- `inactive:` as a value/logic marker is gone: `rg inactiveValuePrefix` and `rg 'HasPrefix.*"inactive:"' internal/component/bgp/` are empty. The string now survives ONLY at input (parse normalization), output (serialize statement writers), and display/API seams (`FilterRefStrings`, `diff_tree.memberDisplay`, `cmd/policy`).
- "Deactivated = not in effect" is now explicit everywhere: ~30 `GetSlice` + ~6 `GetMultiValues` consumers get active-only for free (also fixes a latent path where a deactivated typed member reached resolv.conf/diff/doctor as a malformed `inactive:8.8.8.8`).
- Serializer still emits ONLY the statement form (`inactive: <leaf> <member>` / `nop`); both the statement and inline forms are accepted as input and round-trip to the statement form.
- Trade-off: a member value that legitimately begins with `inactive:` can only be written via the statement form (inline shorthand would misread it) -- the same collision the storage change closes, confined to one input shorthand.

## Gotchas
- **The spec's own data-flow was wrong.** It claimed per-member deactivation reaches the reactor via `ToMap`; it reaches it via `GetMultiValues` (`extractFilterChain`). Validate the *producer* path before designing around a boundary -- `ToMap` is only the plugin-JSON boundary.
- **A-1 was validated with an incomplete grep.** "The raw `inactive:` prefix never appears as input" was checked only against `test/**/*.{conf,ci,et}` -- it missed configs embedded in Go test seeds AND `docs/guide/redistribution.md`, which documents the inline form. The inline `[ inactive:X ]` form is a real feature, not an artifact. Grep docs and Go-embedded configs too.
- **Do not add parser code to make one bad-looking test pass.** When the inline-form test failed, the first fix was parser normalization framed as a workaround; the user challenged it as "working around a structural issue." It turned out to be a real feature, but the lesson stands: a failing test that uses an unfamiliar form is a prompt to find the canonical syntax (`test/parse/*.ci` + serializer output + docs), not to teach the parser to accept whatever the test wrote.
- Changing `GetSlice`/`GetMultiValues`/`ToMap` semantics touches every leaf-list consumer; the compiler cannot catch behavioral drift. Route structure-needing consumers (serialize, diff, doctor, `extractFilterChain`) to `GetMultiValuesState`; leave everyone else on the active-only accessors.
- Full-removal of an in-band convention balloons past the config package: the reactor `[]string` filter chain was the real blast radius (peersettings, reactor_dynamic, reactor_api, filter_ordered, peer, forward_facts, initial_sync + their tests), not the 7 sniff sites the spec listed.

## Files

None recorded.
