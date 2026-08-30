---
kind: directive
level: MUST NOT
stage:
---
**Every file that emits OPEN, NOTIFICATION, ROUTE-REFRESH or NEGOTIATED text or JSON is encoding code, and MUST NOT call `fmt.Sprintf`, `fmt.Fprintf`, `strings.Builder`, `strings.Join`, `strings.NewReplacer`, `strings.ReplaceAll`, `strconv.FormatUint` or `strconv.FormatInt`.** `strconv.AppendUint`, `netip.Addr.AppendTo`, `hex.AppendEncode`, and a local `[N]byte` scratch with `append` MAY be used instead. `json.go` is excluded while its `map[string]any` plus `json.Marshal` idiom remains.
