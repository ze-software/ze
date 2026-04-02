# Spec: Resolve CLI Commands

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-resolve-component |
| Phase | - |
| Updated | 2026-04-02 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/patterns/cli-command.md` - CLI structural template
4. `cmd/ze/iface/main.go` - reference offline command (helpfmt.Page pattern)
5. `cmd/ze/data/main.go` - reference map-based dispatch
6. `cmd/ze/main.go:386-425` - static dispatch where new commands are registered
7. `internal/component/resolve/resolvers.go` - Resolvers container (typed APIs)
8. `internal/component/resolve/cymru/cymru.go` - Cymru resolver API
9. `internal/component/resolve/peeringdb/client.go` - PeeringDB resolver API
10. `internal/component/resolve/irr/client.go` - IRR resolver API

## Task

Add a `ze resolve` top-level offline CLI command with four subcommands: `dns`, `cymru`,
`peeringdb`, and `irr`. Each subcommand calls the typed resolver API directly (no daemon
required). This makes resolution services available as standalone tools for operators.

The command is offline (like `ze data`, `ze interface`) -- it creates resolver instances
on the fly, queries, prints results, and exits. No running daemon, no socket connection.

| Command | Operation | Args | Output |
|---------|-----------|------|--------|
| `ze resolve dns a <hostname>` | A record lookup | hostname | One IP per line |
| `ze resolve dns aaaa <hostname>` | AAAA record lookup | hostname | One IPv6 per line |
| `ze resolve dns txt <hostname>` | TXT record lookup | hostname | One record per line |
| `ze resolve dns ptr <address>` | Reverse DNS | IP address | One name per line |
| `ze resolve cymru asn-name <asn>` | ASN to org name | ASN number | Org name |
| `ze resolve peeringdb [--url <url>] max-prefix <asn>` | Prefix counts | ASN number | `ipv4: N` / `ipv6: N` |
| `ze resolve peeringdb [--url <url>] as-set <asn>` | AS-SET names | ASN number | One AS-SET per line |
| `ze resolve irr [--server <host>] as-set <name>` | AS-SET expansion | AS-SET name | One `AS<N>` per line |
| `ze resolve irr [--server <host>] prefix <name>` | Prefix lookup | AS-SET name | One prefix per line |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/resolve.md` - resolve component structure
  -> Constraint: Resolvers have typed APIs, no string dispatch
- [ ] `.claude/patterns/cli-command.md` - CLI command structural template
  -> Constraint: Offline commands use `cmd/ze/<domain>/main.go` with `Run(args) int`
  -> Constraint: helpfmt.Page struct for help output
  -> Constraint: Handle help/-h/--help before dispatch
  -> Constraint: suggest.Command for unknown subcommand hints

### RFC Summaries (MUST for protocol work)
Not protocol work -- no RFC references needed.

**Key insights:**
- Offline command pattern: `cmd/ze/<domain>/main.go` with `func Run(args []string) int`
- Help output: `helpfmt.Page{Command, Summary, Usage, Sections, Examples}.Write()`
- Registration: add `case "resolve"` in `cmd/ze/main.go` static dispatch (~line 407) + import
- Each subcommand creates its own resolver instance -- no shared state needed for offline tools
- DNS resolver needs `Close()` call (cleanup). Others don't.
- DNS resolver methods do NOT take context.Context -- they use internal timeouts. PeeringDB and IRR do take context.
- PeeringDB needs `--url` flag (default: `https://www.peeringdb.com`). Operators use local mirrors.
- IRR needs `--server` flag (default: `whois.radb.net:43`). Operators use rr.ntt.net, whois.ripe.net, etc.
- Cymru needs a DNS resolver for TXT queries -- create one, wrap ResolveTXT as TXTResolver func
- Context with 30s timeout for PeeringDB/IRR (must exceed their internal 10s timeouts). DNS uses own timeout.
- Each subcommand handler must check for help/-h/--help before parsing operations
- Error output to stderr, data to stdout, exit codes 0/1
- Existing `cmd/ze/resolve/` files in working directory from earlier attempt -- this spec is partially retroactive

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/main.go:386-425` - static dispatch switch for offline commands
  -> Constraint: New commands added as `case "resolve": os.Exit(zeresolve.Run(args[1:]))`
  -> Constraint: Import alias pattern: `zeresolve "codeberg.org/thomas-mangin/ze/cmd/ze/resolve"`
- [ ] `cmd/ze/main.go:940-960` - help page command list
  -> Constraint: Add entry to Tools section of helpfmt.Page
- [ ] `cmd/ze/iface/main.go` - reference offline command implementation
  -> Constraint: Switch dispatch, helpfmt.Page for usage, suggest.Command for typos
- [ ] `cmd/ze/data/main.go` - reference map-based dispatch
  -> Constraint: Map dispatch for >5 subcommands (resolve has 4, switch is fine)
- [ ] `internal/component/resolve/dns/resolver.go` - DNS resolver API
  -> Constraint: ResolveA, ResolveAAAA, ResolveTXT, ResolvePTR return ([]string, error)
  -> Constraint: Methods do NOT take context.Context -- internal timeout via ResolverConfig.Timeout
  -> Constraint: NewResolver(ResolverConfig{}) for default config. Close() required.
- [ ] `internal/component/resolve/cymru/cymru.go` - Cymru resolver API
  -> Constraint: LookupASNName(ctx, uint32) (string, error). Empty string = not found (graceful).
  -> Constraint: Constructor: cymru.New(txtResolver TXTResolver, cache *cache.Cache[string])
- [ ] `internal/component/resolve/peeringdb/client.go` - PeeringDB client API
  -> Constraint: LookupASN(ctx, uint32) (PrefixCounts, error). PrefixCounts{IPv4, IPv6 uint32}.
  -> Constraint: LookupASSet(ctx, uint32) ([]string, error). Nil = no AS-SET registered.
  -> Constraint: NewPeeringDB(baseURL string). Default PeeringDB URL: https://www.peeringdb.com
- [ ] `internal/component/resolve/irr/client.go` - IRR client API
  -> Constraint: ResolveASSet(ctx, string) ([]uint32, error). Sorted ASNs.
  -> Constraint: LookupPrefixes(ctx, string) (PrefixList, error). PrefixList{IPv4, IPv6 []netip.Prefix}.
  -> Constraint: NewIRR(server string). Empty string = whois.radb.net:43 default.

**Behavior to preserve:**
- All existing CLI commands and their dispatch unaffected
- Resolver behavior identical to programmatic callers (same error handling, graceful degradation)

**Behavior to change:**
- New `ze resolve` command tree added to CLI
- New entry in help page command list

## Data Flow (MANDATORY)

### Entry Point
- CLI: `ze resolve <service> <operation> <args>`
- OS argv parsed by `cmd/ze/main.go` dispatch

### Transformation Path
1. `cmd/ze/main.go` matches "resolve", calls `zeresolve.Run(args[1:])`
2. `resolve/main.go` dispatches to `cmdDNS`, `cmdCymru`, `cmdPeeringDB`, or `cmdIRR`
3. Handler checks for help/-h/--help, parses flags (--url, --server where applicable)
4. Handler creates resolver instance with config from flags/defaults
5. Handler calls typed resolver method. DNS: no context (own timeout). PeeringDB/IRR: 30s context.
6. Result printed to stdout (one item per line), errors to stderr
7. Resolver cleaned up (Close if needed), exit code returned

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> Resolver | Direct Go function call (offline, no daemon) | [ ] |
| Resolver -> Network | DNS UDP, HTTP (PeeringDB), TCP (IRR whois) | [ ] |

### Integration Points
- `cmd/ze/main.go` - dispatch case + import + help page entry
- `internal/component/resolve/dns/` - DNS resolver
- `internal/component/resolve/cymru/` - Cymru resolver
- `internal/component/resolve/peeringdb/` - PeeringDB client
- `internal/component/resolve/irr/` - IRR client

### Architectural Verification
- [ ] No bypassed layers (CLI calls resolver APIs directly, same as hub consumers)
- [ ] No unintended coupling (offline command, no daemon dependency)
- [ ] No duplicated functionality (uses existing resolve/ packages, no reimplementation)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze resolve --help` | -> | Help dispatch + helpfmt.Page | `test/parse/resolve-help.ci` |
| `ze resolve dns --help` | -> | DNS help dispatch | `test/parse/resolve-dns-help.ci` |
| `ze resolve dns` (no args) | -> | Usage error exit 1 | `test/parse/resolve-dns-noargs.ci` |
| `ze resolve cymru` (no args) | -> | Usage error exit 1 | `test/parse/resolve-cymru-noargs.ci` |
| `ze resolve cymru asn-name abc` | -> | ASN validation | `test/parse/resolve-cymru-invalid.ci` |
| `ze resolve unknown` | -> | Unknown subcommand hint | `test/parse/resolve-unknown.ci` |
| `ze resolve peeringdb --url ... max-prefix 65001` | -> | PeeringDB via fake server | `test/parse/resolve-peeringdb-maxprefix.ci` |
| `ze resolve peeringdb --url ... as-set 65001` | -> | PeeringDB AS-SET via fake server | `test/parse/resolve-peeringdb-asset.ci` |
| `ze resolve cymru asn-name 13335` | -> | Cymru via fake DNS server | `test/parse/resolve-cymru.ci` |
| `ze resolve irr --server ... as-set AS-TEST` | -> | IRR AS-SET via fake whois server | `test/parse/resolve-irr-asset.ci` |
| `ze resolve irr --server ... prefix AS-TEST` | -> | IRR prefix via fake whois server | `test/parse/resolve-irr-prefix.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze resolve dns a <hostname>` | Prints one IPv4 address per line to stdout, exits 0 |
| AC-2 | `ze resolve dns aaaa <hostname>` | Prints one IPv6 address per line to stdout, exits 0 |
| AC-3 | `ze resolve dns txt <hostname>` | Prints one TXT record per line to stdout, exits 0 |
| AC-4 | `ze resolve dns ptr <address>` | Prints one PTR name per line to stdout, exits 0 |
| AC-5 | `ze resolve cymru asn-name <valid-asn>` | Prints org name to stdout, exits 0 |
| AC-6 | `ze resolve cymru asn-name <unknown-asn>` | Prints `no name found for AS<N>` to stderr, exits 1 |
| AC-7 | `ze resolve peeringdb max-prefix <asn>` | Prints `ipv4: <N>` and `ipv6: <N>` (lowercase, one per line) to stdout, exits 0 |
| AC-8 | `ze resolve peeringdb max-prefix <asn>` with both zero | Additionally prints `warning: both counts are zero` to stderr |
| AC-9 | `ze resolve peeringdb as-set <asn>` with AS-SETs | Prints one AS-SET name per line to stdout, exits 0 |
| AC-10 | `ze resolve peeringdb as-set <asn>` with no AS-SET | Prints `no AS-SET registered for AS<N>` to stderr, exits 1 |
| AC-11 | `ze resolve irr as-set <name>` | Prints one `AS<N>` per line (sorted) to stdout, exits 0 |
| AC-12 | `ze resolve irr as-set <name>` with no members | Prints `no members found for <name>` to stderr, exits 1 |
| AC-13 | `ze resolve irr prefix <name>` | Prints one prefix per line (IPv4 then IPv6) to stdout, exits 0 |
| AC-14 | `ze resolve irr prefix <name>` with no prefixes | Prints `no prefixes found for <name>` to stderr, exits 1 |
| AC-15 | `ze resolve --help` | Prints structured help to stderr, exits 0 |
| AC-16 | `ze resolve dns --help` | Prints DNS help to stderr, exits 0 |
| AC-17 | `ze resolve dns` (missing args) | Prints DNS usage to stderr, exits 1 |
| AC-18 | `ze resolve dns invalid-op host` | Prints `unknown dns operation` + valid list to stderr, exits 1 |
| AC-19 | `ze resolve cymru asn-name notanumber` | Prints `error: invalid ASN` to stderr, exits 1 |
| AC-20 | `ze resolve peeringdb max-prefix 0` | Prints error (not found) to stderr, exits 1 |
| AC-21 | `ze resolve unknown` | Prints `unknown resolve subcommand` + hint to stderr, exits 1 |
| AC-22 | `ze help` includes resolve | Resolve appears in tools section with description |
| AC-23 | `ze resolve peeringdb --url <url> max-prefix <asn>` | Uses custom PeeringDB URL |
| AC-24 | `ze resolve irr --server <host> as-set <name>` | Uses custom IRR server |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| N/A -- CLI handlers are thin wrappers over tested resolver APIs | | | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ASN (cymru, peeringdb) | 0-4294967295 | 4294967295 | N/A (non-numeric rejected) | 4294967296 (parse error) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `resolve-help` | `test/parse/resolve-help.ci` | `ze resolve --help` prints help with Services section, exits 0 | |
| `resolve-dns-help` | `test/parse/resolve-dns-help.ci` | `ze resolve dns --help` prints DNS help, exits 0 | |
| `resolve-dns-noargs` | `test/parse/resolve-dns-noargs.ci` | `ze resolve dns` with no args prints usage, exits 1 | |
| `resolve-cymru-noargs` | `test/parse/resolve-cymru-noargs.ci` | `ze resolve cymru` with no args prints usage, exits 1 | |
| `resolve-cymru-invalid` | `test/parse/resolve-cymru-invalid.ci` | `ze resolve cymru asn-name abc` prints error, exits 1 | |
| `resolve-unknown` | `test/parse/resolve-unknown.ci` | `ze resolve foo` prints error + hint, exits 1 | |

### Test Server Infrastructure

Existing: `ze-test peeringdb` (fake PeeringDB HTTP, deterministic prefix counts from ASN).
Needs extension: `irr_as_set` field not returned by fake -- add it for `ze resolve peeringdb as-set` tests.

New test servers needed:

| Server | Purpose | Pattern |
|--------|---------|---------|
| `ze-test cymru` | Fake DNS TXT server returning deterministic Cymru-format responses | UDP DNS on localhost, responds to `AS<N>.asn.cymru.com.` TXT queries |
| `ze-test irr` | Fake whois server returning deterministic AS-SET expansions | TCP on localhost, responds to `!i` and `!a` RPSL queries |

These enable end-to-end `.ci` tests for all resolve commands without network access.

### End-to-End Functional Tests (with fake servers)
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `resolve-peeringdb-maxprefix` | `test/parse/resolve-peeringdb-maxprefix.ci` | Start `ze-test peeringdb`, run `ze resolve peeringdb --url http://127.0.0.1:$PORT max-prefix 65001`, verify `ipv4: 65001` / `ipv6: 13000` | |
| `resolve-peeringdb-asset` | `test/parse/resolve-peeringdb-asset.ci` | Same server, `ze resolve peeringdb --url ... as-set 65001`, verify AS-SET output | |
| `resolve-cymru` | `test/parse/resolve-cymru.ci` | Start `ze-test cymru`, run `ze resolve cymru asn-name 13335`, verify org name | |
| `resolve-irr-asset` | `test/parse/resolve-irr-asset.ci` | Start `ze-test irr`, run `ze resolve irr --server 127.0.0.1:$PORT as-set AS-TEST`, verify members | |
| `resolve-irr-prefix` | `test/parse/resolve-irr-prefix.ci` | Same server, `ze resolve irr --server ... prefix AS-TEST`, verify prefixes | |

### Future
- `--json` output flag (add when a consumer needs structured output)

## Files to Modify

- `cmd/ze/main.go` - Add `case "resolve"` dispatch + import + help page entry
- `cmd/ze-test/main.go` - Add `case "cymru"` and `case "irr"` dispatch
- `cmd/ze-test/peeringdb.go` - Add `irr_as_set` field to fake response (needed for `as-set` tests)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | N/A -- offline command, not online RPC |
| CLI commands/flags | [ ] | `cmd/ze/main.go` dispatch + help |
| Editor autocomplete | [ ] | N/A -- offline command |
| Functional test for new RPC/API | [ ] | `test/parse/NNN-resolve-*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` - add resolve CLI |
| 2 | Config syntax changed? | [ ] | N/A |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` - add resolve commands |
| 4 | API/RPC added/changed? | [ ] | N/A -- offline command |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [ ] | N/A |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` - built-in resolution CLI |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/resolve.md` - add CLI section |

## Files to Create

**Note:** `cmd/ze/resolve/` files exist in working directory from an earlier attempt. They need
rework to add flag parsing (--url, --server), subcommand --help handling, and output format fixes.

- `cmd/ze/resolve/main.go` - Run() + dispatch + usage (helpfmt.Page). 30s context for PeeringDB/IRR.
- `cmd/ze/resolve/cmd_dns.go` - DNS handler. No context (DNS has own timeout). help/-h/--help check.
- `cmd/ze/resolve/cmd_cymru.go` - Cymru handler. help check. ASN validation with clear error.
- `cmd/ze/resolve/cmd_peeringdb.go` - PeeringDB handler. `--url` flag (default https://www.peeringdb.com). help check.
- `cmd/ze/resolve/cmd_irr.go` - IRR handler. `--server` flag (default whois.radb.net). help check.
- `cmd/ze-test/cymru.go` - Fake DNS TXT server for Cymru (UDP, miekg/dns, deterministic ASN->name)
- `cmd/ze-test/irr.go` - Fake whois server for IRR (TCP, RPSL `!i`/`!a4`/`!a6`, deterministic)
- `test/parse/resolve-help.ci` - Help output test (AC-15)
- `test/parse/resolve-dns-help.ci` - DNS help test (AC-16)
- `test/parse/resolve-dns-noargs.ci` - Missing args test (AC-17)
- `test/parse/resolve-cymru-noargs.ci` - Missing args test
- `test/parse/resolve-cymru-invalid.ci` - Invalid ASN test (AC-19)
- `test/parse/resolve-unknown.ci` - Unknown subcommand test (AC-21)
- `test/parse/resolve-peeringdb-maxprefix.ci` - End-to-end with fake PeeringDB server
- `test/parse/resolve-peeringdb-asset.ci` - End-to-end AS-SET with fake PeeringDB server
- `test/parse/resolve-cymru.ci` - End-to-end with fake Cymru DNS server
- `test/parse/resolve-irr-asset.ci` - End-to-end AS-SET with fake IRR whois server
- `test/parse/resolve-irr-prefix.ci` - End-to-end prefix with fake IRR whois server

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-verify` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

**Note:** Files from earlier attempt exist in `cmd/ze/resolve/`. Rework them rather than starting fresh.

1. **Phase: Command skeleton** -- Rework `cmd/ze/resolve/main.go` with dispatch, help, 30s context (PeeringDB/IRR only)
   - Files: `resolve/main.go`
   - Each subcommand handler must check help/-h/--help as first arg
   - Verify: `ze resolve --help` prints structured help. `ze resolve foo` suggests.

2. **Phase: DNS handler** -- Rework `cmd_dns.go`. No context (DNS uses own timeout). Add --help check.
   - Files: `resolve/cmd_dns.go`
   - Verify: `ze resolve dns --help` prints help. `ze resolve dns a cloudflare.com` prints IPs.

3. **Phase: Cymru handler** -- Rework `cmd_cymru.go`. Add --help check.
   - Files: `resolve/cmd_cymru.go`
   - Verify: `ze resolve cymru asn-name 13335` prints "Cloudflare, Inc."
   - Verify: `ze resolve cymru asn-name abc` prints "invalid ASN" to stderr, exits 1.

4. **Phase: PeeringDB handler** -- Rework `cmd_peeringdb.go`. Add `--url` flag + --help check.
   - Files: `resolve/cmd_peeringdb.go`
   - Flag: `--url` (default https://www.peeringdb.com). Use flag.NewFlagSet.
   - Output: `ipv4: <N>` and `ipv6: <N>` (lowercase). Warning to stderr when both zero.
   - Verify: `ze resolve peeringdb max-prefix 13335` prints counts. `--url` overrides.

5. **Phase: IRR handler** -- Rework `cmd_irr.go`. Add `--server` flag + --help check.
   - Files: `resolve/cmd_irr.go`
   - Flag: `--server` (default empty = whois.radb.net:43). Use flag.NewFlagSet.
   - Output: `AS<N>` per line for as-set, prefix per line for prefix. Messages for empty results.
   - Verify: `ze resolve irr as-set AS-CLOUDFLARE` prints ASNs. `--server` overrides.

6. **Phase: Register in main** -- Add dispatch case, import, help entry
   - Files: `cmd/ze/main.go`
   - Verify: `ze help` lists resolve. `ze resolve --help` works from top-level dispatch.

7. **Phase: Test servers** -- Create fake Cymru DNS and IRR whois servers, extend PeeringDB fake
   - Files: `cmd/ze-test/cymru.go`, `cmd/ze-test/irr.go`, `cmd/ze-test/peeringdb.go`, `cmd/ze-test/main.go`
   - `ze-test cymru --port N`: UDP DNS server, responds to TXT `AS<N>.asn.cymru.com.` with deterministic Cymru format. Uses miekg/dns (already in go.mod).
   - `ze-test irr --port N`: TCP whois server, responds to `!iAS-TEST` with member ASNs, `!a4AS-TEST`/`!a6AS-TEST` with prefixes. Deterministic data.
   - Extend `ze-test peeringdb`: add `irr_as_set` field to JSON response (e.g., ASN 65001 -> "AS-TEST", ASN 65002 -> "AS-FOO AS-BAR").
   - Verify: each server starts, responds correctly to manual queries.

8. **Phase: Error-path functional tests** -- .ci tests for help, validation, dispatch
   - Files: `test/parse/resolve-help.ci`, `resolve-dns-help.ci`, `resolve-dns-noargs.ci`, `resolve-cymru-noargs.ci`, `resolve-cymru-invalid.ci`, `resolve-unknown.ci`
   - All offline (no fake servers needed). Test CLI dispatch and error handling.
   - Verify: `make ze-functional-test` passes

9. **Phase: End-to-end functional tests** -- .ci tests with fake servers
   - Files: `test/parse/resolve-peeringdb-maxprefix.ci`, `resolve-peeringdb-asset.ci`, `resolve-cymru.ci`, `resolve-irr-asset.ci`, `resolve-irr-prefix.ci`
   - Pattern: `cmd=background` starts fake server, `cmd=foreground` runs `ze resolve` with `--url`/`--server` pointing at localhost.
   - Verify: `make ze-functional-test` passes

10. **Full verification** -- `make ze-verify`
11. **Complete spec** -- Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify |
|-------|----------------|
| Completeness | All 9 commands work from CLI |
| Correctness | Output format matches AC table exactly |
| Naming | Package `resolve`, file pattern `cmd_<service>.go` |
| Error handling | All errors to stderr, correct exit codes |
| Help output | helpfmt.Page for all usage functions |
| Data flow | CLI -> resolver API -> stdout (no daemon) |
| Rule: cli-patterns | help/-h/--help handled. suggest.Command for typos. Exit codes 0/1. |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| resolve package exists | `ls cmd/ze/resolve/*.go` |
| All 5 handler files | `ls cmd/ze/resolve/cmd_*.go` |
| Registered in main | `grep "resolve" cmd/ze/main.go` |
| Help entry | `ze help` output includes resolve |
| Fake Cymru server | `ls cmd/ze-test/cymru.go` |
| Fake IRR server | `ls cmd/ze-test/irr.go` |
| PeeringDB fake extended | `grep irr_as_set cmd/ze-test/peeringdb.go` |
| Error-path tests | `ls test/parse/resolve-help.ci test/parse/resolve-unknown.ci` |
| End-to-end tests | `ls test/parse/resolve-cymru.ci test/parse/resolve-irr-asset.ci test/parse/resolve-peeringdb-maxprefix.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | ASN parsed with strconv, rejects non-numeric. AS-SET validated by irr.ValidateASSetName |
| DNS injection | Hostname passed directly to miekg/dns (handles its own validation) |
| HTTP SSRF | PeeringDB URL hardcoded to public API (not user-controlled) |
| Whois injection | IRR uses validateASSetName to reject control characters |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Network timeout | Not a bug -- resolver has 10s default timeout |
| `ze help` doesn't list resolve | Check help page entry in main.go |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Design Insights

### Offline vs Online

This is an offline command. Each invocation creates fresh resolver instances, queries,
prints, and exits. No daemon required, no persistent cache between invocations. This is
the right model for an operator debugging tool -- simple, predictable, stateless.

The daemon's `Resolvers` container (created at hub startup) is for long-lived consumers
(web decorator, LG, prefix update). The CLI command doesn't use it.

### Context and Timeouts

DNS resolver methods don't take context.Context -- they use internal timeouts from
ResolverConfig. PeeringDB and IRR do take context. The CLI creates a 30s context for
PeeringDB/IRR (must exceed their internal 10s timeouts to avoid masking resolver-level
errors). DNS handlers don't use context at all.

### Output Format

One result per line, plain text. No JSON by default (`--json` flag deferred).
Error messages to stderr. This makes the output pipeable:

`ze resolve irr as-set AS-EXAMPLE | wc -l` (count members)
`ze resolve dns a example.com | head -1` (first A record)

Exact formats locked down in AC table: `ipv4: <N>` (lowercase), `AS<N>` (uppercase),
`no name found for AS<N>`, etc.

### Operator Flags

PeeringDB: `--url` flag (default `https://www.peeringdb.com`). Operators with local
PeeringDB mirrors or corporate proxies need this on day one.

IRR: `--server` flag (default empty = `whois.radb.net:43`). Operators commonly use
`rr.ntt.net`, `whois.ripe.net`, `whois.arin.net`, etc. depending on region and policy.

Both use flag.NewFlagSet per subcommand handler (standard ze CLI pattern).

### Subcommand Help

Every handler checks for help/-h/--help as first argument before parsing operations.
Without this, `ze resolve dns --help` would fail with "unknown dns operation: --help".

### Retroactive Status

`cmd/ze/resolve/` files exist from an earlier attempt. They work but need rework:
flags, subcommand help, output format fixes, context handling. The spec captures
what needs to change.

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

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

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |

### Tests from TDD Plan
| Test | Status | Location | Notes |

### Files from Plan
| File | Status | Notes |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-24 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-verify` passes
- [ ] Feature code integrated (`cmd/ze/resolve/`)
- [ ] Integration completeness proven end-to-end
- [ ] Critical Review passes

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per file
- [ ] Explicit > implicit behavior

### TDD
- [ ] Functional tests written
- [ ] Functional tests pass

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
