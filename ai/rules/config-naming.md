# Config Naming Conventions

**When:** Names cross four layers (YANG, env var, Go struct, CLI)
**Severity:** advisory
**Related:** naming, config-design

## Directives

Names cross four layers (YANG, env var, Go struct, CLI). Each layer has
its own convention, but they must be derivable from each other. An
operator reading `show configuration` should recognize the env var from
the docs, and vice versa.

## YANG Leaves

| Rule | Example | Anti-pattern |
|------|---------|-------------|
| kebab-case, no abbreviations | `forward-queue-size` | `fwd-chan-size` |
| Noun or noun-phrase | `read-buffer-size` | `read-buf-sz` |
| Unit suffix when ambiguous | `teardown-grace-seconds` | `teardown-grace` (seconds? milliseconds?) |
| No `ze-` prefix (implicit in the tree) | `cache-ttl` | `ze-cache-ttl` |
| Boolean: positive assertion | `update-groups` | `no-update-groups`, `disable-update-groups` |

**No abbreviations in YANG.** Operators read YANG leaves in CLI completion
and `show configuration`. `fwd` means nothing to someone who did not write
the code. Spell it out: `forward`, `buffer`, `channel`, `maximum`.

Exception: industry-standard abbreviations that are clearer than their
expansion: `ttl`, `mtu`, `tcp`, `bgp`, `asn`, `med`, `ebgp`, `ibgp`.

## Env Vars

| Rule | Example |
|------|---------|
| Dot-separated, lowercase | `ze.bgp.reactor.forward-queue-size` |
| Prefix: `ze.<component>` | `ze.bgp.reactor.cache-ttl` |
| Leaf name matches YANG leaf exactly | YANG `forward-queue-size` = env `ze.bgp.reactor.forward-queue-size` |

**Env var leaf matches YANG leaf.** When a setting exists in both YANG and
env, the final segment of the env var key MUST be the YANG leaf name.
This makes the mapping mechanical and documentable.

Legacy env vars that predate the YANG leaf keep their old key for
backwards compatibility but MUST register an alias matching the YANG name.

| Legacy env var | YANG leaf | Alias (MUST add) |
|----------------|-----------|------------------|
| `ze.fwd.chan.size` | `forward-queue-size` | `ze.bgp.reactor.forward-queue-size` |
| `ze.buf.read.size` | `read-buffer-size` | `ze.bgp.reactor.read-buffer-size` |

## Hierarchy: Env Var Path Mirrors YANG Path

The env var dotted path should mirror the YANG tree path from the
component root down:

| YANG path | Env var |
|-----------|---------|
| `bgp / reactor / cache-ttl` | `ze.bgp.reactor.cache-ttl` |
| `bgp / reactor / forward-queue-size` | `ze.bgp.reactor.forward-queue-size` |
| `bgp / session / openwait` | `ze.bgp.session.openwait` |
| `hub / server / idle-timeout` | `ze.hub.server.idle-timeout` |

When the YANG tree changes (leaf moves to a different container), the env
var path changes too. The old path becomes an alias.

## Go Struct Fields

| Rule | Example |
|------|---------|
| PascalCase of the YANG leaf | `ForwardQueueSize` |
| Same word boundaries | `ReadBufferSize` (not `ReadBufSize`) |

## Container Naming

| Rule | Example | Anti-pattern |
|------|---------|-------------|
| Singular noun for the subsystem | `reactor` | `reactor-settings`, `reactor-config` |
| No `-config` or `-settings` suffix | `session` | `session-config` |
| Group related leaves, not one-per-container | `reactor { cache-ttl; cache-max; forward-queue-size; }` | `reactor-cache { ttl; max; }` + `reactor-forward { queue-size; }` |

## Naming New Settings (checklist)

```
[ ] YANG leaf: full words, kebab-case, no abbreviations
[ ] YANG leaf: unit suffix if ambiguous (seconds, bytes, count)
[ ] Env var: ze.<component>.<container>.<yang-leaf-name>
[ ] Env var leaf segment matches YANG leaf name exactly
[ ] Go struct: PascalCase of YANG leaf, same word boundaries
[ ] If legacy env var exists: alias registered matching new convention
[ ] Boolean: positive form (enabled, not disabled)
```
