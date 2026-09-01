# draft-walton-bgp-hostname-capability - FQDN Capability for BGP

## Meta

| Field | Value |
|-------|-------|
| Draft | draft-walton-bgp-hostname-capability-02 |
| Title | Hostname Capability for BGP |
| Status | Internet Draft (expired 2016-07-09) |
| Date | 2016-01-06 |
| Depends | RFC 5492 (BGP capability advertisement) |
| Enrolment | enrolled |
| Enrolment reason | FQDN capability for BGP (code 73). The draft states NO MUST-level obligation: its only RFC 2119 keyword outside the Section 2 key-words paragraph is one SHOULD in Section 4, so the summary declares zero gated rows and rfc/extraction/draft-walton-bgp-hostname-capability.json signs off under 'manual-walk' with the register-reason that says why zero is a property of the document. Ze sends the capability from per-peer config (encodeValue, internal/component/bgp/plugins/hostname/hostname.go), encodes it with (*FQDN).WriteTo and parses a received one with parseFQDN (internal/core/bgp/capability/capability.go). |
| Support | drafts 50 |
| Support area | BGP FQDN capability code 73 |
| Support status | Supported |
| Support coverage | The draft states no MUST-level obligation: its only RFC 2119 keyword outside the Section 2 key-words paragraph is one SHOULD in Section 4, which is why `rfc/extraction/draft-walton-bgp-hostname-capability.json` signs off under `manual-walk`. Capability code 73, decode support, and per-peer hostname and domain advertisement (`FQDN`, `internal/core/bgp/capability/capability.go`). Carried as `RFC 8516` in the table above until 2026-08-30; RFC 8516 is the CoAP "Too Many Requests" response code and has nothing to do with BGP. IANA names this draft as the reference for code 73, and the scoped config keys the capability emits had always spelled it. |
| Support remaining | - |

**Purpose:** Defines BGP capability code 73, which carries a speaker's hostname
and domain name so an operator sees a name beside a peer address when
troubleshooting.

**Scope:** Capability (OPEN message) only. The draft defines no UPDATE behavior,
no negotiation rule of its own, and no error handling; RFC 5492 Section 3
governs an unrecognized code 73.

## Wire Format

Capability Code: 73 (0x49). Capability Length: variable.

```
+--------------------------------+
|  Hostname Length (1 octet)     |
+--------------------------------+
|  Hostname (variable)           |
+--------------------------------+
|  Domain Name Length (1 octet)  |
+--------------------------------+
|  Domain Name (variable)        |
+--------------------------------+
```

| Field | Size | Semantics |
|-------|------|-----------|
| Hostname Length | 1 byte | "The number of characters in the Hostname" (§3) |
| Hostname | variable | "The hostname encoded via UTF-8" (§3) |
| Domain Name Length | 1 byte | "The number of characters in the Domain Name" (§3) |
| Domain Name | variable | "The domain name encoded via UTF-8" (§3) |

## Ze Implementation

- Capability code: `capability.CodeFQDN` (73), `internal/core/bgp/capability/capability.go`.
- Receive: `parseFQDN` (same file) decodes both length-prefixed strings and
  refuses a truncated value with `ErrShortRead`.
- Send: `(*FQDN).WriteTo` (same file) writes the four fields, clamping each
  string at 255 bytes.
- Advertisement: the `bgp-hostname` plugin builds the payload from per-peer
  config in `encodeValue` and `extractHostnameCapabilities`
  (`internal/component/bgp/plugins/hostname/hostname.go`), and the reactor
  injects it into the OPEN through `Peer.getPluginCapabilities`
  (`internal/component/bgp/reactor/peer.go`), wired at
  `internal/component/bgp/reactor/peer_run.go` `session.SetPluginCapabilityGetter`.
- Config: `session > capability > hostname > host` and `> domain`.

## Compliance Checklist

- [ ] [DRAFT-WALTON-BGP-HOSTNAME-CAPABILITY-4-1] [SHOULD] "The FQDN Capability SHOULD only be used for displaying the hostname and/or domain name of a speaker in order to make troubleshooting easier" (§4)

## Notes

The draft states no MUST, MUST NOT, SHALL, SHALL NOT or REQUIRED obligation.
Its only RFC 2119 keyword outside the Section 2 key-words paragraph is the
Section 4 SHOULD above. Section 3 describes the wire format in indicative
prose, Section 5 records the IANA assignment of code 73, and Section 6 states
that the document "introduces no new security concerns". The zero MUST-level
count is therefore a property of the document, which is what
`rfc/extraction/draft-walton-bgp-hostname-capability.json` records as its
`register-reason`.
