# Pattern: Config Option

Structural template for adding a configuration option to Ze.
Rules: `ai/rules/config.md`. Architecture: `docs/architecture/config/yang-config-design.md`.

## Also Read

| Rule | When it applies |
|------|----------------|
| `ai/rules/config.md` (Listeners) | Network endpoint options: use `zt:listener` + `ze:listener` |
| `ai/rules/config.md` (YANG Structure) | Augment vs grouping: augment only for cross-component extension |
| `ai/rules/go-standards.md` (Environment Variables) | Every YANG `environment/` leaf needs `env.MustRegister()` |
| `ai/rules/protocol.md` | Backend/translator config: exact application or verify/commit fails |
| Full navigation: `ai/INDEX.md` | |

## End-to-End Pipeline

```
1. YANG leaf definition
2. YANG module registration (init() + go:embed)
3. env.MustRegister() (if under environment/)
4. Go struct field + LoadEnvironmentWithConfig() (if env var)
5. Custom validator (if beyond YANG native validation)
6. Functional test
```

**Every step is mandatory for its category.** Missing any step is a bug.

## Step 1: YANG Leaf Definition

File: `internal/component/<name>/yang/ze-<name>-conf.yang` (or existing module).

```yang
leaf my-option {
    type string;                          // From ze-types.yang
    default "my-default";                 // Required if accessed at startup
    description "User-facing help text";  // Mandatory (CLI tooltip)
}
```

**Available types** (from `ze-types.yang`): `string`, `uint16`, `uint32`, `boolean`,
`ipv4-address`, `ipv6-address`, `ip-address`, `asn`, `prefix-ipv4`, `prefix-ipv6`.

**Enum:**
```yang
leaf mode {
    type enumeration {
        enum enable;
        enum disable;
        enum require;
    }
    default "enable";
    description "Operating mode";
}
```

**Constraints:** `range "1..65535"`, `length "1..255"`, `pattern "[a-z]+"`.

**Extensions:**
```yang
ze:sensitive;                    // Mask in display output
ze:validate "registered-families";  // Custom runtime validator
ze:allow-unknown-fields;         // Allow arbitrary child keys
```

## Step 2: YANG Module Registration

If adding to an **existing** module, just add the leaf. If creating a **new** module:

**embed.go:**
```go
package schema

import _ "embed"

//go:embed ze-<name>-conf.yang
var Ze<Name>ConfYANG string
```

**register.go:**
```go
package schema

import "github.com/ze-software/ze/internal/component/config/yang"

func init() {
    yang.RegisterModule("ze-<name>-conf.yang", Ze<Name>ConfYANG)
}
```

**YANG module header:**
```yang
module ze-<name>-conf {
    namespace "urn:ze:<name>:conf";
    prefix <name>;
    import ze-types { prefix zt; }
    import ze-extensions { prefix ze; }
    // ...
}
```

## Step 3: Environment Variable (if under `environment/`)

**Rule (BLOCKING):** Every YANG `environment/<name>` leaf MUST have a matching env var.

File: `internal/component/config/environment.go`

```go
var _ = env.MustRegister(env.EnvEntry{
    Key:         "ze.bgp.section.my-option",
    Type:        "string",     // "int", "bool", "int64", "float64"
    Default:     "my-default",
    Description: "What this does",
})
```

## Step 4: Go Struct + Loading (if env var)

File: `internal/component/config/environment.go`

```go
type SectionEnv struct {
    MyOption string
}
```

In `LoadEnvironmentWithConfig()`:
```go
if e.Section.MyOption, err = SchemaDefaultString(schema, "environment.section.my-option"); err != nil {
    return err
}
```

**Defaults come from YANG, not Go.** `SchemaDefault*()` reads the YANG default.

| Go accessor | YANG type |
|-------------|-----------|
| `SchemaDefaultString()` | `string`, `enumeration` |
| `SchemaDefaultInt()` | `uint16`, `uint32` |
| `SchemaDefaultBool()` | `boolean` |
| `SchemaDefaultFloat64()` | Decimal types |
| `SchemaDefaultOctal()` | File permissions |

## Step 5: Custom Validator (if needed)

When YANG native validation (enum, range, pattern) is not enough.

**validators.go:**
```go
func MyValidator() yang.CustomValidator {
    return yang.CustomValidator{
        ValidateFn: func(path string, value any) error {
            // Return nil if valid, error if not
        },
        CompleteFn: func() []string {
            // Return valid values for CLI tab-completion
        },
    }
}
```

**validators_register.go:**
```go
func RegisterValidators(reg *yang.ValidatorRegistry) {
    reg.Register("my-validator-name", MyValidator())
    // ...existing registrations...
}
```

**YANG reference:**
```yang
leaf my-field {
    type string;
    ze:validate "my-validator-name";
}
```

**Startup check:** `yang.CheckAllValidatorsRegistered()` panics if any `ze:validate` name
has no matching `reg.Register()` call.

## YANG Module Naming

| Category | Pattern | Example |
|----------|---------|---------|
| Config | `ze-<component>-conf` | `ze-bgp-conf` |
| API/RPC | `ze-<component>-api` | `ze-bgp-api` |
| CLI tree | `ze-cli-<verb>-cmd` | `ze-cli-show-cmd` |
| Types (core) | `ze-types` | -- |
| Extensions (core) | `ze-extensions` | -- |

Namespace: `urn:ze:<domain>:<purpose>` (e.g., `urn:ze:bgp:conf`).

## YANG Config or Env Var

`ai/rules/config.md` carries the obligations. This is the decision aid.

| Question | If YES | If NO |
|----------|--------|-------|
| Would an operator change this during normal capacity planning or traffic engineering? | YANG config | Keep reading |
| Does it need validation, commit and rollback, or a config diff? | YANG config | Keep reading |
| Should it appear in `show configuration` or in a config backup? | YANG config | Keep reading |
| Is it a debug, emergency, or development-only knob? | Env var only | YANG config |
| Is it needed before config loads (bootstrap)? | Env var only | YANG config |
| Is it a safety cap that should never be tuned in production? | Env var only | YANG config |

A YANG leaf is visible in `show configuration`, validated by YANG constraints,
part of commit and rollback, included in config backups, and discoverable
through CLI completion. An env-only setting has none of those: it is invisible
unless the operator reads the source, it is unvalidated, and changing it needs a
restart.

## Naming Across Layers

Full naming conventions: `ai/rules/config.md`

| Layer | Convention | Example |
|-------|-----------|---------|
| YANG leaf | kebab-case, no abbreviations | `forward-queue-size` |
| Go struct field | PascalCase of the YANG leaf, same word boundaries | `ForwardQueueSize`, `ReadBufferSize` (not `ReadBufSize`) |
| Env var | `ze.<component>.<container>.<yang-leaf>`, dot-separated and lowercase | `ze.bgp.reactor.forward-queue-size` |
| CLI input | kebab-case | `reactor forward-queue-size 128` |
| Config file | kebab-case | `forward-queue-size 128;` |

### YANG leaf names

| Rule | Example | Anti-pattern |
|------|---------|--------------|
| kebab-case, no abbreviations | `forward-queue-size` | `fwd-chan-size` |
| Noun or noun phrase | `read-buffer-size` | `read-buf-sz` |
| Dimensioned value: state the unit through a `units` statement and keep the name unit-free | `teardown-grace` plus `units seconds;` | `teardown-grace-seconds`, or `teardown-grace` with no `units` |
| No `ze-` prefix; it is implicit in the tree | `cache-ttl` | `ze-cache-ttl` |
| Boolean: positive assertion | `update-groups` | `no-update-groups`, `disable-update-groups` |
| A `leaf-list` or a `list` is named in the PLURAL; a single `leaf` is singular, so a reader knows how many values may be written before reaching the type | `communities`, `prefixes`, `as-sets` | `community`, `prefix`, `as-set` on a leaf-list |

The one exception to "no abbreviations" is an industry-standard abbreviation
clearer than its expansion: `ttl`, `mtu`, `tcp`, `bgp`, `asn`, `med`, `ebgp`,
`ibgp`.

### YANG container names

| Rule | Example | Anti-pattern |
|------|---------|--------------|
| Singular noun for the subsystem | `reactor` | `reactor-settings`, `reactor-config` |
| No `-config` or `-settings` suffix | `session` | `session-config` |
| Group related leaves, rather than one leaf per container | `reactor { cache-ttl; cache-max; forward-queue-size; }` | `reactor-cache { ttl; max; }` plus `reactor-forward { queue-size; }` |

### The env var path mirrors the YANG path

The dotted env var path mirrors the YANG tree from the component root down, and
its final segment is the YANG leaf name exactly.

| YANG path under `environment` | Env var |
|-------------------------------|---------|
| `bgp / reactor / cache-ttl` | `ze.bgp.reactor.cache-ttl` |
| `bgp / reactor / cache-max` | `ze.bgp.reactor.cache-max` |
| `bgp / reactor / update-groups` | `ze.bgp.reactor.update-groups` |
| `bgp / openwait` | `ze.bgp.openwait` |
| `chaos / seed` | `ze.bgp.chaos.seed` |

<!-- source: internal/component/config/apply_env.go -- envPlumbingTable, the YANG-leaf-to-env-key map -->
<!-- source: internal/component/config/environment.go -- env.MustRegister for each key -->

### Legacy env var keys that predate the convention

Each of these is registered with a `Deprecated:` note naming its YANG leaf, and
each still owes the alias that matches the YANG name. `envPlumbingTable`
(`internal/component/config/apply_env.go`) maps the YANG leaf to the legacy key
only, and no alias is registered today.

| YANG leaf under `environment/bgp/reactor` | Legacy env key | Alias owed |
|-------------------------------------------|----------------|------------|
| `forward-queue-size` | `ze.fwd.chan.size` | `ze.bgp.reactor.forward-queue-size` |
| `forward-batch-limit` | `ze.fwd.batch.limit` | `ze.bgp.reactor.forward-batch-limit` |
| `forward-pool-max-bytes` | `ze.fwd.pool.maxbytes` | `ze.bgp.reactor.forward-pool-max-bytes` |
| `forward-pool-headroom` | `ze.fwd.pool.headroom` | `ze.bgp.reactor.forward-pool-headroom` |
| `forward-teardown-grace` | `ze.fwd.teardown.grace` | `ze.bgp.reactor.forward-teardown-grace` |
| `read-buffer-size` | `ze.buf.read.size` | `ze.bgp.reactor.read-buffer-size` |
| `write-buffer-size` | `ze.buf.write.size` | `ze.bgp.reactor.write-buffer-size` |

## Config Override Priority (highest first)

1. `ze.bgp.section.my-option` env var (dot notation)
2. `ze_bgp_section_my_option` env var (underscore notation)
3. Config file value
4. YANG default

## Two-Phase YANG Loading

1. `LoadEmbedded()` -- `ze-extensions.yang`, `ze-types.yang` (hardcoded in binary)
2. `LoadRegistered()` -- all modules registered via `init()` calls

Core types/extensions must be embedded. Plugin schemas are registered at import time.

## Reference Implementations

| What | File |
|------|------|
| YANG config module | `internal/component/bgp/yang/ze-bgp-conf.yang` |
| Hub config module | `internal/component/hub/yang/ze-hub-conf.yang` |
| Env var registration | `internal/component/config/environment.go` |
| Schema loading | `internal/component/config/yang_schema.go` |
| Custom validators | `internal/component/config/validators.go` |
| Config parsing | `internal/component/config/schema.go` |

## Checklist

```
[ ] Classified as YANG config or env-only using the decision table above
[ ] If env-only: documented WHY (debug, bootstrap, safety cap)
[ ] If promoting an env var: old key preserved, precedence documented
[ ] YANG leaf defined with type, default, description
[ ] YANG leaf: full words, kebab-case, no abbreviations
[ ] YANG leaf: dimensioned value carries a `units` statement, name unit-free
[ ] YANG leaf: description names the env var override when one exists
[ ] YANG module registered (init() + go:embed) or existing module extended
[ ] Namespace is urn:ze:<component>:<kind> (kind is a colon segment)
[ ] Prefix is short, unquoted, no hyphens; zt and ze not reused
[ ] Module has a revision and a description; no stray organization
[ ] Every IP, prefix, ASN, port or community leaf uses the zt typedef
[ ] ze:validate used only for runtime sets, never to duplicate pattern/range
[ ] Endpoint uses zt:listener (bind) or zt:endpoint (target); no host:port string
[ ] Toggles are positive `enabled` booleans; no type empty, no enable/disable enum
[ ] Cross-protocol concept matches its siblings (grep OSPF, IS-IS, BGP first)
[ ] 4-space indent; compact leaves only for type (+ default, description)
[ ] If env var: env.MustRegister() in environment.go
[ ] If env var: key is ze.<component>.<container>.<yang-leaf>, final segment exact
[ ] If env var: Go struct field in environment.go, PascalCase of the YANG leaf
[ ] If env var: Loaded in LoadEnvironmentWithConfig() via SchemaDefault*()
[ ] If a legacy env var exists: alias registered matching the new convention
[ ] If custom validation: validator in validators.go
[ ] If custom validation: registered in validators_register.go
[ ] If custom validation: ze:validate in YANG leaf
[ ] Config test (.ci) in test/parse/ verifying it parses
[ ] Documentation updated (config guide or architecture doc)
```
