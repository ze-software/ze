---
kind: directive
level: MUST
stage:
---
**When a tool's output misleads its reader, the output MUST be fixed, and a paragraph that tells the reader how to read past it MUST NOT be written in its place.** This covers every output an operator or an agent acts on: a CLI answer, an MCP tool result or error, a `./le` diagnostic, a hook refusal, a `ze doctor` report. The output is met at the moment of use, and a doc, skill or rule note is read only by whoever already knows to look for it. A page explains a result the corrected output cannot carry by itself, and a skill carries only workflow judgment the tool cannot express. Neither one is the repair for an output that is wrong, unclear or hard to discover.
