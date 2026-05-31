# Handover: finish verb-first command migration

Goal: make command names consistently verb-first. Ze has not released these commands, so invalid old spellings must be removed, not preserved as compatibility aliases.

Evidence to start from:
- RIB runtime commands are registered under verb-first names in `internal/component/bgp/plugins/rib/rib.go:620-646`, but old aliases and old YANG proxy paths may still expose pre-rule spellings.
- Deprecated alias infrastructure is required for future released command migrations, but this migration must not use it for unreleased old names.
- Inter-plugin dispatch strings must use canonical verb-first commands only.

Read first:
- `ai/rules/cli-grammar.md`
- `ai/rules/derive-not-hardcode.md`
- `ai/rules/wiring-completeness.md`
- `plan/spec-command-verb-first.md`

Work:
1. Build the current command inventory from registrations and tests.
2. Rename registrations, handlers, proxy commands, tests, docs, and inter-plugin dispatch strings to the verb-first names from the spec.
3. Remove old names entirely from product registrations, YANG command paths, handlers, tests, docs, and inter-plugin dispatch strings.
4. Keep deprecated alias infrastructure implemented and tested with synthetic fixture commands only.
5. Update functional tests that invoke renamed commands.

Acceptance:
- New command names work through the user entry point.
- Old command names are rejected as unknown.
- No old names remain outside rejection tests or historical notes.
- Targeted unit tests cover at least RIB, sysctl, adj-rib-in, RPKI, GR dispatch, production no-alias guardrails, and synthetic deprecated alias behavior.
- Run the specific functional tests changed by the migration, then run the command/doc consistency checks required by the spec.
