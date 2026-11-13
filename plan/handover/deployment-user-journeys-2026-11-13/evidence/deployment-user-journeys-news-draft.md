---
spec: deployment-user-journeys
topic: Installer
---

## Before

Installing Ze on a Linux distribution required separate binary, state and
service steps. Deployment instructions left gaps between choosing an
installation method and reaching an authenticated, persistent installation.
Local purge could remove package-owned shared credentials when the selected
standalone executable was absent or installed under another prefix.

Lab nodes could start Ze before their local addresses existed. Failed runs
retained too little routing state for diagnosis. OSPF could omit reachable
loopback addresses after Linux's default host127 address, while an IS-IS L2
instance could negotiate L1 on an interface using the default level.
Connected-route startup could miss addresses already installed on the node,
and a redistributed route could carry an internal gateway as its BGP next hop
to a peer for which that gateway was unusable.

## After

`ze install` installs the running executable and initializes absent state.
On systemd hosts, a fresh installation also starts the service. Repeat
installation retains configuration and credentials. Ordinary `ze uninstall`
removes the service and executable while retaining state and the service
account. Explicit `--purge` removes standalone configuration after checking
shared package ownership. Package-owned configuration and credentials remain
under the package manager's control, including when the standalone executable
is absent.

Native amd64/KVM checks passed ISO installation onto the selected disk,
preservation of an extra disk, installed-disk boot and authenticated SSH access.
Appliance update and previous-slot recovery checks verified live service health
and retained configuration and login after reboot.
A custom GRUB builder must now resolve to an executable file before the ISO
prerequisite check reports it ready, using the same selection as the build.

Container examples use a named volume for persistent configuration. A completed
Compose recreation check retained a configuration change and login using an
earlier runtime image without captured source identity. Earlier private preview
checks covered Linux amd64 and arm64 binary archives, amd64 deployment and lab
container exports, source identity and checksums. Public release downloads
remain unavailable. Preview archive checking now refuses sparse outer Docker
entries and entries beyond the size bound before hashing; sparse files inside
filesystem layers remain supported.

The installation guide now distinguishes the deployment paths and gives a
success check for each. Lab startup waits for initial interface configuration,
and failed runs retain private diagnostic state before cleanup.

Live appliance configuration upload now uses an authenticated SSH subsystem
with the operator's SSH agent and pinned host keys. The daemon authorizes the
accepted identity before reading the body, validates the complete replacement,
and commits through its existing source-aware transaction. Acceptance requires
the exact receipt and a successful SSH exit status. If delivery ends without
that receipt, the client reports an uncertain outcome and names the command
that inspects the live configuration. Upload starts no automatic health-window
rollback. The separate boot inbox writes only the legacy active entry.

Named appliance bootstrap keys now enter the local recovery identity through
`meta/auth/local/authorized-keys`. Each key must be a complete plain OpenSSH
public-key line. Malformed lines, certificates and `authorized_keys` options
are refused. Outbound SSH client records remain separate. The same credential
reader supplies `doctor-local-admin-credentials`, with a backup-restore command
for a refused recovery account. The native appliance configuration-upload
action is implemented. Its new VM proof remains pending.

The netlab configurations activate interface monitoring without declaring
managed addresses. The connected plugin subscribes before requesting the
active interface provider's retained inventory, including after a restart.
Both internal and external connected plugins use that event path. Each BGP
peer's redistribution attachment receives state events, so later establishment
can replay the current connected prefixes.

Redistributed gateways now remain a default candidate until the destination
BGP session chooses its next hop. External peers use Ze's session address;
internal peers retain the gateway unless it names that peer or is IPv6
link-local. Connected prefixes without a gateway also use the session address.
Configured peer policies and negotiated wire-format checks still apply.

The supplied two-peer BGP lab explicitly disables resolver file output. A fresh
run established both sessions and checked each opposite prefix through
authenticated SSH. It also rejected incorrect credentials and stopped both
daemons cleanly. Removing the explicit resolver setting made the fixture refuse
the configuration before either daemon started.

All 80 quickstart stress invocations passed. After promotion, the tutorial and
the current native installation lifecycle also passed together. The broader UI
retry passed all three deployment cases: remote trust, quickstart and native
installation lifecycle. It reported 298/323 passes, 18 failures, seven timeouts
and one platform skip overall. Missing stripped binaries, setup-dependent
checks and separate feature failures limit that result; the suite was not green.
Known failures remain recorded rather than triggering another unchanged full run.

The fixture environment lookup now applies the canonical alias rules. Both
affected SSH cases passed with their existing command, completion, authentication
and path-boundary assertions. Committed distribution, appliance, setup, host and
installer builds passed after the quickstart, IS-IS and earlier alias fixture
changes.

Two other UI fixtures now follow their current source contracts: the listener
case explicitly confirms its existing discard action, and inventory uses an
authored committed source fixture. Both retained their original assertions;
inventory also requires every authored plugin name. Both drafts passed, all 80
paired stress invocations passed, and both promoted cases passed. The committed
build-flavor check subsequently passed all five flavors against the repair's
actual committed source.

OSPF includes secondary IPv4 prefixes in stub reachability and advertises
explicit loopback addresses as zero-cost /32 hosts. An inventory-read failure
preserves the area's previous Router and Network LSAs. IS-IS constrains
default interface levels to the instance and reports incompatible settings,
including partial configuration without a NET. Its runtime transactions restore
the previous configuration on rollback, and circuit cleanup and Hello sends stay
bound to the circuit instance that owns them. Controlled regressions and full
race-package checks cover these corrections.

The real daemon rollback proof observes both candidate IGP runtimes before a
later participant rejects the reload. Disabling either rollback callback made
the corresponding restored-state assertion fail; restoring it passed. The first
stress attempt exposed a missing PID/readiness handoff in the fixture. After an
independently judged correction that preserved its assertions and budgets, the
fresh baseline, all 80 stress invocations and the promoted test passed on Ze's
kernel. The promoted run executed one test with zero skips and completed VM
cleanup. All five committed build flavors passed after this proof landed.

The live IS-IS counterfactual also failed as intended: FRR negotiated an Up
dual-level adjacency, which the required L2 assertion rejected. Source was
restored and the VM stopped without leftovers. Both restored IGP FRR cases
subsequently passed their exact adjacency, installed-route and sourced-ping
checks. The three updated netlab renders validated and the focused bootstrap
checks passed. The tool image reported Python 3.14.10, netlab 26.10, Ansible Core
2.22.0 and containerlab 0.83.0, with `pip check` passing.

A later complete netlab attempt failed BGP traffic and prompted the connected,
interface-startup and default-next-hop repairs described above. The restored
BGP FRR control and unchanged six-ping netlab rerun are still awaiting their
runtime verdict. The final source-bound bundle and site checks also remain
pending. No overall routing or deployment completion is claimed.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
