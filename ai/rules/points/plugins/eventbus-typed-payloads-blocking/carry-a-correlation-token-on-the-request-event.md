---
kind: directive
level: SHOULD
stage:
---
- **A payload-carrying request event (`events.Register[T]`) SHOULD be preferred over a payload-less signal (`RegisterSignal`) when a returning batch has to be correlated back to a specific requestor.** The token rides the request and the producer echoes it, which keeps the returning batch peer-agnostic.
