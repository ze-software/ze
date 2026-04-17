# 610 -- vpp-7 Test Harness

## Context

The `spec-vpp-0-umbrella.md` Wiring Test table declared seven
`test/vpp/NNN-*.ci` files (`001-boot` through `007-coexist`) as
evidence for every VPP phase. None existed, because running them
would require a real VPP process (root, DPDK vfio, hugepages, long
boot). The ze-test runner had no `vpp` category. fibvpp and the
`vpp` component were unit-tested in isolation but never driven
end-to-end against a GoVPP-shaped peer in CI. Goal: deliver a
Python-only stub and a `ze-test vpp` runner so the first two
umbrella tests light up, and so every subsequent VPP phase has a
place to prove its wiring.

## Decisions

- **Python 3 stdlib stub** over a Go in-process mock or a real VPP
  container. Stdlib keeps CI dep-free; binapi scraping
  (`MessageName` + `CrcString` regexes across
  `vendor/go.fd.io/govpp/binapi/*/*.ba.go`) keeps the stub in sync
  with whatever VPP release ze pins, no sidecar JSON to maintain.
- **New `vpp.external boolean default false` leaf** over a
  `vpp.binary-path = /bin/sleep infinity` dev hack. `external=true`
  has independent value for systemd-managed and gokrazy-sidecar
  deployments where ze is not VPP's supervisor, not just for the
  stub. `runOnce` skips `cmd.Start` / `cmd.Wait` and the outer
  `Run` skips `writeStartupConf` + `DPDKBinder.BindAll` when
  external; everything else (Connector, stats poller, event bus
  emissions) runs identically. `ze config validate` keeps accepting
  any `external` value because default false preserves the
  ze-owned-lifecycle behavior.
- **`ze-test vpp` reuses `EncodingTests` runner** over a bespoke
  lifecycle manager. The shared infrastructure handles tmpfs
  materialization, `$PORT` substitution, per-test timeouts, and
  stream capture; the new subcommand is ~155 lines of glue pointed
  at `test/vpp/`.
- **`002-fib-route` drives via real BGP UPDATE** (ze-peer
  `option=update:value=send-route`) rather than `bgp rib inject`
  through a plugin. Matches the handoff spirit ("peer announces a
  prefix, stub log contains ip_route_add_del is_add=true") and
  uncovered a CLI merge bug in `cmd/ze-test/peer.go` that was
  silently dropping `SendRoutes` from file-config merges.
- **Stub orders `sockclnt_delete` last** in the MessageTable sent
  to GoVPP clients. GoVPP's `open()` uses
  `strings.HasPrefix(name, "sockclnt_delete_")` to pick
  `sockDelMsgId` and lets later entries overwrite earlier ones;
  alphabetical order would make `sockclnt_delete_reply`'s MsgID win
  and the shutdown frame would be misrouted. The reorder is a
  workaround for a GoVPP-version-sensitive quirk, not a stub
  correctness issue.
- **Deferred Phase 3+ tests** (`003-fib-withdraw`, `004-vpp-restart`,
  `007-coexist`) and Phase 4 fault injection to separate sessions
  per the handoff. Phase-3/4 tests blocked on vpp-3 and vpp-4 are
  recorded in `plan/deferrals.md`.

## Consequences

- Every VPP phase from now on has a place to prove wiring. When
  vpp-3 (MPLS) or vpp-4 (iface) land, they just add
  `test/vpp/NNN-*.ci` files and extend `vpp_stub.py` with the new
  handlers.
- Fixing the `cmd/ze-test/peer.go` `SendRoutes` merge unblocks any
  future standalone `ze-test peer` use of `send-route`. The test
  runner's internal peer path was unaffected, which is why the bug
  was invisible before; any `.ci` file that shells out to ze-peer
  directly (this spec's 002-fib-route being the first) now works.
- `vpp.external` is a user-visible config leaf now. The docs
  (`docs/guide/vpp.md`, `docs/features.md`, `docs/functional-tests.md`)
  document the three use cases: systemd host, container sidecar,
  ze-test stub harness.
- Stub is 340 lines of Python; adding a new message type is 1-2
  handlers plus one header-size rule. Non-CIDR handlers (sockclnt,
  control-ping, ip_route_add_del) are the reference.

## Gotchas

- **GoVPP `sockclnt_create` is typed as `ReplyMessage` and
  `sockclnt_create_reply` as `RequestMessage`** in the vendored
  binapi -- the opposite of intuition. The stub's
  `_request_header_len` honors the declared type: 6-byte header
  for the incoming client sockclnt_create, 10-byte header for the
  reply it emits. Any stub extension that handles a new handshake
  message must check the vendored `GetMessageType` rather than
  guess.
- **GoVPP frame envelope has 16 bytes of pool-reuse padding**;
  only bytes 8..11 (big-endian uint32 payload length) are
  meaningful on read. Bytes 0..7 and 12..15 carry whatever was in
  the sync.Pool buffer at write time -- do not assume zeros.
- **`cmd/ze-test/peer.go` merges only specific fields** from the
  file-config it loads via `peer.LoadExpectFile`. The list hand-
  rolls every field; adding a new field to `peer.Config` requires
  adding it to the merge block too. The
  `SendRoutes` omission is fixed; the pattern is fragile.
- **`ze config validate` ignores YANG `enum`/`range` constraints.**
  It only rejects unknown leaves (via `unknownKeys`). Two
  pre-existing tests (`vpp-config-invalid-hugepage`,
  `vpp-config-invalid-poll-interval`) were failing on
  origin/main before this session started and still fail after --
  logged and corrected in `.claude/known-failures.md`. A full fix
  belongs in `internal/component/config/yang_schema.go`.
- **Stray `.go` files in project `tmp/`** (`debug_paths.go`,
  `old_vpn.go` -- scratch from prior sessions) block `go test ./...`
  because the Go toolchain walks every dir under the module root.
  Moved to `.go.txt` here. Memory entry
  (`rules/memory.md` "Scratch .go Files in tmp/") already
  documents the pattern.
- **`lg-lab.run` orphans in verify-fast.** Not in scope of this
  spec; fixed in a sibling commit by adding `Pdeathsig: SIGKILL`
  to the plugin SysProcAttr and `flock -o` to
  `scripts/dev/verify-lock.sh`. Prior to those, every verify-fast
  run leaked a Python plugin process that held the verify lock's
  inherited fd; the repo had ~200 orphans accumulated over a week.

## Files

- `internal/component/vpp/schema/ze-vpp-conf.yang` -- new `leaf external`
- `internal/component/vpp/config.go` -- `VPPSettings.External` + parse
- `internal/component/vpp/vpp.go` -- `Run`/`runOnce` external branch
- `internal/component/vpp/{config,vpp}_test.go` -- 5 new tests
- `test/scripts/vpp_stub.py` -- Python GoVPP-API stub
- `cmd/ze-test/vpp.go`, `cmd/ze-test/main.go` -- runner subcommand
- `cmd/ze-test/peer.go` -- `SendRoutes` merge fix
- `test/vpp/001-boot.ci`, `test/vpp/002-fib-route.ci` -- first two wiring tests
- `plan/spec-vpp-0-umbrella.md` -- child-spec row + MVP-backstop note
- `docs/guide/vpp.md`, `docs/features.md`, `docs/functional-tests.md` -- user-facing docs
