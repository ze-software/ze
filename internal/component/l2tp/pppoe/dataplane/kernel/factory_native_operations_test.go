// Design: docs/research/l2tpv2-ze-integration.md -- production bind preflight

package kernel

import (
	"strings"
	"testing"
)

func TestFactoryNativeBindingUnavailableRefusesBeforeUnitCreation(t *testing.T) {
	// This existing fixture embeds only the old interface operations. Nil
	// creator/transport cannot create anything; native capability refusal must
	// precede reaching either one and report its actual missing boundary.
	channel, unit, err := setupOriginalNativePPP(&factoryBackend{}, nil, nil, "")
	if err == nil || !strings.Contains(err.Error(), "original-unit mutation binding") {
		t.Fatalf("missing native capability not refused: %v", err)
	}
	if channel != nil || unit != nil {
		t.Fatal("missing native capability allocated a PPP owner")
	}
}
