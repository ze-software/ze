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
| withXFRMNetlink | The doctor package's private XFRM seam is deleted with the probe it swapped. The kernel XFRM dataplane is now one enrolled capability (`internal/component/kernelcap`), whose answer is driven by `ze.test.kernelcap.xfrm`; `checkKernelModules` no longer asks about XFRM at all, so there is no seam left for this helper to hold. |
| TestKernelModulesBuiltInNotMissing | It asserted that `checkKernelModules` reports no missing XFRM module on a built-in kernel. `checkKernelModules` now says nothing about XFRM in any state, which is a stronger claim than the one this test made, and `TestKernelModulesLeavesXFRMToTheCapability` in the same file asserts it over three config shapes. The behaviour it guarded moved to `TestXFRMCapabilitySilentWhenNothingIsWrong` (`internal/component/ike/engine/doctor_xfrm_test.go`). |
| TestKernelModulesXFRMAbsentIsReported | It asserted the module check reports an absent XFRM dataplane as an error naming both modules. That verdict moved to the capability enrolment, where it is still an error and now also refuses the daemon start: `TestXFRMAbsentIsAStartupError` (`internal/component/ike/engine/doctor_xfrm_test.go`) asserts the code, the ERROR severity and that the message names the subsystem, `CONFIG_XFRM_USER` and `vpn ipsec`. Nothing is claimed less than before. |
| TestKernelModulesXFRMSilentWithoutIPsecConfig | It asserted the module check says nothing about XFRM for a config with no `vpn ipsec` container. `TestKernelModulesLeavesXFRMToTheCapability` replaces it with the wider claim that the module check says nothing about XFRM for ANY config, and `TestXFRMCapabilitySilentWhenNothingIsWrong` covers the scoped silence at the capability. |
| checks_linux_test | Three XFRM tests and their seam leave this file because the behaviour they asserted moved to the kernel capability enrolment. Three tests arrive in their place: `TestKernelModulesSilentForAnEmptyIPsecBlock`, `TestKernelModulesWarnsForAConfiguredIPsecPeer` and `TestKernelModulesLeavesXFRMToTheCapability`. The file's assertion count over `checkKernelModules` rises rather than falls. |
| TestCheckMPLSSupportProbesCapabilityNotModuleList | `checkMPLSSupport` is deleted: the MPLS kernel requirement is now an enrolled capability registered by the plugin that programs kernel labels. Its four subtests are replaced by four tests over the same fixtures and the same probe: `TestMPLSCapabilityReadsBuiltInKernel`, `TestMPLSCapabilityAbsentIsENOENT`, `TestMPLSCapabilityUnreadableIsUnknown` and `TestMPLSCapabilityPresentWhenLabelSpaceIsZero`. Absence and cannot-determine are now asserted on the errno rather than only on the diagnostic code, so the file discriminates more than it did. |
| TestCheckMPLSSupportWarnsEvenWhenModulesAreLoaded | Same deletion. `TestMPLSCapabilityIgnoresTheModuleList` replaces it and asserts both directions: a loaded module list does not make an absent table read present, and an unreadable module list changes neither answer. |
| TestCheckMPLSSupportSilentWhenNotApplicable | Same deletion. Its three subtests are covered by `TestMPLSCapabilityReadsBuiltInKernel` (the kernel holds the table), `TestMPLSInUseNamesRealFamilies` (a plain unicast peer is not MPLS in use) and its `another-fib-backend-is-never-gated` subtest, which is the AC-7 case and is also asserted from a second angle by `TestMPLSCapabilityNotGatedOnVPPBackend` (`internal/component/doctor/capability_test.go`). |
| TestCheckMPLSSupportIgnoresUnreadableModuleList | Same deletion. `TestMPLSCapabilityIgnoresTheModuleList` carries the unreadable-module-list case over both probe answers. |
| checks_mpls_linux_test | The four `checkMPLSSupport` tests leave with the function. Six tests arrive over the same fixtures: `TestMPLSCapabilityReadsBuiltInKernel`, `TestMPLSCapabilityAbsentIsENOENT`, `TestMPLSCapabilityUnreadableIsUnknown`, `TestMPLSCapabilityPresentWhenLabelSpaceIsZero`, `TestMPLSCapabilityIgnoresTheModuleList` and `TestMPLSInUseNamesRealFamilies`. `TestReadLoadedModulesDistinguishesEmptyFromUnreadable` is untouched. |
| withXFRMProbe | The ike engine's private XFRM seam is deleted with `probeXFRM`, which dumped the Security Policy Database and therefore could not tell a kernel without XFRM from a process without CAP_NET_ADMIN. `withXFRMState` replaces it in the same file and drives the shared probe through `ze.test.kernelcap.xfrm`, which reaches three answers where the old seam reached two. |
| TestXFRMUnavailableDiagnostic | It asserted `checkXFRMReachable` produces `doctor-ipsec-xfrm-unavailable` at WARNING severity. `checkXFRMReachable` is deleted because it reported the same fact as the capability gate, at a different severity, under the same code. `TestXFRMAbsentIsAStartupError` replaces it and asserts more: the same code at ERROR severity, and that the message names the subsystem, the kernel feature and the configuration that required it. |
| TestXFRMReachableSilentWhenNothingIsWrong | Same deletion. `TestXFRMCapabilitySilentWhenNothingIsWrong` replaces it with the same four negative cases plus a fifth the old test could not express: an EMPTY `vpn { ipsec { } }` block, which installs no Security Association and must gate nothing (AC-11). |
| TestXFRMReachableDoctorCheckRegistered | Same deletion. `TestIPsecCapabilityEnrolled` replaces it: it asserts the ike engine enrolled `ipsec` in `kernelcap.Enrolled()`, that the `kernel-capability-ipsec` doctor check exists with a non-nil function, and that BOTH its codes are declared where the old test checked one. |
| TestXFRMProbeHonorsTestOverride | Same deletion, and the override it drove is deleted with it. `TestXFRMOverrideDrivesEveryVerdict` (`internal/component/kernelcap/probe_linux_test.go`) replaces it and asserts all three answers plus the two non-answers: a misspelled value and an unset value must leave the real probe in charge rather than decide a start. |
