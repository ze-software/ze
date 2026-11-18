//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- retained admission controls.
// Related: original_support_linux_test.go -- exact positive query42 controls.

package kernel

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// Pair operations cannot reach native files without positive receipts on both
// actual originals. A compiled support declaration supplies neither receipt.
func TestOriginalPairMissingReceiptReachesNoNativeOperation(t *testing.T) {
	pair := &claimedSessionPair{}
	if err := connectOriginalPairLocked(pair); err == nil {
		t.Fatal("unprobed pair connected")
	}
	if err := disconnectOriginalSessionPair(pair); err == nil {
		t.Fatal("unprobed pair disconnected")
	}
}

// Invalid bridge inputs cannot manufacture owner claims or native descriptors.
func TestOriginalBridgeRequiresOriginalOwners(t *testing.T) {
	if bridge, err := NewOriginalChannelBridge(nil, nil); err == nil {
		t.Fatalf("nil bridge admitted: %v", bridge)
	}
	original := &CreatedChannel{}
	if bridge, err := NewOriginalChannelBridge(original, original); err == nil {
		t.Fatalf("aliased bridge admitted: %v", bridge)
	}
	if bridge, err := NewOriginalChannelBridge(original, &CreatedChannel{}); err == nil {
		t.Fatalf("unminted bridge admitted: %v", bridge)
	}
	var bridge *OriginalChannelBridge
	if err := bridge.Bridge(); err == nil {
		t.Fatal("nil bridge operated")
	}
	if err := new(OriginalChannelBridge).Unbridge(); err == nil {
		t.Fatal("zero bridge operated")
	}
}

// A valid declaration cannot conceal a failure to identify the answering
// kernel. The provenance cause remains Unknown and never selects a rebuild.
func TestOriginalAttachmentDoctorUnknownKernelProvenance(t *testing.T) {
	want := errors.New("uname access refused")
	check := originalAttachmentDoctorCheck(func(uint32) originalAttachmentResult {
		return originalAttachmentResult{status: originalAttachmentSupported}
	}, func() (string, error) { return "", want },
		func(diagnostic.DoctorCheckContext) (uint32, error) { return pppSupportSession, nil })
	found := check.Check(diagnostic.DoctorCheckContext{})
	if len(found) != 1 {
		t.Fatalf("unknown provenance findings %v", found)
	}
	if !strings.Contains(found[0].Message, want.Error()) {
		t.Fatal("lost provenance refusal")
	}
	if strings.Contains(found[0].Message, "lacks") {
		t.Fatal("unknown provenance became absence")
	}
	if found[0].Fix != "ze show doctor" {
		t.Fatal(found[0].Fix)
	}
}

// A failed scope lookup must remain Unknown and must not probe an unrelated
// dependency using an invented configuration/default.
func TestOriginalSupportDoctorScopeUnknown(t *testing.T) {
	want := errors.New("backend schema unavailable")
	check := originalAttachmentDoctorCheck(func(uint32) originalAttachmentResult {
		t.Fatal("scope failure still queried a guessed role")
		return originalAttachmentResult{}
	}, originalAttachmentKernel, func(diagnostic.DoctorCheckContext) (uint32, error) { return 0, want })
	found := check.Check(diagnostic.DoctorCheckContext{})
	if len(found) != 1 {
		t.Fatal(found)
	}
	if !strings.Contains(found[0].Message, want.Error()) {
		t.Fatal("lost scope cause")
	}
	if found[0].Fix != "ze show doctor" {
		t.Fatal(found[0].Fix)
	}
}

// A later actual attachment refusal remains a refusal after declaration:
// ENOTTY is retained and neither MRU nor channel input enables unit reads.
func TestClientPairLaterENOTTYKeepsUnitUnreadable(t *testing.T) {
	client, _, _ := clientPairFixture(t)
	client.ops.connect = func(*claimedSessionPair) error { return originalAttachmentRefusal(unix.ENOTTY) }
	if err := client.Connect(); !errors.Is(err, unix.ENOTTY) {
		t.Fatalf("late ENOTTY lost: %v", err)
	}
	if err := client.SetReceiveMRU(1500); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 8)
	if _, err := client.ReadPacket(buffer); err != nil {
		t.Fatal(err)
	}
	if buffer[0] != 1 {
		t.Fatal("late attachment refusal admitted unit input")
	}
}
