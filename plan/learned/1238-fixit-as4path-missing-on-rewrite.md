# 1238 -- fixit-as4path-missing-on-rewrite

## Context

`RewriteASPath` / `RewriteASPathDual` (`internal/component/bgp/wireu/`) substitute AS_TRANS
(23456) for a non-mappable (>65535) ASN when encoding AS_PATH for a peer that did NOT
negotiate the four-octet-AS capability, but never emitted the AS4_PATH attribute that RFC 6793
Section 4.2.2 MUSTs alongside it -- so the real ASN was irrecoverable (the receiver sees only
AS_TRANS). This was on the NORMAL eBGP forward path, not a corner case. The fix landed in
commit `fb3e6f20b` ("fix(bgp): emit AS4_PATH when transcoding to an old speaker"); this summary
records it at closure.

## Decisions

- **Single shared owner, consumed by both egress paths** (fix at the source, not two parallel
  copies). `internal/component/bgp/wireu/aspath_as4.go` owns `hasNonMappableASN`,
  `as4PathForPath`, `as4PathForRewrite`, `as4PathWireSize`, `writeAS4PathAttr`. The rewrite path
  (`aspath_rewrite.go` insert/fast-path/full) and the sibling `TranscodeASPath`
  (`aspath_transcode.go`) both call it; the sibling's old LOCAL `hasNonMappableASN` was
  deleted. Verified genuinely shared (not parallel) by mutation: returning nil from
  `as4PathForPath` reds 16 tests across BOTH `aspath_rewrite`/`as4_rfc6793` AND
  `aspath_transcode` test files.
- **Emit AS4_PATH iff a non-mappable ASN is present AND the destination is non-AS4.** All-mappable
  paths and AS4-capable peers get NO AS4_PATH (RFC 6793 MUST NOT add it needlessly) -- gated by
  tests. Confederation segments are excluded from the non-mappable check (`hasNonMappableASN` is
  confed-aware). A received AS4_PATH is prepended to, not replaced.
- **The hot path stays zero-alloc.** ASN4->ASN4 (`BenchmarkRewriteASPath/ASN4_to_ASN4`) is
  0 B/op, 0 allocs/op; the AS4_PATH construction lives on the ASN4->ASN2 slow path only.

## Consequences

- Ze is RFC 6793 S4.2.2-compliant on eBGP egress to an old (non-AS4) speaker: a >65535 ASN is
  recoverable by the receiver from AS4_PATH. Tagged `RFC6793-4.2.2-*` / `RFC6793-6-*`
  (`make ze-rfc-check` green); `docs/features/rfc-status.md` credits AS4_PATH construction.
- Receive-side reconstruction (S4.2.3) and NEW-to-NEW forwarding remain out of this spec's scope
  (still listed as gaps in rfc-status).

## Gotchas

- **The on-wire `.ci` proof could NOT be written** and is deferred (to
  `spec-fixit-redistribute-establishment-stall`, which owns the `.ci` harness contract):
  `internal/test/peer/checker.go` consumes ONE RULE PER MESSAGE, but a meaningful AS4_PATH
  assertion needs TWO facts about a SINGLE UPDATE (AS_TRANS in the 2-octet AS_PATH AND the real
  >65535 ASN in AS4_PATH); either alone cannot distinguish the fix from the bug. The behavior is
  instead proven byte-exact by unit tests (`TestRewriteASPath_AS4PathWireBytes`).
- A bare `go test ./internal/component/bgp/config/` false-reds on `TestExtractAuthzConfig_*` /
  `TestExtractSSHConfig*` ("unknown field in authentication: user" / "environment: ssh") -- that
  is the missing `ze_ssh`/authz build tags, NOT an AS4/aspath regression.

## Files

- internal/component/bgp/wireu/aspath_as4.go (NEW: shared AS4_PATH owner)
- internal/component/bgp/wireu/aspath_rewrite.go (emit AS4_PATH in all three write paths; hot path 0-alloc)
- internal/component/bgp/wireu/aspath_transcode.go (route through the shared owner; local dup deleted)
- internal/component/bgp/wireu/aspath_rewrite_test.go, as4_rfc6793_test.go (AC tests + RFC6793 tags), wire_bench_test.go (0-alloc benchmark)
