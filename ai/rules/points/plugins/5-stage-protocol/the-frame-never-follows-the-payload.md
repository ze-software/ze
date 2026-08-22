---
kind: directive
level: MUST
stage:
---
**The frame never follows the payload.** A plugin answers every `execute-command` with a head, its records and a terminator, a built value included. The VALUE is unchanged, byte for byte. The frame around it is not. A test peer written by hand MUST write and read that frame, or the engine takes a head line's tail for its result.
