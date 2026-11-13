# Configuration upload shutdown continuation

## Applied follow-up, source frozen

ROOT reported actual semantic RED f1e44291 after applying exact f04f1a9b
candidate and formatting364f9a92: both stopped-window operations accepted,
reloaded twice, and changed selected/active source. The useful no-pending and
actual Abort/reload controls passed. ROOT then explicitly released source.

Applied the source fix only after that RED. configMutationRuntime now has an
explicit bound *confirm.Window. bind requires that argument, and commit uses
the bound pointer rather than the public global. Main creates the empty window
before final bind. Both new fixture call sites pass their actual window or
explicit nil for the standalone SSH fixture, which has no confirmation worker.
No optional argument or compatibility shim was added. The stale
sessionReloadHolder comment in confirm_window.go is corrected.

Applied exact DNS fixture patch2ddc97ca only after independent APPROVE(a).
Retained trailer: Test-judge: cmd/ze/hub/config_upload_ssh_test.go: APPROVE(a),
exact 2ddc97ca patch isolates host resolver output while preserving all receiver
assertions and rejection inputs. The 96-byte disconnect declaration remains.

Corrected only the five newly added host-tool remediation strings to
./ze-host appliance ... in cmd_config_push.go/config_upload_ssh.go. Earlier
pre-existing help examples in cmd_config_push.go still spell ze appliance;
no older command sweep was included in this bounded task.

Docs updated in the same patch: remote-operations.md explains the bound window
and shutdown refusal/revert admission. hub-architecture.md now describes empty
window construction before final runtime binding and its retained exclusion.
Both carry source anchors. Existing accepted/pending/revert behavior remains.

Exact seven Go paths to format are config-upload-shutdown-format-files.txt.
No formatter, test, build, code execution, native metadata operation, commit or
child was run by this agent. Source freeze released to ROOT. ROOT next runs
only ^TestConfigUploadRetainsStoppedConfirmWindow$ after formatting. Then the
three new window parents, receiver/host transport selectors and affected hub
population can widen once the previously failing parent passes. Native
appliance proof and independent source review remain ROOT-owned and open.

The lower candidate instructions are historical and now superseded.

Author: config_upload_shutdown. ROOT owns spec deployment-user-journeys,
execution and source release. This is a tests-first private candidate handoff;
no tracked/new package Go was written, no tests/builds/formatters/code/native
metadata execution were run, and no commit or child was created.

## Diagnosis before edits

1. Symptom: a new Upload or source Commit can publish while shutdown has
   withdrawn the confirmed-commit window, although that stopped window left
   a rollback record that boot recovery will apply.
2. Producer: configMutationRuntime.commit currently passes
   daemonConfirmWindow.Load() into commitRuntimeConfig. stopConfirmWindow
   CompareAndSwap withdraws the window before Window.Stop joins its worker.
   The main shutdown path closes the mutation gate only after stop returns.
   confirm.WriteOutside(nil) calls apply directly. A stopped nonnil window's
   do instead refuses with ErrStopped. Stop intentionally retains the pending
   record. The interval after Stop returns but before gate.close is enough;
   no candidate-overwrite assertion or speculative scheduler race is needed.
3. Owning layer: the daemon mutation runtime's lifetime binding of its
   confirmation exclusion, not SSH framing, storage candidate compare, or
   the existing confirmed-window worker.
4. [workaround] close the existing shared reload gate before stopping the
   confirmation worker, or hide writes only in the SSH adapter. [source]
   bind a stable *confirm.Window in the immutable mutation runtime, so both
   Upload and source Commit retain exclusion after public withdrawal. Keep
   Reload admitted until the worker stops, as a revert needs that reload.
5. The workaround would refuse legitimate revert reloads or leave the web
   source Commit bypass intact. The stable owner is the shared boundary.

## Deciding startup facts

startConfirmWindow calls NewWindow/newWindow, which starts with pending=nil
and timer=nil and never loads a recorded pending commit. Boot
recoverConfirmWindow runs separately and synchronously before config load.
Therefore construct/publish the empty window before final owner.bind, passing
the owner's Reload method as its future reverter. The empty worker cannot
autonomously invoke Reload before binding. A request before bind remains
starting/refusing. Use an explicit required window argument on bind, never a
variadic compatibility shim or a new global-pointer lookup in commit.

## Private candidates ready, NOT applied

- config-upload-stop-window-regression.candidate.patch, SHA256
  f04f1a9b298d07d11ccb16c5e9d249a1225c76df3038cc10c8f8d26448cd263b.
  Adds cmd/ze/hub/config_upload_window_test.go. Selector:
  ^TestConfigUpload(RetainsStoppedConfirmWindow|WithoutPendingWindowApplies|WindowRevertCanReload)$.
  Actual owner/window/start/stop/candidate publication and durable recorder;
  memory store bound to stored config source, no daemon/network side effects.
  Stop returns as the deterministic worker-join barrier. Upload and shared
  source Commit after public withdrawal must refuse, leave accepted/active
  bytes and no candidate, and preserve alice's original rollback record.
  Separate useful upload with no pending window must publish, and actual
  Abort must use owner.Reload and restore through candidate promotion.
  These are controlled lifetime seams, not a replacement of actual SSH
  daemon application proof. Expected RED is writes bypassing the stopped
  window; the positive controls should pass.
- config-upload-dns-isolation.candidate.patch, SHA256
  2ddc97ca5be02f70713e4498f9ff471d1a7bb4b62c4b6584b558d2c7df39f0fd.
  Adds exactly set system dns resolv-conf-path "" to sshUploadBefore/After
  and removes the two stale 32-byte prefix comment claims. Keeps the 96-byte
  declaration, payload, length-derived four-MiB bounds and all assertions.
  Sent to retained independent reviewer for exact candidate judgment before
  any fixture edit; only the direction had previously been approved.

## ROOT next steps

1. Let full six-package job48007 finish before releasing source edits.
2. Apply only the new window regression candidate, format from ROOT, and
   observe its actual semantic RED with the three-parent selector above.
   A build/setup failure is not that evidence.
3. After RED, resume this agent for the explicit stable-window binding,
   main startup reorder and affected new fixture bind calls, exact approved
   DNS isolation patch, and new invalid host-driver fix commands. Runtime
   messages in cmd_config_push.go and config_upload_ssh.go must name
   ./ze-host appliance ... rather than ze appliance ... . No older command
   sweep is authorized by this bounded task.
4. Update docs/architecture/appliance/remote-operations.md, the actual shared
   mutation/window lifetime page, and stale confirm_window.go comment naming
   removed sessionReloadHolder. Preserve pending record semantics/revert
   admission and all existing historical tests.
5. ROOT reruns only the failing new window regression first, then widens to
   affected packages/real SSH and native appliance proof. Retained independent
   review remains required; this author has made no GREEN or closure claim.
