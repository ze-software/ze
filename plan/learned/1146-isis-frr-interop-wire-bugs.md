# 1146 - IS-IS FRR interop caught two self-consistent ze wire bugs

## Context
The spec-isis closure marked the AC-1..AC-10 interop acceptance criteria "pending
Linux execution." Implementing the QEMU FRR interop test (`TestISISInteropFRR`,
ze <-> FRR isisd over a veth, run via `make ze-qemu-isis-frr-test`) immediately
exposed two ze wire bugs that every existing ze <-> ze test had passed.

## The two bugs

1. **IIH padded past the link MTU, so the kernel dropped every Hello.**
   `circuit.padHello` padded the IS-IS PDU to the full interface MTU (1500). The
   transport then prepends a 3-byte LLC header, so the 802.3 frame exceeded 1500
   and `Sendto` failed with EMSGSIZE. ze therefore sent NO IIH at all -- but the
   smaller LSP/CSNP frames still flowed, so FRR received ze's LSPs yet never formed
   an adjacency (the misleading symptom: "FRR sees my LSPs but no neighbour").
   Fix: pad to `MTU - LLCHeaderLen` (`internal/component/isis/circuit/runtime.go`).

2. **LAN IIH fixed header was 28 octets, not the spec's 27.**
   The `LANHello` encoder emitted a spurious reserved octet between the Priority
   field and the LAN ID. Per ISO/IEC 10589 clause 9.5 the reserved bit IS the high
   bit of the Priority octet; there is no separate reserved octet. FRR rejected
   every IIH ("Expected fixed header length = 27 but got 28"). The DECODER read the
   phantom byte too, so ze <-> ze round-tripped happily. Fix: remove the byte from
   encode and decode, correct `lanHelloFixedLen`
   (`internal/component/isis/packet/hello.go`); regenerate the pinned wire fixture
   (`test/isis-wire/isis-pdu-1.ci`) and the wire doc.

## Lesson
Same-implementation testing cannot catch a wire/protocol bug that the encoder and
the decoder share. A self-consistent error (an extra octet, a wrong padding target)
round-trips perfectly ze <-> ze and only fails against a different implementation.
Cross-implementation interop (FRR isisd in QEMU, the `ze-qemu-<proto>-frr-test`
pattern, mirroring `ze-qemu-ldp-frr-test`) is REQUIRED to validate protocol
correctness, not optional. The interop test is cheap to run on darwin via QEMU.

## Bonus: a latent .ci runner race
The same QEMU work surfaced an unrelated race in the orchestrated `.ci` runner:
foreground quick-exit `ze` commands (`ze config validate`, `ze show`, ...) were
started-not-awaited (the daemon path) and shared one `clientStdout` buffer, so two
sequential `ze config validate` steps in one test raced and an earlier step's output
was lost (`isis-config`, `isis-redist-arbitration` failed on Linux too, not a
platform issue). Fixed by awaiting non-daemon foreground `ze` with isolated output
buffers (`internal/test/runner/runner_exec_util.go` `isQuickExitZeCommand`). Marking
the tests "linux only" would have masked a real, platform-independent bug.

## Files

None recorded.
