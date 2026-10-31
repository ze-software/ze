# A reply copies a field from the request that the RFC makes the sender choose

A reply builder fills a header field by copying it from the packet it answers,
because the two packets share the field and most replies (an Ack, a Nak) must
echo it. One reply type in the same family carries the opposite rule: the
sender owns the field and must change it on each transmission. Copying is then
wrong, and nothing fails until a peer that repeats its own value reads two of
Ze's replies as one.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-30 | spec-ppp-l2tp-aaa-one-off-defects | l2tp ppp Code-Reject | RFC 1661 Section 5.6: "The Identifier field MUST be changed for each Code-Reject sent." `sendCodeReject` (`ppp/session_run.go`, LCP) and `sendNCPCodeReject` (`ppp/ncp.go`, IPCP and IPv6CP) write the rejected packet's own Identifier as the Code-Reject's, so two rejected packets with one Identifier draw two Code-Rejects with one Identifier | fixed: a Code-Reject Identifier counter per protocol, and tests that two such packets draw two Identifiers; interop `ppp-code-reject-xl2tpd` proves the LCP half |
