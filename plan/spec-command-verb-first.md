# Spec: command-verb-first -- rename all CLI commands to verb-first grammar

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 9/9 |
| Updated | 2026-05-31 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/cli-grammar.md` - verb-first grammar rule
4. `ai/rules/plugin-design.md` - plugin SDK contract

## Task

All CLI commands must follow the grammar `<verb> <noun> [<action>] [<identifier>]` per `ai/rules/cli-grammar.md`. Currently, ~50 commands violate this rule by placing the component/noun before the verb (e.g., `sysctl show` instead of `show sysctl`, `bgp rib clear in` instead of `clear bgp rib in`).

This spec renames every violating command to verb-first form. There are two command registration paths: plugin commands (via `sdk.CommandDecl`) and built-in commands (via YANG RPC tree). Both must follow the grammar. Currently 26 top-level YANG containers exist, of which ~13 are nouns acting as top-level keywords instead of being nested under a verb.

**Relationship to spec-command-strip-prefix:** The strip-prefix spec assumed each plugin's commands share a common noun prefix (e.g., `sysctl *`). With verb-first naming, commands for the same plugin have different verb prefixes (`show sysctl`, `set sysctl`), making a single `CommandPrefix` per plugin unworkable. That spec should be abandoned after this one lands.

**Backward compatibility:** Per `ai/rules/cli-grammar.md`, old noun-first forms must be accepted with a deprecation warning logged once per session. Removed after two release cycles.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/cli-grammar.md` - verb-first grammar rule
  -> Constraint: `<verb> <noun> <action> [<identifier>]`. First token must be a verb keyword. Backward compat required with deprecation warning.
- [ ] `ai/rules/plugin-design.md` - plugin SDK contract, command dispatch
  -> Constraint: engine routes commands by prefix; the caller should not encode the destination.
- [ ] `ai/rules/derive-not-hardcode.md` - derive from registry, never re-hardcode
  -> Constraint: help text and command lists must derive from registration.

### RFC Summaries (MUST for protocol work)
- N/A - internal CLI change, not a wire protocol.

**Key insights:**
- 11 commands already verb-first (bmp, rr, static, sysrib, policyroute). ~50 need renaming.
- Inter-plugin dispatch calls (bgp-gr -> bgp-rib, rpki -> adj-rib-in, bmp -> bgp-rib) use hardcoded command strings that must also be updated.
- Functional tests (~100 dispatch calls in .ci files) reference current command names.
- Documentation (features.md, command-reference.md, rpki.md, operations.md) shows current command names.

## Verb Vocabulary

### Current top-level CLI keywords (from YANG tree)

There are 26 top-level YANG containers producing CLI keywords today. This is the full inventory:

| Current keyword | Type | What it does | Proposed disposition |
|----------------|------|-------------|---------------------|
| `show` | verb | read-only display | keep |
| `monitor` | verb | live streaming display | keep |
| `clear` | verb | clear state/counters | keep (absorbs `del`) |
| `set` | verb | modify runtime value | keep (absorbs `log set`) |
| `request` | verb | (proposed) operational actions | **add** (absorbs `subscribe`/`unsubscribe`) |
| `resolve` | verb | network lookup | keep |
| `commit` | verb | atomic route commit | keep |
| `update` | verb | pull fresh data / firmware | keep |
| `log` | verb | view/set log levels | **fold**: read -> `show log`, write -> `set log` |
| `del` | verb | delete config elements | **fold** into `clear` |
| `subscribe` | verb | subscribe to events | **fold** into `request subscribe` |
| `unsubscribe` | verb | unsubscribe from events | **fold** into `request unsubscribe` |
| `config` | noun | config archive trigger | move to `request config archive` or `show config archive` |
| `cache` | noun | manage BGP UPDATE cache | move: read-only -> `show cache`, actions -> `request cache` |
| `peer` | noun | BGP peer ops (detail, teardown, refresh) | move: read-only -> `show bgp peer`, actions -> `request bgp peer` |
| `rib` | noun | RIB queries and management | move: read-only -> `show bgp rib`, actions -> `request bgp rib` |
| `summary` | noun | BGP peer summary | move to `show bgp summary` |
| `interface` | noun | interface lifecycle | move: read-only -> `show interface`, mutations -> `set interface` |
| `plugin` | noun | plugin operations | move to `show plugin` |
| `system` | noun | system operations | move to `show system` or `request system` |
| `daemon` | noun | daemon lifecycle | move to `request daemon` |
| `event` | noun | event type discovery | move to `show event` |
| `command` | noun | command list | move to `show command` |
| `help` | noun | help text | move to `show help` (or keep as special) |
| `metrics` | noun | Prometheus metrics/pool stats | move to `show metrics` |
| `fakel2tp` | test noun | test plugin | move to `request fakel2tp` |
| `fakeredist` | test noun | test plugin | move to `request fakeredist` |

### Proposed verb set

| Verb | Purpose | Example |
|------|---------|---------|
| `show` | Query local state (read-only, one-shot) | `show bgp rib status` |
| `monitor` | Live streaming display | `monitor event`, `monitor bgp`, `monitor interface rate` |
| `clear` | Clear/remove state (absorbs `del`) | `clear bgp rib in`, `clear bgp peer <addr>` |
| `set` | Modify a value | `set sysctl net.ipv4.ip_forward 1`, `set log <logger> <level>` |
| `request` | Operational action (absorbs `subscribe`/`unsubscribe`) | `request bgp rib inject`, `request subscribe` |
| `resolve` | Network lookup (DNS, ASN, PeeringDB, IRR) | `resolve dns a <host>`, `resolve cymru asn-name <asn>` |
| `commit` | Atomic route commit | `commit` |
| `update` | Pull fresh data / manage firmware | `update bgp peer prefix`, `update system firmware check` |

Eight verbs. Key semantic distinctions: `show` is local state, `resolve` goes to the network, `commit` and `update` are self-describing root actions.

### Folding decisions

| Was | Becomes | Rationale |
|-----|---------|-----------|
| `log levels` | `show log levels` | read-only, local state |
| `log recent` | `show log recent` | read-only, local state |
| `log set <logger> <level>` | `set log <logger> <level>` | modifies a value |
| `subscribe` | `request subscribe` | operational action |
| `unsubscribe` | `request unsubscribe` | operational action |
| `del bgp peer` | `clear bgp peer` | removes state, same semantic as `clear` |
| `resolve dns a <host>` | `resolve dns a <host>` | already verb-first, keep (network lookup) |
| `commit` | `commit` | root verb, keep |
| `update` | `update` | root verb, keep |

### YANG built-in commands to rename (111 paths)

**Note:** `monitor` commands (7) and `show` commands (~95) are already verb-first. The 38 already-correct YANG paths are not listed here.

#### bfd (3) -- noun, move to `show bfd`

| Before | After | Verb |
|--------|-------|------|
| `bfd sessions` | `show bfd sessions` | show |
| `bfd session` | `show bfd session` | show |
| `bfd profile` | `show bfd profile` | show |

#### cache (1) -- noun, move to `show cache`

| Before | After | Verb |
|--------|-------|------|
| `cache` | `show cache` | show |

#### command (3) -- noun, move to `show command`

| Before | After | Verb |
|--------|-------|------|
| `command list` | `show command list` | show |
| `command help` | `show command help` | show |
| `command complete` | `show command complete` | show |

#### config (1) -- noun, move to `request config`

| Before | After | Verb |
|--------|-------|------|
| `config archive` | `request config archive` | request |

#### daemon (5) -- noun, split across `show`/`request`

| Before | After | Verb |
|--------|-------|------|
| `daemon status` | `show daemon status` | show |
| `daemon shutdown` | `request daemon shutdown` | request |
| `daemon reboot` | `request daemon reboot` | request |
| `daemon quit` | `request daemon quit` | request |
| `daemon reload` | `request daemon reload` | request |

#### del (1) -- fold into `clear`

| Before | After | Verb |
|--------|-------|------|
| `del bgp peer` | `clear bgp peer` | clear |

#### event (1) -- noun, move to `show event`

| Before | After | Verb |
|--------|-------|------|
| `event list` | `show event list` | show |

#### help (1) -- noun, move to `show help`

| Before | After | Verb |
|--------|-------|------|
| `help` | `show help` | show |

#### interface (13) -- noun, split across `set`/`clear`/`request`

| Before | After | Verb |
|--------|-------|------|
| `interface create-dummy` | `set interface create-dummy` | set |
| `interface create-veth` | `set interface create-veth` | set |
| `interface create-bridge` | `set interface create-bridge` | set |
| `interface up` | `set interface up` | set |
| `interface down` | `set interface down` | set |
| `interface mtu` | `set interface mtu` | set |
| `interface mac` | `set interface mac` | set |
| `interface addr-add` | `set interface addr-add` | set |
| `interface addr-del` | `set interface addr-del` | set |
| `interface unit-add` | `set interface unit-add` | set |
| `interface unit-del` | `set interface unit-del` | set |
| `interface delete` | `clear interface` | clear |
| `interface migrate` | `request interface migrate` | request |

#### l2tp (19) -- noun, split across `show`/`request`

| Before | After | Verb |
|--------|-------|------|
| `l2tp` | `show l2tp` | show |
| `l2tp tunnels` | `show l2tp tunnels` | show |
| `l2tp tunnel` | `show l2tp tunnel` | show |
| `l2tp sessions` | `show l2tp sessions` | show |
| `l2tp session` | `show l2tp session` | show |
| `l2tp statistics` | `show l2tp statistics` | show |
| `l2tp listeners` | `show l2tp listeners` | show |
| `l2tp config` | `show l2tp config` | show |
| `l2tp observer` | `show l2tp observer` | show |
| `l2tp cqm` | `show l2tp cqm` | show |
| `l2tp echo` | `show l2tp echo` | show |
| `l2tp reliable` | `show l2tp reliable` | show |
| `l2tp tunnel-history` | `show l2tp tunnel-history` | show |
| `l2tp session-history` | `show l2tp session-history` | show |
| `l2tp session-traffic` | `show l2tp session-traffic` | show |
| `l2tp tunnel teardown` | `request l2tp tunnel teardown` | request |
| `l2tp tunnel teardown-all` | `request l2tp tunnel teardown-all` | request |
| `l2tp session teardown` | `request l2tp session teardown` | request |
| `l2tp session teardown-all` | `request l2tp session teardown-all` | request |

#### log (3) -- fold into `show log`/`set log`

| Before | After | Verb |
|--------|-------|------|
| `log levels` | `show log levels` | show |
| `log recent` | `show log recent` | show |
| `log set` | `set log` | set |

#### metrics (3) -- noun, move to `show metrics`

| Before | After | Verb |
|--------|-------|------|
| `metrics values` | `show metrics values` | show |
| `metrics list` | `show metrics list` | show |
| `metrics pool` | `show metrics pool` | show |

#### peer (16) -- noun, move to `show bgp peer`/`request bgp peer`/`clear bgp peer`

| Before | After | Verb |
|--------|-------|------|
| `peer list` | `show bgp peer list` | show |
| `peer detail` | `show bgp peer detail` | show |
| `peer capabilities` | `show bgp peer capabilities` | show |
| `peer statistics` | `show bgp peer statistics` | show |
| `peer raw` | `show bgp peer raw` | show |
| `peer teardown` | `request bgp peer teardown` | request |
| `peer pause` | `request bgp peer pause` | request |
| `peer resume` | `request bgp peer resume` | request |
| `peer flush` | `request bgp peer flush` | request |
| `peer update` | `request bgp peer update` | request |
| `peer refresh` | `request bgp peer refresh` | request |
| `peer borr` | `request bgp peer borr` | request |
| `peer eorr` | `request bgp peer eorr` | request |
| `peer plugin session ready` | `request bgp peer plugin session ready` | request |
| `peer clear soft` | `clear bgp peer soft` | clear |

#### plugin (10) -- noun, split across `show plugin`/`request plugin`

| Before | After | Verb |
|--------|-------|------|
| `plugin encoding` | `show plugin encoding` | show |
| `plugin format` | `show plugin format` | show |
| `plugin ack` | `show plugin ack` | show |
| `plugin help` | `show plugin help` | show |
| `plugin command list` | `show plugin command list` | show |
| `plugin command help` | `show plugin command help` | show |
| `plugin command complete` | `show plugin command complete` | show |
| `plugin session ready` | `request plugin session ready` | request |
| `plugin session ping` | `request plugin session ping` | request |
| `plugin session bye` | `request plugin session bye` | request |

#### pppoe (5) -- noun, move to `show pppoe`

| Before | After | Verb |
|--------|-------|------|
| `pppoe` | `show pppoe` | show |
| `pppoe sessions` | `show pppoe sessions` | show |
| `pppoe session` | `show pppoe session` | show |
| `pppoe statistics` | `show pppoe statistics` | show |
| `pppoe interfaces` | `show pppoe interfaces` | show |

#### rib (9) -- noun, move to `show bgp rib`/`clear bgp rib`/`request bgp rib`

| Before | After | Verb |
|--------|-------|------|
| `rib status` | `show bgp rib status` | show |
| `rib routes` | `show bgp rib routes` | show |
| `rib best` | `show bgp rib best` | show |
| `rib best status` | `show bgp rib best status` | show |
| `rib rpf` | `show bgp rib rpf` | show |
| `rib clear in` | `clear bgp rib in` | clear |
| `rib clear out` | `clear bgp rib out` | clear |
| `rib inject` | `request bgp rib inject` | request |
| `rib withdraw` | `request bgp rib withdraw` | request |

#### subscribe/unsubscribe (2) -- fold into `request`

| Before | After | Verb |
|--------|-------|------|
| `subscribe` | `request subscribe` | request |
| `unsubscribe` | `request unsubscribe` | request |

#### subscriber (2) -- noun, move to `show subscriber`

| Before | After | Verb |
|--------|-------|------|
| `subscriber` | `show subscriber` | show |
| `subscriber detail` | `show subscriber detail` | show |

#### summary (1) -- noun, move to `show bgp summary`

| Before | After | Verb |
|--------|-------|------|
| `summary` | `show bgp summary` | show |

#### system (8) -- noun, split across `show system`/`request system`

| Before | After | Verb |
|--------|-------|------|
| `system help` | `show system help` | show |
| `system version software` | `show system version software` | show |
| `system version api` | `show system version api` | show |
| `system subsystem list` | `show system subsystem list` | show |
| `system command list` | `show system command list` | show |
| `system command help` | `show system command help` | show |
| `system command complete` | `show system command complete` | show |
| `system dispatch` | `request system dispatch` | request |

#### test plugins (5) -- noun, split across `request`/`show`

| Before | After | Verb |
|--------|-------|------|
| `fakel2tp emit` | `request fakel2tp emit` | request |
| `fakel2tp help` | `show fakel2tp help` | show |
| `fakeredist emit` | `request fakeredist emit` | request |
| `fakeredist emit-burst` | `request fakeredist emit-burst` | request |
| `fakeredist help` | `show fakeredist help` | show |

## Command Rename Table

### Already correct (no change)

| Command | Plugin | File |
|---------|--------|------|
| `show bmp sessions` | bmp | `bgp/plugins/bmp/bmp.go` |
| `show bmp peers` | bmp | `bgp/plugins/bmp/bmp.go` |
| `show bmp collectors` | bmp | `bgp/plugins/bmp/bmp.go` |
| `show bmp rib` | bmp | `bgp/plugins/bmp/bmp.go` |
| `show rr status` | rr | `bgp/plugins/rr/rr.go` |
| `show rr peers` | rr | `bgp/plugins/rr/rr.go` |
| `show policy-routes` | policyroute | `plugins/policyroute/register.go` |
| `show static` | static | `plugins/static/register.go` |
| `show rib` | sysrib | `plugins/sysrib/register.go` |
| `show nexthop-table` | sysrib | `plugins/sysrib/register.go` |
| `show ecmp-groups` | sysrib | `plugins/sysrib/register.go` |

### adj-rib-in (7 commands)

| Before | After | Verb |
|--------|-------|------|
| `adj-rib-in status` | `show adj-rib-in status` | show |
| `adj-rib-in show` | `show adj-rib-in` | show |
| `adj-rib-in replay` | `request adj-rib-in replay` | request |
| `adj-rib-in enable-validation` | `request adj-rib-in enable-validation` | request |
| `adj-rib-in accept-routes` | `request adj-rib-in accept-routes` | request |
| `adj-rib-in reject-routes` | `request adj-rib-in reject-routes` | request |
| `adj-rib-in revalidate` | `request adj-rib-in revalidate` | request |

Cross-plugin callers to update:
- `rpki.go:215` -- `DispatchCommand("adj-rib-in enable-validation")` -> `"request adj-rib-in enable-validation"`

### healthcheck (2 commands)

| Before | After | Verb |
|--------|-------|------|
| `healthcheck show` | `show healthcheck` | show |
| `healthcheck reset` | `clear healthcheck` | clear |

### bgp-rib (22 commands)

| Before | After | Verb | Notes |
|--------|-------|------|-------|
| `bgp rib status` | `show bgp rib status` | show | |
| `bgp rib show` | `show bgp rib` | show | main route display |
| `bgp rib show best` | `show bgp rib best` | show | |
| `bgp rib show best status` | `show bgp rib best status` | show | |
| `bgp rib show-protocol` | `show bgp rib protocol` | show | drops embedded `show-` |
| `bgp rib adjacent status` | `show bgp rib adjacent status` | show | |
| `bgp rib help` | `show bgp rib help` | show | |
| `bgp rib command list` | `show bgp rib commands` | show | |
| `bgp rib event list` | `show bgp rib events` | show | |
| `bgp rib rpf` | `show bgp rib rpf` | show | RPF lookup |
| `bgp rib clear in` | `clear bgp rib in` | clear | |
| `bgp rib clear out` | `clear bgp rib out` | clear | |
| `bgp rib inject` | `request bgp rib inject` | request | |
| `bgp rib withdraw` | `request bgp rib withdraw` | request | |
| `bgp rib withdraw-protocol` | `request bgp rib withdraw-protocol` | request | |
| `bgp rib withdraw-router` | `request bgp rib withdraw-router` | request | |
| `bgp rib retain-routes` | `request bgp rib retain-routes` | request | internal, from bgp-gr |
| `bgp rib release-routes` | `request bgp rib release-routes` | request | internal, from bgp-gr |
| `bgp rib mark-stale` | `request bgp rib mark-stale` | request | internal, from bgp-gr |
| `bgp rib purge-stale` | `request bgp rib purge-stale` | request | internal, from bgp-gr |

Aliases (registered alongside primary commands):
| Before | After | Primary |
|--------|-------|---------|
| `bgp rib adjacent inbound empty` | `clear bgp rib adjacent inbound` | `clear bgp rib in` |
| `bgp rib adjacent outbound resend` | `clear bgp rib adjacent outbound` | `clear bgp rib out` |

Cross-plugin callers to update:
- `gr/gr.go:123` -- `"bgp rib mark-stale "` -> `"request bgp rib mark-stale "`
- `gr/gr.go:136` -- `"bgp rib purge-stale "` -> `"request bgp rib purge-stale "`
- `gr/gr.go:139` -- `"bgp rib release-routes "` -> `"request bgp rib release-routes "`
- `gr/gr.go:344` -- `"bgp rib purge-stale "` -> `"request bgp rib purge-stale "`
- `gr/gr.go:345` -- `"bgp rib retain-routes "` -> `"request bgp rib retain-routes "`
- `gr/gr.go:346` -- `"bgp rib mark-stale "` -> `"request bgp rib mark-stale "`
- `gr/gr.go:357` -- `"bgp rib purge-stale "` -> `"request bgp rib purge-stale "`
- `gr/gr.go:503` -- `"bgp rib purge-stale "` -> `"request bgp rib purge-stale "`
- `gr/gr.go:505` -- `"bgp rib retain-routes "` -> `"request bgp rib retain-routes "`
- `gr/gr.go:507` -- `"bgp rib mark-stale "` -> `"request bgp rib mark-stale "`
- `gr/gr.go:519` -- `"bgp rib purge-stale "` -> `"request bgp rib purge-stale "`
- `gr/gr.go:550` -- `"bgp rib purge-stale "` -> `"request bgp rib purge-stale "`
- `gr/gr.go:567` -- `"bgp rib release-routes "` -> `"request bgp rib release-routes "`
- `bmp/bmp.go:408` -- `"bgp rib withdraw-router bmp "` -> `"request bgp rib withdraw-router bmp "`
- `bmp/bmp.go:540` -- `"bgp rib withdraw-router bmp "` -> `"request bgp rib withdraw-router bmp "`

### rpki (6 commands)

| Before | After | Verb |
|--------|-------|------|
| `rpki status` | `show rpki status` | show |
| `rpki cache` | `show rpki cache` | show |
| `rpki roa` | `show rpki roa` | show |
| `rpki summary` | `show rpki summary` | show |
| `rpki validate` | `request rpki validate` | request |
| `rpki aspa` | `show rpki aspa` | show |

### rs (2 commands)

| Before | After | Verb |
|--------|-------|------|
| `rs status` | `show rs status` | show |
| `rs peers` | `show rs peers` | show |

### watchdog (2 commands)

| Before | After | Verb |
|--------|-------|------|
| `watchdog announce` | `request watchdog announce` | request |
| `watchdog withdraw` | `request watchdog withdraw` | request |

### ldp (2 commands)

| Before | After | Verb | Notes |
|--------|-------|------|-------|
| `ldp show-neighbor` | `show ldp neighbor` | show | drops embedded `show-` |
| `ldp show-binding` | `show ldp binding` | show | drops embedded `show-` |

### rsvp-te (3 commands)

| Before | After | Verb | Notes |
|--------|-------|------|-------|
| `rsvp-te show-session` | `show rsvp-te session` | show | drops embedded `show-` |
| `rsvp-te show-interface` | `show rsvp-te interface` | show | drops embedded `show-` |
| `rsvp-te show-tunnel` | `show rsvp-te tunnel` | show | drops embedded `show-` |

### fib-kernel (1 command)

| Before | After | Verb |
|--------|-------|------|
| `fib-kernel show` | `show fib-kernel` | show |

### fib-p4 (1 command)

| Before | After | Verb |
|--------|-------|------|
| `fib-p4 show` | `show fib-p4` | show |

### fib-vpp (1 command)

| Before | After | Verb |
|--------|-------|------|
| `fib-vpp show` | `show fib-vpp` | show |

### l2tp-pool (1 command)

| Before | After | Verb |
|--------|-------|------|
| `l2tp pool show` | `show l2tp pool` | show |

### l2tp-shaper (1 command)

| Before | After | Verb |
|--------|-------|------|
| `l2tp shaper show` | `show l2tp shaper` | show |

### sysctl (6 commands)

| Before | After | Verb |
|--------|-------|------|
| `sysctl show` | `show sysctl` | show |
| `sysctl list` | `show sysctl keys` | show |
| `sysctl describe` | `show sysctl key` | show |
| `sysctl set` | `set sysctl` | set |
| `sysctl list-profiles` | `show sysctl profiles` | show |
| `sysctl describe-profile` | `show sysctl profile` | show |

### Test plugins (7 commands)

| Before | After | Verb |
|--------|-------|------|
| `fakefib emit` | `request fakefib emit` | request |
| `fakefib help` | `show fakefib help` | show |
| `fakel2tp emit` | `request fakel2tp emit` | request |
| `fakel2tp help` | `show fakel2tp help` | show |
| `fakeredist emit` | `request fakeredist emit` | request |
| `fakeredist emit-burst` | `request fakeredist emit-burst` | request |
| `fakeredist help` | `show fakeredist help` | show |

### Summary by verb

| Verb | Plugin commands | YANG built-in commands |
|------|---------------|----------------------|
| `show` | ~37 (all read-only plugin commands) | log levels, log recent, cache, peer detail/list/caps/stats/history, rib status/routes/best, summary, interface, plugin, system, event, command, help, metrics |
| `monitor` | 0 | event, bgp, interface rate, system netlink, vpn ipsec, ping (already correct) |
| `clear` | 5 (bgp-rib in/out/adj-in/adj-out, healthcheck) | interface counters, dns cache, vpn ipsec sa, bgp peer (was `del`) |
| `set` | 1 (sysctl) | log level, system file-descriptors, bgp peer with/save |
| `request` | ~18 (adj-rib-in 5, bgp-rib 8, watchdog 2, rpki 1, test 3) | subscribe, unsubscribe, config archive, daemon, bgp peer teardown/pause/resume/flush/refresh/borr/eorr/clear-soft |
| `resolve` | 0 | dns a/aaaa/txt/ptr, cymru asn-name (already correct) |
| `commit` | 0 | commit (already correct) |
| `update` | 0 | bgp peer prefix, system firmware check/download/apply/restart/rollback (already correct) |

## Backward Compatibility

Per `ai/rules/cli-grammar.md`:
1. Register new verb-first names as primary
2. Keep old noun-first names as deprecated aliases
3. Log deprecation warning once per session on first use of old form
4. Remove old forms after two release cycles

Implementation: the command registry accepts both forms. When the old form matches, the registry logs a warning and dispatches normally.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/command_registry.go` - CommandRegistry stores RegisteredCommand by lowered name, dispatches on longest prefix match
- [ ] `internal/component/plugin/server/command.go` - Dispatcher routes commands to plugins via routeToProcess
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - RIB internal dispatch table keyed by full command name
- [ ] `internal/component/bgp/plugins/gr/gr.go` - GR plugin dispatches inter-plugin commands with hardcoded strings

**Behavior to preserve:**
- Command output format unchanged
- Args extraction unchanged (text after matched command prefix)
- Help/list output shows the new verb-first names
- Command completion works with new names
- Inter-plugin dispatch works after string updates

**Behavior to change:**
- Every violating command name renamed to verb-first
- Old names become deprecated aliases
- Deprecation warning logged on old-form usage

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- User types CLI command (e.g., `show bgp rib status`)
- Dispatcher receives input string

### Transformation Path
1. `Dispatch()` lowercases input, calls `dispatchPlugin()`
2. `dispatchPlugin()` finds longest matching `RegisteredCommand` by prefix
3. Everything after matched prefix becomes `args`
4. `routeToProcess()` sends command name to plugin via RPC
5. Plugin's `OnExecuteCommand` receives command, switches on it

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI input -> Dispatcher | Full input string | [ ] |
| Dispatcher -> RPC wire | Matched command name | [ ] |
| RPC wire -> Plugin handler | Command in `command` param | [ ] |
| Plugin -> Plugin (DispatchCommand) | Full command string | [ ] |

### Integration Points
- `RegisteredCommand.Name` (`command_registry.go`) - stores the full command name
- `routeToProcess` (`command.go`) - sends command name to plugin
- RIB `registeredCommands` (`rib_commands.go`) - secondary dispatch table keyed by command name
- GR `dispatchCommand` (`gr/gr.go`) - hardcoded command strings for inter-plugin dispatch
- RPKI `DispatchCommand` (`rpki/rpki.go:215`) - hardcoded command for adj-rib-in
- BMP `DispatchCommand` (`bmp/bmp.go:408,540`) - hardcoded command for bgp-rib

### Architectural Verification
- [ ] No bypassed layers (dispatch still routes, plugin still handles)
- [ ] No unintended coupling (no new imports)
- [ ] No duplicated functionality (renaming existing commands)
- [ ] Zero-copy preserved (string constants, not dynamic)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| CLI input `show bgp rib status` | -> | RIB handleCommand receives `show bgp rib status` | `TestVerbFirstRIBDispatch` |
| CLI input `show sysctl` | -> | sysctl handleCommand receives `show sysctl` | `TestVerbFirstSysctlDispatch` |
| Deprecated input `bgp rib status` | -> | Dispatches correctly + logs warning | `TestDeprecatedCommandWarning` |
| GR dispatches `retain bgp rib routes` | -> | RIB retainRoutesCommand runs | `TestGRInterPluginVerbFirst` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | All `show` commands renamed | Commands like `show bgp rib status`, `show sysctl`, `show rpki cache` dispatch correctly |
| AC-2 | All `clear` commands renamed | `clear bgp rib in`, `clear bgp rib out` dispatch correctly |
| AC-3 | All `set` commands renamed | `set sysctl <key> <value>` dispatches correctly |
| AC-4 | All `request` commands renamed | `request bgp rib inject`, `request adj-rib-in replay`, `request watchdog announce`, etc. dispatch correctly |
| AC-5 | Old names accepted with warning | `sysctl show` dispatches correctly, logs deprecation warning once |
| AC-6 | Inter-plugin dispatch updated | GR -> RIB commands use new names, RPKI -> adj-rib-in uses new name, BMP -> RIB uses new name |
| AC-7 | Help/list output shows new names | `show bgp rib help` and `show bgp rib commands` list verb-first names |
| AC-8 | All functional tests updated | All .ci files use new command names |
| AC-9 | All documentation updated | features.md, command-reference.md, rpki.md, operations.md use new names |
| AC-10 | `make ze-verify` passes | Full build, lint, tests |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVerbFirstRIBDispatch` | `rib_commands_test.go` | RIB dispatch table uses verb-first keys | |
| `TestVerbFirstSysctlDispatch` | `sysctl/register_test.go` | sysctl handler switches on verb-first names | |
| `TestDeprecatedCommandWarning` | `command_registry_test.go` | Old name dispatches + logs warning | |
| `TestGRInterPluginVerbFirst` | `gr/gr_test.go` | GR dispatches verb-first command strings | |

### Boundary Tests (MANDATORY for numeric inputs)
- N/A - no numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| All existing .ci tests | `test/plugin/*.ci` | Updated to use verb-first names | |

### Interop Tests (MANDATORY for protocol features)
- N/A - internal CLI change, not a wire protocol.

### Future (if deferring any tests)
- Deprecation warning functional test (separate .ci) -- can be deferred if unit test covers it.

## Files to Modify

### Command registration (Name fields)
- `internal/component/bgp/plugins/adj_rib_in/rib.go` - 7 command Names
- `internal/component/bgp/plugins/healthcheck/healthcheck.go` - 2 command Names
- `internal/component/bgp/plugins/rib/rib.go` - 18 command Names
- `internal/component/bgp/plugins/rpki/rpki.go` - 6 command Names
- `internal/component/bgp/plugins/rr/rr.go` - already correct
- `internal/component/bgp/plugins/rs/server.go` - 2 command Names
- `internal/component/bgp/plugins/watchdog/watchdog.go` - 2 command Names
- `internal/component/bgp/plugins/bmp/bmp.go` - already correct
- `internal/component/ldp/register.go` - 2 command Names
- `internal/component/rsvpte/register.go` - 3 command Names
- `internal/plugins/fib/kernel/register.go` - 1 command Name
- `internal/plugins/fib/p4/register.go` - 1 command Name
- `internal/plugins/fib/vpp/register.go` - 1 command Name
- `internal/plugins/l2tppool/register.go` - 1 command Name
- `internal/plugins/l2tpshaper/register.go` - 1 command Name
- `internal/plugins/sysctl/register.go` - 6 command Names
- `internal/plugins/sysrib/register.go` - already correct
- `internal/plugins/static/register.go` - already correct
- `internal/plugins/policyroute/register.go` - already correct
- `internal/test/plugins/fakefib/register.go` - 2 command Names
- `internal/test/plugins/fakel2tp/register.go` - 2 command Names
- `internal/test/plugins/fakeredist/register.go` - 3 command Names

### Command handlers (switch/if statements)
- `internal/component/bgp/plugins/adj_rib_in/rib_commands.go` - 7 case labels
- `internal/component/bgp/plugins/healthcheck/healthcheck.go` - 2 case labels
- `internal/component/bgp/plugins/rib/rib_commands.go` - registeredCommands map keys (~22 entries)
- `internal/component/bgp/plugins/rib/rib_inject.go` - 3 command entries
- `internal/component/bgp/plugins/rpki/rpki.go` - 6 case labels
- `internal/component/bgp/plugins/rs/server_handlers.go` - 2 case labels
- `internal/component/bgp/plugins/watchdog/server.go` - 2 if conditions
- `internal/component/ldp/register.go` - 2 case labels
- `internal/component/rsvpte/register.go` - 3 case labels
- `internal/plugins/fib/kernel/register.go` - 1 if condition
- `internal/plugins/fib/p4/register.go` - 1 if condition
- `internal/plugins/fib/vpp/register.go` - 1 if condition
- `internal/plugins/l2tppool/register.go` - 1 if condition
- `internal/plugins/l2tpshaper/register.go` - 1 if condition
- `internal/plugins/sysctl/register.go` - 6 case labels
- `internal/test/plugins/fakefib/fakefib.go` - 2 if conditions
- `internal/test/plugins/fakel2tp/fakel2tp.go` - 2 if conditions
- `internal/test/plugins/fakeredist/fakeredist.go` - 3 if conditions

### Inter-plugin dispatch callers
- `internal/component/bgp/plugins/gr/gr.go` - 13 DispatchCommand string literals
- `internal/component/bgp/plugins/rpki/rpki.go:215` - 1 DispatchCommand string literal
- `internal/component/bgp/plugins/bmp/bmp.go:408,540` - 2 DispatchCommand string literals

### Backward compatibility (deprecation aliases)
- `internal/component/plugin/server/command_registry.go` - deprecation alias mechanism

### Functional tests (~100+ dispatch calls)
- `test/plugin/rpf-multicast.ci` - bgp rib inject, bgp rib rpf
- `test/plugin/firewall-global-options.ci` - sysctl show
- `test/plugin/fib-recursive.ci` - bgp rib status, bgp rib inject, show rib, show nexthop-table, show ecmp-groups
- `test/plugin/rib-pipe-filter.ci` - bgp rib status, bgp rib show (many variants)
- `test/plugin/api-rib-clear-in.ci` - bgp rib clear in
- `test/plugin/api-rib-clear-out.ci` - bgp rib clear out
- `test/plugin/api-rib-inject.ci` - bgp rib inject, bgp rib show
- `test/plugin/api-rib-withdraw.ci` - bgp rib inject, bgp rib withdraw, bgp rib show
- `test/plugin/api-rib-show-in.ci` - bgp rib routes received
- `test/plugin/api-rib-show-out.ci` - bgp rib routes sent
- `test/plugin/sysctl-show-describe.ci` - sysctl show, sysctl list, sysctl describe
- `test/plugin/sysctl-list.ci` - sysctl list
- `test/plugin/rpki-cache-connect.ci` - rpki status
- `test/plugin/rpki-cache-update.ci` - rpki roa, bgp rib routes
- `test/plugin/rpki-validate-accept.ci` - bgp rib routes
- `test/plugin/rpki-validate-reject.ci` - bgp rib routes
- `test/plugin/rpki-validate-notfound.ci` - bgp rib routes
- `test/plugin/community-strip.ci` - adj-rib-in status
- `test/plugin/community-tag.ci` - adj-rib-in status, bgp rib show
- `test/plugin/community-cumulative.ci` - adj-rib-in status, bgp rib show
- `test/plugin/community-priority.ci` - adj-rib-in status, bgp rib show
- `test/plugin/prefix-filter-accept.ci` - adj-rib-in status
- `test/plugin/prefix-filter-reject.ci` - adj-rib-in status
- `test/plugin/policy-show-list.ci` - adj-rib-in status
- `test/plugin/role-otc-egress-stamp.ci` - adj-rib-in status
- `test/plugin/role-otc-egress-filter.ci` - adj-rib-in status
- `test/plugin/role-otc-ingress-reject.ci` - adj-rib-in status
- `test/plugin/role-otc-unicast-scope.ci` - adj-rib-in status
- `test/plugin/role-otc-export-unknown.ci` - adj-rib-in status
- `test/plugin/adj-rib-in-replay-on-peerup.ci` - adj-rib-in status, adj-rib-in replay
- `test/plugin/forward-two-tier-under-load.ci` - adj-rib-in status
- `test/plugin/forward-overflow-two-tier.ci` - adj-rib-in status
- `test/plugin/show-rr-status.ci` - show rr status, show rr peers (already correct)
- `test/plugin/show-bmp-sessions.ci` - show bmp sessions, show bmp collectors (already correct)
- `test/plugin/bmp-lg-ingest.ci` - show bmp rib (already correct)
- `test/plugin/bmp-lg-disconnect.ci` - show bmp rib (already correct)
- `test/plugin/bmp-lg-bestpath-isolation.ci` - show bmp rib, bgp rib show, bgp rib show best
- `test/plugin/bgp-redistribute-announce.ci` - fakeredist emit
- `test/plugin/bgp-redistribute-burst.ci` - fakeredist emit-burst
- `test/plugin/bgp-redistribute-explicit-nhop.ci` - fakeredist emit
- `test/plugin/bgp-redistribute-filtered-out.ci` - fakeredist emit
- `test/plugin/bgp-redistribute-nexthop-self.ci` - fakeredist emit
- `test/plugin/redistribute-l2tp-announce.ci` - fakel2tp emit
- `test/plugin/redistribute-l2tp-not-configured.ci` - fakel2tp emit
- `test/plugin/redistribute-l2tp-multi-peer-nexthop.ci` - fakel2tp emit
- `test/plugin/fib-table.ci` - fakefib emit
- `test/plugin/fib-mpls-kernel.ci` - fakefib emit
- `test/plugin/fib-srv6-kernel.ci` - fakefib emit
- `test/plugin/fib-blackhole.ci` - fakefib emit
- `test/plugin/fib-sysrib.ci` - bgp rib status, bgp rib inject, show rib
- `test/plugin/fib-metric.ci` - bgp rib status, bgp rib inject, show rib
- `test/plugin/fib-ecmp.ci` - bgp rib status, bgp rib show best, show rib, bgp rib inject, bgp rib withdraw, show ecmp-groups
- `test/plugin/fib-rib-event.ci` - bgp rib status, bgp rib inject, bgp rib show best
- `test/plugin/rib-best-selection.ci` - bgp rib status, bgp rib inject, bgp rib show best
- `test/plugin/rib-show-filter.ci` - bgp rib show (many variants)
- `test/plugin/rib-graph.ci` - bgp rib status, bgp rib inject, bgp rib show
- `test/plugin/rib-graph-best.ci` - bgp rib status, bgp rib inject, bgp rib show best
- `test/plugin/rib-graph-filtered.ci` - bgp rib status, bgp rib inject, bgp rib show
- `test/plugin/rib-forward-handle-observed.ci` - bgp rib show best
- `test/plugin/rib-clear-out-family.ci` - bgp rib clear out
- `test/plugin/multipath-basic.ci` - bgp rib inject, bgp rib show best
- `test/plugin/nexthop-self.ci` - bgp rib show best
- `test/plugin/nexthop-unchanged.ci` - bgp rib show best
- `test/plugin/rr-basic.ci` - bgp rib show best
- `test/plugin/bestpath-reason.ci` - bgp rib inject, bgp rib show best
- `test/plugin/dispatch-command-single-decode.ci` - bgp rib status, bgp rib clear in
- `test/plugin/test-pipe-first-last.ci` - bgp rib routes
- `test/plugin/config-adj-rib.ci` - bgp rib routes
- `test/plugin/rpki-as-set.ci` - bgp rib routes
- `test/plugin/rpki-passthrough.ci` - bgp rib routes
- `test/plugin/rpki-maxlength.ci` - bgp rib routes
- `test/plugin/rpki-multi-prefix.ci` - bgp rib routes
- `test/plugin/rpki-timeout.ci` - bgp rib routes
- `test/plugin/rpki-aspa-policy-logonly.ci` - bgp rib routes
- `test/plugin/rpki-aspa-policy-reject.ci` - bgp rib routes
- `test/plugin/rpki-aspa-policy-unknown-reject.ci` - bgp rib routes
- `test/parse/sysctl-list-profiles.ci` - ze sysctl list-profiles (offline CLI)

### Documentation
- `docs/features.md:17` - sysctl CLI names
- `docs/features.md:25` - `bgp rib rpf`
- `docs/guide/command-reference.md:1583-1665` - bgp rib, healthcheck, sysctl command tables
- `docs/guide/rpki.md:116-124` - rpki CLI commands and examples
- `docs/guide/rpki.md:308` - `adj-rib-in enable-validation` reference
- `docs/guide/rpki.md:316-317` - rpki troubleshooting commands
- `docs/guide/operations.md:322` - `rpki status` example
- `docs/guide/api.md:91` - `bgp rib routes` API example

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | - |
| YANG validation constraints | No | - |
| YANG custom validators | No | - |
| CLI commands/flags | Yes | All command registration files above |
| CLI grammar (action before identifier) | Yes | This is the spec enforcing that rule |
| Editor autocomplete | No | Completion uses registered names |
| Functional test for new RPC/API | No | Existing tests updated |
| Pipe completeness | No | Pipes unaffected |
| Env var registration | No | - |
| Doctor check for runtime dependencies | No | - |
| Prometheus counters/metrics | No | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` - rename all command names in tables |
| 4 | API/RPC added/changed? | Yes | `docs/guide/api.md:91` - update `bgp rib routes` example |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/rpki.md` - rename rpki CLI commands and examples |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | Command names change but SDK contract unchanged |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | Yes | Command names changed in registry |
| 16 | Any changed source file is referenced by existing doc source anchors? | Yes | `docs/features.md` has source anchors referencing sysctl, rib, rpki files |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/command-reference.md`, `docs/guide/rpki.md`, `docs/guide/operations.md` |

## Files to Create
- None (all changes are renames in existing files).

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Deprecation mechanism** -- add deprecated alias support to CommandRegistry
   - Tests: `TestDeprecatedCommandWarning`
   - Files: `command_registry.go`
   - Verify: old names dispatch correctly and log warning

2. **Phase: show commands** -- rename all `show` verb commands (~37)
   - Tests: `TestVerbFirstRIBDispatch`, `TestVerbFirstSysctlDispatch`
   - Files: all registration and handler files for show commands
   - Verify: all show commands dispatch with new names, old names accepted with warning

3. **Phase: clear/set/reset commands** -- rename `clear`, `set`, `reset` commands (~6)
   - Files: bgp-rib (clear in/out), sysctl (set), healthcheck (reset)
   - Verify: all dispatch correctly

4. **Phase: action commands** -- rename domain verb commands (~15)
   - Files: adj-rib-in (replay, enable, accept, reject, revalidate), bgp-rib (inject, withdraw, retain, release, mark, purge), watchdog (announce, withdraw), rpki (validate)
   - Verify: all dispatch correctly

5. **Phase: inter-plugin dispatch** -- update DispatchCommand string literals
   - Files: gr/gr.go, rpki/rpki.go, bmp/bmp.go
   - Verify: inter-plugin commands dispatch correctly

6. **Phase: functional tests** -- update all .ci file dispatch calls
   - Files: ~70 .ci files
   - Verify: all functional tests pass

7. **Phase: documentation** -- update all docs
   - Files: features.md, command-reference.md, rpki.md, operations.md, api.md
   - Verify: no stale command names in docs

8. **Phase: offline CLI** -- update ze subcommand dispatch if affected
   - Files: check `cmd/ze/` and `test/parse/` for offline command references
   - Verify: offline CLI commands work

9. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every command in the rename table has been updated in both registration and handler |
| Correctness | CLI output identical before and after (only command names change) |
| Naming | All verb-first names follow `<verb> <noun> [<action>]` grammar |
| Data flow | Inter-plugin dispatch strings all updated |
| Deprecation | Old names accepted with warning, not silently dropped |
| No stale refs | `grep -rn` for old command strings returns zero hits in non-deprecation code |
| Docs | No old command names remain in documentation |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| All commands verb-first | `grep -rn 'Name: "' --include='*.go' internal/ \| grep '{Name'` shows verb-first names |
| No old names in handlers | `grep -rn '"adj-rib-in show"\|"sysctl show"\|"bgp rib status"\|"rpki status"' internal/` returns 0 in handler code |
| Old names as deprecated aliases | Unit test `TestDeprecatedCommandWarning` passes |
| All .ci files updated | `grep -rn "dispatch.*'adj-rib-in \|dispatch.*'sysctl \|dispatch.*'bgp rib \|dispatch.*'rpki " test/` returns 0 |
| All docs updated | `grep -n 'sysctl show\|adj-rib-in \|bgp rib status\|rpki status' docs/` returns 0 |
| GR dispatch strings updated | `grep -n 'bgp rib mark-stale\|bgp rib retain\|bgp rib release\|bgp rib purge' internal/component/bgp/plugins/gr/` returns 0 |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Command names are compile-time constants. No injection risk. |
| Deprecation bypass | Old names must not bypass any authorization or audit checks. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior, RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural, DESIGN phase |
| Functional test fails | Check AC; if AC wrong, DESIGN; if AC correct, IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| spec-command-strip-prefix (CommandPrefix per plugin) | verb-first naming means commands for the same plugin have different verb prefixes; single CommandPrefix doesn't work | This spec: rename to verb-first first |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## Core Insight

The strip-prefix spec assumed each plugin's commands share a common noun prefix. With verb-first grammar, commands for the same plugin share a noun but have different verb prefixes (`show sysctl`, `set sysctl`). The grammar fix must come first; then prefix stripping (if still wanted) operates on the verb+noun prefix, which varies per verb.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| 8 root verbs | Option B (18+ domain verbs) | Too many keywords. 8 verbs: `show`, `monitor`, `clear`, `set`, `request`, `resolve`, `commit`, `update` |
| `log` folds into `show`/`set` | keep `log` as own verb | `log` is mixed read/write; clean split into `show log` + `set log` |
| `del` folds into `clear` | keep `del` as own verb | `clear` already means remove state; `del bgp peer` -> `clear bgp peer` |
| `subscribe`/`unsubscribe` fold into `request` | keep as own verbs | operational actions belong under `request` |
| `resolve` stays (network lookups) | fold into `show` | `show` = local, `resolve` = network; meaningful semantic distinction |
| `commit`, `update` stay as root verbs | fold into `request` | self-describing actions that stand on their own |
| Backward compat via deprecated aliases | Breaking change (no compat) | cli-grammar.md requires deprecation warnings, remove after 2 cycles |
| Rename internal commands (GR -> RIB) | Leave internal commands noun-first | cli-grammar rule says all CLI commands, no exceptions; internal commands also appear in help/list |

## Known Limitations
- Does not implement prefix stripping (that is a separate concern for after this lands)
- Deprecation aliases add temporary complexity to the command registry
- ~70 functional tests need mechanical updates

## RFC Documentation

N/A - no protocol work.

## Implementation Summary

### What Was Implemented
- [To be filled after implementation]

### Bugs Found/Fixed
- [To be filled after implementation]

### Documentation Updates
- [To be filled after implementation]

### Deviations from Plan
- [To be filled after implementation]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| All commands verb-first | grep | `grep -rn 'Name: "' internal/ \| grep '{Name'` shows no noun-first commands |
| Old names still work | unit test | `TestDeprecatedCommandWarning` |
| CLI output unchanged | functional test | All existing .ci tests pass |
| Docs updated | grep | No old command names in docs/ |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [To be filled]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (Review Gate section filled)
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-command-verb-first.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-command-verb-first.md`
