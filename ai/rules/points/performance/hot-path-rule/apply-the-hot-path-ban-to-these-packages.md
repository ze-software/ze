---
kind: directive
level: MUST NOT
stage:
---
**Code on these paths MUST NOT call any `fmt` function, and MUST NOT concatenate the result of `.String()`. It MUST use the `AppendTo(buf []byte) []byte` pattern from `attribute/text_append.go`.**

| Path | Examples |
|------|----------|
| Wire encoding and decoding | `message/`, `wireu/`, `attribute/` |
| Per-UPDATE processing | `reactor/`, `adj_rib_in/`, `persist/`, `rr/`, `rs/` |
| Per-route evaluation | `rib/bestpath.go`, `rib/event.go`, `rib/route.go` |
| NLRI parsing and formatting | `nlri/*/`, `rib_nlri.go` |
| Filter chain | `reactor/filter_*.go` |
