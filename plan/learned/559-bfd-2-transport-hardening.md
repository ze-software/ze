# 559 -- bfd-2-transport-hardening

## Context

Stage 1 (`plan/learned/556-bfd-1-wiring.md`) wired the BFD plugin into
the engine lifecycle but left the UDP transport in a production-unsafe
state: `readLoop` used `ReadFromUDP` and hard-coded `Inbound.TTL = 0`
with a "future work" comment, outbound packets went out at the kernel
default TTL (64 on Linux, instead of the RFC 5881 Section 5 mandatory
255), sockets never bound to an interface or a VRF device, and the
express loop fired periodic packets on an exact deadline with no
RFC 5880 Section 6.8.7 jitter. `loopFor` returned an explicit "VRF
not yet supported" error for every non-default VRF, and the engine
accepted any packet that made it past the codec regardless of its IP
TTL.

Stage 2 closes every one of those gaps for the IPv4 path: real TTL
extraction via `IP_RECVTTL` cmsg, single-hop GTSM (`TTL == 255`),
multi-hop min-TTL floor, `IP_TTL=255` outbound, `SO_BINDTODEVICE` for
single-hop interface and non-default VRF binding, and per-packet TX
jitter clamped to the detect-multiplier bands the RFC mandates. IPv6
dual-bind is deliberately deferred to `spec-bfd-2b-ipv6-transport`
because it requires a Dual-stack transport refactor that would have
ballooned the diff.

## Decisions

- **Linux build-tag the socket-option code** (`udp_linux.go` /
  `udp_other.go`). `IP_RECVTTL`, `IP_TTL`, and `SO_BINDTODEVICE` are
  Linux-specific; the non-Linux stub returns zero on TTL extraction
  (which the engine fails closed on) and rejects any `Device` request
  outright. Chose build tags over one cross-platform dispatch because
  ze's BFD deployment target is Linux only and the stub keeps developer
  builds on macOS or BSD green without introducing a per-call platform
  check.
- **`net.ListenConfig.Control` for the setsockopt path**, not
  post-bind raw-fd manipulation. Applying `SO_BINDTODEVICE` after
  `bind()` is a capability-vs-ordering hazard on Linux kernels that
  enforce CAP_NET_RAW only before bind; the `Control` callback runs on
  the raw fd before `bind()` is issued, which is the officially
  supported path.
- **Reuse the existing rx pool, add a parallel oob pool.** The readLoop
  already pre-allocates a 16-slot × 128-byte rx backing array and a
  free-slot channel; I mirrored that pattern for the control-message
  buffer (`oobBacking`, 16 × 64 bytes) so TTL extraction has zero
  per-packet allocation. An earlier sketch that used `make([]byte, 64)`
  per packet would have introduced one alloc per recv and regressed
  `BenchmarkRoundTrip`.
- **TTL gate runs AFTER session lookup**, not before. Single-hop's
  `TTL == 255` rule is mode-wide and could run earlier, but multi-hop
  `MinTTL` is per-session (different multi-hop sessions can set
  different floors), so the gate has to know which session the packet
  belongs to. Putting both checks in the same place keeps the policy
  one function (`passesTTLGate`).
- **RNG for jitter is the package-level `math/rand/v2` source**, not a
  per-Loop `*rand.Rand`. Two iterations of per-Loop RNG got blocked:
  first because gosec G404 flags `rand.New(rand.NewPCG(...))` and the
  `.golangci.yml` excludes list does not (yet) cover G404, and second
  because an early attempt with `math/rand/v2.New` ran into a
  `binary.NativeEndian`/`crypto/rand` seeding cycle with the
  auto-linter. The package-level `rand.Float64()` is safe for
  concurrent use and sufficient for BFD jitter (which is a purely
  statistical property, not a security decision). A `//nolint:gosec`
  tag with a one-line rationale closes gosec; the pattern mirrors the
  precedent in `internal/chaos/chaos.go`.
- **Jitter bands verified with 10 000-draw unit tests plus a mean-band
  uniformity check.** The DetectMult=1 band must be `[10%, 25%)` (RFC
  floor) and DetectMult>=2 must be `[0%, 25%)`. Uniformity is asserted
  to within 2% of the theoretical midpoint so an accidental off-by-one
  on the offset (dropping the 10% shift) or clipping at 0 cannot slip
  through. Tolerance is tight enough to catch real bugs and loose
  enough that the test is not flaky.
- **Session MinTTL comes from configReq, not Vars.** `session.Vars`
  mirrors the RFC 5880 Section 6.8.1 state variables verbatim; MinTTL
  is a ze-side configuration, not a BFD state variable, so it stays on
  `Machine.configReq` and is exposed via a new `Machine.MinTTL()`
  getter with the zero→254 default applied in the getter itself. No
  change to `Vars`.
- **DetectMult getter added on `Machine`** so the engine's jitter path
  can reach it without poking into `session.Vars` directly. The
  session package stays the authority on bfd.DetectMult.
- **`AdvanceTxWithJitter(now, reduction)` added to `Machine`** so the
  engine applies the jitter as a delta, not by reaching into the
  private `nextTxAt` field. The existing `AdvanceTx(now)` stays for
  callers that don't care about jitter (tests, Poll/Final replies).
- **Multi-interface-in-same-VRF fallback is permissive.** If two
  single-hop pinned sessions in the default VRF point at different
  interfaces, the shared loop cannot bind to either (there's one
  socket per loop). `resolveLoopDevices` drops back to an empty Device
  and the plugin logs a warning so the operator notices. The engine
  TTL gate still enforces GTSM; the lost belt-and-braces is operator
  choice.
- **Functional test uses `listKey` as the peer fallback.** ze's config
  file parser only carries the first positional value after a list
  name (`single-hop-session 203.0.113.9 { ... }`), so
  `parseSingleHopSession` and `parseMultiHopSession` were changed to
  accept the listKey as `peer` when `fields["peer"]` is empty. This
  was the surprise of the spec: Stage 1 used a profile-only config and
  never exercised the pinned-session code path, so the "one-positional
  list key" limitation of ze's config syntax was invisible until the
  functional test drove a real pinned session. The fallback is
  consistent with how BGP handles `peer peer1 { ... }`.
- **`/ze-review` resolve pass** found five defensive-coding gaps in
  Stage 2 that are all fixed in this landing:
  1. Non-Linux `applySocketOptions` stub now applies IP_TTL=255 via
     the stdlib `syscall` package (which has the constant on every
     Unix). Before the fix, a non-Linux developer build produced
     non-compliant RFC 5881 packets (kernel default TTL=64) silently.
  2. `resolveLoopDevices` collects the full set of conflicting
     interfaces per loop and logs a single deterministic warning line
     at the end of the pass, instead of N nondeterministic warnings
     per Go map iteration. A separate `Info` line fires when a
     non-default VRF causes the session `interface` leaf to be
     overridden by the VRF device binding.
  3. `session.Machine.AdvanceTxWithJitter` now clamps `reduction` to
     `[0, TransmitInterval())` defensively; a caller bug that passed
     `reduction >= interval` would have driven the next-TX deadline
     backwards and spun the express loop.
  4. `engine.Loop.Start` resets `started` to false when
     `l.transport.Start()` fails, so a subsequent `loop.Stop()`
     cannot deadlock on a doneCh that will never close. Pre-existing
     bug made more reachable by Stage 2 adding a real SO_BINDTODEVICE
     error path.
  5. `transport.UDP.readLoop` checks the `MSG_CTRUNC` recvmsg flag
     and logs once per transport via a new `sync.Once` when the
     kernel truncates the oob buffer. Makes future oob-budget bugs
     observable.

## Consequences

- **Single-hop BFD is no longer trivially spoofable from off-link
  sources.** A packet arriving on UDP 3784 with any TTL other than 255
  is silently discarded before it reaches the FSM. Operators no longer
  have to rely on upstream filtering to enforce GTSM.
- **Non-default VRFs are reachable.** `loopFor` no longer returns the
  Stage 1 "not yet supported" error; pinned sessions in a named VRF
  bind the socket to the VRF device. `spec-bfd-3-bgp-client` can now
  plug into VRF-aware BGP peers directly.
- **Stage 2 is IPv4 only.** The `transport.UDP` still binds a single
  v4 socket. IPv6 dual-bind and `IPV6_RECVHOPLIMIT` extraction are
  tracked as `spec-bfd-2b-ipv6-transport` in `plan/deferrals.md`; the
  refactor would have introduced a `transport.Dual` or per-family
  `UDP` pair that the Stage 2 scope explicitly did not want to carry.
- **`config.go` is now the authoritative parser for the
  "list-key-as-peer" convention**. Any future BFD schema change that
  moves the peer leaf out of the YANG key would have to update the
  fallback.
- **The `packet.Pool` zero-alloc round-trip benchmark is preserved.**
  The new oob backing slice is pre-allocated once at goroutine start
  and indexed by slot, following the existing rx backing pattern.
  Running the engine/transport unit tests under `-race` is green.
- **BGP opt-in (`spec-bfd-3-bgp-client`) can build on the stable
  Service contract.** No `api.Service`, `api.SessionHandle`,
  `api.SessionRequest`, or `api.Key` field was touched; the only
  session-side additions are `MinTTL()`, `DetectMult()`, and
  `AdvanceTxWithJitter()` getters/setters on `Machine` (not `Service`).
- **The `multi-interface-same-vrf` permissive fallback is a known
  sharp edge.** An operator who configures two single-hop pinned
  sessions in the same VRF with different `interface` leaves loses
  SO_BINDTODEVICE pinning. A warning is logged. If this surprises a
  deployment, the structural fix is per-(vrf, mode, interface) loops,
  which requires splitting UDP port 3784 across multiple sockets --
  not possible with one kernel socket per port.

## Gotchas

- **ze's config file parser only accepts one positional value as a
  list key.** For multi-key YANG lists like `key "peer vrf interface"`
  the operator can only name one of them in the config file syntax.
  The parser stored ONLY the first positional value as the list key.
  Stage 1 sidestepped this by using profile-only configs; Stage 2's
  functional test hit the limitation immediately. Fix: fall back to
  the listKey when `fields["peer"]` is empty. For multi-key lists
  with more complex key structure, ze's config syntax would need an
  extension (e.g., a key=value prefix).
- **gosec G404 flags any `math/rand` use**, even in non-security
  contexts. The `.golangci.yml` excludes G104/G115/G602/G703/G704/
  G705/G706 but not G404. The accepted pattern across the codebase
  (see `internal/chaos/chaos.go`) is `//nolint:gosec` on the line with
  a one-sentence rationale. Adding G404 to the excludes list would be
  cleaner but was out of scope for this spec.
- **goimports + auto-linter strip unused aliased imports immediately.**
  Adding `cryptorand "crypto/rand"` as a new import and then using it
  in a subsequent edit fails because the linter runs between edits and
  removes the "unused" import. Workaround: add import and usage in a
  single atomic edit that the linter can validate end-to-end.
- **`time.Duration * float64` needs explicit conversion.** The Go type
  checker rejects `base * f` where base is `time.Duration` and f is
  `float64`; use `time.Duration(float64(base) * f)`.
- **`IP_RECVTTL` delivers the TTL as a 32-bit int in host byte order**
  inside an IP_TTL control message (not an IP_RECVTTL cmsg type).
  Parser must match on `unix.IP_TTL`, not `unix.IP_RECVTTL`. The
  setsockopt uses `IP_RECVTTL` (enable flag); the cmsg type is
  `IP_TTL` (value carrier).
- **`SO_BINDTODEVICE` on `lo` worked for an unprivileged test user**
  on the development machine -- the assumption that it requires
  CAP_NET_RAW is conservative but not universal. `TestUDPBindToDeviceLoopback`
  therefore does not skip on non-root; it fails only on kernels that
  do enforce the capability, and emits a Skipf with the underlying
  error so the skip is visible.
- **Node-wide port 3784 collision risk in functional tests.** The
  functional test binds UDP 3784 for its duration. Running two copies
  of the test in parallel would collide; ze-test serialises `.ci`
  tests within the same suite so this is fine in practice, but a
  future parallel runner would need to coordinate.
- **`docs/architecture/bfd.md` had a hard-coded "Next session: start
  here" section pointing at Stage 1.** The spec closed it by replacing
  that block with a "Stage 2 complete" table and pointers to the
  Stage 3/4/5/6 specs. Forgetting to update the architecture doc
  would leave new contributors following a stale roadmap.

## Files

- `internal/plugins/bfd/session/session.go` -- new `MinTTL()` and
  `DetectMult()` getters on `Machine`, `// Detail:` back-references to
  fsm.go / timers.go.
- `internal/plugins/bfd/session/timers.go` -- new
  `AdvanceTxWithJitter(now, reduction)` paired with the existing
  `AdvanceTx(now)`.
- `internal/plugins/bfd/engine/engine.go` -- new
  `JitterMaxFraction` / `JitterMinFractionDetectMultOne` constants and
  `applyJitter` helper using `math/rand/v2.Float64()` under a
  `//nolint:gosec` rationale.
- `internal/plugins/bfd/engine/loop.go` -- new `passesTTLGate` helper
  with RFC 5881 §5 / RFC 5883 §5 comments; `handleInbound` consults it
  after session lookup; `tick` uses `AdvanceTxWithJitter` with a
  per-call jitter reduction from `applyJitter`.
- `internal/plugins/bfd/engine/ttl_test.go` -- NEW. Table-driven unit
  tests for the gate across single-hop, multi-hop, and unknown-mode
  cases.
- `internal/plugins/bfd/engine/jitter_test.go` -- NEW. 10 000-draw
  bounds check per detect-multiplier band plus 20 000-draw uniformity
  check against the theoretical midpoint within 2%.
- `internal/plugins/bfd/transport/udp.go` -- rewritten `Start()` to
  use `net.ListenConfig.ListenPacket` with a Control callback;
  `readLoop` replaced with `ReadMsgUDPAddrPort` + oob parsing; new
  `Device` field; pre-allocated `oobBacking` slice.
- `internal/plugins/bfd/transport/udp_linux.go` -- NEW. `applySocketOptions`
  sets `IP_RECVTTL`, `IP_TTL=255`, and optionally `SO_BINDTODEVICE`;
  `parseReceivedTTL` parses `IP_TTL` cmsgs via
  `unix.ParseSocketControlMessage` + `binary.NativeEndian`.
- `internal/plugins/bfd/transport/udp_other.go` -- NEW. Non-Linux
  stub that rejects any device binding and returns zero on TTL
  extraction.
- `internal/plugins/bfd/transport/udp_ttl_linux_test.go` -- NEW.
  Round-trip test proves outgoing TTL=255 is observable via readback
  and `Inbound.TTL` is non-zero via IP_RECVTTL extraction.
- `internal/plugins/bfd/transport/udp_device_linux_test.go` -- NEW.
  SO_BINDTODEVICE tests for loopback, non-existent device (error
  wrapping), and empty device (skips bind-to-device path).
- `internal/plugins/bfd/config.go` -- `parseSingleHopSession` and
  `parseMultiHopSession` fall back to the listKey when `fields["peer"]`
  is empty; `defaultVRFName` constant replaces the scattered
  `"default"` literal.
- `internal/plugins/bfd/bfd.go` -- `loopFor(key, device)` takes the
  device name; non-default VRF no longer errors; new
  `resolveLoopDevices` derives per-loop device from pinned sessions;
  `newUDPTransport(mode, vrf, device)` signature change.
- `test/plugin/bfd-transport-stage2.ci` -- NEW. Python orchestrator
  drives ze with one single-hop + one multi-hop pinned session,
  asserts the Stage 2 lifecycle log patterns including per-loop
  device naming.
- `docs/guide/bfd.md` -- status block flipped to "plugin live, BGP
  opt-in pending", GTSM / multi-VRF / jitter sections added with
  source anchors.
- `docs/features.md` -- new BFD row with source anchors.
- `docs/comparison.md` -- BFD integration flipped from `No` to `Partial`
  for ze (BGP opt-in still pending).
- `docs/architecture/bfd.md` -- "Next session: start here" block
  replaced with a "Stage 2 complete" table and Stage 3/4/5/6 pointers.
- `plan/deferrals.md` -- Stage 2 row marked `done`; new
  `spec-bfd-2b-ipv6-transport` row added with `open` status.
