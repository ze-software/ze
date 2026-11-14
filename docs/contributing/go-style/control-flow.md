# Ze Go Style: control flow a reader can simulate

The detail behind "Control flow a reader can simulate" in
[the Go style core](../ze-go-style.md), which states the rule. The paragraphs
below follow the order of the core's paragraphs.

Guard clauses followed by the main logic read from top to bottom, and a happy
path inside an `else` does not.

A reader who meets `if a && b && !c` must hold three facts at once to decide
whether every case is covered.

The split changes no meaning, because `||` stops at the first true operand and
the body leaves either way. `./le arch compound-guard check` refuses a top-level
`||` condition with no `else` and a body that ends in `return`, `continue`,
`break` or `goto`, on the lines your unpushed commits and working tree changed.
It does not judge `&&`, an `else`, or a body that falls through, because none of
those splits without a design decision. How it chooses the lines:
[`ze-go-style-background.md`](../ze-go-style-background.md).
<!-- source: internal/le/arch/compoundguard/compoundguard.go -- Check -->

`if index >= length` states the failure of the invariant, and the reader must
invert it.
