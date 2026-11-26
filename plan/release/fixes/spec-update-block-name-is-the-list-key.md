# Spec: update block name is the list key

| Field | Value |
|-------|-------|
| Status | design |
| Priority | P1 |
| Release reason | Preserve operator update identities consistently through parsing, formatting and display. |
| Scope | config |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A BGP update block can be named two ways today, and both work.

`update drop-ssh-scan { ... }` parses: `parseList` takes the word after the list
name as the entry key whether or not the YANG declares a key, so the word lands
in the config tree as the entry's key. `update { name drop-ssh-scan; ... }` also
parses, because `list update` in `internal/component/bgp/yang/ze-bgp-conf.yang`
is keyless and carries a `leaf name` marked `ze:display-key`.

Nothing reconciles the two. A block written the first way leaves the `name` leaf
empty. A block written the second way leaves the entry key at `default`. One
fact has two declarations, which `ai/rules/principles.md` bans, and the surfaces
disagree about which one to read.

The owner ruled on 2026-09-13 that `update <name> { ... }` is the sole spelling
and the `name` leaf form is retired. The owner ruled the same day that the name
is unique per container and a repeat is refused at config load. The parser
enforces that for ordinary names, but currently exempts the literal name
`default` because it uses `KeyDefault` (`"default"`) for anonymous blocks as
well. The name is operator memory: nothing keys behavior off it, it reaches no
BGP message, and it needs no lookup path and no registry. What it owes is that
it survives every read and write round trip and that the CLI and the web show
it.

Three defects follow from the split, all measured with `bin/ze` on 2026-09-13
and verified in HEAD on 2026-11-26:

1. The web finder never shows an operator's name. `buildListColumn` sets the
   display name to the entry key and then, for a list whose schema is keyless,
   overwrites it with `keylessEntrySummary` or with `#<key>`. An entry named
   `drop-ssh-scan` is labelled by its NLRI families.
2. `ze config fmt` and `ze config show` write the parser's internal
   disambiguation suffix into the operator's file. A config with two bare
   `update { }` blocks comes back with the second one as `update "default#1" {`.
   `WriteListEntry` compares the raw key against `KeyDefault` and writes
   anything else, so the generated key escapes into config text, while
   `serializeAnnotatedList` and `serializeBlameList` strip the suffix into
   `displayKey` and still test the raw `key` against `KeyDefault`, emitting
   `update default {`.
3. The web add form prompts for the `ze:display-key` leaf when creating a
   keyless entry, so a name typed there lands in the leaf that is being retired,
   and an entry created with no name gets a fabricated sequential key (`1`,
   `2`), which under the new rule reads as a name the operator never typed.

Additionally, `parseList` passes `KeyDefault` (`"default"`) for bare `update { }`
and the same string for an explicit `update default { }`, so the tree cannot
distinguish anonymous blocks from the legal, unique operator name `default`.

The goal:

- One declaration of the update block name, which is the list entry key, with
  `leaf name` and `ze:display-key` deleted rather than kept beside it.
- Collision-free internal representation of keyless-list anonymous entries via
  `config.KeyAnonymous = "#anonymous"` (disambiguated by `Tree.AddListEntry` as
  `#anonymous`, `#anonymous#1`, `#anonymous#2`, ...) paired with one shared
  identity API in `internal/component/config` (`AnonymousListKey`,
  `IsAnonymousKey`, `IsAnonymousEntryKey`, `ListEntryName`,
  `NextAnonymousListKey`), leaving `KeyDefault` untouched for legacy keyed-list
  anonymous entries.
- Keyless YANG lists receive the retired `leaf name` pattern
  `[a-zA-Z0-9_][a-zA-Z0-9_.\-]*` on `ListNode.KeyLeaf` in `yangToList`, so
  `ValidateListKey` enforces the exact identifier grammar on every
  operator-supplied name (including refusing `#anonymous`, braces, quotes, and
  whitespace) while keeping `default` legal and unique.
- Bare `update { }` blocks remain legal and repeatable at `bgp`, `group`, and
  `peer` levels; every serializer, tree merge, CLI editor/diff/completer path,
  and web finder/add-form path preserves the distinction between named entries
  (including literal `default`) and anonymous entries.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - the Update Block section and the Custom YANG Extensions table
  → Decision: the page documents only the bare `update { attribute {} nlri {} }` form, so it gains the named spelling and uniqueness rule in the same work as the code.
  → Constraint: the extension table at line 2334 declares `ze:display-key` as the label mechanism for keyless list entries, and that row goes with the extension.
- [ ] `docs/architecture/config/yang-config-design.md` - the Ze YANG extension inventory
  → Constraint: the extension row at line 109 is a second declaration of `ze:display-key` and is deleted in this work.
- [ ] `docs/architecture/web-interface.md` - finder columns, URL transport, and the config entry handlers
  → Constraint: the finder column builder reads the config tree shape and `encodeConfigPath` already percent-encodes `#`, so `#anonymous` / `#anonymous#N` addresses travel cleanly with no new URL transport mechanism.
- [ ] `ai/rules/principles.md` - one fact, one declaration; a central enumeration is recognized by what it answers for a case it does not name
  → Decision: "which key is anonymous and what is its operator-visible name" is answered by one shared API in `internal/component/config`, generic to keyless lists and never special-casing `update`.
- [ ] `ai/rules/no-layering.md` - X is deleted, then Y is implemented
  → Constraint: `leaf name` and `ze:display-key` are deleted rather than kept beside the entry key.

### RFC Summaries (Scope: protocol)
Not applicable. Scope is `config`. The name reaches no BGP message:
`extractRoutesFromUpdateBlock` (`internal/component/bgp/config/bgp_routes.go`)
reads the `attribute` container and the `nlri` list of each entry in insertion
order and never reads the entry key or a `name` leaf.

**Key insights:** (minimal context to resume after compaction)
- `config.KeyAnonymous = "#anonymous"` cannot collide with any valid operator name because `[a-zA-Z0-9_][a-zA-Z0-9_.\-]*` forbids `#`, and it cannot collide with CLI positional `#1`/`#2` selectors because `resolveListKey` parses only `#<positive-int>`.
- `config.ListEntryKeys` already preserves insertion order (`tree.listOrder`) in HEAD across all serializers and `cli/diff_tree.go`, and `cli/diff_tree.go` already delegates list-entry rendering to `config.WriteListEntry`.
- `list update` is the only keyless list in any `-conf.yang` module under `internal/` (re-verified on 2026-11-26).
- `allowsDuplicateParsedListEntries` is what keeps two bare `update { }` blocks legal, and today it checks `DisplayKey != ""` and `key == KeyDefault`. Replacing that with `node.KeyName == "" && IsAnonymousListKey(node, key)` in the same edit as deleting `DisplayKey` keeps every existing config with multiple bare `update { }` blocks valid.
- `ListKeyTokens` must preserve `IsAnonymousKey(key)` verbatim (via `storedListKeyTokens`) rather than stripping `#N`, so multiple anonymous blocks (`#anonymous`, `#anonymous#1`) stay distinct in set-format serialization, per-user change files, CLI auto-select, and web URLs.

## Current Behavior (MANDATORY)

**Source files read (verified against HEAD on 2026-11-26):**
- [ ] `internal/component/config/parser.go` - declares `const KeyDefault = "default"` for anonymous list entries; no separate sentinel exists for keyless lists.
- [ ] `internal/component/config/parser_list.go` - `parseList` passes `KeyDefault` for `name { }` and `name { field ... }`, and validates a supplied word with `ValidateListKey`. `addParsedListEntry` skips `canonicalParsedListKey` when `key == KeyDefault`, and allows duplicates only when `allowsDuplicateParsedListEntries` returns true (`key == KeyDefault && node.KeyName == "" && node.DisplayKey != ""`, or a single-child `content` leaf list).
- [ ] `internal/component/config/schema.go` - `ListNode.DisplayKey` holds the `ze:display-key` leaf name.
- [ ] `internal/component/config/leaf_value.go` - `ValidateListKey` validates against `node.KeyLeaf` when non-nil (returning `configyang.PatternRefusal` etc.) and falls back to `ValidateValue(node.KeyType, key)` when `node.KeyLeaf == nil`, which for `TypeString` accepts any string.
- [ ] `internal/component/config/yang_schema.go` - `hasDisplayKeyExtension` matches `ze:display-key`, and `yangToList` scans children of a keyless list (`entry.Key == ""`) for it to set `l.DisplayKey`, leaving `l.KeyLeaf` nil.
- [ ] `internal/component/config/list_key.go` - `ListKeyTokens` strips `#N` via `StripListKeySuffix`, collapsing `#N` disambiguated anonymous entries into one token. `ListPathStep` returns `ListAddress{Key: KeyDefault, First: KeyDefault}` for anonymous steps. `EnsureListEntry` calls `ValidateListKey` whenever `addr.Tokens > 0 && node != nil`. `ResolveListEntryTarget` already refuses `rename` and `copy` on keyless lists.
- [ ] `internal/component/config/tree.go` - `AddListEntry` appends `#1`, `#2`, ... on repeated keys and appends `uniqueKey` to `listOrder`. `mergeFrom` merges lists by overwriting `t.lists[k][key] = v[key]` for equal keys, so merging two parent blocks each holding a bare anonymous entry overwrites the first anonymous entry instead of keeping both.
- [ ] `internal/component/config/serialize.go` - `ListEntryKeys` preserves insertion order from `tree.listOrder`. `WriteListEntry` skips writing the key only when `key == KeyDefault`, so `default#1` is written as `update "default#1" {` and an explicit `update default {` would be stripped. `inlineLeafListEntry` likewise checks `key == KeyDefault`.
- [ ] `internal/component/config/serialize_annotated.go` - `serializeAnnotatedList` strips `#N` into `displayKey`, then tests raw `key != KeyDefault`, so `default#1` is written as `update default {`.
- [ ] `internal/component/config/serialize_blame.go` - `serializeBlameList` has the same `StripListKeySuffix` plus `key != KeyDefault` pair as the annotated serializer.
- [ ] `internal/component/config/serialize_set.go` - `serializeSetList` and `serializeSetMetaList` emit set-form entry paths through `setFormEntry` -> `ListKeyTokens`, so two anonymous entries collapse onto one key token if `#N` is stripped.
- [ ] `internal/component/config/setparser_refusal.go` - `lineKeyRefusal` runs `ValidateListKey` on `tokens[0]` for every set line on a list, so set/change-file replay of a serialized anonymous key token would be refused unless keyless `IsAnonymousKey` tokens are recognized as anonymous replay addresses.
- [ ] `internal/component/cli/diff_tree.go` - `diffPrintedListEntry` already delegates to `config.WriteListEntry` and `diffListKeys` already delegates to `config.ListEntryKeys`, so tree diff inherits plain serializer behavior automatically.
- [ ] `internal/component/cli/editor_walk.go` - `*config.ListNode` branches already delegate to `config.ListPathStep` and `config.EnsureListEntry`.
- [ ] `internal/component/cli/editor.go` - `walkBareListFrom` and `walkSchemaNode` look up `entries[config.KeyDefault]` for anonymous `*config.ListNode` entries, and `resolveListKey` maps `#<positive-int>` to 1-based entries in `GetListOrdered`.
- [ ] `internal/component/cli/editor_annotated.go` - anonymous `*config.ListNode` metadata lookup reads `listMeta.GetListEntry(config.KeyDefault)` and `entries[config.KeyDefault]`.
- [ ] `internal/component/cli/completer.go` - `isDefaultKey` matches `KeyDefault` and `KeyDefault#N` regardless of whether the list is keyless, hiding a literal `default` update block behind `#N`.
- [ ] `internal/component/web/fragment.go` - `buildListColumn` overwrites every keyless entry's label with `keylessEntrySummary` or `#<key>` (ignoring an operator-supplied key) and addresses entries via `ListKeyTokens`. `keylessEntrySummary` prefers `schema.DisplayKey`, then child list keys, then the first non-empty leaf.
- [ ] `internal/component/web/handler_config_entry.go` - `HandleConfigAddWithAuthorizer` accepts `name` from the form, and when empty on a keyless list fabricates sequential numeric keys (`"1"`, `"2"`). `HandleConfigAddForm` passes `listNode.DisplayKey` into `addFormData`.
- [ ] `internal/component/web/view_fragment.go` - `addFormData.DisplayKey` carries the `ze:display-key` leaf name.
- [ ] `internal/component/web/component_add_form_overlay.templ` - for `v.Keyless`, renders `name={"field:" + v.DisplayKey}` only when `v.DisplayKey != ""`, setting the retired leaf instead of the entry key.
- [ ] `internal/component/web/golden_fixtures_test.go` - `webAddFormView(true)` sets `data.DisplayKey = "address"`.
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - `list update` (line 436) is keyless and carries `leaf name` with pattern `[a-zA-Z0-9_][a-zA-Z0-9_.\-]*` and `ze:display-key;`.
- [ ] `internal/component/config/yang/modules/ze-extensions.yang` - declares `extension display-key` (lines 356-365), whose sole user in the tree is `list update`'s `leaf name`.
- [ ] `internal/component/bgp/config/bgp_routes.go` - `extractRoutesFromTree` walks `tree.GetListOrdered("update")` and passes only `entry.Value` to `extractRoutesFromUpdateBlock`; the entry key is never read.

Measured with `bin/ze` on 2026-09-13, on a peer holding `update drop-ssh-scan`,
`update scrubbing` and two bare `update { }` blocks:
- `ze config validate` exits 0 and reports no error.
- `ze config fmt` and `ze config show` both write the second bare block as `update "default#1" {`.
- Renaming `scrubbing` to `drop-ssh-scan` makes validate exit 1 with `line 21: duplicate list key for update: drop-ssh-scan`.
- A config using `update { name drop-ssh-scan; ... }` validates and formats unchanged, which is the spelling being retired.

**Behavior to preserve:**
- `update { attribute {} nlri {} }` with no name stays legal at the `bgp`, `group`, and `peer` levels, and several bare blocks in one container stay legal. Dozens of `.ci`, `.conf`, and interop scenario files depend on it, including seven bare blocks in `test/parse/simple-v4.ci`.
- Repeated names remain refused at load with `duplicate list key for update: <name>`, now including a repeated literal `default` even when anonymous blocks sit beside or between them.
- The routes an update block produces do not change. `extractRoutesFromUpdateBlock` keeps reading `attribute` and `nlri` only, and entries keep being walked in insertion order.
- CLI key completion keeps showing `#N` for unnamed entries and the operator-supplied name for named entries (now including `default`).
- Keyed lists (including `attach { process { } }`, `exabgp` `api`, multi-leaf-key lists, and Go-constructed `List(TypeString, ...)` unit test schemas with `KeyLeaf == nil`) keep their existing key handling and `KeyDefault` behavior.

**Behavior to change:**
- `update { name <x>; }` stops parsing: `leaf name` and `ze:display-key` are deleted, so the parser answers `unknown field in update: name`.
- Anonymous keyless-list entries are stored under collision-free `KeyAnonymous` (`#anonymous`, `#anonymous#1`, ...) and written with no key by plain, annotated, and blame serializers (and CLI tree diff). A literal operator name `default` is stored under `"default"` and written as `update default {`.
- Operator-supplied names on YANG keyless lists are validated against `[a-zA-Z0-9_][a-zA-Z0-9_.\-]*` in `ValidateListKey`, rejecting `#anonymous` and other non-identifier tokens on every input surface.
- Merging two containers holding anonymous keyless entries preserves all anonymous entries rather than overwriting the first.
- The web finder labels a named keyless entry with its operator name under `NamedItems` and keeps the content summary (or `#<pos>` fallback when empty) under `UnnamedItems` for anonymous entries, with distinct per-entry URLs for multiple anonymous blocks.
- The web add form offers an optional `name` input posting the `name` form field; leaving it empty allocates the next anonymous key (`#anonymous`, `#anonymous#1`, ...) rather than a sequential number (`1`, `2`), and commits as a bare `update {`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Insertion-ordered list key iteration | `internal/component/config/serialize.go` (`ListEntryKeys`) | Returns list entry keys in `tree.listOrder` order (with unrecorded keys appended sorted) across `serialize.go`, `serialize_annotated.go`, `serialize_blame.go`, `serialize_set.go`, and `cli/diff_tree.go` | Does not apply (already preserves insertion order in HEAD; retires the earlier sorting limitation) |
| Delegated tree-diff list rendering | `internal/component/cli/diff_tree.go` (`diffPrintedListEntry`) | Calls `config.WriteListEntry` directly to format list entries for tree-aware diff instead of duplicating key checks | Does not apply (already delegates to `WriteListEntry` in HEAD, so updating `WriteListEntry` automatically updates CLI diff) |
| Unified list-address codec and creator | `internal/component/config/list_key.go` (`ListPathStep`, `ParseListAddress`, `EnsureListEntry`, `ListKeyTokens`, `storedListKeyTokens`, `WalkListEntryPath`, `ResolveListEntryTarget`) | Reads list entry addresses, validates typed keys via `ValidateListKey`, preserves literal stored keys in `storedListKeyTokens`, and refuses rename/copy on keyless lists | Extends: uses `AnonymousListKey(node)` in `ListPathStep`, adds `ListAddress.Anonymous` so internal `#anonymous` replay bypasses operator pattern checks, and preserves `IsAnonymousKey` verbatim in `ListKeyTokens` |
| Schema-driven list key validation | `internal/component/config/leaf_value.go` (`ValidateListKey`) | Validates list keys against `ListNode.KeyLeaf` (emitting `configyang.PatternRefusal` operator errors) and falls back to `ValidateValue(node.KeyType, key)` | Extends: `yangToList` populates `ListNode.KeyLeaf` on YANG keyless lists with the identifier pattern `[a-zA-Z0-9_][a-zA-Z0-9_.\-]*`, reusing `ValidateListKey` with no new validator path |
| `ze:display-key` extension and `ListNode.DisplayKey` | `internal/component/config/yang/modules/ze-extensions.yang`, `yang_schema.go` (`hasDisplayKeyExtension`), `schema.go` (`ListNode.DisplayKey`), `web/fragment.go`, `web/handler_config_entry.go`, `web/view_fragment.go`, `web/component_add_form_overlay.templ` | Declares and scans a child leaf as a UI display label for keyless lists | Replaces (deletes): the entry key is the sole declaration of the name, so `leaf name`, `ze:display-key`, `hasDisplayKeyExtension`, and `DisplayKey` are removed (`ai/rules/no-layering.md`) |
| Keyless entry content summary | `internal/component/web/fragment.go` (`keylessEntrySummary`) | Summarizes a keyless entry from `DisplayKey`, then child list keys (e.g. NLRI families), then first non-empty leaf | Extends: removes the `DisplayKey` branch and runs `keylessEntrySummary` only for anonymous entries (`ListEntryName` false) |
| Reversible web path token transport | `internal/component/web/path_transport.go` (`encodeConfigPath`, `decodeConfigPath`) | Percent-encodes each config path token segment individually, safely transporting `#` as `%23` | Does not apply (used as-is; `#anonymous` and `#anonymous#1` round-trip through browser URLs without transport changes) |
| CLI positional `#N` index resolution | `internal/component/cli/editor.go` (`resolveListKey`), `editor_annotated.go` (`resolveMetaListKey`) | Maps `#<positive-int>` tokens to 1-based entries in `GetListOrdered`; returns `nil` for non-numeric suffixes | Does not apply (used as-is; `#anonymous` and `#anonymous#1` fail `strconv.Atoi` and fall through to direct map lookup without colliding with `#1`, `#2`) |
| Route extraction from `update` blocks | `internal/component/bgp/config/bgp_routes.go` (`extractRoutesFromTree`, `extractRoutesFromUpdateBlock`) | Iterates `GetListOrdered("update")` and extracts `attribute` and `nlri` only, ignoring the entry key | Does not apply (preserved untouched; proves block names reach no BGP message) |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "Config is YANG-modeled, and `ConfigProvider` is its single authority" | agrees | Retains `list update` in YANG with one authoritative entry-key identity read by CLI, web, and serializers through `internal/component/config` |
| `docs/architecture/api/update-syntax.md` | Config File Format | anchored to `internal/component/bgp/yang/ze-bgp-conf.yang` | "A config announces static routes with an `update` block" | agrees | Preserves `update` block route extraction (`attribute` and `nlri`) and leaves the API `update text` command surface untouched |
| `docs/architecture/bgp/filter-path-asn.md` | The config | anchored to `internal/component/bgp/yang/ze-bgp-conf.yang` | | silent | Anchored to `ze-bgp-conf.yang` for AS-path filter config; silent on `list update` naming, keyless list keys, and `ze:display-key` |
| `docs/architecture/config/environment-block.md` | Sections | anchored to `internal/component/bgp/yang/ze-bgp-conf.yang` | | silent | Anchored to `ze-bgp-conf.yang` for the `environment { }` block; silent on `list update` naming and keyless list keys |
| `docs/architecture/config/environment.md` | Overview | anchored to `internal/component/bgp/yang/ze-bgp-conf.yang` | | silent | Anchored to `ze-bgp-conf.yang` for environment variables and precedence; silent on `list update` naming and keyless list keys |
| `docs/architecture/config/syntax.md` | Custom YANG Extensions | `// Design:` of `parser.go`, `parser_list.go`, `list_key.go`, `setparser_refusal.go`, `schema.go`, `yang_schema.go`, `tree.go`, `serialize.go`, `serialize_annotated.go`, `serialize_blame.go`; anchored to `ze-bgp-conf.yang`, `ze-extensions.yang`, `fragment.go`; modified by this spec | "Display label for keyless list entries in web UI" | departs | Deletes the `ze:display-key` extension row and adds the `update <name> {` spelling and uniqueness rule under `Update Block (Ze-Native Routes)`, per Thomas's 2026-09-13 ruling retiring the `name` leaf |
| `docs/architecture/config/yang-config-design.md` | Extensions: `ze-extensions.yang` | `// Design:` of `leaf_value.go`, `editor.go`, `editor_walk.go`, `editor_annotated.go`, `completer.go`; anchored to `ze-bgp-conf.yang`, `ze-extensions.yang`, `yang_schema.go`, `schema.go`, `parser_list.go`, `tree.go`; modified by this spec | "Names the leaf the web interface shows for a keyless list entry" | departs | Deletes the `ze:display-key` row from the complete extension inventory because `extension display-key` is removed from `ze-extensions.yang`, per Thomas's 2026-09-13 ruling |
| `docs/architecture/core-design.md` | 19. Component Boundaries | anchored to `ze-bgp-conf.yang`, `ze-extensions.yang`, `yang_schema.go`, `tree.go` | "Each component under `internal/component/` is independently removable." | agrees | Keeps `internal/component/config` free of BGP imports and generic to keyless lists; `cli` and `web` continue calling into `config` |
| `docs/architecture/rib-transition.md` | Executive Summary | anchored to `internal/component/bgp/yang/ze-bgp-conf.yang` | | silent | Anchored to `ze-bgp-conf.yang` for RIB transition architecture; silent on `list update` naming and keyless list keys |
| `ai/doors/bgp-engine.md` | Where it sits | the door covering `internal/component/bgp/yang/ze-bgp-conf.yang` | "The engine speaks the protocol and holds no policy." | agrees | Update block names remain operator-only config metadata and never enter the BGP engine, wire messages, or protocol policy |
| `docs/architecture/web-components.md` | Page Layout | `// Design:` of `web/fragment.go`, `web/view_fragment.go`; anchored to `ze-extensions.yang`, `handler_config_entry.go` | "Named containers (lists with YANG keys) appear above the separator line. Unnamed containers and settings appear below." | agrees | Places named keyless list entries in `NamedItems` above the separator and anonymous entries in `UnnamedItems` below it, keeping `.templ` markup rules intact |
| `ai/doors/config-yang.md` | Who depends on it from outside | the door covering `internal/component/config/*` | "A leaf rename breaks every saved config and every script that writes it." | agrees | Checked the entire repository for `update { name ...; }` (zero hits); deleting the unused retired leaf breaks no saved config and is explicitly approved by the owner's 2026-09-13 ruling |
| `docs/architecture/hub-architecture.md` | Schema Storage | anchored to `internal/component/config/yang_schema.go` | | silent | Anchored to `yang_schema.go` for hub startup and external module publishing; silent on `ze:display-key` and keyless list entry identity |
| `docs/architecture/aaa-tacacs.md` | The schema merge defect this work uncovered | anchored to `internal/component/config/schema.go` | | silent | Anchored to `schema.go` for TACACS+ schema container merging; silent on keyless list naming and `ze:display-key` |
| `docs/architecture/ssh/fixit-bcrypt-hash-credential.md` | Decisions | anchored to `internal/component/config/schema.go` | | silent | Anchored to `schema.go` for `ze:bcrypt` credential hashing; silent on keyless list naming and `ze:display-key` |
| `docs/architecture/web-interface.md` | YANG Schema Integration | `// Design:` of `web/handler_config_entry.go`, `web/component_add_form_overlay.templ`; anchored to `schema.go`, `completer.go`, `fragment.go` | "The YANG schema drives the entire UI. No hardcoded field lists." | agrees | Derives the optional name input and finder labels generically from `ListNode.KeyName == ""` and `config.ListEntryName`, with no `update`-specific UI hardcoding |
| `docs/architecture/api/process-protocol.md` | Config Reload Protocol | anchored to `internal/component/config/tree.go` | | silent | Anchored to `tree.go` for plugin IPC config payloads; silent on `list update` naming and `ze:display-key` |
| `docs/architecture/config/change-file-structural-ops.md` | The shape | anchored to `internal/component/config/tree.go` | "Every structural wire record uses the configuration key quote codec in both directions;" | agrees | Anonymous keyless keys (`#anonymous`, `#anonymous#1`) are preserved by `ListKeyTokens`, quoted by `QuoteIfNeeded`, and replayed by `SetParser` and `ParseChangeFile` without changing structural directive grammar |
| `docs/architecture/storage-backends.md` | Opening and ownership | anchored to `internal/component/cli/editor.go` | | silent | Anchored to `editor.go` for configuration storage backend ownership and locking; silent on keyless list naming and `ze:display-key` |
| `ai/doors/commands-cli.md` | Boundaries | the door covering `internal/component/cli/*` | "Help, usage and completion are derived" | agrees | CLI completion derives named vs. `#N` display from the YANG schema (`entry.Key == ""`) and shared `config.IsAnonymousEntryKey` helper rather than duplicating `KeyDefault` checks |
| `ai/doors/api-surfaces.md` | Boundaries | the door covering `internal/component/web/*` | "Markup is `.templ`, with no inline script or style" | agrees | Updates `component_add_form_overlay.templ` and regenerates `component_add_form_overlay_templ.go` with no inline script or style |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `internal/le/site/testdata/*` and `test/*` | "Never import `internal/le` from product code" | agrees | Only regenerates `internal/le/site/testdata/` fixtures and adds `.ci`, `.wb`, and `.et` tests; product packages never import `internal/le` |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | This spec changes only keyless-list entry naming and `ze:display-key` removal; it leaves VPP/kernel data-plane validation and `ze:backend` gates untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | This spec does not alter backend-mismatch verdicts or introduce divergence between start, commit, reload, and `ze config validate` |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | This spec does not modify firewall YANG annotations or VPP firewall verification |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | This spec does not modify data-plane selector leaves, backend names, or `ze:backend` annotations |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | This spec touches only `list update` naming and does not touch BGP egress next-hop rewriting or export filter chains |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | This spec does not alter `policy { modify }` or `next-hop:*` chain references |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | This spec does not alter `policychain.Chain` compilation or export-chain auto-insertion |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | This spec does not alter export-chain next-hop folding or `wireOverride` provenance checks |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | This spec does not alter `policychain.Sets.NextHop` or import/export next-hop validation |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | This spec leaves static config route extraction (`extractRoutesFromUpdateBlock`) and locally originated next-hop preservation unchanged |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | This spec does not alter filter-chain implied-filter publication or `\| implied` rendering |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | This spec does not change daemon startup service-isolation or reload-refusal policy; schema/syntax parse refusals on `update` blocks remain uniform config-parse checks |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | This spec does not modify VPP/kernel mismatch handling at start, commit, reload, or `ze config validate` |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | This spec changes no engine IPC operation or plugin authorization profile |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | This spec changes no engine-to-plugin event queue or delivery behavior |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator writes `update <name> { ... }` or bare `update { ... }` in a config file, edits it in the CLI editor, or fills (or leaves blank) the optional `name` input in the web add form.
- Format at entry: hierarchical config text read by `internal/component/config/parser_list.go`, set-format / change-file lines read by `internal/component/config/setparser*.go`, CLI editor token paths read by `internal/component/cli/editor*.go`, or HTTP POST form field `name` read by `internal/component/web/handler_config_entry.go`.

### Transformation Path
1. Schema build (`internal/component/config/yang_schema.go` `yangToList`): for every YANG keyless list (`entry.Key == ""`), removes the `ze:display-key` scan and sets `ListNode.KeyLeaf` to a string `LeafNode` enforcing pattern `[a-zA-Z0-9_][a-zA-Z0-9_.\-]*` with a human-readable format description.
2. Hierarchical parse (`internal/component/config/parser_list.go`):
   - Bare `update { ... }` passes `AnonymousListKey(node)` (`KeyAnonymous = "#anonymous"` for keyless lists; `KeyDefault` for keyed lists) to `addParsedListEntry`.
   - Named `update <word> { ... }` validates `<word>` through `ValidateListKey` (accepting `default`, `drop-ssh-scan`, etc.; refusing `#anonymous`, braces, quotes, whitespace) and passes `<word>` verbatim.
   - `addParsedListEntry` allows duplicates on keyless lists only when `IsAnonymousListKey(node, key)` is true (`#anonymous`), so repeated bare blocks store as `#anonymous`, `#anonymous#1`, ... while any repeated operator name (including `default`) is refused with `duplicate list key for <list>: <name>`.
3. Tree storage and merge (`internal/component/config/tree.go`): `AddListEntry` disambiguates repeated anonymous keys as `#anonymous`, `#anonymous#1`, ... in `listOrder`; `Clone` copies them intact; `mergeFrom` allocates a fresh unique anonymous key when merging `IsAnonymousKey` entries so repeated parent blocks retain every bare block without overwriting.
4. Route extraction (`internal/component/bgp/config/bgp_routes.go`): `extractRoutesFromTree` walks `GetListOrdered("update")` and passes only `entry.Value` to `extractRoutesFromUpdateBlock`, so the entry key reaches no route and no BGP message.
5. Hierarchical serialization and CLI tree diff (`serialize.go`, `serialize_annotated.go`, `serialize_blame.go`, `cli/diff_tree.go`): `WriteListEntry`, `serializeAnnotatedList`, and `serializeBlameList` call `ListEntryName(node, key)`. When `named` is true (including literal `default`), they write `update <name> {`; when `named` is false (`#anonymous`, `#anonymous#1`, ...), they write bare `update {`. `cli/diff_tree.go` already delegates to `WriteListEntry`.
6. Set-format, change-file, CLI, and web addressing (`list_key.go`, `setparser_refusal.go`, `cli/editor*.go`, `cli/completer.go`, `web/fragment.go`, `web/handler_config_entry.go`):
   - `ListKeyTokens` preserves `IsAnonymousKey(key)` verbatim (`#anonymous`, `#anonymous#1`) so set lines, change files, CLI auto-select, and web URLs address each anonymous block distinctly.
   - `ListPathStep` and `ParseListAddress` mark anonymous steps and serialized `IsAnonymousKey` tokens on keyless lists with `ListAddress.Anonymous = true`; `EnsureListEntry` and `SetParser.lineKeyRefusal` admit those internal anonymous addresses while operator-facing surfaces (`parseList`, CLI `set`, web add form with non-empty `name`) continue calling `ValidateListKey` directly and refusing spoofed `#anonymous` input.
   - `cli/completer.go` calls `config.IsAnonymousEntryKey(keyless, entry.Key)`, showing `#N` for anonymous entries and the actual name (including `default`) for named entries.
   - `web/fragment.go` calls `config.ListEntryName(listNode, k)`: named entries show their name in `NamedItems`; anonymous entries show `keylessEntrySummary` (or `#<pos>` when empty) in `UnnamedItems`, each linked to its distinct `%23anonymous` / `%23anonymous%23N` URL.
   - `web/handler_config_entry.go` validates a non-empty `name` with `ValidateListKey` and, when `name` is empty on a keyless list, allocates `config.NextAnonymousListKey(existing)` so the new block is stored as anonymous and commits as bare `update {`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config text ↔ config tree | `parseList` assigns `KeyAnonymous` to bare keyless entries and validates named entries; `WriteListEntry`, `serializeAnnotatedList`, and `serializeBlameList` consult `ListEntryName` | No |
| Config tree ↔ set-format & per-user change files | `ListKeyTokens` preserves `#anonymous` / `#anonymous#N` through `QuoteIfNeeded`; `SetParser` and `ParseChangeFile` replay them with `ListAddress.Anonymous = true` | No |
| Config tree ↔ CLI editor, diff, and completion | `editor.go` and `editor_annotated.go` use `AnonymousListKey`; `editor_walk.go` uses `ListPathStep`; `diff_tree.go` delegates to `WriteListEntry`; `completer.go` uses `IsAnonymousEntryKey` | No |
| Config tree ↔ web finder and add form | `addFormOverlay` posts optional `name`; `HandleConfigAddWithAuthorizer` validates non-empty `name` or allocates `NextAnonymousListKey`; `buildListColumn` labels via `ListEntryName` and links via `encodeConfigPath` + `ListKeyTokens` | No |
| Config tree ↔ BGP reactor | `extractRoutesFromUpdateBlock` reads `attribute` and `nlri` only, never the entry key | No |

### Changed-Wire Census

No BGP protocol wire bytes change: `extractRoutesFromUpdateBlock`
(`internal/component/bgp/config/bgp_routes.go:80-120`) reads only `attribute`
and `nlri` children of each `update` entry and never reads the entry key or a
`name` leaf. The wires that change are the config text format, the set/change
file format, and the web add-form/finder HTTP transport for keyless list
entries.

**Search evidence:** Searched `internal/` and `docs/` with `grep -rn` for
`KeyDefault`, `DisplayKey`, `display-key`, `ValidateListKey`, `ListKeyTokens`,
`ParseListAddress`, and `StripListKeySuffix` on 2026-11-26, plus a brace-depth
scan of all `internal/**/*-conf.yang` files confirming `list update` in
`internal/component/bgp/yang/ze-bgp-conf.yang:436` is the sole keyless config
list. Positive controls matched `internal/component/config/serialize.go`
(`WriteListEntry`), `internal/component/config/parser_list.go` (`parseList`),
`internal/component/config/serialize_set.go` (`serializeSetList`),
`internal/component/config/setparser_refusal.go` (`lineKeyRefusal`),
`internal/component/web/component_add_form_overlay.templ` (`addFormOverlay`),
and `internal/component/web/handler_config_entry.go`
(`HandleConfigAddWithAuthorizer`).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Hierarchical config text (`update` block header and body) | writer | `internal/component/config/serialize.go` (`WriteListEntry`), `serialize_annotated.go` (`serializeAnnotatedList`), `serialize_blame.go` (`serializeBlameList`), `internal/component/cli/diff_tree.go` (`diffPrintedListEntry`) | All four emit list headers; `WriteListEntry`, `serializeAnnotatedList`, and `serializeBlameList` compare raw `key != KeyDefault` today; `diffPrintedListEntry` delegates to `WriteListEntry` | Change: route all three config writers through `ListEntryName(node, key)` so anonymous blocks write bare `update {` and named blocks (including `default`) write `update <name> {`; `diffPrintedListEntry` inherits automatically |
| Hierarchical config text (`update` block header and body) | reader | `internal/component/config/parser_list.go` (`parseList`, `addParsedListEntry`), plus operator config files and `test/**/*.ci` fixtures | `parseList` assigns `KeyDefault` to bare blocks and accepts `leaf name` via `ze-bgp-conf.yang`; repo scan found zero configs using `update { name ...; }` | Change: assign `AnonymousListKey(node)` to bare blocks, delete `leaf name` so `update { name x; }` is refused as an unknown field, validate supplied names with `ValidateListKey`, and refuse duplicate literal `default` while keeping bare blocks repeatable |
| Set-format and change-file text (keyless list entry address token) | writer | `internal/component/config/serialize_set.go` (`serializeSetList`, `serializeSetMetaList`), `serialize_annotated.go` (`serializeAnnotatedSetList`), `change_file.go` (`SerializeChangeFile`) | All call `setFormEntry` / `ListKeyTokens` -> `QuoteIfNeeded`; stripping `#N` today collapses multiple anonymous entries onto one token | Change: `ListKeyTokens` preserves `IsAnonymousKey(key)` verbatim (`#anonymous`, `#anonymous#1`), quoted by `QuoteIfNeeded` so distinct anonymous entries survive set-format and change-file writes |
| Set-format and change-file text (keyless list entry address token) | reader | `internal/component/config/setparser.go` (`walkAndSet`), `setparser_entries.go` (`walkAndSetListEntry`), `setparser_refusal.go` (`lineKeyRefusal`), `change_file.go` (`ParseChangeFile`, `materializeMetaPaths`) | `walkAndSetListEntry` validates via `lineKeyRefusal` -> `ValidateListKey` and creates entries via `ParseListAddress` -> `EnsureListEntry`; `materializeMetaPaths` passes `Tokens == 0` | Change: `ParseListAddress` marks keyless `IsAnonymousKey` tokens `Anonymous: true`; `EnsureListEntry` and `lineKeyRefusal` admit those internal replay tokens while rejecting all other invalid names |
| Web HTTP form & URL path (keyless list add and navigation) | writer | `internal/component/web/component_add_form_overlay.templ` (`addFormOverlay`), `internal/component/web/fragment.go` (`buildListColumn`), `internal/component/web/path_transport.go` (`encodeConfigPath`) | `addFormOverlay` currently posts `field:<display-key>` for keyless lists; `buildListColumn` builds `/show/.../<encoded-key>/` via `ListKeyTokens` + `encodeConfigPath` | Change: `addFormOverlay` posts optional `name` field for keyless lists; `buildListColumn` labels via `ListEntryName` and emits distinct `%23anonymous` / `%23anonymous%23N` links via `ListKeyTokens` |
| Web HTTP form & URL path (keyless list add and navigation) | reader | `internal/component/web/handler_config_entry.go` (`HandleConfigAddWithAuthorizer`), `internal/component/web/fragment.go` (`HandleFragment`), `internal/component/web/handler_config_walk.go` (`walkTree`), `internal/component/web/path_transport.go` (`decodeConfigPath`) | `HandleConfigAddWithAuthorizer` reads `r.FormValue("name")` and falls back to sequential `"1"`, `"2"` for keyless lists; `walkTree` resolves path tokens via `ListPathStep` | Change: validate non-empty `name` with `ValidateListKey` (refusing spoofed `#anonymous`), allocate `NextAnonymousListKey` when `name` is empty, and resolve `%23anonymous` / `%23anonymous%23N` through `ListPathStep` |

### Integration Points
- `config.KeyAnonymous` (`"#anonymous"`) - collision-free internal key root for anonymous keyless-list entries, declared beside `config.KeyDefault` in `internal/component/config/parser.go`.
- `config.AnonymousListKey`, `config.IsAnonymousKey`, `config.IsAnonymousEntryKey`, `config.ListEntryName`, `config.NextAnonymousListKey` - the single shared identity API in `internal/component/config`, consumed by `parser_list.go`, `tree.go`, `list_key.go`, `serialize.go`, `serialize_annotated.go`, `serialize_blame.go`, `setparser_refusal.go`, `cli/editor.go`, `cli/editor_annotated.go`, `cli/completer.go`, `web/fragment.go`, and `web/handler_config_entry.go`.
- `ListNode.KeyName` and `ListNode.KeyLeaf` - `KeyName` stays empty for `list update` (keeping the name optional, since YANG `key` makes a leaf mandatory), while `yangToList` populates `KeyLeaf` on every YANG keyless list with the `[a-zA-Z0-9_][a-zA-Z0-9_.\-]*` identifier pattern so `ValidateListKey` enforces it uniformly.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Names enter through `parseList`, `SetParser`, CLI editor, or web add form, validate via `ValidateListKey`, store in `config.Tree`, and leave through `ListEntryName` / `ListKeyTokens`; no surface reads config text directly |
| No unintended coupling (components stay isolated) | Yes | `internal/component/web` and `internal/component/cli` call into `internal/component/config` (the existing allowed direction); `config` holds zero BGP imports and neither `config`, `cli`, nor `web` names `update` |
| No duplicated functionality (extends existing, does not recreate) | Yes | Every surface calls the shared `config.ListEntryName` / `config.IsAnonymousEntryKey` / `config.AnonymousListKey` helpers, `cli/diff_tree.go` already delegates to `config.WriteListEntry`, and keyless name validation reuses `ValidateListKey` via `ListNode.KeyLeaf` |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Config parsing and serialization are off the BGP packet hot path; serializers keep writing through `textbuf.Buffer` |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No new command, view, family, or per-feature field is added; deleting `ListNode.DisplayKey` shrinks `ListNode`, and keyless-list behavior is driven generically by `ListNode.KeyName == ""` |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name (`ai/rules/principles.md`) | Yes | Searched `internal/component/config/`, `internal/component/cli/`, and `internal/component/web/` for `DisplayKey` and `KeyDefault`; every rule keys off `ListNode.KeyName == ""` (or YANG `entry.Key == ""`) and `IsAnonymousKey`, never hardcoding `"update"` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `list update` is the only keyless list the config parser reaches, so widening the duplicate-anonymous allowance and optional-name pattern to every YANG keyless list changes only update blocks | A brace-depth walk of every `internal/**/*-conf.yang` on 2026-11-26 returned exactly one hit: `internal/component/bgp/yang/ze-bgp-conf.yang:436: list update` | A second keyless config list would inherit the identifier pattern and repeatable-anonymous rule | Re-run the `-conf.yang` keyless-list check during implementation and confirm the list set | validated (2026-11-26) |
| A-2 | No config, test, doc, or fixture in the tree uses `update { name <x>; }` | A scan of `.ci`, `.conf`, `.et`, `.md`, and `.txt` returned zero hits on 2026-09-13; grep for `display-key` on 2026-11-26 found only the schema, Go plumbing, and the two extension doc rows | Deleting the leaf breaks a fixture, which is rewritten to `update <name> {` in this work | Scoped tests `./le test unit pkg path ./internal/component/bgp/config/`, `./le test functional parse`, and `./le test functional editor` after deleting `leaf name` | unvalidated |
| A-3 | Nothing reads the update entry key to decide routing behavior | `extractRoutesFromTree` and `extractRoutesFromUpdateBlock` (`internal/component/bgp/config/bgp_routes.go:80-120`) read `entry.Value` (`attribute` and `nlri`) only; owner ruling of 2026-09-13 confirms the name is operator memory | A rename would alter routing | AC-8 (`TestUpdateBlockNameDoesNotReachRoutes`) | unvalidated |
| A-4 | The web `name` form field carries the optional name for keyless lists without a new endpoint or handler route | `HandleConfigAddWithAuthorizer` (`internal/component/web/handler_config_entry.go:80`) already reads `r.FormValue("name")` before the keyless branch | Form submission would need a separate handler branch | AC-6, AC-7, `TestConfigAddKeylessEntryTakesOptionalName`, and `test/web/update-block-name.wb` | unvalidated |
| A-5 | `KeyAnonymous = "#anonymous"` cannot collide with any valid operator name or CLI positional `#N` selector | `[a-zA-Z0-9_][a-zA-Z0-9_.\-]*` forbids `#`; `resolveListKey` (`cli/editor.go:1046`) and `resolveMetaListKey` (`cli/editor_annotated.go:222`) match only `#<positive-int>` | An operator could spoof an anonymous key or `#1` could misresolve | `TestKeylessListRefusesSpoofedAnonymousKey`, `TestConfigTextRoundTrip`, and CLI completion/walk unit tests | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Deleting `ze:display-key` without replacing the `DisplayKey` condition in `allowsDuplicateParsedListEntries` makes every config with two bare update blocks fail to load | `test/parse/simple-v4.ci` and `test/parse/simple-v6.ci` fail immediately | Replace the condition with `node.KeyName == "" && IsAnonymousListKey(node, key)` in the same edit as deleting `DisplayKey`, guarded by AC-2 |
| R-2 | Operator input spoofs `#anonymous` or `#anonymous#1` to create an anonymous block through a name input or bypass duplicate checks | Unit test submitting `update "#anonymous" { }` or web form `name=#anonymous` succeeds | Enforce `ValidateListKey` (pattern `[a-zA-Z0-9_][a-zA-Z0-9_.\-]*`) on every operator-facing entry point (`parseList`, CLI `set`, web add form non-empty `name`); admit `ListAddress.Anonymous` only for anonymous steps and serialized set/change-file replay |
| R-3 | Multiple anonymous blocks in a web session or set-format file collapse into one entry when serialized or clicked in the finder | `ListKeyTokens` strips `#1` from `#anonymous#1`, causing two bare blocks to share one URL or one set path | `ListKeyTokens` preserves `IsAnonymousKey(key)` verbatim via `storedListKeyTokens`, tested in set-format round-trip and web finder tests |
| R-4 | Merging two parent blocks (e.g. repeated `bgp { }` or `peer p1 { }`) that each contain bare `update { }` blocks overwrites the first anonymous entry | `mergeFrom` assigns `t.lists["update"]["#anonymous"] = v["#anonymous"]` | Teach `mergeFrom` to allocate a fresh unique key via `addListEntryLocked` when `IsAnonymousKey(key)` is true |
| R-5 | The web golden fixture for the keyless add form (`add_form_overlay--keyless.html`) and the two published site fixtures (`published-configuration.md`, `published-yang-config-tree.json`) carry `DisplayKey` / `leaf name` and go red | `./le test unit pkg path ./internal/component/web/` and `./le test unit pkg path ./internal/le/site/` report golden mismatches | Regenerate `component_add_form_overlay_templ.go`, the web golden fixture, and both `internal/le/site/testdata/` fixtures in the same work and review every diff line |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Config load or save. A wrong duplicate-entry condition refuses every config with two bare update blocks (most of the BGP test corpus and interop scenarios). A wrong serializer emits a fabricated key into an operator's file on `ze config fmt -w` or drops a literal `default` name. A wrong `ListKeyTokens` collapses multiple bare blocks in set/change files or web finder links. No BGP protocol wire behavior changes either way |
| How is it reverted? | A single commit revert. Config text written with `update <name> {` and bare `update {` stays valid before and after; the only removed syntax is `update { name <x>; }`, which nothing in the tree uses |
| Who else touches this path? | `internal/component/config` serialization is shared by CLI show, tree diff, blame, and annotated views and by the web editor. `ze:display-key` has exactly one user in the tree (`list update`). Keyed lists keep `KeyDefault` untouched so `exabgp/migration`, `bgp/config/plugins.go`, and existing keyed-list tests are unaffected |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate` on a config holding `update drop-ssh-scan { ... }` and `update default { ... }` mixed with bare `update { }` blocks | → | `parseList` + `addParsedListEntry` + `ValidateListKey` | `test-update-block-name` (`test/parse/update-block-name.ci`) |
| `ze config fmt -w -` on a config holding `update default { }` and two bare `update { }` blocks | → | `WriteListEntry` through `ListEntryName` | `TestSerializeOmitsGeneratedListKey` |
| `ze config validate` on a config holding `update { name x; }` | → | deleted `leaf name` -> parser unknown-field refusal | `TestUpdateBlockRejectsRetiredNameLeaf` |
| Web finder GET `/show/bgp/peer/p1/update/` | → | `buildListColumn` through `ListEntryName` and `ListKeyTokens` | `TestBuildListColumnShowsUpdateBlockName` |
| Web add form POST `/config/add/bgp/peer/p1/update/` with a name and with an empty name | → | `HandleConfigAddWithAuthorizer` through `ValidateListKey` and `NextAnonymousListKey` | `test-web-update-block-name` (`test/web/update-block-name.wb`) |
| CLI `edit bgp peer p1 update <tab>` | → | `Completer.listKeyCompletions` through `config.IsAnonymousEntryKey` | `test-editor-update-block-name` (`test/editor/completion/bgp-peer-update-name.et`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A config holds `update drop-ssh-scan { attribute {} nlri {} }` | Config load succeeds, the entry is stored under key `drop-ssh-scan`, plain/annotated/blame serializers write `update drop-ssh-scan {`, CLI completion offers `drop-ssh-scan`, and the web finder lists `drop-ssh-scan` under `NamedItems` | None; every surface must read the one entry-key fact |
| AC-2 | A config holds two or more bare `update { }` blocks in one container (at `bgp`, `group`, or `peer` level, or merged across two parent blocks) | Config load succeeds, each bare block is stored as a distinct anonymous entry (`#anonymous`, `#anonymous#1`, ...) in insertion order, and all routes from every block are extracted | Keyed lists continue to use `KeyDefault` for their legacy anonymous form (`attach process`, `exabgp` `api`, `InlineListNode`) because this spec scopes collision-free `KeyAnonymous` to keyless lists |
| AC-3 | A config holds two update blocks with the same name — including two explicitly named `default` with bare anonymous blocks before or between them — or an invalid name such as `#anonymous` or `bad name` | Config load refuses duplicates with `duplicate list key for update: <name>` naming the line, and refuses invalid names via `ValidateListKey` pattern refusal (`[a-zA-Z0-9_][a-zA-Z0-9_.\-]*`) | Inactive duplicate entries remain permitted beside an active entry per existing `incomingInactive || existing.IsInactive()` rule in `addParsedListEntry` |
| AC-4 | A config holds `update { name drop-ssh-scan; ... }` | Config load refuses the file, reporting `unknown field in update: name` | None; `leaf name` is deleted rather than deprecated (`ai/rules/no-layering.md`) |
| AC-5 | `ze config fmt` or `ze config show` (plain, annotated, or blame) runs over a peer holding `update default { }`, `update drop-ssh-scan { }`, and two bare `update { }` blocks | The output writes `update default {`, `update drop-ssh-scan {`, and two bare `update {` blocks in insertion order, preserving every supplied name verbatim and emitting bare `update {` for every anonymous block | Existing comment and indentation formatting rules remain unchanged |
| AC-6 | An operator adds an update block in the web add form with a valid name (such as `drop-ssh-scan` or `default`) typed into the optional `name` input | The handler creates the entry under that exact key, redirects to it, the finder shows it under `NamedItems` with that name, and committing writes `update <name> {` | Invalid names typed in the `name` input (including `#anonymous`) are refused by `ValidateListKey` with an inline add-form error |
| AC-7 | An operator adds one or more update blocks in the web add form with the optional `name` input left empty | Each block is created under a distinct anonymous key (`#anonymous`, `#anonymous#1`, ...), the finder shows its content summary (or `#<pos>` when empty) under `UnnamedItems` with a distinct clickable URL per block, and committing writes bare `update {` for each block | Existing `keylessEntrySummary` child-list-key and first-leaf fallbacks remain for anonymous entries so unnamed blocks stay recognizable in the finder |
| AC-8 | An update block's name is changed (or added/removed) while its `attribute` and `nlri` children stay identical | `extractRoutesFromTree` produces byte-for-byte identical `StaticRoutes` and `PluginRoutes`, confirming the name reaches no BGP wire message | None; block identity is strictly operator memory |
| AC-9 | The full YANG schema and compiled tree are inspected | The loaded schema has `list update` with `KeyName == ""`, a `KeyLeaf` enforcing `[a-zA-Z0-9_][a-zA-Z0-9_.\-]*`, and zero `ze:display-key` declarations or `ListNode.DisplayKey` references across `internal/` and `docs/` | Prose in `plan/` historical records (outside `internal/` and `docs/`) may mention the retired extension by name as history |
| AC-10 | A config containing `update default { }`, `update drop-ssh-scan { }`, and two bare `update { }` blocks is round-tripped through plain, annotated, blame, and set/change-file serializers and re-parsed, and inspected via CLI diff and completion | Every round trip preserves both supplied names (`default` and `drop-ssh-scan`) and both distinct anonymous blocks; CLI completion offers `default`, `drop-ssh-scan`, `#N` for each anonymous entry, and CLI tree diff renders `update default {` and bare `update {` identically to `Serialize` | Set-format and change-file storage retain the quoted internal key token (`"#anonymous"`, `"#anonymous#1"`) so replay keeps multiple anonymous blocks distinct; hierarchical config text always writes bare `update {` |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Writes `update drop-ssh-scan { ... }`, `update default { ... }`, and two bare `update { }` blocks and runs `ze config fmt` | config text → `parseList` → `Tree` (`drop-ssh-scan`, `default`, `#anonymous`, `#anonymous#1`) → `WriteListEntry` (`ListEntryName`) → config text | `test/parse/update-block-name.ci` |
| 2 | Names two update blocks the same (including two `update default { }` blocks with bare blocks between them) and validates | config text → `parseList` → `addParsedListEntry` → duplicate-key refusal | `test/parse/update-block-name.ci` |
| 3 | Opens the web finder on a peer holding named (including `default`) and bare update blocks | `Tree` → `buildListColumn` (`ListEntryName`, `ListKeyTokens`) → finder column with `NamedItems` and distinct `UnnamedItems` URLs | `TestBuildListColumnShowsUpdateBlockName` |
| 4 | Adds a named update block and an unnamed update block in the web add form and commits | form POST → `HandleConfigAddWithAuthorizer` → `EditorManager` change file (`SetParser`) → commit → hierarchical config text | `test/web/update-block-name.wb` |
| 5 | Types `edit bgp peer p1 update ` and presses tab in the CLI | `Tree` → `Completer.listKeyCompletions` (`config.IsAnonymousEntryKey`) → completion list showing named keys (including `default`) and `#N` for bare entries | `test/editor/completion/bgp-peer-update-name.et` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUpdateBlockNameIsTheEntryKey` | `internal/component/config/parser_list_test.go` | `update drop-ssh-scan { }` and `update default { }` store entries under those keys while bare `update { }` stores under `KeyAnonymous` (AC-1, AC-2) | |
| `TestKeylessListAllowsSeveralAnonymousEntries` | `internal/component/config/parser_list_test.go` | Multiple bare blocks in one container and merged across two parent blocks stay distinct anonymous entries after `DisplayKey` is removed (AC-2) | |
| `TestKeylessListRefusesRepeatedName` | `internal/component/config/parser_list_test.go` | Repeated names — including two explicit `default` entries with anonymous blocks between them — are refused with `duplicate list key for update: <name>` (AC-3) | |
| `TestKeylessListRefusesSpoofedAnonymousKey` | `internal/component/config/parser_list_test.go` | `update "#anonymous" { }`, `update "-bad" { }`, and `update "bad name" { }` are refused by `ValidateListKey` against the real BGP schema (AC-3, R-2) | |
| `TestUpdateBlockRejectsRetiredNameLeaf` | `internal/component/bgp/config/bgp_routes_test.go` | `update { name x; }` is refused as `unknown field in update: name` against the real BGP schema (AC-4) | |
| `TestSerializeOmitsGeneratedListKey` | `internal/component/config/serialize_test.go` | Multiple anonymous entries are written as bare `update {` by plain, annotated, and blame serializers (AC-5) | |
| `TestSerializeWritesOperatorName` | `internal/component/config/serialize_test.go` | Named entries — including literal `default` mixed with bare blocks — are written as `update <name> {` by plain, annotated, and blame serializers (AC-5, AC-10) | |
| `TestConfigTextRoundTrip` | `internal/component/config/serialize_test.go` | Mixed `default`, named, and multiple bare blocks survive hierarchical parse → serialize → parse and set-format / change-file serialize → `SetParser` → hierarchical serialize with every identity intact (AC-10, R-3) | |
| `TestCompleterDistinguishesDefaultFromAnonymous` | `internal/component/cli/completer_test.go` | CLI completion offers `default` and `named` by name and `#N` for anonymous `KeyAnonymous` entries; CLI tree diff renders `update default {` and bare `update {` accurately (AC-10) | |
| `TestBuildListColumnShowsUpdateBlockName` | `internal/component/web/fragment_test.go` | Named entries (including `default`) show their name in `NamedItems`, anonymous entries show content summary (or `#<pos>`) in `UnnamedItems` with distinct `%23anonymous` / `%23anonymous%231` URLs (AC-1, AC-7) | |
| `TestConfigAddKeylessEntryTakesOptionalName` | `internal/component/web/handler_config_entry_test.go` | A posted name becomes the entry key, a spoofed `#anonymous` is refused, and an empty name creates distinct anonymous entries (`#anonymous`, `#anonymous#1`) that commit as bare `update {` (AC-6, AC-7) | |
| `TestUpdateBlockNameDoesNotReachRoutes` | `internal/component/bgp/config/bgp_routes_test.go` | The same block under different names or bare produces identical `StaticRoutes` and `PluginRoutes` (AC-8) | |
| `TestNoDisplayKeyExtensionRemains` | `internal/component/config/yang_schema_test.go` | No loaded module declares or uses `ze:display-key`, and `list update` carries `KeyName == ""` with the optional-name pattern on `KeyLeaf` (AC-9) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| update block name | identifier pattern `[a-zA-Z0-9_][a-zA-Z0-9_.\-]*` carried on `ListNode.KeyLeaf` for YANG keyless lists | `a`, `0`, `_`, `default` as valid identifiers | empty name (selects the bare anonymous block, not an error); leading `-`, `.`, `#`, or space is invalid and refused | N/A, no length ceiling in the schema |
| anonymous blocks in one container | 0 to N | N (no ceiling) | N/A (0 blocks is a container with no update block, which is legal) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-update-block-name` | `test/parse/update-block-name.ci` | Named (`drop-ssh-scan`, `default`) and multiple bare blocks in one peer load, format back with names preserved and bare blocks bare, reparse cleanly, and refuse a repeated `default` or ordinary name | |
| `test-update-block-name-retired-leaf` | `test/parse/update-block-name-retired-leaf.ci` | `update { name x; }` is refused with `unknown field in update: name` | |
| `test-web-update-block-name` | `test/web/update-block-name.wb` | An operator adds a named update block and an unnamed update block in the web, reads both in the finder, and commits | |
| `test-editor-update-block-name` | `test/editor/completion/bgp-peer-update-name.et` | Key completion offers `drop-ssh-scan` and `default` for named blocks and `#N` for bare ones | |

### Interop Tests (Scope: protocol)
Not applicable. Scope is `config` and no wire-visible behavior changes.
`internal/component/bgp/config/bgp_routes.go` reads only the `attribute` and
`nlri` children of an update block and never its key, and AC-8 asserts that a
rename leaves the announced routes identical. Interop scenarios that carry bare
`update { }` blocks, such as `test/interop/scenarios/bgp-update-delay-frr`,
keep loading under AC-2.

## Files to Modify
- `internal/component/bgp/yang/ze-bgp-conf.yang` - delete `leaf name` (and `ze:display-key;`) from `list update`; update `list update`'s `description` and `ze:help` to document the optional `update <name> {` spelling, uniqueness, and display-only semantics
- `internal/component/config/yang/modules/ze-extensions.yang` - delete `extension display-key` and its comment block
- `internal/component/config/parser.go` - declare `KeyAnonymous = "#anonymous"` beside `KeyDefault` and the shared identity helpers (`AnonymousListKey`, `IsAnonymousKey`, `IsAnonymousEntryKey`, `ListEntryName`, `NextAnonymousListKey`)
- `internal/component/config/yang_schema.go` - delete `hasDisplayKeyExtension` and the `DisplayKey` scan; in `yangToList`, populate `l.KeyLeaf` on YANG keyless lists (`entry.Key == ""`) with the identifier pattern `[a-zA-Z0-9_][a-zA-Z0-9_.\-]*` and readable format metadata
- `internal/component/config/schema.go` - delete `ListNode.DisplayKey`
- `internal/component/config/leaf_value.go` - ensure `ValidateListKey` uses `"name"` (or `lastWord(listPath)`) cleanly in refusals for keyless lists whose `KeyLeaf` is populated
- `internal/component/config/parser_list.go` - pass `AnonymousListKey(node)` for bare list blocks; update `addParsedListEntry` and `allowsDuplicateParsedListEntries` to check `IsAnonymousListKey(node, key)` instead of `KeyDefault` / `DisplayKey`, refusing duplicate literal `default` on keyless lists while permitting repeated anonymous entries
- `internal/component/config/list_key.go` - add `Anonymous bool` on `ListAddress`; use `AnonymousListKey(node)` in `ListPathStep`; mark keyless `IsAnonymousKey` tokens `Anonymous: true` in `ParseListAddress`; skip `ValidateListKey` in `EnsureListEntry` when `addr.Anonymous` is true; preserve `IsAnonymousKey` verbatim in `ListKeyTokens` via `storedListKeyTokens`
- `internal/component/config/setparser_refusal.go` - in `lineKeyRefusal`, admit serialized `IsAnonymousKey` tokens on keyless lists so set-format and change-file replay succeed while rejecting all other invalid names
- `internal/component/config/tree.go` - in `mergeFrom`, append `IsAnonymousKey` entries via a unique key rather than overwriting an existing anonymous entry when merging repeated parent blocks
- `internal/component/config/serialize.go` - route `WriteListEntry` and `inlineLeafListEntry` through `ListEntryName(node, key)`
- `internal/component/config/serialize_annotated.go` - route `serializeAnnotatedList` through `ListEntryName(node, key)`
- `internal/component/config/serialize_blame.go` - route `serializeBlameList` through `ListEntryName(node, key)`
- `internal/component/cli/editor.go` - use `config.AnonymousListKey` instead of `config.KeyDefault` for `*config.ListNode` anonymous lookups in `walkBareListFrom` and `walkSchemaNode`
- `internal/component/cli/editor_annotated.go` - use `config.AnonymousListKey` instead of `config.KeyDefault` for `*config.ListNode` anonymous metadata lookups
- `internal/component/cli/completer.go` - replace `isDefaultKey` with `config.IsAnonymousEntryKey(len(keyLeaves) == 0, entry.Key)` in `holdsListKey` and `listKeyCompletions`
- `internal/component/web/fragment.go` - in `buildListColumn`, consult `config.ListEntryName(listNode, k)` so named keyless entries render their name in `NamedItems` and anonymous entries render `keylessEntrySummary` (or `#<pos>` when empty) in `UnnamedItems`; delete the `DisplayKey` branch in `keylessEntrySummary`
- `internal/component/web/handler_config_entry.go` - validate non-empty `name` with `config.ValidateListKey`; when `name` is empty on a keyless list, allocate `config.NextAnonymousListKey(existing)` instead of a sequential number; remove `DisplayKey` from `HandleConfigAddForm`
- `internal/component/web/view_fragment.go` - delete `addFormData.DisplayKey`
- `internal/component/web/component_add_form_overlay.templ` - replace the `v.DisplayKey` branch with an optional `name` input posting `name="name"` (not `required`) for `v.Keyless`; regenerate `internal/component/web/component_add_form_overlay_templ.go`
- `internal/component/web/golden_fixtures_test.go` - remove `data.DisplayKey = "address"` from `webAddFormView`
- `internal/component/web/testdata/golden/component/add_form_overlay--keyless.html` - regenerated
- `internal/le/site/testdata/published-configuration.md` - regenerated without the retired `update` `leaf name`
- `internal/le/site/testdata/published-yang-config-tree.json` - regenerated without the retired `update` `leaf name`
- `docs/architecture/config/syntax.md` - document `update <name> {` and uniqueness under `Update Block (Ze-Native Routes)`; delete the `ze:display-key` row under `Custom YANG Extensions`
- `docs/architecture/config/yang-config-design.md` - delete the `ze:display-key` row under `Extensions: ze-extensions.yang`
- `docs/guide/configuration.md` - show the named `update <name> {` spelling and state that the name is operator memory reaching no BGP message
- `docs/guide/as112.md` - name the two update blocks its prose distinguishes in words

## Files to Create
- `test/parse/update-block-name.ci` - named (`drop-ssh-scan`, `default`) and multiple bare blocks load, format back cleanly, reparse identically, and refuse repeated names (including `default`)
- `test/parse/update-block-name-retired-leaf.ci` - `update { name x; }` is refused naming `name` as an unknown field
- `test/web/update-block-name.wb` - web add form creates named and unnamed update blocks, finder displays both, and commit writes `update <name> {` and bare `update {`
- `test/editor/completion/bgp-peer-update-name.et` - CLI completion offers named keys (including `default`) and `#N` for bare update blocks

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/bgp/yang/ze-bgp-conf.yang` deletes `leaf name`; `internal/component/config/yang/modules/ze-extensions.yang` deletes `extension display-key` |
| YANG validation constraints | Yes | `yangToList` (`internal/component/config/yang_schema.go`) attaches the retired leaf's `[a-zA-Z0-9_][a-zA-Z0-9_.\-]*` pattern to `ListNode.KeyLeaf` on YANG keyless lists so `ValidateListKey` enforces it |
| YANG custom validators | N-A | The identifier pattern is a native regex constraint; no `ze:validate` is needed |
| CLI commands/flags | N-A | No command or flag is added or changed; `edit bgp peer p1 update <name>` already resolves through `editor_walk.go` |
| CLI grammar (keyword before value) | N-A | `update <name>` is a list key after its list name, which is the existing grammar for every list |
| Editor autocomplete | Yes | `internal/component/cli/completer.go` switches from `isDefaultKey` to `config.IsAnonymousEntryKey` |
| Functional test for new RPC/API | N-A | No RPC or API is added; the four functional tests cover config parsing, web, and CLI completion |
| Pipe completeness | N-A | No command output is added or changed |
| Env var registration | N-A | No leaf under `environment/` is touched |
| Doctor check for runtime dependencies | N-A | No file path, socket, service, kernel module, port, or certificate is introduced |
| Doctor code per problem class | N-A | No new runtime warning, error log class, or doctor check is introduced; invalid/duplicate update names use the existing config parse/validation refusal path (`ValidateListKey` / `duplicate list key`) |
| Prometheus counters/metrics | N-A | No observable runtime state is added; the name reaches no runtime decision |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family, capability, or attribute is touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | The named spelling already parses; this work settles it as the single spelling and deletes the retired leaf. `docs/features.md` describes update blocks as route announcements, which does not change |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` and `docs/architecture/config/syntax.md`: the named spelling, the uniqueness rule, and the removed `ze:display-key` row |
| 3 | CLI command added/changed? | No | No command, flag, or exit code changes; `docs/guide/command-reference.md` has nothing to correct |
| 4 | API/RPC added/changed? | No | No RPC is touched. `docs/architecture/api/update-syntax.md` documents the `send bgp ... update` API command and notes bare `update` config blocks, which stay valid |
| 5 | Plugin added/changed? | No | No plugin registration, command, or IPC schema changes |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` (update block section) and `docs/guide/as112.md` |
| 7 | Wire format changed? | No | The name reaches no BGP wire message; AC-8 asserts it |
| 8 | Plugin SDK/protocol changed? | No | `InProcessConfigRouteParser` reads `attribute` and `nlri` tokens of a block, never its key |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Scope is `config`; no RFC requirement changes level or proof |
| 10 | Test infrastructure changed? | No | The four new tests use existing `.ci`, `.wb`, and `.et` runners with no runner changes |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` compares daemon features; no feature is added or removed |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md` deletes the `ze:display-key` row; `docs/architecture/config/syntax.md` deletes `ze:display-key` and documents `update <name> {`; `docs/architecture/web-interface.md` and `docs/architecture/web-components.md` were checked against HEAD and neither mentions `ze:display-key` or `leaf name` |
| 13 | Route metadata keys added/changed? | No | No metadata key is written or read |
| 14 | Prometheus counters added/changed? | No | None added or changed |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing registers or deregisters |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED via `./le spec citation anchors spec plan/release/fixes/spec-update-block-name-is-the-list-key.md` on 2026-11-26. Modified: `docs/architecture/config/syntax.md`, `docs/architecture/config/yang-config-design.md`, `docs/guide/configuration.md`, `docs/guide/as112.md`. Checked and unaffected: `docs/architecture/web-components.md` (DECLARED by `fragment.go`/`view_fragment.go`: describes component inventory and `NamedItems`/`UnnamedItems` layout, agrees with named-before-unnamed finder items), `docs/architecture/web-interface.md` (DECLARED by `handler_config_entry.go`/`component_add_form_overlay.templ`: describes schema-driven UI and URL segment encoding, does not mention `ze:display-key`), `docs/architecture/api/update-syntax.md` (API `update text` syntax; bare `update` config example stays valid), `docs/architecture/api/process-protocol.md` (plugin IPC protocol; mentions `tree.go` for config maps only), `docs/architecture/bgp/filter-path-asn.md` (AS-path filter config in `ze-bgp-conf.yang`), `docs/architecture/config/change-file-structural-ops.md` (structural change-file ops; agrees with quoted key codec), `docs/architecture/config/environment-block.md` and `docs/architecture/config/environment.md` (env block and vars), `docs/architecture/core-design.md` (architecture overview), `docs/architecture/hub-architecture.md` (hub startup and module loading), `docs/architecture/rib-transition.md` (RIB design), `docs/architecture/aaa-tacacs.md` and `docs/architecture/ssh/fixit-bcrypt-hash-credential.md` (mention `schema.go` for unrelated containers/leaves), `docs/architecture/storage-backends.md` (mentions `editor.go` for storage backend locking), `docs/config-migration.md` (ExaBGP migration; mentions `tree.go`), `docs/config-reference.md` (event types), `docs/features/bgp-protocol.md`, `docs/features/configuration.md`, `docs/features/plugins.md`, `docs/features/srv6.md`, `docs/guide/add-path.md`, `docs/guide/bfd.md`, `docs/guide/bgp-peering.md`, `docs/guide/bgp-policy.md`, `docs/guide/config-deactivate.md`, `docs/guide/config-editor.md`, `docs/guide/debugging-tools.md` (mentions `parser.go` for config debug tools), `docs/guide/docker.md`, `docs/guide/flowspec-route-reflector.md`, `docs/guide/irr-filtering.md`, `docs/guide/looking-glass-howto.md`, `docs/guide/plugins.md`, `docs/guide/policy-routing.md`, `docs/guide/redistribution.md`, `docs/guide/route-policy.md`, `docs/guide/route-reflection.md`, `docs/guide/status.md`, `docs/guide/ubuntu-build-install.md`, `docs/guide/web-interface.md`, `docs/contributing/documentation-testing.md`, and `docs/plugin-development/schema.md` (each mentions `ze-bgp-conf.yang`, `tree.go`, `parser_list.go`, or `completer.go` for unrelated features or completion mechanics that keep their shape) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/quickstart.md` and `docs/guide/healthcheck.md` show bare `update { }` blocks, which stay valid under AC-2; `docs/guide/configuration.md` adds the named `update <name> {` example, and `docs/guide/as112.md` names the two update blocks its prose distinguishes in words |

## Design presentation

### What changes

An operator names a BGP `update` block in one place only: as its list key (`update <name> { ... }`). The redundant `leaf name` and the `ze:display-key` YANG extension are deleted. Bare `update { ... }` blocks stay legal and repeatable everywhere (`bgp`, `group`, `peer`), and the literal name `default` becomes an ordinary, unique operator name that survives every round trip. Internally, anonymous keyless entries use a collision-free sentinel key (`#anonymous`, `#anonymous#1`, ...) and every surface — plain/annotated/blame serializers, CLI tree diff, CLI editor and completion, set-format and change-file replay, and the web finder and add form — reads one shared helper (`config.ListEntryName` / `config.IsAnonymousEntryKey`) instead of guessing from `"default"`.

### Before and after

1. **Formatting two bare update blocks and a block named `default`:**
   - Before: `ze config fmt` writes the second bare block as `update "default#1" {`, and collapses `update default {` with bare `update {` (dropping the `default` name and failing to refuse two explicit `update default {` blocks).
   - After: `ze config fmt`, `ze config show`, annotated, and blame views all write `update default {` for the named block and bare `update {` for every anonymous block, and `ze config validate` refuses two explicit `update default {` blocks with `duplicate list key for update: default`.
2. **Retired `name` leaf syntax:**
   - Before: `update { name drop-ssh-scan; attribute { ... } nlri { ... } }` loads silently while leaving the entry key at `default`.
   - After: `ze config validate` refuses it with `unknown field in update: name`.
3. **Web finder and add form on `bgp/peer/<p>/update/`:**
   - Before: the finder hides `update drop-ssh-scan` behind its NLRI family summary; the add form posts `field:name` into the retired leaf, and leaving it empty creates an entry keyed `"1"` that commits as `update 1 {`.
   - After: the finder lists `drop-ssh-scan` and `default` by name under `NamedItems` and lists bare blocks by content summary (or `#1`, `#2`) under `UnnamedItems` with distinct URLs; the add form asks for an optional `name` posting the entry key, and leaving it empty creates an anonymous entry that commits as bare `update {`.
4. **CLI tab completion (`edit bgp peer p1 update <tab>`):**
   - Before: a block explicitly named `default` is hidden behind `#N` because `isDefaultKey` treats `"default"` as anonymous.
   - After: completion offers `drop-ssh-scan` and `default` by name and `#N` only for genuinely anonymous blocks.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Add `key "name"` to `list update` in YANG | YANG `key` makes the key leaf mandatory, which would break every bare `update { }` block in existing configs, tests, and interop scenarios |
| Keep `leaf name` and reconcile it with the entry key at parse time | Violates `ai/rules/principles.md` (two declarations of one fact) and `ai/rules/no-layering.md` (retaining retired syntax beside its replacement); Thomas ruled on 2026-09-13 to retire `leaf name` |
| Track anonymity solely via an out-of-band `Tree.anonymous` boolean while still storing bare entries under `"default"` / `"default#1"` | Collides with an explicitly named `update default { }` in `t.lists["update"]`, `MetaTree`, `ToMap`, set-format replay, and web URLs |
| Use `#1`, `#2` as the internal map keys for anonymous entries | Collides with CLI positional `#N` resolution (`resolveListKey` indexes 1-based across all entries including named ones) |
| Keep a per-list opt-in extension in `allowsDuplicateParsedListEntries` after deleting `ze:display-key` | Introduces a central enumeration (`ai/rules/principles.md`); `node.KeyName == "" && IsAnonymousListKey(node, key)` is generic to keyless lists and `list update` is the only keyless config list |

### Documents departed from

1. `docs/architecture/config/syntax.md` — section `Custom YANG Extensions` (line 2334), statement `"Display label for keyless list entries in web UI"` (row for `ze:display-key`), and section `Update Block (Ze-Native Routes)` (line 1864). After this work, the `ze:display-key` row is removed from `Custom YANG Extensions`, and `Update Block (Ze-Native Routes)` documents the optional `update <name> { ... }` spelling, uniqueness per container (including `default`), and display-only semantics, implementing Thomas's 2026-09-13 ruling.
2. `docs/architecture/config/yang-config-design.md` — section `Extensions: \`ze-extensions.yang\`` (line 109), statement `"Names the leaf the web interface shows for a keyless list entry"` (row for `ze:display-key`). After this work, the `ze:display-key` row is deleted from the extension inventory so the table stays complete and exact against `ze-extensions.yang`.

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the entry points before changing behavior
   - Tests: `TestUpdateBlockNameIsTheEntryKey`, `TestBuildListColumnShowsUpdateBlockName`, `test/parse/update-block-name.ci`
   - Files: `internal/component/config/parser_list_test.go`, `internal/component/web/fragment_test.go`, `test/parse/update-block-name.ci`
   - Verify: the existing named parse succeeds while mixed named-`default`/anonymous round trip, web finder label, and `.ci` formatting assertions fail red before implementation.
2. **Phase: collision-free anonymous identity and shared readers** -- introduce `KeyAnonymous` and route parser, tree, serializers, set/change-file codec, and CLI through one helper
   - Tests: `TestUpdateBlockNameIsTheEntryKey`, `TestKeylessListRefusesRepeatedName`, `TestKeylessListRefusesSpoofedAnonymousKey`, `TestSerializeOmitsGeneratedListKey`, `TestSerializeWritesOperatorName`, `TestConfigTextRoundTrip`, `TestCompleterDistinguishesDefaultFromAnonymous`, `test/editor/completion/bgp-peer-update-name.et`
   - Files: `internal/component/config/parser.go`, `parser_list.go`, `list_key.go`, `setparser_refusal.go`, `tree.go`, `serialize.go`, `serialize_annotated.go`, `serialize_blame.go`, `internal/component/cli/editor.go`, `editor_annotated.go`, `completer.go`
   - Verify: plain, annotated, blame, CLI tree diff, and set/change-file paths preserve `default`, ordinary names, and multiple distinct anonymous blocks; spoofed `#anonymous` is refused on every operator entry point.
3. **Phase: delete the retired spelling** -- remove `leaf name`, `ze:display-key`, `hasDisplayKeyExtension`, and `ListNode.DisplayKey`, and attach the identifier pattern to YANG keyless `KeyLeaf`
   - Tests: `TestUpdateBlockRejectsRetiredNameLeaf`, `TestKeylessListAllowsSeveralAnonymousEntries`, `TestUpdateBlockNameDoesNotReachRoutes`, `TestNoDisplayKeyExtensionRemains`, `test/parse/update-block-name-retired-leaf.ci`
   - Files: `internal/component/bgp/yang/ze-bgp-conf.yang`, `internal/component/config/yang/modules/ze-extensions.yang`, `internal/component/config/yang_schema.go`, `internal/component/config/schema.go`, `internal/component/config/leaf_value.go`, `internal/component/config/parser_list.go`
   - Verify: the duplicate-anonymous condition in `allowsDuplicateParsedListEntries` updates in the same edit as deleting `DisplayKey` (R-1), and existing parse tests (`test/parse/simple-v4.ci`, `test/parse/simple-v6.ci`) stay green.
4. **Phase: web finder and add form read the one fact** -- update finder labels, optional add-form name input, templ codegen, and golden fixture
   - Tests: `TestBuildListColumnShowsUpdateBlockName`, `TestConfigAddKeylessEntryTakesOptionalName`, `test/web/update-block-name.wb`
   - Files: `internal/component/web/fragment.go`, `handler_config_entry.go`, `view_fragment.go`, `component_add_form_overlay.templ`, `component_add_form_overlay_templ.go`, `golden_fixtures_test.go`, `testdata/golden/component/add_form_overlay--keyless.html`
   - Verify: named entries (including `default`) show their name in `NamedItems`, anonymous entries show content summary (or `#<pos>`) in `UnnamedItems` with distinct URLs, and adding with an empty name commits as bare `update {`.
5. **Phase: documentation and published fixtures** -- update the architecture/guide pages and regenerate published site fixtures
   - Tests: scoped unit tests for `internal/component/config`, `internal/component/bgp/config`, `internal/component/cli`, `internal/component/web`, `internal/le/site`; functional tests `parse`, `web`, `editor`; `./le doc check all`; `./le doc check templ-output`; `./le spec citation anchors`
   - Files: `docs/architecture/config/syntax.md`, `docs/architecture/config/yang-config-design.md`, `docs/guide/configuration.md`, `docs/guide/as112.md`, `internal/le/site/testdata/published-configuration.md`, `internal/le/site/testdata/published-yang-config-tree.json`
   - Verify: `./le doc check all` and `./le spec citation anchors` are clean, and regenerated fixtures carry no trace of the retired `name` leaf.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-10 has an implementation at file:line, and AC-9 is verified by a tree-wide grep over `internal/` and `docs/` |
| Feature completeness | The name survives hierarchical config text, set/change files, CLI editor, CLI diff, CLI completion, web finder, web add form, and commit, with no surface reading a deleted field |
| Correctness | Anonymous identity (`#anonymous`, `#anonymous#1`, ...) survives parse, `Tree.Clone`, `Tree.mergeFrom`, set/change-file replay, and web URL transport without colliding with literal `default` or CLI `#1`/`#2` positional selectors |
| Naming | Shared helpers in `internal/component/config` name what they answer (`ListEntryName`, `IsAnonymousEntryKey`, `AnonymousListKey`, `NextAnonymousListKey`), and no caller re-implements `KeyDefault` guesses |
| Data flow | Nothing reads the update entry key to decide routing behavior; `extractRoutesFromUpdateBlock` is unchanged |
| Operator errors and doctor codes | Invalid operator-supplied names produce `ValidateListKey`'s standard `opererr` pattern refusal with its fix line; duplicate names produce `duplicate list key for update: <name>` with line number |
| Rule: `ai/rules/no-layering.md` | `leaf name` and `ze:display-key` are deleted, not deprecated, and no fallback reads a `name` leaf |
| Rule: `ai/rules/principles.md` | One shared operation answers the operator-visible name; no `update`-specific hardcoding exists in `config`, `cli`, or `web` |
| Rule: `ai/rules/documentation.md` | All four doc pages and both published fixtures update in phase 5 of this work, not in a follow-up |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No `ze:display-key` in product code or docs | `grep -rn "display-key" --include="*.yang" --include="*.go" --include="*.md" internal/ docs/` returns nothing |
| No `DisplayKey` field or symbol | `grep -rn "DisplayKey" --include="*.go" internal/` returns nothing |
| One shared entry-name API | Every serializer, CLI, and web reader calls `config.ListEntryName`, `config.IsAnonymousEntryKey`, or `config.AnonymousListKey` |
| Mixed named-`default` and bare round trip | Scoped unit tests in `internal/component/config`, `internal/component/cli`, `internal/component/web`, `internal/component/bgp/config`, `internal/le/site` pass |
| Four functional tests pass | `./le test functional parse`, `./le test functional web`, and `./le test functional editor` pass on the new tests |
| Generated templ and docs clean | `./le doc check templ-output` and `./le doc check all` pass |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Operator-supplied update block names from config files, CLI `set`, and web add form POSTs are validated by `ValidateListKey` against `[a-zA-Z0-9_][a-zA-Z0-9_.\-]*`, rejecting `#anonymous`, braces, quotes, whitespace, and newlines so no input can inject config syntax or spoof an anonymous sentinel |
| Output escaping | Serializers continue calling `QuoteIfNeeded` on emitted keys; templ escapes web finder labels automatically |
| Resource exhaustion | An unbounded number of anonymous entries is already possible; `NextAnonymousListKey` scans only existing entries in the single list |
| Error leakage | Duplicate-name and invalid-name refusals name only the operator's input and line number |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| A config in the corpus stops loading | R-1: inspect `allowsDuplicateParsedListEntries` before anything else |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- `config.KeyAnonymous = "#anonymous"` solves both collision problems at once: `[a-zA-Z0-9_][a-zA-Z0-9_.\-]*` forbids `#` (so no valid operator name can collide with it), and `resolveListKey` parses only `#<positive-int>` (so CLI `#1`/`#2` positional indexing never collides with it).
- `config.ListEntryKeys` already preserves insertion order in HEAD across all serializers, and `cli/diff_tree.go` already delegates list-entry formatting to `config.WriteListEntry`. Retiring both old work items keeps the change set minimal and avoids duplicating existing mechanisms.
- YANG cannot declare an optional key because `key` makes its leaf mandatory. Keeping `list update` keyless (`KeyName == ""`) while populating `ListNode.KeyLeaf` with the optional-name pattern in `yangToList` enforces native key validation without breaking bare `update { }` blocks or Go-constructed `List(TypeString, ...)` unit test schemas.
- Preserving `IsAnonymousKey` verbatim in `ListKeyTokens` (via existing `storedListKeyTokens`) prevents multiple anonymous blocks from collapsing in set-format serialization, per-user change files, CLI auto-select, and web finder URLs.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Represent anonymous keyless-list entries with `config.KeyAnonymous = "#anonymous"` (disambiguated as `#anonymous`, `#anonymous#1`, ...) while leaving `KeyDefault` for legacy keyed-list anonymous entries | Out-of-band `Tree.anonymous` flag over `"default"`; `#1`/`#2` keys | Avoids map-key collisions with literal `update default { }` across `Tree`, `MetaTree`, `ToMap`, change files, and web URLs, avoids colliding with CLI positional `#N` indexing, and avoids disturbing keyed-list callers of `KeyDefault` |
| Keep `list update` keyless in YANG (`KeyName == ""`) and attach the `[a-zA-Z0-9_][a-zA-Z0-9_.\-]*` pattern to `ListNode.KeyLeaf` in `yangToList` for every YANG keyless list | Add `key "name"` in YANG; put `pattern` directly on `list` (invalid YANG RFC 7950); hardcode `"update"` in `ValidateListKey` | Keeps bare `update { }` legal, stays valid RFC 7950 YANG, enforces the retired leaf's exact identifier grammar via existing `ValidateListKey`, and keeps Go-built `List(TypeString, ...)` test schemas (`KeyLeaf == nil`) working |
| `update <name> { }` is the one spelling; `leaf name` and `ze:display-key` are deleted (owner ruling, 2026-09-13) | Keep `leaf name` and reconcile at load | Eliminates dual declarations (`ai/rules/principles.md`) and layering (`ai/rules/no-layering.md`); zero configs in the tree use `update { name ...; }` |
| Unique names refused at load, including literal `default` (owner ruling, 2026-09-13) | Exempt `default` or allow repeated names | Separating `KeyAnonymous` from `"default"` enforces uniform uniqueness on every operator-supplied name while keeping bare anonymous blocks repeatable |
| One shared identity API (`ListEntryName`, `IsAnonymousEntryKey`, `AnonymousListKey`, `NextAnonymousListKey`) in `internal/component/config` | Per-caller `KeyDefault` / `StripListKeySuffix` checks | Single declaration of anonymity and display identity across all serializers, CLI, and web |
| Preserve `IsAnonymousKey` verbatim in `ListKeyTokens` and admit it via `ListAddress.Anonymous` during set/change-file replay while refusing it via `ValidateListKey` on operator inputs | Strip `#N` on all keys; allow `#anonymous` through `ValidateListKey` | Keeps multiple anonymous entries distinct across change files and web URLs while blocking operators from spoofing internal sentinel keys |

## Known Limitations
- `ze:display-key` is deleted rather than repurposed. A future keyless list that wants a content-derived label for anonymous entries still has `keylessEntrySummary`, which keeps its child-list-key and first-leaf fallbacks.
- `rename` and `copy` on entries of a keyless list remain refused (`ResolveListEntryTarget`), matching `docs/architecture/config/syntax.md`.

## RFC Documentation (Scope: protocol)

Not applicable. Scope is `config`, no protocol behavior is implemented or
changed, and AC-8 asserts that a block's name reaches no BGP message.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-update-block-name-is-the-list-key.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
