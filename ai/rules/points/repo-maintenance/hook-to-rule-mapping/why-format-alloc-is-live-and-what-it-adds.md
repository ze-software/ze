---
kind: note
level:
stage:
---
> `writeGoPatterns` is the live edit-time allocation-pattern check. Its registered
> function blocks `fmt.Sprintf`, `fmt.Fprintf`, `fmt.Printf`, and
> `strconv.FormatInt` or `strconv.FormatUint` in production Go. The broader
> allocation audit stays with its native verification action rather than an
> undocumented hook branch.
