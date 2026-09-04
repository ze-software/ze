# Test weakenings this commit accepts

**This file is REPLACED for each commit. It never accumulates.** Delete the rows
of the last commit, write the rows of this one. The commit gate refuses a row
naming a test the prospective commit does not weaken, so a row left behind by an
earlier commit blocks the next author rather than helping anybody. Git history
holds every past entry: `git log -p -- test/weakened.md` shows the rows of any
commit beside the change they justified.

**Several sessions share this checkout, and this is one shared path.** Write your
rows immediately before you run `./le commit create`, then read the file again
between writing them and running the script. Rows written earlier are a window
for another session to replace them, and a session that writes with `cat >`
rather than an edit replaces the file whole. The refusal is the safe outcome. The
unsafe one is silent: your commit lands carrying another session's justification,
and no gate sees it, because the file is present and the row count is plausible.
Say so on the message bus before you take the slot.

A row here is the AUTHOR's own justification. The owner's approval for changing a
test that carries an `RFC requirement:` tag is a different file,
`test/rfc-changed.md`, and a row here does not authorize one there.

`parseLedger` (`internal/le/testweakened/ledger.go`) reads the first
`| Test | Reason |` table it finds and every table row under it, so this prose is
safe above the table. Do not write a second such header anywhere in the file: the
parser refuses two tables rather than guess which one the gate should read.

**A test in a NEW file needs no row in either ledger, and the two gates disagree
about that.** The commit gate reads the file at HEAD (`committedText` in
`internal/le/commit/rfcchange.go`) and skips a path with no HEAD version, so it
computes no change for a new file and REFUSES a row naming a test in one. The
write hook reads the file on disk instead (`Proposed` in
`internal/le/testweakened/proposed.go`, where `taggedCarrier` tests the current
`oldText`), so it DEMANDS a row before it lets you edit a new file that already
carries `RFC requirement:` tags. An author who obeys the hook is then refused by
the gate. Until one of them changes, write the tags in the same edit that creates
the file, and carry no row for it.

| Test | Reason |
|------|--------|
| TestSSHAuthenticatorFallsBackToLocalWhenTheBundleIsAbsent | Deleted. They tested `liveAAABundleAuthenticator{fallback: ...}`, a construct this commit DELETES from production. It was an ssh-level workaround for a defect whose real cause this commit fixes: aaa.Build aborted on the first backend error, so a keyless TACACS+ server took the LOCAL backend down with it. With the chain composing correctly, ssh needs no fallback of its own and the field is gone, so a test of it would pin a construct no caller builds. The claim they carried is now proven at the layer that owns it, in internal/component/aaa/chain_survives_a_failed_backend_test.go: six cases including the four the owner stated on 2026-09-04 (remote answers, remote rejects and the chain stops, remote unreachable and local answers, unreachable plus wrong local password and nobody logs in). Net coverage RISES. |
| TestSSHAuthenticatorPrefersTheBundleOverTheLocalFallback | Deleted, same reason and same replacement as the row above. It asserted the bundle wins over the fallback; with no fallback there is nothing for the bundle to win over, and the chain-order claim it stood for is now `TestTheLocalBackendAnswersOnlyAfterTheRemoteOneFails`. |
| TestSSHAuthenticatorRejectsAnUnknownUserWithNoBundle | Deleted, same reason. Its claim, that the fallback grants nothing to a user no account names, is now the fourth subtest of `TestTheLocalBackendAnswersOnlyAfterTheRemoteOneFails`: an unreachable server plus a wrong local password logs nobody in. |
| TestInfraSetupGivesSSHAnAuthenticatorThatFailsOver | Deleted. It asserted infraSetup hands ssh an authenticator carrying that fallback, which is exactly the wiring this commit removes. `TestSSHIsStartedWhenTheChainComposed` replaces it and asserts the authenticator ssh receives reads the installed chain. |
| TestInfraSetupStartsSSHWhenTheBundleIsNil | REPLACED by `TestSSHIsNotStartedWhenNothingComposed`, which asserts the OPPOSITE and is the owner's rule of 2026-09-04: no user, no login. A nil bundle now means every backend failed, so nothing can authenticate and a listener would be a port rather than a service. The old test was written while ssh started regardless, which was the workaround. |
| sshLocalUser | Deleted helper, not a test. It built a bcrypt config user for the fallback tests above; with those gone it has no caller. The web suite keeps its own copy behind //go:build ze_web, which is where it came from. |
| ssh_local_failover_test | File-level count. The file is rewritten around the fixed design and is SHORTER, because three tests of a deleted construct left and two tests of the real wiring arrived. The claims did not leave the suite: they moved to internal/component/aaa, where six cases now cover what three did. |
