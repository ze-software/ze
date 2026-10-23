<!-- ze-review spec=le-command-namespaces verdict=clean rounds=1 reviewers=HistoricalNamespaceClosureReview model=unknown ts=2026-10-23T10:08:40Z -->
# Independent review — le-command-namespaces

files:
  81885c3155433725054185dd9d24518730b7a78b71170e9ee07d304f9c96cc80  ai/rules/cli.md
  7d88407628e20c7f35cb8a2d8ec7cba6e1c118efca1013c056a6b2997cf7c6fe  cmd/ze/ze_le_register.go
  9a9ec7069fb64934dd875238a6b3212830eebc81d14579e8db9987422f7ead93  docs/architecture/cli/command-namespacing.md
  65b89491e943effb8698b27ebf8d9e87e98060729d6da6adf9fbf0d2d79a1000  docs/architecture/cli/root-namespace-grammar.md
  7d414931f0b8ece57ce3dc2aa0bb7a19115cada673acceafa1d4bae672b114a1  docs/architecture/core-design.md
  64aeacb293278ca3fe8ea575d08926479004e63ab4c9a58d2067c68b1977f086  docs/architecture/testing/verify-freshness-scope.md
  f170fa5ecf5c2e32ee0ab22b6e0ca137201f55b0719315195561f4fdbfa49eb1  internal/component/command/answer_shape.go
  05c92d52db263475a2c096903db4aea439feb5092588e283ff267758f57a3e0e  internal/component/command/grammar/checker.go
  274155039956c9bd4dee644dd920defb1948b8acdbd92ad85d2d0aa372fdd3f3  internal/component/command/registry/registry.go
  045a5a25f555a2d44b406ce0537d98edc7f6f0313101ac1b8978222322c65a6c  internal/le/actions_test.go
  0968c4cd3fefbbfe6132046f80b01676f4b7441bb47f0a8601e2a0fb8689b130  internal/le/cligrammar/actions.go
  0069adff9f19cf16b94718895faa47adeee439d6f76990f3d70743322e96dc4e  internal/le/cligrammar/cligrammar.go
  f2b3cf7bbfb4ab5cdb23d80cd73a703f3ebc0b0b3c44700ee09068b5d8ef4af5  internal/le/cligrammar/cligrammar_test.go
  a059ac0fc9a194055426beb29273cb2bd14236d7a47c9b3fa61935ea77e8a325  internal/le/cligrammar/register.go
  cd2a554bafc4675457b7accd2e04432461590ca58cd8a1506d2ffb416df55252  internal/le/command/list/commandlist.go
  a90f5b92602fd95561bdcd77aff08f8ac7eff89f635a01183c6348a1a03ddaaa  internal/le/command/list/register.go
  d1c0be463df250e7775ab2f73d282a2de0aae50db2072050935bf29021451a4c  internal/le/command/ownership/commandownership.go
  e7bad06a49629ea19db69d4cd5483ce4fbc7561bfcbaae0b9082815249ba6cc9  internal/le/command/ownership/register.go
  7adee16372edeb7e8d4d3436557ef8ad3d5c8a72818c49d0195278a3f37572f7  internal/le/completeness_record_test.go
  1370388c670a3778e4bbe350e3ace1ce3380039da780911ab611f991904d99f1  internal/le/completeness_test.go
  426529da407fcc073e4a2fea1c8e12917984e1bd9a2be408bde91efb4edf86d1  internal/le/config/claims/configclaims.go
  733fe16f0572e7e0bce9b5f105ba65f2faa2abd83f8840fc50e57500e2231f91  internal/le/config/claims/register.go
  17a86a2e59ed5dbdbd79fb6608e934ca6e503bb4ffaddb899466e19c08462c02  internal/le/config/coercion/actions.go
  7e53d7f486a5e2b5ab77ca7ccb48a2f0c0637f7579da56b4965f3c8db52d9e27  internal/le/config/coercion/register.go
  e7287e409ecb3c1b6723c256c5d60821038ebad444723fa326cb2b6d96550c34  internal/le/doc/check/actions.go
  52b038bfa719068b43f304d6601d85fb9aef4acd0e73bcd73d3fea6a0f494132  internal/le/doc/check/register.go
  5bb25467ea770045266ebd4083c71de7a3d40b75f0ebf4ac7ea8c53937ee6b61  internal/le/doc/wiring/docwiring.go
  baf6116dd45665918c00645b9326ba59d1d181077275c65d1ad9468dba2ff1e8  internal/le/doc/wiring/register.go
  dae18e68281d6eaa20d1aa02539c55c8681312cc42d49802e92fb64b327a6632  internal/le/gaterun/gaterun.go
  c7908482fdb00bcb9aa290318e3f9e76988f54612cc05bff91cc8273595ab6dc  internal/le/group_test.go
  1bc5fba18b971868a08d835298524225ebf7589928df6b9c509c7ea0e6702792  internal/le/leaction/leaction.go
  17e9b28bd788bf416b36ffd3a5ba506410fa2e81457bcaa29da8f81559fc069d  internal/le/leroot/dispatch.go
  1fbc116a38d08a567043aca9d221c11358afdd5c5f080755df64a1ee7d285a8d  internal/le/leroot/leroot.go
  f3ea7c1c151e6c0e148feff597c189b51bad3e44b69f739d85facc5376f6d00f  internal/le/leroot/leroot_test.go
  f6b82091b3c62b661d942810f412dbcf7941d19b8a205ee0c3c59968f28da684  internal/le/leroot/namespace_test.go
  c9a1ec4c5ba4e2c892e9e63aac9c339969cd8b5487a925cbd448415ff3299e8d  internal/le/plugin/boundary/actions.go
  9151eff8d2afa40edeb4683eebafb12c9fe0eab56d549294eaa8fb982265d1f5  internal/le/plugin/boundary/register.go
  509fc57f68a851eccbaefcdf88dd329e87559a3d8586459bb64e576387ecfeb8  internal/le/plugin/declarations/register.go
  7c9403aa7d41b8e28c568dfb73bd28799c0fe06b918c278b9c60aa4e6b14c25a  internal/le/plugin/imports/actions.go
  c79ef63d2f9b4e99847b0c7c00d512fb050ce69aec59a5a8a29510023f1aef03  internal/le/plugin/imports/register.go
  e4390262fa266b08971ef92e09328814dfa71674cfb3a3b8b25b8e0680f6ca05  internal/le/register.go
  c72b5113143cb54006237c5ebd3c27e9ff1bb5c0a3cc5c5b9aa12a8b4c3c911b  internal/le/register_test.go
  2dcc45740b8edcf54457763fa4a1f80218caa5ed38781d96f9c17f886129464a  internal/le/repository/actions.go
  d05cb6058162e0206ca7965c799073f141da543809cb34b8cfc46cfd59882907  internal/le/repository/register.go
  fe61e72c9ee09fd4bb58e885ef0f8254d2c825399cdd1ca03f71c16daaead4a1  internal/le/repository/trackedbuild/actions.go
  cf20345b63b9ef894b7f103fb0a7bfc2990693c25d78ca592939712699a23008  internal/le/repository/trackedbuild/register.go
  90ea3eba433698a81629450c2c5baade98ea776437951d26a58bcc7a31aebd23  internal/le/site/actions.go
  b77cdeb6ec0bfc89448efe3bee11a4f664a3888ebc5d5a9c758c6fd61cbead9d  internal/le/site/facts/actions.go
  726ccd44f3ddb9256558ba30f19f812a28d0d2e9f8d2bd7aaf9aa6e062111acb  internal/le/site/facts/register.go
  a6d62904c286c816dd2c5063384cfd44435c9abdb90f8b282ad2ffd8a320ce17  internal/le/site/register.go
  7db28d84af523104de7f119a89cddc578257ab23f3212615acb5fe91dbb24208  internal/le/site/wiki/register.go
  e0c442a49a3a3b06cd934878ae129d1d101470f08232e7bda6e9a64e56bfa920  internal/le/spec/citation/actions.go
  c3faa4affe7fde4a99ed0302e2288fb7de77f16e4f655dc649b7842053f8de28  internal/le/spec/citation/register.go
  4f648c1f0ff6a462055b88129be890d2c136e78ef4e85d35b6e7e20a96f3a4fc  internal/le/spec/roadmap/register.go
  a55ddd2aba29c58b84f2658eb0d786e4a0f3bec5c9808db065326284fcfa465c  internal/le/spec/session/actions.go
  60478e2f383d18961ec39124e42b1ce8f54aba43d24b7315f6be5db3cfb19d4c  internal/le/spec/session/register.go
  b13081cfd3f8f8e0f5cae17d25660327e5cfea9089c44a76ab85a75090fea5d1  internal/le/spec/status/answer.go
  572878740c75d3c6b8bd3e7681d8afb58d7a44c0985fcf748296534f1e919793  internal/le/spec/status/register.go
  292d90a4de2d8ccb8025bd5443a5f4883aa39aeb2ab734ed3b73a7a88cb9ddf7  internal/le/verify/actions.go
  18536c0debd69ea96625a67fe54432d7956d8778ce3c5c7bab1a344f501ef10a  internal/le/verify/deps/actions.go
  5ce9276cd524ab2ec6e86a81c8211874913320a497db6dd7db56af13cc2f86fe  internal/le/verify/deps/register.go
  f46a5f85b6ab13f281aef658a7348bc661a62ecc3869d591a0c6322424fffc50  internal/le/verify/deps/verifydeps.go
  23cbf3850e5f45a370327c916734dd3224c1b68102ae4a26e2305df7d7b94a5e  internal/le/verify/dispatch/dispatch.go
  2807e1aa9e44bdd27f0570a5767d9b6e532ce378845af15842430ca299733afa  internal/le/verify/dispatch/dispatch_test.go
  f9038f7ece2ea1e5251b92d776e120bb2df0c967edce2019d14f99c684bcd548  internal/le/verify/engine/part_test.go
  1ae437ffa12e382cb237a660d3136b7b92ed6d21e03a86135b6af4d807ebbaea  internal/le/verify/engine/run.go
  1604d56d36c6695f7824ef1b09ff71d78d43bd9687a9ab8656b640fd95cc22b0  internal/le/verify/engine/run_test.go
  ea31ea6230d82a0078aa692b7eaacd995cca437db8be3befbd2c5bce70297842  internal/le/verify/engine/stages.go
  f73475e35e69905065c41f765f2db394538b0fb36236184271f689d9f56920a3  internal/le/verify/engine/stages_structural_test.go
  792db051def166113dc9b8fa3ab1db29c72da296b16b39c070deb3af6fe985ac  internal/le/verify/engine/verifyengine_test.go
  5ffff5f7c33f995b6a0210f8d21a535943cf9872182932fbb2c4a80461f9c3ff  internal/le/verify/lifecycle.go
  c9bf892d368f7fc63cbf770480fa6c6eb59ad83ff81b20adcb772014ac1eef13  internal/le/verify/lint/actions.go
  0e9332eedfdd4810ff54083b8dbe9330200abbaead502249ade8c5b8aa49bc7c  internal/le/verify/lint/matrix.go
  c8067faead6ac07a027a2ed968699dfa1adcdf0171f2677cbf4f6d18e3a6cd97  internal/le/verify/lint/register.go
  f216bc3a60281577040398d7a2d30dd3d8ffe06be187828e285fcd46f3acdb41  internal/le/verify/lock/answer.go
  d8a772e3fa2b6a35fd7855006015f09693ad603d6d09914985f5586b3f7c91f5  internal/le/verify/lock/register.go
  5bde9f3c0f4ebfec22ef6795f040f6289e7ce506dd8e66edc2dd1c505e6a0e66  internal/le/verify/register.go
  7d9ef371595d21464108aece302f4656d72368740f345494be8c216354f8169b  internal/le/verify/stagelogname_test.go
  93c2149ecaf78d9d5de6430560027d1d31e670d2bb1adcac79d952fb9219f017  internal/le/verify/status/answer.go
  63e4dc2d685d1844adf6ad77540d03f9bcea62c0c1cebf3e7d7cfc0412a9673d  internal/le/verify/status/register.go
  cf76144232498ecbc03dc3ce4ede3ea8af6e9e776bf611b3f756050672ee0fc5  internal/le/verify/summary/answer.go
  3b23de0685d14799fa6575f451e7bd8c317d398736386dcec075a46011664519  internal/le/verify/summary/register.go
  7757d8aa32cb9a1000a69a86b9e7c83241cf00b56af2ae8defcb16885d55e27b  internal/le/yang/glue/actions.go
  f048543cb3c6da4296a4240befd62732cfa04d8a72d8ee7cf8d6850442dab9c3  internal/le/yang/glue/register.go
  5fb023591e82930e68e69db0f03e4f6c54a756af2813b389c3b675bbc3558280  internal/le/yang/leafmentions/actions.go
  c0b6e2446fabe063ae51eac1c3c6221e085b94a2e7fe5db13dbaf5cf9a7dde40  internal/le/yang/leafmentions/register.go
  f48f19c1e5d287121e38feeeda16503bfe4f977ceddd97035ed3c737f2690853  internal/le/yang/migration/actions.go
  b263ea0d11aed3825168e219a9f29e49fd3c8fd68cb29c6c20ace03e83c5a253  internal/le/yang/migration/register.go
  55166d07afe3ce490b5fc00074d5753b255ec6f62318606f5b57e0a40207f35b  internal/test/fixture/misc_fixture_runner_scope.go
  e4815d2a25f5a4e55982a1a1db2610d0a6f599cd4d30e10a5b76166da05dd526  internal/test/fixture/register_le_namespace_dispatch.go
  8c546bb4060f60d1fdbaec60eedf92fef835f7c3f93976fa654bb2706eebcf17  internal/test/fixture/ui_fixture_le_namespace_dispatch.go
  5a3dc013a90884c1ced8357f9f22e3c4dc15f05dbad803de02b7b6c8ff474700  internal/test/fixture/ui_fixture_le_spec_status_answers.go
  DELETED  plan/spec-le-command-namespaces.md
  6cd2304425d699dd5e8fa74673516a3491a7065bc12dcbf4cfabdd819a3b834b  plan/spec-le-subject-first-command-tree.md
  8f8fb4ed887a5d803241c901069a1d9bd943239b9e414dc1af92c741e39706f6  plan/verification-debt/c392fa50.md
  e3f1e753766de3329af31ee476c38f8ceed9f7816a86bf9790aab4fc3be2ee50  test/runner/verify-scope-freshness-scoped.ci
  174384040408e1a3ede4201f46eda71676125766e6085ba08220acb12da27dcc  test/ui/le-namespace-dispatch.ci
  ac918c53e4393269cda095775a5467d6ea982f7b814ef6b18392e038bb8e3627  test/ui/le-spec-status-answers.ci

## Findings

**Overall correctness: correct. Confidence: 0.91. Review rounds: 1. Findings: none.**
Reviewed only `c39fb1198b2f4feef5f1afda3b3d7c6fb32acf1d`, using the exported historical tree, its patch, and separate parent context. This is a substantive supersession closure—not N/A and not completed successor implementation; I found no patch-introduced defect in the disposition of the old requirements.
**Evidence:** source and test inspection only. No checks, tests, builds, lint, formatters, edits, commits, or proof recording performed.

### All 17 old acceptance criteria
Paths below are relative to the postcommit export root; the deleted specification was read separately from the parent export.
| Old AC | Historical disposition and inspected producer |
|---|---|
| 1 | Retained: five `verify` members register separately and reach their handlers; parent worktree/current/list actions remain. Read `internal/le/verify/actions.go`, `internal/le/verify/deps/actions.go`, `internal/le/verify/lint/actions.go`, `internal/le/verify/lock/answer.go`, `internal/le/verify/status/answer.go`, `internal/le/verify/summary/answer.go`. |
| 2 | Retained: citation, session/current, and status route to their respective `Answer` implementations. Read `internal/le/spec/citation/actions.go`, `internal/le/spec/session/actions.go`, `internal/le/spec/status/answer.go`. |
| 3 | Retained: glue/check, migration, and leaf-mentions retain separate registrations and action tables. Read `internal/le/yang/glue/actions.go`, `internal/le/yang/migration/actions.go`, `internal/le/yang/leafmentions/actions.go`. |
| 4 | Retained: claims reaches `Audit`; coercion/check reaches its registered action. Read `internal/le/config/claims/configclaims.go`, `internal/le/config/coercion/actions.go`. |
| 5 | Retained: boundary/check reaches `CheckCheckout`; imports/check reaches `Check`. Read `internal/le/plugin/boundary/actions.go`, `internal/le/plugin/imports/actions.go`. |
| 6 | Retained: list reaches `Collect`; ownership reaches `Check`. Read `internal/le/command/list/commandlist.go`, `internal/le/command/ownership/commandownership.go`. |
| 7 | Retained: doc/check/verify dispatches to `docwiring.Verify`; bare doc/wiring dispatches its check action. Read `internal/le/doc/check/actions.go`, `internal/le/doc/wiring/docwiring.go`. |
| 8 | Retained: site/facts/check has its own registration; all six parent actions remain in the table. Read `internal/le/site/facts/actions.go`, `internal/le/site/actions.go`. |
| 9 | Retained: repository/tracked-build/check reaches `Run`; the four parent actions remain. Read `internal/le/repository/trackedbuild/actions.go`, `internal/le/repository/actions.go`. |
| 10 | Retained: `resolve` offers at most two words after `le`, stops before the pipe, and returns the unconsumed argv to the handler. Read `internal/le/leroot/dispatch.go` and `internal/component/command/registry/registry.go`; inspected bounded-prefix, longer-prefix, trailing-argument, and pipe tests in `internal/le/leroot/namespace_test.go`. |
| 11 | **Open, transferred to successor AC-31.** Historical `Dispatch` emits member names and exits 1; `TestBareNamespaceTokenListsItsMembers` explicitly requires 1. Descriptive listing/exit 0 remains successor work, not historical completion. |
| 12 | Retained: the named families occupy the predicted nested directories; `directoryFor`, `registeringDirectories`, and the bidirectional directory test enforce the two-level rule. Read `internal/le/group_test.go` and `internal/le/register_test.go`. |
| 13 | **Feeder retained; exception policy superseded by AC-19.** `cligrammar.Answer` supplies `leroot.Owned()` to `Check`, which consumes the R9 findings. Historical exceptions include both `test-*` and `go-*`; empty exemptions are a future requirement. Read `internal/le/cligrammar/actions.go`, `internal/le/cligrammar/cligrammar.go`, `internal/le/cligrammar/cligrammar_test.go`, `internal/component/command/grammar/checker.go`. |
| 14 | **Deliberately reversed by successor AC-18.** Historical `stageLogPath` flattens spaces and slashes to hyphens; existing filename pins remain. Read `internal/le/verify/engine/run.go`, `internal/le/verify/stagelogname_test.go`. No claim that successor filename changes already landed. |
| 15 | Retained: inspected family registrations declare shapes at their full paths. `ShapeForCommand` uses prefix lookup, while `TestEveryCommandRegistersItsOwnAnswerShape` checks declaration presence rather than relying on inherited resolution. Read `internal/le/leroot/leroot.go`, `internal/component/command/answer_shape.go`, `internal/le/group_test.go`. |
| 16 | Retained: parent action tables do not collide with their member names; `TestNoMemberShadowsItsNamespaceRootVerb` checks that invariant against registered metadata. Read `internal/le/group_test.go` and the parent action tables above. |
| 17 | Retained: the engine is under `internal/le/verify/engine`; the general runner remains under `internal/le/gaterun`. Read `internal/le/verify/engine/stages.go`, `internal/le/gaterun/gaterun.go`. |

### Supporting inspection and evidence limits
- Closure context: `plan/verification-debt/c392fa50.md`, especially line 12; `plan/spec-le-subject-first-command-tree.md`, especially status `ready`, supersession text, AC-18/19/31, and lines 725–738. Parent-only `plan/spec-le-command-namespaces.md` was read in full, including AC-1–17.
- Composition and consumers: `cmd/ze/ze_le_register.go`, `internal/le/register.go`, `internal/le/leaction/leaction.go`, `internal/le/verify/register.go`, `internal/le/verify/dispatch/dispatch.go`, `internal/le/verify/dispatch/dispatch_test.go`; registration declarations and full-path shape declarations in the nine families’ `register.go` files were inspected.
- Population/tests: `internal/le/completeness_test.go`, `internal/le/completeness_record_test.go`, `internal/le/register_test.go`, `internal/le/leroot/leroot_test.go`, `internal/le/verify/engine/verifyengine_test.go`, `internal/le/verify/engine/run_test.go`. Inspected tests are not claimed to have passed.
- Functional scope: `internal/test/fixture/register_le_namespace_dispatch.go`, `internal/test/fixture/ui_fixture_le_namespace_dispatch.go`, `test/ui/le-namespace-dispatch.ci`, `internal/test/fixture/ui_fixture_le_spec_status_answers.go`, `test/ui/le-spec-status-answers.ci`, `internal/test/fixture/misc_fixture_runner_scope.go`, `test/runner/verify-scope-freshness-scoped.ci`.
- **Do not overstate those fixtures:** namespace-dispatch exercises the parent action `verify list`, not a separately registered two-word member, and deliberately omits the bare-namespace exit assertion. The status fixtures pass `"spec status"`/`"verify status"` as single argv elements. These pre-existing coverage limitations do not constitute patch-introduced findings or prove successor acceptance.
- Historical documentation inspected: `docs/architecture/cli/command-namespacing.md`, `docs/architecture/cli/root-namespace-grammar.md`, `docs/architecture/core-design.md`, `docs/architecture/testing/verify-freshness-scope.md`, `ai/rules/cli.md`. The retained historical behavior is distinct from the successor’s deliberate reversal of `test-*`, `docvalid`, `docs-to-code`, and log-name decisions.
- **Authorization boundary:** committed owner-decision statements were reviewed as historical claims only. This result supplies no independent owner authorization or owner attestation, and clears no full-native-verification obligation.
