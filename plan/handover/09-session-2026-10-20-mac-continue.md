# Handover: Linux session d8e7dbbf to the Mac (2026-10-20)

The Linux session moves to the Mac. Everything it committed is on `origin/main`.
Detailed history of the lab-cleanup work: `plan/handover/06-lab-docker-resources-reaped.md`.

## Work for the Mac

`plan/spec-lab-docker-cleanup-on-macos.md` (status ready) exists for exactly this
machine. Its acceptance criteria need macOS, Colima, Docker Desktop and HVF:

| AC | What the Mac proves |
|----|---------------------|
| AC-1 | `kern.bootsessionuuid` and `IOPlatformUUID` readable without sudo; paste both values into the spec |
| AC-2 | A perf DUT's `ze.lab.*` labels carry the Mac's machine id, boot id, `darwin`, the live `le` pid, no pid-ns |
| AC-3 | After a reboot, `./le test integration cleanup dry` lists a killed run's resources as "would remove", and `cleanup` removes them |
| AC-4 | `TestKilledLabRunIsCleanedByNextStart` (`-tags docker`, `ZE_REQUIRE_DOCKER=1`) passes on Colima and on Docker Desktop |
| AC-5 | `./le test qemu docker-lab` with interop-ipsec under HVF: start sweep summary, labeled creates accepted by the guest dockerd |
| AC-6 | One Ctrl-C on `./le perf run dut ze` leaves no container, network or retained tag of that run |
| AC-7 | FreeRtr image build on both daemons carries `ze.lab.kind=image` and leaves no retained tag |
| AC-8 | One Ctrl-C on the `vpp-test` and `vpp-iface-test` deployment proofs: exit 1, no verdict line, nothing of the run left |

The exact commands and expected output are in the spec's AC table; run them as
written. The spec must be claimed on the Mac (`./le spec claim spec
spec-lab-docker-cleanup-on-macos.md`).

## Still open, not Mac work

| Item | Blocked on |
|------|-----------|
| `plan/spec-lab-docker-cleanup-and-go-test-containers.md` | A green full verify run, and the first nightly run carrying `docker-peers` / `live-rpki` (AC-25). Review gate is CLEAN at round 10 |
| `plan/spec-interop-image-copies-a-prebuilt-ze.md` AC-10 | `./le verify current mode full` passing |
| Verification debt of the specs this session closed (LDP keepalive expiry, BFD link-local, BGP prefix-SID, boot record after listen) | The full suites. The owner reserved them to another session; verify is STALE since the 2026-10-19T14:57Z failure |
| `plan/spec-lab-kernel-features-in-setup.md` (vhost-net and modprobes in the Linux setup) | Owner: bucket `plan/` or `plan/pre-release/`; Linux work |
| RFC 4761 Section 3.5 VPLS reclassification | Owner ruling |
| LDP spec AC-6 rewording (spec already closed) | Owner acceptance |
| 208 untested RFC rows | Absent features, other roles, or awaiting rulings; RFC scope is frozen |
| Round 10 NOTEs N2 and N6 (`setup docker-kernel install` stop wording and second Ctrl-C) | Rows in `plan/journal/stop-leaves-the-work-running.md`, not fixed |

## Done this session (for orientation)

- G-2: every lab verb stops its docker children on Ctrl-C/SIGTERM; 23 fix commits, review rounds 6 to 10, real Ctrl-C proofs recorded in handover 06.
- Specs closed: LDP keepalive expiry, BFD link-local peer zone, BGP prefix-SID, boot record after listen.
- RFC tests with both polarities and recorded mutant proofs: OSPF RFC 8362, RPKI 8210bis, LDP RFC 3031, EAP RFC 7627.
