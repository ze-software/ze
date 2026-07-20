# Config Surface: YANG Config vs Env Var

**When:** deciding whether a new setting is a YANG config leaf or an env var
**Severity:** advisory
**Related:** config-design

## Directives

Naming: `ai/rules/config-naming.md`

Every tunable setting must live at the right level. Misplacement erodes
operator trust: invisible knobs surprise, config-tree clutter confuses.

## Decision Table

| Question | If YES | If NO |
|----------|--------|-------|
| Would an operator change this during normal capacity planning or traffic engineering? | YANG config | Keep reading |
| Does it need validation, commit/rollback, or config diff? | YANG config | Keep reading |
| Should it appear in `show configuration` or config backups? | YANG config | Keep reading |
| Is it a debug, emergency, or development-only knob? | Env var only | YANG config |
| Is it needed before config loads (bootstrap)? | Env var only | YANG config |
| Is it a safety cap that should never be tuned in production? | Env var only | YANG config |

**Default answer: YANG config.** Env-only is the exception, not the default.
When uncertain, the setting goes in YANG. Promoting later is a breaking
workflow change for operators who already use the env var.

## YANG Config (operator-facing)

Settings that belong in the config tree:

- Queue depths, buffer sizes, batch limits, pool budgets
- Timers that affect convergence or session behavior
- Feature toggles that change observable routing behavior
- Capacity knobs (max peers, max prefixes, max routes)
- Any setting an operator would document in a change ticket

Properties: visible in `show configuration`, validated by YANG constraints,
part of commit/rollback, included in config backups, discoverable via CLI
completion.

## Env Var Only (internal/debug)

Settings that stay as env vars:

- Emergency escape hatches (safety valves, deadline overrides)
- Debug instrumentation (artificial delays, verbose tracing)
- Bootstrap settings needed before config is parsed
- Internal safety caps that protect against code bugs, not traffic
- Metrics/observability plumbing intervals

Properties: invisible to operators unless they read the source, no
validation, no commit/rollback, requires restart to change.

## When Both Exist

When a setting is promoted from env-only to YANG config, the env var
remains as an override. Precedence (highest wins):

1. Env var (emergency override, always wins)
2. YANG config value
3. YANG default (from schema)

The YANG leaf description MUST document that an env var override exists
and name it. Operators should not be surprised that their config value
is being overridden.

## Promotion Signals

An env var should be promoted to YANG config when any of these are true:

- It appears in runbooks or deployment documentation
- Multiple operators have asked about it or been told to set it
- It controls behavior visible in `show` commands or logs
- Changing it is part of normal scaling or tuning workflows
- It was added as env-only for expedience during implementation

## New Setting Checklist

Before adding any tunable setting:

```
[ ] Classified as YANG config or env-only using the decision table above
[ ] If YANG: leaf defined with type, range, default, description
[ ] If YANG: description mentions env var override if one exists
[ ] If env-only: env.MustRegister() with clear description
[ ] If env-only: document WHY it is not in YANG (debug, bootstrap, safety cap)
[ ] If promoting: old env var preserved, precedence documented
```
