---
kind: directive
level: MUST
stage:
---
**A function added from 2026-09-24 that implements part of an RFC MUST quote the RFC text it implements, with the RFC number and section, in the comment directly above the function or directly above the statement that implements it (`// RFC NNNN Section X.Y: "quoted text"`) (owner directive, 2026-09-24).** This applies to every implementing function: an encoder, a decoder, a state transition, a timer, a selection step, not only a check that enforces a MUST. Quote the sentence the code performs, and put the quote where a reader of the code meets it: above the statement when one statement of a longer function carries the obligation.
**A function added from 2026-09-24 that calls such a function to verify or apply part of an RFC MUST name the RFC and the section affected in a comment at the call (`// RFC NNNN Section X.Y`).** The quote stays with the implementing function, and the caller carries the reference, so a reader of the caller knows which obligation the call discharges without opening the callee.
**These two requirements apply to functions added from 2026-09-24, and they MUST NOT be read as an order to change code written before that date.** A reviewer refuses a new implementing function without its quote, or a new caller without its section reference, the same way it refuses a missing test.
