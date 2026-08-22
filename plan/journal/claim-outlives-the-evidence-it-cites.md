# Claim outlives the evidence it cites

The mirror of `ephemeral-record-stored-permanently`. A claim is written where it
survives, and the evidence it rests on is written where it does not. Nothing
breaks at the moment the evidence goes: the claim still reads as authority, still
names a path, and the reader who follows that path finds nothing. The claim is
then worse than no claim, because it costs a reader the time to discover it is
empty and it discourages the work of re-establishing what it asserted.

The general practice is that a claim and its evidence want the same lifetime. Where
that is impossible, the claim carries enough to be re-checked without the artifact,
or it is not recorded until the moment it is preserved.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-22 | - | spec Review Gate section against `tmp/review/` | The Review Gate table records a verdict, the round count, the lenses used and the path of the artifact that pins the reviewed files by hash. The table lives in the spec, which is tracked. The artifact lives under `tmp/`, which is cleaned on a cadence. A spec that records a clean verdict and does NOT close in the same session therefore keeps a citation that expires, and the hashes expire with it, so nothing about the recorded verdict can be re-checked. Two specs met this on one day: each read `clean` or `findings`, each named an artifact that no longer existed, and each cost a full independent re-review to close. The gate itself is sound: `commit_helper.py` requires the artifact at closure-commit time, so a spec that closes in the session that reviewed it is never exposed | not fixed. The narrow repair is to treat a recorded verdict as valid only for the session that recorded it, which is what the two-commit closure rule already assumes: a verdict is written when the closure commit is about to carry it, never in advance of one. The wider repair would give the artifact the lifetime of the claim, and that is a decision about where review evidence belongs rather than a defect in the gate |
