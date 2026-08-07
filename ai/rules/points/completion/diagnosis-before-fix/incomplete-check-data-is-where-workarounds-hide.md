---
kind: note
level:
stage:
---
The third option is where most "I worked around it" bugs hide. Example: `update bgp irr` rejected because `update` was missing from the registry's allowed-verb set. The verb gate was correct; renaming the command was a workaround. The fix was adding `update` to the registry.
