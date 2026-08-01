# fixit-vpp-lcp-netns-remediation

`ze doctor` shipped a remediation that broke the dataplane of any operator who followed
it. The detection was right; the "what to do next" was invented. Spec:
`plan/spec-fixit-vpp-lcp-netns-remediation.md` (closed; `git log -p` it for the full
design record). Implementation: `287aa411e`.

## The bug

`doctor-vpp-lcp-netns` fires when `bgp` is configured and `vpp.lcp.netns` is not
root-reachable. It then said:

> Set vpp.lcp.netns to host or root, or run BGP in that namespace.

To VPP, `host` and `root` are ordinary namespace NAMES, not "the host namespace". An
operator following the advice makes VPP `open("/var/run/netns/host")`; absent a namespace
of that literal name, LCP pair creation fails and the kernel shadow interfaces never
appear. The tool whose job is to stop operators breaking things was telling them how to.

## VPP's netns model (verified in VPP's C, now vendored in-tree)

Each fact at its producer in `third_party/vpp-linux-cp/src/`:

| Producer | Line | What it produces |
|----------|------|------------------|
| `lcp_set_default_ns` | `lcp.c:73-74` | `format (0, "/var/run/netns/%s%c", ...)` then `open`. The leaf is a NAME under `/var/run/netns/`. `host`/`root` are not special. |
| `lcp_get_default_ns` | `lcp.c:28-36` | Only RETURNS `lcpm->default_namespace`. It formats nothing and opens nothing. |
| `lcp_itf_pair_create` | `lcp_interface.c:850-855` | `if (ns == 0 \|\| ns[0] == 0) ns = lcp_get_default_ns ();` An EMPTY per-pair netns falls back to the GLOBAL default. |
| `startupconf.go:106` | | ze itself writes `default netns <leaf>` whenever LCP is enabled. |
| `RealListenerFactory.Listen` | `internal/core/network/network.go:167` | Bare `net.ListenConfig`; `Control` only sets MD5/TTL. **BGP has no netns awareness**, so the DETECTION is correct and must not be weakened. |

**The keystone:** the model is TWO-LEVEL. `""` means "VPP's own namespace" only when the
global default is unset, and ze never leaves it unset when LCP is enabled. So there is
NO config-only remedy. The honest next step is deployment placement: run ze in the
namespace where the TAPs are. Netns-aware binding is specced (`plan/spec-bgp-netns.md`),
not implemented.

Deliberately NOT named as a remedy: `mount --bind /proc/1/ns/net /var/run/netns/host`.
It would give the literal name a meaning, but VPP's behaviour on such a bind-mount was
never read at a producer, and ze neither performs nor documents it. Naming it would have
repeated the original bug in a new costume.

## How it survived three sessions

The false claim had FOUR assertion sites (`doctor.go` message, `doctor.go` doc comment,
`lcp.go` doc comment, `codes.go` registry Description) and ZERO verification sites. The
entire evidence base was Go comments plus a generated binapi stub
(`vendor/go.fd.io/govpp/binapi/lcp/lcp.ba.go:354`, "optional tap netns"), which documents
only that the FIELD exists and cannot express a resolution rule. Both banned patterns in
`ai/rules/no-fabrication.md` (comment-as-intent, binding-stub-as-foreign-semantics) fired
here at once.

Five unit tests covered this check. All five asserted the diagnostic's COUNT and CODE.
None asserted its MESSAGE. That is the gap the bad advice shipped through: a test proving
a check FIRES says nothing about whether its advice is survivable.

## Gotchas

- **A citation into a scratch fetch rots the moment the source is vendored.** The Review
  Gate caught this on the fix itself. The comments cited `lcp_interface.c:856-861`, correct
  against `tmp/vpp-lcp/` (the author's fetch, git-ignored via `.gitignore:12`), while the
  same commit vendored a DIFFERENT copy into `third_party/`, where the fallback is at
  `:850-855` and `:856-861` is the sub-interface block. The vendoring pass substituted the
  PATH and left the LINE NUMBERS, so the citation pointed confidently at unrelated code.
  The tell was cosmetic: three comment lines left ~125 chars and unreflowed. **Cite the
  in-tree copy, and re-anchor line numbers when a source is vendored.**
- **The same pass misattributed the producer**: comments credited `lcp_get_default_ns`
  with formatting/opening the path (it does neither; `lcp_set_default_ns` does). A
  plausible function name is not a producer read. Both slips are this spec's own Core
  Insight recurring inside its own fix.
- `plan/spec-bgp-netns.md` (`:202`, `:332`, `:448`, `:738`) still cites
  `lcp_interface.c:856-861` against upstream FDio/vpp master, which it labels honestly.
  Now that `third_party/vpp-linux-cp/` exists in-tree, it may want to re-anchor when it
  next lands.

## Known limitations (live, not fixed here)

- **The check is still SILENT for `vpp.lcp.netns host` (or `root`).**
  `lcpNetnsIsRootReachable` returns true for those names, so no warning fires, yet VPP
  resolves them as literal names and LCP pair creation fails at apply. An operator setting
  `host` gets no diagnostic and a broken dataplane. This is a DETECTION gap, distinct from
  the false-remediation bug fixed here; detection was frozen by Thomas for this fix.
  Tracked: `plan/deferrals.md` row dated 2026-07-16, destination
  `plan/spec-fixit-vpp-lcp-reachability.md`.
- **No functional `.ci`.** `test/ui/doctor-vpp-lcp-netns.ci` remains absent; it is AC-12 of
  `plan/spec-fixit-vpp-lcp-reachability.md`. The two unit tests drive the real check
  function and the real registry instead.
- `SeverityWarning` retained. Thomas chose "fix the message now, standalone" over also
  dropping the check to a note.

## Rule candidate (raised, not applied)

`ai/rules/doctor-checks.md` "Test Requirement" says a check's unit test must assert that it
emits the registered code. That is precisely what the five existing tests did, and it was
not enough to stop false advice shipping. The requirement could extend to asserting the
diagnostic's REMEDIATION. The sibling `doctor-vpp-wireguard` message is equally unasserted
today. This generalizes `ai/rules/fail-closed-guards.md`'s test corollary: for a
diagnostic, the shape that should be rejected is BAD ADVICE, and only a message-content
assertion rejects it.

## Design notes

- Assert diagnostic text by REGEXP over substantive properties (banned directive shapes,
  required remedy clauses), never the exact string. `plan/spec-bgp-netns.md` AC-3 will later
  narrow this message; a property test survives that rewrite and keeps guarding the
  invariant, while a golden-string test would be deleted with the first cosmetic edit.
- The message keeps its stable leading phrase `bgp is enabled and vpp.lcp.netns=` so log
  scanners still match, quotes the offending value, and pushes the long form to
  `ze explain doctor-vpp-lcp-netns` per `ai/rules/error-messages.md` ("if the next step
  needs more than one line, attach a diagnostic code").
- The "host/root do not work" clause is kept in the MESSAGE, not only in `ze explain`: the
  old advice is in the wild (operators' notes, and `docs/guide/vpp.md` carried it until
  `c49d36524`), and a reader who already believes it will not run `ze explain`.

## Core insight

An error message's "what to do next" is a behavioral claim about a foreign system, and it
must be verified at that system's producer like any other claim. This one was verified at a
Go comment instead, and the comment was its author's belief. `ze doctor`'s remediation is
not documentation: it is executed by operators, so it earns the same producer-verification
bar as code.

## Files

None recorded.
