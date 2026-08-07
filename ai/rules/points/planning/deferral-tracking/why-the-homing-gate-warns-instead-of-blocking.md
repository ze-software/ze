---
kind: note
level:
stage:
---
The commit gate that checks homing **WARNS, it does not block** (see "Status Vocabulary (the gate reads this)"
and the gate note below). An unhomed deferral row is harmless to software behaviour: the
worst case is that it is committed too early or in the wrong commit. Blocking every commit on
it, including commits that never touched deferrals, and rows another session wrote into the
shared working tree, held real work back for no software reason. So the obligation to home
a deferral is a discipline the gate reminds you of, not one it enforces: the warning keeps an
unhomed row visible so it is not lost, but you are the one who must give it a home.
