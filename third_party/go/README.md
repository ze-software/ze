# Go -- the licence of the language, runtime and standard library

**Nothing in this directory is compiled.** It holds the licence texts of the
Go release that builds Ze, because every `ze` binary links Go's runtime and
standard library, and the licence asks that a binary carry its notice.

| Field | Value |
|-------|-------|
| Go version | `go1.27.4`, the version the `go` directive of `go.mod` pins |
| Upstream | [github.com/golang/go](https://github.com/golang/go), tag `go1.27.4` |
| `LICENSE` | Byte for byte the `LICENSE` of that tag. Its first line: "Copyright 2009 The Go Authors." SHA-256 `911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad` |
| `PATENTS` | Byte for byte the `PATENTS` of that tag, which the Go distribution ships beside its licence. Its first line: "Additional IP Rights Grant (Patents)". SHA-256 `96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc` |
| Read from | The installed distribution of `go1.27.4` (`LICENSE` beside the Homebrew package, `PATENTS` in its GOROOT), on 2026-11-20 |
| Confirmed against | `https://raw.githubusercontent.com/golang/go/go1.27.4/LICENSE` and `.../PATENTS`, fetched the same day; `cmp` found both identical |

## When Go is upgraded

A change to the `go` directive of `go.mod` owes these two files from the new
tag, read the same way and compared with `cmp`, and this table updated.
