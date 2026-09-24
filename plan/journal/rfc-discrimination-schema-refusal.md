| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-22 | - | Native `rfc discriminate-record` for RFC2205-3-37 | The command rejects `rfc/discrimination/rfc2205.json`: fingerprint key `internal/plugins/rsvpte/transport_linux.go::rawTransport.SendPath` does not match its accepted key format. | Instrument remains red. The second record command was not run against the same rejected prerequisite. Native transport RED/GREEN evidence remains separate from this missing metadata. |
