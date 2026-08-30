---
kind: directive
level: MUST NOT
stage:
---
**A new feature MUST register itself and be discovered; it MUST NOT require an edit to a switch, a case, a factory, a field list, or any other central enumeration.** A central list is a second declaration of what already exists, so adding a feature means editing code that has nothing to do with it, and removing one means finding every place that named it. Registration makes the feature's own package the only thing that has to change.
