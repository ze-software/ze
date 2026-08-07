---
kind: note
level:
stage:
---
A payload-carrying request event (`events.Register[T]`) is preferred over a
payload-less signal (`RegisterSignal`) when a returning batch must be correlated
back to a specific requestor: the token rides the request and the producer echoes
it, keeping the returning batch peer-agnostic.
