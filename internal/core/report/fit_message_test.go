package report

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestFitMessageKeepsAnOverlongEventOnTheBus proves that a message longer than
// the bus accepts still raises its event once passed through FitMessage, cut
// at a rune boundary. Method: raise an error event whose message is a
// multi-byte rune that straddles the bound, then read the ring.
//
// PREVENTS: a refused reload whose error text is long being dropped by
// validFields with only a debug line, so `ze doctor` never shows it.
func TestFitMessageKeepsAnOverlongEventOnTheBus(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	// One byte short of the bound, then a three-byte rune that crosses it.
	long := strings.Repeat("a", MessageMax-1) + "€" + "tail"
	fitted := FitMessage(long)
	if len(fitted) > MessageMax {
		t.Fatalf("fitted message holds %d bytes, the bus accepts %d", len(fitted), MessageMax)
	}
	if !utf8.ValidString(fitted) {
		t.Fatalf("fitted message is cut inside a rune: %q", fitted[len(fitted)-3:])
	}
	if fitted != long[:MessageMax-1] {
		t.Fatalf("fitted message is %d bytes, want the %d bytes before the rune", len(fitted), MessageMax-1)
	}

	RaiseErrorFix("src", "code", "subject", fitted, "ze show errors")
	if got := Errors(0); len(got) != 1 {
		t.Fatalf("the bus holds %d events after the raise, want 1", len(got))
	}

	short := "short message"
	if got := FitMessage(short); got != short {
		t.Fatalf("FitMessage changed a message within the bound: %q", got)
	}
}
