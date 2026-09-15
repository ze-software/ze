# A guard keys on a field the thing it protects never reads

A filter stands in front of a consumer and decides which messages reach it. The
filter identifies the flow by one field, and the consumer identifies it by
another. Every message the consumer accepts under its own key, and the filter
never claims under its key, walks past: the guard protects against the one
attacker who spoofs the field it reads, and against nobody else. The tell is a
match on the OUTER header of a message whose consumer looks up state by an
INNER one, or on a sender field for a message any third party can send.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-15 | gtsm-related-icmp-quoted-destination | `peerTerms` (`internal/component/gtsm/gtsm.go`) in front of `tcp_v4_err` | The RFC 5082 related-message drop claimed an ICMPv4 error by its OUTER source address being the peer. `tcp_v4_err` (`net/ipv4/tcp_ipv4.c`) finds the socket by the QUOTED 4-tuple and never reads the outer source, so a forged error from any other address, quoting the session and carrying a low outer TTL, reached the TCP stack and was judged on the TTL the forger wrote inside the quote. The spec that introduced the filter modelled the Section 5.3 attacker (spoofs the peer's address) and not the Section 6.1 one (forges an error from anywhere) | The term reads the destination QUOTED inside the error (`MatchICMPErrorQuotedDestination`, lowered as a 4-octet compare at transport offset 24 behind the 0x45 guard) and no outer field but the TTL. Proven from a third source in `TestGTSMDropsADangerousQuotedICMPError/from_another_host` |
