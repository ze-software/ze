# Configuration and YANG

**When:** adding or changing a config option, YANG module, env var, listener endpoint, or code that reads config values
**Severity:** blocking
**Related:** go-standards, cli, architecture, evidence, plugins

## Directives

**Config content MUST be manipulated by one of two methods only: a parsed YANG tree when a loaded tree is in memory, or `set` command lines when building or merging config text.** Raw text surgery, a custom merge function that parses config syntax outside the config system, and any manipulation that infers structure from text patterns MUST NOT be used. An unknown key MUST fail at every level with the closest valid key suggested, and MUST NOT be ignored.

**A tunable setting MUST default to a YANG leaf; env-only is the exception, reserved for an emergency override, debug instrumentation, a bootstrap value read before config parses, or an internal safety cap.** Precedence is env var, then config value, then YANG default, and the leaf `description` MUST name the env var that overrides it. Names cross YANG, env var, Go field and CLI, and the four MUST be derivable from each other: leaf names are spelled out in full with no abbreviation, and the env key's final segment is the leaf name. The structural template is `ai/patterns/config-option.md` and the YANG system is `docs/architecture/config/yang-config-design.md`.

**Every delivered config value arrives as a JSON string, so every coercion MUST carry a `case string:` arm and MUST NOT assert `v.(bool)` or `v.(float64)` directly.** `./le config coercion check` refuses both shapes. A `leaf-list` MUST be read with `configvalue.LeafList`, a `list` with `configvalue.ListEntries`, an `ordered-by user` list with `configorder.Entries`, and a plugin's config MUST be lowered with `Tree.ToPluginMap` rather than `Tree.ToMap`; a slice assertion MUST NOT be used on any of them. The shape each node arrives in is `docs/architecture/config/yang-config-design.md`.
