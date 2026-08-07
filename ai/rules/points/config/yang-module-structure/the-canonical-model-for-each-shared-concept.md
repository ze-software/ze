---
kind: table
level:
stage:
---
| Concept | Canonical | Do NOT |
|---------|-----------|--------|
| BFD integration | `container bfd { leaf enabled; [leaf mode;] leaf profile; }` referencing a profile in the top-level `bfd { profile <name> }` list (BGP's pattern) | redefine BFD timers inline (`min-tx`/`min-rx`/`multiplier`); every protocol that supports BFD references a profile |
| Authentication | reference a shared `key-chains` list (IS-IS's model) via a `leaf key-chain`; name the auth container the same everywhere | a per-protocol private key store; container named `md5`/`auth`/`authentication` differently per protocol; reference leaf named `key-chain` in one place and `auth-key-chain` in another |
| Per-interface protocol config | `container interfaces { list interface { key "name"; ... } }` (OSPF/IS-IS) | a bare top-level `list interface` (RSVP-TE) or a `leaf-list interfaces` when per-interface settings exist (LDP) |
| Multiplier / interval / timer names | one vocabulary for the same concept; dimensioned via a `units` statement (see Units) | four names for one concept (`detect-multiplier` vs `multiplier` for the same BFD field) |
| Toggle | positive `enabled` at every nesting level, including sub-features | `enabled` on the interface but `enable` on its sub-blocks |
