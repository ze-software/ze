# Historical VRRP closure recording script

Archived at Thomas's request on 2026-10-05. The original script is preserved
verbatim below as historical evidence, not as a runnable repository tool.
Its paths and commands describe the original recording attempt.

```bash
#!/usr/bin/env bash
# vrrp closure author: discrimination records for every tag added 2026-09-30.
cd /home/thomas/Code/github.com/ze-software/ze/main || exit 1
S=tmp/session/2026-09-28-869df689-cc8f-4d78-9161-1d7c87434c8e/scratch
OUT=$S/vrrp-closure-records.txt
: > "$OUT"

rec() { # id polarity unitfile func producer
  local stem log rc
  stem=$(echo "$1" | sed -E 's/^RFC([0-9]+)-.*/rfc\1/')
  log="$S/vrrp-closure-rec-$1-$2-$4.log"
  if [ -n "$KERNEL" ]; then
    flock "$S/children/ledger-$stem.lock" ./le rfc discriminate-record id "$1" polarity "$2" unit "$3::$4" route revert producer "$5" kernel "$KERNEL" > "$log" 2>&1
  else
    flock "$S/children/ledger-$stem.lock" ./le rfc discriminate-record id "$1" polarity "$2" unit "$3::$4" route revert producer "$5" > "$log" 2>&1
  fi
  rc=$?
  echo "$1 $2 $4 rc=$rc observed=$(grep -ci 'observed' "$log")" >> "$OUT"
}

V=internal/plugins/vrrp
P=$V/packet/rfc9568_ipv6_pseudo_header_test.go
if [ -z "$SKIP_HOST" ]; then
  rec RFC9568-5.2.8-3 positive $P TestDecodeV3IPv6ChecksumCoversPseudoHeader $V/packet/checksum.go::pseudoSumV6
  rec RFC9568-5.2.8-3 negative $P TestDecodeV3IPv6ChecksumCoversPseudoHeader $V/packet/checksum.go::verifyReceived
fi
KERNEL=tmp/kernel/build/vmlinuz

A=$V/vmac_state_integration_linux_test.go
for id in RFC9568-8.1.2-6 RFC5798-8.1.2-6 RFC3768-8.2-4; do
  for pol in positive negative; do
    rec $id $pol $A TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly $V/dataplane_linux.go::applyDataplaneSysctls
  done
done

N=$V/nd_nonowner_integration_linux_test.go
for id in RFC9568-8.2.2-8 RFC5798-8.2.2-8; do
  for pol in positive negative; do
    rec $id $pol $N TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly $V/dataplane_linux.go::applyDataplaneSysctls
  done
done

O=$V/owner_answer_integration_linux_test.go
for id in RFC9568-8.1.2-6 RFC5798-8.1.2-6 RFC3768-8.2-4; do
  rec $id positive $O TestVRRPOwnerAnswersWithVirtualMACOnly $V/ownerfilter.go::ownerARPReplyTerm
  rec $id negative $O TestVRRPOwnerAnswersWithVirtualMACOnly $V/ownerfilter.go::ownerFilterTables
done
for id in RFC9568-8.2.2-8 RFC5798-8.2.2-8; do
  rec $id positive $O TestVRRPOwnerAnswersWithVirtualMACOnly $V/ownerfilter.go::ownerNDAdvertTerm
  rec $id negative $O TestVRRPOwnerAnswersWithVirtualMACOnly $V/ownerfilter.go::ownerFilterTables
done

R=$V/redirect_integration_linux_test.go
rec RFC5798-8.1.1-2 positive $R TestVRRPRedirectSourceFollowsVirtualMAC $V/dataplane_linux.go::applyDataplaneSysctls
rec RFC5798-8.1.1-2 negative $R TestVRRPRedirectSourceFollowsVirtualMAC $V/dataplane_linux.go::applyDataplaneSysctls
echo done >> "$OUT"
```
