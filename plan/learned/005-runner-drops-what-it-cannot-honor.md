# Learned: runner-drops-what-it-cannot-honor

Five ways a `.ci` could pass while asserting nothing were found in the `.ci`
runner in one week. They look like five defects in five parsers. They are one
defect in one habit: **the runner ANSWERED a directive it did not understand
instead of refusing it.** Nothing else about them is shared. Different files,
different authors, different years. The habit is what recurs, so the habit is
what this record is about.

## The class

A test harness reads a declaration and turns it into a stimulus. Between those
two steps sits a decision the harness makes on the author's behalf, and there
are exactly two safe answers to a declaration it cannot honor: do what it says,
or refuse the file. Every instance below took a third answer.

| The declaration | What the harness did instead | What the test then proved |
|-----------------|------------------------------|---------------------------|
| `expect=stdout:not-contains=X` | dropped the key, kept the directive | nothing; ten lines in seven files |
| `cmd=...:timeout=15s:env=K=V` | ran `env` into the value before it, then discarded `timeout` as unparseable | nothing about the variable, and a budget it never declared |
| `exec=ze bgp decode pcap -:stdin=cap` | wrote the block to a file and substituted the path | that the PATH form works |
| `exec=ze-peer ... -:stdin=peer` | appended a file the peer never read | that a peer with an empty expect set refuses to bind |
| `expect=stderr:contains=X` under one `cmd=` line | read `rec.ClientOutput`, the accumulated stdout AND stderr of every command in the file | that SOME command in the file produced the text, on SOME stream |

Read the middle column alone and each is a local mistake. Read the right column
and they are one: **a stimulus the author did not write, judged by an assertion
that is perfectly honest about it.** The assertions in this runner were never
the problem. What was dishonest is the path from the author's text to the
process, and that is where a harness's silent answers live in general.

## What the four detectors are worth, and which one generalizes

The spec proposed four and rejected one on a rule. Ranked by what they actually
catch:

**A refusal on every declared vocabulary (implemented).** Each switch in
`record_parse.go` now gates on a list in `record_parse_vocabulary.go`, and the
refusal prints the same slice the gate read. That last clause is the whole
design: the accepted set an author is shown cannot describe a vocabulary the
runner does not have, because it IS the vocabulary. The reverse direction needs
no test either, because a word with a case and no listing is refused by the gate
in every `.ci` that writes it. What DOES need a test is a word listed with no
arm behind it, which would be advertised and then refused, and
`TestDirectiveVocabularyIsLive` walks every list to prove no such word exists.

This catches a misspelling. It catches nothing else, and misspelling was one of
the five.

**A record of what RAN (implemented).** `recordExecStep`
(`internal/test/runner/runner_exec_trace.go`) writes the argv the runner built
and where each `stdin=` block went, and the report prints it under `-v` for
PASSING tests as well as failing ones. That last part is the only part that
matters. A `.ci` whose stimulus was rewritten passes exactly like one whose
stimulus was honored, so the failing-test report was the one place the
information was never needed.

It also turned out to be the measuring instrument for the whole change. Asking
"which of 269 failing tests ran an argv this change moved" is answerable in one
grep over the traces, because the routing only ever moves a block from a file to
the pipe, so a moved argv is one whose trace says `piped]`. A change with no
trace would have owed a before/after run of every suite instead.

**A stored forced red (implemented, and the one that generalizes).** No static
check finds this class. The file is well formed, the assertion is real, and only
the STIMULUS is wrong, so there is nothing for a linter to read. The only
general detector is to break the thing the green judges and watch it go red, and
the only way to KEEP that observation is to store it. That is
`internal/test/runner/testdata/mustfail/`: one `.ci` per refusal or judgement
the runner owes, and a gate that fails when one PASSES.

**A corpus lint against each command's stdin capability (rejected).** It needs a
central list of `ze` verbs, which `ai/rules/principles.md` bans, and it is the
rejected option wearing a lint's clothes.

## The trap inside the fix

A must-fail suite is a green bar whose meaning is inverted, so it inherits the
class it exists to catch, one level up. A fixture that starts failing for a NEW
reason is as broken as one that starts passing, and the verdict alone cannot
tell them apart.

So each fixture names its expected failure on a `# must-fail:` line, and
`TestCIMustFailFixturesAllFail` compares that text against `rec.Error`. A
fixture with no such line fails the gate rather than being run loosely: a
harness that cannot tell WHY a fixture failed has the same defect as the runner
that could not tell why a `.ci` passed.

## The sharpest single illustration

`test/ui/config-history-stdin-refused.ci` asserts that `ze config history -` is
refused, and it asserts two things: exit code 2, and the stderr line naming the
reason. With the `cliio.IsStdin` guard cut out of `cmdHistory`
(`internal/component/config/cli/cmd_history.go`), so the command opens the path
instead, **the exit assertion still passed.** The path does not exist either, and
`main.go` answers `exitError` for that too. Only the stderr text discriminated.

A negative test that checks THAT something failed, rather than WHICH failure it
was, will pass against a build where the refusal it exists to prove no longer
happens. The verdict is free. The reason is the evidence.
`plan/journal/negative-test-asserts-the-verdict-not-the-reason.md` collects the
class.

## What was NOT fixed, and why the number is worth keeping

The gating run measured for AC-14 carried 269 reds and none of them is this
change: `piped]` appears in zero of the 269 step traces, so no failing test ran
an argv this work moved. 224 of them share one harness defect. `generateOpen`
(`internal/test/peer/peer.go`) mirrors ze's own OPEN and then overwrites the
two-octet My AS field alone, leaving the mirrored AS4 capability holding ze's
ASN, so the peer sends an OPEN whose two AS carriers disagree. Ze reads the AS4
capability first, which RFC 6793 Section 3 requires, and answers NOTIFICATION
2/2 Bad Peer AS. **The harness is wrong and the product is right.**

It is recorded rather than fixed (`plan/journal/test-against-broken-path.md`)
because it is instrument code that blocks no goal of this spec, and its repair
changes what all 702 `ze-peer` lines send, so it owes its own gating run. The
number is the point: one harness defect accounts for 83% of a gating run's reds,
and a session that reads that log as 269 separate product findings will spend
its budget on nothing.
