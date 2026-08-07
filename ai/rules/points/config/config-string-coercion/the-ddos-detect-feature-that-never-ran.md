---
kind: note
level:
stage:
---
Confirmed real instance: `ddos-detect` never ran in any daemon. `enabled` parsed `false` from the string `"true"`, so the detector never subscribed to the rate feed and never fired (session 6503). The BPS/persistence/confidence code was correct; it was never reached.
