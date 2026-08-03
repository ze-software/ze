# enabled-gate-discards-service-settings

**Date:** 2026-08-03
**Spec:** `plan/spec-fixit-mgmt-listener-auth-guard.md` (AC-5)
**Class:** recurring. Second known instance.

## The shape

A service extractor answers one question with one boolean:

    enabled, _ := svc.Get("enabled")
    if enabled != configTrue {
        return Config{}, false
    }
    // ... tls, token, auth-mode parsed only past this point

That reads as "config does not start this service", which is correct. The defect
is that the SETTINGS are parsed after the gate, so `ok=false` also throws away
`tls`, `token`, and `auth-mode`.

It only bites when something OTHER than the config starts the listener: an env
var, or a CLI flag. The service then runs with zero-value settings, which for
every management surface in Ze means unauthenticated and plaintext. The operator
wrote `token`, read the config back, and got no listener behavior from it and no
diagnostic either.

## Where it has happened

| Service | Found by | Fixed in |
|---------|----------|----------|
| `environment.mcp` | a strengthened `test/plugin/task-identity-scope.ci`. The old one asserted per-principal task isolation while Alice and Bob were the same anonymous principal | `spec-mcp2026-1-stateless-core`, via `ExtractMCPSettings` / `ExtractMCPConfig` |
| `environment.looking-glass` | a security reviewer subagent tracing `LGTLSExplicit` end to end, during `spec-fixit-mgmt-listener-auth-guard` | `ExtractLGSettings` / `ExtractLGConfig`, same split |

The looking-glass instance was written by an implementer who had read the MCP
finding in the same spec file, one screen above the code being changed. Knowing
the pattern did not prevent repeating it, which is why it is recorded as a rule
shape rather than as a note.

## The fix

Split the one boolean into two questions, and give each its own function:

| Question | Function | ok=true means |
|----------|----------|---------------|
| Does config start a listener? | `ExtractXConfig` | the block exists AND says `enabled true` |
| What are this service's settings? | `ExtractXSettings` | the block exists |

A private `extractXBlock(tree) (Config, enabled, present bool)` parses everything
once and applies no gate of its own. Neither caller can inherit the other's
meaning. The hub takes addresses from the first and settings from the second.

## How to catch it

The unit test that matters asserts on a block that is deliberately NOT enabled:

    tree := lgTree("true", "lg-s3cret")
    ...Set("enabled", "false")
    _, ok := ExtractLGConfig(tree)      // false: config starts nothing
    cfg, ok := ExtractLGSettings(tree)  // true: settings survive
    // cfg.TLS, cfg.TLSExplicit, cfg.Token all preserved

A functional test that only starts the service FROM the config file cannot see
this defect: the gate passes, so the settings are read. The test has to start the
listener the way the operator did, from an env var or a flag
(`test/plugin/lg-token-gate.ci`, `test/plugin/mcp-cli-listener-honors-config-auth.ci`).

## The general rule

When an extractor returns `(Config, ok)`, ask what a caller does with `ok=false`.
If any caller proceeds to run the service anyway, `ok` is answering a narrower
question than its zero value implies, and every field behind the gate is now a
silent default (`ai/rules/protocol.md`, `ai/rules/evidence.md`).

Adding a third service to `environment` is the moment to check this, because the
copy-paste source is whichever extractor was written first.

## Files

- `internal/component/config/loader_extract.go` -- `extractLGBlock`,
  `ExtractLGSettings`, `ExtractLGConfig`, and the MCP originals
  `extractMCPBlock`, `ExtractMCPSettings`, `ExtractMCPConfig`
- `cmd/ze/hub/main.go` -- the hub takes addresses from one and settings from the other
- `internal/component/config/lg_extract_test.go` -- `TestExtractLGSettingsSurviveDisabledBlock`
- `test/plugin/lg-token-gate.ci` -- the looking-glass functional proof
- `test/plugin/mcp-cli-listener-honors-config-auth.ci` -- the MCP functional proof
- `plan/learned/RECURRING-PATTERNS.md` -- the short form of this entry
