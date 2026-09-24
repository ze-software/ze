# Spec: RSVP-TE Hello extension (RFC 3209 Section 5)

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-23 |

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later Opus 5 session reviews that commit and closes. `-` closes in the same session. -->

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     The file's DIRECTORY carries the release bucket: plan/immediate/ for a defect
     an operator meets, plan/pre-release/ for work the release cannot go out
     without, plan/ for everything else (plan/README.md). -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Backlog boundary:** This skeleton records an unstarted capability or conditional RFC obligation. The owner's authorization to finish already-started work does not select this feature. Its requirements are not completed or waived.

The RSVP Hello extension (RFC 3209 Section 5: Hello message type 20, HELLO REQUEST and HELLO ACK objects class 22, per-neighbor Src_Instance and Dst_Instance tracking, the hello_interval timers and the loss-of-communication verdict) is a feature Ze does not offer today: `internal/plugins/rsvpte` holds no message type 20 and no object class 22. RFC 3209 Section 5 says "The Hello extension is specifically designed so that one side can use the mechanism while the other side does not." Link failure detection on every link comes from the interface component (`internal/plugins/rsvpte/engine.go::handleLinkDown`), and Ze configures no unnumbered link (Section 5.4). The owner can decline the feature in one word; until then every MUST below is owed.

| Requirement | RFC text | Producer or absence |
|---|---|---|
| RFC3209-5.2.2-1 | "This value MUST change when the sender is reset, when the node reboots, or when communication is lost to the neighboring node and otherwise remains the same." (S5.2.2) | absent: no Hello extension in `internal/plugins/rsvpte` |
| RFC3209-5.2.2-2 | "This field MUST NOT be set to zero (0)." (S5.2.2) | absent: no Hello extension |
| RFC3209-5.2.2-3 | "This field MUST be set to zero (0) when no value has ever been seen from the neighbor." (S5.2.2) | absent: no Hello extension |
| RFC3209-5.3-1 | "This value MUST NOT change while the agent is exchanging Hellos with the corresponding neighbor." (S5.3) | absent: no Hello extension |
| RFC3209-5.3-2 | "On receipt of a message containing a HELLO REQUEST object, the receiver MUST generate a Hello message containing a HELLO ACK object." (S5.3) | absent: no Hello extension |
| RFC3209-5.3-3 | "If the value differs or the Src_Instance field is zero, then the node MUST treat the neighbor as if communication has been lost." (S5.3) | absent: no Hello extension |
| RFC3209-5.3-4 | "If the neighbor continues to advertise a wrong non-zero value after a configured number of intervals, then the node MUST treat the neighbor as if communication has been lost." (S5.3) | absent: no Hello extension |
| RFC3209-5.3-5 | "On receipt of a message containing a HELLO ACK object, the receiver MUST verify that the neighbor has not reset." (S5.3) | absent: no Hello extension |
| RFC3209-5.3-6 | "The receiver of a HELLO ACK object MUST also verify that the neighbor is reflecting back the receiver's Instance value." (S5.3) | absent: no Hello extension |
| RFC3209-5.3-7 | "If the neighbor advertises a wrong value in the Dst_Instance field, then a node MUST treat the neighbor as if communication has been lost." (S5.3) | absent: no Hello extension |
| RFC3209-5.3-8 | "If no Instance values are received, via either REQUEST or ACK objects, from a neighbor within a configured number of hello_intervals, then a node MUST presume that it cannot communicate with the neighbor." (S5.3) | absent: no Hello extension |
| RFC3209-5.3-9 | "If a node does re-initiate it MUST use a Src_Instance value different than the one advertised in the previous HELLO message." (S5.3) | absent: no Hello extension |
| RFC3209-5.3-10 | "This new value MUST continue to be advertised to the corresponding neighbor until a reset or reboot occurs, or until another communication failure is detected." (S5.3) | absent: no Hello extension |
| RFC3209-5.3-11 | "If a new instance value has not been received from the neighbor, then the node MUST advertise zero in the Dst_instance value field." (S5.3) | absent: no Hello extension |
| RFC3209-5.4-1 | "When the links between neighbors are numbered, then Hellos MUST be run on each link and the previously described mechanisms apply." (S5.4) | absent: no Hello extension |
| RFC3209-5.4-2 | "When the links are unnumbered, link failure detection MUST be provided by some means other than Hellos." (S5.4) | Ze RSVP-TE has no unnumbered link: `register.go` configures interfaces by name with IPv4 addresses and every ERO hop is an IPv4 prefix (`wire.go::decodeERO`); link failure is detected by the interface component for every link (`engine.go::handleLinkDown`) |
