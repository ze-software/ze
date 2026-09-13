# Spec: update block name is the list key

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | config |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-13 |

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

The owner ruled on 2026-09-13 that `update <name> { ... }` is the spelling. The
`name` leaf form is retired. The owner ruled the same day that the name is
unique and a repeat is refused at config load, which is what the parser already
does. The name is operator memory: nothing keys behavior off it, it reaches no
BGP message, and it needs no lookup path and no registry. What it owes is that
it survives a read and write round trip and that the CLI and the web show it.

Three defects follow from the split, all measured with `bin/ze` on 2026-09-13:

1. The web finder never shows an operator's name. `buildListColumn` sets the
   display name to the entry key and then, for a list whose schema is keyless,
   overwrites it with `keylessEntrySummary` or with `#<key>`. An entry named
   `drop-ssh-scan` is labelled by its NLRI families.
2. `ze config fmt` and `ze config show` write the parser's internal
   disambiguation suffix into the operator's file. A config with two bare
   `update { }` blocks comes back with the second one as `update "default#1" {`.
   `serializeListBlocks` compares the raw key against `KeyDefault` and writes
   anything else, so the generated key escapes into config text.
3. The web add form prompts for the `ze:display-key` leaf when creating a
   keyless entry, so a name typed there lands in the leaf that is being retired,
   and an entry created with no name gets a fabricated sequential key (`1`,
   `2`), which under the new rule reads as a name the operator never typed.

The goal: one declaration of the name, which is the list entry key; the retired
leaf and the `ze:display-key` extension deleted with it; every surface reading
the one fact; and a bare `update { }` still legal, because the whole tree writes
it that way.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - the Update Block section and the YANG extension table
  → Decision: the page documents only the bare `update { attribute {} nlri {} }` form, so it owes the named spelling in the same work as the code.
  → Constraint: the extension table at line 1414 declares `ze:display-key` as the label mechanism for keyless list entries, and that row goes with the extension.
- [ ] `docs/architecture/config/yang-config-design.md` - the Ze YANG extension inventory
  → Constraint: the extension row at line 68 is a second declaration of the same mechanism and is deleted in this work.
- [ ] `docs/architecture/web-interface.md` - finder columns and the config entry handlers
  → Constraint: the finder column builder reads the config tree shape, so the entry key is already available to it and no new plumbing is owed.
- [ ] `ai/rules/principles.md` - one fact, one declaration; a central enumeration is recognized by what it answers for a case it does not name
  → Decision: "which key is the anonymous one" is currently spelled in five places, so this work leaves one.
- [ ] `ai/rules/no-layering.md` - X is deleted, then Y is implemented
  → Constraint: the `name` leaf and `ze:display-key` are deleted rather than left beside the entry key.

### RFC Summaries (Scope: protocol)
Not applicable. Scope is `config`. The name reaches no BGP message:
`extractRoutesFromUpdateBlock` (`internal/component/bgp/config/bgp_routes.go`)
reads the `attribute` container and the `nlri` list of each entry and never
reads the entry key or a `name` leaf.

**Key insights:** (minimal context to resume after compaction)
- The parser already stores the positional word as the entry key, for every list, keyed or not. No parser work is owed for the new spelling.
- The CLI is already correct. `diff_tree.go` strips the `#N` suffix before comparing against `KeyDefault`, and `completer.go` shows the key for a named entry and `#N` for an unnamed one. The web and `serialize.go` are the outliers.
- `list update` is the only keyless list in the config tree. A walk of every `.yang` under `internal/` finds eleven keyless lists, and the other ten are rpc input and output lists in `ze-system-api.yang`, `ze-plugin-api.yang`, `ze-bgp-api.yang` and `ze-rib-api.yang`, which the config parser never reaches.
- `allowsDuplicateParsedListEntries` is what keeps two bare `update { }` blocks legal, and it reads `DisplayKey` to decide. Deleting the extension without replacing that condition makes every config with two bare blocks fail to load, and the tree holds dozens.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/config/parser_list.go` - `parseList` takes the word after a list name as the entry key and calls `ValidateListKey`; an immediate `{` gives the entry `KeyDefault`. `addParsedListEntry` refuses a repeated key unless `allowsDuplicateParsedListEntries` allows it, which for a keyless list means the list carries a `DisplayKey` and the key is `KeyDefault`.
- [ ] `internal/component/config/schema.go` - `ValidateListKey` validates against `KeyLeaf` when the list has one, and against `KeyType` otherwise, which accepts any word on a keyless list. `ListNode.DisplayKey` holds the `ze:display-key` leaf name.
- [ ] `internal/component/config/yang_schema.go` - `hasDisplayKeyExtension` matches the extension keyword, and the loader scans the children of a keyless list for it and stores the first match in `DisplayKey`.
- [ ] `internal/component/config/tree.go` - `AddListEntry` appends `#N` to a repeated key so both entries survive, and `listOrder` keeps insertion order.
- [ ] `internal/component/config/serialize.go` - `serializeListBlocks` sorts the keys, then writes the key after the list name for every key other than the literal `KeyDefault`, so a generated `default#1` is written out. `StripListKeySuffix` exists in the same file and is not used there.
- [ ] `internal/component/config/serialize_annotated.go` - strips the suffix into `displayKey`, then still tests the raw `key` against `KeyDefault`, so a `default#1` entry is written as `update default`.
- [ ] `internal/component/config/serialize_blame.go` - the same pair of lines as the annotated writer, with the same result.
- [ ] `internal/component/cli/diff_tree.go` - strips the suffix and compares the STRIPPED key against `KeyDefault`, which is the correct shape and the one this spec adopts everywhere.
- [ ] `internal/component/cli/completer.go` - `isDefaultKey` answers for `KeyDefault` and `KeyDefault#N`; the key completion shows the key for a named entry and `#N` for an unnamed one.
- [ ] `internal/component/cli/editor_walk.go` - a path element that is not a schema child of the list is taken as the entry key, so `edit bgp peer p1 update drop-ssh-scan` already reaches a named block; a path that stops at the list name uses `KeyDefault`.
- [ ] `internal/component/web/fragment.go` - `buildListColumn` sets the display name to the key, then for a keyless schema replaces it with `keylessEntrySummary` or `#<key>`, and files the entry under `UnnamedItems` when the summary is empty. `keylessEntrySummary` prefers the `DisplayKey` leaf, then child list keys, then the first non-empty leaf.
- [ ] `internal/component/web/handler_config_entry.go` - `HandleConfigAddWithAuthorizer` takes the entry key from the `name` form field when one is posted, and otherwise gives a keyless list a sequential numeric key. `HandleConfigAddForm` passes `DisplayKey` to the overlay.
- [ ] `internal/component/web/component_add_form_overlay.templ` - for a keyless list the overlay renders an input named `field:<display-key>`, which sets the leaf rather than the key, and renders nothing at all when the list carries no display key.
- [ ] `internal/component/web/view_fragment.go` - `addFormData.DisplayKey` carries the leaf name to the overlay.
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - `list update` is keyless and carries `leaf name` with `ze:display-key`, described as an optional display-only label.
- [ ] `internal/component/config/yang/modules/ze-extensions.yang` - declares `extension display-key`, whose only user in the tree is that leaf.
- [ ] `internal/component/bgp/config/bgp_routes.go` - `extractRoutesFromUpdateBlock` reads `attribute` and `nlri` from each entry and reads neither the entry key nor a `name` leaf.
- [ ] `test/parse/simple-v6.ci` - two bare `update { }` blocks in one peer, the spelling the tree uses everywhere.

Measured with `bin/ze` on 2026-09-13, on a peer holding `update drop-ssh-scan`,
`update scrubbing` and two bare `update { }` blocks:
- `ze config validate` exits 0 and reports no error.
- `ze config fmt` and `ze config show` both write the second bare block as `update "default#1" {`.
- Renaming `scrubbing` to `drop-ssh-scan` makes validate exit 1 with `line 21: duplicate list key for update: drop-ssh-scan`.
- A config using `update { name drop-ssh-scan; ... }` validates and formats unchanged, which is the spelling being retired.

**Behavior to preserve:** (unless the user explicitly said to change it)
- `update { attribute {} nlri {} }` with no name stays legal at the bgp, group and peer levels, and several of them in one container stay legal. Dozens of `.ci`, `.conf` and interop scenario files depend on it, including seven bare blocks in `test/parse/simple-v4.ci`.
- A repeated name is refused at load, with the existing message `duplicate list key for update: <name>`.
- The routes an update block produces do not change. `extractRoutesFromUpdateBlock` keeps reading `attribute` and `nlri` only, and entries keep being walked in insertion order.
- The CLI key completion keeps showing `#N` for an unnamed entry and the key for a named one.
- Every other list keeps its current key handling. `list update` is the only keyless list the config parser reaches.

**Behavior to change:**
- `update { name <x>; }` stops parsing. The leaf is gone, so the parser answers `unknown field in update: name`.
- A keyless list entry whose stored key is the anonymous one is written with no key at all, by all three serializers. `update "default#1" {` and `update default {` stop being written.
- The web finder labels an update entry with its name when it has one, and keeps the content summary only for an unnamed one.
- The web add form asks for an optional name and posts it as the entry key. An entry created with no name gets the anonymous key, not a sequential number.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator writes `update <name> { ... }` in a config file, or types it in the CLI editor, or fills the name field in the web add form.
- Format at entry: config text read by `internal/component/config/parser_list.go`, a CLI editor path, or an HTTP POST form field.

### Transformation Path
1. `parseList` reads the word after `update` and calls `ValidateListKey`, which checks it against the list's key type.
2. `addParsedListEntry` refuses a repeat of an existing name, and lets two anonymous entries through.
3. `Tree.AddListEntry` stores the entry under that key, appending `#N` when the key repeats, and records insertion order.
4. `extractRoutesFromUpdateBlock` reads `attribute` and `nlri` from the entry and ignores the key, so the name reaches no route and no BGP message.
5. On the way out, the three serializers and the CLI diff view ask one helper what an entry's operator-visible name is, and write the list name alone when there is none.
6. The web finder column and the add form ask the same helper, so the label the operator sees is the key they typed.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config text ↔ config tree | The entry key is the name; parse and serialize are inverses of each other | No |
| Config tree ↔ CLI editor and diff | `editor_walk` takes the path element as the key; `diff_tree` shows the stripped key | No |
| Config tree ↔ web finder and add form | HTTP form field `name` becomes the entry key; the finder column shows it | No |
| Config tree ↔ BGP reactor | `extractRoutesFromUpdateBlock` reads attribute and nlri only, so nothing keys off the name | No |

### Integration Points
- `config.StripListKeySuffix` - already the repository's answer to a generated key, and the base of the one helper this work leaves behind.
- `config.KeyDefault` - the anonymous key, currently compared against in five places with three different meanings.
- `ListNode.KeyName` - stays empty for `list update`, which is what keeps the name optional. YANG has no way to declare an optional key, so the keyless list plus an optional stored key IS the declaration.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The name enters through `parseList` and leaves through the serializers; no surface reads config text directly |
| No unintended coupling (components stay isolated) | Yes | `internal/component/web` and `internal/component/cli` both call into `internal/component/config`, which is the existing direction; neither learns about `update` by name |
| No duplicated functionality (extends existing, does not recreate) | Yes | The work removes four copies of the anonymous-key test and keeps the one in `internal/component/config`; `StripListKeySuffix` is reused rather than reimplemented |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Config parsing is not a hot path; the serializers keep writing through `textbuf.Buffer` |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | Nothing is registered and nothing is added; `ListNode` loses a field |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | No new name is introduced. The lists searched for `display-key` are every `.yang`, `.go`, `.html` and `.md` under the checkout (one YANG user, one extension declaration, two doc tables, two published site fixtures); the lists searched for the anonymous-key test are every `KeyDefault` reference under `internal/`, which the helper now derives from |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `list update` is the only keyless list the config parser reaches, so widening the duplicate-entry allowance to every keyless list changes only update blocks | A brace-depth walk of every `.yang` under `internal/` on 2026-09-13 found eleven keyless lists, ten of them rpc input or output lists | The allowance silently accepts a second anonymous entry where a duplicate was an operator error, in a list nobody looked at | Re-run the keyless-list walk during implementation and name every list it finds | unvalidated |
| A-2 | No config, test, doc or fixture in the tree uses `update { name <x>; }` | A scan of every `.ci`, `.conf`, `.et`, `.md` and `.txt` for a `name` leaf inside an update block returned zero hits on 2026-09-13 | Deleting the leaf breaks a test, which then has to be rewritten to the new spelling in this work | `./le verify current mode full` after the leaf is deleted | unvalidated |
| A-3 | Nothing reads the update entry key to decide behavior | `extractRoutesFromUpdateBlock` (`internal/component/bgp/config/bgp_routes.go`) reads `attribute` and `nlri` only; the owner's ruling of 2026-09-13 says the name is operator memory | A name change would alter routing, and the name would be an identifier rather than a mnemonic | AC-8: renaming a block leaves the announced routes byte-identical | unvalidated |
| A-4 | The `name` form field the web add form already posts can carry the optional name for a keyless list with no handler change to the key path | `HandleConfigAddWithAuthorizer` appends the `name` form value to the path before it looks at whether the list is keyless | The handler needs a keyless branch of its own | AC-6 and its web functional test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Deleting `ze:display-key` without replacing the `DisplayKey` condition in `allowsDuplicateParsedListEntries` makes every config with two bare update blocks fail to load | `test/parse/simple-v4.ci` and `test/parse/simple-v6.ci` go red on the first run | Change the condition in the same edit as the extension deletion, and keep AC-2 as the test that names it |
| R-2 | Suppressing the key when its stripped form is the anonymous one hides a legitimate entry named `default` | A config with a block named `default` round trips to an unnamed block | The name pattern accepts `default`, so the implementation must decide on the STRIPPED key equalling the anonymous constant, and the .ci covers a block named `default` |
| R-3 | `serializeListBlocks` sorts entry keys, so a rewrite reorders update blocks and naming a block moves it. This is a defect of every list, not of this work | `ze config fmt` on the measurement config put the two bare blocks before the two named ones, reversing the file | Out of scope here and named in Known Limitations; this spec asserts the name and the absence of a generated key, not the order |
| R-4 | The web golden fixture for the keyless add form and the two published site fixtures carry the retired leaf, so they go red | `./le verify current mode full` reports golden mismatches | Regenerate both in the same work and read the diff before accepting it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Config load. A wrong duplicate-entry condition refuses every config with two bare update blocks, which is most of the BGP test corpus and every interop scenario that announces routes. A wrong serializer writes a fabricated name into an operator's file on `ze config fmt -w`. No wire behavior changes either way |
| How is it reverted? | A single commit revert. The config text an operator writes stays valid before and after, with the one exception of `update { name <x>; }`, which nothing in the tree uses |
| Who else touches this path? | `internal/component/config` serialization is shared by the CLI show, diff, blame and annotated views and by the web editor. `ze:display-key` has exactly one user in the tree. No open spec in `plan/` names `display-key` except `plan/immediate/spec-generated-command-usage.md`, which mentions it in passing |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate` on a config holding `update drop-ssh-scan { ... }` | → | `parseList` stores the word as the entry key | `test-update-block-name` (`test/parse/update-block-name.ci`) |
| `ze config fmt -w -` on a config holding two bare `update { }` blocks | → | `serializeListBlocks` through the one entry-name helper | `TestSerializeOmitsGeneratedListKey` |
| `ze config validate` on a config holding `update { name x; }` | → | the deleted leaf, so the parser's unknown-field path | `TestUpdateBlockRejectsRetiredNameLeaf` |
| Web finder GET `/show/bgp/peer/p1/update/` | → | `buildListColumn` | `TestBuildListColumnShowsUpdateBlockName` |
| Web add form POST `/config/add/bgp/peer/p1/update/` with a name | → | `HandleConfigAddWithAuthorizer` | `test-web-update-block-name` (`test/web/update-block-name.wb`) |
| CLI `edit bgp peer p1 update <tab>` | → | `Completer` key completion through the shared helper | `test-editor-update-block-name` (`test/editor/completion/bgp-peer-update-name.et`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A config holds `update drop-ssh-scan { attribute {} nlri {} }` | It loads, and the block's name is `drop-ssh-scan` everywhere it is shown |
| AC-2 | A config holds two or more bare `update { }` blocks in one container | It loads, and each block is a separate entry, at the bgp, group and peer levels |
| AC-3 | A config holds two update blocks with the same name | It is refused at load, naming the repeated name and the line |
| AC-4 | A config holds `update { name drop-ssh-scan; }` | It is refused, naming `name` as an unknown field of `update` |
| AC-5 | `ze config fmt` or `ze config show` runs over a config holding named and bare blocks | Each named block is written as `update <name> {`, each bare block as `update {`, and no generated key such as `default`, `default#1` or a sequential number appears |
| AC-6 | An operator adds an update block in the web with a name typed in the form | The block's entry key is that name, the finder lists it under that name, and the committed config text carries `update <name> {` |
| AC-7 | An operator adds an update block in the web with the name left empty | The block is created as an anonymous entry, the finder shows the content summary for it, and the committed config text carries a bare `update {` |
| AC-8 | The name of an update block is changed and nothing else is | The routes the block announces are unchanged, and no BGP message differs |
| AC-9 | The schema is loaded | No YANG module in the tree declares or uses `ze:display-key`, and `ListNode` carries no display-key field |
| AC-10 | A config file is read and written back | Every update block name the operator typed is read back as typed, and no name the operator did not type appears |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Writes `update drop-ssh-scan { ... }` and runs `ze config fmt` | config text → `parseList` → tree entry key → `serializeListBlocks` → config text | `test/parse/update-block-name.ci` |
| 2 | Names two update blocks the same and loads the config | config text → `parseList` → `addParsedListEntry` → refusal | `test/parse/update-block-name.ci` |
| 3 | Opens the web finder on a peer's update blocks | tree → `buildListColumn` → finder column | `TestBuildListColumnShowsUpdateBlockName` |
| 4 | Adds a named update block in the web and commits | form POST → `HandleConfigAddWithAuthorizer` → editor tree → commit → config text | `test/web/update-block-name.wb` |
| 5 | Types `edit bgp peer p1 update` and presses tab in the CLI | tree → `Completer` → completion list | `test/editor/completion/bgp-peer-update-name.et` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUpdateBlockNameIsTheEntryKey` | `internal/component/config/parser_list_test.go` | `update drop-ssh-scan { }` stores the entry under that key (AC-1) | |
| `TestKeylessListAllowsSeveralAnonymousEntries` | `internal/component/config/parser_list_test.go` | Two bare blocks are two entries after the `DisplayKey` condition is gone (AC-2) | |
| `TestKeylessListRefusesRepeatedName` | `internal/component/config/parser_list_test.go` | A repeated name is refused with the existing message (AC-3) | |
| `TestUpdateBlockRejectsRetiredNameLeaf` | `internal/component/bgp/config/bgp_routes_test.go` | `update { name x; }` is refused as an unknown field against the real BGP schema (AC-4) | |
| `TestSerializeOmitsGeneratedListKey` | `internal/component/config/serialize_test.go` | A second anonymous entry is written with no key by all three serializers (AC-5) | |
| `TestSerializeWritesOperatorName` | `internal/component/config/serialize_test.go` | A named entry is written as `update <name> {`, including a block named `default` (AC-5, R-2) | |
| `TestConfigTextRoundTrip` | `internal/component/config/serialize_test.go` | Parse, serialize and parse again gives the same set of block names (AC-10) | |
| `TestBuildListColumnShowsUpdateBlockName` | `internal/component/web/fragment_test.go` | A named entry is labelled by its key and an unnamed one by its summary (AC-1) | |
| `TestConfigAddKeylessEntryTakesOptionalName` | `internal/component/web/handler_config_entry_test.go` | A posted name becomes the entry key, and an empty name creates an anonymous entry rather than a numbered one (AC-6, AC-7) | |
| `TestUpdateBlockNameDoesNotReachRoutes` | `internal/component/bgp/config/bgp_routes_test.go` | The same block under two different names produces identical routes (AC-8) | |
| `TestNoDisplayKeyExtensionRemains` | `internal/component/config/yang_schema_test.go` | No loaded module declares or uses the extension (AC-9) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| update block name | the leaf pattern it replaces: first character a letter, a digit or an underscore, then letters, digits, underscore, hyphen, dot | `a` and `0` as one-character names | the empty name, which is the bare block and is not an error | N/A, the name has no length ceiling in the schema |
| anonymous blocks in one container | 1 to N | N (no ceiling) | 0, which is the container with no update block and is legal | N/A |

The name has no numeric range. The row above records the boundary that exists,
which is the pattern the retired leaf declared and which the key validation must
now carry, so a name the leaf would have refused is still refused.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-update-block-name` | `test/parse/update-block-name.ci` | Named and bare blocks in one peer load, format back unchanged, and a repeated name is refused | |
| `test-update-block-name-retired-leaf` | `test/parse/update-block-name-retired-leaf.ci` | `update { name x; }` is refused with a message naming the field | |
| `test-web-update-block-name` | `test/web/update-block-name.wb` | An operator adds a named update block in the web and reads the name back in the finder | |
| `test-editor-update-block-name` | `test/editor/completion/bgp-peer-update-name.et` | Key completion offers the name of a named block and `#1` for a bare one | |

The `.ci` discrimination, one assertion per property: the fmt assertion names
`update drop-ssh-scan {`, which no input line carries in that form after the
serializer normalizes the file; the reject names `default`, which is what the
current serializer writes and which no correct output carries; the refusal case
asserts the exit code and the message text, which a parser that accepts a
repeat cannot produce.

### Interop Tests (Scope: protocol)
Not applicable. Scope is `config` and no wire-visible behavior changes.
`internal/component/bgp/config/bgp_routes.go` reads the attribute and nlri
children of an update block and never its key, and AC-8 asserts that a rename
leaves the announced routes identical. Interop scenarios that carry bare
`update { }` blocks, such as `test/interop/scenarios/bgp-update-delay-frr`,
keep loading under AC-2.

## Files to Modify
- `internal/component/bgp/yang/ze-bgp-conf.yang` - delete `leaf name` from `list update`; move the name's pattern and its help text onto the list itself so the named spelling is documented where an operator meets it
- `internal/component/config/yang/modules/ze-extensions.yang` - delete `extension display-key` and its comment block
- `internal/component/config/yang_schema.go` - delete `hasDisplayKeyExtension` and the keyless-list scan that fills `DisplayKey`
- `internal/component/config/schema.go` - delete `ListNode.DisplayKey`; make `ValidateListKey` apply the update list's name pattern to a keyless list's key
- `internal/component/config/parser_list.go` - `allowsDuplicateParsedListEntries` allows a repeated anonymous key on any keyless list, which is what keyless means, and stops reading `DisplayKey`
- `internal/component/config/serialize.go` - one helper answering, for a stored key, the operator-visible name and whether the entry has one, built on `StripListKeySuffix`; `serializeListBlocks` writes the key through it
- `internal/component/config/serialize_annotated.go` - use the helper in place of the stripped-key and raw-key pair
- `internal/component/config/serialize_blame.go` - the same
- `internal/component/cli/diff_tree.go` - use the helper in place of its own copy of the test
- `internal/component/cli/completer.go` - `isDefaultKey` becomes a call to the helper
- `internal/component/web/fragment.go` - `buildListColumn` shows the entry's name when it has one and the content summary only when it does not; `keylessEntrySummary` loses its `DisplayKey` preference
- `internal/component/web/handler_config_entry.go` - the add form offers an optional name for a keyless list; an entry created with no name takes the anonymous key rather than a sequential number
- `internal/component/web/view_fragment.go` - `addFormData` loses `DisplayKey`
- `internal/component/web/component_add_form_overlay.templ` - a keyless list renders the name input, not required, posting the `name` field the keyed branch already posts; regenerate `component_add_form_overlay_templ.go`
- `internal/component/web/golden_fixtures_test.go` - the keyless add-form fixture stops setting a display key
- `internal/component/web/testdata/golden/component/add_form_overlay--keyless.html` - regenerated
- `internal/le/site/testdata/published-configuration.md` - regenerated; it publishes the retired leaf's description
- `internal/le/site/testdata/published-yang-config-tree.json` - regenerated for the same reason
- `docs/architecture/config/syntax.md` - the Update Block section gains the `update <name> {` spelling and the uniqueness rule; the extension table loses the `ze:display-key` row
- `docs/architecture/config/yang-config-design.md` - the extension inventory loses the `ze:display-key` row
- `docs/guide/configuration.md` - the update block examples show the named spelling and say the name is a label for the operator that reaches no BGP message

## Files to Create
- `test/parse/update-block-name.ci` - named and bare blocks load, format back unchanged, and a repeated name is refused
- `test/parse/update-block-name-retired-leaf.ci` - the retired spelling is refused
- `test/web/update-block-name.wb` - the web add form and finder carry the name
- `test/editor/completion/bgp-peer-update-name.et` - CLI completion offers the name

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/bgp/yang/ze-bgp-conf.yang` loses `leaf name`; `internal/component/config/yang/modules/ze-extensions.yang` loses `extension display-key` |
| YANG validation constraints | Yes | The name pattern the retired leaf carried moves onto the list's key validation, so a name the leaf refused is still refused |
| YANG custom validators | N-A | The pattern is a native constraint; no `ze:validate` is needed |
| CLI commands/flags | N-A | No command or flag is added or changed; `edit bgp peer p1 update <name>` already resolves through `editor_walk` |
| CLI grammar (keyword before value) | N-A | `update <name>` is a list key after its list name, which is the existing grammar for every keyed list |
| Editor autocomplete | Yes | `internal/component/cli/completer.go`; the key completion keeps its shape and reads the shared helper |
| Functional test for new RPC/API | N-A | No RPC or API is added; the four functional tests above cover the config, web and CLI paths |
| Pipe completeness | N-A | No command output is added or changed |
| Env var registration | N-A | No leaf under `environment/` is touched |
| Doctor check for runtime dependencies | N-A | No file path, socket, service, module, port or certificate is introduced |
| Prometheus counters/metrics | N-A | No observable runtime state is added; the name reaches no runtime decision |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family, capability or attribute is touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The named spelling already parses; this work settles it and deletes the alternative. `docs/features.md` describes update blocks as route announcements, which does not change |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` and `docs/architecture/config/syntax.md`: the named spelling, the uniqueness rule, and the removed `ze:display-key` row |
| 3 | CLI command added/changed? | No | No command, flag or exit code changes; `docs/guide/command-reference.md` has nothing to correct |
| 4 | API/RPC added/changed? | No | No RPC is touched. `docs/architecture/api/update-syntax.md` documents the `send bgp ... update` API command, which is a different surface and stays correct |
| 5 | Plugin added/changed? | No | No plugin registration, command or schema changes |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md`, the update block section |
| 7 | Wire format changed? | No | The name reaches no BGP message; AC-8 asserts it |
| 8 | Plugin SDK/protocol changed? | No | `InProcessConfigRouteParser` takes the attribute and nlri tokens of a block, never its key |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Scope is config; no requirement changes level or proof |
| 10 | Test infrastructure changed? | No | The four new tests use the existing `.ci`, `.wb` and `.et` runners with no new capability |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` compares features, and no feature is added or removed |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md` loses the extension row; `docs/architecture/web-interface.md` is checked against the finder label change and corrected if it describes the keyless summary |
| 13 | Route metadata keys added/changed? | No | No metadata key is written or read |
| 14 | Prometheus counters added/changed? | No | None added or changed |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing registers or deregisters |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED, run on 2026-09-13: `./le spec citation anchors spec plan/immediate/spec-update-block-name-is-the-list-key.md` reports no BLOCKING page and 15 advisory mentions. Named and edited: `docs/architecture/config/syntax.md`, `docs/architecture/config/yang-config-design.md`, `docs/guide/configuration.md`. Named as unaffected, with the reason: `docs/config-reference.md` documents the `update` EVENT type and carries no update-block example; `docs/guide/config-editor.md` and `docs/contributing/documentation-testing.md` describe completion behavior, which keeps its shape; `docs/features/configuration.md` and the six `ze-bgp-conf.yang` guides (`add-path`, `bfd`, `bgp-policy`, `flowspec-route-reflector`, `looking-glass-howto`, `status`, plus `environment-block`, `environment`, `features/bgp-protocol`) describe other containers of the same module; `docs/architecture/aaa-tacacs.md` and `docs/architecture/ssh/fixit-bcrypt-hash-credential.md` mention `schema.go` for unrelated leaves. `docs/architecture/web-components.md` is DECLARED by `fragment.go` and is checked here: it describes the component inventory and says nothing about how a keyless list entry is labelled, so the finder change leaves it correct; `docs/architecture/web-interface.md` and `docs/guide/web-interface.md` are read in the same pass and corrected where either describes the add form prompting for a display-key leaf. Re-run the command at implementation and re-answer any page the tree has gained since |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/quickstart.md`, `docs/guide/healthcheck.md` and `docs/guide/as112.md` each show bare `update { }` blocks. Those stay valid under AC-2 and are checked rather than rewritten; `docs/guide/as112.md` gains a name on the two blocks its prose distinguishes in words |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the entry points before changing behavior
   - Tests: `TestUpdateBlockNameIsTheEntryKey`, `TestBuildListColumnShowsUpdateBlockName`, `test/parse/update-block-name.ci`
   - Files: `internal/component/config/parser_list_test.go`, `internal/component/web/fragment_test.go`, `test/parse/update-block-name.ci`
   - Verify: the parser test passes today (the key is already stored) and the web test and the `.ci` fail, which is the split this work closes
2. **Phase: one declaration of the entry name** -- add the helper and route every reader through it
   - Tests: `TestSerializeOmitsGeneratedListKey`, `TestSerializeWritesOperatorName`, `TestConfigTextRoundTrip`
   - Files: `internal/component/config/serialize.go`, `serialize_annotated.go`, `serialize_blame.go`, `internal/component/cli/diff_tree.go`, `internal/component/cli/completer.go`
   - Verify: the three serializers and the CLI diff give the same answer for the same key, and no generated key reaches config text
3. **Phase: delete the retired spelling** -- the leaf, the extension, and the plumbing
   - Tests: `TestUpdateBlockRejectsRetiredNameLeaf`, `TestKeylessListAllowsSeveralAnonymousEntries`, `TestNoDisplayKeyExtensionRemains`
   - Files: `internal/component/bgp/yang/ze-bgp-conf.yang`, `internal/component/config/yang/modules/ze-extensions.yang`, `internal/component/config/yang_schema.go`, `internal/component/config/schema.go`, `internal/component/config/parser_list.go`
   - Verify: the duplicate-entry allowance is changed in the same edit as the extension deletion (R-1), and the whole `.ci` corpus still loads
4. **Phase: the web reads the one fact** -- finder label and add form
   - Tests: `TestConfigAddKeylessEntryTakesOptionalName`, `test/web/update-block-name.wb`
   - Files: `internal/component/web/fragment.go`, `handler_config_entry.go`, `view_fragment.go`, `component_add_form_overlay.templ` and its regenerated Go, `golden_fixtures_test.go`, the golden HTML
   - Verify: a named entry is labelled by its name, an unnamed one by its summary, and an entry created with an empty name commits as a bare block
5. **Phase: pages and published fixtures** -- the doc edits land here, not at closure
   - Tests: `./le verify current mode full`
   - Files: the four pages in the documentation checklist and the two `internal/le/site/testdata` fixtures
   - Verify: `./le spec citation anchors` reports no unanswered page, and the regenerated fixtures carry no trace of the retired leaf

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, and AC-9 is satisfied by a grep over the whole tree rather than over the edited files |
| Feature completeness | The name survives config text, the CLI editor, the CLI diff, the web finder, the web add form and a commit, with no surface left reading a deleted field |
| Correctness | The anonymous-key test is on the STRIPPED key in every caller, so `default#1` is anonymous and a block literally named `default` is not |
| Naming | The helper names what it answers (the entry's operator-visible name), not how it is computed, and no caller re-implements it |
| Data flow | Nothing reads the entry key to decide behavior; `extractRoutesFromUpdateBlock` is unchanged |
| Rule: `ai/rules/no-layering.md` | The `name` leaf and `ze:display-key` are deleted, not deprecated, and no fallback reads the leaf when the key is empty |
| Rule: `ai/rules/principles.md` | After the change, `KeyDefault` is compared against in one place, and the count is evidence in the review |
| Rule: `ai/rules/documentation.md` | The four pages are edited in phase 5 of this work, not in a follow-up |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No `ze:display-key` anywhere | `grep -rn "display-key" --include="*.yang" --include="*.go" --include="*.md" .` returns nothing outside this spec |
| No `DisplayKey` field | `grep -rn "DisplayKey" --include="*.go" internal/` returns nothing |
| One anonymous-key test | `grep -rn "KeyDefault" --include="*.go" internal/` outside test files shows the parser's two uses and the helper, and no comparison in a caller |
| No generated key in config text | `./bin/ze config fmt test/parse/simple-v4.ci`-shaped config prints no `default` |
| Four functional tests run | `./le test ci test/parse/update-block-name.ci` and the `.wb` and `.et` runs pass |
| Whole gate | `./le verify worktree` passes |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The entry key of `list update` is operator input from a config file and from an HTTP form. It must be validated against the same pattern the retired leaf carried, in `ValidateListKey`, so a name holding a brace, a quote or a newline cannot reach the serializer and produce config text that reparses differently |
| Output escaping | The serializers already call `quoteIfNeeded` on a key; the helper must not bypass it, and the web finder must escape the name it renders as a label |
| Resource exhaustion | An unbounded number of anonymous entries is already possible; the duplicate-entry change adds no new growth path |
| Error leakage | The duplicate-name refusal names the name and the line, both of which the operator wrote |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| A config in the corpus stops loading | R-1: the duplicate-entry condition, before anything else |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- The repository already held the right answer and did not apply it uniformly. `internal/component/cli/diff_tree.go` strips the generated suffix before testing for the anonymous key, and `internal/component/cli/completer.go` shows the key for a named entry and `#N` for an unnamed one. That pair IS the design; the work is to make the other four readers agree with it.
- YANG cannot declare an optional key, because a `key` statement makes its leaves mandatory. Ze's config language has the concept anyway, and it is spelled as a keyless list whose entries may carry a stored key. That is why `list update` stays keyless: adding `key "name"` would make every bare `update { }` in the tree illegal.
- A generated value escaping into a user-visible artifact is the failure shape behind two of the three defects here. `default#1` and the sequential `1` are both internal disambiguation that reached the operator's file.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The list stays keyless in YANG and the stored entry key is the name | Add `key "name"` to `list update` | A YANG key is mandatory. Adding one makes `update { }` illegal, and the tree writes it that way in dozens of `.ci`, `.conf` and interop files. The name is optional by the owner's own requirement, so the schema must not demand it |
| `update <name> { }` is the one spelling; `leaf name` and `ze:display-key` are deleted | Keep the leaf and reconcile it with the key at load | Two declarations of one fact is the defect (`ai/rules/principles.md`), and keeping both while preferring one is the hybrid `ai/rules/no-layering.md` bans. Nothing in the tree uses the leaf form, so deleting it costs no config |
| The name is unique, refused at load (owner ruling, 2026-09-13) | Allow repeats, since a mnemonic arguably need not be unique | → Decision: the owner ruled on 2026-09-13 that the name is the list key and a repeat is a config error the operator sees at load. This is what `addParsedListEntry` already does. "Operator memory only" constrains what the name MUST NOT do (no behavior keys off it, it reaches no BGP message, it needs no lookup path or registry), not how it is stored. Do not reopen |
| One helper answers "does this entry have an operator-visible name, and what is it" | Leave each reader its own test against `KeyDefault` | The test is spelled five times today with three different meanings, and two of the five are wrong. One declaration is the rule, and the count of `KeyDefault` comparisons is the review's evidence |
| `allowsDuplicateParsedListEntries` allows a repeated anonymous key on any keyless list | Keep a per-list opt-in, now keyed on something other than `DisplayKey` | A keyless list has no key, so two anonymous entries are two entries and "duplicate list key" is not a statement about them. An opt-in would be a new central enumeration of which lists may repeat, which is the shape `ai/rules/principles.md` names. `list update` is the only keyless list the config parser reaches, so the wider rule changes nothing else today |
| A web-created entry with no name takes the anonymous key | Keep the sequential numeric key | Under this spec a stored key IS the name, so a sequential `1` publishes a name the operator never typed, and writes it into their config file. The anonymous key is what the parser gives a bare block, so the two entry points agree |

## Known Limitations
- `serializeListBlocks` sorts list entry keys, so writing a config back reorders update blocks and naming a block moves it in the file, while the config tree keeps insertion order and the route extractor walks that order. Measured on 2026-09-13: a peer holding `drop-ssh-scan`, `scrubbing` and two bare blocks formats back with the bare blocks first. This is a defect of every list, not of the name, and it is not fixed here. It is reported to the main thread for a journal row in `plan/journal/`, and this spec asserts the block NAMES that come back, not their order.
- `ze:display-key` is deleted rather than repurposed. A future keyless list that wants a content-derived label still has `keylessEntrySummary`, which keeps its child-list-key and first-leaf fallbacks.

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
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
