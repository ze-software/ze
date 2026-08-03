# Evidence and Guards

**When:** stating what code does, writing or reviewing a guard, or writing any string that enumerates data a registry already holds
**Severity:** blocking
**Related:** writing, planning, protocol

## Directives

**State only what the source explicitly says or does. A factual or behavioral claim about the code, and any recommendation premised on one, must be verified against the code that produces the behavior, not inferred from the code that consumes it.**

**A guard must fail closed or say something. Silent degradation into a permissive no-op is the bug, and a zero value that downstream reads as a legitimate answer is how it hides.**

**If enumerated data has a canonical source (registry, map, typed enum, list function), DERIVE every display/help/error/usage/doc string from it. No second hardcoded copy.**

Detail: `ai/rationale/derive-not-hardcode.md`.

## No Fabrication

### Rule

If the source material does not contain the information needed to answer the question, say so.
Do not infer, interpret, or reconstruct an answer from structure, context, or pattern-matching.

### Behavioral claims and recommendations

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

### Citation

Verification and citation are two decisions, and this rule owns the first.

**Verification is an action: read the producing function. The citation is written for the reader, and it names the file and the symbol.**

**A line number is correct only when the line IS the fact, and then it belongs in a fenced block as quoted output, never in prose.**

A pasted line number proves nothing about what you read, and it goes stale at the
next edit (`ai/rules/writing.md`). Line-number citations were stripped from
the whole corpus on 2026-08-03, and `c_line_number_ref` in
`.claude/hooks/pretool-writeedit.py` blocks new ones in `ai/`, `docs/`, `plan/`,
and `.claude/` markdown.

### Mechanical check

Before answering a factual question about file content:

1. Can I point to the text that states the answer? If yes, answer.
2. If no: "The file doesn't say. [what's missing]."

Before claiming code behaves a certain way, or recommending work premised on it:

1. Name the single keystone fact the claim depends on (e.g. "a session-down yields `err == nil`").
2. Read the function that *produces* that fact (returns or sets the value), not only the one that consumes it.
3. If I have read only the consumer, label it a hypothesis, not a finding, and verify before recommending any action.

### Banned

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

### Mechanical backstop

The `design-without-lsp` check in `.claude/hooks/pretool-writeedit.py` blocks
writing a `plan/spec-*.md` or `plan/design-*.md` file unless this session has
investigated implementation source (read a `.go` under `internal/`, `pkg/`, or
`cmd/`, or used the LSP tool) within the last 30 minutes. It catches the case
where a spec is authored for a behavioral claim that was never traced to the
producing code. It is a backstop, not a guarantee: it cannot verify that the
code you read was the code your claim depends on. See `ai/rules/repo-maintenance.md`.

## Fail-Closed Guards

### Rule

A guard is any code whose purpose is to reject: an authorization check, a
cardinality constraint, a capability lookup, a ratchet, a validator.

| Requirement | Meaning |
|-------------|---------|
| Fail closed | On a miss, an unmapped input, an empty set, or an error, deny. Never fall through to the permissive branch. |
| Or say something | A guard that genuinely cannot deny MUST log, error, or fail its gate. A guard that neither denies nor speaks does not exist. |
| Make the miss explicit at the producer | Do not rely on a downstream layer to notice. The layer that knows it missed is the only one that can say so. |

### The zero-value trap

A zero value must never be a valid-looking answer. Where a lookup's miss returns
a zero that downstream reads as a legitimate outcome (allow, match-nothing,
success, count-of-1), the miss is invisible at every later layer.

Go's bare map read is the archetype: `m[k]` on an absent key yields the zero
value with no signal. Prefer `v, ok := m[k]` and handle `!ok` explicitly.

**A present-but-empty value passes `ok`.** `ok` proves the key exists, not that
the value is usable. When empty is also wrong, check `!ok || len(v) == 0`.

### Test corollary

**Drive the guard from the entry point that triggers it.** A unit test on the
guard helper proves the helper is correct. It proves nothing about whether the
caller ever reaches it with the input that matters.

A green unit test on an uncalled guard is worse than no test: it stops the
question being asked. `TestCheckCardinality`
(`internal/component/config/yang/validator_test.go`) passes, including its
count-0 row, while `walkTree` (`internal/component/config/yang/validator.go`)
iterated only present keys and its leaf-value branch skipped non-strings, so
leaf-list `min-elements` was only ever handed exactly 1 and could never reject.

**Test the shape that should be rejected, not only the shapes that work.**

### Evidence corollary

**A doc or comment asserting a safety property is not evidence the property holds. Read the producing function.**

This is the No Fabrication section above, applied to safety claims, where the
cost of a reassuring wrong answer is highest: the reader asks the right question,
gets the doc's answer, and stops.

Beware the guard that works where you spot-check it and not where it matters.
Cardinality visibly rejects on `list` nodes, which is exactly why "YANG checks
cardinality" survives a probe and is false for leaf-lists.

### Worked example

`authz.Store.Authorize` (`internal/component/authz/authz.go`): with no
assignment and no config users (`hasUsers == false`) it returns
`BuiltinAdminProfile()`. An empty profile set is indistinguishable from "never
seen", because `aaa.RecordLoginProfiles`
(`internal/component/aaa/login_profiles.go`) early-returns on
`len(profiles) == 0` and records nothing. The zero value meant ADMIN: two live
privilege escalations, via TACACS+ and RADIUS. `docs/guide/radius.md` asserted
the opposite as fact. Fixed in `ff87bf61a`.

Four more instances of the same shape, found in one day:
`plan/learned/1157-fail-open-auth-empty-profiles.md`.

## Derive, Never Hardcode

### Rule

| Surface | Pull from |
|---------|----------|
| Error messages ("valid: a,b,c") | registry `List()` / `Keys()` |
| `registry.Meta.Subs` / `Description` | derived at `init()` |
| CLI `flag.NewFlagSet.Usage` | derived at call time |
| Help / `--help` output | derived |
| `.ci` test expectations listing names | test pulls the list |
| Generated docs | `make ze-inventory` |

**If the lookup is awkward, add a `List()` accessor. Do not paste the list twice.**

### Structured Data, Not Pre-Formatted Strings

Handlers return typed values; the display layer owns rendering.

| Do | Don't |
|----|-------|
| Return typed struct (`*CPUInfo`, `[]NICInfo`) | Return `"CPU: Intel N100, 4 cores"` |
| Numeric fields (`*-bytes`, `*-mhz`) | Human string (`"8.0 GiB"`) |
| Kebab-case JSON with typed fields | YAML-ish text blocks |
| Let `\| table` / web UI render | Render text in handler |

Principle: **registry/struct is truth; every surface is a view of it**.

### Mechanical Check

- Grep for duplicates before committing (`grep -rn "FOO\|BAR\|BAZ" .claude docs cmd internal test plan`).
- Output shape: "could pipe framework and web UI both render without re-parsing?" No -> emit typed field instead.

### When a Hardcoded List Is OK

- Canonical registry doesn't exist yet; you are creating it (same commit as consumer).
- Test fixture deliberately asserts against drift.
- YANG / JSON Schema where the enum IS the contract.

Otherwise: derive.
