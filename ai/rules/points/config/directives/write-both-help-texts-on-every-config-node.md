---
kind: directive
level: MUST
stage:
---
**Every config node you ADD or CHANGE MUST declare BOTH texts: a `description` of one line an operator reads on the completion row, and a `ze:help` beside it carrying the paragraph the `?` box prints.** The `description` MUST fit `command.MaxSummaryChars` (96) and `ste.MaxDescriptiveWords` (25), because `overlayInnerWidth` clamps every Ze overlay to [48, 96] characters and a longer summary cannot render whole in any of them. The `ze:help` MUST differ from the `description` and MUST fit `command.MaxLongHelpBytes`. `./le docvalid help-shape` refuses each of those over the config tree, the command tree, the RPCs and the offline registry, and it judges the long text only on what the working tree added or changed against `HEAD`. A `module`, `revision`, `grouping`, `typedef` or `import` description is schema documentation no row renders, so no bound reaches it and its prose MUST NOT be moved into a `//` comment to satisfy one.
