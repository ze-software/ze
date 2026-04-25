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
<!-- source: internal/component/bgp/schema/ze-bgp-conf.yang -- config tree structure -->
<!-- source: internal/component/bgp/schema/ze-bgp-api.yang -- RPC definitions -->

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
| `ze:syntax` | Config parser syntax mode (flex, freeform, inline-list, etc.) | mode name |
| `ze:validate` | References a Go validator function for runtime validation + completion | function name |
| `ze:command` | Marks a `config false` container as an executable CLI command | WireMethod string |
| `ze:edit-shortcut` | Makes a command available in edit mode without `run` prefix | (none) |
| `ze:sensitive` | Marks a leaf as containing sensitive data (passwords, keys) | (none) |
| `ze:key-type` | Key type for inline-list nodes | type name |
| `ze:route-attributes` | Marks a node as accepting standard BGP route attributes | (none) |
| `ze:allow-unknown-fields` | Container accepts arbitrary key-value pairs | (none) |
| `ze:related` | V2 workbench: declares an operator tool descriptor on a config node | descriptor string |

<!-- source: internal/component/config/yang/modules/ze-extensions.yang -- all extension definitions -->

### Config Schemas: `ze-bgp-conf.yang` and siblings

Define the actual configuration tree that users interact with. These are the modules
the CLI completer walks to know what nodes are valid at each position.

Config schemas import `ze-types` for leaf types and `ze-extensions` for behavior annotations.

| Schema | Owns | Location |
|--------|------|----------|
| `ze-bgp-conf` | BGP configuration (peers, families, capabilities) | `component/bgp/schema/` |
| `ze-hub-conf` | Hub/environment settings | `component/hub/schema/` |
| `ze-system-conf` | System-level configuration | `component/config/system/schema/` |
| `ze-plugin-conf` | Plugin configuration | `component/plugin/schema/` |
| `ze-ssh-conf` | SSH transport configuration | `component/ssh/schema/` |
| `ze-authz-conf` | Authorization configuration | `component/authz/schema/` |
| `ze-telemetry-conf` | Telemetry configuration | `component/telemetry/schema/` |
| Plugin schemas | Per-plugin config (GR, RPKI, role, hostname, etc.) | `component/bgp/plugins/<name>/schema/` |

<!-- source: internal/component/bgp/schema/ze-bgp-conf.yang -- BGP config tree -->
<!-- source: internal/component/hub/schema/ze-hub-conf.yang -- Hub/environment config -->

### API Schemas: `ze-bgp-api.yang` and `ze-*-cmd.yang`

Define RPCs (request/response operations) and the CLI command tree. The `-api.yang` modules
define RPC signatures. The `-cmd.yang` modules define the CLI navigation hierarchy using
`config false` containers with `ze:command` extensions.

| Schema | Purpose | Location |
|--------|---------|----------|
| `ze-bgp-api` | BGP peer/route/cache RPCs | `component/bgp/schema/` |
| `ze-bgp-cmd-peer-api` | Peer management commands | `component/bgp/plugins/cmd/peer/schema/` |
| `ze-rib-api` | RIB query RPCs | `component/bgp/plugins/rib/schema/` |
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
component/<name>/schema/
    ze-<name>.yang          # Schema file (embedded via //go:embed)
    register.go             # init() calls yang.RegisterModule()
```

<!-- source: internal/component/config/yang/register.go -- RegisterModule, Module struct -->

---

## 4. Validation

Ze validates configuration trees in two layers.

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

In Go, each validator registers a `CustomValidator` with two functions:

| Function | Purpose |
|----------|---------|
| `ValidateFn(path, value) error` | Validates a value at parse/commit time |
| `CompleteFn() []string` | Returns valid values for CLI completion (optional) |

<!-- source: internal/component/config/yang/validator_registry.go -- CustomValidator, ValidatorRegistry -->

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

<!-- source: internal/component/cli/completer.go -- valueCompletions, validateCompletions, typeHint -->

**Why `ze:validate` takes priority over enum:** If a developer sets `ze:validate` on an enum leaf,
they want dynamic completion from runtime state, not the static enum values. The `CompleteFn`
queries whatever is currently registered (families, event types, send types), reflecting the
plugins that are actually loaded.

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

### CLI Help from YANG

Leaf descriptions and type constraints generate help text:

```
ze(edit)# hold-time ?
  <0, 3-65535>    Hold time in seconds (RFC 4271: 0 or >= 3)
```

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
internal/component/bgp/schema/
    ze-bgp-conf.yang        # BGP configuration tree
    ze-bgp-api.yang         # BGP RPCs

internal/component/hub/schema/
    ze-hub-conf.yang        # Hub/environment config

internal/component/config/system/schema/
    ze-system-conf.yang     # System config

internal/component/bgp/plugins/<name>/schema/
    ze-<name>.yang          # Plugin-specific config
    ze-<name>-api.yang      # Plugin RPCs (if any)
    ze-<name>-cmd.yang      # CLI command tree (if any)

internal/component/cmd/<name>/schema/
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
