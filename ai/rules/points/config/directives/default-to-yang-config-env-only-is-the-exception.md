---
kind: directive
level: MUST
stage:
---
**A tunable setting MUST default to a YANG leaf; env-only is the exception, reserved for an emergency override, debug instrumentation, a bootstrap value read before config parses, or an internal safety cap.** Precedence is env var, then config value, then YANG default, and the leaf `description` MUST name the env var that overrides it. Names cross YANG, env var, Go field and CLI, and the four MUST be derivable from each other: leaf names are spelled out in full with no abbreviation, and the env key's final segment is the leaf name. The structural template is `ai/patterns/config-option.md` and the YANG system is `docs/architecture/config/yang-config-design.md`.
