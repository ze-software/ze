---
kind: directive
level: MUST
stage:
---
**Every wire-encodable type MUST implement `wire.BufWriter`: `WriteTo(buf []byte, off int) int`. A type that also validates capacity, or that a caller needs a length from, MUST implement `wire.CheckedBufWriter`, which adds `CheckedWriteTo(buf, off) (int, error)` and `Len() int`.**
**A context-dependent attribute MUST take the source and destination `*bgpctx.EncodingContext`, through `WriteToWithContext(buf, off, srcCtx, dstCtx) int`, so ADD-PATH and ASN4 encode per peer.**
