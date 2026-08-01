# 928 -- isis-2-wire

## Context
Spec `isis-2-wire` is the IS-IS serialization boundary: a pure, self-contained codec
package `internal/component/isis/packet/` that parses received frames into PDU views
and serializes PDU structs back to bytes. It depends only on the domain types from
isis-1 (`internal/component/isis/types`) plus the standard library and
`internal/core/textbuf` for display. It contains no runtime, sockets, timers, LSDB, or
FSM; those live in the later children (isis-3 transport, isis-5 adjacency, isis-6 LSDB,
isis-7 flooding). The codec covers the common 8-octet header, all 9 PDU types, the core
TLV set for a dual-stack L1+L2 router (1, 2, 6, 8, 9, 10, 22, 129, 132, 135, 137, 232,
236, 240) with sub-TLVs 4/6/8, the ISO 8473 Fletcher checksum with the two-step
adjustment, and opaque passthrough of unknown TLVs. Implementation is DONE: the codec
compiles for darwin and linux, the package's 179 `TestISIS*` cases pass under `-race`,
and golangci-lint is clean.

## Decisions
- Per-TLV-family file split (core / neighbours / ipv4 / ipv6 / auth / opaque), not one
  file per TLV (bio-rd) nor one monolith (FRR). Sane file count, clear ownership,
  matches the umbrella layout.
- Decode is lazy and zero-copy: a PDU view holds the caller's slice plus offsets, and a
  `TLVIterator` yields `(type, value-slice)` without copying. Unknown TLVs are retained
  as opaque spans so the LSDB can re-flood them verbatim (`tlv_opaque.go`). The lifetime
  contract is written in `doc.go`: a decoded view is valid only while the caller's
  backing slice is stable; isis-6 copies the LSP bytes it retains.
- Encode is buffer-first: every PDU and TLV writes into a caller-owned buffer via
  `WriteTo(buf []byte, off int) int`. The PDU Length field and the LSP Fletcher checksum
  are written by skip-and-backfill, never a `Len()`-then-`WriteTo()` double traversal.
- Fletcher checksum isolated and vector-tested FIRST, before any PDU body (the
  highest-risk item, R-1). `Checksum` (encode direction, returns the two adjusted
  octets) and `VerifyChecksum` (re-sum, both sums zero) are separate, separately tested
  functions in `checksum.go`.
- PDU type constants taken from ISO/IEC 10589 clause 9 (the authoritative values), NOT
  from the research guide sec 2 list, which transcribes the L1 codes incorrectly (it
  lists L1 LSP 0x18, L1 CSNP 0x24, L1 PSNP 0x26). `TestISISPDUConstants` pins the 9
  correct bytes (0x0f,0x10,0x11,0x12,0x14,0x18,0x19,0x1a,0x1b) so a regression cannot
  silently break interop.
- TLV 10 (authentication) is a STRUCTURAL codec only here: encode/decode the
  auth-type byte + opaque value, and report the TLV's index so a strict peer's
  first-TLV requirement can be enforced. The HMAC sign/verify, key store, and per-PDU
  enforcement are isis-10 (they grew into `auth_types.go`/`auth_sign.go`/
  `auth_verify.go` in the same package directory).
- TLV 6 (IS Neighbours, the 6-byte SNPA list) is encode+decode (REQUIRED for LAN
  three-way adjacency, isis-5). TLV 2 (narrow IS Reachability) is DECODE-ONLY: it parses
  a peer's bytes but has no encoder (Ze originates the wide TLV 22 instead).
  `TestISISTLV2NoEncoder` pins the absence of a `WriteTo` on the narrow type.
- The wiring proof is a dedicated OFFLINE root verb `ze isis-decode`
  (`internal/component/isis/cli/`), intentionally distinct from the `isis` config root
  (isis-4) and the `show isis` tree (isis-13). It reads a hex blob from stdin and emits
  a JSON PDU view; the CLI uses `textbuf` for diagnostics, never `fmt.Sprintf`.

## Consequences
- Metric widths differ by TLV and the codec keeps them distinct: TLV 22 IS metric is
  3 octets (24-bit), TLV 135/236 prefix metric is 4 octets (32-bit). The 32-bit metric
  is never capped at 24-bit; boundary tests pin 16777215 and 4294967295.
- The up/down bit lives in the control/flags octet of TLV 135/236, not in the high bit
  of the metric (a classic IS-IS trap). The 1-octet sub-TLV-length field is emitted and
  parsed ONLY when the sub-TLV-present (S) bit is set; `TestISIS*NoSubTLVNoLengthOctet`
  pins that no spurious length octet appears when S is clear.
- A fixture pin (`gen_ci_test.go`, `TestISISCIFixtureDecodes`) ties the exact bytes of
  `test/isis-wire/isis-pdu-1.ci` to `DecodePDU`, so the functional fixture and the codec
  cannot drift. Sibling specs added the same pinning pattern for their fixtures
  (`flood_ci_test.go` for isis-7, `pseudonode_ci_test.go` for isis-8).

## Gotchas / Traps
- **Functional-test path moved.** The plan named `test/decode/isis-pdu-1.ci`; the
  implemented location is `test/isis-wire/isis-pdu-1.ci`, registered as the `isis-wire`
  CI root in `internal/test/cli/register.go` and run by `make ze-isis-wire-test`. An
  additional `test/isis-wire/isis-truncated.ci` covers the AC-11 error path (lone 0x83
  -> exit 1 + `error: decode PDU`). The spec's tables were updated to the actual paths
  during closure; `test/decode/isis-pdu-1.ci` does NOT exist.
- **`TestISISTLVCoreRoundTrip` was split** into three named per-TLV tests
  (`TestISISTLVAreaAddressesRoundTrip`, `TestISISTLVLSPEntriesRoundTrip`,
  `TestISISTLVProtocolsSupported`); the behaviour is fully covered, the single planned
  name just does not exist.
- **Fletcher arithmetic is mod 255, not mod 256.** A computed 0 octet is stored as 0xFF
  (255 == 0 mod 255) so the field can never collide with the reserved all-zero
  "checksum not computed" value while still verifying. The closed-form adjustment is
  `X = (m-1)*C0 - C1` and `Y = C1 - m*C0` (mod 255) where `m = L - checkOff`; see the
  derivation comment in `checksum.go`.
- **`ID length 0` on the wire means 6.** The common header accepts both 0 and 6 on
  receive and sends 6 (Ze fixes the System ID at 6 octets). The header parser rejects
  any other value.
- The auth concern outgrew the single planned `tlv_auth.go`: the structural TLV 10
  codec lives in `tlv_auth.go` (this spec), the sign/verify lives in
  `auth_types.go`/`auth_sign.go`/`auth_verify.go` (isis-10) in the same directory. When
  auditing this spec, only the structural codec is in scope.

## Interop validation pending Linux execution
On-wire interop with FRR `isisd` is owned by the runtime children (isis-13), not by this
codec child. The interop scenario files are WRITTEN under `test/interop/scenarios/`
(`isis-p2p-frr`, `isis-lan-dis-frr`, `isis-dualstack-frr`, `isis-auth-frr`,
`isis-convergence-frr`, `isis-redist-frr`), each with `check.py`/`frr.conf`/`ze.conf`,
and each self-documents that it runs under the Linux Docker/QEMU interop harness ONLY
(raw L2 + FRR isisd) and cannot run on darwin. These were NOT executed on the darwin
host where this work was done: interop validation is pending Linux execution. The codec
itself is validated on darwin by the exhaustive round-trip unit tests, the Fletcher
vector tests, the fuzz targets, and the offline `test/isis-wire/` decode functional
tests.

## Files
- `internal/plugins/isis/packet/header.go` -- common 8-octet header codec + 9 PDU type
  constants (pinned); `pdu.go` -- top-level `DecodePDU` dispatch.
- `internal/plugins/isis/packet/hello.go` (LAN L1/L2 IIH, P2P IIH), `lsp.go` (LSP +
  checksum backfill), `csnp.go`, `psnp.go` -- the 9 PDU bodies.
- `internal/plugins/isis/packet/checksum.go` -- Fletcher two-step `Checksum` +
  `VerifyChecksum`.
- `internal/plugins/isis/packet/tlv.go` (iterator + framing + TLV constants),
  `tlv_core.go` (1/8/9/22+subs/129/137/240), `tlv_neighbours.go` (6 encode+decode, 2
  decode-only), `tlv_ipv4.go` (132/135), `tlv_ipv6.go` (232/236), `tlv_auth.go` (10
  structural), `tlv_opaque.go` (unknown-TLV verbatim retention).
- `internal/plugins/isis/packet/json.go` (PDU JSON view), `doc.go` (package contract).
- `internal/plugins/isis/cli/{register,run,decode}.go` -- offline `ze isis-decode` verb.
- `internal/test/cli/register.go` -- registers the `isis-wire` CI root.
- `test/isis-wire/isis-pdu-1.ci` (decode of a captured LAN L1 IIH),
  `test/isis-wire/isis-truncated.ci` (error path).
- `*_test.go` across the package (179 `TestISIS*` cases + 3 fuzz targets); fixture pin
  `gen_ci_test.go`.
- Docs: `docs/architecture/wire/isis.md` (PDU/TLV codec, layering `types <- packet`),
  `docs/functional-tests.md` (`test/isis-wire/` suite + `make ze-isis-wire-test`).
