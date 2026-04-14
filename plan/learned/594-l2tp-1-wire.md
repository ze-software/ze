# 594 — L2TP-1 Wire Format

## Context

Ze is growing an L2TPv2 (RFC 2661) subsystem that will act as both LNS and
LAC for BNG workloads (PPP sessions tunneled over UDP, subscriber routes
redistributed into BGP). The umbrella spec splits the work into eight
phases; phase 1 is the pure-library wire layer.

Before this phase the repository had zero L2TP code. The data-plane split
for Linux is clean (kernel `l2tp_ppp` handles encap/decap), so everything
ze needs to implement is control-plane: messages on UDP/1701, AVP-carrying
payloads, challenge/response authentication, and hidden-AVP encryption.
Phase 1 delivers the parse/encode primitives used by every later phase.

## Decisions

- **Flat package `internal/component/l2tp/`** (no `wire` subpackage). Matches
  the integration guide; later phases add `reactor.go`, `tunnel.go`,
  `session.go` without a namespace jump. Chose this over BGP-style
  subpackages because L2TP is smaller (~40 AVPs, no extended-length
  variant, no address-family zoo).
- **Offset-based `AVPIterator`** returning `(vendorID, attrType, flags,
  value []byte, ok)`. Same pattern as `attribute.AttrIterator` in BGP.
  `value` is a subslice of the caller's buffer — zero copy. `Err()`
  reports the cause when `ok=false` is from malformed data rather than
  exhaustion.
- **Typed writers over generic.** `WriteAVPUint16/32/64/String/Bytes/Empty`
  exist even though `WriteAVPBytes` + caller scratch would be enough,
  because the typed versions avoid the caller-side scratch allocation and
  keep encoding sites short. Compound helpers for the five AVPs with
  non-primitive layouts (Result Code, Q.931 Cause, Call Errors, ACCM,
  Proxy Authen ID).
- **Caller-provided scratch for hidden AVPs.** `HiddenEncrypt` /
  `HiddenDecrypt` take a `dst []byte` parameter. No internal
  `make([]byte)` — phase 3 will pass a second pool buffer for the
  encrypt/decrypt scratch region.
- **Stack-backed MD5 input.** `ChallengeResponse` builds the concatenated
  input in a 128-byte stack buffer and only falls back to `make` for
  secret+challenge > 127 bytes. Happens once per tunnel setup, never on
  hot path.
- **`ze l2tp decode` CLI** as the wiring test entry point. Offline, no
  daemon, no YANG, no env vars. Hex stdin -> JSON stdout. The minimum
  that lets a user exercise the library.
- **Rename `Header` → `MessageHeader` and `ParseHeader` →
  `ParseMessageHeader`** because `check-existing-patterns.sh` flagged
  collisions with `internal/component/bgp/message.Header` and
  `internal/component/bgp/attribute.ParseHeader`. Mechanical cost, zero
  semantic cost.
- **Precondition panics (`panic("BUG: ...")`) over silent truncation.**
  `WriteAVPHeader` rejects `totalLen` outside `[AVPHeaderLen, AVPMaxLen]`;
  `WriteAVPString` rejects short destination buffers; `ChallengeResponse`,
  `HiddenEncrypt`, and `HiddenDecrypt` reject empty secret / Random Vector
  / challenge. The buffer-first "caller sizes" convention is still in
  force, but the failure mode is now loud instead of silent-corrupt. The
  `block-panic-error.sh` hook allows `panic("BUG: ...")` for programmer
  errors, so the guards sit cleanly inside the existing rules.
- **Version-before-length in `ParseMessageHeader`.** The first two bytes
  carry the version word; only after validating Ver == 2 do we enforce
  the 6-byte minimum. Phase 3 needs to distinguish L2TPv3, L2F, and
  truncated L2TPv2 to react correctly; the initial order collapsed all
  three into `ErrShortBuffer`.

## Consequences

- Phases 2-8 can assume a stable wire API. `MessageHeader`, `AVPIterator`,
  the `Write*`/`Read*` families, `ChallengeResponse`, and `HiddenEncrypt`
  /`HiddenDecrypt` are the surface the reactor and state machines will use.
- Buffer-first is enforced across the whole wire layer. Any phase that
  adds a new `Write*` helper must follow the offset+return-int pattern.
- The integration-completeness rule was satisfied via a Go integration
  test (`cmd/ze/l2tp/decode_test.go`) that exercises the decoder from its
  user entry point by redirecting stdin/stdout/stderr. The `.ci` files
  under `test/l2tp-wire/` are specification artifacts for a future
  ze-test category; they are not auto-discovered today.
- Fuzz targets guard the parse perimeter: `FuzzParseMessageHeader`,
  `FuzzAVPIterator`, `FuzzHiddenDecrypt`. Each runs clean for 5s; seed
  corpora are committed.
- Deferred items logged in `plan/deferrals.md`: AC-14
  (oversize-buffer contract — cancelled as "caller sizes" is the package
  convention) and the L2TP ze-test runner category (open, destination
  `spec-l2tp-3-tunnel`).

## Gotchas

- **Length field validation is two-step.** The parser's initial Length
  sanity check (Length >= off) only bounds Length after the L field is
  parsed (off is still 4 at that point). Fuzz surfaced a case where a
  tiny `Length` slipped through for a header that carried S=1 / O=1 and
  had PayloadOff=12. Added a final check `Length >= PayloadOff` at the
  end of parse.
- **Hidden AVP chain direction matters.** The first draft of `hidden.go`
  tried to share a single XOR loop between encrypt and decrypt. It
  cannot: the chain key is `MD5(secret || prev_ciphertext)`, and on
  encrypt `prev_ciphertext` is what we just wrote, while on decrypt it
  is what the input contained. Two loops with explicit direction is
  clearer and smaller.
- **`_, _ =` is blocked project-wide.** Either handle the error or
  switch to a one-shot API (`md5.Sum(buf)` instead of
  `h.Write(...)`+`h.Sum`). Also `nolint:gosec` requires the `// reason`
  tail per the project linter rules.
- **`type Header` is reserved.** The duplicate-type hook blocks new
  files that define `Header` even across package boundaries. Prefix
  with the protocol name (`MessageHeader`).
- **`test/parse/` runner is config-only.** The ze-test parse runner
  rejects `.ci` files whose stdin block is `stdin=payload:hex=`.
  Dedicated directories (`test/l2tp-wire/`) stay out of its reach.
- **`copy()` silently truncates.** When you pass a string to
  `copy(buf[off:], s)`, the return count tells you the truth; the helper
  does not. If the helper's public contract is "wrote total bytes" and
  the wire format carries that total in a length field, silent
  truncation produces valid-looking but wrong bytes. Compare the copy
  count against `len(s)` or panic.
- **Masking a 10-bit Length field is an invisible bug.** `uint16(n) &
  0x03FF` for `n >= 1024` quietly drops the overflow and packs a wrong
  length into the wire. Validate at the API boundary, not inside the
  bit-packing expression.
- **Empty crypto inputs are unsafe, not merely "weird".** MD5 keyed on
  `secret=""` or `rv=""` collapses to something deterministic per
  attribute type; the subsystem should refuse such config, but the wire
  helper is the last line of defense and must say no.
- **Run `/ze-review` before claiming done.** The review found five
  hardening opportunities that unit tests and fuzz missed. Cheap relative
  to the rework of finding them in a later phase or after ship.

## Files

- `rfc/short/rfc2661.md` — RFC summary.
- `docs/architecture/wire/l2tp.md` — architecture doc for the wire layer.
- `docs/guide/command-reference.md` — `ze l2tp decode` section.
- `internal/component/l2tp/`
  - `doc.go`, `errors.go`, `pool.go`
  - `header.go` (+ `header_test.go`, `header_fuzz_test.go`)
  - `avp.go`, `avp_compound.go` (+ `avp_test.go`, `avp_compound_test.go`, `avp_fuzz_test.go`)
  - `auth.go` (+ `auth_test.go`)
  - `hidden.go` (+ `hidden_test.go`, `hidden_fuzz_test.go`)
  - `roundtrip_test.go`
- `cmd/ze/main.go` — subcommand registration.
- `cmd/ze/l2tp/main.go`, `decode.go`, `decode_test.go`.
- `test/l2tp-wire/decode-sccrq.ci`, `decode-truncated.ci`.
