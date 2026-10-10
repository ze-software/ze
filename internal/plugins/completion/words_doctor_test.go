package completion

import (
	"bytes"
	"strings"
	"testing"

	_ "github.com/ze-software/ze/internal/component/doctor" // registers the doctor root this test completes
)

// TestWordsDoctorOffersItsKeywords proves `ze doctor <TAB>` offers the two
// keywords the doctor grammar takes.
// VALIDATES: `ze completion words doctor` lists config and kernel-capabilities,
// read from the doctor root's registered Subs.
// PREVENTS: a doctor mode an operator cannot discover by tab-completion.
func TestWordsDoctorOffersItsKeywords(t *testing.T) {
	var buf bytes.Buffer
	if code := writeWords(&buf, []string{"doctor"}); code != 0 {
		t.Fatalf("writeWords(doctor) = %d, want 0", code)
	}
	offered := map[string]bool{}
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		word, _, _ := strings.Cut(line, "\t")
		offered[word] = true
	}
	for _, want := range []string{"config", "kernel-capabilities"} {
		if !offered[want] {
			t.Errorf("ze doctor completion does not offer %q: %q", want, buf.String())
		}
	}
}
