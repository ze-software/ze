# draft-abraitis-bgp-version-capability - Software Version Capability for BGP

## Meta

| Field | Value |
|-------|-------|
| Draft | draft-abraitis-bgp-version-capability-18 |
| Title | Software Version Capability for BGP |
| Status | Internet Draft (Informational, expired 2026-03-11) |
| Date | 2025-09-07 |
| Depends | RFC 5492 (capability advertisement), RFC 3629 (UTF-8), RFC 9072 (extended OPEN parameters) |

**Purpose:** Defines BGP capability code 75, which carries the routing software
version of a speaker so an operator can correlate a fault with a release.

**Scope:** Capability (OPEN message) only. The draft defines no UPDATE behavior.
Both advertising the version and processing a received one are OPTIONAL: the
Abstract states "This BGP capability is an optional advertisement" and
"Implementations are not required to advertise the version nor to process
received advertisements."

## Wire Format

Capability Code: 75 (0x4B). Capability Length: one octet, greater than zero.

```
+--------------------------------+
|  Version Length (1 octet)      |
+--------------------------------+
|  Version string (variable)     |
+--------------------------------+
```

The Capability Value is "the software version encoded as a UTF-8 [RFC3629]
string" (§3). It is not null-terminated. The value has the shape
`identifier = product ["/" product-version]`, for example `frrouting/8.4.2`.

## Ze Implementation

- Send: `encodeValue` (`internal/component/bgp/plugins/softver/softver.go`)
  writes one length octet followed by the constant `ZeVersion`, `"Ze/0.1.0"`.
- Advertisement gate: `extractSoftverCapabilities` (same file) appends a
  `sdk.CapabilityDecl` for code 75 only for a peer or a group whose config
  carries a `software-version` capability key whose mode is neither `disable`
  nor `refuse`. With no such key the capability is not declared at all.
- OPEN injection: `Peer.getPluginCapabilities`
  (`internal/component/bgp/reactor/peer.go`), wired at
  `internal/component/bgp/reactor/peer_run.go` through
  `session.SetPluginCapabilityGetter`.
- Receive: `parseCapability` (`internal/core/bgp/capability/capability.go`)
  has no case for code 75, so its default branch returns an `Unknown` holding
  the raw bytes. Nothing in the reactor reads it. Ze therefore does not process
  a received Software Version Capability.
- Operator decode: `decodeSoftwareVersion` and `RunCLIDecode`
  (`internal/component/bgp/plugins/softver/softver.go`) decode a hex payload
  passed to `ze bgp decode`. This path is a diagnostic tool and is not on the
  session receive path.
- Config: `session > capability > software-version > mode`
  (`internal/component/bgp/plugins/softver/yang/ze-softver.yang`).

## Compliance Checklist

- [ ] [DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-1] [MUST] "If an implementation supports the inclusion of the capability, the implementation MUST include a configuration option to enable or disable its use, and MUST default to disabled" -- the configuration option half (§3)
- [ ] [DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-2] [MUST] "If an implementation supports the inclusion of the capability, the implementation MUST include a configuration option to enable or disable its use, and MUST default to disabled" -- the default-to-disabled half (§3)
- [ ] [DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-3] [MUST] "The Capability Length for the Software Version Capability MUST be greater than zero" (§3)
- [ ] [DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-4] [SHALL] "A value of zero SHALL be treated as an encoding error and the Capability MUST be ignored" -- the encoding-error half (§3)
- [ ] [DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-5] [MUST] "A value of zero SHALL be treated as an encoding error and the Capability MUST be ignored" -- the ignore half (§3)
- [ ] [DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-6] [MUST] "The Version field MUST be encoded using UTF-8" (§3)
- [ ] [DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-7] [MUST NOT] "A receiving BGP speaker MUST NOT interpret invalid UTF-8 sequences" (§3)
- [ ] [DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-8] [MUST NOT] "A sender SHOULD limit generated product identifiers to what is necessary to identify the product; a sender MUST NOT generate advertising or other nonessential information within the product identifier" -- the MUST NOT half (§3)
- [ ] [DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-9] [SHOULD] "The Capability Length SHOULD be no greater than 64" (§3)
- [ ] [DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-10] [SHOULD] "A sender SHOULD limit generated product identifiers to what is necessary to identify the product" (§3)
- [ ] [DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-11] [SHOULD NOT] "A sender SHOULD NOT generate information in product-version that is not a version identifier" (§3)
- [ ] [DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-12] [NOT RECOMMENDED] "It is NOT RECOMMENDED for use outside a single Autonomous System, or a set of Autonomous Systems under a common administration" (§3)
- [ ] [DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3.1-1] [REQUIRED] "Implementations of this specification are REQUIRED Extended Optional Parameters Length for BGP OPEN Message support as defined in [RFC9072]" (§3.1)
- [ ] [DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-1] [MUST] "The Software Version Capability MUST only be used for displaying the version of a BGP speaker's router daemon to make troubleshooting easier" (§4)
- [ ] [DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-2] [MUST] "Enabling (i.e., turning on) this capability requires bouncing all existing BGP sessions and the feature MUST be explicitly configured before an implementation advertizes the Software Version Capability" (§4)

## Notes

The draft's OPTIONAL sentences are in the Abstract and repeated in Section 1
and Section 3: "The inclusion of the Software Version Capability is OPTIONAL"
(§3). Ze offers the send half and declines the receive half. Sections 3-4, 3-5
and 3-7 address a receiver; Ze meets each by never recognizing code 75 on the
session path, which is the RFC 5492 Section 3 unrecognized-capability outcome
the draft itself points at ("An implementation that does not recognize or
support the Software Version Capability but receives one must ignore it, as
described in Section 3 of [RFC5492]", §3).
