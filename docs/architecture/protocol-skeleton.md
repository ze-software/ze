# The Protocol Subpackage Skeleton

Every protocol in Ze uses the same subpackage layout, so a reader who learns one
protocol can navigate the next one. This page describes the layout and how each
existing protocol maps to it.

The layout is ADVISORY. No build gate enforces it, nothing is moved or renamed
to satisfy it, and `ai/rules/protocol.md` states what a new protocol owes.

A single-package protocol needs none of this. LDP and RSVP-TE are a root package
plus `yang/`, and that is the right size for them. The skeleton applies once a
protocol grows subpackages.

## The modules

| Module | Required? | Holds |
|--------|-----------|-------|
| root package | required | `register.go` registration (the `sdk.NewWithConn` engine entry, `pkg/plugin/sdk/sdk.go`) and config plumbing. It may BE the engine |
| `packet/` | required | The wire codec: parse and encode the protocol's PDUs and TLVs |
| `transport/` | required | Socket I/O delivering wire bytes to and from the engine. An in-memory loopback for tests belongs here |
| `yang/` | required | Embedded and registered YANG schema modules |
| engine home | required | The long-lived runtime loop: either the root package (IS-IS, OSPF) or a dedicated `engine/` (BFD, IKE) |
| per-peer state | required when the protocol has per-peer conversations | Named by the protocol's OWN RFC term: `session` (BFD), `adjacency` (IS-IS), `neighbor` (OSPF), `fsm` (BGP, RFC 4271). Do not flatten these to one word: the RFC name is the discoverable one |
| `types/` | optional | Shared leaf types imported by codec and engine |
| `cli/` or `cmd/` | optional | Operational command handlers |
| `redistribute/` | optional | Route redistribution glue |
| domain modules | optional | Protocol concepts named after the RFC concept: `lsdb`, `spf`, `sr`, `crypto`, `eap`, `ipsec`, `auth`, `circuit`, `iface`. Free naming, one concept per package |
| `v<N>/` | optional | Wire-version split (`ospf/v3`): version-specific `packet`, `types` and `transport` under a version directory, with a shared engine above it |

The module names come from the package-naming glossary in
`ai/rules/go-standards.md`.

## The five classes

`Classify` in `internal/le/protocolskeleton/protocolskeleton.go` puts every
subpackage of every listed protocol into one of five classes, in this order:

| Class | Membership |
|-------|------------|
| `legacy-exception` | A documented kept name that predates the glossary. `legacyExceptions` holds `message`, `wireu` and `reactor` for BGP, and `wire` for IKE |
| `canonical` | `packet`, `transport`, `engine`, `yang`, `types`, `cli`, `cmd`, `redistribute` |
| `rfc-state` | `session`, `adjacency`, `neighbor`, `fsm` |
| `version` | A `v` followed by digits and nothing else |
| `domain` | Everything else. This is a valid class, not a finding |

The exception list is checked first, so a name that reaches both an exception
and a class list resolves as the documented exception.

## How each protocol maps

BFD is the reference layout: `packet`, `engine`, `session`, `transport`, `auth`,
`cmd`, `api`, `yang`.

| Protocol | Root | Modules and their classes |
|----------|------|---------------------------|
| BFD | `internal/component/bfd` | canonical `packet`, `transport`, `engine`, `cmd`, `yang`; RFC state `session`; domain `api`, `auth` |
| IS-IS | `internal/plugins/isis` | canonical `packet`, `transport`, `types`, `cli`, `redistribute`, `yang`; RFC state `adjacency`; domain `circuit`, `lsdb`, `spf`. The engine is the root package |
| OSPF | `internal/plugins/ospf` | canonical `packet`, `transport`, `types`, `cli`, `redistribute`, `yang`; RFC state `neighbor`; version `v3`; domain `iface`, `lsdb`, `spf`, `sr`, `wire`. The engine is the root package, and `wire` here is the raw handoff type rather than a codec |
| IKE | `internal/component/ike` | canonical `transport`, `engine`, `cmd`, `yang`; legacy exception `wire`, a full codec where the skeleton says `packet`; domain `crypto`, `eap`, `ipsec`, `dataplane` |
| BGP | `internal/component/bgp` | canonical `cli`, `types`, `redistribute`, `yang`; RFC state `fsm`; legacy exception `message`, `wireu`, `reactor`; everything else domain, `server` included. BGP is the platform archetype and predates the SDK. It is not a template for new work |
| LDP, RSVP-TE | `internal/plugins/ldp`, `internal/plugins/rsvpte` | `yang` alone. Below the subpackage threshold, so the skeleton does not apply until they grow |

The protocol list is hand-maintained: whether a directory is a protocol needs
judgement, so `manifest` in `internal/le/protocolskeleton/protocolskeleton.go`
declares it and gains a row when a protocol lands.

## The advisory report

    ./le protocol-skeleton report

It classifies every module of every listed protocol and prints a one-line
summary. Pipe it through `| json` for the per-protocol table. Report mode always
exits 0, whatever it found: it is a lens, not a gate. An enforced skeleton would
need a large allowlist, which the tiers work already measured as the wrong
trade. Only a tree it could not READ answers non-zero.

    ./le protocol-skeleton selftest

The selftest is the one part that fails, and it fails when the classifier itself
stopped telling the five classes apart.
