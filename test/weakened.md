# Tests this commit weakens

Each row accepts one test weakening in the commit that carries this file. A
weakening removes an assertion, a case, or a test. `ai/rules/testing.md` forbids
every weakening. This file is the record of the ones a reviewer accepted.

Two gates read it. `c_test_weakening` (`.claude/hooks/pretool-writeedit.py`)
refuses the edit until this file names the test the edit weakens.
`scripts/dev/commit_helper.py` recomputes the weakenings of the paths the commit
names, and refuses a commit whose weakenings this file does not cover.

Both gates call one implementation, `scripts/dev/check_weakened_tests.py`, so
neither can disagree with the other about what a diff weakens or which row
covers it. `make ze-test-weakened-check` runs that checker over this file alone. It is
a stage of `ze-precommit-verify` in both modes, so a table this parser cannot read goes red
before any commit needs it.

A commit that weakens nothing carries the table with no rows.

## The commit carries this file

`scripts/dev/commit_helper.py` refuses a commit that weakens a test and does not
name `test/weakened.md` in its own `--file` list. A row that stays in the working
tree records nothing. The reason must sit in git history beside the weakening it
accepts, because that is the only place a later reader can find it.

## The file is replaced per commit

Delete the rows of the last commit. Write the rows of this one. This file never
accumulates.

The reason is the mechanism it replaces. A `test-relax:` comment stayed in the
test file forever, and a ceiling file capped their number. That corpus reached
601 tokens and 2,660 lines of prose across 413 test files. Nobody can read 601
justifications, so nobody read them, so writing one cost nothing.

A justification explains one diff. It is written at edit time and it is read at
review time. After the commit lands, it explains a change the reader of the test
file can no longer see. Storing that record permanently is what built the pile.

Git history holds every past entry. `git log -p -- test/weakened.md` shows the
rows of any commit beside the change they accepted.

## The test name

| Carrier | The name |
|---------|----------|
| Go, inside a top-level func | the enclosing `func TestXxx` |
| Go, outside every top-level func | the file stem |
| `.ci`, `.et` | the file stem, because each such file is one test |

`scripts/dev/rfc_tagged_scope.py` resolves each one. `go_func_units` returns the
top-level functions of a Go file with their names. `scope_reader` treats a file
that is not Go as one unit.

Row two is the case with no enclosing func to name. An `ignore` build tag in the
file header drops the whole file from the build, and a count can fall over
package-level code. The stem is then the only name available, so a weakening in
`a_test.go` is written as `a_test`.

A bare name is accepted when it resolves to exactly one weakened test in the
commit. Write `package.TestName` when it does not.
`TestNoGoFileBuildsMarkup` exists in `internal/component/lg` and in
`internal/component/web`, so a commit that weakens both writes
`lg.TestNoGoFileBuildsMarkup` and `web.TestNoGoFileBuildsMarkup`.

## The reason

Name what left the suite, and say why the commit is correct without it. A reason
that names no lost coverage tells the reviewer that the detector fired on a
change which removed none.

Every weakening kind needs a row at commit time, a falling count included. The
hook is narrower on purpose. An edit that only lowers a count gets a notice and
lands. Consolidating three cases into one table lowers a count exactly as
deleting a check does. So the commit gate asks for a row the hook did not, and
that row is where you say which of the two happened.

| Test | Reason |
|------|--------|
| TestReloadHashesPlaintextPassword | The test moved unchanged to `main_reload_auth_test.go`. It keeps the disk loader and bcrypt assertions. |
| TestLiveLocalAuthorizerFollowsStoreSwap | Replaced by `TestLiveLocalAuthorizerFollowsIdentityPublication` through the same bundle authorizer path. |
| TestCloseAAABundleClearsLiveLocalAuthorization | Replaced by `TestCloseAAABundleClearsAcceptedLocalIdentity`, which covers the complete accepted state. |
| TestBuildUserAuthenticatorRecordsRecoveryProfiles | Replaced by `TestBuildAPIAuthenticationBindsRecoveryProfiles` and `TestAPIRequestCarriesAuthenticatedAuthorizationGeneration`. |
| TestAPILoginAdmitsPowerAndConfigUsers | The same six cases remain. One compound assertion now checks the username and authentication result. |
| UpdateAuth | Removed because REST and gRPC now read the accepted `api.Authentication` provider for each request. |
| TestReloadListenersRebuildsAuthenticationOn | Replaced by the REST and gRPC `AuthenticationProviderPublishesModesAtomically` tests and `TestRunReloadPublishesAcceptedIdentityAtomically`. |
| TestReloadListenersRebuildsAuthenticationOff | Replaced by the transport provider-mode tests and `TestRunReloadEmptyUsersSelectNoAPIAuthentication`. |
| TestReloadListenersRestoresAuthenticationWhenMigrationFails | Replaced by `TestRunReloadCertificateFailurePreservesAcceptedAPIToken` and the listener rollback tests. |
| TestReloadListenersRestoresEarlierServiceWhenLaterResolveFails | Replaced by `TestReloadListenersFailsClosedWhenAuthCannotBeResolved`. All modes resolve before migration. |
| TestReloadListenersRestoresEarlierServiceWhenLaterRebuildFails | Removed because the per-server credential rebuild no longer exists. Provider publication is atomic. |
| TestReloadListenersUndoRevertsInstalledCredentials | Replaced by `TestRunReloadCertificateFailurePreservesAcceptedAPIToken`. |
| TestReloadListenersLeavesAuthAloneWhenConfigIsSilent | Replaced by `TestAPIAuthReloaderAbsentBlockKeepsRunningMode` and `TestRunReloadWithoutAPIBlockPreservesAcceptedAPIToken`. |
| TestReloadListenersFailsClosedWhenServerRefusesRebuild | Removed with transport `UpdateAuth`. Provider publication and exposure validation are separate. |
| TestApplyAuthIntentsInstallsAndRestoresCredentials | Removed with `applyAuthIntents`. Transport-provider and hub identity tests cover publication and restoration. |
| listener_migrate_test | The count fell with obsolete credential-mutation fixtures. Listener resolution, exposure, migration, and undo assertions remain. |
| withReloadAuthorization | Replaced by `reloadIdentitySystem` and `withReloadIdentity`, which combine credentials and policy. |
| TestRunReloadSuccessfulReloadSwapsLiveAuthorization | Replaced by `TestRunReloadPublishesAcceptedIdentityAtomically`. |
| TestRunReloadFailedReloadPreservesLiveAuthorization | Replaced by `TestRunReloadFailurePreservesAcceptedIdentity`. |
| TestRunReloadUndoesCredentialsWhenCertificateRotationFails | Replaced by `TestRunReloadCertificateFailurePreservesAcceptedAPIToken`. |
| TestRunReloadKeepsCredentialsWhenReloadSucceeds | Replaced by `TestRunReloadPublishesAcceptedIdentityAtomically`, including new-password success and old-password denial. |
| reloadUserPassword | Moved unchanged to `main_reload_auth_test.go` beside its caller. |
| main_reload_test | The file count fell because the plaintext reload test moved to `main_reload_auth_test.go`. |
| TestAPIUserAuthenticatorFollowsTheRunningConfig | Replaced by `TestAPIAcceptedAuthenticationFollowsIdentityPublication`. |
| TestAPIUserAuthenticatorWithoutLiveSourceUsesItsList | Replaced by `TestBuildAPIAuthenticationUsesImmutableUserList` with the same bcrypt fixture. |
| TestAPIAuthReloaderResolvesConfiguredToken | Replaced by `TestRunReloadPublishesExactCandidateAPIToken`. |
| TestAPIAuthReloaderSilentWithoutBlock | Replaced by `TestAPIAuthReloaderAbsentBlockKeepsRunningMode`. |
| apiReloadUser | Replaced by the shared bcrypt fixtures and `reloadIdentitySystem`. |
| apiReloadSystemRoot | Replaced by `reloadIdentitySystem`, including its authorization policy. |
| TestAPIAuthReloaderUsesLiveUsersWithoutSSHBlock | Replaced by `TestRunReloadPublishesAcceptedIdentityAtomically` and `TestAPIAcceptedAuthenticationFollowsIdentityPublication`. |
| TestAPIAuthReloaderFailsClosedWhenLiveUsersUnreadable | Replaced by `TestRunReloadUserSourceErrorPreservesAcceptedAPICredentials`. |
| TestAPIAuthReloaderProceedsWithTokenAndNoUsers | Replaced by `TestRunReloadPublishesExactCandidateAPIToken`, `TestRunReloadEmptyUsersSelectNoAPIAuthentication`, and `TestRunReloadConfiguredUsersTakePrecedenceOverSharedToken`. |
| TestReloadListenersProceedsWhenSiblingTransportWasNeverBuilt | Replaced by `TestUnbuiltSurfaceResolvesNoAuthIntent` and `TestReloadListenersIgnoresUnclassifiedService`. |
| TestGRPCBuildAuthenticatesConfigUserWithoutSSH | The same test now uses `runYANGConfig`. Real TLS RPC, wrong-password, construction, and shutdown checks remain. |
| TestProfileRecordingAuthenticatorRecordsOnSuccess | Replaced by `TestProfileAuthorizerBindsRemoteProfiles`. |
| TestProfileRecordingAuthenticatorIgnoresFailure | Replaced by `TestProfileAuthorizerIgnoresFailure`. |
| TestRecordLoginProfilesEmptyDoesNotErase | Replaced by request-scoped `TestStoreAuthorizerBoundProfilesDoNotCrossSessions`. The global map is gone. |
| TestRecordLoginProfilesCopies | Replaced by `TestProfileAuthorizerCopiesRemoteProfiles`. |
| TestBuildWrapsAuthenticatorWithProfileRecording | Replaced by `TestBuildWrapsAuthenticatorWithProfileAuthorizer`. |
| TestProfileRecordingAuthenticatorRejectsReservedUsername | Replaced by `TestProfileAuthorizerRejectsReservedUsername` with the same reserved-name matrix. |
| TestGRPCUpdateAuthTurnsAuthenticationOn | Replaced by `TestGRPCAuthenticationProviderPublishesModesAtomically` through real RPCs. |
| TestGRPCUpdateAuthTurnsAuthenticationOff | Replaced by the no-auth publication case in `TestGRPCAuthenticationProviderPublishesModesAtomically`. |
| TestGRPCUpdateAuthRestoreRevertsCredentials | Replaced by `TestRunReloadCertificateFailurePreservesAcceptedAPIToken` and `TestGRPCRejectedCandidateNeverAuthenticates`. |
| TestGRPCUpdateAuthRefusesToUnauthenticateNonLoopback | Replaced by `TestGRPCReconfigureAppliesPublishedExposureModeAndTLS` and the hub exposure test. |
| TestGRPCReconfigureRefusesNonLoopbackWithoutTLS | Replaced by `TestGRPCReconfigureAppliesPublishedExposureModeAndTLS`. |
| TestGRPCUpdateAuthUndoRefusesToExposeMigratedListener | Replaced by `TestRunReloadListenerRollbackFailureStaysFailClosed` and gRPC staging rejection. |
| TestGRPCUpdateAuthUndoRestoresWhenLoopback | Replaced by `TestRunReloadCertificateFailurePreservesAcceptedAPIToken` and `TestGRPCRejectedCandidateNeverAuthenticates`. |
| TestGRPCUpdateAuthRefusedAfterStop | Removed with `UpdateAuth`. A stopped server serves no requests. |
| auth_reload_test | The count fell with `UpdateAuth`. Real-RPC publication, candidate rejection, and reconfiguration checks remain. |
| TestRESTUpdateAuthTurnsAuthenticationOn | Replaced by `TestRESTAuthenticationProviderPublishesModesAtomically`. |
| TestRESTUpdateAuthTurnsAuthenticationOff | Replaced by the no-auth publication case in `TestRESTAuthenticationProviderPublishesModesAtomically`. |
| TestRESTUpdateAuthRestoreRevertsCredentials | Replaced by `TestRunReloadCertificateFailurePreservesAcceptedAPIToken` and `TestRESTRejectedCandidateNeverAuthenticates`. |
| TestRESTUpdateAuthInstallsPerUserAuthenticator | Replaced by `TestRunReloadConfiguredUsersTakePrecedenceOverSharedToken` and `TestRESTAuthenticator`. |
| TestRESTUpdateAuthRefusedAfterShutdown | Removed with `UpdateAuth`. A stopped REST server serves no requests. |
| auth_test | The count fell with `UpdateAuth`. Live provider publication and rejected-candidate request checks remain. |
| TestStoreAuthorizeConfigAssignmentWinsOverLogin | Replaced by `TestStoreAuthorizeLoginBindingWinsOverConfigAssignment` for result-scoped sessions. |
| TestStoreAuthorizeLoginProfilesDoNotLeakAcrossUsers | Replaced by `TestStoreAuthorizeProfilesDoNotLeakAcrossUsers` and `TestStoreAuthorizerBoundProfilesDoNotCrossSessions`. |
| authz_test | Two extractor assertions moved to the no-SSH ownership test; the parsed-tree and malformed-shape coverage remains. |
