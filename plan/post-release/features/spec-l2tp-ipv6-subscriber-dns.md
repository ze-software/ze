# Spec: l2tp-ipv6-subscriber-dns

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | Owner decision (2026-11-08): a spec of its own, queued after the home-CPE umbrella, which it does not block. Subscriber IPv6 is itself post-release (`spec-l2tp-ipv6-subscriber.md`), so no shipped session reaches the missing resolver today |
| Depends | `plan/post-release/features/spec-l2tp-ipv6-subscriber.md` (IPv6CP opens and the PPP RA is sent); `spec-home-cpe-23-dns-source-fall-through` (the `dns-source` grammar, `docs/features/dns-resolver.md`) and `spec-home-cpe-23b-dns-source-consumers` (closed; the `radius` class and the pool consumer, documented in `docs/features/dns-resolver.md` and `docs/guide/l2tp.md`); after `spec-home-cpe-0-umbrella` (closed) |
| Phase | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

All values in this spec are invented (RFC 3849 prefixes).

## Task

An IPv6 subscriber of the LNS or the PPPoE server is told to fetch other
configuration and is given no resolver: the PPP Router Advertisement sets the
M and O flags and carries no RDNSS option, and nothing sends DHCPv6 option 23.
This spec hands an IPv6 subscriber its resolvers.

Thomas, on 2026-11-08, about the IPv6 subscriber DNS that child 23 had listed
as its question O-6: "spec of its own ... I don't want to delay it, but I don't
want to hold back the [home-CPE] umbrella completion. We need the feature, but
not for the test I want to do in my house."

What it owes when it runs, carried over from child 23's O-6:

| # | Item |
|---|------|
| 1 | Read RFC 6911 attribute 169 (DNS-Server-IPv6-Address) from the Access-Accept, in received order, into the subscriber's auth metadata |
| 2 | A pool IPv6 DNS leaf as the `configured` source for an IPv6 subscriber |
| 3 | RDNSS in the PPP Router Advertisement |
| 4 | DHCPv6 option 23 (DNS_SERVERS) in the PPP DHCPv6 replies |
| 5 | The source of items 3 and 4 is the pool consumer's `dns-source` evaluation from child 23, filtered to IPv6, so `radius`, `configured` and the mode rule apply unchanged |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/access-security.md`, `ai/doors/core.md`
- [ ] `docs/architecture/l2tp/subscriber-session-model.md`
- [ ] `docs/features/dns-resolver.md`, "The `dns-source` list and the learned record" (the record, the classes, the mode rule; 23a, `spec-home-cpe-23-dns-source-fall-through`), and the `radius` row of its class table and the pool consumer section (built by `spec-home-cpe-23b-dns-source-consumers`, closed)

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc6911.md` (enrolment out-of-scope today; this spec decides whether a profile lists it, under the owner's RFC scope rule)
  → Constraint: Section 3.2: "If more than one DNS-Server-IPv6-Address Attribute is present in the Access- Accept packet, the addresses from the Attributes SHOULD be copied in the same order as received."
- [ ] `rfc/short/rfc8106.md` (RDNSS)
- [ ] RFC 3646 (DHCPv6 DNS_SERVERS; text at `rfc/full/rfc3646.txt`)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/ppp/ra_send.go` `raSender.send` - builds the RA with Managed and OtherConfig set and no RDNSS option
- [ ] `internal/component/l2tp/ppp/dhcpv6.go` - defines `D6OptDNSServers` (23); no non-test code sends it
- [ ] `internal/component/l2tp/plugins/authradius/extract.go` `extractAuthMetadata` - reads no DNS attribute, IPv4 or IPv6
- [ ] `internal/component/l2tp/plugins/pool/register.go` - the pool answers IPv4 only; DNS comes from `dnsPrimary`/`dnsSecondary`

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec |
|-----------|------------------------------------------|--------------------|-----------|
| DNS source selection | child 23's core leaf rule and the pool consumer's `dns-source` | selects IPv4 subscriber DNS from `radius` and `configured` | extends: the same evaluation, IPv6 family |
| RADIUS metadata | `extractAuthMetadata`, `l2tp.AuthMetadata` | Framed-* and, after child 23, the MS IPv4 DNS attributes | extends: attribute 169 |
| PPP RA | `raSender.send`, `BuildRA` | M and O set, no options | extends: one RDNSS option |
| PPP DHCPv6 | `internal/component/l2tp/ppp/dhcpv6.go` | prefix delegation answers | extends: option 23 |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An Access-Accept for a subscriber whose IPv6CP opens

### Transformation Path
1. `extractAuthMetadata` stores each attribute 169 address in received order
2. The pool consumer evaluates its `dns-source` for the IPv6 family
3. The PPP RA carries the result as one RDNSS option; the PPP DHCPv6 reply carries it as option 23

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RADIUS to pool | `l2tp.AuthMetadata` field, as child 23's IPv4 fields | No |
| Pool to PPP | the IPv6 response the pool returns | No |

### Integration Points
- child 23's pool `dns-source` evaluation, called for the IPv6 family

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Access-Accept with attribute 169, IPv6 subscriber | → | extraction, pool, RA, DHCPv6 | named at design |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Access-Accept with attribute 169 values 2001:db8::53, 2001:db8::54 | the PPP RA's RDNSS and the DHCPv6 option 23 carry both, in that order | none |
| AC-2 | No attribute 169, pool IPv6 DNS configured | the configured addresses are carried | none |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| named at design | authradius, pool, ppp | AC-1, AC-2 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| named at design | `test/l2tp/` | AC-1 through the LNS | |

## Files to Modify
- `internal/component/l2tp/plugins/authradius/extract.go`, l2tp `session_metadata.go` - attribute 169
- `internal/component/l2tp/plugins/pool/register.go`, its YANG - IPv6 DNS leaf and evaluation
- `internal/component/l2tp/ppp/ra_send.go`, `internal/component/l2tp/ppp/dhcpv6.go` - RDNSS and option 23

## Files to Create
- the functional and interop tests named at design

## Implementation Steps
1. Design after `spec-l2tp-ipv6-subscriber.md` and child 23 land

## Open questions for the design phase

| # | Question |
|---|----------|
| Q-1 | Whether a profile lists RFC 6911 (owner's RFC scope rule) |
| Q-2 | The pool IPv6 DNS leaf name and its count bound |

## Checklist

### Goal Gates (MUST pass)
- [ ] The targeted tests for what changed pass, run in the main checkout: `./le test functional l2tp`, `./le test integration interop-radius-subscriber`, and the scoped package tests of the changed packages through `./le job run`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
