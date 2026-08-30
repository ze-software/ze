---
kind: directive
level: MUST
stage:
---
**These paths are the hot paths the ban above governs, and code on them MUST use the `AppendTo(buf []byte) []byte` pattern from `attribute/text_append.go`:** wire encoding and decoding (`message/`, `wireu/`, `attribute/`), per-UPDATE processing (`reactor/`, `adj_rib_in/`, `persist/`, `rr/`, `rs/`), per-route evaluation (`rib/bestpath.go`, `rib/event.go`, `rib/route.go`), NLRI parsing and formatting (`nlri/*/`, `rib_nlri.go`), and the filter chain (`reactor/filter_*.go`).
