# IPsec checkpoint for continuation, 2026-11-13

Branch: `main`. Both specs remain `in-progress`:

- `plan/release/fixes/spec-ipsec-dual-form-receive-spd-bypass.md`
- `plan/release/fixes/spec-rfc4301-architecture-gaps.md`

The owner asked to commit as much existing work as possible and then report so
he can decide whether to continue here or move machines. This is a checkpoint,
not closure. No push was ordered. Dual-form AC-4 is unchanged: RFC4301-5.2-2
must lose `{partial}` only when the broader obligation is actually proven.

## Landed work

| Commit | Change and evidence limit |
|--------|---------------------------|
| `8b3c7a34cd`, `2db581e7d6` | Earlier RFC 4301 implementation checkpoints |
| `abeb7d6535` | Dual-form implementation; the old journal refusal was resolved. Its incorrect `Spec-work: deployment-user-journeys` metadata must not hide its files from final review |
| `bd8e21b487` | Receiver diagnostics, ESP GRO doctor check, and the three judged test corrections |
| `14bec40b17` | Independent source directories for concurrent kernel workers; named regression and scoped race package passed; all five committed build configurations compiled |
| `352f231413` | Netlink selector-family and state-scope work, repaired forward-application headers |
| `b43120b371`, `c9ce6ba81d` | Ordered netlink series and upstream-style tests; see the library handover |
| `5c063bc8d6` | K1 exact state-scope and K2 fragment-context kernel patches; no native C compilation or packet proof |
| `2ae4f0f3f4` | K3 draft preserved at `gokrazy/kernel/proposals/xfrm-fragment-reassembly.patch`; unenrolled and unproven |

Go caller checkpoint `466ab09fd8` landed 55 files, including the callers,
selector repairs, diagnostics, regression probes and companion documentation.
The required `./le repo compiles check` passed at `466ab09fd8b9` for all five
configurations: distro, appliance, setup, host and installer. This checks
committed production code and dependencies; it does not compile test files
or prove the kernel C patches. No fresh tests ran for this checkpoint.

The Go checkpoint carries immutable scope construction, exact projection and
capability admission, single-pair inbound state binding, protocol/MH refusal,
FWD-default ownership and doctor reporting, and the saved packet regressions.
It does not enable full scope installation on a partial kernel receipt.

## Evidence already obtained

All paths in this section are under
`tmp/session/2026-11-10-225434/scratch/` unless stated otherwise.

- `job-ipsec-dualform-host-redwalks-aa95f75a.log`: all sixteen remaining host
  mutations, 27, 31–33, 37 and 39–49, returned native red verdicts with each
  clean own-test pass and broken own-test failure. Guest mutant 20 is not run.
- `job-unit-pkg-58fce57a.log`: targeted corrected dataplane package run passed.
  `job-ipsec-reopen-schedule-e00a01c8.log`: narrow race/count-20 scheduling
  run passed. These are not the final guest population.
- `job-ipsec-binding-doctor-units-419db7fc.log`: named receiver doctor and
  binding fixture tests passed, including ESP offload findings.
- `job-ipsec-scope-params-family-green-67381781.log`: targeted immutable scope,
  capability admission, VPP refusal, projection and selector-family units
  passed after observed semantic failures. This proves Go behavior only.
- `job-ipsec-kernel-workspace-package-81a675f5.log`: kernelbuilder race package
  passed; its optional Docker cancellation test skipped because
  `KERNEL_CONTAINER_PROOF_IMAGE` was unset.

## Remaining work, in order

1. Inspect the committed Go checkpoint and coordinate the private-ABI move
   with the netlink-series session. Its handover is
   `plan/handover/2026-11-13-netlink-xfrm-upstream-series.md`; that session was
   waiting for the caller checkpoint before editing the same dataplane files.
2. Compile and run fresh final guest binaries for dataplane, engine and
   trafficusage. Use the exact selections in `ipsec-close-validation.txt`,
   step 3, and require each named test and required subtest to run. Then run
   step 6's guest mutant 20. Regenerate changed mutation coordinates rather
   than accepting stale-hunk failures as red evidence.
3. Run the doctor draft at
   `test/draft/ipsec/ipsec-esp-form-fast-path-doctor.ci`, prove all 80 stress
   iterations with the documented selected-test harness, promote through
   `ze-test`, and run the promoted test. Final stress and promotion are owed.
4. Run the remaining interop scenarios and their deliberate breaks:
   `mtu-nat-installed-endpoint`, `icmp-error-payload-check` with bare ESP and
   both redwalks, `esp-form-change`, `esp-form-firewall-drop` with its redwalk,
   and `ipsec-bgp-redistribute-frr`. Step 9 of the validation file gives the
   native commands. Earlier success is not final-code evidence.
5. Produce RFC4301-5.2-2 positive and negative records, RFC7296-2.24-2
   positive, and refreshed RFC5082-3-4 positive. Step 7 gives the routes;
   regenerate line/column selectors from current source. Phase 4 also owes
   its 13 mutation walks, four port-field records and affected audit reseals,
   prepared in `phase4-validation.txt` and `phase4-proofs-run.py`.

Phase 2 is needed after those five work packages: finish and prove the kernel
fragment/SA-scope work, strict ICMP return mapping, and engine scope writer;
then complete scoped review, RFC checks and the two spec-closure commits.

## Kernel work that remains source only

The active kernel series contains 0001–0004. K1/K2 advertise only feature
bits 0–2. Bit 3 and the full-guard runtime configuration remain disabled.
Do not claim full capability from these patches or from the Go unit tests.
Freeze the patch directory during a kernel build: the native cache identity
hashes every file there, not only enrolled series entries.

K3's receipt/reference/accounting review found no issue, but the route review
found a source-reachable transport defect: authenticated IPv6 transport ESP
can carry plaintext Fragment headers and reach reassembly. Correct the current
authenticated transport-state boundary before reassembly and output transform.
Preserve legitimate outer ciphertext fragmentation and distinguish atomic
fragments. Do not reject every earlier transport entry in a nested secpath.
No fix or runtime probe for this finding has been implemented.

Old Ze-kernel probes also exposed mixed-protection fragment delivery and
general multi-pair/overlapping SA-scope leaks. Forwarded ICMP return under
`unmatched discard` remains red; the strict final-policy marker design is in
`rfc4301-icmp-final-marker-design.txt`. Do not replace it with an ambient ICMP
allow rule. The saved new probes must pass against the finished kernel.

## Uncommitted and separately owned work

The RFC spec's scope/publication-reconciliation edits remain uncommitted for
owner/closure review. The checkpoint selects factual lifecycle updates and the
owned family row only; it does not publish acceptance-criterion replacements.
Preserve the complete working-tree RFC spec when transferring machines.

The seq-4 expected-error hunk in
`test/parse/ipsec-icmp-type-selector-refused.ci` belongs to the config-enum
spec and remains outside this checkpoint. Numerous other sessions' changes
share this checkout. Do not stage, remove or overwrite those changes.

The three separately named fix specs remain open:
`spec-privilege-drop-keeps-capabilities`,
`spec-anomaly-shape-limit-drops-nothing`, and
`spec-ike-peer-info-reads-child-after-unlock`.

## Transfer and recovery

Carry the committed branch through the owner's chosen transfer route; pushing
still needs his order. Git alone does not carry ignored proof artifacts.
Preserve both session trees, including state, scratch scripts, manifests,
approved proposal patches and raw logs:

- `tmp/session/2026-11-10-225434/`
- `tmp/session/2026-11-09-222726/`

The current state files are under the first tree's `state/`, named
`session-state-ipsec-dual-form-receive-spd-bypass-c43b4620-2510-44dd-8936-84100b444bb5.md`
and `session-state-rfc4301-architecture-gaps-c43b4620-2510-44dd-8936-84100b444bb5.md`.
The main harness identity was `c43b4620-2510-44dd-8936-84100b444bb5`, commit
namespace `79a29bf3`. A resumed session uses its own identity and native claim;
inspect the generated `Spec-work` trailer because the old claim drifted.

The pinned archive is `tmp/qemu/build/x86_64/linux-7.3.tar.xz`, SHA-256
`87363c24f10e06321eb362765521094e94471f6bff8bce2d6b3e40f39f004953`.
The third-party work and immutable snapshots are under scratch paths beginning
`rfc4301-kernel-`; preserve the proposal plus manifests before any cleanup.
No deletion or cleanup was authorized by this checkpoint request.

Cached kernel/image and executable paths in the command file are machine
specific. The old baseline used Ze 7.3 on amd64 with KVM. Rebuild on the new
host's native architecture through the registered actions and record kernel
identity; copied old binaries do not prove the current source. Tests run in
the main checkout, one CPU-intensive job at a time, with no `./le verify` run.
