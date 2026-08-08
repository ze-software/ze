# Why a found problem gets a spec before it gets a fix

Owner decision, 2026-08-08.

`completion.md` makes you the owner of a defect you walk into, and it used to make
you its owner *this minute*: find it, finish the primary task, fix it in the same
session. That produced sessions that never landed. The work in hand stayed open
while an unrelated fix grew, the closing commit lost its single focus, and the
gates that were already green restarted.

The order is now fixed, and the fix itself is the owner's call:

| Step | What it produces |
|------|------------------|
| Write the spec at the moment of discovery | The defect survives the session in a form somebody can pick up |
| Close the work in hand | The thing you were asked to do lands, on its own gates, in its own commit |
| Ask Thomas whether to implement the new spec | The decision about what the next session costs belongs to him |

Two properties keep this from becoming a parking lot.

**A blocker is exempt.** When the current goal does not hold without the fix, the
defect is not separable and you cannot close around it. That case still routes to
"Fix a defect that blocks your goal", unchanged.

**A spec is not a record.** `plan/known-failures/`, a `tmp/` note, and a sentence
in a report remain banned, because none of them is work anybody can start. The
spec is the first step of the fix. The ask is what schedules the rest.
