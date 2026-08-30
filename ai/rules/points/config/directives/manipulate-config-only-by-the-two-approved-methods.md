---
kind: directive
level: MUST
stage:
---
- **Config content MUST be manipulated through one of two methods only.** See Config Manipulation.
- **Every tunable setting MUST live at the right level.** Misplacement erodes operator trust: invisible knobs surprise, config-tree clutter confuses.
- **Names cross four layers (YANG, env var, Go struct, CLI).** Each layer has its own convention, and the four MUST be derivable from each other. The conventions and the worked example are `ai/patterns/config-option.md`.
- **One concept, one spelling.** Where a shape already exists in the shared modules (`internal/component/config/yang/modules/ze-types.yang`, prefix `zt`; `ze-extensions.yang`, prefix `ze`), it MUST be reused. It MUST NOT be re-expressed locally.
- **Every coercion of a delivered config value MUST accept the string form.** See Config String Coercion.
- Config MUST NOT contain version numbers. Migration MUST be designed to be machine-transformable.
- Unknown keys MUST fail at any level. Silent ignore MUST NOT happen. The closest valid key MUST be suggested.
- Every YANG `environment/<name>` leaf MUST have a matching `ze.<name>.<leaf>` env var registered via `env.MustRegister()`. Env vars are part of the config interface, not follow-up work.
- The structural template is `ai/patterns/config-option.md`; the YANG system it plugs into is `docs/architecture/config/yang-config-design.md`; the rationale is `ai/rationale/config-design.md`.
