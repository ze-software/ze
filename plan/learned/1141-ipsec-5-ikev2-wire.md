# 1141 -- ipsec-5 IKEv2 Wire Format Codec

## Context

Ze's native IKEv2 engine needs a wire format codec for encoding and decoding all IKEv2 message types defined in RFC 7296 and RFC 7427. This is the foundational layer: pure encode/decode with no state machine, no network I/O, and no cryptographic operations. The codec lives at `internal/component/ike/wire/` and follows Ze's buffer-first encoding convention.

## Decisions

- Chose `WriteTo(buf, off) int` / `ReadFrom(data) error` over a streaming reader/writer pattern, following the BGP wire codec convention (see buffer-writer.md)
- Chose skip-and-backfill for variable-length payloads (SA proposals, transforms) over pre-computing lengths, matching the existing BGP reactor pattern
- Chose to store SK (Encrypted) payload as raw ciphertext over importing crypto, keeping codec pure and deferring decryption to ipsec-6
- Chose `make([]byte)` in ReadFrom (decoding) over slice-into-input, because decoded data must outlive the input buffer; encoding uses only offset writes into caller buffers
- Chose flat payload type dispatch in `decodePayload()` over a registry pattern, since payload types are a closed set defined by the RFC

## Consequences

- The codec is a pure library with zero imports outside stdlib (encoding/binary, errors, fmt); no registration, no init()
- ipsec-7 (engine) will consume this directly: construct Message structs and call WriteTo for outbound, call ReadFrom on UDP datagrams for inbound
- Unknown payload types are handled per RFC 7296: skip if critical bit clear, reject if set
- Traffic selector decoding validates address size against TS type (4 bytes for IPv4, 16 for IPv6), catching malformed selectors early

## Gotchas

- Traffic selector address splitting must validate even length before dividing by 2; odd-length address data would silently drop a byte
- Proposal.WriteTo can panic if SPISize > len(SPI); added a guard, but callers must still set both fields consistently
- The delete payload's `NumSPIs * SPISize` multiplication is safe on 64-bit (uint16 * uint8 max = 16M), but an overflow guard is good defensive practice

## Files

- `internal/component/ike/wire/header.go` -- 28-byte IKEv2 header encode/decode
- `internal/component/ike/wire/payload.go` -- generic payload header, Payload interface, type dispatch
- `internal/component/ike/wire/payload_sa.go` -- SA payload with nested Proposal/Transform structures
- `internal/component/ike/wire/payload_ke.go` -- Key Exchange payload
- `internal/component/ike/wire/payload_nonce.go` -- Nonce payload (16-256 byte validation)
- `internal/component/ike/wire/payload_id.go` -- IDi/IDr identification payloads
- `internal/component/ike/wire/payload_auth.go` -- AUTH payload (methods 1, 2, 14)
- `internal/component/ike/wire/payload_cert.go` -- CERT and CERTREQ payloads
- `internal/component/ike/wire/payload_notify.go` -- Notify payload with type constants
- `internal/component/ike/wire/payload_delete.go` -- Delete payload with SPI list
- `internal/component/ike/wire/payload_vendor.go` -- Vendor ID payload
- `internal/component/ike/wire/payload_ts.go` -- TSi/TSr with IPv4/IPv6 traffic selectors
- `internal/component/ike/wire/payload_sk.go` -- SK (Encrypted) payload (raw ciphertext)
- `internal/component/ike/wire/payload_cp.go` -- Configuration payload with attributes
- `internal/component/ike/wire/payload_eap.go` -- EAP payload
- `internal/component/ike/wire/message.go` -- Full message with payload chain
- Plus `*_test.go` for each (30 tests total)
