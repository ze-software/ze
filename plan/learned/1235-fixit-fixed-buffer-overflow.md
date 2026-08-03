# 1235 -- fixit-fixed-buffer-overflow

## Context

Two encoder families built a wire message into a FIXED-SIZE buffer and indexed
directly into it, with no capacity guard. Because the encoders index `buf[off]`
before returning the written length, an oversized message panics with a
slice-bounds error INSIDE the encoder (before `n` is returned), so the classic
post-hoc `n > cap(buf)` check at the caller is DEAD CODE -- control never reaches
it. The two families (the only unguarded members of the class; the repo-wide
sweep found every other fixed-buffer + `WriteTo`/`Build` site already guarded):

- **IKE `wire.Message.WriteTo`** into `make([]byte, 512|4096)` at
  `responder.go` (`sendSAInitNotify` 512, `buildSAInitResponse` 4096),
  `initiator.go` (`buildSAInitRequest` 4096), `dpd.go` (`sendDPD` 512).
- **L2TP `BuildDHCPv6Reply` / `BuildDHCPv6StatusReply`** into `var buf [512]byte`
  at the 5 `ipv6_service.go` DHCPv6-PD call sites.

Both are latent (not remotely reachable today: IKE emits only tiny
NO_PROPOSAL/INVALID_KE notifies; the parsed DHCPv6 ClientID is capped at
`maxDUIDLen=128`), so this is class hardening, not a live-bug fix.

## Decisions

- **Contract home = per-encoder methods in each package, NOT a shared interface**
  (D-1). BGP already has `CheckedBufWriter` (`internal/core/bgp/wire/writer.go`)
  but importing it into IKE/L2TP would couple two components to BGP wire (and the
  Architectural Verification forbids that). Nothing consumes the contract
  polymorphically here -- every caller holds a concrete type -- so a shared
  `internal/core` interface would be speculative generality. IKE `*wire.Message`
  and the L2TP DHCPv6 builders each grow their own `Len()`+`CheckedWriteTo` /
  `Checked*` methods mirroring the CheckedBufWriter SHAPE with zero new
  cross-package imports. `make ze-tier-check` stays green by construction (no new
  import edges).
- **`Len()` added to the IKE `Payload` interface** (D-2), implemented on all 22
  payload/sub-structure types (SA/Proposal/Transform, KE, Nonce, Notify, TS/TS-sel,
  SK, AUTH, CERT/CERTREQ, CP, Delete, EAP, ID, VendorID, Raw). `Message.Len()` =
  `HeaderLen + Σ(GenericHeaderLen + payload.Len())`; `CheckedWriteTo` rejects when
  `len(buf)-off < Len()` and otherwise delegates to the untouched `WriteTo`, so
  fitting messages are byte-identical (AC-3). The compiler enforces completeness.
  Drift risk (Len disagreeing with WriteTo) is pinned by `wire/len_test.go`
  asserting `Len()==WriteTo` for every type.
- **IKE `sendSAInitNotify`/`sendDPD`/`buildSAInitResponse` call `CheckedWriteTo`
  and SKIP+LOG on overflow** (D-3): a truncated IKE message is malformed, worse
  than a panic (`ai/rules/completion.md`), so never
  copy-truncate. `buildSAInitResponse` gained an `error` return; its sole caller
  (`handleSAInitRequest`) logs peer+length and sets `StateDead`.
- **IKE `buildSAInitRequest` uses `make([]byte, msg.Len())` (Len-first sizing),
  NOT the error-returning checked path** (D-4). Its signature could not change:
  its callers live in RFC-tagged test files (`responder_test.go`,
  `rfc7296_test.go`) that `.claude/hooks/pretool-writeedit.py` blocks from edits
  without user approval. Sizing the buffer to the message length is the OTHER
  spec-endorsed durable fix and is provably panic-proof (buffer == message
  length). Legitimate here because the request is built entirely from ze's own
  config (proposals, our DH pubkey) -- its length is not remotely influenced, so
  there is nothing to "skip+log": it simply always fits.
- **L2TP handlers return `(nil, err)` on overflow; the DHCPv6 server loop logs it**
  (D-5). `dhcpv6ServerLoop` already logs `HandleDHCPv6` errors at Warn and skips
  the send, so propagating the checked-builder error routes cleanly to the
  existing log-and-drop path. `noPrefixAvailReply` gained an `error` return too.
  Raw `Build*` stay as the unchecked primitive (the BufWriter/CheckedBufWriter
  split); callers use only the `Checked*` wrappers.

## Consequences

- An oversized IKE notify or DHCPv6 reply is now an explicit logged drop, never a
  slice-bounds panic and never a truncated packet. Proven by
  `TestSendSAInitNotifyOversizedRejected` (drives real loopback transports;
  asserts no-panic + not-received + logged peer/length) and
  `TestDHCPv6ReplyOversizedRejected` (drives the real `HandleDHCPv6` entry with an
  oversized server DUID). Both PANIC on the pre-fix code (verified by temporarily
  reverting the two guards) and pass after.
- Byte-identical fitting output pinned by `TestSendSAInitNotifyBytesUnchanged` +
  `TestDHCPv6ReplyBytesUnchanged` (checked == raw, and `Len` == written length
  across all DUID types).

## Gotchas

- The panic is INSIDE the encoder's `WriteTo`/`Build`, before the length is
  returned -- a `n > cap(buf)` guard at the caller is DEAD CODE. Only a
  `Len()`-first allocation or a `CheckedWriteTo` that sizes before indexing
  actually prevents it.
- A signature change ripples into RFC-tagged test files, which a hook blocks.
  Prefer Len-first sizing (no signature change) when the message length is
  ze-controlled; reserve the error-returning checked path for the
  remotely-influenced sites.
- OUT OF SCOPE (noted follow-up, `ai/rules/completion.md`): `BuildRA`
  (`ra.go`, into `var buf [256]byte` at `ra_linux.go`) is the same
  unguarded-builder pattern with a variable-length RDNSS option; it cannot
  overflow today only because its sole caller passes a fixed no-RDNSS `RAConfig`.
  It should adopt the same `Checked*` bound in a follow-up.

## Files

- internal/component/ike/wire/payload.go (`Len()` on the `Payload` interface + `PayloadRaw`)
- internal/component/ike/wire/message.go (`Message.Len`, `Message.CheckedWriteTo`)
- internal/component/ike/wire/payload_{sa,ke,nonce,notify,ts,sk,auth,cert,cp,delete,eap,id,vendor}.go (`Len()` per type; SA adds `Transform.length`/`Proposal.length`)
- internal/component/ike/engine/responder.go (`sendSAInitNotify` + `buildSAInitResponse` checked; caller skip+log)
- internal/component/ike/engine/dpd.go (`sendDPD` checked)
- internal/component/ike/engine/initiator.go (`buildSAInitRequest` Len-first sizing)
- internal/component/l2tp/ppp/dhcpv6.go (`duidLen`/`dhcpv6ReplyLen`/`dhcpv6StatusReplyLen` + `CheckedBuildDHCPv6Reply`/`CheckedBuildDHCPv6StatusReply`)
- internal/component/l2tp/ppp/ipv6_service.go (5 call sites + `noPrefixAvailReply` use the checked builders)
- internal/component/ike/wire/len_test.go, internal/component/ike/engine/overflow_test.go, internal/component/l2tp/ppp/dhcpv6_overflow_test.go
