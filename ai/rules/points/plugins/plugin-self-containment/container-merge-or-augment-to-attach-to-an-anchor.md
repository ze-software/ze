---
kind: directive
level: SHOULD
stage:
---
- **A new carve SHOULD attach to a verb anchor by container merge rather than by `augment`.** The YANG loader unions same-named top-level containers, so container merge creates no base-module coupling. An `augment` names its target module, so deleting the anchor breaks every augmenting owner's build.
