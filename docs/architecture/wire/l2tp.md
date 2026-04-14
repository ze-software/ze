# L2TPv2 Wire Format

Ze implements L2TPv2 per [RFC 2661](../../../rfc/short/rfc2661.md).
Phase 1 (see `plan/learned/NNN-l2tp-1-wire.md`) delivers the wire layer: header parsing
and serialization, AVP iteration and serialization, challenge/response
computation, and hidden-AVP encryption. All code lives in
`internal/component/l2tp/`.

<!-- source: internal/component/l2tp/doc.go — package overview -->

## Package surface

| Area | File | Key symbols |
|------|------|-------------|
| Buffers | `pool.go` | `BufSize`, `GetBuf`, `PutBuf` |
| Errors | `errors.go` | `ErrShortBuffer`, `ErrUnsupportedVersion`, `ErrMalformedControl`, `ErrInvalidAVPLen`, `ErrHiddenLenMismatch` |
| Header | `header.go` | `MessageHeader`, `ControlHeaderLen`, `ParseMessageHeader`, `WriteControlHeader`, `WriteDataHeader` |
| AVP | `avp.go` | `AVPType` + catalog constants, `MessageType` + 14 message codes, `AVPFlags`, `AVPIterator`, `WriteAVP{Header,Bytes,Empty,Uint8,Uint16,Uint32,Uint64,String}`, `ReadAVPUint{8,16,32,64}` |
| Compound AVPs | `avp_compound.go` | `ResultCodeValue`, `Q931CauseValue`, `CallErrorsValue`, `ACCMValue`, `ProxyAuthenIDValue`, `ProtocolVersionValue` |
| Challenge/response | `auth.go` | `ChallengeResponse`, `VerifyChallengeResponse`, `ChapIDSCCRP`, `ChapIDSCCCN` |
| Hidden AVPs | `hidden.go` | `HiddenEncrypt`, `HiddenDecrypt` |

<!-- source: internal/component/l2tp/header.go — header parser -->
<!-- source: internal/component/l2tp/avp.go — AVP catalog and iterator -->

## Header layout

| Offset | Bytes | Field | Notes |
|--------|-------|-------|-------|
| 0 | 2 | Flags + Version | Control messages are `0xC802` (T=1,L=1,S=1,O=0,P=0,Ver=2) |
| 2 | 2 | Length | Present when L=1. Total message length in octets. |
| 2-4 | 2 | Tunnel ID | Offset shifts when L=0 |
| 4-6 | 2 | Session ID | |
| 6-8 | 2 | Ns | Present when S=1 |
| 8-10 | 2 | Nr | Present when S=1 |
| n | 2 | Offset Size | Present when O=1 |
| n+2 | Offset Size | Offset Pad | Uninitialized on send; skipped on receive |

Control messages are fixed 12 octets. Data messages vary from 6 to 14+N
octets depending on flags.

`ParseMessageHeader` returns a `MessageHeader` value carrying the decoded
flag bits and numeric fields, plus a `PayloadOff` offset into the input
slice where AVPs (or a PPP frame, for data messages) begin.

## AVP layout

| Offset | Bytes | Field | Notes |
|--------|-------|-------|-------|
| 0 | 2 | Flags + Length | M (bit 0), H (bit 1), Reserved (bits 2-5), Length (bits 6-15; 10 bits, 6-1023) |
| 2 | 2 | Vendor ID | 0 = IETF standard |
| 4 | 2 | Attribute Type | |
| 6 | Length-6 | Value | |

`AVPIterator` walks this stream without copying. `Next()` returns
`(vendorID, attrType, flags, value, ok)` where `value` is a subslice of the
iterator's input. On the first malformed AVP the iterator returns
`ok=false` and `Err()` reports the cause.

## Buffer discipline

All encoding helpers write into caller-provided buffers. No function in the
package allocates a `[]byte` on the hot path. Callers typically:

1. Get a 1500-byte buffer from `GetBuf`.
2. Reserve `ControlHeaderLen` bytes at the start, write AVPs from offset 12.
3. Write the header last via `WriteControlHeader` so the total length is known.
4. Send the filled slice, then `PutBuf` the pointer.

See `.claude/rules/buffer-first.md` for the mechanical rules this package
implements.

## Hidden AVP cipher (RFC 2661 Section 4.3)

The hidden-AVP value is encrypted as a stream of MD5 blocks keyed by the
shared secret and the per-message Random Vector. The first block's key
input includes the AVP Attribute Type, so two hidden AVPs with the same
Random Vector but different attribute types produce different key streams.

`HiddenEncrypt` and `HiddenDecrypt` take caller-provided scratch buffers.
The encoder prepends a 2-byte Original Length field before XOR-ing the
result with the keystream; the decoder reverses this and extracts the
original value by that length.

## Challenge/response (RFC 2661 Section 4.2)

`ChallengeResponse(chapID, secret, challenge)` computes
`MD5(chapID || secret || challenge)`. `chapID` is the Message Type of the
response-bearing message: 2 for SCCRP, 3 for SCCCN. Verification uses
`crypto/subtle.ConstantTimeCompare`.

## CLI

`ze l2tp decode` reads hex from stdin and emits JSON describing the parsed
header and AVPs. See `docs/guide/command-reference.md`.

<!-- source: cmd/ze/l2tp/decode.go — decode subcommand -->
