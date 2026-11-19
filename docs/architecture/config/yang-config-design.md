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

A `uses` statement can hold several `augment` statements. Each is applied to
that use's copy of the grouping, before the copy joins its parent. The loader
retries dependent augment targets until no progress is possible, then refuses
an unresolved target. A uses-local augment inside a grouping leaves its
added identifiers unbound until a use outside every grouping instantiates
the grouping. Those identifiers and the grouping's original nodes then take
the instantiating module's namespace (RFC 7950 Section 7.13). An augment outside
a grouping retains its declaring module's namespace (Section 7.17.2). The
declaration's provenance does not pre-bind the copied data nodes.
An empty augment body is refused. A body containing a use of an empty grouping
remains valid: Section 14 requires a data-definition statement, which need
not produce an effective child. `anydata` and `anyxml` are permitted children,
as verified erratum 4916 clarifies.
This correction is retained in the goyang vendor patch
`internal/le/vendorpatch/patches/goyang-uses-augment.patch`.
<!-- source: vendor/github.com/openconfig/goyang/pkg/yang/yang.go -- Uses -->
<!-- source: vendor/github.com/openconfig/goyang/pkg/yang/entry.go -- ToEntry, Augment -->

`deviate add` refuses a singleton property already on its target, including
one added by an earlier deviation. Explicit statement presence is retained
separately from the value, so `config false`, `mandatory false`, `units ""`,
`min-elements 0` and `max-elements unbounded` each count as present. A child's
inherited config value does not count as its own statement. Adding a missing
property and replacing an existing one remain permitted. `type` is not admitted
by the Section 14 add grammar; a type change uses `deviate replace`.
This correction is retained in
`internal/le/vendorpatch/patches/goyang-deviate-add-presence.patch`. These
checks do not establish complete deviation support; replace-presence and
delete keyword/value matching remain outside their proof.
<!-- source: vendor/github.com/openconfig/goyang/pkg/yang/entry.go -- deviationPresenceFor, checkDeviationAdd, ApplyDeviate -->

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
| Typedefs | `ipv4-address`, `asn`, `port`, `prefix-ipv4`, `community`, `address-family`, `vlan-id` |
| Groupings | `route-attributes`, `peer-info`, `command-info`, `transaction-result` |

Other modules import it: `import ze-types { prefix zt; }` and reference its types:
`leaf as { type zt:asn; }`.

A typedef carries the operator's message on its own restriction, so every leaf
that uses it refuses in the same words. `zt:vlan-id` is a `uint16` with range
`1..4094` and an `error-message` that says IEEE 802.1Q reserves 0 and 4095. The
interface unit `vlan-id` leaf and the `vid` arguments of the interface commands
and RPCs use it ("Constraint error messages", in section 4).

<!-- source: internal/component/config/yang/modules/ze-types.yang -- typedef vlan-id -->
<!-- source: internal/component/config/yang/constraint_message.go -- LeafConstraintMessages -->
### Extensions: `ze-extensions.yang`

Defines ze-specific YANG extensions (RFC 7950 Section 7.19). These are annotations that
standard YANG tools ignore but ze interprets at runtime.

| Extension | Purpose | Argument |
|-----------|---------|----------|
| `ze:allow-unknown-fields` | Container accepts arbitrary key-value pairs | (none) |
| `ze:backend` | Restricts a node to named backends. Commit validates it and completion filters on it | space-separated backend names |
| `ze:bcrypt` | Leaf holds a one-way bcrypt hash. The commit hook hashes its `plaintext-<name>` sibling into it | (none) |
| `ze:command` | Marks a `config false` container as an executable CLI command | WireMethod string |
| `ze:method` | Declares the wire method of an rpc or notification that no command node points at: the plugin IPC protocol and the event stream | WireMethod string |
| `ze:rpc` | On a `ze:command` node, names the rpc that documents the command's input and output; the rpc is published under the node's wire method | `module:rpc-name` |
| `ze:cumulative` | Leaf-list accumulates values from the bgp, group and peer levels instead of the most specific level replacing them | (none) |
| `ze:decorate` | Attaches a registered display-time decorator to a leaf | decorator name |
| `ze:display-key` | Names the leaf the web interface shows for a keyless list entry | (none) |
| `ze:edit-shortcut` | Makes a command available in edit mode without the `run` prefix | (none) |
| `ze:ensure-exists` | Marks a command container as a resource checkpoint. Each descendant command ensures the resource exists | WireMethod of the rollback handler |
| `ze:ephemeral` | Node is validated and completed but never written to the config file | (none) |
| `ze:filter` | Marks a list under the top-level `policy` root as a named filter type. `policy.BuildRegistry` discovers the filter types from this marker, so a list without it (policy routing's `route`) is never taken for one | (none) |
| `ze:flatten` | Serializes a container's children with the container name as a leading keyword | (none) |
| `ze:help` | Declares the one-line SUMMARY of a command node, an rpc, a config node, or an enum value. The `description` statement beside it declares the long explanation | one sentence, one line |
| `ze:hidden` | Hides a node from the annotated and blame views and from the web editor. On a command or grouping, hides that path and its descendants from help and completion while full-path dispatch remains available. The file form (`Serialize`, written by `ze config fmt` and `ze config migrate`) and the set form still print a configured hidden node, so writing a config never deletes it | `true` or `false` |
| `ze:inherit` | Says whether a command takes the leaves its ancestor containers declare | `none` |
| `ze:key-type` | Key type for inline-list nodes | type name |
| `ze:keyword-only` | Marks a command's leaf as given only after its own keyword: a word no keyword introduced is never read as its value, and the usage line writes the keyword in front of the value. The `content` payload leaves carry it (`docs/architecture/api/commands.md`, "Every route validates") | (none) |
| `ze:listener` | Marks a list entry as a network listener endpoint, for port-conflict detection at parse time | (none) |
| `ze:modifier` | Marks a `config false` container as a trailing argument group of its parent command. `one-of` declares no leaf: it wraps sibling groups, the operator types exactly one of them, and the usage line renders them as a required alternation `(a <value>\|b <value>)`. The wrapper's own name is never typed, completed or listed | `once`, `repeat`, `required`, `choice`, `one-of` |
| `ze:ordered` | Leaf-list is an ordered sequence whose duplicates are meaningful (AS_PATH prepends, MPLS label stacks) | (none) |
| `ze:os` | Restricts a node to one operating system. The schema drops the node elsewhere, and records its name on the parent, so the parser refuses it as a node of that OS (`NodeOfAnotherOSError`, reported by `ze config validate` under the host-verdict code `config-node-other-os`) rather than as an unknown field. The build still converts the pruned node and keeps it on the parent, so `Schema.LookupDeclared` answers what the module declares about it, such as its defaults, on every host; `Lookup` and the parsers never reach it | GOOS value |
| `ze:rest-of-line` | Marks a command's string leaf as taking every word left on the line, with no word in it read as a keyword. At most one leaf per command, declared last (`docs/architecture/api/commands.md`, "Every route validates") | (none) |
| `ze:related` | workbench: declares an operator tool descriptor on a config node | descriptor string |
| `ze:reference` | Leaf or leaf-list names a configured entry that no leafref path can express. The argument names a source a component registers with `yang.RegisterReferenceSource`; the source answers, as a `yang.ReferenceAnswer`, the names offered at the leaf's position and, optionally, a `Check` that judges a value. Editor completion offers exactly those names (`Validator.ReferenceInstances`). Commit (`Validator.ValidateReferences`) refuses a value the `Check` refuses, with its message, or, with no `Check`, any value not among the names, naming the leaf, the value and the configured choices. A `Check` MUST accept every name offered; it exists for an owner that accepts spellings it does not offer, such as a qualified `prefix-list:NAME` or an external plugin's `<plugin>:<filter>` (the `policy-filter` source, `internal/component/policy/reference_source.go`). A source no component registered refuses every value, so `ze start` refuses a module that names one ("Joining a module that no `init()` registered", below). Use a standard `leafref` wherever a path fits | source name |
| `ze:required` | Field must hold a value after config inheritance resolves | path |
| `ze:route-attributes` | Node accepts standard BGP route attributes | (none) |
| `ze:sensitive` | Leaf holds sensitive data. Display obfuscates it with JunOS-compatible `$9$` encoding, which `ze secret decode` reverses offline (`docs/guide/command-reference.md`, "ze secret"). Every parser (hierarchical, set, set-meta, inline list entries) decodes a `$9$` value and keeps the text it read (`decodeSecretLeaf`, `Tree.SetStoredSecret`). Every printer (`Serialize`, `SerializeSet`, `SerializeSetWithMeta`, so `ze config fmt`, `ze config migrate` and the editor) prints that text back while it decodes to the value held, and encodes a key given in clear once, keeping that encoding as the stored text (`EncodeSecrets` in `config/secret_text.go`). `SerializeSetWithMeta` also writes every session entry of such a leaf in the `$9$` form, the `^previous` key included and a value one session set that another deleted (`encodeSecretMeta`), and the set-meta parser decodes both fields back. gNMI Get, Subscribe and the Set echo answer it in the `$9$` form too (`EncodeSecrets`, `SecretValueAtPath`), and `Editor.SetValue` decodes a `$9$` value a client sends and keeps its text. A mask placeholder prints as itself | (none) |
| `ze:suggest` | Field appears in a creation dialog with its inherited default. The entry is created without it | path |
| `ze:syntax` | Config parser syntax mode (flex, freeform, inline-list) | mode name |
| `ze:task-support` | MCP task-support level for a command | `required`, `optional`, `forbidden` |
| `ze:ui-csp` | CSP directive advertised in `_meta.ui.csp` | policy string |
| `ze:ui-permissions` | MCP App permission capabilities advertised in `_meta.ui.permissions` | space-separated capabilities |
| `ze:ui-resource` | Associates an embedded MCP App UI bundle with a command group | path under `internal/component/mcp/ui/` |
| `ze:validate` | References a Go validator function for runtime validation and completion | function name |

The table is complete: it holds every extension the module declares, in name
order. An extension that is absent here is a defect of this page.

The loader refuses an extension statement that nothing declares. goyang keeps
any `prefix:keyword` statement it does not know, and Ze's readers match an
extension by its keyword, so a misspelled `ze:comand` or `ze:hepl` would load
and the feature it names would be absent with no error. `Loader.Resolve`
therefore walks every statement of every loaded module and resolves each
prefix through the module's own `prefix` (a submodule's `belongs-to`) and
`import` statements. A prefix that resolves to no loaded module is refused,
and so is a keyword that the module behind the prefix, or a submodule it
includes, does not declare with an `extension` statement. The allowed set is
those declarations, so a module that declares its own extension, as
`ze-traffic-control-conf` does, needs no change here. Each refusal wraps
`ErrUndeclaredExtension` and names the module, the file location and the
statement. `DefaultLoader` returns this one with every other failure, so the
daemon, the CLI and the `./le` tools all refuse the same schema. These callers return the error with
its cause and never work from a missing schema: an SSH session and the config
editor refuse to build their completion tree, `ze help ai` and the MCP
`ze_reference` tool fail, `ze yang` fails, interface-name validation fails
rather than accept a reserved CLI keyword, and authorization profile
extraction fails at daemon start and on reload instead of checking match
entries against an empty command set. The API and MCP command metadata
(`commandMetaSource`) caches the error with the metadata and returns it on
every call: REST and gRPC answer an internal error naming the cause, and MCP
`tools/list` and `tools/call` answer a JSON-RPC internal error. The `ze cli` <!-- doc-links: ignore (JSON-RPC method name, not a path) -->
client builds its YANG state on first use, never at package init, and caches
the error with it: every accessor returns it, so `ze` help, usage, the menu,
verb dispatch, shell completion and the command catalog report the cause and
exit 1, and `IsDeclaredCommand` returns it rather than `false`, so the
local-handler lookup refuses the match instead of serving a handler that
shadows a declared command.

The same two callers refuse the structures below, which RFC 7950 forbids and
goyang accepts, and the three built-in types whose values Ze cannot check. Each
refusal names the module, the file location and the fault.

| Structure refused | RFC 7950 | Why goyang misses it | Error |
|-------------------|----------|----------------------|-------|
| An `include` of a submodule whose `belongs-to` names a module other than the including module, or other than the module the including submodule belongs to | 7.2.2 | it finds an included submodule by its name alone and never reads its `belongs-to` | `ErrIncludeBelongsTo` |
| A `length` whose parts, in the order written, overlap or descend, with `min` and `max` read as the bounds of the type being restricted | 9.4.4 | it sorts and coalesces the parts before it checks them | `ErrLengthOrder` |
| An enumeration whose values break Section 9.6.4.2: a value outside int32, a value assigned twice, an enum with no value after one holding 2147483647 | 9.6.4.2 | it checks only an enumeration it resolves, and it resolves none in a grouping no schema node uses | `ErrEnumValue` |
| An `enum` in a restricted enumeration that the base type does not assign, or whose `value` differs from the base type's | 9.6.4, 9.6.4.2 | it builds the restricted values afresh and never compares them with the base | `ErrEnumRestriction` |
| A statement under an extension statement that is not a YANG keyword, that the block holding it does not admit, whose argument breaks its Section 14 argument rule, whose substatements break the counts or the alternatives of its rule's block, or that omits a block its rule requires | 7.19 | it keeps an extension statement as raw text | `ErrExtensionSubstatement` |
| A leafref whose path names no node, ends at a node that is not a leaf or leaf-list, puts a predicate on a node that is not a list or on a leaf that is not one of its keys, or whose `current()` expression reaches no leaf; a node name, at a step, a key predicate or a `current()` expression, whose prefix the module the path is written in (for a typedef, the module defining it, not the one using it; for a type a `deviate` gives the leaf, the deviating module) neither declares as its own nor imports, or whose prefix names a module other than the one holding the node. The first step of an absolute path, and of a relative path or `current()` expression that climbs to the root, is looked up at the top level of the module its prefix names, among all loaded modules, so a path may enter an imported module or a node another module augments; a prefixed first step its module's top level does not hold is looked up among every loaded module, so the refusal names the right prefix. An unprefixed first step is looked up at the top level of the module whose namespace the referring node is in, and only there: for a leaf another module augments in, that is the augmenting module, not the module holding the augment target (4.2.8). An unprefixed name, at any step, key predicate or `current()` expression, must name a node of that module (inside a grouping, the module using it). A section Ze merges across `-conf` modules into one config data node is no exception: a path into another module's section writes that module's prefix, as in `/zb:sec/zb:y`. A path read from the top level whose steps name no node, where another loaded module's top level holds them by their local names, is refused naming that prefixed path and every import to add. The accessible tree holds data nodes only, plus the operation the referring leaf is defined in (6.4.1): an rpc or a top-level notification is a child of the root, an action or a nested notification a child of the data node it is defined in (7.15.2), and its children are the parameters of the referring leaf's own input or output, or the notification's nodes. No other rpc, action or notification is reachable, and a step naming one is refused saying it is an rpc, action or notification outside the accessible tree; `../` passes through `input` and `output` as it does through choice and case, so from an action parameter `../` is the action and `../../` its container. Each copy of an rpc or action that a grouping use or a submodule include makes has its own input and output, parented to it, and so does an input or output an augment creates, so an augment or a deviation aimed at the parameters of one use of a grouped action changes that use only (7.17; goyang vendor patch `internal/le/vendorpatch/patches/goyang-entry-dup-rpc.patch`). A prefix refusal names the fix: the written module's own prefix, the prefix it imports the node's module under, the import to add, or, inside a grouping used in the node's module, the name with no prefix | 9.9.2, 6.4, 6.4.1 | it keeps the path as text and resolves nothing | `ErrLeafrefPath` |
| A config leafref with require-instance true whose referred leaf is state (a leaf under an rpc or action input or output, or a notification, is no configuration and may refer to state), and a chain of leafrefs that comes back to a leaf already in it | 9.9 | it never follows a path | `ErrLeafrefPath` |

The grammar under an extension is the RFC's own: `rfc7950.abnf`, embedded in
the package, is the Section 14 code component of `rfc/full/rfc7950.txt`, and
`TestEmbeddedGrammarIsTheRFC7950Grammar` turns red if the two differ.
`parseYANGGrammar` parses every rule of it into an RFC 5234 expression and
reads each `<name>-stmt` rule into a production: its keyword, its argument
rule, whether its block is required (a bare `{`) or may be omitted (`stmtend`,
or `";" / "{" ... "}"`), and the expression its block holds. A keyword with
several rules has several productions: `deviate` four, `augment` two.

Each statement under an extension is resolved to a production chosen among
those its parent's block admits, the one whose argument rule its argument
matches. The parent's rule is the context, so `augment` under `uses` is
`uses-augment-stmt` and takes a descendant path, while `augment` in a
submodule body is `augment-stmt` and takes an absolute one, and each `deviate`
argument selects its own block. Directly under the extension usage the block
is `unknown-statement`'s, `*((yang-stmt / unknown-statement) optsep)`, which
admits every production.

The statement is then checked against its production. Its argument matches
the rule's checker; every argument rule has one, `uri-str` read as an RFC 3986
URI and `path-arg-str` as the Section 14 `path-arg`. Its substatements, read in
any order as the ABNF comment allows, must match the block: each repetition
bounds a count (`[x]` at most one, `*x` any, `1*x` at least one, a bare `x`
exactly one), and the counts together must form one reading of the block, so
`deviate not-supported` cannot sit beside `deviate add`, and a `type` holds
the restrictions of one alternative of `type-body-stmts` only. goyang records
no block for a statement with no substatement, so where an empty block would
match a rule that requires one (`refine-stmt` alone), the text the
statement's module was parsed from is read to tell `refine x;` from
`refine x {}`. The loader binds each text to the modules its parse added,
never to the file name, so two texts loaded under one name each answer for
their own modules. A module goyang read from disk itself, resolving an
import, has no bound text, and the question is refused rather than guessed.

The ABNF does not decide which alternative of `type-body-stmts` a base type
takes: the grammar accepts `type int8 { length "1"; }`, because
`string-restrictions` is one alternative, and binding restrictions to base
types is RFC 7950 Section 9's work, not Section 14's. Nor does it decide the
order of substatements, which its comment frees.

`min` and `max` in a `length` are the first and last bounds of the effective
length of the type being restricted, which goyang resolves through the whole
typedef chain, or 0 and 18446744073709551615 when no type in the chain carries
a length.

An enum's value is Ze's, never goyang's. `parseEnumAssignment`
(`internal/component/config/yang/enum_assignment.go`) reads the enum statements
of the root `type enumeration`: an enum with no `value` takes 0 when it is the
first, and otherwise one more than the highest value before it, whatever its
`if-feature`. goyang v1.6.3 started its count at 0 after a negative value,
assigning the enum after `enum p { value -5; }` 0 where the RFC assigns -4, and
refused `enum p { value -5; } enum q; enum r { value 0; }` as a conflict on 0.
Ze vendors upstream goyang with the fix proposed in openconfig/goyang#317
applied as a vendor patch (`internal/le/vendorpatch/patches/goyang-enum-negative.patch`,
`docs/contributing/patching-the-standard-library.md`), until a goyang release
holds it. goyang still numbers a restriction's enum statements afresh. A
restriction, however many typedefs down, keeps the root's values. Both checks
above, the schema node's enum list (`EnumNamesDeclared`) and a command
argument's (`argDefFor`) read that one assignment, and the two lists are in
value order. An enumeration in a grouping no schema node uses is still checked
for its values. A restricted enumeration that sits in such a grouping
is never resolved by goyang, restricts nothing, and is not checked. goyang
does resolve the type a `length` restricts in such a grouping, so its `min`
and `max` read the typedef's bounds there too; a `length` over a typedef that
goyang left unresolved and that names `min` or `max` is refused as unresolved
rather than checked against a guessed span.

<!-- source: internal/component/config/yang/loader.go -- Resolve, checkExtensions, DefaultLoader -->
<!-- source: internal/component/config/yang/enum_assignment.go -- parseEnumAssignment, assignEnumValues -->
<!-- source: internal/component/config/yang/loader_structure.go -- checkStructure, moduleIncludeErrors, restrictedLengthSpan, resolveProduction, extensionSubstatementError -->
<!-- source: internal/component/config/yang/loader_reference.go -- checkReferences, checkLeafrefSchema, representsConfig -->
<!-- source: internal/component/config/yang/reference_path.go -- leafrefTarget, checkPathKeys, checkPathPrefix, topLevelParents, pathStatement, prefixFix, dataParent, operationContext, outsideOperation, rootStepsFix -->
<!-- source: internal/component/config/yang/loader_abnf.go -- parseYANGGrammar, rfc7950Grammar -->
<!-- source: internal/component/config/yang/loader_grammar.go -- argumentCheckers, checkURI, checkPathArg -->
<!-- source: internal/component/config/yang/loader_source.go -- statementHasBlock -->
<!-- source: internal/component/config/yang/loader.go -- sourcedModules.Parse, sourcedModules.source -->
<!-- source: cmd/ze/hub/command_meta.go -- commandMetaSource -->
<!-- source: internal/component/cli/client/main.go -- loadYANGState, buildYANGState -->
<!-- source: internal/component/cli/client/verb_tree.go -- IsDeclaredCommand -->
<!-- source: internal/component/command/local.go -- LookupLocal -->
<!-- source: cmd/ze/hub/session_factory.go -- buildCommandTree -->
<!-- source: internal/component/config/cli/cmd_edit.go -- buildEditorCommandTree -->
<!-- source: internal/component/aihelp/aihelp.go -- CLISubcommands, Build -->
<!-- source: internal/component/config/yang/cli/tree.go -- addCommandNodes -->
<!-- source: internal/component/config/infra/authz.go -- validateMatchEntries -->
<!-- source: internal/component/iface/validate.go -- loadReservedIfaceNames -->

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
`config false` containers with `ze:command` extensions. A command node's
`ze:rpc` statement names the rpc that documents its input and output, and the
rpc is published under that node's wire method: the rpc names no method of its
own, and no method is built from a module's file name. An rpc no node reaches
and a notification declare their wire method with `ze:method`. This includes
the plugin IPC protocol in `internal/core/ipc/yang/` and operations registered
by a subsystem, such as BGP source leases. The declaration publishes the exact
method a plugin sends without adding an operator command. An rpc that has
neither declaration is published under no name, and the command contract gate
refuses it.
<!-- source: internal/component/config/yang/rpc_publish.go -- PublishedRPCs -->
<!-- source: internal/core/ipc/yang/ze-plugin-engine.yang -- select-routes, cancel-request -->
<!-- source: internal/component/bgp/yang/ze-bgp-api.yang -- source-lease -->

| Schema | Purpose | Location |
|--------|---------|----------|
| `ze-bgp-api` | BGP peer/route/cache RPCs | `component/bgp/yang/` |
| `ze-rib-api` | RIB query RPCs | `component/bgp/plugins/rib/yang/` |
| `ze-*-cmd` | CLI command tree nodes | Various `schema/` directories |

---

> **See also:** [Config Transaction Protocol](transaction-protocol.md) for the bus-based
> verify/apply/rollback lifecycle that config changes go through after validation.

---

## 3. Module Loading

YANG modules are loaded in two phases at startup.

<!-- source: internal/component/config/yang/loader.go -- LoadEmbedded, LoadRegistered, DefaultLoader, Resolve, Resolved, ErrLoaderResolved -->
<!-- source: internal/component/config/yang/command.go -- BuildCommandTree, compiledPatterns.compiled -->
<!-- source: internal/le/doc/yangcontract/usage.go -- usageBaseline -->

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

<!-- source: internal/component/config/yang/register.go -- RegisterModule -->

Registration stamps the caller's package into a private field. `Module.Registrar()`
reads that provenance, and `ModuleRegistrar(name)` looks it up in the registry.
`Modules()` returns a caller-owned snapshot. A caller can edit or sort that
snapshot without changing registered names, text, provenance or the schema generation.

<!-- source: internal/component/config/yang/register.go -- Module, Registrar, ModuleRegistrar, Modules -->

`LoadRegistered()` attempts every registered module and joins every parse
error, each naming its module, so one broken module does not hide the modules
registered after it.

After both phases, `Resolve()` resolves all cross-module imports via goyang and
runs the checks above. It is the one transition from loading to reading, and it
answers a distinct type:

| Type | Holds | Operations |
|------|-------|------------|
| `Loader` | Modules added, not yet checked | `LoadEmbedded`, `LoadRegistered`, `AddModuleFromText`, `AddModuleFromFile`, `Resolve`; `StageProcess` for the staged benchmark |
| `Resolved` | A module set every check passed, and every pattern `checkPatterns` compiled, keyed by its text | `GetModule`, `GetEntry`, `ModuleNames`, `ConfModuleNames`, `APIModuleNames`; `RecheckExtensions`, `RecheckPatterns`, `RecheckStructure`, `RecheckReferences` for the staged benchmark |

Only a successful `Resolve` (or `DefaultLoader`) produces a `Resolved`, and
every reader of a checked module set takes one: `BuildCommandTree`, the
`PathTo*` and `WireMethodTo*` maps, `PublishedRPCs`, `ExtractRPCs`,
`ExtractNotifications`, `NewValidator` and `CheckAllValidatorsRegistered`. A
caller that ignored a resolution error has no value to pass, so building a
command tree from a module set that failed its checks does not compile. The
command lowering reads each argument's patterns from the `Resolved` rather than
compiling them again; a pattern it does not find there is a lowering that
reads a pattern the check never saw, a Ze defect, and panics with a `BUG`.

`Resolve` runs once. Go leaves the `Loader` usable after it, and the `Resolved`
shares its module set, so every load and every further `Resolve` after the
first, successful or not, returns `ErrLoaderResolved`. The zero value and nil
of `Resolved` compile in any package; only Ze code can build one, and every
accessor on one ends in a `BUG` panic rather than answering an empty module
set.

The build can also run one stage at a time, so the cost of each is measured
alone (`loader_stages.go`). `StageLoad` performs the load `resolveDefault`
performs, `Loader.StageProcess` runs goyang's `Process` and leaves the loader
resolved as `Resolve` would, and the `Recheck` methods of a `Resolved` run one
of `Resolve`'s checks again over its module set, which no check writes to.
`ForgetCompiledPatterns` empties the process-wide pattern cache, so a pattern
check compiles every pattern as a process's first `Resolve` does. A joined
`Resolved` (`Join`) holds compiled-in modules the join marked judged, which
the extension, pattern and structure checks skip, so their rechecks refuse
such a set with `ErrRecheckJoinedSet` rather than measure the external
modules under the whole check's name; the leafref recheck judges every module
and runs on any `Resolved`. Each entry runs the stage function itself, never
a copy of its body, and `TestStagesRunEveryStageOfResolve` refuses a call
`Resolve` or `resolveDefault` gains on their `Loader`, or of a package
function, that the file does not call. A stage added inside one of the
functions they call is measured on that function's line. The one user is
`BenchmarkSchemaBuildStages` (`internal/component/plugin/all`, where every
module is registered), which prints one line per stage (load, process, each
check, the whole `Resolve`, and the lowering to `config.Schema`) with its
allocations and the module count it measured.
<!-- source: internal/component/config/yang/loader_stages.go -- StageLoad, StageProcess, RecheckStructure, ErrRecheckJoinedSet, ForgetCompiledPatterns -->
<!-- source: internal/component/plugin/all/schema_build_bench_test.go -- BenchmarkSchemaBuildStages -->

`DefaultLoader()` runs both phases and `Resolve()`, and it is strict: nothing is
best-effort. A registered module that does not parse, an import that no linked
package registers, an undeclared extension, an uncompilable pattern and a
refused structure each come back, joined, and `DefaultLoader` then answers no
`Resolved`. The `./le doc yang-contract` usage baseline built from the modules at
git HEAD goes through the same `Resolve`. Its callers return or report that error and never serve a schema that
silently lacks a module. A binary links every module a linked module imports,
because the generated `register.go` of each `yang/` package blank-imports the
packages registering its modules' imports (`command-ownership.md`, "YANG as
Data, Not Code").

A test binary is one of those callers, and it links only what its own blank
imports reach, so the resolve error is the one place a module it lacks is
named. `TestEverySchemaResolvingCallerChecksTheResolveError`
(`internal/component/config/yang/schema_census_test.go`) holds every caller to
that, tests included. It derives the functions that answer the schema from the
two producer packages' source: each package-level function of `config/yang`
and `config` that answers a `*Resolved` or `*Schema` with an error and reaches
`resolveDefault`. It then walks the tree and refuses any call that assigns
their error to `_` or drops it, unless the line carries a reviewed
`//nolint:errcheck // <reason>`. `TestThisBinaryLinksEveryModuleItsSchemaImports`
(`internal/component/config/cli/schema_link_test.go`) resolves the schema in
the binary of the schema-dependent config CLI cases, so a module that binary
does not register is named there before any case reads a path.
<!-- source: internal/component/config/yang/schema_census_test.go -- TestEverySchemaResolvingCallerChecksTheResolveError, schemaResolvers -->
<!-- source: internal/component/config/cli/schema_link_test.go -- TestThisBinaryLinksEveryModuleItsSchemaImports -->

`DefaultLoader()` resolves once per registered module set and hands the same
`Resolved`, or the same error, to every caller in the process. Every CLI
model, editor session, restart, completer and value validator asks for that
schema at construction, and each used to resolve every module again. The
cache holds one entry, keyed by the registry generation that `RegisterModule`
and `RegisterModuleForTest` advance and the latter's restore puts back, so a
test that registers a module for its own duration is answered over that module
set. Once the process published the modules of its external plugins, the entry
holds the compiled-in set joined to them ("Publishing the joined set to the
whole process", below). The shared `Resolved` is read-only. Its `Loader` is unreachable and its
module set refuses every load (`ErrLoaderResolved`). `Resolve` has already
converted every module and submodule to its entry tree, so `GetEntry` reads
goyang's lock-guarded entry cache and concurrent readers are safe. A caller
MUST NOT write to an entry or a module it reads. A loader built from text
(`AddModuleFromText`) and the set the join answers for module texts
(`YANGSchemaWithPlugins`, below) are not shared, because that module set is
the caller's own. The plugin-free
`config.YANGSchema()` and `config.YANGValidator()` are built over
`DefaultLoader`'s `Resolved` and cached against that same `Resolved`
(`resolvedCache`), so the schema, the value validator and the completer always
answer from one module set, a registered test module included.
<!-- source: internal/component/config/yang_schema.go -- YANGSchema, YANGValidator, resolvedCache, loadYANGModules -->

### Joining a module that no `init()` registered

An external plugin declares its YANG module in its Stage 1 declaration, so no
`init()` registers it. Such a module enters a module set through one function,
`yang.Join`. It takes the resolved compiled-in set, the declared modules (each
with the plugin name, the declared module name, the declared namespace and the
text) and the validator registry, and it answers a new `Resolved` or an
operator error that names the plugin and the module. `config.JoinExternalModules`
is its entry: it passes the set `DefaultLoader` shares and the registry
`RegisterValidators` fills.

The daemon's start path calls the join with each plugin's declaration:
`runYANGConfig` (`cmd/ze/hub/main.go`) calls `joinDeclaredPlugins`
(`cmd/ze/hub/declared_plugins.go`) between the first pass and the full parse,
and that calls `config.PublishExternalModules`, which joins the declared
modules and publishes the result ("Who publishes", below). Config under an
external plugin's own root therefore loads at `ze start`
(`docs/plugin-development/schema.md`, "What works today"). No command with no
daemon calls the join with a declaration: it starts no plugin. The join's
other callers are `YANGSchemaWithPlugins` and `YANGValidatorWithPlugins`,
which pass module texts a Go caller holds (`loadYANGModules`, with
`yang.ExternalModuleFromText` reading each text's name and namespace). They
are how a compiled-in plugin that is selected on the command line (`ze start
--plugin <name>`, and each reload that repeats it) reaches the join with its
`Registration.YANG`. `loadConfig` collects those texts with
`plugin.CollectPluginYANG` and parses with `YANGSchemaWithPlugins`, and the
BGP reactor's file loader builds its validator the same way.
`loadYANGModules` leaves out a text identical to a module the set holds, so
an in-tree plugin, whose yang package's `init()` registers the same text,
adds nothing. Any other text goes through `JoinExternalModules`. A
registration whose module no `init()` registered therefore has its root in
the schema only when its plugin is selected (`TestReloadVerifierRetainsSelectedPluginSchema`).
This is not a second road beside the published set: both roads end in
`yang.Join`, and they carry different inputs.
<!-- source: internal/component/config/loader.go -- loadConfig -->
<!-- source: internal/component/bgp/config/loader.go -- loadReactorFile -->
<!-- test: internal/component/bgp/config/loader_test.go TestReloadVerifierRetainsSelectedPluginSchema -->

The join parses the text of every compiled-in module again and runs goyang's
processing over the whole set with the declared modules (`resolveJoined`), so
a start that declares an external plugin parses and processes the module set
twice. Ze's per-module checks are not run twice: see the next paragraph.
<!-- source: cmd/ze/hub/declared_plugins.go -- joinDeclaredPlugins -->
<!-- source: internal/component/config/yang/join.go -- resolveJoined -->
<!-- source: cmd/ze/hub/main.go -- runYANGConfig -->
<!-- source: internal/component/config/yang/publish.go -- PublishExternalModules -->

That text road has one rule of its own. A text whose module the compiled-in
set already holds with the same content, byte for byte, is that module: it
adds nothing, so it is left out before the join (`Resolved.HoldsModuleText`,
which compares with the text the set recorded when it loaded the module,
embedded or registered). A text with a held name and different content goes to
the join and is refused as a name clash. When every text is left out, the
answer is the shared compiled-in set and nothing is resolved. The rule does not
apply to a module an external plugin declares: `JoinExternalModules` refuses a
compiled-in name even when the text is identical.

The joined set is resolved by one `Resolve` of a new `Loader` that holds every
module text of the compiled-in set and the external ones
(`Resolved.reloaded`). goyang's processing and the leafref check
(`checkReferences`) judge the whole joined set, because a path or an augment
can reach a node of another module. The extension, pattern and structure
checks (`checkExtensions`, `checkPatterns`, `checkStructure`) judge the
external modules only: the loader marks every reloaded module judged
(`markJudged`), and the checks skip a judged one (`sourceModules`). That is
sound because each of the three reads one module's statements and text and
the modules its prefix, imports, includes and belongs-to name, every
compiled-in module passed them when its set resolved, and the join refuses an
external module that holds a compiled-in name or a submodule before it
resolves anything, so no external module can change what those names reach.
The joined `Resolved` holds the patterns the compiled-in set compiled beside
the external ones, so the command lowering finds every pattern it reads. The
compiled-in `Resolved` is not changed. With no external module the join
answers the compiled-in `Resolved` itself and resolves nothing, so
`YANGSchema` and `YANGValidator`, which are cached against that `Resolved`,
build nothing twice.

When the resolve of several external modules fails, the join names the first
declared module an error belongs to, read where each producer records it and
never from text an error quotes (`failingModule`). A goyang error belongs to
the module whose file it begins with: goyang writes the location of the node
it was building first, and a node an augment adds twice begins with the
augment. Each module is parsed under its own file name
(`<plugin>/<module>.yang`). A leafref error belongs to the module whose
statement wrote the leaf. An extension, pattern or structure error belongs to
the module those checks refuse when they judge it alone. Some goyang errors
carry no location: an import or include of a module nothing loaded (`no such
module: <name>`, `no such submodule: <name>`) and a deviation whose target or
type it cannot find. For those alone each module is resolved again with the
set, the first that fails is named, and when every one resolves alone the
refusal names every plugin and says the modules are not valid together.

goyang records a node an augment adds that its target already holds on the
target's entry, and its `Process` reports entry errors only for a module that
still holds an augment it could not apply. `Resolve` reads every entry tree
again after a `Process` that reported nothing (`augmentErrors`), so such an
augment is refused rather than dropped.
<!-- source: internal/component/config/yang/join.go -- Resolved.reloaded, resolveJoined, failingModule, resolveErrorOrigins -->
<!-- source: internal/component/config/yang/loader.go -- markJudged, sourceModules, checkPatterns, augmentErrors, moduleCheckErrors -->
<!-- test: internal/component/config/yang/join_checks_test.go TestJoinDoesNotCheckCompiledInModulesAgain -->
<!-- test: internal/component/config/yang/join_checks_test.go TestJoinNamesTheFailingModuleOfSeveral -->
<!-- test: internal/component/config/yang/join_checks_test.go TestJoinedSetKeepsThePatternsOfCompiledInModules -->


The join refuses each case below, and nothing is dropped in silence:

| Case | The refusal names |
|------|-------------------|
| The text does not parse or resolve | The plugin, the module and the loader's error with its location; of several modules, the first an error belongs to |
| The text holds no module, several modules, or a submodule | The plugin |
| The declared module name or namespace differs from the text | The plugin and both values |
| The module name is held by a compiled-in module, at any revision, or by an earlier external module | The plugin, the module and the holder |
| The namespace is held the same way | The plugin and both modules |
| The module declares definitions no reader of ze takes: its name does not end in `-conf`, it augments nothing, it is not a `-cmd` module whose `config false` nodes carry a `ze:command` or a `ze:help`, it holds no rpc, and no other module of the set imports it. The compiled-in set is held to the same rule (`TestNoShippedModuleAddsNothing`) | The plugin, the module and the naming rule |
| The module declares no statement at all (a header, imports and revisions only) and no other module of the set imports it. The compiled-in set is held to ship none (`EmptyModuleNames`, `TestNoShippedModuleAddsNothing`) | The plugin, the module, and the fix that drops it from the declaration |
| A `ze:command` on a path, or a wire method, another module already declares | The plugin, the module, the path or method, and the holder |
| A `ze:rpc` or wire method `PublishedRPCs` refuses over the joined set and not over the compiled-in one, or a `ze:rpc` naming a module no module is | The plugin and the pointer |
| A top-level node of an external `-conf` module is already defined by a `-conf` module | The plugin, the node, the module that defines it and the `augment` to write |
| The module names a `ze:validate` function or a `ze:reference` source the daemon does not hold | The plugin, the module, the extension, the name and the schema path |

goyang keys a module by name and revision, so it accepts a second revision of
a held name as another module; the join compares names itself.

The redefinition refusal judges external modules only. The compiled-in modules
may share a top-level container by a second definition, which `Schema.Define`
merges: six shipped modules each define `container service`. An external module
reaches such a container with `augment`, importing one module that defines it.

The registration check reads each extension's own registry: the
`ValidatorRegistry` for `ze:validate` and the sources `RegisterReferenceSource`
holds for `ze:reference`. It walks the joined entry trees and judges the
entries an external module wrote, so a leaf augmented into a compiled-in
container is judged too. Two registries cannot be reached from `config/yang`
and are not judged by the join: the display decorators of `ze:decorate`
(`internal/component/web`) and the backends of `ze:backend` (each component's
`RegisterBackend`).

The rule for `ze:validate` and `ze:reference` is one rule on both sides: a
name ze does not hold refuses the start. One judge (`unheldName`) reads both
registries. For an external module the join refuses it, naming the plugin. For
a compiled-in module `ze start` refuses before the first pass
(`refuseWronglyBuilt`, `RefuseUnheldCompiledRegistrations`). That refusal
names the module, the extension, the name and the schema path, says this ze
was built wrong and the config is not at fault, and is recorded under
`doctor-build-registration-unheld`. Every Ze package registers its names from
`init()`, and a gated module's names are registered under the same gate, so a
shipped ze holds them all. `TestCompiledModulesNameOnlyHeldRegistrations`
(`plugin/all`) judges the shipped set under the tags of its run. A process
that is not a daemon (a local command, or a test binary that links a module
without the package registering its names) does not refuse when it builds the
value validator. `validatorFromResolved` logs each unheld name under the same
code: a leaf whose function is absent is not validated there, and one whose
source is absent refuses every value.
<!-- source: internal/component/config/yang/registration_held.go -- unheldName, RefuseUnheldCompiledRegistrations -->
<!-- source: cmd/ze/hub/build_registrations.go -- refuseWronglyBuilt -->
<!-- test: internal/component/plugin/all/registrations_held_test.go TestCompiledModulesNameOnlyHeldRegistrations -->

The schema and the validator of a joined set are built by the functions that
build them for the compiled-in set (`schemaFromResolved`,
`validatorFromResolved`), so a range or a pattern of an external module
refuses a value as a compiled-in one does. The same conversion orders every
block's children in YANG definition order, a joined module's augments
included, by the rule `docs/architecture/config/syntax.md` states in "Child
order".
<!-- source: internal/component/config/yang/join.go -- Join, ExternalModule, ExternalModuleFromText, declaredModules, resolveJoined, refuseIdleModules, refuseRedefinedRoots, refuseUnheldRegistrations -->
<!-- source: internal/component/config/yang_schema.go -- JoinExternalModules, loadYANGModules, newValidatorRegistry, schemaFromResolved, validatorFromResolved -->
<!-- source: internal/component/config/yang/join_test.go -- TestJoinWithNoExternalModuleAnswersTheCompiledSet, TestJoinRefusesModuleNameClash, TestJoinRefusesTopLevelRedefinition -->
<!-- source: internal/component/config/yang/join.go -- Resolved.HoldsModuleText -->
<!-- source: internal/component/config/join_schema_test.go -- TestJoinedModuleRangeRefusesOutOfRangeValue, TestModuleTextRoadRefusesRegisteredModule, TestModuleTextRoadLeavesOutIdenticalRegisteredModule, TestJoinExternalModulesRefusesIdenticalCompiledInModule -->

### Publishing the joined set to the whole process

The join answers a set to its caller. `yang.PublishExternalModules` makes that
set the one `DefaultLoader` answers for the rest of the process, so every
reader of the schema sees the modules of external plugins: it takes what the
join takes, calls the join, and swaps the joined `Resolved` into
`DefaultLoader`'s cached entry. `config.PublishExternalModules` is its entry,
with the validator registry the daemon holds.

| Call | What happens |
|------|--------------|
| The join refuses a module | Nothing is published. The join's error comes back unchanged, `DefaultLoader` keeps answering the compiled-in set, and a later publish is still possible |
| No external module | Nothing is joined or resolved, and `DefaultLoader` answers the `Resolved` it answered before. The process's set is recorded as holding no external module |
| A second call with the same modules, in any order | Nothing changes |
| A second call with other modules | Refused with an operator error whose fix is the restart of `ze`, naming each plugin whose declaration differs |

The set is published at most once per process because a plugin, and so its
module, changes only when `ze` restarts. `yang.PublishedExternalModules`
answers the published modules and whether a publish happened, and
`yang.PublishedModule` compares one declaration (plugin, module name,
namespace and text) with the published set: not yet published, published,
changed, or no module of that plugin.

A refused join at boot is logged and raised as `doctor-plugin-schema-refused`
and ends the start (`pluginserver.ReportRefusedSchema`). A declared plugin of
which boot holds no schema is reported as `doctor-plugin-schema-undeclared`
(`docs/architecture/hub-architecture.md`, "What the daemon does at start").

The schema CLI reads the published set and never a plugin. Inside a running
daemon `buildSchemaRegistry` registers each published module beside the
compiled-in ones, so `show schema list` lists an external plugin's module
with the config roots it wants. The daemon serves no `show schema show`: the
text of one module is printed by the local `ze schema show <module>` alone,
which holds compiled-in modules only when no daemon published a set. With no
daemon the published set is empty, and a `--plugin` that names
an external program is not started: `ze schema` answers the compiled-in
modules and says on stderr that the plugin's module is not available with no
running ze, with `ze cli` as the fix: its why names `show schema list` and
`show yang tree config`, typed in that session. The earlier run of the program with
`--yang` is deleted; `--yang` remains a flag of Ze's own plugin CLI
(`ze plugin <name> --yang`), which prints a module compiled into `ze` and
starts no program. `show yang` reads `yang.DefaultLoader`, so inside the daemon
it answers the published set by itself: `show yang tree config` shows the
nodes of an external plugin's module, with their types and descriptions.
`ze cli -c` answers both commands in its own process, so the daemon's answer
is read in a `ze cli` session or over SSH.
<!-- source: internal/component/config/schema/cli/main.go -- buildSchemaRegistry, registerPublishedYANG, externalNotShown, commandLineRegistry -->
<!-- source: internal/test/fixture/ui_fixture_external_yang_session.go -- externalYANGDaemonSchema -->
<!-- source: internal/component/config/schema/cli/external_plugin_test.go -- TestSchemaRegistryStartsNoExternalProgram, TestSchemaRegistryHoldsPublishedExternalModule, TestExternalNotShownNamesThePluginAndTheFix -->

A plugin the daemon starts again, after it ended or by `request plugin
restart`, sends its Stage 1 declaration again, and the `schema-unchanged`
entry of the Stage 1 table compares it with that set (`refuseChangedSchema`).
The same list of modules (config, command and rpc modules alike), or none
where the plugin declared none, lets the restart proceed. A module whose text,
name or namespace differs from the one published under its name, a module the
plugin did not declare when ze started, a published module it no longer
declares, or no module where it declared some refuses the plugin at Stage 1
with an operator error that names the first difference and whose fix is the
restart of `ze`. The refusal is
logged and raised as the warning `doctor-plugin-schema-changed`, the published
set is not written, and the config under the plugin's root stays loaded. The
first Stage 1 of a plugin takes the same check, with the declaration boot
published. A plugin compiled into `ze` is not compared.
<!-- source: internal/component/plugin/server/stage1_schema.go -- refuseChangedSchema, schemaDifference, moduleDifference -->
<!-- source: internal/component/plugin/server/stage1_checks.go -- stage1Checks -->
<!-- source: internal/component/plugin/server/stage1_schema_test.go -- TestStage1RefusesChangedDeclaration, TestStage1AcceptsUnchangedDeclaration, TestOnRegistrationRefusesChangedSchema -->

A `Resolved` is immutable, so a reader that already holds one keeps the set it
took, and a reader that asks after the publish gets the joined set. The swap
runs under the lock `DefaultLoader` reads under: a concurrent caller gets the
set before the publish or the set after it. `config.YANGSchema` and
`config.YANGValidator` are keyed on the `Resolved` (`resolvedCache`), so they
build once more for the joined set and follow the publish by themselves. With
no external module they build nothing twice.

A value built from the module set and kept longer than one call follows the
publish in one of the ways below, and none can answer from the earlier set in
silence:

| Value | How it follows the publish |
|-------|----------------------------|
| `config.YANGSchema`, `config.YANGValidator` | Keyed on the `Resolved` `DefaultLoader` answers |
| The `cli/client` YANG state (`loadYANGState`) and the headless harness's command tree (`headlessCommandTree`) | Keyed on the `Resolved`: `ze` builds the first one to route its own command line, before the daemon publishes |
| The command argument definitions (`SeedCommandArgDefs`, `commandArgDefs`) | Built again by the publish, because any route that judges arguments may have built them first |
| The completer's value schema (`Completer.valueSchema`) | Built from the `Resolved` the completer took at `NewCompleter`, so a completer always judges with the set it completes from |
| The plugin server's command maps (`pluginserver.NewServer`), the gNMI server's config schema and model list (`gnmi.NewServer`), the three backend gate schemas (`validateBackendGate` in `iface`, `firewall` and `traffic`), the reserved interface names (`iface.ValidateIfaceName`) | Built once for the life of the process, after the publish. Each records itself with `yang.KeepsModuleSet` before it reads the set, and a publish that would change the set after one of them is refused, naming it |
| The hub's command metadata (`commandMetaSource`) | Built once, from a plugin server, so it exists only after `NewServer` |

The process therefore publishes before it parses the full config and before it
builds its plugin server. A publish that comes after a recorded value is a Ze
defect in the order of the start, and the refusal says so.
<!-- source: internal/component/config/yang/publish.go -- PublishExternalModules, swapModuleSet, PublishedExternalModules, PublishedModule, ModulePublication, KeepsModuleSet, refuseKeptModuleSet, republishRefusal, rebuildCommandArgDefs, ResetPublishedModulesForTest -->
<!-- source: internal/component/config/yang/loader.go -- DefaultLoader, moduleSetLocked, resolveProcessSet -->
<!-- source: internal/component/config/yang_schema.go -- PublishExternalModules, JoinExternalModules, resolvedCache -->
<!-- source: internal/component/config/yang/publish_test.go -- TestPublishMakesDefaultLoaderAnswerTheJoinedSet, TestPublishSameModulesTwiceChangesNothing, TestPublishOtherModulesIsRefusedNamingTheRestart, TestPublishRefusedModulePublishesNothing, TestPublishNoModuleResolvesNothing, TestPublishRefusesAfterAKeeperReadTheSet, TestPublishRebuildsCommandArgDefs -->
<!-- source: internal/component/config/publish_schema_test.go -- TestPublishedSetReachesSchemaCaches, TestPublishWithNoExternalModuleKeepsTheSharedSet -->
<!-- source: internal/component/cli/client/main.go -- loadYANGState, yangStateMemo -->
<!-- source: internal/component/cli/testing/headless.go -- headlessCommandTree, headlessTreeMemo -->
<!-- source: internal/component/cli/completer_validate.go -- Completer.valueSchema -->
<!-- source: internal/component/plugin/server/server.go -- NewServer, moduleSetKeeperName -->
<!-- source: internal/component/gnmi/server.go -- NewServer, moduleSetKeeperName -->
<!-- source: internal/component/iface/validate.go -- ValidateIfaceName, reservedIfaceNamesKeeper -->
<!-- source: cmd/ze/hub/command_meta.go -- commandMetaSource -->

### Who publishes: boot, and a command with no daemon

Only the daemon publishes the module of an external plugin. Owner ruling
(Thomas, 2026-11-16): "plugin command requiring external plugins which can not
be started from the code when no daemon is running to pass it to are not
available, for command built-in then yes you should run them as we can look
them up in the yang hardcoded in the binary". With no daemon running, Ze starts
no external plugin's program, so it holds no schema of one, and what needs that
schema is not available. A compiled-in plugin is unchanged: its YANG is in the
binary.

| Process | The module set it parses against | Caller |
|---------|----------------------------------|--------|
| The daemon, at `ze start` | The compiled-in set joined to the module of each declared external plugin, read from its live Stage 1 declaration | `joinDeclaredPlugins` in `cmd/ze/hub` |
| A command that parses a config with no daemon | The compiled-in set alone. No program is started | `config.SchemaWithDeclaredPlugins`, and `config.PublishDeclaredSchemas` for a caller that parses through `ReadConfig` or `ParseTreeForValidation` |

`config.SchemaWithDeclaredPlugins` is the one step every such command calls in
place of `config.YANGSchema`, before it parses. It takes the first pass of each
config it is given, keeps the names of the external plugins they declare, fixes
the process's module set to the compiled-in one, and answers the schema. It
starts no program in any process.

| Case | What the step does |
|------|--------------------|
| No config declares an external plugin | The schema is the compiled-in one, and the process's set is recorded as holding no external module |
| A config declares an external plugin and holds no name only its schema could define | The config parses. Nothing is reported about the plugin |
| A config declares an external plugin and the parse refuses a name the schema does not define (`config.RefusesUnknownNode`: an unknown top-level keyword or an unknown field) | `DeclaredSchemas.ParseRefusal`, `Explain` and `Unchecked` answer one operator error: the refusal, the external plugins the config declares, that Ze reads such a plugin's schema when `ze start` runs it and that this command starts no plugin, so only a running Ze that started with these plugins checks such a section. A command that asked no running Ze says so, and its fix is `ze config validate <file>`, which asks one. `ze config validate` itself, which did ask (`DeclaredSchemas.Unanswered`), says why none answered, and its fix is the command for that reason. The message never says a plugin owns the refused name: Ze holds no schema that could tell |
| Any other parse refusal | Unchanged: no plugin's schema explains a value out of range or a syntax mistake |
| A second call in one process | Answers the same as the first |
| The process's module set is already fixed (`yang.ModuleSetFixed`: it published, or it built a value that keeps the set, as a running daemon does) | Publishes nothing. A declared plugin whose module the process holds is not listed, and its section parses. A declared plugin the process holds no module of is named beside the refusal of an unknown name, with the restart of `ze` as the fix, unless the daemon started it with that same declaration and it declared no module: boot records what each started plugin declared (`config.RecordStartedPlugins`, from `startedPlugins` in `cmd/ze/hub`), and a plugin that defines no section explains no refused name. A plugin that gave no declaration at boot, one the daemon did not start, and one declared with another `run` line or leaf are still named |
| A config the first pass refuses | Declares nothing to the step. The caller's full parse refuses the same block |

### `ze config validate` asks the running Ze first

A running Ze holds the schema of the external plugins it started with, so it
can judge a section this process cannot. `ze config validate <file>` therefore
asks it first, for a config that declares an external plugin this process
holds no schema of (`judgeConfig`). It takes the walk the local `ze doctor`
takes (`sshclient.ReachDaemon`: the daemon's address, a probe of its listener,
the stored credential with no prompt) and sends
`show config supplied validation content <base64> name <label>`: the file's
bytes, not its path, and the file's base name as the label the daemon's own
messages use. The label is left out when the daemon would refuse it (standard
input, a name with a control character, a name that starts with a dash), and
the daemon then uses `supplied.conf`. The daemon reads a file a caller names only from its own config
directories (`command.CallerPath`), never from the operator's working
directory, so a path would be refused for the usual case. The daemon's verdict
is drawn by the local renderer, so the text and the exit codes (0 valid, 1
invalid, 2 for a file that cannot be read) are those of a local verdict.

| Config | Ze running | Ze not running |
|--------|------------|----------------|
| Declares an external plugin | Judged in full by the running Ze | Judged here; a section under the plugin's root is refused, and the message says why no Ze answered |
| Declares no external plugin | Judged here; no Ze is asked | Judged here |

A config that declares no external plugin is not sent: every schema it needs
is compiled into the binary, so the local verdict is complete, and it does not
depend on which binary the daemon runs or on whether it can be reached. A
process that holds a daemon's module set never asks (`DeclaredSchemas.Offline`
is false there), and the daemon serves the command through
`suppliedValidation`, which judges the bytes with the producer of
`show config validation` (`validationBytesAnswer`) and asks nobody.

A failed ask is never read as a verdict. The answer counts only when it is a
validation result this binary reads (`diagnostic.DecodeValidateResult`) and
its verdict agrees with the status beside it. Otherwise the config is judged
in this process, and the refusal of an unknown name carries the reason and one
fix:

| Why no running Ze answered | Fix |
|----------------------------|-----|
| Nothing listens at Ze's SSH address | `ze start`, then the same command |
| A Ze listens, but this account has no stored credentials, or the login or the ask failed, or the answer is not a validation result | `ze config validate --user <name> <file>` |
| Ze's SSH address could not be read | `ze status`, then the same command |
| The config is larger than one command carries (195,840 bytes, the bound of the `content` leaf) and lies outside the daemon's config directories | `ze config validate <config-dir>/<name>`, after copying the file there |

A config over the bound that already lies inside the daemon's config
directories is sent by path instead, as `show config validation file <path>`.

The noninteractive completion command uses `completerForBytes` for both its
file/stdin adapter and `show config supplied completion`. Schema selection
reads declarations from the supplied configuration and uses modules already
available to this process; it never starts a declared program. The supplied
operation carries the configuration and literal editing input in one bounded,
masked JSON envelope. Context and ghost selection are modeled arguments. Only
the standalone shell adapter interprets `+` in `--input` as a space.
<!-- source: internal/component/config/cli/cmd_completion.go -- completerForBytes -->
<!-- source: internal/component/config/cli/config_supplied_utilities.go -- suppliedCompletion -->

### `ze config fmt`, `dump`, `diff` and `graph` take the same route

The four commands that read a config and answer about it share the route
(`suppliedQuestion` and `routedAnswer` in `internal/component/config/cli`
`daemon_route.go` and `inspect_route.go`), on the operator's own route only:
`ze config <sub>` and the local `ze show config <sub>`. The daemon serves the
same handlers on its route and asks nobody.

| Command | Line sent | How the answer is drawn |
|---------|-----------|-------------------------|
| `fmt` | `show config supplied fmt content <base64> name <label>`, with no `check` or `diff` leaf | The formatted text comes back. `--check`, `--diff` and `-w` are made from it by `fmtFormattedAnswer`, the step a local format takes, and `-w` writes the operator's file in this process |
| `graph` | `show config supplied graph content <base64> name <label>` | The JSON text is printed |
| `dump` | `show config supplied dump content <base64> name <label> [strip-private]` | The document is the answer of `ze show config dump`; `ze config dump` prints its `bgp` block and its plugin list (`daemonDump`) |
| `diff` | `show config supplied diff content <base64>`, the content a JSON object with `before` and `after` | The three sets are drawn by the local printer (`daemonDiff`). Both sides are read here in every form the command has: two files, a revision and a file, a backup artifact, standard input |

`fmt` sends no `check` or `diff` leaf because a check that found changes exits
1 with a line of text, which the daemon route carries as a failed status, and
a failed status must stay readable as the daemon's refusal.

A refusal the daemon states is drawn, after `error: `, only when this process
refuses the config too. The daemon holds every schema this binary compiles in,
so its refusal of a config this process answers about is not about the config
(a denied account, a daemon that does not serve the command), and the local
answer stands. The table of failed asks above applies, with the command the
operator ran in place of `ze config validate`; each of the four declares
`--user <name>`. A producer of these commands builds its own
`DeclaredSchemas`, so the route records the failed ask for the process
(`config.RecordDaemonUnanswered`) before it answers locally.

Content over the bound goes by path to `show config fmt`, `dump`, `graph` or
`diff` when every file lies inside the daemon's config directories. A
revision, a backup artifact and standard input name no file the daemon could
open, so a comparison over the bound that uses one is not sent.

An external module that adds a node under `plugin` or `environment` is refused
at the join (`refuseFirstPassAddition`, `yang.FirstPassRoots`): Ze reads those
two roots before it starts a plugin, strictly and from the compiled-in schema,
so no config could hold the added name.
`--user <name>` logs in as that account,
with the password from `ze.ssh.password` or a prompt.

Through the daemon, as with no daemon, validation is the parser, the YANG
checks and each registration's in-process verifier: a plugin's live
`OnConfigVerify` is not run.

`show config validation <file>` typed in a `ze cli` session, or sent over SSH,
the web, MCP, REST or gRPC, reaches the daemon's handler too, and reads the
file on the daemon's host from its config directories; those callers send a
config held elsewhere with `show config supplied validation content <base64>`
(`docs/architecture/api/commands.md`, "Caller-supplied configuration
inspection"). `ze cli -c "show config validation <file>"` answers in its own
process (`command.ServeLocal`) and asks no daemon.
<!-- source: internal/component/config/cli/validate_route.go -- judgeConfig, validationQuestion, askDaemonValidation, decodeDaemonValidation -->
<!-- source: internal/component/config/cli/daemon_route.go -- asksRunningDaemon, suppliedQuestion.line, sentLabel, askOverSSH, unreachedOrAsked -->
<!-- source: internal/component/cli/sshclient/reach.go -- ReachDaemon -->
<!-- source: internal/component/config/declared_schemas.go -- DaemonUnanswered, Unanswered -->
<!-- source: internal/component/config/cli/config_supplied.go -- suppliedHandler, suppliedValidation -->
<!-- source: internal/component/config/cli/config_data.go -- dataValidate, validationBytesAnswer -->
<!-- test: internal/component/config/cli/validate_route_test.go TestValidateRouteDrawsTheDaemonsAcceptance -->
<!-- test: internal/component/config/cli/validate_route_test.go TestValidateRouteFallsBackWhenNoDaemonListens -->
<!-- test: internal/component/config/cli/validate_route_test.go TestValidateRouteDoesNotAskForCompiledInConfig -->

A comparison passes each successfully read input, including stored revision
bytes, before either full parse, so the plugins of both sides are remembered.
File and supplied-content dump and graph readers use the same step and retain
its explanation when a plugin's schema is unavailable.
<!-- source: internal/component/config/cli/cmd_diff.go -- compareDiffInputs, readDiffSource -->
<!-- source: internal/component/config/cli/config_supplied.go -- suppliedDiff -->
<!-- source: internal/component/config/cli/cmd_dump.go -- resolveDumpBytes -->
<!-- source: internal/component/config/cli/cmd_graph.go -- graphBytesAnswer, declaredParseExplanation -->

The callers are the `ze config` commands (`validate`, `fmt`,
`dump`, `diff`, `migrate`, `completion`), `show config graph`, the editor,
`ze doctor`, the support report, the crash-capture readiness read, the TACACS+
probe, `show plugin declarations config` and `ze plugin test`.

One command still runs a program with no daemon, because running the program
the operator names on its own command line is its purpose: `ze show plugin
conformance run "<command>"`. `show plugin declarations config <file>` follows
the rule of this section: with no daemon it starts no plugin, and the row of an
external plugin reads `not-available` with the two commands that read its
declaration. It asks this step which process it is in
(`DeclaredSchemas.Offline`, through `readConfiguredPlugins`), so the parse and
the command decide "no daemon" in one place. A running daemon answers the same
command by running each external plugin of the file in declare mode
(`plugin.QueryDeclaration`).
<!-- source: internal/component/config/declared_schemas.go -- SchemaWithDeclaredPlugins, PublishDeclaredSchemas, DeclaredSchemas, publishDeclaredSchemas -->
<!-- source: internal/component/config/yang/publish.go -- ModuleSetFixed -->
<!-- source: internal/component/config/declared_schemas.go -- RecordStartedPlugins, StartedPlugin, SchemaOutcome, startedWithoutSchemaLocked -->
<!-- source: cmd/ze/hub/declared_plugins.go -- startedPlugins, schemaOutcome -->
<!-- source: internal/component/config/declared_schemas_test.go -- TestDeclaredSchemasDoesNotNameAStartedPluginThatDeclaredNoModule, TestDeclaredSchemasNamesThePluginsThatCouldExplain, TestDeclaredSchemasWithoutARecordNamesEveryPlugin -->
<!-- source: internal/component/command/local_data.go -- ServeLocal -->
<!-- source: internal/component/plugin/declarations.go -- QueryDeclaration, configuredDeclarationRow, unavailableDeclarationRow -->
<!-- source: internal/component/config/register_plugin_declarations.go -- readConfiguredPlugins -->
<!-- source: internal/component/config/declared_schemas_test.go -- TestDeclaredSchemasStartsNoDeclaredProgram, TestParseRefusalSaysTheSectionIsNotAvailableOffline, TestDeclaredSchemasNoExternalPluginRunsNothing, TestDeclaredSchemasRunsNothingOnceTheSetIsFixed, TestDeclaredSchemasAnswersFromThePublishedSet, TestDeclaredSchemasTakesPluginsOfEverySource -->
<!-- source: internal/component/config/cli/cmd_validate.go -- runValidation -->
<!-- source: internal/component/config/cli/cmd_diff.go -- readDiffSource, parseAndResolve -->

### Decided, not built: modules compiled at build time

Everything above describes the code today. The owner has decided where module
loading goes next; it is designed in
`plan/post-release/engineering/spec-config-yang-compiled-at-build-time.md` and
not built.

| Date | Decision (Thomas) | The decided design |
|------|-------------------|--------------------|
| 2026-10-16 | "compile [the YANG] at build time, use go:generate script and import" | A generator compiles each module and its checks run at build time, so a bad module fails the build and no compiled-in module is parsed or resolved at run time. The compiler stays in `ze` for the modules of external plugins |
| 2026-11-18 | "Per module." | Each module's compiled form is a generated Go file committed beside its `.yang`, independent of the build tags; its augments are graft records linked when its package initializes |
| 2026-11-18 | "No leaf ref must work cross module." | A leafref may name a node of any module the module where its path is written imports (RFC 7950 Section 6.4.1), compiled-in or external. Such a path never leaves the module's import closure, which every binary links with it, so it is checked once when the module compiles and never refused for crossing a module |
| 2026-11-18 | "Yes, I keep the order of the definition." | Decision D-1 of 2026-10-28 holds for the compiled form: a block's own children in source order, then augmented children by module name, then augment order, whatever order the packages initialize in |
| 2026-11-18 | "Okay, do what you need." (on the hand-over below) | The module-text hand-over of `--plugin` (`CollectPluginYANG`, `loadYANGModules`, "Joining a module that no `init()` registered") is deleted with the compiled form, because a compiled registration's module is always registered by its own package. The `--plugin` flag keeps starting the plugin it names |

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

Each built-in type checks its value's lexical form (RFC 7950 Section 9):

| Type | Value accepted |
|------|----------------|
| integers, `decimal64`, `boolean`, `string`, `enumeration`, `union` | the Section 9 lexical form, then the type's range, length, pattern or enum set |
| `binary` | base64 with padding (RFC 4648 Section 4), no whitespace; `length` counts the decoded octets |
| `bits` | the names of the set bits separated by one or more space characters, each a defined bit and none twice; the empty string sets no bit |
| `empty` | only the presence value the config tree stores (`true`); the type has no lexical form of its own |
| `leafref` | a value of the type of the leaf the path names (Section 9.9.4); with require-instance true (the default), a value equal to an instance of that leaf in the whole config, or to its default where the default is in use, compared in canonical form: an integer by number (`+17` and `017` equal `17`), a decimal64 by its Section 9.3.2 form (`1.50` equals `1.5`); a path predicate compares a list key with the `current()` value in the key leaf's canonical form too |
| `identityref` | the name of an identity derived from the base, written `name`, `prefix:name` or `module:name` with the defining module's prefix or name; the base itself is refused, and an unprefixed name two modules define is refused naming the qualified forms. Config text carries no XML namespace, so the prefix is the module's, never a local declaration |
| `instance-identifier` | the Section 14 rule, every node name prefixed (9.13.2) with its module's prefix or name, each step a data node, one quoted predicate per key of each keyed list, `[.='v']` on a leaf-list, `[n]` on a keyless list only; with require-instance true, the node must exist in the config or be a leaf whose default is in use, every key and `[.='v']` value compared with the configured or default value in the canonical form of its leaf's type, as for `leafref` |
| any kind goyang adds, and an unresolved type | nothing: the value is refused, never accepted unchecked |

The instance checks of `leafref` and `instance-identifier` read the whole
config at once, because a reference may cross sections:
`Validator.ValidateReferences` runs over `Tree.ToMap()` inside
`config.ValidateCustomSections`, which `ze config validate`, the daemon's load
and the editor's commit share. A union value takes the instance rule of the
first member type that accepts it (Section 9.12). The traffic-control
`default-class` leaf is the first Ze leaf of these types: a leafref to
`../class/name`.

The parsers and the `set` path judge a leaf before the tree is walked, through
`ValidateLeafValue`, and a list key through `ValidateListKey`. There a union
leaf carries its member types, each with its own range, length, pattern and
enum set, and a value is accepted at the first member that accepts it
(Section 9.12). A union list key, such as the prefix of a `prefix-list` entry,
is therefore refused at parse when no member accepts it: the refusal lists the
values the enumeration members accept and names every other member type in
words. A ranged `decimal64` leaf is compared as a decimal of its declared
`fraction-digits` (Section 9.3), so an in-range value is accepted and an
out-of-range one gets the range refusal with the decimal bounds, while a value
that is not a decimal of that form keeps its type refusal.

<!-- source: internal/component/config/leaf_value.go -- ValidateLeafValue, validateUnionValue, ValidateListKey, validateDecimal64Leaf -->
<!-- source: internal/component/config/yang_schema.go -- unionMembersFromType -->
<!-- source: internal/component/config/yang/validator.go -- ValidateTree, walkTree, validateLeafValue -->
<!-- source: internal/component/config/yang/validator_lexical.go -- validateEmpty, validateBinary, validateBits, uncheckedKindError -->
<!-- source: internal/component/config/yang/validator_reference.go -- validateLeafref, validateIdentityref, validateInstanceIdentifier -->
<!-- source: internal/component/config/yang/validator_instance.go -- ValidateReferences, checkLeafrefInstance, checkInstanceIdentifierInstance, canonicalValue, sameValue, canonicalKey -->
<!-- source: internal/component/config/validate_sections.go -- ValidateCustomSections, referenceFailures -->

#### Constraint error messages

A `range`, `length` or `pattern` restriction may carry an `error-message`
substatement, and Ze speaks it when that restriction refuses a value.

RFC 7950 Section 7.5.4.1: "If the constraint evaluates to "false", the string is passed as <error-message> in the <rpc-error> in NETCONF."

On every path (the tree walk, the parsers, the `set` path and the command
argument refusal) the message is the why line of the value refusal ("Value
refusals" below), in place of the generic `<leaf> accepts ...` line; the
problem line and the fix line stay as they are. The tree walk's `Message`
field also keeps the older text, which `speakConstraintMessage` writes:

| Restriction | The tree walk's `Message` |
|-------------|---------------------------|
| `range` | the out-of-range text, then `: ` and the message |
| `length` | the length text, then `: ` and the message |
| `pattern` | `invalid value "<value>": ` and the message, with no regular expression |

A restriction with no `error-message` keeps its generic why line. goyang parses the
statement but drops it from the resolved type, so
`LeafConstraintMessages` reads it from the type statements: the leaf's own
type, a typedef it derives from, or a union member. Along a typedef chain a
range or length takes the message of the nearest statement that declares one,
because a derived restriction replaces its base's, and each pattern keeps the
message of its own statement, because patterns add up.

All three validators speak it:

| Path | Where the message is read |
|------|---------------------------|
| The tree walk (`ze config validate`, the daemon load, the editor commit) | `validateLeafValue` in `config/yang/validator.go` |
| The parsers and the `set` path, a single leaf and each union member | `LeafNode.Constraint`, filled by `yangToLeaf` and `unionMembersFromType` |
| The parsers and the `set` path, each member of a leaf-list, a multi-value leaf or a bracket leaf-list | the node's `Constraint` (`ValueOrArrayNode`, `MultiLeafNode`, `BracketLeafListNode`), filled in `yangToNodeOfAnyOS`, read through the node's `memberLeaf` by `ValidateLeafValue` |
| Command arguments, through the RPC validator | `ArgOptions.RangeMessage`, `LengthMessage` and `PatternMessages`, filled by `yangTypeToArgDef` |

The restriction and its message travel together. `yangValueLeaf` builds native
type facts once for scalar leaves and all three multi-valued node forms:
numeric ranges, string lengths and patterns, enumeration values, union members,
and a decimal64's fraction digits. `memberLeaf` carries those facts unchanged
into `ValidateLeafValue`, so hierarchical, set and annotated-set parsing and the
editor's value guard refuse the same member before commit. A string length
counts Unicode characters; a decimal range keeps decimal arithmetic and the
declared precision. For example, the shipped
`control-plane-protection bgp protected-port` leaf-list refuses port `0` at
these boundaries while accepting `1` through `65535`.
<!-- source: internal/component/config/yang_schema_leaflist.go -- yangValueLeaf, yangToMultiLeaf, yangToBracketLeafList, yangToValueOrArray -->
<!-- source: internal/component/config/leaf_value.go -- memberLeaf, ValidateNodeValue, ValidateLeafValue -->
<!-- test: internal/component/config/leaflist_native_restrictions_test.go TestYANGLeafListNativeRange -->
<!-- test: internal/component/config/leaflist_native_restrictions_test.go TestYANGLeafListNativeLength -->
<!-- test: internal/component/config/leaflist_native_restrictions_test.go TestYANGLeafListNativeUnion -->
<!-- test: internal/component/config/leaflist_native_restrictions_test.go TestYANGLeafListNativeDecimal -->
<!-- test: internal/component/config/leaflist_native_restrictions_test.go TestCoppProtectedPortNativeRange -->

One input gets one guard. The constraint is the first check on every path, so
its `error-message` is where the operator's message goes. A `ze:validate`
behind it that re-checks the same shape is either never reached, because the
constraint refused first, or speaks a second refusal for the same value. The
IS-IS `system-id` pattern and `zt:vlan-id` are the worked examples.

<!-- source: internal/component/config/yang/constraint_message.go -- ConstraintMessages, LeafConstraintMessages, typeConstraintMessages, WithConstraintMessage -->
<!-- source: internal/component/config/yang/validator.go -- validateLeafValue, speakConstraintMessage -->
<!-- source: internal/component/config/schema.go -- LeafNode.Constraint, MultiLeafNode.Constraint, BracketLeafListNode.Constraint, ValueOrArrayNode.Constraint -->
<!-- source: internal/component/config/leaf_value.go -- validateLeafPatterns, validateLengths, MultiLeafNode.memberLeaf -->
<!-- source: internal/component/config/yang_schema.go -- yangToNodeOfAnyOS, yangToLeaf, unionMembersFromType -->
<!-- test: internal/component/config/leaflist_pattern_message_test.go TestLeafListPatternRefusalSpeaksTheMessage -->
<!-- source: internal/component/config/yang/refusal.go -- attachRefusal, refusalFacts.why -->
<!-- source: internal/component/command/argvalue.go -- rangeFault, lengthFault, patternFault -->

#### Value refusals

A value outside an enumeration, a range, a length or a pattern is refused with
one operator error (`ai/rules/user-facing-errors.md`), built from the schema
alone. `configyang.ValidationError` holds it: `Error()` renders it, and
`Unwrap` answers the `*opererr.Error`, so `errors.As` finds both the typed
facts (`Type`, `Expected`, `Got`, the JSON diagnostic's fields) and the
problem, why and fix a web notice reads. Every other kind (missing, type,
reference, cardinality, unknown) holds none and keeps
`<type> error at <path>: <message>`.

| Part | Enum | Range | Length | Pattern |
|------|------|-------|--------|---------|
| problem | `<path>: "<value>" is not an accepted value`, ` (did you mean X?)` when one value is close | `... is outside the accepted range` | `... is outside the accepted length (N characters)` | `... is not in the accepted format` |
| why | `<leaf> accepts only these values:`, then one line per value in declared order with its `ze:help` | `<leaf> accepts a value in <bounds>` | `<leaf> accepts <bounds> characters` | `<leaf> accepts <format>` |
| fix | `set <path> <suggestion>`, else the alternation `<a\|b\|c>` | `set <path> <bounds>` | `set <path> <bounds characters>` | `set <path> <typedef-name>` or `<value>` |

This section is the one description of the refusal; the operator guides and
the other architecture pages link here rather than restate it. What
`ze config validate` prints for a mistyped enum value in a hierarchical file
(`test/parse/enum-refusal-proposes-valid-values.ci`):

```text
line 18: bgp peer peer1 session capability route-refresh: "requre" is not an accepted value (did you mean require?)
route-refresh accepts only these values:
  enable   Advertise the capability; no enforcement
  disable  Do not advertise the capability; no enforcement
  require  Advertise the capability; refuse the session when the peer does not offer it
  refuse   Do not advertise the capability; refuse the session when the peer offers it
fix: route-refresh require;
```

A value close to no accepted value (`banana`) gets no hint, and its fix is the
alternation: `fix: route-refresh <enable|disable|require|refuse>;`. Close
means `suggest.Correction`: within one edit for a value of up to four
characters, within a third of its length beyond. A range, a length and a
pattern read the same way (`test/parse/range-length-pattern-refusal.ci`):

```text
line 11: ospf interfaces interface eth0 cost: "0" is outside the accepted range
cost accepts a value in 1..65535
fix: cost <1..65535>;
```

```text
line 3: l2tp shaper default-rate: "fast" is not in the accepted format
default-rate accepts Traffic rate string (for example 10mbit, 100kbit, 1gbps)
fix: default-rate <rate>;
```

A union lists the values of its enumeration members, then names its other
member types, and the alternation names them too
(`test/parse/enum-refusal-union.ci`):

```text
line 9: bgp peer peer1 connection local ip: "autoo" is not an accepted value (did you mean auto?)
ip accepts one of these values:
  auto  Let the operating system choose the source address of the connection.
or a value of type ip-address
fix: ip auto;
```

The problem and the why are the same on every surface. Only the head of the
first line and the fix line change, each framed for where the operator types
the correction:

| Surface | Head of the first line | Fix line |
|---------|------------------------|----------|
| `ze config validate` of a hierarchical file | `line N: ` | the file line, `route-refresh require;`; a refused list key `redistribute <connected\|static\|kernel\|bgp\|isis> { ... }` |
| `ze config validate` of a set-format file | `line N: ` | the line with the value replaced, `set ospf interfaces interface eth0 cost <1..65535>` |
| `ze start` and a reload of a file | `error: load config: parse config: line N: `, `reload error: reload: parse config: line N: ` | the file line |
| SSH editor and web terminal `set`, `insert`, `rename`, `copy`; commit, `commit verify`, `commit confirmed` | none | `FrameFix`: the command as typed, corrected, at the position it was typed at; `set <full path> ...` from the root; `top set <full path> ...` inside an `edit` context |
| Web forms, the add form, the commit modal | none | `set <full path> ...` from the root |
| REST, gRPC and gNMI commit (`ValidateContent`) | `config validation failed:`, then each refusal as a block indented two spaces; gNMI answers `Aborted` with `commit: ` before it | each block's own `set <full path> ...` from the root |
| gNMI `Set` update whose list key the session refuses | none; the status is `InvalidArgument` and the message is the refusal whole | the refusal's own fix line, unchanged |
| Operational command argument | the command path and the argument | the whole command restated ("YANG-Typed Command Arguments" in `docs/architecture/api/commands.md`) |

Validation collects independent value refusals before it returns, so a session
commit names each invalid setting in its own block. The hierarchical parser
continues only at a scalar statement's terminator. The set parsers continue
at the next complete line. Structural errors stop collection, and any error
returns no usable config tree. Daemon parsing still stops at its first error.
<!-- source: internal/component/config/parser_validation.go -- ParseDiagnostics, collectValueRefusal, ParseSetDiagnostics, collectLineRefusal -->
<!-- source: internal/component/config/cli/cmd_validate.go -- runValidation, ValidateContent -->

`show config validation <file> | json`, the JSON form of `ze config validate`,
carries the refusal as one diagnostic: `code` names the kind
(`config-yang-enum`, `config-yang-range`, `config-yang-length`,
`config-yang-pattern`), `message` is the refusal without its `line N:`, `line`
holds the line, `path` the walk's slash path, `expected` the accepted values
or bounds, `actual` the refused value (absent on a secret leaf), and `fix`
the fix line alone, with no label. A parser refusal and a tree walk refusal of
the same leaf carry the same fields.
<!-- source: internal/component/config/cli/cmd_validate.go -- refusalDiagnostic, refusalFix, walkFormPath, yangErrorCode, ValidateContent, sessionFixRefusal -->
<!-- source: internal/component/cli/fix_frame.go -- FrameFix, FramedSetRefusal -->
<!-- source: internal/component/gnmi/set.go -- applyUpdate, updateRefusal -->

The editor checks a new ancestor list key before a `set`, `insert`, `activate`
or `deactivate` can change the candidate. A refused ancestor leaves its parent
containers untouched, and its fix replaces that key in the complete command.
Section walk refusals carry the same typed fix as parser refusals, so a commit
inside an edit context writes `top set`. A missing `ze:required` field keeps
its path as words, including every ancestor list key, for the same framing.
A key with several leaves expands to its individual address words. A single
key holding spaces stays one quoted argument in the printed set hint and
editor fix. Command quoting protects a key equal to `|` as a literal argument
in both forms. These paths retain the stored key's bytes, including a literal
`#3` ending, so a hint addresses the same entry that needs the field.
<!-- source: internal/component/cli/editor_commands.go -- walkOrCreate, InsertLeafListValue, ActivateLeafListValue, DeactivateLeafListValue -->
<!-- source: internal/component/cli/validator.go -- sectionErrors, validateWithYANG -->
<!-- source: internal/component/config/required.go -- RequiredViolation, walkRequired, checkListRequired -->

Direct API edits can retain an invalid leaf value or leaf-list member until
commit validates the candidate. Session write-through stores that value as
candidate data, and restoring its change file defers the same leaf constraints.
This lets the next edit correct a refused value, including after reopening the
editor, without discarding other pending changes. Schema paths, list keys,
token counts and structural syntax remain checked on restoration. File loading and ordinary
set parsing still validate leaf values; restoring a candidate does not make it
eligible for commit.
<!-- source: internal/component/config/change_file.go -- ParseCandidateChangeFile -->
<!-- source: internal/component/cli/editor_draft.go -- readChangeFile, writeThroughSet, SaveDraft -->

The path is the setting path as an operator types it, words separated by
spaces, list keys included. A declared `error-message` is the why line in
place of the generic one. An enum leaf written with no value says
`<path>: needs a value`. On a `ze:sensitive` or `ze:bcrypt` leaf the value slot
holds `/* SECRET-DATA */`, and neither a suggestion nor a length is computed.

The pattern's format comes from one reader, `LeafValueFormat`: the nearest
typedef's `ze:help`, else the first sentence of its `description`, else the
leaf's own `ze:help`; with none of them the why names the typedef
(`a value of type <name>`), and an inline pattern on a leaf with no help says
`<leaf> does not accept this value`. The regular expression is never shown.

The tree walk attaches the refusal in `validateLeafValue`, where the leaf is
known; the parsers build it with `EnumRefusal`, `MissingEnumValue`,
`RangeRefusal`, `LengthRefusal` and `PatternRefusal`. A surface that frames
its own fix (the file form `route-refresh require;`, a command typed inside an
`edit` context) calls `WithFix` with the line built from `Correction()`, and
the parser names the full path once with `WithPath`; each answers a copy whose
other facts are the original's. The value lines and the alternation are
written by `suggest.ValueLines` and `suggest.Alternation`, which the command
argument refusal shares. The schema nodes keep the facts the parsers build
from: `LeafNode.Enums` in declared order with `EnumHelp`, `Format`, and the
same fields on `MultiLeafNode`, `BracketLeafListNode` and `ValueOrArrayNode`.

`Correction()` carries the replacement value or placeholder as data.
`CorrectionIsPlaceholder()` distinguishes those cases at command boundaries;
`CorrectionText()` writes concrete replacements with the shared byte-preserving
token quoter and leaves declared placeholders raw. An enum value named
`in progress` is therefore corrected as `"in progress"`, while
`<1..4096 characters>` remains a placeholder. Hierarchical statements, set-file
lines and the shared YANG default fix all use this distinction.
<!-- source: internal/component/config/yang/refusal.go -- correctionToken, CorrectionText, CorrectionIsPlaceholder -->
<!-- source: internal/core/configtext/token.go -- QuoteToken, QuoteCommandWord -->

<!-- source: internal/component/config/yang/refusal.go -- RefusedValue, EnumRefusal, MissingEnumValue, RangeRefusal, LengthRefusal, PatternRefusal, WithFix, WithPath, Correction, attachRefusal, pathWords -->
<!-- source: internal/component/config/yang/validator_error.go -- ValidationError, Error, Unwrap -->
<!-- source: internal/component/config/yang/format.go -- ValueFormat, LeafValueFormat, firstSentence -->
<!-- source: internal/component/config/yang/enum.go -- EnumNamesDeclaredOf, EnumValuesOf -->
<!-- source: internal/component/config/yang/sensitive.go -- SecretValuePlaceholder, EntryHoldsSecret -->
<!-- source: internal/core/suggest/valuelist.go -- Value, ValueLines, Alternation -->
<!-- source: internal/component/config/yang_schema.go -- enumFactsFromEntry, yangToLeaf, unionMembersFromType -->
<!-- test: internal/component/config/yang/refusal_test.go TestValidationErrorOperatorError -->

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

A suggestion may come from another plugin than the leaf's. The AS-SET leaves of
`bgp-filter-irr` and `firewall-irr` carry `ze:validate "irr-as-set"`, which the
irr owner declares, because the owner is the one plugin that knows every AS-SET
in play. Its values are read from the daemon's store, where the editor runs, so
they answer the same when the owner runs forked (`docs/architecture/irr-owner.md`,
"Completion").

<!-- source: internal/component/config/yang/validator_registry.go -- RegisterCompleteFn, RegisterSuggestion, MergeGlobalCompletions -->
<!-- source: internal/component/config/yang/validator.go -- applyCustomValidators -->

### Registered Validators

Twenty-five validators are registered. The table below is a SAMPLE that shows
the two kinds, and it is not the list: `reg.Register` in
`validators_register.go` is the only place that holds all of them, and a reader
who needs the set reads that file rather than this page.

| Name | Validates | Provides Completion |
|------|-----------|-------------------|
| `registered-address-family` | Value is a plugin-registered AFI/SAFI | Yes -- queries `registry.FamilyMap()` |
| `registered-protocol` | Value is a protocol some component registered | Yes -- queries `redistevents.ProtocolNames()` |
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

The walk has three callers, so they reach one verdict on the same bytes:
`LoadConfig` at daemon start and reload, `ze config validate`, and the SSH
config editor's `commit` and `commit verify` through `sectionErrors`. The editor
runs it over a clone pruned of inactive nodes, as `LoadConfig` prunes before it
walks, and reports every failure as an error that blocks the commit, bgp global
failures excepted because its own bgp pass reports those already.

<!-- source: internal/component/cli/validator.go -- sectionErrors -->

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

A section declares a rule only through an annotation that names a validator
with a `ValidateFn`, or one nobody registered (`annotationRefuses`). A
completion-only annotation (Completion-Only Validators above) refuses nothing,
so the walk has nothing to run for it and its section owes neither a walk nor
an excuse. The `firewall` section is that case: its only annotations are the
irr owner's `irr-as-set` suggestion on the firewall-irr AS-SET leaves.

Read the resolved model, never the YANG source: a `ze:validate` written inside a
grouping lands wherever that grouping is used, so the source cannot say which
section owns it. The derivation is also only as complete as the modules the
binary linked, and every plugin sits behind a feature build tag, which is why
the test asserts its recorded exclusions are present before it reads an empty
answer as good news.

<!-- source: internal/component/config/validate_sections.go -- ValidatorSectionCoverage, knownUnwalkedValidatorSections, annotationRefuses -->

Components can also register an original-tree validator with
`RegisterTreeValidator`. `ValidateCustomSections` calls these on the original
tree after inactive subtrees are pruned and before inheritance. Candidate parsing
does not run them: successive `set` commands may build a policy before adding
its final, but validation, commit and daemon load refuse that incomplete policy.
BGP registers its direction policy check here: every explicitly present
import/export container requires its
own `final accept` or `final reject`, including a global layer with no peers.
The peer builder repeats the check for callers that supply a tree directly.
The schema still declares the presence containers and mandatory final enumeration;
the registered boundary supplies the original path and corrective syntax.
`LoadConfig` runs this gate before publishing the candidate's environment.
The registered value validators use supplied values, schema constraints and
registries; plugin completion registrations do not add validation functions.
<!-- source: internal/component/config/tree_validators.go -- RegisterTreeValidator, validateOriginalTree -->
<!-- source: internal/component/config/validate_sections.go -- ValidateCustomSections -->
<!-- source: internal/component/config/loader.go -- LoadConfig -->
<!-- source: internal/component/bgp/config/register.go -- init -->
<!-- source: internal/component/bgp/config/redistribution.go -- validateFilterPolicies -->

The mandatory check (RFC 7950 Section 7.6.5) is not bound by that list.
`mandatoryFailures` walks every other top-level section present in the config,
the set `Validator.Sections` derives from the compiled schema, and keeps only
the missing-mandatory errors, so no custom validator gains reach through it.
`bgpMandatoryFailures` does the same for the whole bgp section, peer and group
entries included. It counts a global value every session inherits as present,
and only a global one: a node missing inside a `peer[...]` or `group[...]`
entry is refused. RFC 7950 Section 7.6.5 makes the list entry that leaf's
closest ancestor, and the entry exists, so the leaf must exist in it.
`ResolveBGPTree` does merge the bgp defaults and the group's fields into each
peer, so a value a group sets reaches its peers at run time, but the check does
not count it: the owner's ruling on inherited values covers the global
router-id and local ASN only. A future mandatory peer-level leaf that only the
group sets is therefore refused, and needs an owner ruling when it lands. A
bgp section with no peer and no group answers as carried for the leaves
`bgpSessionPathOf` names, router-id and the local ASN: the section is accepted
and does nothing, so no session consumes them. The inheritance ruling binds
only once a session exists. Any other global leaf stays required there. A peer `create bgp peer`
builds later is refused by `createdPeerSettings` when it resolves no router-id,
and by the peer parser when it resolves no local AS. An
entry path never counts as carried, because `EverySessionCarries` looks its
`peer[...]` segment up as a session field and no session holds one. A section
absent from the config is not walked, which is the reading libyang's
`LYD_VALIDATE_PRESENT` takes.

<!-- source: internal/component/config/validate_sections.go -- mandatoryFailures -->
<!-- source: internal/component/config/validate_mandatory_bgp.go -- bgpMandatoryFailures, EverySessionCarries -->
<!-- source: internal/component/bgp/config/resolve.go -- ResolveBGPTree -->
<!-- source: internal/component/bgp/reactor/reactor_peers.go -- createdPeerSettings -->
<!-- source: internal/component/config/yang/validator_mandatory.go -- Sections -->

A case of a choice is the closest ancestor of the mandatory nodes it holds,
and RFC 7950 Section 7.6.5 owes them only once any node of that case exists in
the data. A case never appears in the data under its own name: its nodes sit
in the map of the node that holds the choice, while goyang keeps them under
the choice and case entries. `walkTree` therefore finds a data key through
`schemaChild`, which descends through each choice and case, so the data a case
holds is type-checked like any other. `requireMandatoryChildren` checks each
case that has a node present (`requireMandatoryInPresentCases`) and skips the
others. A gre tunnel with no `remote ip` is refused with the path
`interface/tunnel[<name>]/encapsulation/gre/remote/ip` and the `set` command
that adds it. Defaults follow the same case. `definitionOrder` (`yang_schema_order.go`) yields the
data nodes of each case under the node that holds the choice, and the
container or list records the goyang entry of each one, whose parent chain
still names its case and choice. `ApplyDefaults` fills the default of such a
leaf, or creates such a non-presence container, only while
`yang.CaseInEffect` answers that its case is in effect: the case holds data
and no other case does, or no case holds data and it is the choice's default
case (RFC 7950 Sections 7.6.1 and 7.9.3). A presence container or a list that
a case holds is filled only when it already exists, so the case does not
decide for it. The reference walk asks the same predicate.

<!-- source: internal/component/config/yang/validator_mandatory.go -- requireMandatoryChildren, requireMandatoryInPresentCases -->
<!-- source: internal/component/config/schema_defaults.go -- ApplyDefaults, choiceMembers -->
<!-- source: internal/component/config/yang/validator_default.go -- CaseInEffect -->
<!-- source: internal/component/config/yang/reference_path.go -- schemaChild -->
<!-- source: internal/component/config/yang/validator.go -- walkTree -->

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

<!-- source: internal/component/bgp/config/peers.go -- peersAndDynamicGroups, validatePeerProcessCaps -->
<!-- source: internal/component/config/infra/bgp.go -- ValidateBGPPeers -->
<!-- source: internal/component/cli/validator.go -- bgpPeerErrors -->
<!-- source: internal/component/cli/editor_commit.go -- validateStagedTree -->
<!-- source: cmd/ze/hub/editor_adapter.go -- newEditorFactory -->

### Claim Completeness Gate

Validation says a config value is well formed. It does not say the value reaches
anything. Delivery is claimed per path: `Server.reloadConfig` selects the plugins
whose `WantsConfigRoots` match the changed paths, and those hold both the roots a
plugin owns (`ConfigRoots`) and the roots it only reads (`ConfigReads`), so a
read root such as `pki` claims its subtree for every reader. `Hub.RouteCommand` resolves
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
consuming symbol. The entries are `plugin`, `pppoe`, `storage`, `system`,
`telemetry`, and each `environment` subtree that no plugin claims. The hub reads the listener blocks (`web`, `ssh`, `mcp`, and the
rest) through their own extractors, and the config loader pushes the other
blocks into the env layer through `ExtractEnvironment`. An entry without a
reason and an owner is a failure, and so is an entry whose path is now claimed.

At run time `ze doctor` judges one config on one build and reports
`doctor-config-root-unclaimed` for a configured subtree this binary delivers to
nobody. That covers what the build-time gate cannot see: a plugin compiled out,
or one that failed to load.

`./le config unread-leaves report` is the advisory companion. It reports YANG leaves
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
completions. This is how config keyword names appear in autocomplete. The menu is sorted by
name (`getSortedChildren`), where a printed block follows definition order: a menu is what may
be typed, scanned for a name.
<!-- test: internal/component/cli/completer_order_test.go TestCompletionOrderUnchanged -->

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

The help text is read off the parse-tree type statements: the resolved
`EnumType` keeps only the name and the value. An enumeration that arrives
through a typedef, in whichever scope the typedef sits and through however many
typedefs, completes with the help the typedef's own `enum` statements declare,
because goyang leaves the typedef's type statement at `YangType.Base` when it
resolves the reference and `yang.EnumValueSummaries` follows that chain. A value
declared with no `ze:help` completes with no help rather than with the wrong
help.

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
| `leaf { type empty }` | `foo` (flag, no value). `foo true` is also read, so value-form output round-trips. Any other word is refused, because the flag would be set whatever the word said |

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

### Who may change the candidate

The config editor authorizes each config-mode verb that changes the
candidate, the store or the running configuration, when its surface installs
an authorizer (`Model.SetConfigAuthorizer`). The check runs in
`dispatchCommandWords`, which every config-mode line reaches: `top <command>`
comes back through it with its inner command, the operational-mode shortcut
that enters config mode on `set` dispatches the same line, and a pipe accepts
`show` alone. It runs before the verb's handler, so a refused line leaves the
candidate and the store as they were.

The verb table (`cli.ConfigPermission`) is the one declaration of which verb is
asked about with which string; the web terminal reads it too:

| Verbs | Permission |
|-------|------------|
| `set`, `activate` | `config set` |
| `delete`, `deactivate` | `config delete` |
| `commit` | `config commit` |
| `discard`, `disconnect` | `config discard` |
| `rollback` | `config rollback` |
| `rename` | `config rename` |
| `copy`, `insert` | `config add` |
| `save`, `load` | `config save` |

Every other verb (`edit`, `up`, `top`, `show`, `who`, `option`, `help`, `exit`)
changes nothing and is never asked about. A session on a named stored
configuration asks with the string, a space, then the name (`config commit
client-edge.conf`). A refusal reads `command restricted by access control:
<permission>`, wraps the dispatcher's `ErrUnauthorized` and ends with the fix
line. The SSH session factory installs the authorizer bound to the
connection's login, and plain `ze config edit` opens such an SSH session, so
it is authorized the same way. The attached console and `ze config edit` in
its file mode (`-f`) and backup mode (`--backup`) authenticate nobody and
install none, so every verb runs there.
<!-- source: internal/component/cli/config_permission.go -- ConfigPermission, authorizeConfigVerb, configRefusal -->
<!-- source: internal/component/cli/model_commands.go -- dispatchCommandWords -->
<!-- source: internal/component/cli/model.go -- SetConfigAuthorizer -->

### CLI Help from YANG

A config leaf's `ze:help` summary and its type constraints generate its help text:

```
ze(edit)# hold-time ?
  <0, 3-65535>    Hold time in seconds (RFC 4271: 0 or >= 3)
```

#### A command node declares two help texts

A command node in a `-cmd.yang` module declares its help in two statements, and
each one answers a different question.

| Statement | Holds | Read by |
|-----------|-------|---------|
| `ze:help` | the one-line SUMMARY of the command | every surface that shows a command on one line: a list row, a table cell, and the message line under the interactive completion menu |
| `description` | the LONG explanation of that one command | the help page for that command, and the box that Tab opens in the interactive CLI |
<!-- source: internal/component/cli/model_render.go -- warningText, renderExplanationBox -->

`mergeYANGEntry` (`internal/component/config/yang/command.go`) writes them to
`command.Node.ShortHelp` and `command.Node.Description`. Neither field is derived
from the other, and no reader shortens either one. A summary is authored short
because it is a summary.

#### An rpc declares the same two texts

An `rpc` statement carries the same pair, in the same two statements.
`ExtractRPCs` (`internal/component/config/yang/rpc.go`) writes them to
`RPCMeta.ShortHelp` and `RPCMeta.Description`.
<!-- source: internal/component/config/yang/rpc.go -- ExtractRPCs -->

`TestExtractRPCs` in `internal/core/ipc/yang_test.go` uses resolved fixture
modules to compare the extracted names, module names and exact help texts.
It also covers identical RPC names in different modules, either help text
being absent, and a loaded module with data and a notification but no RPCs.
The fixture expectations do not track the installed plugins' RPC inventory.

One reader serves both carriers. `GetHelpExtension` takes the extension
statement list, which a command container reaches through `Entry.Exts` and an
rpc through `gyang.RPC.Exts()`, and returns the summary. The explanation is the
goyang entry's own `Description`. A second reader would let the two surfaces
drift into two spellings of one declaration.

`./le doc yang-contract help-shape` checks command and RPC summaries: each is
one sentence of 25 words at most, on one line, with no semicolon and a full stop
at the end. The command reports the population it judges.

Extraction preserves an absent `description` as empty rather than inventing
one. The help-shape gate refuses an operator-visible command or RPC without
that explanation. If such a node reaches the help page, the page prints the
summary alone, and the interactive CLI says that the command declares none.
<!-- source: internal/component/cli/model_keys.go -- revealExplanation -->
<!-- source: internal/le/doc/yangcontract/helpshape.go -- judgePair -->

An empty `ze:help` is a defect. Every list that names the command shows a
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
| `ze:help` | the one-line SUMMARY of the node | the message row under the completion menu, the tooltip of the web editor form, the node line of the published configuration reference and its `llms.txt` roots, `show yang tree --config \| json` as `short-help`, and every list that names the node |
| `description` | the LONG explanation of that node | the box `?` opens on the highlighted candidate, the block under the input in the web editor form, the paragraph under the node in the configuration reference, and `show yang tree --config \| json` as `description` |
| `ze:help` on an `enum` value | the one-line SUMMARY of that value | the value completion row for the leaf, `show yang tree --config \| json` as `values[].short-help`, and the value list under the leaf in the configuration reference |
<!-- source: internal/component/cli/completer.go -- entryShortHelp, entryDescription, valueCompletions -->
<!-- source: internal/component/cli/model_keys.go -- revealCandidateExplanation -->
<!-- source: internal/component/config/yang/cli/tree.go -- walkYANGEntry, yangEnumValues -->
<!-- source: internal/component/web/handler_config_leaf.go -- buildLeafField -->
<!-- source: internal/le/site/config.go -- writeConfigChildMirror -->

Every carrier fills the pair from the same two calls: `GetHelpExtension` for
the summary and the goyang entry's `Description` for the explanation. An enum
value's summary has one reader too, `EnumValueSummaries`
(`internal/component/config/yang/enum.go`), which reads the value's own
`ze:help` off the leaf's parse-tree node. The completer resolves it once per
leaf and serves every later keystroke from that cache. A value that declares
no `ze:help` renders an empty summary: nothing stands in for it, and nothing
derives one text from the other on any surface.

`matchChildren` and `matchEditTargets` put both texts on the `contract.Completion`
they build, in `ShortHelp` and `Description`. The reader of the extension is
`GetHelpExtension`, the one a command container and an rpc already use.

A node that declares no `description` declares no explanation. `?` then says
`<path>: no explanation is declared` on the message row. The summary does not
stand in for it: a box repeating the row is the defect this split removes.

##### Several modules can declare one node, and the `?` box carries them all

A plugin attaches its leaves to a shared node by declaring a container of the
same name in its own module. It does not import the module that owns the node.
Removing the plugin then removes its leaves and nothing else, which is what
plugin self-containment requires. `class-of-service` reaches `interface` that
way.

`mergeAugmentedEntries` unions those declarations into one virtual entry. The
`description` of every declaration is JOINED into that entry, separated by a
blank line and in module-name order. A module that declares no help can no longer
erase one that does.

The join carries no module NAME, so the operator reads N paragraphs and cannot
tell which module wrote each one. The `?` box wraps the joined text into pages
inside the current terminal frame. PgUp and PgDn reach the remaining rows; the
box shows these keys and its current row range when paging is needed. The
declaration allowance is unchanged, so a node explained by several modules can
retain its full text without pushing the prompt off screen.
<!-- source: internal/component/cli/model_explanation.go -- explanationPage, scrollExplanation -->
<!-- source: internal/component/cli/model_render.go -- renderExplanationBox -->

The one-line `ze:help` of every declaration is joined the same way, onto the one
line the completion row draws, because nothing in the schema says which module
OWNS a shared node. Showing the first in module-name order showed `interface` as
the cos plugin's scaffolding text, and a module that declares no summary erased
one that does.

<!-- source: internal/component/cli/completer.go -- mergeAugmentedEntries, mergeDescriptions, mergeHelpExts -->

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
and `<name>@<revision>`. `Resolved.ModuleNames` answers the bare name alone, so a
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

A leaf with no `default` of its own takes the default of the typedef its type
derives from (RFC 7950 Section 7.3.4). The schema build refuses a default in
five cases: the value is not valid for the type, the leaf is `mandatory true`,
the leaf-list has `min-elements` of one or more, a leaf or typedef narrows the type so that the inherited default is no longer
valid and gives no new default, or the default names an enum or a bit that
carries an `if-feature`. That enum or bit is looked up in the leaf's own type,
in every typedef the type derives from, and, for a union, in the first member
type that accepts the default (RFC 7950 Section 9.12 order).

An integer default may be written in a module in decimal, in hexadecimal
(`0x1f`) or in octal (`017`, since a leading zero means octal there), each with
an optional sign (RFC 7950 Section 9.2.1). The schema build reads the notation,
checks the decimal value against the type, and gives the config tree the
decimal form, so `default 0x10` on a `uint8` leaf is the value `16`. A config
value is always decimal: the notations belong to the module alone. A config
value may still carry leading zeros, and they do not make it octal, so `010` in
a config file is ten (RFC 7950 Section 9.2.1: "integer is always interpreted as
a decimal number, and leading zeros are allowed").

A union default is read the way RFC 7950 Section 9.12 reads any union value:
against each member type in the order the module declares them, until one
accepts it. An integer member reads it in the module notations, so `default
0x10` on a union of `int32` then `string` is the value `16`, while on a union of
`string` then `int32` it stays the string `0x10`. A default no member accepts
stops the schema build.

A refused integer default names its cause: a notation that is none of the
three, a leading `0` followed by an 8 or a 9 (which octal has no digit for), or
a value larger than `uint64` holds. The CLI analysis tree shows the same
canonical default the config tree takes (`yangDefault`).
<!-- source: internal/component/config/yang_schema.go -- validateLeafDefaults, validateTypedefDefault, validateDefaultNotIfFeature, unionMemberHolding, checkModuleDefault, yangToLeaf -->
<!-- source: internal/component/config/yang/module_default.go -- CanonicalModuleDefault, canonicalUnionDefault, parseModuleInteger -->
<!-- source: internal/component/config/yang/cli/tree.go -- yangDefault -->

| Rule | Detail |
|------|--------|
| Positive assertion, one word | `enabled`, not `enable`, `disable` or `disabled` |
| Standard admin-state words are the only exception | `shutdown` (BFD, RFC 5880 section 6.8.16) and interface `disable` (kernel admin-down) are the canonical protocol and kernel terms, so they are allowed. BFD `shutdown` is typed `boolean` with `default false`. Interface `disable` is a `type empty` flag: `disable;` disables the interface, an absent leaf leaves it enabled, and `disable false;` is refused |
| No boolean-as-enum | A two-value on/off is not `enumeration { enum enable; enum disable; }`. A genuine tri-state for config inheritance is justified in the module; it is an exception |
| Bare flag | "This section is on when present" is a `presence` container, not a `type empty` leaf |

<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- leaf disable, type empty -->
<!-- source: internal/component/iface/config.go -- the disable leaf is read by presence -->
<!-- source: internal/component/config/parser.go -- parseLeaf, TypeEmpty -->

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
`./le verify current`) parses every `internal/**/config.go` and fails
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
inside the entry. A list whose key names several leaves is keyed by the entry's
canonical address instead (`fe80::b interface zbl1`: the first key value, then
`name value` for each other key leaf not at its YANG default), and those other
key leaves are also fields inside the entry. A consumer takes the first key
value as the map key's first space-separated token. The address is stated on
`syntax.md`, "What a reader receives".
<!-- source: internal/component/config/list_key.go -- BuildListKey -->

<!-- source: internal/core/configvalue -- LeafList, ListEntries -->
<!-- source: internal/core/configorder -- Entries, OrderKey -->

### Config text is set commands

The config format is set commands. Duplicate blocks are additive and the parser
merges them, so concatenating two valid config texts produces valid config.

| Manipulation method | When |
|---------------------|------|
| Parsed YANG tree | When a loaded config tree is in memory |
| Set command lines | When building or merging config text |
