---
kind: directive
level: MUST NOT
stage:
---
**Operator-facing text MUST NOT name the library, driver or vendor package Ze
implements a feature with.** It names what the OPERATOR configured. This covers
every surface a person reads: an error string, a log line, CLI output, the TUI,
and a `ze explain` description.

The operator wrote `traffic { control { backend vpp } }`, so `vpp` is theirs and
it belongs in the message. `govpp` is the Go package Ze happens to speak VPP
with. It appears in no configuration, no documentation an operator is given, and
no search they would think to run. A message naming it asks them to debug a
dependency they did not choose.

**A message MUST NOT name a Go symbol either.** `govpp WaitConnected: not
connected after 5s` names a library AND a method. `vpp not connected after 5s`
says the same thing to the person who has to start VPP.

The test is one question: could the operator have typed this word into their
configuration, or read it in the guide? A backend name, an interface name, a
peer address and a protocol all pass. A package, a type, a function and a
vendored module do not.

The rule stops at the boundary. A wrapped error travelling between packages MAY
name whatever helps a developer, and an identifier in the code is not
operator-facing text. What crosses to a person is what this governs, and the
last wrap before that crossing is where the internal name gets removed.
