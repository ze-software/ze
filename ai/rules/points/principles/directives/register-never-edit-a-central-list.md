---
kind: directive
level: MUST NOT
stage:
---
**A new feature MUST register itself and be discovered; it MUST NOT require an edit to a switch, a case, a factory, a field list, or any other central enumeration.** A central list is a second declaration of what already exists, so adding a feature means editing code that has nothing to do with it, and removing one means finding every place that named it. Registration makes the feature's own package the only thing that has to change.

**A central enumeration MUST be recognized by what it answers for a feature it does not name, and MUST NOT be recognized by whether it reads as a list.** It is almost always spelled as something else: a parser, a validator, a seed map, a runner, a help string, a completion table. One question finds every shape, and it is asked of the code before it is written: when a feature arrives and nobody edits this file, does it refuse, or does it answer? A default arm that returns a plausible value, an `ok == false` that no caller reads, and a name map whose literal entries decide what the protocol treats as recognized are each this defect under another name, so the enumeration MUST be derived from the registry that holds the fact.
