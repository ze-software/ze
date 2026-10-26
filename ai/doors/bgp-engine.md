# The BGP engine door

<!-- door-base: internal/component/bgp internal/core/bgp -->

Read `ai/doors/system.md` first. This door covers the BGP engine:

- the reactor
- the FSM
- message parsing and building
- the wire and attribute packages
- the attribute pools
- the encoding contexts
- the filter pipeline

The BGP plugins
under `internal/component/bgp/plugins/` have their own door, `bgp-plugins.md`.
How a session and an UPDATE flow is in `ai/digests/bgp-reactor.md` and
`ai/digests/wire-and-pools.md`. This page does not repeat it.

## Where it sits

- **The engine speaks the protocol and holds no policy.** It owns the FSM,
  parsing, wire I/O and the recent-UPDATE cache. RIB storage, best path, policy
  and GR state belong to plugins (`docs/architecture/api/architecture.md#tldr-read-this-first`). There is no Loc-RIB in the engine (`plan/learned/DESIGN-HISTORY.md`,
  "Best-path lives in bgp-rib plugin").
- **BGP is a feature that compiles out.** Every package under
  `internal/component/bgp` is gated by `ze_bgp` through the generated
  `internal/component/plugin/all/all_ze_bgp.go`. None carries its own build tag
  (`feature-gates.txt`, the `ze_bgp` block). Always-on code must not import it.
- **BGP is a subsystem, and its plugins extend it.** The subsystem owns TCP,
  the FSM and the wire (`docs/architecture/plugin/component-boundaries.md`).
  Its boot mechanism today: the BGP plugin
  (`internal/component/bgp/plugin/register.go`) builds the reactor and hands it
  to the Coordinator, rather than through `Engine.RegisterSubsystem`. Peers start only after every plugin phase finished
  (`ai/digests/hub-engine.md`, step 7).
- **Two tiers of package.** `internal/core/bgp/{attribute,nlri,capability,context,wire}`
  are core leaves with no config lifecycle. `internal/component/bgp/{reactor,fsm,message,wireu,attrpool,filterapi,server}`
  are the engine. `filterapi` stays a near-leaf so plugins, the reactor and
  protocol filters import it without cycles (`filterapi.go` package doc).

## Who depends on it from outside

| Outside party | Contract | Page |
|---------------|----------|------|
| A protocol peer | The RFCs, then ExaBGP API compatibility, then ExaBGP's behavior | `ai/rules/rfc-compliance.md`, `docs/architecture/testing/interop.md` |
| A plugin, ours or third-party, in any language | The UPDATE event: JSON with optional raw wire hex per attribute and family, in the format and encoding the process asked for | `docs/architecture/api/json-format.md`, `docs/architecture/api/architecture.md` ("Wire Bytes in Events") |
| The same plugin, sending | `send bgp <selector> update text ...`, the flat keyword grammar | `docs/architecture/api/update-syntax.md`, `docs/architecture/api/commands.md` ("Route Commands") |
| A forwarding plugin (route server, reflector) | The cache: `bgp cache retain/release/expire`, `send bgp <selector> cached <id>`, and the typed SDK pair for high rates. An ack is cumulative | `docs/architecture/api/commands.md`, `docs/architecture/api/architecture.md` ("BGP Cache Control") |
| A policy plugin | The `filter-update` RPC (`FilterUpdateInput`, `FilterUpdateOutput` in `pkg/plugin/rpc`) and the `<plugin>:<filter>` chain reference | `docs/architecture/api/process-protocol.md`, `docs/architecture/core-design.md` ("Policy Filter Chain") |
| A capability plugin | OPEN injection through `rpc.CapabilityDecl` | `docs/architecture/api/capability-contract.md` |
| A migrated ExaBGP script | The bridge turns each ExaBGP line into `send bgp ... update text`, so the text grammar is its contract too | `docs/architecture/exabgp-bridge.md`, `docs/exabgp/exabgp-differences.md` |
| An operator | Which process sees which peer is the peer's `attach process` block | `docs/architecture/api/architecture.md` ("Attaching a Process to a Peer") |

A change to an event field, a keyword of `update text`, a command word or an
RPC field breaks code that is not in this repository. Change it on purpose,
with its page, and check the ExaBGP bridge in the same change.

## Boundaries

- **The engine reaches plugin features only through what they register:**
  - `filterapi.Register` for compiled-in ingress and egress filters
  - the `InProcess*` fields of `registry.Registration` for in-process codecs.
    An external plugin answers the same question over RPC (`registry.go`,
    `InProcessNLRIDecoder` comment).
  - the `filter-update` RPC for policy filters

  Never name a plugin package from the reactor.
- **A `filterapi` filter is in-process only.** It is a Go function pointer,
  so it exists only in a binary that links the plugin. A third-party filter is
  a policy-chain filter and is reached through `filter-update`
  (`docs/architecture/core-design.md`, "All ingress filtering runs in one
  stage-ordered pass").
- **Received bytes are borrowed.** A received UPDATE points into a reusable
  read buffer, so `RawMessage.IsAsyncSafe()` is false and anything that hands
  it to another goroutine copies first (`plan/learned/DESIGN-HISTORY.md`,
  BGP engine invariants, and `docs/architecture/memory/lifetime-contracts.md`).
- **Encoding is buffer-first:** `WriteTo(buf, off) int` into a pooled buffer
  (`ai/rules/performance.md`, `docs/architecture/buffer-architecture.md`).
- **`internal/core/bgp` imports no component**, and a plugin never imports
  another plugin's implementation (`./le arch tier check`).

## Read before code

| Read | It answers |
|------|------------|
| `ai/digests/bgp-reactor.md` | Peer lifecycle, session read loop, FSM, delivery to plugins |
| `ai/digests/wire-and-pools.md` | Wire encoding, buffers, pools, and their gotchas |
| `docs/architecture/core-design.md`, sections 2, 3 and 9 | Encoding contexts per peer, the UPDATE as container, the ingress and egress filter pipeline |
| `docs/architecture/encoding-context.md` | When `ContextID` equality allows forwarding the bytes unchanged |
| `docs/architecture/bgp/structural-forwarding.md`, `docs/architecture/bgp/fanout-dedup.md` | What the forward path does per destination, and what it shares |
| `docs/architecture/bgp/egress-attribute-rules.md` | The egress attribute prohibitions and their order |
| `docs/architecture/update-building.md`, `docs/architecture/wire/*.md` | Building UPDATEs, and the byte layout of each message, attribute and NLRI |
| `docs/architecture/pool-architecture.md`, `docs/architecture/memory/lifetime-contracts.md` | Who owns a buffer, and when it can be released |
| `docs/architecture/behavior/fsm.md` | The FSM states and their transitions |
| `ai/patterns/bgp-family.md` | Adding an NLRI family, a capability or an attribute |
| `ai/rules/performance.md`, `ai/rules/rfc-compliance.md`, `ai/rules/protocol.md` | Hot-path encoding, RFC quotes on every MUST, wire diagrams |

## Traps

Each source below is `plan/learned/DESIGN-HISTORY.md`, "BGP engine: wire
encoding and RIB", load-bearing invariants, unless another source is named.

- **ADD-PATH is per family and per direction, never one bool.** Three sites
  regressed on this, one of them corrupting stored routes. A peer has two
  `EncodingContext`s.
- **Zero-copy needs the received bytes to survive.** Normalizing attributes
  on ingress breaks it while the `ContextID` comparison still says it is safe.
  The size check against the destination's maximum comes before that
  comparison.
- **RFC 4271's 4096-byte cap is deliberately not enforced**, because RFC 8654
  allows 65535. An RFC pass reading only RFC 4271 "fixes" it.
- **Two UPDATE split paths exist and must not serve each other's traffic:**
  `sendUpdateWithSplit` for built routes, `SplitWireUpdate` for forwarded bytes.
- **The cache entry goes in before the event is dispatched, and acks are
  cumulative**, so a forwarder keeps FIFO id order. Breaking either lost about
  98% of routes at 522K.
- **Pooling a buffer that is handed back to the caller** is a use-after-free
  that every test passes.
- **A filter `modify` rewrote only the IPv4 body NLRI** and copied
  MP_REACH_NLRI verbatim, so a dropped IPv6 prefix leaked
  (`plan/journal/guard-added-to-one-half-of-a-pair.md`, 2026-10-17).
- **When a fixture disagrees with the RFC, fix the fixture.** Attribute order
  was once changed to match ExaBGP fixtures, then reverted (same page,
  "Abandoned approaches").
- **An untagged package test does not link BGP:** run it with `-tags ze_bgp`
  (`plan/journal/feature-test-missing-build-tag.md`).

## Gates

| Command | It judges |
|---------|-----------|
| `./le test functional encode`, `decode`, `parse`, `plugin` (one test: `./le test bgp encode <n>`) | Wire output, decoding, config parsing and plugin paths through the real daemon (`docs/functional-tests.md`) |
| `./le test functional exabgp` | ExaBGP compatibility of the encoding |
| `./le test integration interop` | Ze against other BGP daemons in Docker. Any wire-visible change owes it, and it is long, so name it rather than run it in a short context |
| `./le rfc check` | RFC requirement rows, tags, and discrimination records |
| `./le arch tier check`, `./le plugin boundary check` | Import direction, and in-process calls that break when a plugin runs external |
| `./le job run label <label> command go test -tags ze_bgp <package>` | The scoped package test |
