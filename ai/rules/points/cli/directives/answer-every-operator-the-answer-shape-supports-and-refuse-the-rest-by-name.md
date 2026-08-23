---
kind: directive
level: MUST
stage:
---
- **Every command MUST answer every operator its answer's SHAPE supports, and MUST refuse the rest by name, saying why.** An answer holding one value has no rows, so `first`, `last`, `match`, `count`, `display` and `fill` have nothing to act on there, and `| origin` over a version string is meaningless. Refusing is the requirement, not a permission: accepting an operator and answering something is worse than refusing it, because the answer looks plausible and a caller cannot tell. `show bgp | count` answered 6, the number of top-level keys, for as long as that was allowed. A command DECLARES its shape with `RegisterShape` so the refusal can happen before it runs and so the published page can state what it supports; an undeclared command is still refused, from the shape of the answer in hand. These two directives replaced one that read "every command that produces output MUST support all pipe operators", which could not be met and could not be gated: it made a claim the product did not meet and a generated wiki page that repeated the claim on 381 entries.
