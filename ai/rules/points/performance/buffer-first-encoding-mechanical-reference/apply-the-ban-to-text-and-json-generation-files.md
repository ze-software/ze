---
kind: note
level:
stage:
---
The same rule covers the BGP text/JSON format-generation files migrated by
fmt-0 and fmt-2-json-append (the `format-alloc` hook check is currently a
no-op, see `ai/rules/repo-maintenance.md`; the rule still applies): every file
that emits OPEN / NOTIFICATION / ROUTE-REFRESH / NEGOTIATED text or JSON is
covered, and `fmt.Sprintf`, `fmt.Fprintf`, `strings.Builder`,
`strings.Join`, `strings.NewReplacer`, `strings.ReplaceAll`,
`strconv.FormatUint`, `strconv.FormatInt` are rejected at Write/Edit time.
Allowed helpers: `strconv.AppendUint`, `netip.Addr.AppendTo`,
`hex.AppendEncode`, or a local `[N]byte` scratch plus `append`. `json.go`
is intentionally excluded while its `map[string]any` + `json.Marshal`
idiom remains.
