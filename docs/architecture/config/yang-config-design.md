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
| `ze:method` | Declares the wire method of an rpc or notification that no command node points at: the plugin IPC protocol and the event stream | WireMethod string |
| `ze:rpc` | On a `ze:command` node, names the rpc that documents the command's input and output; the rpc is published under the node's wire method | `module:rpc-name` |
| `ze:cumulative` | Leaf-list accumulates values from the bgp, group and peer levels instead of the most specific level replacing them | (none) |
| `ze:decorate` | Attaches a registered display-time decorator to a leaf | decorator name |
| `ze:display-key` | Names the leaf the web interface shows for a keyless list entry | (none) |
| `ze:edit-shortcut` | Makes a command available in edit mode without the `run` prefix | (none) |
| `ze:ensure-exists` | Marks a command container as a resource checkpoint. Each descendant command ensures the resource exists | WireMethod of the rollback handler |
| `ze:ephemeral` | Node is validated and completed but never written to the config file | (none) |
| `ze:filter` | Marks a list as a named filter type of the route policy framework | (none) |
| `ze:flatten` | Serializes a container's children with the container name as a leading keyword | (none) |
| `ze:help` | Declares the one-line SUMMARY of a command node, an rpc, a config node, or an enum value. The `description` statement beside it declares the long explanation | one sentence, one line |
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
`tools/list` and `tools/call` answer a JSON-RPC internal error. The `ze cli`
client builds its YANG state on first use, never at package init, and caches
the error with it: every accessor returns it, so `ze` help, usage, the menu,
verb dispatch, shell completion and the command catalog report the cause and
exit 1, and `IsDeclaredCommand` returns it rather than `false`, so the
local-handler lookup refuses the match instead of serving a handler that
shadows a declared command.

The same two callers refuse four structures RFC 7950 forbids and goyang
accepts. Each refusal names the module, the file location and the fault.

| Structure refused | RFC 7950 | Why goyang misses it | Error |
|-------------------|----------|----------------------|-------|
| A `length` whose parts, in the order written, overlap or descend, with `min` and `max` read as the bounds of the type being restricted | 9.4.4 | it sorts and coalesces the parts before it checks them | `ErrLengthOrder` |
| An enumeration whose values break Section 9.6.4.2: a value outside int32, a value assigned twice, an enum with no value after one holding 2147483647 | 9.6.4.2 | it checks only an enumeration it resolves, and it resolves none in a grouping no schema node uses | `ErrEnumValue` |
| An `enum` in a restricted enumeration that the base type does not assign, or whose `value` differs from the base type's | 9.6.4, 9.6.4.2 | it builds the restricted values afresh and never compares them with the base | `ErrEnumRestriction` |
| A statement under an extension statement that is not a YANG keyword, that the block holding it does not admit, whose argument breaks its Section 14 argument rule, whose substatements break the counts or the alternatives of its rule's block, or that omits a block its rule requires | 7.19 | it keeps an extension statement as raw text | `ErrExtensionSubstatement` |

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
Ze builds against the `ze-software/goyang` fork, which carries the fix proposed
in openconfig/goyang#317 (the `replace` in `go.mod`), until a goyang release
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
<!-- source: internal/component/config/yang/loader_structure.go -- checkStructure, restrictedLengthSpan, resolveProduction, extensionSubstatementError -->
<!-- source: internal/component/config/yang/loader_abnf.go -- parseYANGGrammar, rfc7950Grammar -->
<!-- source: internal/component/config/yang/loader_grammar.go -- argumentCheckers, checkURI, checkPathArg -->
<!-- source: internal/component/config/yang/loader_source.go -- statementHasBlock -->
<!-- source: internal/component/config/yang/loader.go -- sourcedModules.Parse, sourcedModules.source -->
<!-- source: cmd/ze/hub/command_meta.go -- commandMetaSource -->
<!-- source: internal/component/cli/client/main.go -- loadYANGState, buildYANGState -->
<!-- source: internal/component/cli/client/verb_tree.go -- IsDeclaredCommand -->
<!-- source: internal/component/command/registry/registry.go -- LookupLocal -->
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
(the plugin IPC protocol in `internal/core/ipc/yang/`) and a notification
declare their wire method with `ze:method`. An rpc that has neither is
published under no name, and the command contract gate refuses it.
<!-- source: internal/component/config/yang/rpc_publish.go -- PublishedRPCs -->

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

`LoadRegistered()` attempts every registered module and joins every parse
error, each naming its module, so one broken module does not hide the modules
registered after it.

After both phases, `Resolve()` resolves all cross-module imports via goyang and
runs the checks above. It is the one transition from loading to reading, and it
answers a distinct type:

| Type | Holds | Operations |
|------|-------|------------|
| `Loader` | Modules added, not yet checked | `LoadEmbedded`, `LoadRegistered`, `AddModuleFromText`, `AddModuleFromFile`, `Resolve` |
| `Resolved` | A module set every check passed, and every pattern `checkPatterns` compiled, keyed by its text | `GetModule`, `GetEntry`, `ModuleNames`, `ConfModuleNames`, `APIModuleNames` |

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

One reader serves both carriers. `GetHelpExtension` takes the extension
statement list, which a command container reaches through `Entry.Exts` and an
rpc through `gyang.RPC.Exts()`, and returns the summary. The explanation is the
goyang entry's own `Description`. A second reader would let the two surfaces
drift into two spellings of one declaration.

`./le doc yang-contract help-shape` holds both corpora to one shape: 601 command tree
nodes and 211 RPCs, each summary one sentence of 25 words at most, on one line,
with no semicolon and a full stop at the end.

An empty `description` means nobody has written an explanation for that
command. That is not a defect. The help page then prints the summary alone, and
the interactive CLI says that the command declares none.
<!-- source: internal/component/cli/model_keys.go -- revealExplanation -->

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
tell which module wrote each one. The `?` box also draws what fits and no more:
`renderExplanationBox` wraps to the rows the box has, prints `... N more`, and
no key scrolls it. A node several modules explain can therefore hold more text
than the box will ever show.

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
<!-- source: internal/component/config/yang_schema.go -- validateLeafDefaults, validateTypedefDefault, validateDefaultNotIfFeature, unionMemberHolding -->

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
