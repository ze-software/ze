# Source identities to record in MAIN

The author ran no hasher or formatter. Record SHA-256 after MAIN formats only
`after/catalog_cancel_test.go.txt`, and retain the resulting exact delta.

| Input | Current source status | SHA-256 |
|-------|-----------------------|---------|
| `before/catalog_cancel_test.go.txt` | Exact canonical comparison succeeded | MAIN to calculate |
| `after/catalog_cancel_test.go.txt` | Additive, unformatted, not compiled | MAIN to calculate after formatting |
| `overlay.json` | One existing test-file mapping | MAIN to calculate |
| Selected Go compiler and Go executable | Actual runtime paths/digests are emitted by the helper | Not yet observed |
| Native named binary | Actual `/proc/<entry>/exe` digest is emitted at readiness | Not yet observed |

No placeholder above is a claimed digest or execution result. Canonical source
must still match `before/` before MAIN starts either native root.
