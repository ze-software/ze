# RFC Compliance Gate Report

Source: `scripts/dev/rfc_requirements.py`, `rfc/short/*.md`, `docs/features/rfc-status.md`, `rfc/audit/*.json`, and `.claude/hooks/pretool-writeedit.py`.

## Current gate output

```
rfc-requirements OK: 2963 gated MUST-level requirement(s) across 170 enrolled RFC(s); 3339 test tag(s) resolved.
```

| Metric | Value |
|---|---:|
| Gate issues | 0 |
| Gated MUST-level requirements | 2,963 |
| Enrolled RFCs | 170 |
| Resolved test tags | 3,339 |
| Declared gaps | 528 |
| RFCs with declared gaps | 82 |
| Fresh semantic audit verdicts | 52 |
| Shifted semantic audit verdicts | 0 |
| Stale semantic audit verdicts | 0 |

## Requirement buckets

| Bucket | Count | Share | Source condition |
|---|---:|---:|---|
| Positive and negative tests | 1,223 | 41.3% | `positive tag + negative tag` |
| One polarity plus reason | 371 | 12.5% | `{single-polarity} annotation + required tag` |
| Not applicable | 841 | 28.4% | `{not-applicable} annotation` |
| Declared gap | 528 | 17.8% | `{gap} annotation + public ledger disclosure` |

## Gap disclosure

| Public status for RFCs with gaps | RFCs |
|---|---:|
| Partial | 60 |
| Experimental | 15 |
| Supported | 5 |
| Not supported | 1 |
| Unsupported | 1 |

### Supported rows that still disclose a gap

- **RFC 1350:** RFC1350-2-3 unmet (Sorcerer's Apprentice fix): sendAndWaitACK retransmits DATA on any non-matching ACK (handler.go) instead of silently ignoring a duplicate or stale ACK.
- **RFC 4301:** One MUST gap (RFC4301-4.4.1.1-1): SPD selectors are limited to IP-prefix + next-layer-protocol; TCP/UDP/SCTP port and ICMP type/code selectors are not modeled or projected (internal/component/ike/dataplane/dataplane.go; engine/initiator.go tsToIPNet drops ports).
- **RFC 6396:** One MUST gap gated in rfc/short/rfc6396.md [RFC6396-4.4.3-1]: the live BGP4MP writer always emits the BGP4MP_MESSAGE_AS4 subtype and records the on-wire message verbatim without checking the session's negotiated 4-byte-AS capability, so a message from an OLD (2-byte) peer carries a 2-byte AS_PATH mislabeled as AS4. RIB-path AS_PATH is unaffected (canonicalized to 4-byte).
- **RFC 7313:** Four MUST-level receive-side gaps annotated in `rfc/short/rfc7313.md`: RFC7313-4-4/4-5 -- a received BoRR/EoRR is log-only (internal/component/bgp/plugins/rib/rib.go), so ze marks no Adj-RIB-In routes stale and purges none; and RFC7313-4-6/4-7 -- neither the send nor receive path applies a Graceful-Restart End-of-RIB gate to BoRR emission or acceptance.
- **RFC 7911:** One MUST gap, gated in `rfc/short/rfc7911.md`: on re-advertisement ze preserves the ingress Path Identifier (internal/component/bgp/reactor/forward_body.go copies the received path-id and the egress RIB key rib_structured.go carries it) rather than minting its own per RFC7911-2-2, so a re-advertised path is not assigned a fresh Path Identifier.

## Top gap clusters

| RFC | Declared gaps | Public status |
|---|---:|---|
| `RFC 9012` | 51 | Partial |
| `DRAFT-IETF-BESS-MUP-SAFI` | 37 | Partial |
| `RFC 1661` | 25 | Partial |
| `RFC 4271` | 20 | Partial |
| `RFC 9830` | 20 | Partial |
| `RFC 2131` | 18 | Partial |
| `RFC 7166` | 17 | Unsupported |
| `RFC 4577` | 16 | Not supported |
| `RFC 7432` | 15 | Partial |
| `RFC 5880` | 14 | Partial |
| `RFC 8665` | 14 | Partial |
| `RFC 9514` | 13 | Partial |

## AI guard and gate inputs

| Input | Producer | Observed value |
|---|---|---|
| Requirement source | `rfc/short/*.md` | 2,963 gated MUST-level requirements |
| Enrollment | `rfc/enrolled.txt` | 170 enrolled RFCs |
| Test tags | `internal/, pkg/, test/` | 3,339 resolved tags |
| Public ledger | `docs/features/rfc-status.md` | 82 RFCs with gaps |
| Semantic audits | `rfc/audit/*.json` | 52 fresh, 0 shifted, 0 stale, 2,911 missing |
| AI write/edit guard | `.claude/hooks/pretool-writeedit.py` | ON |
| Verify integration | `Makefile` and `scripts/status/verify_run.go` | 2 verify stages |

## Check results

| Check | Open issues |
|---|---:|
| Enrolment ratchet | 0 |
| New summary ratchet | 0 |
| Retired requirement ratchet | 0 |
| Coverage polarity ratchet | 0 |
| Summary parse | 0 |
| Requirement ID allocation | 0 |
| Requirement coverage | 0 |
| Public claim agreement | 0 |
| Semantic audit freshness | 0 |
| Generated ledger freshness | 0 |
