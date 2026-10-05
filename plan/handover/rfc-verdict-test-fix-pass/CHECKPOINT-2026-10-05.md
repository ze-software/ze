# Portable recovery checkpoint — 2026-10-05

This is a recovery snapshot, not completion of the BGP work. The owner selected
this form so another machine can recover shared dependencies without landing
another session's unfinished source under this session's commit.

## What is saved

`checkpoint-2026-10-05.tar.gz` contains:

- `working-tree.patch`: all 950 tracked-path deltas against base commit
  `dc3c3cd998b6e81e1c7ad62f807e45388af8eb70`.
- `payload/`: 131 untracked source/configuration files and the ignored plugin
  drafts. These are deliberately not promoted to the live test suite.
- `evidence/`: pending RFC judgments and requests, semantic overlays, native
  logs, LLGR stress captures, historical D-15 approvals, session state and
  independent agent reports.
- `manifest.json`: the captured population, modes, sizes and SHA-256 hashes.

Archive SHA-256:
`9181a0f25f7208eaa4863c95efa9a279015fda3d425a58d96811f3c9f539c896`.

The archive was reopened and every manifest member's hash verified. Its tracked
patch was checked and applied to a disposable export of the base commit; every
payload file was restored there without collision and compared byte-for-byte.
This validates recovery, not compilation or protocol correctness of the snapshot.
Binaries, caches, VM images and the complete process transcript are not included.

## Offline transfer

The resume document, recovery archive and `bgp-checkpoint-0c516ea8fb.bundle`
are saved in `plan/handover/rfc-verdict-test-fix-pass/`, not `tmp/`.
The bundle carries checkpoint commit `0c516ea8fb` and requires existing commit
`ae727e4ff19d44d3d2b6328d1725251a59a0b4df` in the destination repository.
Bundle verification passed, and the saved copy matches its verified SHA-256:
`f67bc67a398384124f2bd72bb5c66d36c0320e8ac033f949a547f9cf5603bdd9`.

Copy the bundle to the other machine. In its clean repository, run:

```sh
git pull --ff-only /path/to/bgp-checkpoint-0c516ea8fb.bundle main
```

If fast-forward fails, stop rather than overwrite divergent work. The bundle
carries the original checkpoint instructions; this transfer section was added
after that checkpoint. Then follow the recovery steps below.

## Restore after pulling

Start with a clean checkout and no other writer. Extract into a separate empty
directory, never directly over the repository:

```sh
recovery=$(mktemp -d)
tar -xzf plan/handover/rfc-verdict-test-fix-pass/checkpoint-2026-10-05.tar.gz -C "$recovery"
git apply --check "$recovery/working-tree.patch"
```

Before applying, compare every `payload/` path with the checkout. An existing
file, including a dangling symlink, is a conflict: stop and reconcile it rather
than overwrite it. If the patch check fails, some work may already have landed
since the recorded base. Do not reset, force-apply or discard newer work.

Once the patch and additions are conflict-free:

```sh
git apply "$recovery/working-tree.patch"
set -o pipefail
tar -C "$recovery/payload" -cf - . | tar -C "$PWD" -xkf -
```

Keep the extracted `evidence/` directory. Its files are the portable substitutes
for the old machine's `tmp/` artifacts. Read
`evidence/state/session-state-rfc-verdict-fix-bgp-01a0fdb1-2ee3-7297-adf9-cd21f70400f0.md`
from its final section first. `evidence/scratch/checkpoint-runtime-commands.json`
contains the executed command recipes. Rebase their old absolute checkout,
session and overlay paths; rebuild native tools and DUTs on the new architecture.
Do not reuse the old machine's executable paths or claim its evidence as a fresh run.

## Resume in this order

### 1. Fix the reproduced normalized-OPEN defect

The physical LLGR readvertise stress failed at invocation 18 in
`evidence/stress/bgp-plugin-draft-llgr-readvertise-20261005-224246.log`.
GR dispatched `retain-routes` with `afi-3/safi-0` and `afi-384/safi-0`, then the
observer found no retained routes.

`internal/component/bgp/format/decode.go::formatCapability` currently hex-encodes
`cap.WriteTo`'s complete TLV into normalized `Value`. GR's JSON OPEN consumers
expect value bytes. A valid `000300010180` payload becomes
`4006000300010180`: the code/length pair is read as restart time and the real
restart-time header becomes a family tuple. Repair the normalized producer,
not the GR decoder, timer or family list.

Two new untagged regressions are saved but **unformatted and unexecuted**:

- `format/decode_capability_value_test.go::TestDecodeOpenOpaqueCapabilityValues`.
- `plugins/gr/gr_open_normalization_test.go::TestNormalizedOpenRetainsNativeFamilies`.

Run them before changing production, through native `job run`, with `ze_core`
and all unique tags from `feature-gates.txt`. Then change the fallback from
`hex.EncodeToString(buf)` to `hex.EncodeToString(buf[2:])`, document the value-only
contract, and remove the old incidental Route Refresh spelling assertion rather
than repinning it. Update `docs/architecture/api/json-format.md` and the existing
GR guide. The independent diagnosis and exact test selector are saved in
`evidence/scratch/checkpoint-agent-reports.json`.

After the fix, rebuild both the native runner and DUT. Rerun all four original
LLGR workflows with `MAY_ATTACH=0`, `any-failure`, 80 iterations, parallel 2 and
burners 2. Transition, RIB-stale and peer-stale-time reached 80 in the latest
pre-fix binary; readvertise did not. None establishes the pending JSON-path fix.
The older `20261005-202249` captures contain 19/23/23/29 follower results and do
not establish 80 physical executions. Do not promote from those counts.
Three LLGR workflows remain drafts; peer-stale-time is already live.

### 2. Repair the remaining real workflow failures

| Surface | Last observed result | Portable log in `evidence/scratch/` |
|---|---|---|
| MED, FRR/GoBGP inputs and pmacct, nondefault network | Pass | `job-peer-final-bgp-addpath-best-path-pmacct-344d6809.log` |
| Ordinary OSPF/FRR | Pass | `job-peer-final-ospf-te-frr-3d5d6c4a.log` |
| Inter-AS OSPF/FRR | Assertion 0: peer output lacks `inter-as` | `job-peer-final-ospf-te-interas-frr-4ee78923.log` |
| Guest VPN RS IPv4/IPv6 | Pass | `job-bgp-linux-current-consumer-drafts-09998778.log` |
| Guest AIGP | Recipient did not acknowledge AIGP107 and unknown-cost withdrawal | Same guest log |
| Guest VPN RR IPv4/IPv6 | Peer-exchange mismatch | Same guest log |

These are observed failures, not requests to reproduce them again. Inspect their
preserved diagnostics and producing code. The AIGP, VPN and FlowSpec interop
commands chained after inter-AS never started. LLGR's independent FRR proof and
two prepared semantic interop overlays also remain unexecuted. Serialize interop
runs that share staged binaries/images; `NO_BUILD=0` is required for fresh or
mutated DUT proof. The former VPP compile blocker was repaired externally; the
latest host and Linux builds succeeded.

### 3. Finish native RFC evidence without duplicating successful work

Main is the sole canonical writer for this BGP child. Serialize
`discriminate-record`: its overlay scratch is shared within a session.

- RFC9494-4.2-1/4.2-2, RFC7311-3.4.3-6, link-local 4-1/4-5/4-9 and
  RFC9552-5.2.1-1/5.3.2.1-1 have been independently rejudged and stamped.
- Four new OSPF discrimination records succeeded; preserve existing IS-IS covers.
- The 27-request VPN/FlowSpec batch recorded requests 1–18. Request 19 timed out
  after 15 minutes and wrote no record. Requests 19–27 remain owed. The full
  output is `checkpoint-native-record-batch.txt`; the interrupted overlay is
  under `evidence/interrupted-discrimination/`. Diagnose the timeout before retry.
- Request 15 captured a handler-factory setup panic, not packet-editing execution.
  Replace it using the corrected `applyNextHopFamily` target in
  `flow-vpn-native-requests.json`.
- The three MED requests in `med-final-native-requests.json` remain unexecuted.
- Stamp the accepted MED, RFC8277 and RFC5575 pending judgments with `mode rejudge`.
  RFC8955-4-3 is a **first** stamp; it has no existing canonical verdict.
- Foreign RIB changes stale additional RFC8277/RFC8955 records. Reconcile current
  producer fingerprints before renewal; do not repeat a successful unchanged run.
- Native halt records establish reachability, not a semantic mutant result.
  ADD-PATH's ingress-ID substitution produced four concrete reds and clean race
  passed; FlowSpec, AIGP, MRT and MED semantic evidence is preserved separately.

### 4. Reconcile closure before claiming eligibility

The child and parent remain open. The independent report in
`checkpoint-closure-review.json` identifies row/disposition gaps beyond the
runtime checks: LLGR's time bound, Link Name splits, MPLS reserved fields,
SR-policy optional fields, multi-label reserved fields and other AC-C3/C6 items.
Treat that report as a source-reading guide, not an already verified ruling or
permission to implement absent features. Remove stale blocker metadata only
after checking the current canonical verdict and exact owner AC.

External RFC8654-5-4/6-1 ownership has no confirmed P-3 transfer. The other session
owns enum/directional-length/AS_PATH, VPP, tooling, site and test-health work.
Its identity is `01a10694-3795-71f2-8250-25e3a877f4cf`; direct messaging was not
available and acknowledgment is unconfirmed. Shared snapshot inclusion is not
transfer of that ownership. Do not duplicate its global lint/site suites.

### 5. Complete the original remaining scope

Keep the forwarding pooling/dedup/lifetime work, all four LLGR workflows,
remaining owned BGP findings, coordination and eligible child/parent closure
as separate unfinished obligations. Still owed are these forwarding workflows:
`rfc4271-partial-unknown-transitive`, `bgp-rs-mod-copy`,
`bgp-rs-reactor-fastpath`, `bgp-rs-reactor-fastpath-fallback`,
`bgp-rs-community-strip-multi`, `bgp-rs-community-strip-multi-fastpath` and
`bgp-rs-asn4-transcode`. MED also owes `rib-best-selection`, `bestpath-reason`
and `show-rib-best-addpath`. Final affected race proof, draft promotions plus
the whole live suite, and independent closure gates remain open.
MRT and EPE are already landed as `67a74ef4d4` and `ba1176da6f`; do not recommit them.

No protocol fix, test assertion, scope boundary or completion criterion is
weakened by this checkpoint. Resume implementation only after recovery and the
current repository's session-start safety checks.

## Publication limit

The owner authorized publication for machine transfer. The native commit tool
refused the requested push because 138 verification-debt rows were open.
The checkpoint can be committed locally, but an ordinary remote pull cannot
retrieve it until publication succeeds. Do not bypass the push gate or describe
the local checkpoint as already available from origin.
