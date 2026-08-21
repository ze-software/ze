---
kind: directive
level: MUST
stage:
---
**The frame follows the DECLARATION, never the payload.** A declaring plugin answers every `execute-command` with a head, its records and a terminator, a built value included. The VALUE is unchanged, byte for byte. The frame around it is not. A test peer written by hand MUST read the frame the peer declared, or it takes a head line's tail for its result.
