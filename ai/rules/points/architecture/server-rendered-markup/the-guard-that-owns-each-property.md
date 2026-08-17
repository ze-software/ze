---
kind: table
level:
stage:
---
| Guard | Where the rules live | What it refuses |
|---|---|---|
| `TestNoGoFileBuildsMarkup` | `internal/test/markupcheck`, `AssertNoMarkup` | a Go string literal that builds a tag. It reads the FORM of a tag, so `usage: set <leaf>` is not a finding, and it knows HTML's void elements, so a bare `<br>` is one. An exemption that explains nothing is a finding, and so is a table that changed size |
| `TestTemplatesAvoidInlineScriptAndStyle` | `internal/test/markupcheck`, `AssertNoInlineScriptOrStyle` | an inline `<script>` block, an inline `style=`, an `on*` handler, an `hx-on` attribute |
| `TestTemplAssetsResolve` | `internal/test/markupcheck`, `AssertAssetsResolve` | a `src` or `href` the served filesystem does not hold, and one naming an asset tree the package does not serve |
| `TestWebViewDataIsTyped`, `TestLGViewDataIsTyped` | `internal/test/templcheck`, `AssertTyped` | a component parameter that is a map, a named map, a bare `any`, or a struct wrapping any of them |
| `make ze-templ-output-check` | `Makefile` | a `*_templ.go` its `.templ` source no longer produces |
| `make ze-web-golden-check` | `Makefile` | a rendered byte that moved with no fixture behind it |
