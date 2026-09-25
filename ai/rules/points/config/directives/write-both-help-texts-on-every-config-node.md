---
kind: directive
level: MUST
stage:
---
**Every config node you ADD or CHANGE MUST declare BOTH texts: a `ze:help` of one line an operator reads on the completion row, and a `description` beside it carrying the paragraph the `?` box prints.** The `ze:help` MUST fit `command.MaxSummaryChars` (96) and `ste.MaxDescriptiveWords` (25), because `overlayInnerWidth` clamps every Ze overlay to [48, 96] characters and a longer summary cannot render whole in any of them. The `description` MUST differ from the `ze:help` and MUST fit `command.MaxDescriptionBytes`. An `enum` value carries its one-line text as `ze:help` alone. `./le doc yang-contract help-shape` refuses each of those over the config tree, the command tree, the RPCs and the offline registry, and every rule is absolute: a node with no `ze:help`, and a `ze:help` with no `description` beside it, are each refused wherever they sit. A `module`, `revision`, `grouping`, `typedef` or `import` description is schema documentation no row renders, so no bound reaches it and its prose MUST NOT be moved into a `//` comment to satisfy one.
