# Spec: firewall-remote-group

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Depends | - |
| Phase | - |
| Updated | 2026-09-02 |

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     The file's DIRECTORY carries the release bucket: plan/immediate/ for a defect
     an operator meets, plan/pre-release/ for work the release cannot go out
     without, plan/ for everything else (plan/README.md). -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze's firewall exposes nftables named sets whose members are written literally in
config (`list set` in
`internal/component/firewall/yang/ze-firewall-conf.yang`, lowered by `lowerSet`
in `internal/plugins/firewall/nft/lower_linux.go`). There is no way to populate a
set from an external list.

Ze has no URL-sourced firewall group and no FQDN or domain group. No firewall
tree performs an HTTP fetch or a DNS resolution for set membership. The only
timer-refreshed firewall data is IRR-derived (`irrPlugin.refreshLoop`,
`internal/component/firewall/plugins/irr/irr.go`), driven by a single global
`firewall irr refresh-interval` that refreshes every reference at once.

Operators publish blocklists, bogon lists and threat feeds as plain address or
prefix lists over HTTP. Today the only way to use one in Ze is to regenerate
config out of band.

Goal: add a firewall group whose members are downloaded from a URL, cached on
disk, reloaded into its nftables set on a timer, and refreshed on a per-group
interval that falls back to a global default. A failed download must keep serving
the cached list, and must retry sooner than the full group interval rather than
waiting it out.

Related, and deliberately out of scope here:

- `plan/immediate/spec-firewall-dynamic-address-group.md` covers packet-triggered set
  population through nftables `dynset`. That is a different mechanism for a
  different purpose, and it also owns finishing the inert `flags-dynamic`
  lowering.
- FQDN and domain groups are a third mechanism. They are not in scope for this
  spec.

Provenance: VyOS T9076 added a per-group `interval` on top of an existing
`firewall group remote-group <name> url <url>` feature. Ze needs both halves.

## Scope change, 2026-09-02 (owner directive)

Everything above is preserved as written on 2026-08-01. Two things changed, and
both WIDEN this spec.

**One plugin serves both sources, not two plugins.** `plan/spec-firewall-domain-group.md`
was written on 2026-09-02 as a separate plugin that would depend on this spec
for shared substrate: the cache, keeping the last good answer when a source
fails, programming the set, and the show, update and clear commands. That left
two plugin processes, two timers, two caches and two command sets for one idea,
which is what `ai/rules/simplicity.md` exists to catch. The owner decided one
plugin with two sources. This spec now owns that plugin and the URL source;
domain-group becomes the DNS source inside it and is edited to say so. The
dependency dissolves rather than being scheduled: there is no shared package
with exactly two callers, because there is one package.

**"Group" is a display word, not a config concept, so the source belongs on
`list set`.** The task text above says "firewall group", following the VyOS
spelling, and Ze has no such config node. `handleShowFirewallGroup`
(`internal/plugins/firewall/nft/cmd_show.go`) reads `firewall.LastApplied` and
reports every named set as a "group"; its own YANG help says "Without arguments,
lists all known groups. With a name, shows the set elements." Config has
`list set`. So this spec does NOT add a `remote-group` list. It gives the
existing `list set` a source, and a set whose members are written literally
keeps working with no config change. `show firewall group` then displays a
sourced set for free, because it already displays every set.

The three sources are therefore mutually exclusive alternatives on one set:
members written literally in config, members downloaded from a URL, or members
resolved from domain names. Earlier research found `choice` is this codebase's
shape for exactly that (`ze-iface-conf.yang` `choice kind`,
`ze-static-conf.yang` `choice action`), and confirming that a `choice` can wrap
the existing `list element` without breaking operator config already committed
is the load-bearing research question for this spec.

Still deliberately out of scope, unchanged:
`plan/immediate/spec-firewall-dynamic-address-group.md` owns packet-triggered population
through nftables `dynset`, and owns finishing the inert `flags-dynamic` and
`flags-timeout` lowering in `applySet`.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `ai/digests/firewall.md` - the firewall global options and the IRR flow.
  → Constraint: the digest covers `buildSets` and `prefixRange` end to end and
    does NOT mention any element cap, so the cap below was verified at source
    rather than taken from the digest.
- [ ] `docs/architecture/firewall/firewall-irr.md` - the closest working feature.
  → Constraint: a reply cut short by the read cap is an ERROR, not a shorter
    answer. The doc states it directly: "A reply cut short by the 4 MB read cap
    has no status line, so a partial record set is an error rather than a
    shorter prefix list." A truncated blocklist download follows the same rule:
    it is a failed fetch that keeps the cached list, never a shorter list.
- [ ] `ai/rules/performance.md` - allocation rules.
  → Decision: the hot-path allocation ban does NOT apply here. Its named paths
    are per-UPDATE and per-packet code, and the directives explicitly permit
    ordinary allocation in "config parsing". A timer-driven download and parse
    is cold-path work, so `make`, `append` and `strings.Split` are permitted.
    Avoid `fmt.Sprintf` inside the per-line loop as hygiene, not as a rule.
- [ ] `.golangci.yml` - `noctx` and `bodyclose` are both enabled with no carve-out.
  → Constraint: every outbound HTTP call is built with
    `http.NewRequestWithContext` carrying the refresh loop's cancelable context.
    `http.Get` and a context-less client call are refused.
  → Constraint: `resp.Body` is closed on every return path, including the error
    branches after a non-2xx status.

**Key insights:** (minimal context to resume after compaction)
- **Set size is bounded in exactly one place, and it truncates silently.**
  `prefixesToIntervalElements` (`irr/sets.go`) stops at a private
  `maxPrefixEntries = 500_000` with a WARN and `break`, returning no error, and
  IPv4 and IPv6 SHARE that one budget through `buildSets`. Nothing else counts
  elements: `Set.Validate` (`model.go`) checks only the name and the type,
  `applySet` (`backend_linux.go`) and `lowerSet` (`lower_linux.go`) loop
  unconditionally. The constant is unexported and not reusable.
- **One `firewall.Set` holds one address family.** `SetType` is single-valued
  and `lowerSet` maps one set to one `nftables.Set` of one key type, so a
  downloaded list mixing families must be split into two named sets at parse
  time, as `setNames` and `buildSets` do.
- **There is no shared HTTP client in ze.** Every caller builds its own
  `&http.Client{Timeout: ...}` inline: `peeringdb` at 10s, the update checker at
  30s, `rir.go` at 60s. That is the convention by repetition, not a helper.
- **The response-size cap is a strong, consistent precedent.** `peeringdb`
  caps at 1 MB, the update checker at 64 KiB, JWKS at 256 KiB, the IRR whois
  client at 4 MB, each with `io.LimitReader`.
- **There is no proxy or CA-bundle config surface to honor**, and no fetcher
  sets `Transport.Proxy`. Do not invent one.
- **`managed.Backoff` is NOT reusable.** The type is exported but `newBackoff`
  and every field are unexported, and a zero value multiplies zero forever. No
  generic retry or backoff helper exists in `internal/core/`. The failure-retry
  timer this spec needs is new local code.
- **Conditional fetch is new ground.** No ETag, If-None-Match or 304 handling
  exists anywhere in `internal/`. Re-downloading an unchanged list every
  interval is the default unless this spec designs the alternative.
- **No plain-list parser exists.** The shape to copy is `parseDelegation`
  (`internal/component/resolve/irr/rir.go`): `bufio.Scanner` over an
  `io.LimitReader`, skipping blank and `#` lines. Its own format is
  pipe-delimited RIR records, so only the shape transfers, not the parser.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfcNNNN.md` - [why relevant]
  → Constraint: [specific RFC rule that applies here]

**Key insights:** (minimal context to resume after compaction)
- [insight from docs]

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/yang/ze-firewall-conf.yang` - `list set` keyed
  by name, carrying `leaf type`, four presence containers (`flags-interval`,
  `flags-timeout`, `flags-constant`, `flags-dynamic`) and a `list element` of
  static `value` plus optional `timeout`. Members are written literally. There
  is no source of any kind.
- [ ] `internal/component/config/yang_schema.go` - `flattenChildren` and
  `flattenChoiceCases`. Choice and case are transparent at the data layer: the
  inner data node's name is used directly and the case wrapper is bypassed.
  This is what makes a `choice` safe to add around the existing `list element`.
- [ ] `internal/component/iface/config.go` - `parseTunnelEntry` reads
  `m["encapsulation"]` and finds keys named for the case's inner container
  (`gre`, `gretap`), never the choice or case names. It also performs its own
  Go-side exclusivity check, because the walker does not.
- [ ] `internal/component/firewall/plugins/irr/sets.go` -
  `prefixesToIntervalElements` caps at the unexported `maxPrefixEntries` of
  500000 and `break`s past it with a WARN and no error. `buildSets` gives IPv4
  the full budget and IPv6 whatever remains, so the two families share one
  budget. `setNames` produces one name per family.
- [ ] `internal/component/firewall/model.go` - `SetType` is single-valued, so
  one `Set` holds one address family. `Set.Validate` checks only the name and
  that the type is non-zero, and never inspects `Elements`.
- [ ] `internal/plugins/firewall/nft/lower_linux.go` and
  `internal/plugins/firewall/nft/backend_linux.go` - `lowerSet` and `applySet`
  loop over `Elements` unconditionally. Nothing downstream bounds set size.
- [ ] `internal/component/resolve/peeringdb/client.go` - the fetch convention:
  an inline `&http.Client{Timeout: ...}` at 10 seconds, and a 1 MB body cap
  through `io.LimitReader`. TLS is relaxed only for localhost.
- [ ] `internal/component/resolve/irr/rir.go` - `parseDelegation` is the
  line-parser shape to copy: `bufio.Scanner` over an `io.LimitReader`, skipping
  blank and `#` lines. Its own record format is pipe-delimited and does not
  transfer.
- [ ] `internal/component/managed/reconnect.go` - `Backoff` is exported but
  `newBackoff` and every field are not, and a zero value multiplies zero
  forever. It cannot be reused from another package.
- [ ] `internal/component/firewall/plugins/irr/irr.go` - `refreshLoop` and
  `refreshAllNow` are the error posture to copy: every fetch failure is logged,
  the cached data is kept, and the loop continues.

**Behavior to preserve:**
- A `set` whose members are written literally parses and behaves exactly as it
  does today, with no config migration. This is AC-10 and it is the reason the
  `choice` question was researched before anything else.
- `show firewall group` keeps listing every named set, sourced or not.
- The IRR plugin's own behavior is unchanged by the cap being moved. Only the
  declaration site moves.

**Behavior to change:**
- A `set` may name a source instead of literal members.
- The plugin fetches, parses, caches, and programs the set on a timer.
- A failure keeps the previous contents and retries sooner than the full
  interval.
- The element cap becomes one exported bound, read by both sources.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

Three, differing in kind:

- **Config.** An operator writes a `set` naming a source. Format at entry is the
  YANG-parsed `map[string]any` the plugin receives in `OnConfigVerify` and
  `OnConfigure`. Because choice and case are transparent, the map carries
  `element`, `url` or `domain-names` as a direct key.
- **A refresh timer.** Per set, at its own `refresh-interval` or the global
  default; and after a failure, at `retry-interval` instead.
- **An operator command.** `update firewall group <name>` forces a fetch now.

### Transformation Path

1. Config parse. `parseSet` reads the three optional keys and refuses a set that
   names more than one, because the schema walker does not enforce choice
   exclusivity.
2. Verify. A set naming a source that has never been fetched refuses the commit,
   the same rule `verifyRefs` (`irr.go`) applies to an uncached IRR reference,
   and the same rule `plan/spec-firewall-domain-group.md` applies to a name.
   Verify reads the cache only and performs no network I/O.
3. Fetch. `http.NewRequestWithContext` with the loop's cancelable context, an
   explicit client timeout, and the stored `ETag` and `Last-Modified` sent as
   `If-None-Match` and `If-Modified-Since`.
4. Classify the response. `304` means unchanged: stop, reprogram nothing. A
   non-2xx, a transport error, or a body that hits the byte cap is a FAILED
   fetch: keep the previous contents and arm the retry timer.
5. Parse. `bufio.Scanner` over an `io.LimitReader`, skipping blank and `#`
   lines. Unparsable lines are counted and skipped. A fetch that yields zero
   entries is a failed fetch, not an empty list.
6. Bound. If the entry count exceeds the exported cap, the fetch FAILS and the
   previous contents are kept. It is never truncated.
7. Split by family. One `firewall.Set` per family that has entries, named as
   `setNames` does it.
8. Persist. Write the parsed entries and the new validators to the cache.
9. Program. `firewall.RegisterTables` then `firewall.ApplyAll`.
10. Schedule. Arm the next refresh at the set's interval, or the retry interval
    after a failure.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin → the internet | HTTPS GET with a context, a client timeout, a body cap, and conditional headers | No |
| Plugin → Firewall registry | `firewall.RegisterTables` and `ApplyAll` under one owner | No |
| Plugin → zefs | Registered key per set holding entries and the cache validators | No |
| Plugin → Hub (resolution) | The resolve RPC that `plan/spec-firewall-domain-group.md` adds, used by the domain source | No |
| Hub → Plugin (show) | `enrich-show` carrying source provenance into `show firewall group` | No |
| Operator → Plugin | `pluginserver.RegisterRPCs` and `ForwardToPlugin` | No |

### Integration Points
- `firewall.RegisterTables` and `ApplyAll` (`registry.go`) - programs the sets.
  A rule naming a set no owner registered holds its whole table back
  (`dropTablesMissingAProvidedSet`), which is what the verify refusal prevents.
- The exported element cap in `internal/component/firewall` - read by this
  plugin and by `irr/sets.go`.
- `show.Enrich` (`internal/core/show/show.go`) - the call site the firewall show
  handlers still lack.
- `ProcessManager.Respawn` (`internal/component/plugin/process/manager.go`) -
  restarts the plugin with its cache intact.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A `choice` around the existing `list element` leaves operator config parsing unchanged | `flattenChildren` and `flattenChoiceCases` (`yang_schema.go`) make case transparent: "the inner data node's name is used directly, matching the YANG source". `parseTunnelEntry` (`iface/config.go`) reads case-inner keys and proves it live | Existing firewall config breaks on upgrade, which is the worst outcome this spec could have | A parse test over a set written in today's syntax, asserting an identical parsed result before and after the schema change | unvalidated |
| A-2 | Go-side code must enforce choice exclusivity, because the walker does not | The schema walker flattens cases and keeps no record of which case supplied a key; `parseTunnelEntry` performs its own check for the same reason | A set naming two sources parses, and the plugin silently honors whichever it reads first | A parse test asserting a set with both `url` and `domain-names` is refused, naming both | unvalidated |
| A-3 | Moving the element cap to an exported bound leaves IRR behavior unchanged | The cap is read at exactly one place, `prefixesToIntervalElements`, called by `buildSets` and `buildTermSets` (`irr/sets.go`) | The IRR plugin's set contents change, which is a regression in a shipped feature this spec does not own | The existing IRR set tests passing unchanged, plus a test asserting the same cap value | unvalidated |
| A-4 | The domain source can reuse this plugin's cache, timer and command surface unchanged | Both sources answer the same question, "what are this set's members now", and differ only in how they are obtained | The two sources need separate machinery after all, and the one-plugin decision has to be revisited | Implementing the URL source first, then the domain source against the same interfaces | unvalidated |
| A-5 | A blocklist publisher serves usable `ETag` or `Last-Modified` headers | Common practice for static list files behind a CDN, and unverified in this codebase because no ze code has ever sent a conditional request | Conditional fetch is dead weight: every refresh downloads the whole list anyway, and the saving the owner asked for does not materialize | A functional test asserting a 304 path, plus a manual check against a real published list before closure | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A list larger than the cap silently under-filters, the defect journaled at `plan/journal/guard-addition-drops-what-it-refuses.md` | Refresh outcome counter labeled for the over-cap result | DECIDED (owner, 2026-09-02): an over-cap download is a FAILED fetch that keeps the previous contents. It is never truncated. The IRR whois client's own rule is the precedent: a reply cut short by its read cap "is an error rather than a shorter prefix list" |
| R-2 | The element cap is a fact declared twice once a second source exists | The two constants drift, and two sources disagree about the same bound | DECIDED (owner, 2026-09-02): one exported bound in `internal/component/firewall`, read by this plugin and by `irr/sets.go`. This puts the IRR file in the diff, and its existing tests must pass unchanged |
| R-3 | A set naming two sources is accepted, because the schema walker does not enforce choice exclusivity | A config that should have been refused commits cleanly | Go-side refusal in `parseSet`, following `parseTunnelEntry`. A-2 covers the test |
| R-4 | A URL is attacker-influenced input that decides what a firewall permits or drops | None at runtime; the box does what the config asked | Cap the body with `io.LimitReader`, cap the entry count, treat a truncated body as a failed fetch, and require HTTPS in the leaf pattern. Document that a permit rule sourced from a URL trusts whoever serves it |
| R-5 | A fetch failure that retries too eagerly hammers the publisher, and one that retries too slowly leaves the list stale | Refresh outcome counters, and the retry interval relative to the refresh interval | A `retry-interval` leaf with a sane default, and exponential growth with jitter written locally, since `managed.Backoff` cannot be imported |
| R-6 | A panic while parsing an untrusted downloaded list kills the plugin | The plugin process exiting, `ze_plugin_restarts_total` rising | Recover in the refresh loop so a bad list costs one cycle. `ze-go-style.md`: "A peer MUST NOT be able to panic the daemon", and a downloaded list is the same class of input |
| R-7 | Cold start with no cached list holds back every table naming the set | The WARN in `dropTablesMissingAProvidedSet` | Persist to the cache and refuse the commit at verify, the same treatment `plan/spec-firewall-domain-group.md` chose and `verifyRefs` already implements for IRR |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Two things, in increasing severity. A bad fetch path leaves a set stale, which under-filters or over-filters. A bad `choice` change breaks parsing of firewall config operators have already written, which takes the whole firewall section down on upgrade. A-1 exists for the second |
| How is it reverted? | Single commit revert for the plugin. The `choice` schema change is also a single revert, because it adds no new required node and rewrites no existing config |
| Who else touches this path? | `plan/spec-firewall-domain-group.md` adds the domain source to this same plugin. `plan/immediate/spec-firewall-dynamic-address-group.md` owns `flags-dynamic` and `flags-timeout` lowering in `applySet`. `internal/component/firewall/plugins/irr/sets.go` is touched by the cap move |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A committed config carrying a set with a `url` source | → | the plugin's `OnConfigure`, then `firewall.RegisterTables` | `TestSourcedSetConfigureRegistersSet` |
| A set's refresh interval expiring | → | the plugin's refresh loop, then the fetcher | `TestSourcedSetRefreshFiresAtInterval` |
| A fetch failure | → | the retry timer rather than the refresh timer | `TestSourcedSetFailureArmsRetryInterval` |
| `update firewall group <name>` typed by an operator | → | the forwarded plugin command handler | `test/plugin/firewall-sourced-group-update.ci` |
| `show firewall group` typed by an operator | → | `show.Enrich` call site plus the plugin's `OnEnrichShow` | `test/firewall/firewall-cli-sourced-group-show.ci` |
| A daemon restart with a populated cache | → | the plugin's cache load on `OnConfigure` | `TestSourcedSetColdStartProgramsFromCache` |
| A commit naming a source never fetched | → | the plugin's `OnConfigVerify` | `TestSourcedSetVerifyRefusesUnfetchedSource` |
| A set written in today's literal-element syntax | → | `parseSet`, unchanged | `TestParseSetLiteralElementsUnchangedByChoice` |
| A set naming two sources | → | the Go-side exclusivity check in `parseSet` | `TestParseSetRefusesTwoSources` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A set naming a URL is committed, after the list has been fetched once | The nftables set holds exactly the entries the list contained, and a rule matching that set filters on them |
| AC-2 | The downloaded list mixes IPv4 and IPv6 entries | Two named sets are programmed, one per family, and neither contains an entry of the other family |
| AC-3 | A refresh returns a transport error or a non-2xx status | The set keeps its previous contents, the failure is logged and counted, and nothing is reprogrammed |
| AC-4 | A downloaded list contains more entries than the element cap | The fetch FAILS. The set keeps its previous contents. The list is never truncated, and the failure names both the entry count and the cap |
| AC-5 | A response body reaches the byte cap before the list ends | The fetch FAILS and the previous contents are kept, because a body cut short is an error rather than a shorter list |
| AC-6 | A refresh fails | The next attempt is made at the retry interval, not at the full refresh interval, and repeated failures back off with jitter rather than at a fixed rate |
| AC-7 | The server answers 304 Not Modified | Nothing is parsed, nothing is reprogrammed, no cache write occurs, and the set keeps serving what it had |
| AC-8 | A 200 response carries an ETag or Last-Modified | Both are stored, and the next fetch sends them as If-None-Match and If-Modified-Since |
| AC-9 | A set declares its own refresh interval while a global default is also configured | The set's own interval is used. A set that declares none uses the global default |
| AC-10 | A set written in today's literal-element syntax is committed, with no edit | It parses and programs exactly as it does today. No migration, no warning, no behavior change |
| AC-11 | A set names two sources at once | The commit is REFUSED, and the error names the set and both sources |
| AC-12 | A commit names a set whose source has never been fetched | The commit is REFUSED at verify, naming the set and the command that would fetch it. No table is silently held back |
| AC-13 | The daemon restarts with a populated cache and the network unreachable | Every sourced set is programmed from the cache before any fetch is attempted, so filtering resumes without waiting for the network |
| AC-14 | The downloaded list contains blank lines, `#` comments, and lines that are neither an address nor a prefix | Blank and comment lines are skipped silently. Unparsable lines are skipped and counted, and the count is reported |
| AC-15 | A fetch succeeds but yields zero usable entries | The fetch FAILS and the previous contents are kept, because an empty list is indistinguishable from a broken publisher and an empty set is not a filter |
| AC-16 | The IRR plugin runs after the element cap has moved to a shared bound | Its set contents and its existing tests are unchanged |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Points a set at a published blocklist and commits a rule dropping it | `update firewall group` -> fetch -> parse -> cache -> commit -> verify passes -> `RegisterTables` -> `ApplyAll` -> nft | `test/plugin/firewall-sourced-group-update.ci` |
| 2 | Commits a rule naming a set never fetched | commit -> `OnConfigVerify` -> cache miss -> commit refused naming the fetch command | `TestSourcedSetVerifyRefusesUnfetchedSource` |
| 3 | Reads which sets are sourced, from where, and how fresh | `show firewall group` -> `show.Enrich` -> `enrich-show` -> source and last-fetch attached | `test/firewall/firewall-cli-sourced-group-show.ci` |
| 4 | Keeps filtering while the publisher is down | refresh fails -> previous contents kept -> retry timer armed | `TestSourcedSetFailureArmsRetryInterval` |
| 5 | Reboots with the network down | restart -> `OnConfigure` -> cache load -> `RegisterTables` | `TestSourcedSetColdStartProgramsFromCache` |
| 6 | Upgrades a box whose firewall config uses literal set elements | config parse -> `parseSet` -> unchanged result | `TestParseSetLiteralElementsUnchangedByChoice` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestXxx` | `internal/.../xxx_test.go` | [description] | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| [field] | [min-max] | [value] | [value or N/A] | [value or N/A] |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-xxx` | `test/.../*.ci` | [what the user expects to happen] | |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-feature-peer` | `test/interop/scenarios/` | [FRR/BIRD/GoBGP/strongSwan] | [protocol behavior validated] | |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/...` - [feature changes]

## Files to Create
- `internal/...` - [new feature file]
- `test/.../*.ci` - [functional test for end-user behavior]

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | `internal/component/<name>/yang/` or the owning plugin's `yang/`. Read `ai/rules/config.md` (YANG vs env var) and `ai/rules/config.md` (naming) |
| YANG validation constraints | | Every leaf takes maximum native validation: `range`, `length`, `pattern`, `enumeration`, `type` from `ze-types.yang`. See `ai/patterns/config-option.md` |
| YANG custom validators | | Where native constraints are insufficient: `ze:validate` + `ValidateFn` + `CompleteFn` for completion |
| CLI commands/flags | | `cmd/ze/*/main.go` or subcommand files |
| CLI grammar (keyword before value) | | `ai/rules/cli.md` |
| Editor autocomplete | | Automatic for YANG enum/type leaves. Dynamic values need `CompleteFn` |
| Functional test for new RPC/API | | `test/plugin/*.ci` or `test/decode/*.ci` |
| Pipe completeness | | Route output through `ApplyPipes`/`ProcessPipes` per `ai/rules/cli.md` |
| Env var registration | | YANG leaves under `environment/` need a matching `ze.<name>.<leaf>` via `env.MustRegister()` |
| Doctor check for runtime dependencies | | Any new file path, socket, service, kernel module, listen port, procfs/sysctl, netlink, binary, or certificate: owning-package check + `internal/core/diagnostic/codes.go` + unit and functional test (`ai/rules/repo-maintenance.md`) |
| Prometheus counters/metrics | | Observable state: define, register, and list the metric names and labels here |
| BGP family surface (new SAFI / capability / attribute) | | The 12-section checklist in `ai/patterns/bgp-family.md` -- read it and record the answers there, not inline |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | | `docs/features.md` |
| 2 | Config syntax changed? | | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | | `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | | `rfc/short/rfcNNNN.md` and the `docs/features/rfc-status.md` row, with source anchors |
| 10 | Test infrastructure changed? | | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | | `docs/comparison.md` |
| 12 | Internal architecture changed? | | `docs/architecture/core-design.md` or subsystem doc |
| 13 | Route metadata keys added/changed? | | `docs/architecture/meta/README.md`, `docs/architecture/meta/<plugin>.md` |
| 14 | Prometheus counters added/changed? | | `docs/plugin-development/metrics.md` or subsystem telemetry doc |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | | Grep `docs/` for `source: <changed-file>` and update each stale claim |
| 17 | Existing docs show config/CLI/API examples for this area? | | Verify examples against YANG/parser/handler and update stale syntax |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- register entry points, write failing wiring tests
   - Tests: [wiring test names from the Wiring Test table]
   - Files: [register.go, handler skeleton, route registration]
   - Verify: the entry point exists and is reachable. The wiring test fails because the feature is a stub
2. **Phase: [name]** -- [what to implement]
   - Tests: [test names from the TDD Plan]
   - Files: [files from Files to Modify]
   - Verify: tests fail → implement → tests pass → wiring test progresses

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | [feature-specific, for example "merge order correct", "error messages name the offending value"] |
| Naming | [feature-specific, for example "JSON keys kebab-case", "YANG leaf matches env var leaf"] |
| Data flow | [feature-specific, for example "resolution in X only, reactor unaware of Y"] |
| Rule: [relevant rule] | [what to check] |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| [concrete thing that must exist] | [grep/ls/test command] |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | [what inputs need validation and how] |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     needs a row in the deferral shard named in the metadata table. -->
- [What was deliberately not done and why]

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Work Inherited From a Deferral Row

<!-- The deferral directory was deleted on 2026-09-05. A row that named this spec as
     its destination is reproduced here, so the item and the reasoning behind it
     survive the directory. Each row is outstanding work this spec owns. -->

### From `firewall-domain-group.md`, 2026-09-02

Deferred by spec-firewall-domain-group.

The shared sourced-group substrate: disk cache, keeping the last good answer when a source fails, set programming, and the show/update/clear command shape

### From `firewall-domain-group.md`, 2026-09-02

Deferred by spec-firewall-domain-group.

A registration path for dynamic CLI value completion, so a plugin's configured names complete without hand-wiring
