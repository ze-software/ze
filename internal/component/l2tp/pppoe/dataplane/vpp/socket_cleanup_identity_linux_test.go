// Design: docs/architecture/l2tp/bng-5-pppoe.md -- original cleanup coordinates
// Related: socket_linux.go -- original canceled constructor; diagnostic event declaration

//go:build linux

package vpp

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// Full original coordinates remain available when valid names cannot fit
// beside the bounded failure text. The actual worker owns this failure.
func TestVPPControlSocketCleanupRetainsLongOriginalCoordinates(t *testing.T) {
	socketCleanupIsolate(t)
	request := socketCleanupRequest(49166)
	request.AccessName = strings.Repeat("a", 255)
	request.Namespace = strings.Repeat("n", 255)
	cause := errors.New("original long-name endpoint close failed")
	socketCleanupCancel(t, request, cause)
	issue := socketCleanupIssue(t)
	for key, expected := range map[string]any{
		"access": request.AccessName, "device": request.Device,
		"control-device": request.ControlDevice, "namespace": request.Namespace,
		"session-id": request.SessionID, "client-mac": fmt.Sprintf("%x", request.PeerMAC),
	} {
		if got := issue.Detail[key]; got != expected {
			t.Errorf("original coordinate %q changed: got=%v want=%v", key, got, expected)
		}
	}
	if !strings.Contains(issue.Message, cause.Error()) {
		t.Fatalf("long coordinates displaced the original cause: %+v", issue)
	}
}

// The owning package declares its transient cleanup class for the registered
// recent-events doctor check, rather than attaching an unowned log code.
func TestVPPControlSocketCleanupDeclaresDoctorEvent(t *testing.T) {
	if !slices.Contains(diagnostic.EventCodes(), socketCleanupCode) {
		t.Fatal("PPPoE VPP socket cleanup has no owning doctor event declaration")
	}
}
