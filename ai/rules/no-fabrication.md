# No Fabrication

**When:** State only what the source explicitly says or does
**Severity:** blocking

## Directives

State only what the source explicitly says or does. A factual or behavioral claim about the code, and any recommendation premised on one, must be verified against the code that produces the behavior, not inferred from the code that consumes it.

## Rule

If the source material does not contain the information needed to answer the question, say so.
Do not infer, interpret, or reconstruct an answer from structure, context, or pattern-matching.

## Behavioral claims and recommendations

A claim about what code *does* at runtime is not verified until you have read the
code that produces the behavior. Reading a value's consumer and assuming its
producer is inference, not evidence. This bites hardest when recommending work:
"there is a gap, we should fix it" is itself a claim, and its premise must be
traced to source *before* you propose the work, or you spec a non-problem.

A claim about design *intent*, or about a foreign system, has a producer too, and
it is not the nearest text: a comment states what its author believed, while the
decisions actually taken are recorded in `plan/deferrals/`, `plan/learned/`,
and specs; a generated binding stub states that a field exists, never what the
system it binds to does with it. Read the decision record before asserting
intent, especially before calling something a mistake, and read the foreign
system's own source before asserting its semantics.

A self-consistent story is a hypothesis, not a finding. Coherence is not
verification, and breadth of research (many files skimmed) does not substitute
for reading the one function the claim depends on.

Incident: session bgp-reconnect-flap (2026-06-27) claimed the peer reconnect loop
amplified session flaps and recommended a spec, after reading `run()` (the error
*consumer*) and assuming a clean session close returns `err == nil`. Reading
`Session.Run` (the *producer*) showed it never returns nil, so the `err == nil`
branch is dead and the claimed gap did not exist. Root cause: the keystone fact,
what `Run` returns on session end, was inferred from the caller, never read.

## Mechanical check

Before answering a factual question about file content:

1. Can I point to the exact line(s) that state the answer? If yes, answer.
2. If no: "The file doesn't say. [what's missing]."

Before claiming code behaves a certain way, or recommending work premised on it:

1. Name the single keystone fact the claim depends on (e.g. "a session-down yields `err == nil`").
2. Read the function that *produces* that fact (returns or sets the value), not only the one that consumes it.
3. If I have read only the consumer, label it a hypothesis, not a finding, and verify before recommending any action.

## Banned

| Pattern | Why |
|---------|-----|
| Inferring status from position in a list | The file may not encode status by position |
| Inferring done/not-done without explicit markers | Fabrication dressed as analysis |
| Presenting interpretation as fact | The user asked what the file says, not what you think |
| Guessing what the user meant and presenting the guess as a conclusion | Say you don't know, ask |
| Inferring a function's return value or behavior from its caller | Read the producer of the value, not the consumer |
| Citing a code comment as the project's design intent | A comment is its author's belief, not a decision record; read `plan/deferrals/`, `plan/learned/`, specs |
| Inferring a foreign system's semantics from a generated binding stub | The stub documents a field's existence, not what the system does with it; read that system's source (e.g. VPP's C, vendored at `third_party/vpp-linux-cp/`, not `binapi/*.ba.go`) |
| Recommending work premised on an unverified behavioral claim | The premise is itself a claim; trace it to source first |
| Treating a coherent narrative as verified | A self-consistent story is a hypothesis until the keystone fact is read |

## Mechanical backstop

The `design-without-lsp` check in `.claude/hooks/pretool-writeedit.py` blocks
writing a `plan/spec-*.md` or `plan/design-*.md` file unless this session has
investigated implementation source (read a `.go` under `internal/`, `pkg/`, or
`cmd/`, or used the LSP tool) within the last 30 minutes. It catches the case
where a spec is authored for a behavioral claim that was never traced to the
producing code. It is a backstop, not a guarantee: it cannot verify that the
code you read was the code your claim depends on. See `ai/rules/hook-mapping.md`.
