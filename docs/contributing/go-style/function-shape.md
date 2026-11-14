# Ze Go Style: the shape of a function

The detail behind "The shape of a function" in
[the Go style core](../ze-go-style.md), which states each rule. Each numbered
item below continues the core's bullet in the same position.

A function that needs a scroll must be remembered, and memory is where defects
hide.

1. (Nothing more on the parameters and the return type.)
2. The parent holds the state in locals, and a helper computes the change rather
   than applying it. A leaf function is pure.
3. Split only when the separation is right: one concern scattered over three
   files is worse than one long file. Full rule: `ai/rules/go-standards.md`,
   "File Modularity".
4. The call then states its behavior, and a changed default cannot change Ze in
   silence.
5. Control stays inside Ze, the work can batch, and the work for
   each unit of time has a bound.
