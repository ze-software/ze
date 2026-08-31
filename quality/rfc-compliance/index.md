# RFC Compliance Gate Report

Source: `internal/le/rfc`, `rfc/short/*.md`, `rfc/enrolled.txt`, `docs/features/rfc-status.md`, and `rfc/audit/*.json`.

## Current gate output

```
rfc-requirements: 2 violation(s)
```

| Metric | Value |
|---|---:|
| Gate issues | 2 |
| Gated MUST-level requirements | 3,037 |
| Enrolled RFCs | 172 |
| Resolved test tags | 3,937 |
| Declared gaps | 502 |
| RFCs with declared gaps | 79 |
| Fresh semantic audit verdicts | 50 |
| Shifted semantic audit verdicts | 0 |
| Stale semantic audit verdicts | 2 |

## Requirement buckets

| Bucket | Count | Share | Source condition |
|---|---:|---:|---|
| Positive and negative tests | 1,333 | 43.9% | `positive tag + negative tag` |
| One polarity plus reason | 366 | 12.1% | `{single-polarity} annotation + required tag` |
| Not applicable | 836 | 27.5% | `{not-applicable} annotation` |
| Declared gap | 502 | 16.5% | `{gap} annotation + public ledger disclosure` |

## Gap disclosure

| Public status for RFCs with gaps | RFCs |
|---|---:|
| Partial | 59 |
| Experimental | 14 |
| Supported | 4 |
| Not supported | 1 |
| Unsupported | 1 |

### Supported rows that still disclose a gap

- **RFC 1350:** RFC1350-2-3 unmet (Sorcerer's Apprentice fix): sendAndWaitACK retransmits DATA on any non-matching ACK (handler.go) instead of silently ignoring a duplicate or stale ACK.
- **RFC 4301:** One MUST gap (RFC4301-4.4.1.1-1): SPD selectors are limited to IP-prefix + next-layer-protocol; TCP/UDP/SCTP port and ICMP type/code selectors are not modeled or projected (internal/component/ike/dataplane/dataplane.go; engine/initiator.go tsToIPNet drops ports).
- **RFC 6396:** One MUST gap gated in rfc/short/rfc6396.md [RFC6396-4.4.3-1]: the live BGP4MP writer always emits the BGP4MP_MESSAGE_AS4 subtype and records the on-wire message verbatim without checking the session's negotiated 4-byte-AS capability, so a message from an OLD (2-byte) peer carries a 2-byte AS_PATH mislabeled as AS4. RIB-path AS_PATH is unaffected (canonicalized to 4-byte).
- **RFC 7313:** Four MUST-level receive-side gaps annotated in `rfc/short/rfc7313.md`: RFC7313-4-4/4-5 -- a received BoRR/EoRR is log-only (internal/component/bgp/plugins/rib/rib.go), so ze marks no Adj-RIB-In routes stale and purges none; and RFC7313-4-6/4-7 -- neither the send nor receive path applies a Graceful-Restart End-of-RIB gate to BoRR emission or acceptance.

## Top gap clusters

| RFC | Declared gaps | Public status |
|---|---:|---|
| `RFC 9012` | 51 | Partial |
| `DRAFT-IETF-BESS-MUP-SAFI` | 37 | Partial |
| `RFC 1661` | 24 | Partial |
| `RFC 9830` | 20 | Partial |
| `RFC 2131` | 18 | Partial |
| `RFC 7166` | 17 | Unsupported |
| `RFC 4577` | 16 | Not supported |
| `RFC 4271` | 15 | Partial |
| `RFC 7432` | 15 | Partial |
| `RFC 5880` | 14 | Partial |
| `RFC 8665` | 14 | Partial |
| `RFC 9514` | 13 | Partial |

## Gate inputs

| Input | Producer | Observed value |
|---|---|---|
| Requirement source | `rfc/short/*.md` | 3,037 gated MUST-level requirements |
| Enrollment | `rfc/enrolled.txt` | 172 enrolled RFCs |
| Test tags | `internal/, pkg/, test/` | 3,937 resolved tags |
| Public ledger | `docs/features/rfc-status.md` | 79 RFCs with gaps |
| Semantic audits | `rfc/audit/*.json` | 50 fresh, 0 shifted, 2 stale, 2,985 missing |
| Pre-commit verification | `internal/le/verify/engine/stages.go` | `./le rfc check`, 1 of 43 full-mode stages |

## Check results

- rfc/short/rfc7606.md:331: RFC7606-5.1-3 has a STALE audit verdict -- what it judged changed: internal/le/interoplab/bgp/check_rfc.go::checkRFC7606MixedUpdate (func-scoped). This is NOT a line shift and ./le rfc reseal will refuse it. Re-read rfc7606 with the ze-rfc-audit skill (ai/skills/ze-rfc-audit.md)
- rfc/short/rfc7606.md:333: RFC7606-5.4-1 has a STALE audit verdict -- what it judged changed: internal/le/interoplab/bgp/check_rfc.go::checkRFC7606TypedNLRIDiscard (func-scoped). This is NOT a line shift and ./le rfc reseal will refuse it. Re-read rfc7606 with the ze-rfc-audit skill (ai/skills/ze-rfc-audit.md)
