# YANG Configuration System

Ze uses YANG (RFC 7950) as the schema language for configuration, CLI commands, and API operations.
YANG schemas drive config parsing, validation, CLI autocomplete, and command dispatch.

<!-- source: internal/component/config/yang/loader.go -- Loader, LoadEmbedded, LoadRegistered -->

> **See also:** [Hub Architecture](../hub-architecture.md) for how the config reader integrates
> with the multi-process plugin architecture.

---

## 1. Key Principle

**YANG defines format. Extensions declare behavior. Implementation executes behavior.**

Standard YANG tools see valid schemas. Ze additionally executes custom extensions
(`ze:validate`, `ze:command`, `ze:syntax`, etc.) that bridge static schema to runtime Go code.

Ze uses [goyang](https://github.com/openconfig/goyang) (pure Go) for schema parsing and validation.

---

## 2. YANG Module Architecture

Ze's YANG modules fall into four categories. Understanding the distinction is essential
for knowing where to look and what each module controls.

| Category | Purpose | Contains | Example |
|----------|---------|----------|---------|
| **Type library** | Reusable type definitions | `typedef`, `grouping` | `ze-types.yang` |
| **Extensions** | Custom ze-specific behavior declarations | `extension` | `ze-extensions.yang` |
| **Config schemas** | Configuration tree structure (drives CLI autocomplete) | `container`, `list`, `leaf`, `augment` | `ze-bgp-conf.yang` |
| **API schemas** | RPC/command definitions for CLI and IPC | `rpc`, `notification`, `ze:command` | `ze-bgp-api.yang`, `ze-*-cmd.yang` |

<!-- source: internal/component/config/yang/modules/ze-types.yang -- typedef, grouping definitions -->
<!-- source: internal/component/config/yang/modules/ze-extensions.yang -- extension declarations -->
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- config tree structure -->
<!-- source: internal/component/bgp/yang/ze-bgp-api.yang -- RPC definitions -->

### Type Library: `ze-types.yang`

Defines reusable types and groupings imported by other modules. Contains no tree nodes
(no containers, lists, or leaves). Think of it as a header file with type definitions
but no variables.

| Kind | Examples |
|------|----------|
| Typedefs | `ipv4-address`, `asn`, `port`, `prefix-ipv4`, `community`, `address-family` |
| Groupings | `route-attributes`, `peer-info`, `command-info`, `transaction-result` |

Other modules import it: `import ze-types { prefix zt; }` and reference its types:
`leaf as { type zt:asn; }`.

### Extensions: `ze-extensions.yang`

Defines ze-specific YANG extensions (RFC 7950 Section 7.19). These are annotations that
standard YANG tools ignore but ze interprets at runtime.

| Extension | Purpose | Argument |
|-----------|---------|----------|
| `ze:allow-unknown-fields` | Container accepts arbitrary key-value pairs | (none) |
| `ze:backend` | Restricts a node to named backends. Commit validates it and completion filters on it | space-separated backend names |
| `ze:bcrypt` | Leaf holds a one-way bcrypt hash. The commit hook hashes its `plaintext-<name>` sibling into it | (none) |
| `ze:command` | Marks a `config false` container as an executable CLI command | WireMethod string |
| `ze:cumulative` | Leaf-list accumulates values from the bgp, group and peer levels instead of the most specific level replacing them | (none) |
| `ze:decorate` | Attaches a registered display-time decorator to a leaf | decorator name |
| `ze:display-key` | Names the leaf the web interface shows for a keyless list entry | (none) |
| `ze:edit-shortcut` | Makes a command available in edit mode without the `run` prefix | (none) |
| `ze:ensure-exists` | Marks a command container as a resource checkpoint. Each descendant command ensures the resource exists | WireMethod of the rollback handler |
| `ze:ephemeral` | Node is validated and completed but never written to the config file | (none) |
| `ze:filter` | Marks a list as a named filter type of the route policy framework | (none) |
| `ze:flatten` | Serializes a container's children with the container name as a leading keyword | (none) |
| `ze:help` | Declares the LONG explanation of a command node, an rpc, or a config node. The `description` statement declares the one-line summary | free text, several lines allowed |
| `ze:hidden` | Hides a leaf from config display and from the web editor | `true` or `false` |
| `ze:inherit` | Says whether a command takes the leaves its ancestor containers declare | `none` |
| `ze:key-type` | Key type for inline-list nodes | type name |
| `ze:listener` | Marks a list entry as a network listener endpoint, for port-conflict detection at parse time | (none) |
| `ze:modifier` | Marks a `config false` container as a trailing argument group of its parent command | `once`, `repeat`, `required`, `choice` |
| `ze:ordered` | Leaf-list is an ordered sequence whose duplicates are meaningful (AS_PATH prepends, MPLS label stacks) | (none) |
| `ze:os` | Restricts a node to one operating system. The schema drops the node elsewhere | GOOS value |
| `ze:related` | workbench: declares an operator tool descriptor on a config node | descriptor string |
| `ze:required` | Field must hold a value after config inheritance resolves | path |
| `ze:route-attributes` | Node accepts standard BGP route attributes | (none) |
| `ze:sensitive` | Leaf holds sensitive data. Display obfuscates it with JunOS-compatible `$9$` encoding | (none) |
| `ze:suggest` | Field appears in a creation dialog with its inherited default. The entry is created without it | path |
| `ze:syntax` | Config parser syntax mode (flex, freeform, inline-list) | mode name |
| `ze:task-support` | MCP task-support level for a command | `required`, `optional`, `forbidden` |
| `ze:ui-csp` | CSP directive advertised in `_meta.ui.csp` | policy string |
| `ze:ui-permissions` | MCP App permission capabilities advertised in `_meta.ui.permissions` | space-separated capabilities |
| `ze:ui-resource` | Associates an embedded MCP App UI bundle with a command group | path under `internal/component/mcp/ui/` |
| `ze:validate` | References a Go validator function for runtime validation and completion | function name |

The table is complete: it holds every extension the module declares, in name
order. An extension that is absent here is a defect of this page.

<!-- source: internal/component/config/yang/modules/ze-extensions.yang -- all extension definitions -->

### Config Schemas: `ze-bgp-conf.yang` and siblings

Define the actual configuration tree that users interact with. These are the modules
the CLI completer walks to know what nodes are valid at each position.

Config schemas import `ze-types` for leaf types and `ze-extensions` for behavior annotations.

| Schema | Owns | Location |
|--------|------|----------|
| `ze-bgp-conf` | BGP configuration (peers, families, capabilities) | `component/bgp/yang/` |
| `ze-hub-conf` | Hub/environment settings | `component/hub/yang/` |
| `ze-system-conf` | System-level configuration | `component/config/system/yang/` |
| `ze-plugin-conf` | Plugin configuration | `component/plugin/yang/` |
| `ze-ssh-conf` | SSH listener configuration and user public-key augmentation | `component/ssh/yang/` |
| `ze-authz-conf` | Local user authentication and profile authorization configuration | `component/authz/yang/` |
| `ze-telemetry-conf` | Telemetry configuration | `component/telemetry/yang/` |
| Plugin schemas | Per-plugin config (GR, RPKI, role, hostname, etc.) | `component/bgp/plugins/<name>/yang/` |

<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- BGP config tree -->
<!-- source: internal/component/hub/yang/ze-hub-conf.yang -- Hub/environment config -->

### API Schemas: `ze-bgp-api.yang` and `ze-*-cmd.yang`

Define RPCs (request/response operations) and the CLI command tree. The `-api.yang` modules
define RPC signatures. The `-cmd.yang` modules define the CLI navigation hierarchy using
`config false` containers with `ze:command` extensions.

| Schema | Purpose | Location |
|--------|---------|----------|
| `ze-bgp-api` | BGP peer/route/cache RPCs | `component/bgp/yang/` |
| `ze-bgp-cmd-peer-api` | Peer management commands | `component/bgp/plugins/cmd/peer/yang/` |
| `ze-rib-api` | RIB query RPCs | `component/bgp/plugins/rib/yang/` |
| `ze-*-cmd` | CLI command tree nodes | Various `schema/` directories |

---

> **See also:** [Config Transaction Protocol](transaction-protocol.md) for the bus-based
> verify/apply/rollback lifecycle that config changes go through after validation.

---

## 3. Module Loading

YANG modules are loaded in two phases at startup.

<!-- source: internal/component/config/yang/loader.go -- LoadEmbedded, LoadRegistered -->

### Phase 1: Embedded (bootstrap)

`LoadEmbedded()` loads the two foundation modules compiled into the binary:

| Module | Content |
|--------|---------|
| `ze-extensions.yang` | Extension definitions (every other module imports this) |
| `ze-types.yang` | Shared typedefs and groupings |

### Phase 2: Registered (plugin-contributed)

`LoadRegistered()` loads all modules registered via `init()` functions using
`yang.RegisterModule(name, content)`. Each component embeds its own `.yang` files
and registers them at import time.

After both phases, `Resolve()` resolves all cross-module imports via goyang.

### Registration Pattern

Each component with a YANG schema follows this pattern:

```
component/<name>/yang/
    ze-<name>.yang          # Schema file (embedded via //go:embed)
    register.go             # init() calls yang.RegisterModule()
```

<!-- source: internal/component/config/yang/register.go -- RegisterModule, Module struct -->

---

## 4. Validation

Ze validates configuration trees in four layers. The first two are declared in
YANG and reach one value at a time. The other two run over whole subtrees, which
is what a rule needs when its answer depends on two sibling nodes.

### Layer 1: YANG Native Validation (goyang)

`ValidateTree` recursively walks the config tree against YANG schema entries,
checking constraints at every level.

| Constraint | YANG Syntax | Example |
|------------|-------------|---------|
| Enumeration | `type enumeration { enum igp; }` | `origin` must be igp/egp/incomplete |
| Range | `type uint16 { range "0 \| 3..65535"; }` | Hold time validation |
| Pattern | `type string { pattern '...'; }` | IPv4 address format |
| Length | `type string { length "1..255"; }` | String bounds |
| Mandatory | `mandatory true;` | Required fields |

<!-- source: internal/component/config/yang/validator.go -- ValidateTree, walkTree -->

### Layer 2: Custom Validators (`ze:validate`)

When YANG native constraints are insufficient (runtime-determined valid sets, cross-field
checks), the `ze:validate` extension references a registered Go function.

In YANG:

```yang
leaf name {
    type zt:address-family;
    ze:validate "registered-address-family";
}
```

In Go, each validator registers a `CustomValidator` with three functions. They
are independent, and a validator carries any subset of them:

| Function | Purpose |
|----------|---------|
| `ValidateFn(path, value) error` | Validates a value at parse/commit time |
| `CompleteFn() []string` | Returns valid values for CLI completion (optional) |
| `DescribeFn(value) string` | Says what one offered value means, for the dropdown (optional) |

<!-- source: internal/component/config/yang/validator_registry.go -- CustomValidator, ValidatorRegistry -->

### Completion-Only Validators

**A validator with a nil `ValidateFn` refuses nothing.** It SUGGESTS: every
value the leaf's YANG type admits stays valid, and `applyCustomValidators` skips
it during the walk. Completion and refusal are separate jobs, so offering the
well-known values for a leaf never narrows it.

This is how a plugin completes its own leaf. `RegisterValidators` is a central
list in the config package; config cannot import a plugin to write a row in it,
and the plugin cannot reach the list. So the plugin calls `yang.RegisterSuggestion`
from its own `init()`, passing the values and, optionally, a function that says
what one value means.

Two global registrations exist and they assert different things:

| Call | Asserts | A name config does not declare |
|------|---------|--------------------------------|
| `RegisterCompleteFn(name, values)` | fill the `CompleteFn` slot of a validator config already declared | is left absent, so the startup check still names the leaf |
| `RegisterSuggestion(name, values, describe)` | DECLARE a completion-only validator | is created, with no `ValidateFn` |

The split keeps a forgotten `ValidateFn` loud. If an orphan `CompleteFn` created
a validator, a leaf whose `ze:validate` names a validator nobody wrote would
pass the startup check and lose its validation with no surface saying so.

Neither call overwrites. A name the registry already holds keeps its
`ValidateFn` and gains only the slots it left empty.

`bgp-filter-path-asn` is the worked example: its `asn` leaf-list carries
`ze:validate "transit-asn"`, and the plugin offers the well-known transit-free
ASNs with their network names while accepting every other uint32
(`docs/architecture/bgp/filter-path-asn.md`).

<!-- source: internal/component/config/yang/validator_registry.go -- RegisterCompleteFn, RegisterSuggestion, MergeGlobalCompletions -->
<!-- source: internal/component/config/yang/validator.go -- applyCustomValidators -->

### Registered Validators

| Name | Validates | Provides Completion |
|------|-----------|-------------------|
| `registered-address-family` | Value is a plugin-registered AFI/SAFI | Yes -- queries `registry.FamilyMap()` |
| `receive-event-type` | Value is a valid BGP event type | Yes -- queries registered event types |
| `send-message-type` | Value is a valid send type (update, refresh, etc.) | Yes -- base types + plugin-registered |
| `nonzero-ipv4` | Valid IPv4, not 0.0.0.0 | No |
| `literal-self` | Literal string "self" | No |
| `community-range` | Community in ASN:value format, both parts uint16 | No |

<!-- source: internal/component/config/validators.go -- validator implementations -->
<!-- source: internal/component/config/validators_register.go -- RegisterValidators -->

### Pipe-Separated Validators

A single `ze:validate` argument can contain multiple validator names separated by `|`.
The value passes if ANY validator accepts it. Completions are the union of all
validators' `CompleteFn` results.

```yang
leaf next-hop { type string; ze:validate "nonzero-ipv4|literal-self"; }
```

This accepts either a valid non-zero IPv4 address or the literal "self".

<!-- source: internal/component/config/yang/validator_registry.go -- SplitValidatorNames -->

### Startup Integrity Check

`CheckAllValidatorsRegistered` walks the entire YANG tree at startup and verifies that every
`ze:validate` reference has a registered implementation. Missing validators abort startup.

<!-- source: internal/component/config/yang/validator_registry.go -- CheckAllValidatorsRegistered -->

### Where a validator actually runs

Declaring a `ze:validate` is not sufficient to make it run. `ValidateCustomSections`
iterates `validatedSections`, a list of top-level section names, and checks only
inside those, so an annotation under any other section never executes.

The check above cannot see that: it asks whether the validator FUNCTION exists,
never whether the walk reaches it, so a dead annotation and a live one are
spelled identically and both pass. Three sections had dead annotations for as
long as they had existed, and the first one found was a DHCP `default-router`
that `ze config validate` accepted as `2001:db8::1`.

`ValidatorSectionCoverage` derives the declaring sections from the resolved
model and subtracts `validatedSections` and `knownUnwalkedValidatorSections`,
which records each deliberate exclusion against its reason.
`TestEveryValidatorSectionIsWalkedOrExcused` fails on anything left over, so a
new plugin that declares a validator under a new section is told rather than
shipping a rule that does nothing.

Read the resolved model, never the YANG source: a `ze:validate` written inside a
grouping lands wherever that grouping is used, so the source cannot say which
section owns it. The derivation is also only as complete as the modules the
binary linked, and every plugin sits behind a feature build tag, which is why
the test asserts its recorded exclusions are present before it reads an empty
answer as good news.

<!-- source: internal/component/config/validate_sections.go -- ValidatorSectionCoverage, knownUnwalkedValidatorSections -->

### Layer 3: Plugin config verifiers

A plugin declares `InProcessConfigVerifier` on its registration and receives its
own config subtree, before and after, at `VerifyPluginConfig` and
`VerifyPluginConfigContentTransition`. It sees every node under its root, so a
rule comparing two leaves of that subtree lives here. It does NOT see the peer
population and it does not run at daemon startup, which is the door a
hand-edited file uses.

<!-- source: internal/component/config/plugin_verify.go -- VerifyPluginConfig, VerifyPluginConfigContentTransition -->

### Layer 4: The BGP peer pipeline

`bgp` is deliberately absent from `validatedSections`, because the BGP tree has
a deeper walk of its own: `PeersFromConfigTree` resolves the group and peer
layers, builds each peer's settings and filter chains, and validates the result.
A rule that must see a peer's role AND its filter chains at the same time lives
there, and nowhere else can: the role is one plugin's leaf and the chains are
another's, and only the peer pipeline holds both.

Five doors reach it, so one rule answers on every path an operator takes:
daemon startup and reload through `CreateReactorFromTree`, `ze config validate`
and `ze doctor` through `infra.ValidateBGPPeers`, the SSH config editor's
`commit` through `bgpPeerErrors`, and the web editor's commit through the
pre-commit validator the daemon injects into every editor it hands out
(`newEditorFactory` passes `config/cli.ValidateContent`, which calls the same
seam).

The editor door runs the check over the tree the commit is about to WRITE, not
over the draft. `Editor.SaveDraft` validates the shared draft base plus this
session's entries; a commit writes the committed file plus this session's
entries. The two agree only while no other session holds a saved draft, so
`validateStagedTree` runs at each of the two commit writes and makes the checked
config and the staged config the same config.

<!-- source: internal/component/bgp/config/peers.go -- peersAndDynamicGroups, validatePeerProcessCaps, validateLeakFilterObligations -->
<!-- source: internal/component/config/infra/bgp.go -- ValidateBGPPeers -->
<!-- source: internal/component/cli/validator.go -- bgpPeerErrors -->
<!-- source: internal/component/cli/editor_commit.go -- validateStagedTree -->
<!-- source: cmd/ze/hub/editor_adapter.go -- newEditorFactory -->

### Claim Completeness Gate

Validation says a config value is well formed. It does not say the value reaches
anything. Delivery is claimed per path: `Server.reloadConfig` selects the plugins
whose `WantsConfigRoots` match the changed paths, and `Hub.RouteCommand` resolves
a path to a subsystem through `SchemaRegistry.FindHandler`. A path matched by
neither is stored and delivered nowhere, with one Info log line.

`TestConfigSchemaRootsClaimed` closes that hole at build time. It resolves the
full config schema through the YANG loader, unions the claims from the plugin
registry and the schema registry, and fails when a config subtree is covered by
neither. `TestConfigRootsPhantomClaims` runs the inverse: a declared config root
that names no schema node never matches, so the plugin that declared it is never
selected. Both inventories are read live, so neither can drift from a list.

A subtree that a component reads straight from the config tree, rather than
through the plugin RPC, is recorded in `allowlist.json` with a reason and the
consuming symbol. Five paths are recorded today: `plugin`, `pppoe`, `storage`,
`system`, and `telemetry`. An entry without a reason and an owner is a failure,
and so is an entry whose path is now claimed.

At run time `ze doctor` judges one config on one build and reports
`doctor-config-root-unclaimed` for a configured subtree this binary delivers to
nobody. That covers what the build-time gate cannot see: a plugin compiled out,
or one that failed to load.

`./le yang leaf-mentions report` is the advisory companion. It reports YANG leaves
whose kebab name appears in no string literal of the owning package, which is a
candidate for "delivered but never read". The signal is a heuristic, so it exits
0 and sits in no verify stage.

<!-- source: internal/component/config/claims/claims.go -- Audit -->
<!-- source: internal/component/plugin/all/config_claims_test.go -- TestConfigSchemaRootsClaimed -->
<!-- source: internal/component/doctor/checks_config_claims.go -- checkConfigClaims -->

---

## 5. CLI Completion

The CLI completer walks the **config schema tree** to determine what is valid at each cursor position.
The type library (`ze-types.yang`) is not walked directly; its types are resolved into the config
tree by goyang during module resolution.

<!-- source: internal/component/cli/completer.go -- Completer, Complete -->

### Node Completion

When the user presses Tab at a position in the config tree, the completer navigates to the current
context path in the YANG tree and offers the valid child nodes (containers, lists, leaves) as
completions. This is how config keyword names appear in autocomplete.

### Value Completion

When the cursor is at a leaf value position, three sources are checked in priority order:

| Priority | Source | When Used | Example |
|----------|--------|-----------|---------|
| 1 | `ze:validate` `CompleteFn` | Leaf has `ze:validate` with a registered `CompleteFn` | Address families from plugin registry |
| 2 | YANG enum values | Leaf type is `enumeration` | `origin`: igp, egp, incomplete |
| 3 | Type hint | Neither of the above | `<ipv4-address>`, `<0-65535>` |

<!-- source: internal/component/cli/completer.go -- valueCompletions, validateCompletions, TypeHint -->

**Why `ze:validate` takes priority over enum:** If a developer sets `ze:validate` on an enum leaf,
they want dynamic completion from runtime state, not the static enum values. The `CompleteFn`
queries whatever is currently registered (families, event types, send types), reflecting the
plugins that are actually loaded.

Each offered value is labeled `valid value` unless the validator carries a
`DescribeFn`, which returns the meaning of one value. A value that `DescribeFn`
does not know keeps the generic label.

### List Key Completion

A list key is not a leaf value, so `listKeyCompletions` answers it rather than
`valueCompletions`. It offers the wildcard `*`, then the keys the config already
holds.

**A list keyed by an `enumeration` also offers the keys nobody has created yet**,
each carrying the help text its `enum` declares, because for that list the
schema knows every key there can be. For every other list the set of keys is
what the operator created, so there is nothing more the schema can offer and the
placeholder hint `<value>` is shown instead.

The help text is read off the leaf's parse-tree node: the resolved `EnumType`
keeps only the name and the value. An enumeration that arrives through a typedef
or a grouping therefore completes with no help rather than with the wrong help.

<!-- source: internal/component/cli/completer.go -- listKeyCompletions, enumKeyVocabulary -->

### Ghost Text

The completer also provides ghost text (inline suggestions) for partial input, showing what
would complete the current word.

---

## 6. CLI Mapping

YANG constructs map to CLI syntax as follows:

| YANG Construct | CLI Syntax |
|----------------|------------|
| `container foo` | `foo` (enters context) |
| `list foo { key "name" }` | `foo <name>` |
| `leaf bar` | `bar <value>` |
| `leaf-list baz` | `baz <value>` (repeatable) |
| `presence container` | `foo` (no value, enables) |
| `leaf { type empty }` | `foo` (flag, no value) |

### Leaf-List Editing Semantics

Plain leaf-lists (`leaf-list` without `ze:syntax`, compiled to
`ValueOrArrayNode`) use JunOS-style member operations in every editing mode:

| Command | Effect |
|---------|--------|
| `set <path> <member>` | Adds one member (idempotent; never replaces the list) |
| `delete <path> <member>` | Removes one member |
| `delete <path>` | Removes the whole leaf-list |
| `insert <path> <member> first\|last\|before <ref>\|after <ref>` | Adds at an exact position |
| `deactivate <path> <member>` / `activate <path> <member>` | Toggles one member in place |

A plain leaf-list is a **set**: repeated values are deduplicated on parse
(`Tree.AppendSlice`). A leaf-list that models an ordered **sequence** whose
duplicate values are meaningful (AS_PATH prepends, MPLS label stacks) must carry
the `ze:ordered` extension so the parser preserves duplicates
(`Tree.AppendSequence`). Without it, `as-path [ 65001 65001 65001 ]` collapses to
a single `65001` and silently drops the prepends. Because deactivation is
value-keyed, a repeated member of an ordered leaf-list cannot be deactivated
individually — `DeactivateMultiValue` rejects it rather than blank every copy.
<!-- source: internal/component/config/tree.go -- AppendSlice / AppendSequence -->

**Invariant: leaf-list nodes MUST use the multi-value Tree API
(`AddMultiValueMember`, `RemoveMultiValueMember`, `SetSlice`,
`InsertMultiValue`) in every write and apply path.** Every serializer reads
the multi-value store; a value stored through the scalar `Set` is silently
dropped on the next serialize. The scalar map only carries a joined copy,
synchronized by the multi-value API for `Get()` callers.
<!-- source: internal/component/config/tree.go -- AddMultiValueMember, RemoveMultiValueMember -->
<!-- source: internal/component/config/setparser.go -- walkAndSet ValueOrArrayNode member merge -->

Session change tracking is per-member: each add or remove records one
metadata entry with `MetaEntry.Member` set, so concurrent sessions adding
different members never conflict, and commit applies each member operation
idempotently. Ordered operations (insert position, deactivate, activate) are
recorded as structural ops (`insert-member`, `deactivate-member`,
`activate-member`) so the exact position survives the change-file → draft →
commit chain.

Per-member deactivation is stored **out-of-band** on the `Tree`
(`inactiveMembers`, sibling to the member slice): the member value itself is
never rewritten, so it stays clean for every reader. Effective-config accessors
(`GetSlice`/`GetMultiValues`/`ToMap`) return active members only; the structural
view (`GetMultiValuesState`) reports every member with its deactivation flag.

Two on-disk input forms deactivate a member, and both are accepted:
- the canonical **statement** form the serializer emits — the member stays bare
  in the leaf/`set` line plus a follow-up `inactive: <leaf> <member>`
  (hierarchical) or `nop <path> <member>` (set-format) line;
- the compact **inline** form `<leaf> [ inactive:MEMBER ... ]`, normalized at the
  parse boundary into the out-of-band marker.

Serialization always emits the statement form (active members as `set`,
deactivated as `nop`) — a raw `inactive:` item is never written to a value.
Trade-off of the inline form: a member value that legitimately begins with
`inactive:` can only be expressed via the statement form.
<!-- source: internal/component/config/tree.go -- Tree.inactiveMembers, GetMultiValuesState -->
<!-- source: internal/component/config/parser_list.go -- stripInactiveMemberPrefix (inline-form normalization) -->
<!-- source: internal/component/config/meta.go -- MetaEntry.Member -->
<!-- source: internal/component/config/change_file.go -- StructuralOpInsertMember -->
<!-- source: internal/component/config/serialize_set.go -- writeLeafListMemberLines, emitValueOrArrayNop -->

### CLI Help from YANG

A config leaf's description and its type constraints generate its help text:

```
ze(edit)# hold-time ?
  <0, 3-65535>    Hold time in seconds (RFC 4271: 0 or >= 3)
```

#### A command node declares two help texts

A command node in a `-cmd.yang` module declares its help in two statements, and
each one answers a different question.

| Statement | Holds | Read by |
|-----------|-------|---------|
| `description` | the one-line SUMMARY of the command | every surface that shows a command on one line: a list row, a table cell, and the message line under the interactive completion menu |
| `ze:help` | the LONG explanation of that one command | the help page for that command, and the box that Tab opens in the interactive CLI |
<!-- source: internal/component/cli/model_render.go -- warningText, renderExplanationBox -->

`mergeYANGEntry` (`internal/component/config/yang/command.go`) writes them to
`command.Node.Description` and `command.Node.Help`. Neither field is derived
from the other, and no reader shortens either one. A summary is authored short
because it is a summary.

#### An rpc declares the same two texts

An `rpc` statement carries the same pair, in the same two statements.
`ExtractRPCs` (`internal/component/config/yang/rpc.go`) writes them to
`RPCMeta.Description` and `RPCMeta.Help`.

One reader serves both carriers. `getHelpExtension` takes the extension
statement list, which a command container reaches through `Entry.Exts` and an
rpc through `gyang.RPC.Exts()`. A second reader would let the two surfaces drift
into two spellings of one declaration.

`./le docvalid help-shape` holds both corpora to one shape: 601 command tree
nodes and 211 RPCs, each summary one sentence of 25 words at most, on one line,
with no semicolon and a full stop at the end.

An empty `ze:help` means nobody has written an explanation for that command.
That is not a defect. The help page then prints the summary alone, and the
interactive CLI says that the command declares none.
<!-- source: internal/component/cli/model_keys.go -- revealExplanation -->

An empty `description` is a defect. Every list that names the command shows a
blank cell, and `validateNode` warns for each one by path.

Two modules can contribute the same command path. `mergeHelpText` decides each
of the two fields on its own, in three cases:

- The module that marks the node executable states both halves of that
  command's help.
- An empty field takes what arrives.
- Two different non-empty values leave the first value in place. The merge logs
  `YANG command help text mismatch` and names the field that collided.

#### A config node declares the same two texts

A container, a list or a leaf in a config module declares the pair in the same
two statements. Each text reaches its own surface of the interactive CLI.

| Statement | Holds | Read by |
|-----------|-------|---------|
| `description` | the one-line SUMMARY of the node | the message row under the completion menu, the web editor form, and every list that names the node |
| `ze:help` | the LONG explanation of that node | the box `?` opens on the highlighted candidate |
<!-- source: internal/component/cli/completer.go -- entryDescription, entryLongHelp -->
<!-- source: internal/component/cli/model_keys.go -- revealCandidateExplanation -->

`matchChildren` and `matchEditTargets` put both texts on the `contract.Completion`
they build, in `Description` and `LongHelp`. The reader of the extension is
`GetHelpExtension`, the one a command container and an rpc already use.

A node that declares no `ze:help` declares no explanation. `?` then says
`<path>: no explanation is declared` on the message row. The description does
not stand in for it: a box repeating the row is the defect this split removes.

##### Several modules can declare one node, and the `?` box carries them all

A plugin attaches its leaves to a shared node by declaring a container of the
same name in its own module. It does not import the module that owns the node.
Removing the plugin then removes its leaves and nothing else, which is what
plugin self-containment requires. `class-of-service` reaches `interface` that
way.

`mergeAugmentedEntries` unions those declarations into one virtual entry. The
`ze:help` of every declaration is JOINED into that entry, separated by a blank
line and in module-name order. A module that declares no help can no longer
erase one that does.

The join carries no module NAME, so the operator reads N paragraphs and cannot
tell which module wrote each one. The `?` box also draws what fits and no more:
`renderExplanationBox` wraps to the rows the box has, prints `... N more`, and
no key scrolls it. A node several modules explain can therefore hold more text
than the box will ever show.

The one-line `description` can show only one text, and it is the first in
module-name order. Nothing in the schema says which module OWNS a shared node,
so that row can name the wrong module until one does.

<!-- source: internal/component/cli/completer.go -- mergeAugmentedEntries, mergeHelpExts -->

---

## 7. File Organization

### Bootstrap Modules (embedded in binary)

```
internal/component/config/yang/modules/
    ze-extensions.yang      # Extension definitions
    ze-types.yang           # Shared typedefs and groupings
```

### Domain Schemas (registered via init())

```
internal/component/bgp/yang/
    ze-bgp-conf.yang        # BGP configuration tree
    ze-bgp-api.yang         # BGP RPCs

internal/component/hub/yang/
    ze-hub-conf.yang        # Hub/environment config

internal/component/config/system/yang/
    ze-system-conf.yang     # System config

internal/component/bgp/plugins/<name>/yang/
    ze-<name>.yang          # Plugin-specific config
    ze-<name>-api.yang      # Plugin RPCs (if any)
    ze-<name>-cmd.yang      # CLI command tree (if any)

internal/component/cmd/<name>/yang/
    ze-cli-<name>-api.yang  # CLI command RPCs
    ze-cli-<name>-cmd.yang  # CLI command tree
```

### Validation and Completion

```
internal/component/config/yang/
    loader.go               # Module loading (embedded + registered)
    register.go             # Module registration (init() pattern)
    validator.go            # ValidateTree (recursive schema validation)
    validator_registry.go   # CustomValidator, ValidatorRegistry

internal/component/config/
    validators.go           # Validator implementations
    validators_register.go  # RegisterValidators()

internal/component/cli/
    completer.go            # YANG-driven CLI completion
    completer_command.go    # Command mode completion
    completer_plugin.go     # Plugin SDK method completion
```


## 8. Module Identity

| Element | Canonical | Anti-pattern |
|---------|-----------|--------------|
| Module name | `ze-<component>[-<kind>]`, matching the filename | `exabgp` (unprefixed; external-compat only) |
| Namespace | `urn:ze:<component>:<kind>`, where `<kind>` (`conf`, `cmd`, `api`) is always a final colon segment | `urn:ze:ddos-detect-conf` (kind fused with a hyphen), `urn:ze:role` (no kind segment) |
| Prefix | short, lowercase, unquoted, no hyphens, derived from the module | `prefix "bgp-mon-api";` (quoted, hyphens, abbreviated), `prefix updateshowcmd;` |
| `revision` | at least one `revision YYYY-MM-DD { description ...; }` | no revision statement |
| `description` | module-level `description` required | omitted |
| `organization` / `contact` | omitted; not a project convention, and present in only one legacy batch | adding `organization` to a new module |

`<component>` may contain hyphens for a multi-word name (`ddos-detect`,
`firewall-irr`). `internal/plugins/ddos/local/` is the current inconsistency:
its config module declares `urn:ze:ddos-local-conf` while its command module
declares `urn:ze:ddos-local:cmd`.

`zt` (ze-types) and `ze` (ze-extensions) are reserved prefixes.

goyang keys a module that declares a `revision` under TWO names, its bare name
and `<name>@<revision>`. `Loader.ModuleNames` answers the bare name alone, so a
caller that counts what it walks counts each module once. Reach a module by its
bare name; the revision key is goyang's, not an identity Ze uses.

### Command-module naming is not converged

The `-cmd` (grammar tree) and `-api` (handler) modules for operational verbs
carry several names for the same verb: `ze-cli-monitor-cmd`
(`internal/component/cmd/monitor/yang/`), `ze-monitor-cmd`
(`internal/component/bgp/plugins/cmd/monitor/yang/`) and `ze-command-monitor-cmd`
(`internal/plugins/meta/yang/`) all exist, and `ze-bgp-cmd-log-api` names a
non-BGP command. Converging them is a rename that touches `//go:embed`,
`register.go` and the YANG dispatch keys, so it is tracked separately and is not
done piecemeal. A new command module for a verb takes the majority
`ze-cli-<verb>-cmd` form with a paired `-api`.

## 9. Value Typing

Use the shared typedef. Do not re-express the same constraint a second way.

| Concept | Use | Do not use |
|---------|-----|------------|
| IPv4, IPv6, or either address | `zt:ipv4-address`, `zt:ipv6-address`, `zt:ip-address` | raw `type string`; `type string; ze:validate "ipv4-address"` |
| IPv4, IPv6, or either prefix | `zt:prefix-ipv4`, `zt:prefix-ipv6`, `zt:ip-prefix` | `type string; ze:validate "ipv4-prefix\|ipv6-prefix"` |
| ASN, port | `zt:asn`, `zt:asn2`, `zt:port`, `zt:listener-port` | an inline `uint32` or `uint16` with a copied range |
| Community, route distinguisher, address family | `zt:community`, `zt:route-distinguisher`, `zt:address-family` | a per-module pattern for the same shape |
| MAC address | `zt:mac-address`, which `ze-types.yang` does not declare yet and which is added there | a per-plugin `ze:validate "mac-address"` |
| Duration or other dimensioned value | an unsigned integer leaf with a `units` statement | `type string` for a duration; the unit implied only in the description |

`ze:validate` is for runtime-determined valid sets: registered address families,
plugin names, IRR set references, or a union with a literal keyword
(`nonzero-ipv4|literal-self`). It does not duplicate a constraint that a native
`pattern`, `range` or `enumeration`, or an existing `zt` typedef, already
expresses. That is the contract stated on the `validate` extension in
`ze-extensions.yang`.

<!-- source: internal/component/config/yang/modules/ze-types.yang -- the typedef and grouping library -->
<!-- source: internal/component/config/yang/modules/ze-extensions.yang -- extension validate -->

## 10. Units

A leaf whose value carries a physical unit states that unit once, through the
YANG `units` statement, and keeps the leaf name unit-free. This supersedes any
unit-suffix-in-the-name guidance.

| Rule | Canonical | Anti-pattern |
|------|-----------|--------------|
| One mechanism | `type uint32; units milliseconds;` | the unit in the leaf name (`min-tx-us`, `spf-delay-ms`, `teardown-grace-seconds`) |
| Full word, unquoted | `units microseconds;`, `units seconds;`, `units bytes/second;` | `units "seconds";` (quoted), `-us`, `-ms`, `-secs` abbreviations |
| Integer, not string | `type uint32; units seconds;` | `type string` for a duration |
| Protocol-sane default | every dimensioned leaf carries a `default` set to the protocol's standard value (OSPF `hello-interval` 10s, `dead-interval` 40s, BFD tx and rx per RFC 5880) | no `default`, so omitting the leaf yields 0 or undefined timing |

```
leaf hello-interval { type uint32; units seconds; default 10; }
```

## 11. Network Endpoints

An endpoint, a place to bind or a remote to connect to, is two structured fields
from a shared grouping. A combined `"host:port"` string is never used.

| Endpoint kind | Grouping | Fields | Port type |
|---------------|----------|--------|-----------|
| Inbound bind (the service listens) | `uses zt:listener` plus the `ze:listener` extension | `ip` (local literal), `port` | `zt:listener-port` (0 means OS-assigned) |
| Outbound target (the service connects out) | `uses zt:endpoint`, added to `ze-types.yang`, which does not declare it yet | `address` (IP or hostname), `port` | `zt:port` (1..65535) |

`ip` is a local literal address (`zt:ip-address`); `address` is a remote host
that may be a name. The two field names encode that difference on purpose.
`host` is not used, and `ip` is not used for a remote target.

`refine` sets the per-service defaults for `ip` and `port`.

| Listener pattern | When |
|------------------|------|
| `container` + `ze:listener` + `uses zt:listener` | Single-endpoint services: web, SSH, MCP, LG, telemetry, BGP global listen |
| `list` + `ze:listener` + `uses zt:listener` | Named multi-instance listeners, such as the plugin hub server |
| `container` + `ze:listener` + a manual ip/port pair | Only when the ip type differs from the standard one. BGP peer-local is the documented exception: a union with an `auto` enum |

## 12. Structure, Toggles, Defaults and Layout

| Pattern | Use |
|---------|-----|
| `grouping` plus `uses` | Shared structure within or across components |
| `augment` | Only when a plugin extends another component's YANG |

An on/off setting has one shape:
`leaf enabled { type boolean; default false; }`.

| Rule | Detail |
|------|--------|
| Positive assertion, one word | `enabled`, not `enable`, `disable` or `disabled` |
| Standard admin-state words are the only exception | `shutdown` (BFD, RFC 5880 section 6.8.16) and interface `disable` (kernel admin-down) are the canonical protocol and kernel terms, so they are allowed, typed `boolean` with `default false`, never `type empty` |
| No boolean-as-enum | A two-value on/off is not `enumeration { enum enable; enum disable; }`. A genuine tri-state for config inheritance is justified in the module; it is an exception |
| Bare flag | "This section is on when present" is a `presence` container, not a `type empty` leaf |

| Rule | Canonical | Anti-pattern |
|------|-----------|--------------|
| Boolean default is unquoted | `default false;` | `default "false";` |
| enum `value N` only for wire numbers | assign `value` when the number is protocol-significant (AFI, SAFI, ORIGIN); otherwise omit it | assigning arbitrary values to cosmetic enums |

| Layout rule | Detail |
|-------------|--------|
| Indentation | 4 spaces per level. No tabs, no 2-space modules |
| Compact leaf | A leaf whose body is only `type`, optionally with `default` and `description`, may be one line: `leaf med { type uint32; description "..."; }` |
| Expanded leaf | A leaf with nested constraints (`pattern`, `range`, `enumeration`, `must`, or several sub-statements) is expanded, one statement per line |
| List key | quoted: `key "name";`. Prefer `name` for the operator-assigned key |

## 13. Cross-Protocol Consistency

Equivalent concepts are modelled the same way across BGP, OSPF, IS-IS, BFD, LDP
and RSVP-TE, so an operator who has configured one protocol recognizes the next.

| Concept | Canonical | Do not |
|---------|-----------|--------|
| BFD integration | `container bfd { leaf enabled; [leaf mode;] leaf profile; }` referencing a profile in the top-level `bfd { profile <name> }` list, which is BGP's pattern | redefine BFD timers inline (`min-tx`, `min-rx`, `multiplier`) |
| Authentication | reference a shared `key-chains` list, which is IS-IS's model, through a `leaf key-chain`, and name the auth container the same everywhere | a per-protocol private key store; a container named `md5` in one place and `authentication` in another; a reference leaf named `key-chain` here and `auth-key-chain` there |
| Per-interface protocol config | `container interfaces { list interface { key "name"; ... } }`, as OSPF and IS-IS do | a bare top-level `list interface` (RSVP-TE), or a `leaf-list interfaces` when per-interface settings exist (LDP) |
| Multiplier, interval and timer names | one vocabulary for one concept, dimensioned through a `units` statement | four names for one concept (`detect-multiplier` against `multiplier` for the same BFD field) |
| Toggle | positive `enabled` at every nesting level, sub-features included | `enabled` on the interface and `enable` on its sub-blocks |

A genuine RFC-term difference is the only allowed divergence, and it is
justified in the leaf or container `description`. Two exist: the metric name,
OSPF `cost` against IS-IS `metric`, and the router identity, `router-id` for
BGP, OSPF and RSVP-TE, `lsr-id` for LDP, `system-id` plus `net` for IS-IS.

## 14. How a Config Value Reaches a Plugin

### Every delivered value is a JSON string

The plugin config framework hands every YANG leaf value to a plugin's
`ParseConfig` as a JSON string (`"true"`, `"50000"`, `"3.5"`), never the native
JSON type. A hand-written parser that coerces with a native type assertion fails
that assertion and silently falls back to the leaf's default. There is no error,
no panic and no log line, so a boolean `enabled` gate reading `"true"` leaves the
whole feature off.

A string-tolerant helper is the shape that works:

```go
func cfgBool(v any) (bool, bool) {
	switch b := v.(type) {
	case bool:
		return b, true
	case string:
		if pb, err := strconv.ParseBool(strings.TrimSpace(b)); err == nil {
			return pb, true
		}
	}
	return false, false
}
```

`./le config coercion check`
(`internal/le/config/coercion/configcoercion.go`, wired into
`./le verify current mode full`) parses every `internal/**/config.go` and fails
on a type switch whose cases include a numeric or bool type but not `string`, or
on a direct type assertion to a numeric or bool type.

<!-- source: internal/le/config/coercion/configcoercion.go -- the coercion check -->
<!-- source: internal/plugins/trafficusage/config.go -- cfgBool and the string arms of toInt and toFloat -->

### The shapes a leaf-list and a list arrive in

`Tree.ToMap` does not hand a reader what the YANG node type suggests, and JSON
delivery adds one more shape on top.

| Node | Members | Shape in process | Shape after JSON |
|------|---------|------------------|------------------|
| `leaf-list` | none active | key absent | key absent |
| `leaf-list` | exactly one | bare `string` | bare `string` |
| `leaf-list` | two or more | `[]string` | `[]any` |
| `list` | any count, one included | `map[string]any` keyed by the list key | `map[string]any` keyed by the list key |

A `list` is never a slice, and its key leaf is the map key rather than a field
inside the entry.

<!-- source: internal/core/configvalue -- LeafList, ListEntries -->
<!-- source: internal/core/configorder -- Entries, OrderKey -->

### Config text is set commands

The config format is set commands. Duplicate blocks are additive and the parser
merges them, so concatenating two valid config texts produces valid config.

| Manipulation method | When |
|---------------------|------|
| Parsed YANG tree | When a loaded config tree is in memory |
| Set command lines | When building or merging config text |
