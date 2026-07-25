# Pattern: Config Option

Structural template for adding a configuration option to Ze.
Rules: `ai/rules/config-design.md`. Architecture: `docs/architecture/config/yang-config-design.md`.

## Also Read

| Rule | When it applies |
|------|----------------|
| `ai/rules/config-design.md` (Listeners) | Network endpoint options: use `zt:listener` + `ze:listener` |
| `ai/rules/config-design.md` (YANG Structure) | Augment vs grouping: augment only for cross-component extension |
| `ai/rules/go-standards.md` (Environment Variables) | Every YANG `environment/` leaf needs `env.MustRegister()` |
| `ai/rules/exact-or-reject.md` | Backend/translator config: exact application or verify/commit fails |
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

## Naming Across Layers

Full naming conventions: `ai/rules/config-naming.md`
Decision framework (YANG vs env var): `ai/rules/config-surface.md`

| Layer | Convention | Example |
|-------|-----------|---------|
| YANG leaf | kebab-case, no abbreviations | `forward-queue-size` |
| Go struct field | PascalCase of YANG leaf | `ForwardQueueSize` |
| Env var | `ze.<component>.<container>.<yang-leaf>` | `ze.bgp.reactor.forward-queue-size` |
| CLI input | kebab-case | `reactor forward-queue-size 128` |
| Config file | kebab-case | `forward-queue-size 128;` |

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
[ ] YANG leaf defined with type, default, description
[ ] YANG module registered (init() + go:embed) or existing module extended
[ ] If env var: env.MustRegister() in environment.go
[ ] If env var: Go struct field in environment.go
[ ] If env var: Loaded in LoadEnvironmentWithConfig() via SchemaDefault*()
[ ] If custom validation: validator in validators.go
[ ] If custom validation: registered in validators_register.go
[ ] If custom validation: ze:validate in YANG leaf
[ ] Config test (.ci) in test/parse/ verifying it parses
[ ] Documentation updated (config guide or architecture doc)
```
