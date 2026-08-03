# 1027 -- DNS server harness: extract geodns primitives to internal/core/dnsserver

## Context

geodns's DNS server internals (listener lifecycle, EDNS0 client-IP resolution,
authoritative-answer/recursion-refusal shaping, CIDR longest-prefix matcher)
were the only reusable primitives an upcoming second DNS plugin
(`spec-as112-2-dns-server.md`) needs, but a plugin MUST NOT import a sibling
plugin (`plugins.md`). The only sanctioned reuse path is a lower
tier -- the pattern `internal/core/probe` already set for ping+traceroute.
This spec extracted those primitives into a new core leaf package
`internal/core/dnsserver` and migrated geodns onto it behavior-preservingly,
unblocking as112 without touching it.

## Decisions

- Extracted to `internal/core/dnsserver` (core leaf) over (a) as112 importing
  geodns [forbidden, `plugins.md`], (b) duplicating the
  authoritative-only handler in as112 [security-sensitive duplication risk],
  (c) a component [no config-driven lifecycle -> `architecture.md` puts it in
  core]. `internal/core/probe` is the precedent.
- Overrode the "3+ use cases" heuristic at 2 consumers (geodns + approved
  imminent as112), same as `internal/core/probe` did for ping+traceroute --
  the alternative to extraction was a forbidden import or a security-sensitive
  copy-paste, not "wait for a 3rd".
- `Authoritative` makes the recursion-refusal guard an *enforced invariant*,
  not a convention (AC-5, the single-source recursion guard): `AnswerFunc`
  receives a read-only `Peer` (RemoteAddr only, no WriteMsg), never the full
  `dns.ResponseWriter`, so an answer func cannot write its own reply; the
  wrapper owns the single `w.WriteMsg` and re-asserts the authoritative shape
  via `shapeAuthoritative` (authoritative bit, no recursion, no compression)
  both before AND after fn, so even an answer func that sets
  `msg.RecursionAvailable = true`/`Compress = true` cannot put it on the wire.
  `Peer` is satisfied by `dns.ResponseWriter`, so the wrapper passes it through
  with no per-query allocation, and the packet source stays lazy
  (`RemoteAddr(p)` only on paths whose answer needs it -- a refused query pays
  nothing for it). Made as a post-review hardening (two review rounds) that
  replaced the first-landed design where fn held the full ResponseWriter and
  the guard was only a convention it had to remember; the shape lives in one
  `shapeAuthoritative` helper, not hand-copied at the two call sites.
- Process note: the invariant hardening above landed AFTER this spec was closed
  and deleted, made directly in the working tree by owner direction (no spec
  reopened). Recorded here so a later reader sees why the shipped `dnsserver`
  API (Peer-based `AnswerFunc`, wrapper-owned write) differs from what the
  closed spec's Current-Behavior text described.
- Kept geodns's own `apply(cfg)`/`stopAll()` call shape via a thin
  `geodnsServer` adapter over `dnsserver.Manager`, chosen over exposing
  `dnsserver.Manager` directly to `register.go`, because it meant geodns's
  `register.go` and almost all of its existing tests needed ZERO edits -- the
  harness's endpoint-agnostic `Apply(enabled, endpoints)`/`Stop()` signature is
  a deliberate divergence from geodns's config-shaped original, and the
  adapter absorbs that divergence in one place.
- `Freebind` is an opt-in `Options` field, default off, implemented as a
  `//go:build linux` / `!linux` split (mirroring
  `internal/plugins/dhcpserver/socket_linux.go`'s `SO_BINDTODEVICE` pattern)
  over requiring `golang.org/x/sys/unix` -- plain `syscall.SetsockoptInt` with
  the raw `IP_FREEBIND` constant (15, not exposed by the stdlib `syscall`
  package) needed no new dependency.
- Matcher genericized as `Entry{Prefix, Label string}` over keeping geodns's
  `HostSet` field name in core -- keeps "host-set" (geodns's own vocabulary)
  out of the core package while the longest-prefix mechanism is fully shared.

## Consequences

- A second DNS plugin can build `dnsserver.Manager`/`Authoritative`/
  `ClientIP`/`Matcher` directly without importing geodns; `TestManager_BindsAndServes`
  proves the harness works with zero geodns involvement.
- The recursion-refusal guard now lives in exactly one place
  (`dnsserver.Authoritative`) and is *enforced*, not merely offered: a consumer
  supplies an `AnswerFunc` that can only shape msg and choose send/drop, so any
  future consumer inherits the authoritative-DNS security invariant unbypassably
  instead of hand-copying it.
- geodns's metric names/values, `show geodns`, doctor check, and YANG config
  are all still fully owned by geodns -- the harness is metrics-agnostic
  (`Options.OnListenerChange` callback) and policy-agnostic (`answerQuestions`
  stays geodns's).
- `internal/core/dnsserver` needs no `tier_non_engine_categories.txt` manifest
  row -- it is a pure library with no `sdk.NewWithConn` and no registry side
  effect, confirmed by `dep_audit.py --check` passing with zero changes to
  that file.

## Gotchas

- Go's export rules mean "move a function/method to another package's public
  API" is not a cosmetic rename -- `lookup` became `Lookup` mechanically,
  forcing edits to 3 geodns test call sites (`source_test.go`, `state_test.go`)
  plus removing one whole test (`server_test.go`'s `TestClientIPSourceModes`,
  since the `clientIP` function it unit-tested no longer exists in geodns).
  None of these changed any assertion or expected value -- the moved tests are
  byte-identical in their new home (`internal/core/dnsserver/{client,matcher}_test.go`)
  -- but "every geodns test passes with NO edit" needed a documented,
  `test-relax`-marked exception rather than a literal zero-diff.
- A spec-authored mechanical verification command can be stricter than the
  actual acceptance criterion: the Deliverables Checklist's
  `! rg -q geodns internal/core/dnsserver/` matches ANY mention of "geodns"
  including doc comments, while AC-1's real requirement ("no geodns
  identifier") is about Go identifiers only. Doc comments naming the
  historical motivating consumer are normal practice (see
  `internal/core/probe`'s own header comment naming ping/traceroute) --
  reworded them to generic "a consumer plugin" phrasing rather than debating
  the check, since it cost nothing and removed the ambiguity entirely.
- `net.ListenConfig.ListenPacket`/`Listen` have pointer receivers, so
  `listenConfig(freebind).ListenPacket(...)` chained directly on a
  function-call result fails to compile (not addressable) -- caught by
  cross-vetting with `GOOS=linux go vet -tags integration
  ./internal/core/dnsserver/...` before any real Linux/QEMU run would have hit
  it. Assign to a local variable first.

## Files

- `internal/core/dnsserver/manager.go`, `manager_test.go` -- Manager (bind/apply/stop lifecycle)
- `internal/core/dnsserver/handler.go`, `handler_test.go` -- Authoritative wrapper (RFC 1035, recursion guard)
- `internal/core/dnsserver/client.go`, `client_test.go` -- ClientIP/RemoteAddr (RFC 7871)
- `internal/core/dnsserver/matcher.go`, `matcher_test.go` -- generic CIDR longest-prefix Matcher
- `internal/core/dnsserver/freebind_linux.go`, `freebind_other.go`, `freebind_integration_linux_test.go` -- opt-in IP_FREEBIND
- `internal/plugins/geodns/server.go` -- migrated onto the harness via a thin `geodnsServer` adapter
- `internal/plugins/geodns/source.go` -- `buildMatcher` now delegates to `dnsserver.Matcher`
- `internal/plugins/geodns/state.go` -- `resolverState.matcher` retyped to `*dnsserver.Matcher`
- `internal/plugins/geodns/server_test.go`, `source_test.go`, `state_test.go` -- mechanical call-site updates (see Gotchas)
